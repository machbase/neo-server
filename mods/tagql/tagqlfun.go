package tagql

import (
	"fmt"
	"math/cmplx"
	"time"

	"github.com/machbase/neo-server/mods/expression"
	"gonum.org/v1/gonum/dsp/fourier"
)

var yieldFunctions = map[string]expression.Function{
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
	"GROUP_TIME": mapf_GROUP_TIME,
	"FFT":        mapf_FFT,
	"MERGE":      mapf_MERGE,
	"FLATTEN":    mapf_FLATTEN,
}

// `map=GROUP_TIME(K, V, 1000*1000*1000)` --> group by 1 second
func mapf_GROUP_TIME(args ...any) (any, error) {
	if len(args) != 3 {
		return nil, fmt.Errorf("f(GROUP_TIME) invalid number of args (n:%d)", len(args))
	}
	// K : time
	key, ok := args[0].(time.Time)
	if !ok {
		return nil, fmt.Errorf("f(GROUP_TIME) arg k should be time, but %T", args[0])
	}
	// V : value
	val, ok := args[1].([]any)
	if !ok {
		return nil, fmt.Errorf("f(GROUP_TIME) arg v should be []any, but %T", args[1])
	}
	// duration
	var mod int64
	switch d := args[2].(type) {
	case float64:
		mod = int64(d)
	case string:
		td, err := time.ParseDuration(d)
		if err != nil {
			return nil, fmt.Errorf("f(GROUP_TIME) 3rd argument should be duration, %s", err.Error())
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

// `map=MERGE(K, V, NewKEY)` --> merge to array of array(K, V) with NewKEY
func mapf_MERGE(args ...any) (any, error) {
	if len(args) != 3 {
		return nil, fmt.Errorf("f(MERGE) invalid number of args (n:%d)", len(args))
	}
	// K : time
	key, ok := args[0].(time.Time)
	if !ok {
		return nil, fmt.Errorf("f(MERGE) arg k should be time, but %T", args[0])
	}
	// V : value
	val, ok := args[1].([]any)
	if !ok {
		return nil, fmt.Errorf("f(MERGE) arg v should be []any, but %T", args[1])
	}
	// newkey
	newKey, ok := args[2].(string)
	if !ok {
		return nil, fmt.Errorf("f(MERGE) 3rd argument should be string, but %T", args[2])
	}

	newVal := append([]any{key}, val...)

	ret := &ExecutionParam{
		K: newKey,
		V: newVal,
	}
	return ret, nil
}

// `map=FLATTEN(V)` --> produce key: V -> V[0], value: V[1:]
func mapf_FLATTEN(args ...any) (any, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("f(FLATTEN) invalid number of args (n:%d)", len(args))
	}
	// V : value
	switch val := args[0].(type) {
	case []any:
		if len(val) < 2 {
			return nil, fmt.Errorf("f(FLATTEN) arg v length should be larger 2, but %d", len(val))
		}
		ret := &ExecutionParam{
			K: val[0],
			V: val[1:],
		}
		return ret, nil
	case [][]any:
		ret := make([]*ExecutionParam, len(val))
		for i, v := range val {
			if len(v) < 2 {
				return nil, fmt.Errorf("f(FLATTEN) arg v elements should be larger than 2, but %d", len(v))
			}
			if len(v) == 2 {
				ret[i] = &ExecutionParam{K: v[0], V: v[1]}
			} else {
				ret[i] = &ExecutionParam{K: v[0], V: v[1:]}
			}
		}
		return ret, nil
	default:
		return nil, fmt.Errorf("f(FLATTEN) arg v should be []any or [][]any, but %T", val)
	}
}

// `map=FFT(K, V)` : K is any, V is array of array(time, value)
func mapf_FFT(args ...any) (any, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("f(FFT) invalid number of args (n:%d)", len(args))
	}
	// K : any
	K := args[0]
	// V : value
	V, ok := args[1].([]any)
	if !ok {
		return nil, fmt.Errorf("f(FFT) arg v should be []any, but %T", args[1])
	}
	lenSamples := len(V)
	if lenSamples < 16 {
		return nil, fmt.Errorf("f(FFT) samples should more than 16")
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
	for i, c := range coeff {
		hz := fft.Freq(i) * period
		if hz == 0 {
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
