package tql

import (
	"fmt"
	"math"
	"time"

	"github.com/machbase/neo-server/mods/nums/fft"
)

type maxHzOption float64

func (x *Task) fmMaxHz(freq float64) maxHzOption {
	return maxHzOption(freq)
}

type minHzOption float64

func (x *Task) fmMinHz(freq float64) minHzOption {
	return minHzOption(freq)
}

func (x *Task) fmFastFourierTransform(node *Node, key any, value []any, args ...any) (any, error) {
	minHz := math.NaN()
	maxHz := math.NaN()
	// options
	for _, arg := range args {
		switch v := arg.(type) {
		case minHzOption:
			minHz = float64(v)
		case maxHzOption:
			maxHz = float64(v)
		}
	}

	lenSamples := len(value)
	if lenSamples < 16 {
		// fmt.Errorf("f(FFT) samples should be more than 16")
		// drop input, instead of raising error
		return nil, nil
	}

	sampleTimes := make([]time.Time, lenSamples)
	sampleValues := make([]float64, lenSamples)
	for i := range value {
		tuple, ok := value[i].([]any)
		if !ok {
			return nil, fmt.Errorf("f(FFT) sample should be a tuple of (time, value), but %T", value[i])
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

	freqs, values := fft.FastFourierTransform(sampleTimes, sampleValues)

	newVal := [][]any{}
	for i := range freqs {
		hz := freqs[i]
		amplitude := values[i]
		if hz == 0 || hz < minHz || hz > maxHz {
			continue
		}
		newVal = append(newVal, []any{hz, amplitude})
	}

	ret := node.NewRecord(key, newVal)
	return ret, nil
}
