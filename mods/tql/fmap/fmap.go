package fmap

import (
	"fmt"
	"math"
	"math/cmplx"
	"strings"
	"time"

	"github.com/machbase/neo-server/mods/expression"
	"github.com/machbase/neo-server/mods/tql/context"
	"github.com/machbase/neo-server/mods/tql/conv"
	"github.com/machbase/neo-server/mods/tql/fcom"
	"gonum.org/v1/gonum/dsp/fourier"
)

var mapFunctionsMacro = [][2]string{
	{"SCRIPT(", "SCRIPT(CTX,K,V,"},
	{"TAKE(", "TAKE(CTX,K,V,"},
	{"DROP(", "DROP(CTX,K,V,"},
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
	"maxHz":      optf_maxHz,
	"minHz":      optf_minHz,
	"SCRIPT":     mapf_SCRIPT,
	"TAKE":       mapf_TAKE,
	"DROP":       mapf_DROP,
	"PUSHKEY":    mapf_PUSHKEY,
	"POPKEY":     mapf_POPKEY,
	"GROUPBYKEY": mapf_GROUPBYKEY,
	"FLATTEN":    mapf_FLATTEN,
	"FILTER":     mapf_FILTER,
	"FFT":        mapf_FFT,
}

func init() {
	for k, v := range fcom.Functions {
		functions[k] = v
	}
}

func Functions() []string {
	ret := []string{}
	for k := range functions {
		ret = append(ret, k)
	}
	return ret
}

func mapf_TAKE(args ...any) (any, error) {
	if len(args) != 4 {
		return nil, conv.ErrInvalidNumOfArgs("TAKE", 4, len(args))
	}
	ctx, ok := args[0].(*context.Context)
	if !ok {
		return nil, conv.ErrWrongTypeOfArgs("TAKE", 0, "context", args[0])
	}
	if limit, ok := args[3].(float64); ok {
		if ctx.Nrow > int(limit) {
			return context.ExecutionCircuitBreak, nil
		}
	} else if limit, ok := args[3].(int); ok {
		if ctx.Nrow > int(limit) {
			return context.ExecutionCircuitBreak, nil
		}
	} else {
		return nil, conv.ErrWrongTypeOfArgs("TAKE", 3, "int", args[3])
	}

	return &context.Param{K: args[1], V: args[2]}, nil
}

func mapf_DROP(args ...any) (any, error) {
	if len(args) != 4 {
		return nil, conv.ErrInvalidNumOfArgs("DROP", 4, len(args))
	}
	ctx, ok := args[0].(*context.Context)
	if !ok {
		return nil, conv.ErrWrongTypeOfArgs("DROP", 0, "context", args[0])
	}

	if limit, ok := args[3].(float64); ok {
		if ctx.Nrow <= int(limit) {
			return nil, nil
		}
	} else if limit, ok := args[3].(int); ok {
		if ctx.Nrow <= int(limit) {
			return nil, nil
		}
	} else {
		return nil, conv.ErrWrongTypeOfArgs("DROP", 3, "int", args[3])
	}

	return &context.Param{K: args[1], V: args[2]}, nil
}

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

	ret := &context.Param{
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
		ret := &context.Param{K: newKey, V: newVal}
		return ret, nil
	case [][]any:
		ret := make([]*context.Param, len(val))
		for i, v := range val {
			if len(v) < 2 {
				return nil, fmt.Errorf("f(POPKEY) arg elements should be larger than 2, but %d", len(v))
			}
			if len(v) == 2 {
				ret[i] = &context.Param{K: v[0], V: v[1]}
			} else {
				ret[i] = &context.Param{K: v[0], V: v[1:]}
			}
		}
		return ret, nil
	}
}

func mapf_GROUPBYKEY(args ...any) (any, error) {
	if len(args) != 3 {
		return nil, fmt.Errorf("f(GROUPBYKEY) invalid number of args (n:%d)", len(args))
	}

	ctx, ok := args[0].(*context.Context)
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
		ret := []*context.Param{}
		for _, elm := range arr {
			if subarr, ok := elm.([]any); ok {
				for _, subelm := range subarr {
					ret = append(ret, &context.Param{K: K, V: subelm})
				}
			} else if subarr, ok := elm.([][]any); ok {
				for _, subelm := range subarr {
					ret = append(ret, &context.Param{K: K, V: subelm})
				}
			} else {
				ret = append(ret, &context.Param{K: K, V: elm})
			}
		}
		return ret, nil
	} else {
		return &context.Param{K: K, V: V}, nil
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

	return &context.Param{K: args[1], V: args[2]}, nil
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
		// fmt.Errorf("f(FFT) samples should be more than 16")
		// drop input, instead of raising error
		return nil, nil
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

	ret := &context.Param{
		K: K,
		V: newVal,
	}

	return ret, nil
}
