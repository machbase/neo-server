package tql

import (
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	spi "github.com/machbase/neo-spi"
)

/*
Example)

	 INPUT(
		FAKE( oscillator() | meshgrid() | linspace() )
*/
func (x *Node) fmFake(origin any) (any, error) {
	switch gen := origin.(type) {
	case DataSource:
		return gen, nil
	case [][][]float64:
		return &meshgrid{task: x.task, vals: gen}, nil
	case []float64:
		return &linspace{task: x.task, vals: gen}, nil
	default:
		return nil, ErrWrongTypeOfArgs("FAKE", 0, "fakeSource", origin)
	}
}

type meshgrid struct {
	task *Task
	vals [][][]float64

	alive     bool
	closeWait sync.WaitGroup
}

func (mg *meshgrid) Gen() <-chan *Record {
	ch := make(chan *Record)
	mg.alive = true
	mg.closeWait.Add(1)
	go func() {
		mg.task.SetResultColumns([]*spi.Column{
			{Name: "id", Type: "int"},
			{Name: "x", Type: "double"},
			{Name: "y", Type: "double"},
		})
		id := 0
		for x := range mg.vals {
			for y := range mg.vals[x] {
				if !mg.alive {
					goto done
				}
				elm := mg.vals[x][y]
				if len(elm) == 2 {
					id++
					ch <- NewRecord(id, []any{elm[0], elm[1]})
				}
			}
		}
	done:
		close(ch)
		mg.closeWait.Done()
	}()
	return ch
}

func (mg *meshgrid) stop() {
	mg.alive = false
	mg.closeWait.Wait()
}

type linspace struct {
	task *Task
	vals []float64

	ch        chan *Record
	alive     bool
	closeWait sync.WaitGroup
}

func (ls *linspace) Gen() <-chan *Record {
	ls.ch = make(chan *Record)
	ls.alive = true
	ls.closeWait.Add(1)
	go func() {
		ls.task.SetResultColumns([]*spi.Column{
			{Name: "id", Type: "int"},
			{Name: "x", Type: "double"},
		})
		id := 0
		for _, v := range ls.vals {
			if !ls.alive {
				goto done
			}
			id++
			ls.ch <- NewRecord(id, []any{v})
		}
	done:
		close(ls.ch)
		ls.closeWait.Done()
	}()
	return ls.ch
}

func (ls *linspace) stop() {
	ls.alive = false
	for range ls.ch {
		// drain remains
	}
	ls.closeWait.Wait()
}

func (x *Node) fmSphere() *sphere {
	return &sphere{
		task:    x.task,
		latStep: 36,
		lonStep: 18,
	}
}

type sphere struct {
	task    *Task
	latStep float64
	lonStep float64

	alive     bool
	closeWait sync.WaitGroup
}

func (sp *sphere) Gen() <-chan *Record {
	ch := make(chan *Record)
	sp.alive = true
	sp.closeWait.Add(1)
	go func() {
		sp.task.SetResultColumns([]*spi.Column{
			{Name: "x", Type: "double"},
			{Name: "y", Type: "double"},
			{Name: "z", Type: "double"},
		})
		var u, v float64
		for u = 0; sp.alive && u < 2.0*math.Pi; u += (2.0 * math.Pi) / sp.latStep {
			for v = 0; sp.alive && v < math.Pi; v += math.Pi / sp.lonStep {
				x := math.Cos(u) * math.Sin(v)
				y := math.Sin(u) * math.Sin(v)
				z := math.Cos(v)
				ch <- NewRecord(x, []any{y, z})
			}
		}
		close(ch)
		sp.closeWait.Done()
	}()
	return ch
}

func (sp *sphere) stop() {
	sp.alive = false
	sp.closeWait.Wait()
}

// // oscillator(
// //		range(time('now','-10s'), '10s', '1ms'),
// //		freq(100, amplitude [,phase [, bias]]),
// //		freq(240, amplitude [,phase [, bias]]),
// //	)
// // )
func (x *Node) fmOscillator(args ...any) (any, error) {
	ret := &oscillator{task: x.task}
	for _, arg := range args {
		switch v := arg.(type) {
		default:
			return nil, fmt.Errorf("f(oscillator) invalid arg type '%T'", v)
		case *freq:
			ret.frequencies = append(ret.frequencies, v)
		case *TimeRange:
			if ret.timeRange != nil {
				return nil, fmt.Errorf("f(oscillator) duplicated time range, %v", v)
			}
			ret.timeRange = v
		}
	}
	if ret.timeRange == nil {
		return nil, errors.New("f(oscillator) no time range is defined")
	}
	if ret.timeRange.Period <= 0 {
		return nil, errors.New("f(oscillator) period should be positive")
	}
	return ret, nil
}

type oscillator struct {
	task        *Task
	timeRange   *TimeRange
	frequencies []*freq
	ch          chan *Record
	alive       bool
	closeWait   sync.WaitGroup
}

func (fs *oscillator) Gen() <-chan *Record {
	fs.ch = make(chan *Record)
	fs.alive = true
	fs.closeWait.Add(1)
	go func() {
		var from int64
		var to int64
		var step int64 = int64(fs.timeRange.Period)
		if fs.timeRange.Duration < 0 {
			from = fs.timeRange.Time.Add(fs.timeRange.Duration).UnixNano()
			to = fs.timeRange.Time.UnixNano()
		} else {
			from = fs.timeRange.Time.UnixNano()
			to = fs.timeRange.Time.Add(fs.timeRange.Duration).UnixNano()
		}

		fs.task.SetResultColumns([]*spi.Column{
			{Name: "time", Type: "datetime"},
			{Name: "value", Type: "double"},
		})
		for x := from; fs.alive && x < to; x += step {
			value := 0.0
			for _, fr := range fs.frequencies {
				value += fr.Value(float64(x) / float64(time.Second))
			}
			fs.ch <- NewRecord(time.Unix(0, x), []any{value})
		}
		close(fs.ch)
		fs.closeWait.Done()
	}()
	return fs.ch
}

func (fs *oscillator) stop() {
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
func (x *Node) fmFreq(frequency float64, amplitude float64, args ...float64) *freq {
	ret := &freq{
		hertz:     frequency,
		amplitude: amplitude,
	}
	if len(args) > 0 {
		ret.bias = args[0]
	}
	if len(args) > 1 {
		ret.phase = args[1]
	}
	return ret
}
