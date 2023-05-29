package fmap

import (
	"fmt"
	"math"
	"math/cmplx"
	"strings"
	"time"

	"github.com/machbase/neo-server/mods/expression"
	"github.com/machbase/neo-server/mods/tagql/ctx"
	"gonum.org/v1/gonum/dsp/fourier"
)

var mapFunctionsMacro = [][2]string{
	{"PUSHKEY(", "PUSHKEY(CTX,K,V,"},
	{"POPKEY(", "POPKEY(CTX,K,V,"},
	{"GROUPBYKEY(", "GROUPBYKEY(CTX,K,V,"},
	{"FLATTEN(", "FLATTEN(CTX,K,V,"},
	{"FILTER(", "FILTER(CTX,K,V,"},
	{"FFT(", "FFT(CTX,K,V,"},
}

func Parse(text string) (*expression.Expression, error) {
	for _, f := range mapFunctionsMacro {
		text = strings.ReplaceAll(text, f[0], f[1])
	}
	text = strings.ReplaceAll(text, ",V,)", ",V)")
	text = strings.ReplaceAll(text, "K,V,K,V", "K,V")
	return expression.NewWithFunctions(text, functions)
}

var functions = map[string]expression.Function{
	"len":        mapf_len,
	"roundTime":  mapf_roundTime,
	"round":      mapf_round,
	"element":    mapf_element,
	"maxHz":      optf_maxHz,
	"minHz":      optf_minHz,
	"PUSHKEY":    mapf_PUSHKEY,
	"POPKEY":     mapf_POPKEY,
	"GROUPBYKEY": mapf_GROUPBYKEY,
	"FLATTEN":    mapf_FLATTEN,
	"FILTER":     mapf_FILTER,
	"FFT":        mapf_FFT,
}

// `len(V)`
func mapf_len(args ...any) (any, error) {
	return float64(len(args)), nil
}

// `roundTime(time, duration)`
func mapf_roundTime(args ...any) (any, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("f(roundTime) invalud args 'roundTime(time, 'duration')' (n:%d)", len(args))
	}
	var dur time.Duration
	if str, ok := args[1].(string); ok {
		if d, err := time.ParseDuration(str); err != nil {
			return nil, fmt.Errorf("f(roundTime) 2nd arg should be duration")
		} else {
			dur = d
		}
	} else if num, ok := args[1].(float64); ok {
		dur = time.Duration(int64(num))
	}
	if dur == 0 {
		return nil, fmt.Errorf("f(roundTime) zero duration")
	}

	var ret time.Time
	if ts, ok := args[0].(time.Time); ok {
		ret = time.Unix(0, (ts.UnixNano()/int64(dur))*int64(dur))
	} else if ts, ok := args[0].(*time.Time); ok {
		ret = time.Unix(0, (ts.UnixNano()/int64(dur))*int64(dur))
	} else {
		return nil, fmt.Errorf("f(roundTime) 1st arg should be time, but %T", args[0])
	}
	return ret, nil
}

// `round(number, number)`
func mapf_round(args ...any) (any, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("f(round) invalud args 'round(int, int)' (n:%d)", len(args))
	}
	var num int64
	var mod int64
	if d, ok := args[0].(int64); ok {
		num = d
	} else {
		return nil, fmt.Errorf("f(round) args should be non-zero int")
	}
	if d, ok := args[1].(int64); ok {
		mod = d
	} else {
		return nil, fmt.Errorf("f(round) args should be non-zero int")
	}

	return (num / mod) * mod, nil
}

// `element(V, idx)`
func mapf_element(args ...any) (any, error) {
	if len(args) < 3 {
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

// make new key by modulus of time
// `map=MODTIME('100ms')` produces `K:V` ==> `K':[K, V]` (K' = K % 100ms)
/*
func mapf_MODTIME(args ...any) (any, error) {
	if len(args) != 4 {
		return nil, fmt.Errorf("f(MODTIME) invalid number of args (n:%d)", len(args))
	}
	// K : time
	key, ok := args[1].(time.Time)
	if !ok {
		return nil, fmt.Errorf("f(MODTIME) K should be time, but %T", args[1])
	}
	// V : value
	val, ok := args[2].([]any)
	if !ok {
		return nil, fmt.Errorf("f(MODTIME) V should be []any, but %T", args[2])
	}
	// duration
	var mod int64
	switch d := args[3].(type) {
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

	ret := &ctx.Param{
		K: newKey,
		V: newVal,
	}
	return ret, nil
}
*/

// Merge all incoming values into a single key,
// incresing dimension of vector as result.
// `map=PUSHKEY(NewKEY)` produces `NewKEY: [K, V...]`
func mapf_PUSHKEY(args ...any) (any, error) {
	if len(args) != 4 {
		return nil, fmt.Errorf("f(PUSHKEY) invalid number of args (n:%d)", len(args))
	}
	// K : time
	key := args[1]
	// V : value
	var newVal []any
	if val, ok := args[2].([]any); ok {
		newVal = append([]any{key}, val...)
	} else {
		return nil, fmt.Errorf("f(PUSHKEY) V should be []any, but %T", args[2])
	}
	// newkey
	newKey := args[3]

	ret := &ctx.Param{
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
	if len(args) < 3 || len(args) > 4 {
		return nil, fmt.Errorf("f(POPKEY) invalid number of args (n:%d)", len(args)-3)
	}
	var nth = 0
	if len(args) == 4 {
		if arg2, ok := args[3].(float64); !ok {
			return nil, fmt.Errorf("f(POPKEY) 1st arg should be index of V, but %T", args[2])
		} else {
			nth = int(arg2)
		}
	}

	// V : value
	switch val := args[2].(type) {
	default:
		return nil, fmt.Errorf("f(POPKEY) V should be []any or [][]any, but %T", val)
	case []any:
		if nth < 0 || nth >= len(val) {
			return nil, fmt.Errorf("f(POPKEY) 1st arg should be between 0 and %d, but %d", len(val)-1, nth)
		}
		newKey := val[nth]
		newVal := append(val[0:nth], val[nth+1:]...)
		ret := &ctx.Param{K: newKey, V: newVal}
		return ret, nil
	case [][]any:
		ret := make([]*ctx.Param, len(val))
		for i, v := range val {
			if len(v) < 2 {
				return nil, fmt.Errorf("f(POPKEY) arg elements should be larger than 2, but %d", len(v))
			}
			if len(v) == 2 {
				ret[i] = &ctx.Param{K: v[0], V: v[1]}
			} else {
				ret[i] = &ctx.Param{K: v[0], V: v[1:]}
			}
		}
		return ret, nil
	}
}

func mapf_GROUPBYKEY(args ...any) (any, error) {
	if len(args) != 3 {
		return nil, fmt.Errorf("f(GROUPBYKEY) invalid number of args (n:%d)", len(args))
	}

	ctx, ok := args[0].(*ctx.Context)
	if !ok {
		return nil, fmt.Errorf("f(GROUPBYKEY) expect context, but %T", args[0])
	}
	K := args[1]
	V := args[2]
	var curKey any

	curKey, _ = ctx.Get("curKey")
	defer func() {
		ctx.Set("curKey", curKey)
	}()
	if curKey == nil {
		curKey = K
	}

	ctx.Buffer(K, V)

	if curKey != K {
		ctx.YieldBuffer(curKey)
		curKey = K
	}
	return nil, nil
}

func mapf_FLATTEN(args ...any) (any, error) {
	K := args[1]
	V := args[2]

	if arr, ok := V.([]any); ok {
		ret := []*ctx.Param{}
		for _, elm := range arr {
			if subarr, ok := elm.([]any); ok {
				for _, subelm := range subarr {
					ret = append(ret, &ctx.Param{K: K, V: subelm})
				}
			} else if subarr, ok := elm.([][]any); ok {
				for _, subelm := range subarr {
					ret = append(ret, &ctx.Param{K: K, V: subelm})
				}
			} else {
				ret = append(ret, &ctx.Param{K: K, V: elm})
			}
		}
		return ret, nil
	} else {
		return &ctx.Param{K: K, V: V}, nil
	}
}

// `map=FILTER(LEN(V)<1900)`
func mapf_FILTER(args ...any) (any, error) {
	if len(args) != 4 {
		return nil, fmt.Errorf("f(FILTER) invalid number of args (n:%d)", len(args))
	}
	flag, ok := args[3].(bool)
	if !ok {
		return nil, fmt.Errorf("f(FILTER) arg should be boolean, but %T", args[4])
	}
	if !flag {
		return nil, nil // drop this vector
	}

	return &ctx.Param{K: args[1], V: args[2]}, nil
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
	if len(args) < 3 {
		return nil, fmt.Errorf("f(FFT) invalid number of args (n:%d)", len(args))
	}
	// K : any
	K := args[1]
	// V : value
	V, ok := args[2].([]any)
	if !ok {
		return nil, fmt.Errorf("f(FFT) arg v should be []any, but %T", args[1])
	}
	minHz := math.NaN()
	maxHz := math.NaN()
	// options
	for _, arg := range args[3:] {
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

	ret := &ctx.Param{
		K: K,
		V: newVal,
	}

	return ret, nil
}
