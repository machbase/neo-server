package tagql

import (
	"fmt"
	"math"
	"math/cmplx"
	"strings"
	"time"

	"github.com/machbase/neo-server/mods/expression"
	"gonum.org/v1/gonum/dsp/fourier"
)

var fieldFunctions = map[string]expression.Function{
	"STDDEV":          func(args ...any) (any, error) { return nil, nil },
	"AVG":             func(args ...any) (any, error) { return nil, nil },
	"SUM":             func(args ...any) (any, error) { return nil, nil },
	"COUNT":           func(args ...any) (any, error) { return nil, nil },
	"TS_CHANGE_COUNT": func(args ...any) (any, error) { return nil, nil },
	"SUMSQ":           func(args ...any) (any, error) { return nil, nil },
	"FIRST":           func(args ...any) (any, error) { return nil, nil },
	"LAST":            func(args ...any) (any, error) { return nil, nil },
}

var mapFunctions = map[string]expression.Function{
	"len":      mapf_len,
	"element":  mapf_element,
	"maxHzOpt": optf_maxHz,
	"minHzOpt": optf_minHz,
	"MODTIME":  mapf_MODTIME,
	"PUSHKEY":  mapf_PUSHKEY,
	"POPKEY":   mapf_POPKEY,
	"FLATTEN":  mapf_FLATTEN,
	"FILTER":   mapf_FILTER,
	"FFT":      mapf_FFT,
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
	expr = strings.ReplaceAll(expr, "K,V,K,V)", "K,V)")
	return expr
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
		return nil, fmt.Errorf("f(maxHzOpt) invalid number of args (n:%d)", len(args))
	}
	if v, ok := args[0].(float64); ok {
		return maxHzOpt(v), nil
	}
	return nil, fmt.Errorf("f(maxHzOpt) invalid parameter: %T", args[0])
}

type minHzOpt float64

func optf_minHz(args ...any) (any, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("f(minHzOpt) invalid number of args (n:%d)", len(args))
	}
	if v, ok := args[0].(float64); ok {
		return minHzOpt(v), nil
	}
	return nil, fmt.Errorf("f(minHzOpt) invalid parameter: %T", args[0])
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
