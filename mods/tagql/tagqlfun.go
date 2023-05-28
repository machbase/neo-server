package tagql

import (
	"fmt"
	"math"
	"math/cmplx"
	"strconv"
	"strings"
	"time"

	"github.com/machbase/neo-server/mods/codec"
	"github.com/machbase/neo-server/mods/expression"
	"github.com/machbase/neo-server/mods/stream/spec"
	"gonum.org/v1/gonum/dsp/fourier"
)

// var fieldFunctions = map[string]expression.Function{
// 	"STDDEV":          func(args ...any) (any, error) { return nil, nil },
// 	"AVG":             func(args ...any) (any, error) { return nil, nil },
// 	"SUM":             func(args ...any) (any, error) { return nil, nil },
// 	"COUNT":           func(args ...any) (any, error) { return nil, nil },
// 	"TS_CHANGE_COUNT": func(args ...any) (any, error) { return nil, nil },
// 	"SUMSQ":           func(args ...any) (any, error) { return nil, nil },
// 	"FIRST":           func(args ...any) (any, error) { return nil, nil },
// 	"LAST":            func(args ...any) (any, error) { return nil, nil },
// }

var mapFunctions = map[string]expression.Function{
	"len":     mapf_len,
	"element": mapf_element,
	"maxHz":   optf_maxHz,
	"minHz":   optf_minHz,
	"MODTIME": mapf_MODTIME,
	"PUSHKEY": mapf_PUSHKEY,
	"POPKEY":  mapf_POPKEY,
	"FLATTEN": mapf_FLATTEN,
	"FILTER":  mapf_FILTER,
	"FFT":     mapf_FFT,
}

var mapFunctionsMacro = [][2]string{
	{"MODTIME(", "MODTIME(K,V,"},
	{"PUSHKEY(", "PUSHKEY(K,V,"},
	{"POPKEY(", "POPKEY(K,V,"},
	{"FLATTEN()", "FLATTEN(K,V)"},
	{"FILTER(", "FILTER(K,V,"},
	{"FFT(", "FFT(K,V,"},
}

func normalizeMapFuncExpr(expr string) string {
	for _, f := range mapFunctionsMacro {
		expr = strings.ReplaceAll(expr, f[0], f[1])
	}
	expr = strings.ReplaceAll(expr, "V,)", "V)")
	expr = strings.ReplaceAll(expr, "K,V,K,V", "K,V")
	return expr
}

var sinkFunctions = map[string]expression.Function{
	"heading":    sinkf_heading,
	"rownum":     sinkf_rownum,
	"timeformat": sinkf_timeformat,
	"precision":  sinkf_precision,
	"size":       sinkf_size,
	"title":      sinkf_title,
	"OUTPUT":     sinkf_OUTPUT,
}

func normalizeSinkFuncExpr(expr string) string {
	expr = strings.ReplaceAll(expr, "OUTPUT(", "OUTPUT(outstream,")
	expr = strings.ReplaceAll(expr, "outputstream,)", "outputstream)")
	return expr
}

var srcFunctions = map[string]expression.Function{
	"range": srcf_range,
	"limit": srcf_limit,
	"INPUT": srcf_INPUT,
}

// make new key by modulus of time
// `map=MODTIME('100ms')` produces `K:V` ==> `K':[K, V]` (K' = K % 100ms)
func mapf_MODTIME(args ...any) (any, error) {
	if len(args) != 3 {
		return nil, fmt.Errorf("f(MODTIME) invalid number of args (n:%d)", len(args))
	}
	// K : time
	key, ok := args[0].(time.Time)
	if !ok {
		return nil, fmt.Errorf("f(MODTIME) K should be time, but %T", args[0])
	}
	// V : value
	val, ok := args[1].([]any)
	if !ok {
		return nil, fmt.Errorf("f(MODTIME) V should be []any, but %T", args[1])
	}
	// duration
	var mod int64
	switch d := args[2].(type) {
	case float64:
		mod = int64(d)
	case string:
		td, err := time.ParseDuration(d)
		if err != nil {
			return nil, fmt.Errorf("f(MODTIME) 1st arg should be duration, %s", err.Error())
		}
		mod = int64(td)
	}

	newKey := time.Unix(0, (key.UnixNano()/mod)*mod)
	newVal := append([]any{key}, val...)

	ret := &ExecutionParam{
		K: newKey,
		V: newVal,
	}
	return ret, nil
}

// Merge all incoming values into a single key,
// incresing dimension of vector as result.
// `map=PUSHKEY(NewKEY)` produces `NewKEY: [K, V...]`
func mapf_PUSHKEY(args ...any) (any, error) {
	if len(args) != 3 {
		return nil, fmt.Errorf("f(PUSHKEY) invalid number of args (n:%d)", len(args))
	}
	// K : time
	key := args[0]
	// V : value
	val, ok := args[1].([]any)
	if !ok {
		return nil, fmt.Errorf("f(PUSHKEY) V should be []any, but %T", args[1])
	}
	// newkey
	newKey := args[2]

	newVal := append([]any{key}, val...)
	ret := &ExecutionParam{
		K: newKey,
		V: newVal,
	}
	return ret, nil
}

// Drop Key, then make the first element of value to promote as a key,
// decrease dimension of vector as result if the input is not multiple dimension vector.
// `map=POPKEY(V, 0)` produces
// 1 dimension : `K: [V1, V2, V3...]` ==> `V1 : [V2, V3, .... ]`
// 2 dimension : `K: [[V11, V12, V13...],[V21, V22, V23...], ...] ==> `V11: [V12, V13...]` and `V21: [V22, V23...]` ...
func mapf_POPKEY(args ...any) (any, error) {
	if len(args) < 2 || len(args) > 3 {
		return nil, fmt.Errorf("f(POPKEY) requires 2 args, but got %d", len(args))
	}
	var nth = 0
	if len(args) == 3 {
		if arg2, ok := args[2].(float64); !ok {
			return nil, fmt.Errorf("f(POPKEY) 1st arg should be index of V, but %T", args[1])
		} else {
			nth = int(arg2)
		}
	}

	// V : value
	switch val := args[1].(type) {
	default:
		return nil, fmt.Errorf("f(POPKEY) V should be []any or [][]any, but %T", val)
	case []any:
		if nth < 0 || nth >= len(val) {
			return nil, fmt.Errorf("f(POPKEY) 1st arg should be between 0 and %d, but %d", len(val)-1, nth)
		}
		newKey := val[nth]
		newVal := append(val[0:nth], val[nth+1:]...)
		ret := &ExecutionParam{K: newKey, V: newVal}
		return ret, nil
	case [][]any:
		ret := make([]*ExecutionParam, len(val))
		for i, v := range val {
			if len(v) < 2 {
				return nil, fmt.Errorf("f(POPKEY) arg elements should be larger than 2, but %d", len(v))
			}
			if len(v) == 2 {
				ret[i] = &ExecutionParam{K: v[0], V: v[1]}
			} else {
				ret[i] = &ExecutionParam{K: v[0], V: v[1:]}
			}
		}
		return ret, nil
	}
}

func mapf_FLATTEN(args ...any) (any, error) {
	K := args[0]
	V := args[1]

	if arr, ok := V.([]any); ok {
		ret := []*ExecutionParam{}
		for _, elm := range arr {
			if subarr, ok := elm.([]any); ok {
				for _, subelm := range subarr {
					ret = append(ret, &ExecutionParam{K: K, V: subelm})
				}
			} else if subarr, ok := elm.([][]any); ok {
				for _, subelm := range subarr {
					ret = append(ret, &ExecutionParam{K: K, V: subelm})
				}
			} else {
				ret = append(ret, &ExecutionParam{K: K, V: elm})
			}
		}
		return ret, nil
	} else {
		return &ExecutionParam{K: K, V: V}, nil
	}
}

// `map=FILTER(LEN(V)<1900)`
func mapf_FILTER(args ...any) (any, error) {
	if len(args) != 3 {
		return nil, fmt.Errorf("f(FILTER) invalid number of args (n:%d)", len(args))
	}
	flag, ok := args[2].(bool)
	if !ok {
		return nil, fmt.Errorf("f(FILTER) arg should be boolean, but %T", args[2])
	}
	if !flag {
		return nil, nil // drop this vector
	}

	return &ExecutionParam{K: args[0], V: args[1]}, nil
}

// `len(V)`
func mapf_len(args ...any) (any, error) {
	return float64(len(args)), nil
}

// `element(V, idx)`
func mapf_element(args ...any) (any, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("f(element) invalud number of args (n:%d)", len(args))
	}
	var idx int
	if n, ok := args[len(args)-1].(float64); ok {
		idx = int(n)
	} else {
		return nil, fmt.Errorf("f(element) 2nd arg should be int")
	}
	if len(args)-1 <= idx {
		return nil, fmt.Errorf("f(element) out of index %d / %d", idx, len(args)-1)
	}
	switch v := args[idx].(type) {
	case int:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case float64:
		return v, nil
	case string:
		return v, nil
	case bool:
		return v, nil
	case time.Time:
		return float64(v.UnixNano()) / float64(time.Second), nil
	default:
		return nil, fmt.Errorf("f(element) unsupported type %T", v)
	}
}

type maxHzOpt float64

func optf_maxHz(args ...any) (any, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("f(maxHz) invalid number of args (n:%d)", len(args))
	}
	if v, ok := args[0].(float64); ok {
		return maxHzOpt(v), nil
	}
	return nil, fmt.Errorf("f(maxHz) invalid parameter: %T", args[0])
}

type minHzOpt float64

func optf_minHz(args ...any) (any, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("f(minHz) invalid number of args (n:%d)", len(args))
	}
	if v, ok := args[0].(float64); ok {
		return minHzOpt(v), nil
	}
	return nil, fmt.Errorf("f(minHz) invalid parameter: %T", args[0])
}

// `map=FFT()` : K is any, V is array of array(time, value)
func mapf_FFT(args ...any) (any, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("f(FFT) invalid number of args (n:%d)", len(args))
	}
	// K : any
	K := args[0]
	// V : value
	V, ok := args[1].([]any)
	if !ok {
		return nil, fmt.Errorf("f(FFT) arg v should be []any, but %T", args[1])
	}
	minHz := math.NaN()
	maxHz := math.NaN()
	// options
	for _, arg := range args[2:] {
		switch v := arg.(type) {
		case minHzOpt:
			minHz = float64(v)
		case maxHzOpt:
			maxHz = float64(v)
		}
	}

	lenSamples := len(V)
	if lenSamples < 16 {
		return nil, fmt.Errorf("f(FFT) samples should be more than 16")
	}

	sampleTimes := make([]time.Time, lenSamples)
	sampleValues := make([]float64, lenSamples)
	for i := range V {
		tuple, ok := V[i].([]any)
		if !ok {
			return nil, fmt.Errorf("f(FFT) sample should be a tuple of (time, value), but %T", V[i])
		}
		sampleTimes[i], ok = tuple[0].(time.Time)
		if !ok {
			if pt, ok := tuple[0].(*time.Time); !ok {
				return nil, fmt.Errorf("f(FFT) invalid %dth sample time, but %T", i, tuple[0])
			} else {
				sampleTimes[i] = *pt
			}
		}
		sampleValues[i], ok = tuple[1].(float64)
		if !ok {
			if pt, ok := tuple[1].(*float64); !ok {
				return nil, fmt.Errorf("f(FFT) invalid %dth sample value, but %T", i, tuple[1])
			} else {
				sampleValues[i] = *pt
			}
		}
	}

	samplesDuration := sampleTimes[lenSamples-1].Sub(sampleTimes[0])
	period := float64(lenSamples) / (float64(samplesDuration) / float64(time.Second))
	fft := fourier.NewFFT(lenSamples)

	// amplifier := func(v float64) float64 { return v }
	amplifier := func(v float64) float64 {
		return v * 2.0 / float64(lenSamples)
	}

	coeff := fft.Coefficients(nil, sampleValues)

	newVal := [][]any{}
	// newVal := make([][]any, len(coeff))
	for i, c := range coeff {
		hz := fft.Freq(i) * period
		if hz == 0 {
			// newVal[i] = []any{0., 0.}
			continue
		}
		if hz < minHz {
			continue
		}
		if hz > maxHz {
			continue
		}
		magnitude := cmplx.Abs(c)
		amplitude := amplifier(magnitude)
		// phase = cmplx.Phase(c)
		newVal = append(newVal, []any{hz, amplitude})
	}

	ret := &ExecutionParam{
		K: K,
		V: newVal,
	}

	return ret, nil
}

func sinkf_timeformat(args ...any) (any, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("f(timeformat) invalid arg `timeformat(string)`")
	}
	if timeformat, ok := args[0].(string); !ok {
		return nil, fmt.Errorf("f(timeformat) invalid arg `timeformat(string)`")
	} else {
		return codec.TimeFormat(timeformat), nil
	}
}

func sinkf_heading(args ...any) (any, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("f(heading) invalid arg `heading(bool)`")
	}
	if flag, ok := args[0].(bool); !ok {
		return nil, fmt.Errorf("f(heading) invalid arg `heading(bool)`")
	} else {
		return codec.Heading(flag), nil
	}
}

func sinkf_rownum(args ...any) (any, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("f(rownum) invalid arg `rownum(bool)`")
	}
	if flag, ok := args[0].(bool); !ok {
		return nil, fmt.Errorf("f(rownum) invalid arg `rownum(bool)`")
	} else {
		return codec.Rownum(flag), nil
	}
}

func sinkf_precision(args ...any) (any, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("f(precision) invalid arg `precision(int)`")
	}
	if prec, ok := args[0].(float64); !ok {
		return nil, fmt.Errorf("f(precision) invalid arg `precision(int)`")
	} else {
		return codec.Precision(int(prec)), nil
	}
}
func sinkf_size(args ...any) (any, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("f(size) invalid arg `size(width string, height string)`")
	}
	width, ok := args[0].(string)
	if !ok {
		return nil, fmt.Errorf("f(size) invalid width, should be string, but %T`", args[0])
	}
	height, ok := args[1].(string)
	if !ok {
		return nil, fmt.Errorf("f(size) invalid height, should be string, but %T`", args[1])
	}

	return codec.Size(width, height), nil
}

func sinkf_title(args ...any) (any, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("f(title) invalid arg `title(string)`")
	}
	str, ok := args[0].(string)
	if !ok {
		return nil, fmt.Errorf("f(title) invalid title, should be string, but %T`", args[0])
	}
	return codec.Title(str), nil
}

// `sink=OUTPUT(format, opts...)`
func sinkf_OUTPUT(args ...any) (any, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("f(OUTPUT) invalid number of args (n:%d)", len(args))
	}
	outstream, ok := args[0].(spec.OutputStream)
	if !ok {
		return nil, fmt.Errorf("f(OUTPUT) invalid output stream, but %T", args[0])
	}

	format, ok := args[1].(string)
	if !ok {
		return nil, fmt.Errorf("f(OUTPUT) 1st arg must be format in string, but %T", args[1])
	}

	opts := []codec.Option{
		codec.OutputStream(outstream),
	}
	for i, arg := range args[2:] {
		if op, ok := arg.(codec.Option); !ok {
			return nil, fmt.Errorf("f(OUTPUT) invalid option %d %T", i, arg)
		} else {
			opts = append(opts, op)
		}
	}
	encoder := codec.NewEncoder(format, opts...)
	return encoder, nil
}

type Range struct {
	ts       string
	tsTime   time.Time
	duration time.Duration
	groupBy  time.Duration
}

func srcf_range(args ...any) (any, error) {
	if len(args) != 2 && len(args) != 3 {
		return nil, fmt.Errorf("f(range) invalid number of args (n:%d)", len(args))
	}
	ret := &Range{}
	if str, ok := args[0].(string); ok {
		if str != "now" && str != "last" {
			return nil, fmt.Errorf("f(range) 1st args should be time or 'now', 'last', but %T", args[0])
		}
		ret.ts = str
	} else {
		if num, ok := args[0].(float64); ok {
			ret.tsTime = time.Unix(0, int64(num))
		} else {
			if ts, ok := args[0].(time.Time); ok {
				ret.tsTime = ts
			} else {
				return nil, fmt.Errorf("f(range) 1st args should be time or 'now', 'last', but %T", args[0])
			}
		}
	}
	if str, ok := args[1].(string); ok {
		if d, err := time.ParseDuration(str); err == nil {
			ret.duration = d
		} else {
			return nil, fmt.Errorf("f(range) 2nd args should be duration, %s", err.Error())
		}
	} else if d, ok := args[1].(float64); ok {
		ret.duration = time.Duration(int64(d))
	} else {
		return nil, fmt.Errorf("f(range) 2nd args should be duration, but %T", args[1])
	}
	if len(args) == 2 {
		return ret, nil
	}

	if str, ok := args[2].(string); ok {
		if d, err := time.ParseDuration(str); err == nil {
			ret.groupBy = d
		} else {
			return nil, fmt.Errorf("f(range) 3rd args should be duration, %s", err.Error())
		}
	} else if d, ok := args[1].(float64); ok {
		ret.groupBy = time.Duration(int64(d))
	} else {
		return nil, fmt.Errorf("f(range) 3rd args should be duration, but %T", args[1])
	}
	if ret.duration <= ret.groupBy {
		return nil, fmt.Errorf("f(range) 3rd args should be smaller than 2nd")
	}

	return ret, nil
}

type Limit struct {
	limit int
}

func (lm *Limit) String() string {
	return strconv.Itoa(lm.limit)
}

func srcf_limit(args ...any) (any, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("f(limit) invalid number of args (n:%d)", len(args))
	}
	ret := &Limit{}
	if d, ok := args[0].(float64); ok {
		ret.limit = int(d)
	} else {
		return nil, fmt.Errorf("f(range) arg should be int, but %T", args[1])
	}
	return ret, nil
}

type SrcInput struct {
	Columns []string
	Range   *Range
	Limit   *Limit
}

func newSrcInput() *SrcInput {
	return &SrcInput{
		Columns: []string{},
		Range:   &Range{ts: "last", duration: time.Second, groupBy: 0},
		Limit:   &Limit{limit: 1000000},
	}
}

// src=INPUT('value', 'STDDEV(val)', range('last', '10s', '1s'), limit(100000) )
func srcf_INPUT(args ...any) (any, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("f(INPUT) invalid number of args (n:%d)", len(args))
	}

	ret := newSrcInput()
	for i, arg := range args {
		switch tok := arg.(type) {
		case string:
			ret.Columns = append(ret.Columns, tok)
		case *Range:
			ret.Range = tok
		case *Limit:
			ret.Limit = tok
		default:
			return nil, fmt.Errorf("f(INPUT) unknown type of args[%d], %T", i, tok)
		}
	}
	return ret, nil
}
