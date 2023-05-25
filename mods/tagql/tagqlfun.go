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
	"MODTIME": mapf_MODTIME,
	"PUSHKEY": mapf_PUSHKEY,
	"POPKEY":  mapf_POPKEY,
	"FFT":     mapf_FFT,
}

// make new key by modulus of time
// `map=MODTIME(K, V, '100ms')` produces `K' : [K, V...]` (K' = K % 100ms)
func mapf_MODTIME(args ...any) (any, error) {
	if len(args) != 3 {
		return nil, fmt.Errorf("f(MODTIME) invalid number of args (n:%d)", len(args))
	}
	// K : time
	key, ok := args[0].(time.Time)
	if !ok {
		return nil, fmt.Errorf("f(MODTIME) 1st arg should be time, but %T", args[0])
	}
	// V : value
	val, ok := args[1].([]any)
	if !ok {
		return nil, fmt.Errorf("f(MODTIME) 2nd arg should be []any, but %T", args[1])
	}
	// duration
	var mod int64
	switch d := args[2].(type) {
	case float64:
		mod = int64(d)
	case string:
		td, err := time.ParseDuration(d)
		if err != nil {
			return nil, fmt.Errorf("f(MODTIME) 3rd arg should be duration, %s", err.Error())
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

// merge all incoming values into a single key
// `map=PUSHKEY(NewKEY, K, V)` produces `NewKEY: [K, V...]`
func mapf_PUSHKEY(args ...any) (any, error) {
	if len(args) != 3 {
		return nil, fmt.Errorf("f(PUSHKEY) invalid number of args (n:%d)", len(args))
	}
	// newkey
	newKey := args[0]
	// K : time
	key := args[1]
	// V : value
	val, ok := args[2].([]any)
	if !ok {
		return nil, fmt.Errorf("f(PUSHKEY) 3rd arg should be []any, but %T", args[1])
	}

	newVal := append([]any{key}, val...)
	ret := &ExecutionParam{
		K: newKey,
		V: newVal,
	}
	return ret, nil
}

// Drop Key, then make the first element of value to promote as a Key
// `map=POPKEY(V)` produces
// 1 dimension : `K: [V1, V2, V3...]` ==> `V1 : [V2, V3, .... ]`
// 2 dimension : `K: [[V11, V12, V13...],[V21, V22, V23...], ...] ==> `V11: [V12, V13...]` and `V21: [V22, V23...]` ...
func mapf_POPKEY(args ...any) (any, error) {
	fmt.Printf("==> %#v\n", args)
	if len(args) != 1 {
		return nil, fmt.Errorf("f(POPKEY) invalid number of args (n:%d)", len(args))
	}
	// V : value
	switch val := args[0].(type) {
	default:
		if len(args) < 2 {
			return nil, fmt.Errorf("f(POPKEY) arg should be []any or [][]any, but %T", val)
		}
		ret := &ExecutionParam{
			K: args[0],
			V: args[1:],
		}
		return ret, nil
	case []any:
		if len(val) < 2 {
			return nil, fmt.Errorf("f(POPKEY) arg length should be larger 2, but %d", len(val))
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
