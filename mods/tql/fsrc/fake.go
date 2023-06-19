package fsrc

import (
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/machbase/neo-server/mods/tql/conv"
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
		FAKE( oscilator() | sphere() )
*/
func src_FAKE(args ...any) (any, error) {
	if len(args) != 1 {
		return nil, conv.ErrInvalidNumOfArgs("FAKE", 1, len(args))
	}
	if gen, ok := args[0].(fakeSource); ok {
		return gen, nil
	} else if arr, ok := args[0].([][][]float64); ok {
		return &meshgrid{vals: arr}, nil
	} else if arr, ok := args[0].([]float64); ok {
		return &linspace{vals: arr}, nil
	} else {
		return nil, conv.ErrWrongTypeOfArgs("FAKE", 0, "fakeSource", args[0])
	}
}

var _ fakeSource = &meshgrid{}

type meshgrid struct {
	vals [][][]float64

	ch        chan []any
	alive     bool
	closeWait sync.WaitGroup
}

func (mg *meshgrid) Header() spi.Columns {
	return []*spi.Column{{Name: "x", Type: "double"}, {Name: "y", Type: "double"}, {Name: "z", Type: "double"}}
}

func (mg *meshgrid) Gen() <-chan []any {
	mg.ch = make(chan []any)
	mg.alive = true
	mg.closeWait.Add(1)
	go func() {
		id := 0
		for x := range mg.vals {
			for y := range mg.vals[x] {
				if !mg.alive {
					goto done
				}
				elm := mg.vals[x][y]
				if len(elm) == 2 {
					id++
					mg.ch <- []any{id, elm[0], elm[1]}
				}
			}
		}
	done:
		close(mg.ch)
		mg.closeWait.Done()
	}()
	return mg.ch
}

func (mg *meshgrid) Stop() {
	mg.alive = false
	mg.closeWait.Wait()
}

var _ fakeSource = &linspace{}

type linspace struct {
	vals []float64

	ch        chan []any
	alive     bool
	closeWait sync.WaitGroup
}

func (ls *linspace) Header() spi.Columns {
	return []*spi.Column{{Name: "x", Type: "double"}, {Name: "y", Type: "double"}, {Name: "z", Type: "double"}}
}

func (ls *linspace) Gen() <-chan []any {
	ls.ch = make(chan []any)
	ls.alive = true
	ls.closeWait.Add(1)
	go func() {
		id := 0
		for _, v := range ls.vals {
			if !ls.alive {
				goto done
			}
			id++
			ls.ch <- []any{id, v}
		}
	done:
		close(ls.ch)
		ls.closeWait.Done()
	}()
	return ls.ch
}

func (ls *linspace) Stop() {
	ls.alive = false
	ls.closeWait.Wait()
}

func src_sphere(args ...any) (any, error) {
	return &sphere{
		latStep: 36,
		lonStep: 18,
	}, nil
}

var _ fakeSource = &sphere{}

type sphere struct {
	latStep float64
	lonStep float64

	ch        chan []any
	alive     bool
	closeWait sync.WaitGroup
}

func (sp *sphere) Header() spi.Columns {
	return []*spi.Column{{Name: "x", Type: "double"}, {Name: "y", Type: "double"}, {Name: "z", Type: "double"}}
}

func (sp *sphere) Gen() <-chan []any {
	sp.ch = make(chan []any)
	sp.alive = true
	sp.closeWait.Add(1)
	go func() {
		var u, v float64
		for u = 0; sp.alive && u < 2.0*math.Pi; u += (2.0 * math.Pi) / sp.latStep {
			for v = 0; sp.alive && v < math.Pi; v += math.Pi / sp.lonStep {
				x := math.Cos(u) * math.Sin(v)
				y := math.Sin(u) * math.Sin(v)
				z := math.Cos(v)
				sp.ch <- []any{x, y, z}
			}
		}
		close(sp.ch)
		sp.closeWait.Done()
	}()
	return sp.ch
}

func (sp *sphere) Stop() {
	sp.alive = false
	sp.closeWait.Wait()
}

// // oscilator(
// //		range(time('now','-10s'), '10s', '1ms'),
// //		freq(100, amplitude [,phase [, bias]]),
// //		freq(240, amplitude [,phase [, bias]]),
// //	)
// // )
func src_oscillator(args ...any) (any, error) {
	ret := &oscillator{}
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

type oscillator struct {
	timeRange   *timeRange
	frequencies []*freq
	ch          chan []any
	alive       bool
	closeWait   sync.WaitGroup
}

var _ fakeSource = &oscillator{}

func (fs *oscillator) Header() spi.Columns {
	return []*spi.Column{{Name: "time", Type: "datetime"}, {Name: "value", Type: "double"}}
}

func (fs *oscillator) Gen() <-chan []any {
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

func (fs *oscillator) Stop() {
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
		return nil, conv.ErrInvalidNumOfArgs("freq", 2, len(args))
	}
	var err error
	ret := &freq{}

	ret.hertz, err = conv.Float64(args, 0, "freq", "frequency(float64)")
	if err != nil {
		return nil, err
	}

	ret.amplitude, err = conv.Float64(args, 1, "freq", "amplitude(float64)")
	if err != nil {
		return nil, err
	}

	if len(args) >= 3 {
		ret.bias, err = conv.Float64(args, 2, "freq", "bias(float64)")
		if err != nil {
			return nil, err
		}
	}

	if len(args) >= 4 {
		ret.bias, err = conv.Float64(args, 3, "freq", "phase(float64)")
		if err != nil {
			return nil, err
		}
	}
	return ret, nil
}

type timeRange struct {
	tsTime   time.Time
	duration time.Duration
	period   time.Duration
}

func srcf_range(args ...any) (any, error) {
	if len(args) != 2 && len(args) != 3 {
		return nil, fmt.Errorf("f(range) invalid number of args (n:%d)", len(args))
	}
	ret := &timeRange{}
	if str, ok := args[0].(string); ok {
		if str != "now" {
			return nil, fmt.Errorf("f(range) 1st arg should be time or 'now', but %T", args[0])
		}
		ret.tsTime = time.Now()
	} else {
		if num, ok := args[0].(float64); ok {
			ret.tsTime = time.Unix(0, int64(num))
		} else {
			if ts, ok := args[0].(time.Time); ok {
				ret.tsTime = ts
			} else {
				return nil, fmt.Errorf("f(range) 1st arg should be time or 'now', but %T", args[0])
			}
		}
	}
	if str, ok := args[1].(string); ok {
		if d, err := time.ParseDuration(str); err == nil {
			ret.duration = d
		} else {
			return nil, fmt.Errorf("f(range) 2nd arg should be duration, %s", err.Error())
		}
	} else if d, ok := args[1].(float64); ok {
		ret.duration = time.Duration(int64(d))
	} else {
		return nil, fmt.Errorf("f(range) 2nd arg should be duration, but %T", args[1])
	}
	if len(args) == 2 {
		return ret, nil
	}

	if str, ok := args[2].(string); ok {
		if d, err := time.ParseDuration(str); err == nil {
			ret.period = d
		} else {
			return nil, fmt.Errorf("f(range) 3rd arg should be duration, %s", err.Error())
		}
	} else if d, ok := args[2].(float64); ok {
		ret.period = time.Duration(int64(d))
	} else {
		return nil, fmt.Errorf("f(range) 3rd arg should be duration, but %T", args[1])
	}
	if ret.duration <= ret.period {
		return nil, fmt.Errorf("f(range) 3rd arg should be smaller than 2nd")
	}

	return ret, nil
}
