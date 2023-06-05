package fsrc

import (
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	spi "github.com/machbase/neo-spi"
)

type fakeSource interface {
	Header() spi.Columns
	Gen() <-chan []any
	Stop()
}

/*
Example)

	 INPUT(
		FAKE(
		    oscilator(
		            range(time('now','-10s'), '10s', '1ms'),
		            freq(100, amplitude [,phase [, bias]]),
		            freq(240, amplitude [,phase [, bias]]),
		        )
		    )
		)
*/
func src_FAKE(args ...any) (any, error) {
	if len(args) != 1 {
		return nil, errInvalidNumOfArgs("FAKE", 1, len(args))
	}
	if gen, ok := args[0].(fakeSource); ok {
		return gen, nil
	} else {
		return nil, errWrongTypeOfArgs("FAKE", 0, "fakeSource", args[0])
	}
}

func src_oscilator(args ...any) (any, error) {
	ret := &oscilator{}
	for _, arg := range args {
		switch v := arg.(type) {
		default:
			return nil, fmt.Errorf("f(oscilator) invalid arg type '%T'", v)
		case *freq:
			ret.frequencies = append(ret.frequencies, v)
		case *timeRange:
			if ret.timeRange != nil {
				return nil, fmt.Errorf("f(oscilator) duplicated time range, %v", v)
			}
			ret.timeRange = v
		}
	}

	if ret.timeRange == nil {
		return nil, errors.New("f(oscilator) no time range is defined")
	}
	if ret.timeRange.period <= 0 {
		return nil, errors.New("f(oscilator) period should be positive")
	}
	return ret, nil
}

type oscilator struct {
	timeRange   *timeRange
	frequencies []*freq
	ch          chan []any
	alive       bool
	closeWait   sync.WaitGroup
}

var _ fakeSource = &oscilator{}

func (fs *oscilator) Header() spi.Columns {
	return []*spi.Column{{Name: "time", Type: "datetime"}, {Name: "value", Type: "double"}}
}

func (fs *oscilator) Gen() <-chan []any {
	fs.ch = make(chan []any)
	fs.alive = true
	fs.closeWait.Add(1)
	go func() {
		var from int64
		var to int64
		var step int64 = int64(fs.timeRange.period)
		if fs.timeRange.duration < 0 {
			from = fs.timeRange.tsTime.Add(fs.timeRange.duration).UnixNano()
			to = fs.timeRange.tsTime.UnixNano()
		} else {
			from = fs.timeRange.tsTime.UnixNano()
			to = fs.timeRange.tsTime.Add(fs.timeRange.duration).UnixNano()
		}

		for x := from; fs.alive && x < to; x += step {
			value := 0.0
			for _, fr := range fs.frequencies {
				value += fr.Value(float64(x) / float64(time.Second))
			}
			fs.ch <- []any{time.Unix(0, x), value}
		}
		close(fs.ch)
		fs.closeWait.Done()
	}()
	return fs.ch
}

func (fs *oscilator) Stop() {
	fs.alive = false
	fs.closeWait.Wait()
}

type freq struct {
	hertz     float64
	amplitude float64
	phase     float64
	bias      float64
}

func (fr *freq) Value(x float64) float64 {
	return fr.amplitude*math.Sin(2*math.Pi*fr.hertz*x+fr.phase) + fr.bias
}

// freq(240, amplitude [, bias [, phase]])
func srcf_freq(args ...any) (any, error) {
	if len(args) < 2 || len(args) > 4 {
		return nil, errInvalidNumOfArgs("freq", 2, len(args))
	}
	ret := &freq{}
	if fr, ok := args[0].(float64); ok {
		ret.hertz = fr
	} else {
		return nil, errWrongTypeOfArgs("freq", 0, "frequency(float64)", args[0])
	}

	if amp, ok := args[1].(float64); ok {
		ret.amplitude = amp
	} else {
		return nil, errWrongTypeOfArgs("freq", 0, "amplitude(float64)", args[1])
	}

	if len(args) >= 3 {
		if bias, ok := args[2].(float64); ok {
			ret.bias = bias
		} else {
			return nil, errWrongTypeOfArgs("freq", 0, "bias(float64)", args[2])
		}
	}
	if len(args) >= 4 {
		if phase, ok := args[3].(float64); ok {
			ret.phase = phase
		} else {
			return nil, errWrongTypeOfArgs("freq", 0, "phase(float64)", args[3])
		}
	}
	return ret, nil
}
