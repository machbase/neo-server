package tql

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"expvar"
	"fmt"
	"io"
	"math"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/machbase/neo-server/v8/api"
	"github.com/machbase/neo-server/v8/mods/nums"
	"github.com/machbase/neo-server/v8/mods/nums/opensimplex"
	"github.com/machbase/neo-server/v8/mods/util/glob"
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
	case *arrange:
		genArrange(node, gen)
	case *doOnce:
		genOnce(node, gen)
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
	case *statz:
		genStatz(node, gen)
	default:
		return nil, ErrWrongTypeOfArgs("FAKE", 0, "fakeSource", origin)
	}
	return nil, nil
}

type statz struct {
	interval   time.Duration
	keyFilters []string
}

func genStatz(node *Node, gen *statz) {
	statz := api.QueryStatz(gen.interval, func(kv expvar.KeyValue) bool {
		if len(gen.keyFilters) == 0 {
			return true
		} else {
			for _, filter := range gen.keyFilters {
				if glob.IsGlob(filter) {
					if ok, _ := glob.Match(filter, kv.Key); ok {
						return true
					}
				} else {
					if filter == kv.Key {
						return true
					}
				}
			}
		}
		return false
	})
	if statz.Err != nil {
		ErrorRecord(statz.Err).Tell(node.next)
		return
	}
	// set columns
	cols := []*api.Column{
		api.MakeColumnRownum(),
		api.MakeColumnDatetime("time"),
	}
	cols = append(cols, statz.Cols...)
	node.task.SetResultColumns(cols)
	if len(statz.Rows) == 0 {
		return
	}
	// yield rows
	for i, row := range statz.Rows {
		NewRecord(i, append([]any{row.Timestamp}, row.Values...)).Tell(node.next)
	}
}

func (node *Node) fmStatz(samplingInterval string, keyFilters ...string) *statz {
	var interval = api.MetricShortTerm
	switch strings.ToLower(samplingInterval) {
	case "short":
		interval = api.MetricShortTerm
	case "mid":
		interval = api.MetricMidTerm
	case "long":
		interval = api.MetricLongTerm
	default:
		if dur, err := time.ParseDuration(samplingInterval); err == nil {
			interval = dur
		}
	}

	ret := &statz{
		interval:   interval,
		keyFilters: keyFilters,
	}
	return ret
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
			cols := []*api.Column{api.MakeColumnRownum()}
			for i := 0; i < len(values); i++ {
				cname := fmt.Sprintf("column%d", i)
				cols = append(cols, api.MakeColumnString(cname))
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
		cols := []*api.Column{api.MakeColumnRownum()}
		for i := 0; i < colCount; i++ {
			cname := fmt.Sprintf("column%d", i)
			switch jd.Data[0][i].(type) {
			case string:
				cols = append(cols, api.MakeColumnString(cname))
			case float64:
				cols = append(cols, api.MakeColumnDouble(cname))
			case bool:
				cols = append(cols, api.MakeColumnBoolean(cname))
			default:
				cols = append(cols, api.MakeColumnAny(cname))
			}
		}
		node.task.SetResultColumns(cols)
	}
	for i, v := range jd.Data {
		rec := NewRecord(i+1, v)
		rec.Tell(node.next)
	}
}

func (node *Node) fmOnce(v float64) (*doOnce, error) {
	return &doOnce{value: v}, nil
}

type doOnce struct {
	value float64
}

func genOnce(node *Node, gen *doOnce) {
	node.task.SetResultColumns([]*api.Column{
		api.MakeColumnRownum(),
		api.MakeColumnDouble("x"),
	})
	NewRecord(1, []any{gen.value}).Tell(node.next)
}

func (node *Node) fmArrange(start float64, stop float64, step float64) (*arrange, error) {
	if step == 0 {
		return nil, fmt.Errorf("FUNCTION %q step can not be 0", "arrange")
	}
	if start == stop {
		return nil, fmt.Errorf("FUNCTION %q start, stop can not be equal", "arrange")
	}
	if start <= stop && step < 0 {
		return nil, fmt.Errorf("FUNCTION %q step can not be less than 0", "arrange")
	}
	if start > stop && step > 0 {
		return nil, fmt.Errorf("FUNCTION %q step can not be greater than 0", "arrange")
	}
	return &arrange{start: start, stop: stop, step: step}, nil
}

type arrange struct {
	start float64
	stop  float64
	step  float64
}

func genArrange(node *Node, ar *arrange) {
	node.task.SetResultColumns([]*api.Column{
		api.MakeColumnRownum(),
		api.MakeColumnDouble("x"),
	})
	i := 0
	if ar.start < ar.stop {
		for v := ar.start; v <= ar.stop; v += ar.step {
			rec := NewRecord(i+1, []any{v})
			rec.Tell(node.next)
			i++
		}
	} else {
		for v := ar.start; v >= ar.stop; v += ar.step {
			rec := NewRecord(i+1, []any{v})
			rec.Tell(node.next)
			i++
		}
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
	node.task.SetResultColumns([]*api.Column{
		api.MakeColumnRownum(),
		api.MakeColumnDouble("x"),
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
	case *arrange:
		xv = nums.Arrange(v.start, v.stop, v.step)
	case []float64:
		xv = v
	}
	switch v := ms.y.(type) {
	case *linspace:
		yv = nums.Linspace(v.start, v.stop, v.num)
	case *arrange:
		yv = nums.Arrange(v.start, v.stop, v.step)
	case []float64:
		yv = v
	}
	vals := nums.Meshgrid(xv, yv)

	node.task.SetResultColumns([]*api.Column{
		api.MakeColumnRownum(),
		api.MakeColumnDouble("x"),
		api.MakeColumnDouble("y"),
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
	node.task.SetResultColumns([]*api.Column{
		api.MakeColumnRownum(),
		api.MakeColumnDouble("x"),
		api.MakeColumnDouble("y"),
		api.MakeColumnDouble("z"),
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
	node.task.SetResultColumns([]*api.Column{
		api.MakeColumnRownum(),
		{Name: "time", DataType: api.DataTypeDatetime},
		{Name: "value", DataType: api.DataTypeFloat64},
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

func (node *Node) fmSimplex(seed int64, x float64, args ...float64) (float64, error) {
	var gen *opensimplex.Generator
	if v, ok := node.GetValue("simplex_generator"); ok {
		gen = v.(*opensimplex.Generator)
	} else {
		gen = opensimplex.New(seed)
		node.SetValue("simplex_generator", gen)
	}
	dim := []float64{x}
	for i := 0; i < 4 && i < len(args); i++ {
		dim = append(dim, args[i])
	}
	return gen.Eval(dim...), nil
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

func (node *Node) fmStrIndex(str string, substr string) int {
	return strings.Index(str, substr)
}

func (node *Node) fmStrLastIndex(str string, substr string) int {
	return strings.LastIndex(str, substr)
}
