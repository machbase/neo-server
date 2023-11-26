package tql

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/machbase/neo-server/mods/nums"
	spi "github.com/machbase/neo-spi"
)

/*
Example)

	FAKE( oscillator() | meshgrid() | linspace() | json() )
*/
func (node *Node) fmFake(origin any) (any, error) {
	switch gen := origin.(type) {
	case *linspace:
		genLinspace(node, gen)
	case *meshgrid:
		genMeshgrid(node, gen)
	case *sphere:
		genSphere(node, gen)
	case *oscillator:
		genOscillator(node, gen)
	case *jsondata:
		genJsonData(node, gen)
	default:
		return nil, ErrWrongTypeOfArgs("FAKE", 0, "fakeSource", origin)
	}
	return nil, nil
}

func (node *Node) fmJsonData(data string) (*jsondata, error) {
	nodeName := node.Name()
	if nodeName == "FAKE()" {
		return &jsondata{
			content: data,
		}, nil
	}
	return nil, fmt.Errorf("FUNCTION %q doesn't support json()", nodeName)
}

type jsondata struct {
	content string  `json:"-"`
	Data    [][]any `json:"data"`
}

// the content format should
func genJsonData(node *Node, jd *jsondata) {
	// it makes {"data":[ [a1,a2],[b1,b2] ]}
	// fromjson({ [a1,a2],[b1,b2] })
	content := `{"data":[` + jd.content + `]}`
	if err := json.Unmarshal([]byte(content), jd); err != nil {
		ErrorRecord(err).Tell(node.next)
		return
	}
	for i, v := range jd.Data {
		rec := NewRecord(i+1, v)
		rec.Tell(node.next)
	}
}

func (node *Node) fmLinspace(start float64, stop float64, num int) *linspace {
	return &linspace{start: start, stop: stop, num: num}
}

func (node *Node) fmLinspace50(start float64, stop float64) *linspace {
	return &linspace{start: start, stop: stop, num: 50}
}

type linspace struct {
	start float64
	stop  float64
	num   int
}

func genLinspace(node *Node, ls *linspace) {
	node.task.SetResultColumns([]*spi.Column{
		{Name: "ROWNUM", Type: "int"},
		{Name: "x", Type: "double"},
	})
	vals := nums.Linspace(ls.start, ls.stop, ls.num)
	for i, v := range vals {
		rec := NewRecord(i+1, []any{v})
		rec.Tell(node.next)
	}
}

func (node *Node) fmMeshgrid(x any, y any) *meshgrid {
	return &meshgrid{x: x, y: y}
}

type meshgrid struct {
	x any
	y any
}

func genMeshgrid(node *Node, ms *meshgrid) {
	var xv []float64
	var yv []float64
	switch v := ms.x.(type) {
	case *linspace:
		xv = nums.Linspace(v.start, v.stop, v.num)
	case []float64:
		xv = v
	}
	switch v := ms.y.(type) {
	case *linspace:
		yv = nums.Linspace(v.start, v.stop, v.num)
	case []float64:
		yv = v
	}
	vals := nums.Meshgrid(xv, yv)

	node.task.SetResultColumns([]*spi.Column{
		{Name: "ROWNUM", Type: "int"},
		{Name: "x", Type: "double"},
		{Name: "y", Type: "double"},
	})
	id := 0
	for x := range vals {
		for y := range vals[x] {
			elm := vals[x][y]
			if len(elm) == 2 {
				id++
				NewRecord(id, []any{elm[0], elm[1]}).Tell(node.next)
			}
		}
	}
}

func (node *Node) fmSphere(lonStep float64, latStep float64) *sphere {
	if lonStep == 0 {
		lonStep = 18
	}
	if latStep == 0 {
		latStep = 36
	}
	return &sphere{lonStep: lonStep, latStep: latStep}
}

type sphere struct {
	lonStep float64
	latStep float64
}

func genSphere(node *Node, sp *sphere) {
	node.task.SetResultColumns([]*spi.Column{
		{Name: "ROWNUM", Type: "int"},
		{Name: "x", Type: "double"},
		{Name: "y", Type: "double"},
		{Name: "z", Type: "double"},
	})
	var u, v float64
	var id = 0
	for u = 0; u < 2.0*math.Pi; u += (2.0 * math.Pi) / sp.latStep {
		for v = 0; v < math.Pi; v += math.Pi / sp.lonStep {
			x := math.Cos(u) * math.Sin(v)
			y := math.Sin(u) * math.Sin(v)
			z := math.Cos(v)
			id++
			NewRecord(id, []any{x, y, z}).Tell(node.next)
		}
	}
}

// // oscillator(
// //		range(time('now','-10s'), '10s', '1ms'),
// //		freq(100, amplitude [,phase [, bias]]),
// //		freq(240, amplitude [,phase [, bias]]),
// //	)
// // )
func (node *Node) fmOscillator(args ...any) (*oscillator, error) {
	ret := &oscillator{}
	var timeRange *TimeRange
	for _, arg := range args {
		switch v := arg.(type) {
		default:
			return nil, fmt.Errorf("f(oscillator) invalid arg type '%T'", v)
		case *freq:
			ret.frequencies = append(ret.frequencies, v)
		case *TimeRange:
			if timeRange != nil {
				return nil, fmt.Errorf("f(oscillator) duplicated time range")
			}
			timeRange = v
		}
	}
	if timeRange == nil {
		return nil, errors.New("f(oscillator) no time range is defined")
	}
	if timeRange.Period <= 0 {
		return nil, errors.New("f(oscillator) period should be positive")
	}
	if timeRange.Duration < 0 {
		ret.from = timeRange.Time.Add(timeRange.Duration).UnixNano()
		ret.to = timeRange.Time.UnixNano()
	} else {
		ret.from = timeRange.Time.UnixNano()
		ret.to = timeRange.Time.Add(timeRange.Duration).UnixNano()
	}
	ret.step = int64(timeRange.Period)
	return ret, nil
}

type oscillator struct {
	frequencies []*freq
	from        int64
	to          int64
	step        int64
}

func genOscillator(node *Node, gen *oscillator) {
	node.task.SetResultColumns([]*spi.Column{
		{Name: "ROWNUM", Type: "int"},
		{Name: "time", Type: "datetime"},
		{Name: "value", Type: "double"},
	})
	rownum := 0
	for x := gen.from; x < gen.to; x += gen.step {
		value := 0.0
		for _, fr := range gen.frequencies {
			value += fr.Value(float64(x) / float64(time.Second))
		}
		rownum++
		NewRecord(rownum, []any{time.Unix(0, x), value}).Tell(node.next)
	}
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
