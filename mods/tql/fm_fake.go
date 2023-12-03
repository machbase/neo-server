package tql

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"
	"strconv"
	"strings"
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
	case *csvdata:
		genCsvData(node, gen)
	case *rawdata:
		genRawData(node, gen)
	default:
		return nil, ErrWrongTypeOfArgs("FAKE", 0, "fakeSource", origin)
	}
	return nil, nil
}

type rawdata struct {
	data any
}

func genRawData(node *Node, gen *rawdata) {
	rec := NewRecord(1, gen.data)
	rec.Tell(node.next)
}

func (node *Node) fmCsvData(data string) (*csvdata, error) {
	nodeName := node.Name()
	if nodeName == "FAKE()" {
		return &csvdata{
			content: data,
		}, nil
	}
	return nil, fmt.Errorf("FUNCTION %q doesn't support csv()", nodeName)
}

type csvdata struct {
	content string
}

func genCsvData(node *Node, cd *csvdata) {
	r := csv.NewReader(bytes.NewBufferString(string(cd.content)))
	for i := 0; true; i++ {
		values, err := r.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
			ErrorRecord(err).Tell(node.next)
			return
		}
		if i == 0 {
			cols := []*spi.Column{{Name: "ROWNUM", Type: "int"}}
			for i := 0; i < len(values); i++ {
				cname := fmt.Sprintf("column%d", i)
				cols = append(cols, &spi.Column{Name: cname, Type: "string"})
			}
			node.task.SetResultColumns(cols)
		}
		v := make([]any, len(values))
		for i, s := range values {
			v[i] = s
		}
		rec := NewRecord(i+1, v)
		rec.Tell(node.next)
	}
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
		ErrorRecord(fmt.Errorf("%s %s", node.Name(), err.Error())).Tell(node.next)
		return
	}
	if len(jd.Data) > 0 {
		colCount := len(jd.Data[0])
		cols := []*spi.Column{{Name: "ROWNUM", Type: "int"}}
		for i := 0; i < colCount; i++ {
			cname := fmt.Sprintf("column%d", i)
			switch jd.Data[0][i].(type) {
			case string:
				cols = append(cols, &spi.Column{Name: cname, Type: "string"})
			case float64:
				cols = append(cols, &spi.Column{Name: cname, Type: "double"})
			case bool:
				cols = append(cols, &spi.Column{Name: cname, Type: "boolean"})
			default:
				cols = append(cols, &spi.Column{Name: cname, Type: "unknown"})
			}
		}
		node.task.SetResultColumns(cols)
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

func (x *Node) fmRandom() float64 {
	return rand.Float64()
}

func (node *Node) fmParseFloat(str string) (float64, error) {
	return strconv.ParseFloat(str, 64)
}

func (node *Node) fmParseBoolean(str string) (bool, error) {
	ret, err := strconv.ParseBool(str)
	if err != nil {
		return false, fmt.Errorf("parseBool: parsing %q: invalid syntax", str)
	}
	return ret, nil
}

func (node *Node) fmStrTrimSpace(str string) string {
	return strings.TrimSpace(str)
}

func (node *Node) fmStrTrimPrefix(str string, prefix string) string {
	return strings.TrimPrefix(str, prefix)
}

func (node *Node) fmStrTrimSuffix(str string, suffix string) string {
	return strings.TrimSuffix(str, suffix)
}

func (node *Node) fmStrReplaceAll(str string, old string, new string) string {
	return strings.ReplaceAll(str, old, new)
}

func (node *Node) fmStrReplace(str string, old string, new string, n int) string {
	return strings.Replace(str, old, new, n)
}

func (node *Node) fmStrSprintf(format string, args ...any) string {
	return fmt.Sprintf(format, args...)
}

func (node *Node) fmStrHasPrefix(str string, prefix string) bool {
	return strings.HasPrefix(str, prefix)
}

func (node *Node) fmStrHasSuffix(str string, suffix string) bool {
	return strings.HasSuffix(str, suffix)
}

func (node *Node) fmStrToUpper(str string) string {
	return strings.ToUpper(str)
}

func (node *Node) fmStrToLower(str string) string {
	return strings.ToLower(str)
}

func (node *Node) fmStrSub(str string, args ...int) string {
	var offset, count = 0, -1
	if len(args) == 0 {
		return str
	}
	if len(args) > 0 {
		offset = args[0]
	}
	if len(args) > 1 {
		count = args[1]
		if count < 0 {
			count = -1
		}
	}

	rs := []rune(str)

	idx := offset
	if idx < 0 {
		if idx*-1 >= len(rs) {
			return ""
		}
		idx = len(rs) + idx
	} else {
		if offset >= len(rs) {
			return ""
		}
	}
	end := idx + count
	if end >= len(rs) {
		end = len(rs)
	}

	if count == -1 {
		return string(rs[idx:])
	} else {
		return string(rs[idx:end])
	}
}
