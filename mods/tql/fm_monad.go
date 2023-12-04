package tql

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"

	gocsv "encoding/csv"

	"github.com/machbase/neo-server/mods/util"
	"github.com/machbase/neo-server/mods/util/glob"
	spi "github.com/machbase/neo-spi"
	"gonum.org/v1/gonum/stat"
)

type lazyOption struct {
	flag bool
}

func (node *Node) fmLazy(flag bool) *lazyOption {
	return &lazyOption{flag: flag}
}

func (node *Node) fmTake(args ...int) (*Record, error) {
	limit := 0
	if n, ok := node.GetValue("limit"); !ok {
		if len(args) == 1 {
			limit = args[0]
		} else if len(args) == 2 {
			limit = args[1]
		}
		node.SetValue("limit", limit)
	} else {
		limit = n.(int)
	}
	if limit < 0 {
		return nil, ErrArgs("TAKE", 1, "limit should be larger than 0")
	}
	offset := 0
	if n, ok := node.GetValue("offset"); !ok {
		if len(args) == 2 {
			offset = args[0]
		}
		node.SetValue("offset", offset)
	} else {
		offset = n.(int)
	}
	count := 0
	if n, ok := node.GetValue("count"); ok {
		count = n.(int)
	}
	count++
	node.SetValue("count", count)

	if count > offset+limit {
		return BreakRecord, nil
	}
	if count <= offset {
		return nil, nil
	}
	return node.Inflight(), nil
}

func (node *Node) fmDrop(args ...int) (*Record, error) {
	limit := 0
	if n, ok := node.GetValue("limit"); !ok {
		if len(args) == 1 {
			limit = args[0]
		} else if len(args) == 2 {
			limit = args[1]
		}
		node.SetValue("limit", limit)
	} else {
		limit = n.(int)
	}
	if limit < 0 {
		return nil, ErrArgs("DROP", 1, "limit should be larger than 0")
	}
	offset := 0
	if n, ok := node.GetValue("offset"); !ok {
		if len(args) == 2 {
			offset = args[0]
		}
		node.SetValue("offset", offset)
	} else {
		offset = n.(int)
	}
	count := 0
	if n, ok := node.GetValue("count"); ok {
		count = n.(int)
	}
	count++
	node.SetValue("count", count)

	if count > offset && count <= offset+limit {
		return nil, nil
	}
	return node.Inflight(), nil
}

func (node *Node) fmFilter(flag bool) *Record {
	if !flag {
		return nil // drop this vector
	}
	return node.Inflight()
}

func (node *Node) fmFlatten() any {
	rec := node.Inflight()
	if rec.IsArray() {
		ret := []*Record{}
		for _, r := range rec.Array() {
			k := r.Key()
			switch value := r.Value().(type) {
			case []any:
				for _, v := range value {
					if v == nil {
						continue
					}
					ret = append(ret, NewRecord(k, v))
				}
			case any:
				ret = append(ret, r)
			default:
				ret = append(ret, ErrorRecord(fmt.Errorf("fmtFlatten() unknown type '%T' in array record", value)))
			}
		}
		return ret
	} else if rec.IsTuple() {
		switch value := rec.Value().(type) {
		case [][]any:
			k := rec.Key()
			ret := []*Record{}
			for _, v := range value {
				if len(v) == 0 {
					continue
				}
				ret = append(ret, NewRecord(k, v))
			}
			return ret
		case []any:
			k := rec.Key()
			ret := []*Record{}
			for _, v := range value {
				if v == nil {
					continue
				}
				ret = append(ret, NewRecord(k, v))
			}
			return ret
		case any:
			return rec
		default:
			return ErrorRecord(fmt.Errorf("fmtFlatten() unknown type '%T' in array record", value))
		}
	} else {
		return rec
	}
}

func (node *Node) fmGroup(args ...any) any {
	var gr *Group
	var columns []*GroupAggregate
	var by *GroupAggregate
	var shouldSetColumns bool

	if obj, ok := node.GetValue("group"); ok {
		gr = obj.(*Group)
	} else {
		gr = &Group{
			buffer:    map[any][]GroupWindowColumn{},
			chunkMode: true,
		}
		node.SetValue("group", gr)
		node.SetEOF(gr.onEOF)
		shouldSetColumns = true
		for _, arg := range args {
			switch arg.(type) {
			case *GroupAggregate:
				gr.chunkMode = false
			}
			// if has at least one aggregate, chunk mode is off
			if !gr.chunkMode {
				break
			}
		}
	}

	for _, arg := range args {
		switch v := arg.(type) {
		case *GroupAggregate:
			columns = append(columns, v)
			if v.Type == "by" {
				by = v
			}
		case *lazyOption:
			gr.lazy = v.flag
		default:
			return ErrorRecord(fmt.Errorf("GROUP() unknown type '%T' in arguments", v))
		}
	}
	if by == nil {
		return ErrorRecord(fmt.Errorf("GROUP() has no by() argument"))
	}
	if by.Value == nil {
		return ErrorRecord(fmt.Errorf("GROUP() by() can not be NULL"))
	}
	if shouldSetColumns {
		if !gr.chunkMode {
			cols := make([]*spi.Column, len(columns)+1)
			cols[0] = &spi.Column{Name: "ROWNUM", Type: "int"}
			for i, c := range columns {
				cols[i+1] = &spi.Column{
					Name: c.Name,
					Type: c.ColumnType(),
				}
			}
			node.task.SetResultColumns(cols)
		}
	}
	if gr.chunkMode {
		gr.pushChunk(node, by)
	} else {
		gr.push(node, by, columns)
	}
	return nil
}

type Group struct {
	lazy      bool
	buffer    map[any][]GroupWindowColumn
	curKey    any
	chunkMode bool
}

func (gr *Group) onEOF(node *Node) {
	if gr.chunkMode {
		for k, cols := range gr.buffer {
			r := cols[0].Result()
			if v, ok := r.([]any); ok {
				node.yield(k, v)
			} else {
				node.yield(k, []any{r})
			}
		}
		gr.buffer = nil
	} else {
		for k, cols := range gr.buffer {
			v := make([]any, len(cols))
			for i, c := range cols {
				v[i] = c.Result()
			}
			node.yield(k, v)
		}
		gr.buffer = nil
	}
}

func (gr *Group) pushChunk(node *Node, by *GroupAggregate) {
	var chunk *GroupWindowColumnChunk
	if cs, ok := gr.buffer[by.Value]; ok {
		chunk = cs[0].(*GroupWindowColumnChunk)
	} else {
		chunk = &GroupWindowColumnChunk{name: "chunk"}
		gr.buffer[by.Value] = []GroupWindowColumn{chunk}
	}
	chunk.Append(node.Inflight().Value())
	if !gr.lazy && gr.curKey != nil && gr.curKey != by.Value {
		if ret, ok := gr.buffer[gr.curKey]; ok {
			r := ret[0].Result()
			if v, ok := r.([]any); ok {
				node.yield(gr.curKey, v)
			} else {
				node.yield(gr.curKey, []any{r})
			}
			delete(gr.buffer, gr.curKey)
		}
	}
	gr.curKey = by.Value

}

func (gr *Group) push(node *Node, by *GroupAggregate, columns []*GroupAggregate) {
	var cols []GroupWindowColumn
	if cs, ok := gr.buffer[by.Value]; ok {
		cols = cs
	} else {
		for _, c := range columns {
			if buff := c.NewBuffer(); buff != nil {
				cols = append(cols, buff)
			} else {
				node.task.LogErrorf("%s, invalid aggregate %q", node.Name(), c.Type)
				return
			}
		}
		gr.buffer[by.Value] = cols
	}

	for i, c := range columns {
		cols[i].Append(c.Value)
	}

	if !gr.lazy && gr.curKey != nil && gr.curKey != by.Value {
		if cols, ok := gr.buffer[gr.curKey]; ok {
			v := make([]any, len(cols))
			for i, c := range cols {
				v[i] = c.Result()
			}
			node.yield(gr.curKey, v)
			delete(gr.buffer, gr.curKey)
		}
	}
	gr.curKey = by.Value
}

func (node *Node) fmBy(value any, args ...string) any {
	ret := &GroupAggregate{Type: "by"}
	ret.Value = unboxValue(value)
	if len(args) > 0 {
		ret.Name = args[0]
	} else {
		ret.Name = "GROUP"
	}
	return ret
}

type GroupAggregate struct {
	Type  string
	Value any
	Name  string
}

func newAggregate(typ string, value any, args ...string) *GroupAggregate {
	ret := &GroupAggregate{Type: typ, Value: value}
	if len(args) > 0 {
		ret.Name = args[0]
	} else {
		ret.Name = strings.ToUpper(typ)
	}
	return ret
}

func (ga *GroupAggregate) ColumnType() string {
	switch ga.Type {
	case "by":
		if ga.Value == nil {
			return "string"
		}
		switch ga.Value.(type) {
		case time.Time:
			return "time"
		case string:
			return "string"
		case float64:
			return "float64"
		default:
			return "interface{}"
		}
	case "chunk":
		return "array"
	}
	return "float64"
}

func (ga *GroupAggregate) NewBuffer() GroupWindowColumn {
	switch ga.Type {
	case "by":
		return &GroupWindowColumnConst{value: ga.Value}
	case "chunk":
		return &GroupWindowColumnChunk{name: ga.Type}
	case "mean", "median", "median-interpolated", "stddev", "stderr", "entropy", "mode":
		return &GroupWindowColumnContainer{name: ga.Type}
	case "first", "last", "min", "max", "sum":
		return &GroupWindowColumnSingle{name: ga.Type}
	case "avg", "rss", "rms":
		return &GroupWindowColumnCounter{name: ga.Type}
	default:
		return nil
	}
}

func (node *Node) fmFirst(value float64, args ...string) any {
	return newAggregate("first", value, args...)
}

func (node *Node) fmLast(value float64, args ...string) any {
	return newAggregate("last", value, args...)
}

func (node *Node) fmMin(value float64, other ...any) (any, error) {
	if node.Name() == "GROUP()" {
		if len(other) > 0 {
			var name string
			if str, ok := other[0].(string); ok {
				name = str
			} else {
				name = fmt.Sprintf("%v", other[0])
			}
			return newAggregate("min", name), nil
		} else {
			return newAggregate("min", value), nil
		}
	} else { // math.Min
		if len(other) == 1 {
			rv, err := util.ToFloat64(other[0])
			if err != nil {
				return value, nil
			}
			return math.Min(value, rv), nil
		} else {
			return value, fmt.Errorf("min() requires two float64 values, got %d", len(other)+1)
		}
	}
}

func (node *Node) fmMax(value float64, other ...any) (any, error) {
	if node.Name() == "GROUP()" {
		if len(other) > 0 {
			var name string
			if str, ok := other[0].(string); ok {
				name = str
			} else {
				name = fmt.Sprintf("%v", other[0])
			}
			return newAggregate("max", name), nil
		} else {
			return newAggregate("max", value), nil
		}
	} else { // math.Max
		if len(other) == 1 {
			rv, err := util.ToFloat64(other[0])
			if err != nil {
				return value, nil
			}
			return math.Max(value, rv), nil
		} else {
			return value, fmt.Errorf("max() requires two float64 values, got %d", len(other)+1)
		}
	}
}

func (node *Node) fmSum(value float64, args ...string) any {
	return newAggregate("sum", value, args...)
}

func (node *Node) fmMean(value float64, args ...string) any {
	return newAggregate("mean", value, args...)
}

func (node *Node) fmMedian(value float64, args ...string) any {
	return newAggregate("median", value, args...)
}

func (node *Node) fmMedianInterpolated(value float64, args ...string) any {
	return newAggregate("median-interpolated", value, args...)
}

func (node *Node) fmStdDev(value float64, args ...string) any {
	return newAggregate("stddev", value, args...)
}

func (node *Node) fmStdErr(value float64, args ...string) any {
	return newAggregate("stderr", value, args...)
}

func (node *Node) fmEntropy(value float64, args ...string) any {
	return newAggregate("entropy", value, args...)
}

func (node *Node) fmMode(value float64, args ...string) any {
	return newAggregate("mode", value, args...)
}

func (node *Node) fmAvg(value float64, args ...string) any {
	return newAggregate("avg", value, args...)
}
func (node *Node) fmRSS(value float64, args ...string) any {
	return newAggregate("rss", value, args...)
}
func (node *Node) fmRMS(value float64, args ...string) any {
	return newAggregate("rms", value, args...)
}

func (node *Node) fmGroupByKey(args ...any) any {
	var gw *GroupWindow
	if obj, ok := node.GetValue("groupbykey"); ok {
		gw = obj.(*GroupWindow)
	} else {
		gw = NewGroupWindow()
		node.SetValue("groupbykey", gw)
		node.SetEOF(gw.onEOF)

		gw.columns = []string{}
		if len(args) > 0 {
			for _, arg := range args {
				switch v := arg.(type) {
				case string:
					gw.columns = append(gw.columns, v)
				case *lazyOption:
					gw.lazy = v.flag
				}
			}
		}
		if len(gw.columns) == 0 {
			gw.columns = append(gw.columns, "chunk")
		}
	}
	gw.push(node)
	return nil
}

func NewGroupWindow() *GroupWindow {
	ret := &GroupWindow{}
	return ret
}

type GroupWindow struct {
	lazy    bool
	columns []string
	buffer  map[any][]GroupWindowColumn
	curKey  any
}

func (gw *GroupWindow) newColumn() ([]GroupWindowColumn, error) {
	ret := make([]GroupWindowColumn, len(gw.columns))
	for i, typ := range gw.columns {
		switch typ {
		case "chunk":
			ret[i] = &GroupWindowColumnChunk{name: typ}
		case "mean", "median", "median-interpolated", "stddev", "stderr", "entropy", "mode":
			ret[i] = &GroupWindowColumnContainer{name: typ}
		case "first", "last", "min", "max", "sum":
			ret[i] = &GroupWindowColumnSingle{name: typ}
		case "avg", "rss", "rms":
			ret[i] = &GroupWindowColumnCounter{name: typ}
		default:
			return nil, fmt.Errorf("GROUPBYKEY unknown aggregator %q", typ)
		}
	}
	return ret, nil
}

func (gw *GroupWindow) onEOF(node *Node) {
	for k, cols := range gw.buffer {
		if gw.isChunkMode() {
			r := cols[0].Result()
			if v, ok := r.([]any); ok {
				node.yield(k, v)
			} else {
				node.yield(k, []any{r})
			}
		} else {
			v := make([]any, len(cols))
			for i, c := range cols {
				v[i] = c.Result()
			}
			node.yield(k, v)
		}
	}
	gw.buffer = nil
}

func (gw *GroupWindow) isChunkMode() bool {
	return len(gw.columns) == 1 && gw.columns[0] == "chunk"
}

func (gw *GroupWindow) push(node *Node) {
	key := node.Inflight().key
	value := node.Inflight().value
	var cols []GroupWindowColumn

	if gw.buffer == nil {
		gw.buffer = map[any][]GroupWindowColumn{}
	}
	if cs, ok := gw.buffer[key]; ok {
		cols = cs
	} else {
		if newCols, err := gw.newColumn(); err != nil {
			node.task.LogErrorf("%s, %s", node.Name(), err.Error())
			return
		} else {
			cols = newCols
		}
		gw.buffer[key] = cols
	}

	if gw.isChunkMode() {
		cols[0].Append(value)
	} else {
		for i := range cols {
			if v, ok := value.([]any); ok {
				if i < len(v) {
					cols[i].Append(v[i])
				} else {
					cols[i].Append(nil)
				}
			} else if i == 0 {
				cols[i].Append(value)
			} else {
				cols[i].Append(nil)
			}
		}
	}

	if !gw.lazy && gw.curKey != nil && gw.curKey != key {
		if cols, ok := gw.buffer[gw.curKey]; ok {
			if gw.isChunkMode() {
				r := cols[0].Result()
				if v, ok := r.([]any); ok {
					node.yield(gw.curKey, v)
				} else {
					node.yield(gw.curKey, []any{r})
				}
			} else {
				v := make([]any, len(cols))
				for i, c := range cols {
					v[i] = c.Result()
				}
				node.yield(gw.curKey, v)
			}
			delete(gw.buffer, gw.curKey)
		}
	}
	gw.curKey = key
}

type GroupWindowColumn interface {
	Append(any) error
	Result() any
}

var (
	_ GroupWindowColumn = &GroupWindowColumnSingle{}
	_ GroupWindowColumn = &GroupWindowColumnContainer{}
	_ GroupWindowColumn = &GroupWindowColumnCounter{}
	_ GroupWindowColumn = &GroupWindowColumnChunk{}
	_ GroupWindowColumn = &GroupWindowColumnConst{}
)

// chunk
type GroupWindowColumnChunk struct {
	name   string
	values []any
}

func (gc *GroupWindowColumnChunk) Result() any {
	ret := gc.values
	gc.values = []any{}
	return ret
}

func (gc *GroupWindowColumnChunk) Append(v any) error {
	gc.values = append(gc.values, v)
	return nil
}

// const
type GroupWindowColumnConst struct {
	value any
}

func (gc *GroupWindowColumnConst) Result() any {
	return gc.value
}

func (gc *GroupWindowColumnConst) Append(v any) error {
	return nil
}

// "mean", "median", "median-interpolated", "stddev", "stderr", "entropy", "mode"
type GroupWindowColumnContainer struct {
	name   string
	values []float64
}

func (gc *GroupWindowColumnContainer) Result() any {
	defer func() {
		gc.values = gc.values[0:]
	}()
	var ret float64
	switch gc.name {
	case "mean":
		if len(gc.values) == 0 {
			return nil
		}
		ret, _ = stat.MeanStdDev(gc.values, nil)
	case "median":
		if len(gc.values) == 0 {
			return nil
		}
		sort.Float64s(gc.values)
		ret = stat.Quantile(0.5, stat.Empirical, gc.values, nil)
	case "median-interpolated":
		if len(gc.values) == 0 {
			return nil
		}
		sort.Float64s(gc.values)
		ret = stat.Quantile(0.5, stat.LinInterp, gc.values, nil)
	case "stddev":
		if len(gc.values) < 1 {
			return nil
		}
		_, ret = stat.MeanStdDev(gc.values, nil)
	case "stderr":
		if len(gc.values) < 1 {
			return nil
		}
		_, std := stat.MeanStdDev(gc.values, nil)
		ret = stat.StdErr(std, float64(len(gc.values)))
	case "entropy":
		if len(gc.values) == 0 {
			return nil
		}
		ret = stat.Entropy(gc.values)
	case "mode":
		if len(gc.values) == 0 {
			return nil
		}
		ret, _ = stat.Mode(gc.values, nil)
	default:
		return nil
	}
	if ret != ret { // NaN
		return nil
	}
	return ret
}

func (gc *GroupWindowColumnContainer) Append(v any) error {
	f, err := util.ToFloat64(v)
	if err != nil {
		return err
	}
	gc.values = append(gc.values, f)
	return nil
}

// avg, rss, rms
type GroupWindowColumnCounter struct {
	name  string
	value float64
	count int
}

func (gc *GroupWindowColumnCounter) Result() any {
	if gc.count == 0 {
		return nil
	}
	defer func() {
		gc.count = 0
		gc.value = 0
	}()

	var ret float64
	switch gc.name {
	case "avg":
		ret = gc.value / float64(gc.count)
	case "rss":
		ret = math.Sqrt(gc.value)
	case "rms":
		ret = math.Sqrt(gc.value / float64(gc.count))
	}
	return ret
}

func (gc *GroupWindowColumnCounter) Append(v any) error {
	f, err := util.ToFloat64(v)
	if err != nil {
		return err
	}
	switch gc.name {
	case "avg":
		gc.count++
		gc.value += f
	case "rss", "rms":
		gc.count++
		gc.value += f * f
	}
	return nil
}

// first, last, min, max, sum
type GroupWindowColumnSingle struct {
	name     string
	value    any
	hasValue bool
}

func (gc *GroupWindowColumnSingle) Result() any {
	if gc.hasValue {
		ret := gc.value
		gc.value, gc.hasValue = 0, false
		return ret
	}
	return nil
}

func (gc *GroupWindowColumnSingle) Append(v any) error {
	if gc.name == "first" {
		if gc.hasValue {
			return nil
		}
		gc.value = v
		gc.hasValue = true
		return nil
	} else if gc.name == "last" {
		gc.value = v
		gc.hasValue = true
		return nil
	}

	f, err := util.ToFloat64(v)
	if err != nil {
		return err
	}
	if !gc.hasValue {
		gc.value = f
		gc.hasValue = true
		return nil
	}

	old := gc.value.(float64)
	switch gc.name {
	case "min":
		if old > f {
			gc.value = f
		}
	case "max":
		if old < f {
			gc.value = f
		}
	case "sum":
		gc.value = old + f
	}
	return nil
}

// Drop Key, then make the first element of value to promote as a key,
// decrease dimension of vector as result if the input is not multiple dimension vector.
// `map=POPKEY(V, 0)` produces
// 1 dimension : `K: [V1, V2, V3...]` ==> `V1 : [V2, V3, .... ]`
// 2 dimension : `K: [[V11, V12, V13...],[V21, V22, V23...], ...] ==> `V11: [V12, V13...]` and `V21: [V22, V23...]` ...
func (node *Node) fmPopKey(args ...int) (any, error) {
	var nth = 0
	if len(args) > 0 {
		nth = args[0]
	}

	// V : value
	inflight := node.Inflight()
	if inflight == nil || inflight.value == nil {
		return nil, nil
	}
	switch val := inflight.value.(type) {
	default:
		return nil, fmt.Errorf("f(POPKEY) V should be []any or [][]any, but %T", val)
	case []any:
		if nth < 0 || nth >= len(val) {
			return nil, fmt.Errorf("f(POPKEY) 1st arg should be between 0 and %d, but %d", len(val)-1, nth)
		}
		if _, ok := node.GetValue("isFirst"); !ok {
			node.SetValue("isFirst", true)
			columns := node.task.ResultColumns() // it contains ROWNUM
			cols := columns
			if len(columns) > nth+1 {
				cols = []*spi.Column{columns[nth+1]}
				cols = append(cols, columns[1:nth+1]...)
			}
			if len(columns) >= nth+2 {
				cols = append(cols, columns[nth+2:]...)
			}
			node.task.SetResultColumns(cols)
		}
		newKey := val[nth]
		newVal := append(val[0:nth], val[nth+1:]...)
		ret := NewRecord(newKey, newVal)
		return ret, nil
	case [][]any:
		ret := make([]*Record, len(val))
		if _, ok := node.GetValue("isFirst"); !ok {
			node.SetValue("isFirst", true)
			columns := node.task.ResultColumns()
			if len(columns) > 1 {
				node.task.SetResultColumns(columns[1:])
			}
		}
		for i, v := range val {
			if len(v) < 2 {
				return nil, fmt.Errorf("f(POPKEY) arg elements should be larger than 2, but %d", len(v))
			}
			if len(v) == 2 {
				ret[i] = NewRecord(v[0], v[1])
			} else {
				ret[i] = NewRecord(v[0], v[1:])
			}
		}
		return ret, nil
	}
}

// Merge all incoming values into a single key,
// incresing dimension of vector as result.
// `map=PUSHKEY(NewKEY)` produces `NewKEY: [K, V...]`
func (node *Node) fmPushKey(newKey any) (any, error) {
	if _, ok := node.GetValue("isFirst"); !ok {
		node.SetValue("isFirst", true)
		node.task.SetResultColumns(append([]*spi.Column{node.AsColumnTypeOf(newKey)}, node.task.ResultColumns()...))
	}
	rec := node.Inflight()
	if rec == nil {
		return nil, nil
	}
	key, value := rec.key, rec.value
	var newVal []any
	switch val := value.(type) {
	case []any:
		newVal = append([]any{key}, val...)
	case any:
		newVal = []any{key, val}
	default:
		return nil, ErrArgs("PUSHKEY", 0, fmt.Sprintf("Value should be array, but %T", value))
	}
	return NewRecord(newKey, newVal), nil
}

func (node *Node) fmMapKey(newKey any) (any, error) {
	if _, ok := node.GetValue("isFirst"); !ok {
		node.SetValue("isFirst", true)
		cols := node.task.ResultColumns()
		if len(cols) > 0 {
			node.task.SetResultColumns(append([]*spi.Column{node.AsColumnTypeOf(newKey)}, node.task.ResultColumns()[1:]...))
		}
	}
	rec := node.Inflight()
	if rec == nil {
		return nil, nil
	}
	return NewRecord(newKey, rec.value), nil
}

func (node *Node) fmPushValue(idx int, newValue any, opts ...any) (any, error) {
	var columnName = "column"
	if len(opts) > 0 {
		if str, ok := opts[0].(string); ok {
			columnName = str
		}
	}

	inflight := node.Inflight()
	if inflight == nil {
		return nil, nil
	}

	if idx < 0 {
		idx = 0
	}
	switch val := inflight.value.(type) {
	case []any:
		if idx > len(val) {
			idx = len(val)
		}
	default:
		if idx > 0 {
			idx = 1
		}
	}

	if _, ok := node.GetValue("isFirst"); !ok {
		node.SetValue("isFirst", true)
		cols := node.task.ResultColumns() // cols contains "ROWNUM"
		if len(cols) >= idx {
			newCol := node.AsColumnTypeOf(newValue)
			newCol.Name = columnName
			head := cols[0 : idx+1]
			tail := cols[idx+1:]
			updateCols := []*spi.Column{}
			updateCols = append(updateCols, head...)
			updateCols = append(updateCols, newCol)
			updateCols = append(updateCols, tail...)
			node.task.SetResultColumns(updateCols)
		} else {
			for i := len(cols); i < idx; i++ {
				newCol := &spi.Column{}
				newCol.Name = fmt.Sprintf("column%d", i)
				cols = append(cols, newCol)
			}
			node.task.SetResultColumns(cols)
		}
	}

	switch val := inflight.value.(type) {
	case []any:
		head := val[0:idx]
		tail := val[idx:]
		updateVal := []any{}
		updateVal = append(updateVal, head...)
		updateVal = append(updateVal, newValue)
		updateVal = append(updateVal, tail...)
		return NewRecord(inflight.key, updateVal), nil
	default:
		if idx <= 0 {
			return NewRecord(inflight.key, []any{newValue, val}), nil
		} else {
			return NewRecord(inflight.key, []any{val, newValue}), nil
		}
	}
}

func (node *Node) fmPopValue(idxes ...int) (any, error) {
	inflight := node.Inflight()
	if inflight == nil || len(idxes) == 0 {
		return inflight, nil
	}

	includes := []int{}
	switch val := inflight.value.(type) {
	case []any:
		count := len(val)
		for _, idx := range idxes {
			if idx < 0 || idx >= count {
				return nil, ErrArgs("PUSHKEY", 0, fmt.Sprintf("Index is out of range, value[%d]", idx))
			}
		}
		offset := 0
		for i := 0; i < count; i++ {
			if offset < len(idxes) && i == idxes[offset] {
				offset++
			} else {
				includes = append(includes, i)
			}
		}
	default:
		return nil, ErrArgs("POPHKEY", 0, fmt.Sprintf("Value should be array, but %T", val))
	}

	if _, ok := node.GetValue("isFirst"); !ok {
		node.SetValue("isFirst", true)
		cols := node.task.ResultColumns() // cols contains "ROWNUM"
		updateCols := []*spi.Column{cols[0]}
		for _, idx := range includes {
			updateCols = append(updateCols, cols[idx+1])
		}
		node.task.SetResultColumns(updateCols)
	}

	val := inflight.value.([]any)
	updateVal := []any{}
	for _, idx := range includes {
		updateVal = append(updateVal, val[idx])
	}
	return NewRecord(inflight.key, updateVal), nil
}

func (node *Node) fmMapValue(idx int, newValue any, opts ...any) (any, error) {
	inflight := node.Inflight()
	if inflight == nil {
		return nil, nil
	}
	switch val := inflight.value.(type) {
	case []any:
		if idx < 0 || idx >= len(val) {
			return node.fmPushValue(idx, newValue, opts...)
		}
		if _, ok := node.GetValue("isFirst"); !ok {
			node.SetValue("isFirst", true)
			if len(opts) > 0 {
				if newName, ok := opts[0].(string); ok {
					cols := node.task.ResultColumns() // cols contains "ROWNUM"
					cols[idx+1].Name = newName
				}
			}
		}
		val[idx] = newValue
		ret := NewRecord(inflight.key, val)
		return ret, nil
	default:
		if idx != 0 {
			return node.fmPushValue(idx, newValue, opts...)
		}

		if _, ok := node.GetValue("isFirst"); !ok {
			node.SetValue("isFirst", true)
			if len(opts) > 0 {
				if newName, ok := opts[0].(string); ok {
					cols := node.task.ResultColumns() // cols contains "ROWNUM"
					cols[idx+1].Name = newName
				}
			}
		}
		ret := NewRecord(inflight.key, newValue)
		return ret, nil
	}
}

func (node *Node) fmRegexp(pattern string, text string) (bool, error) {
	var expr *regexp.Regexp
	if v, exists := node.GetValue("$regexp.pattern"); exists {
		if v.(string) == pattern {
			if v, exists := node.GetValue("$regexp"); exists {
				expr = v.(*regexp.Regexp)
			}
		}
	}
	if expr == nil {
		compiled, err := regexp.Compile(pattern)
		if err != nil {
			return false, err
		}
		expr = compiled
		node.SetValue("$regexp", expr)
		node.SetValue("$regexp.pattern", pattern)
	}
	return expr.MatchString(text), nil
}

func (node *Node) fmGlob(pattern string, text string) (bool, error) {
	return glob.Match(pattern, text)
}

type LogDoer []any

func (ld LogDoer) Do(node *Node) error {
	node.task.LogInfo(ld...)
	return nil
}

func (node *Node) fmDoLog(args ...any) LogDoer {
	return LogDoer(args)
}

type HttpDoer struct {
	method  string
	url     string
	args    []string
	content any

	client *http.Client
}

func (doer *HttpDoer) Do(node *Node) error {
	var body io.Reader
	if doer.content != nil {
		buff := &bytes.Buffer{}
		csvEnc := gocsv.NewWriter(buff)
		switch v := doer.content.(type) {
		case []float64:
			arr := make([]string, len(v))
			for i, a := range v {
				arr[i] = fmt.Sprintf("%v", a)
			}
			csvEnc.Write(arr)
		case float64:
			csvEnc.Write([]string{fmt.Sprintf("%v", v)})
		case []string:
			csvEnc.Write(v)
		case string:
			csvEnc.Write([]string{v})
		case []any:
			arr := make([]string, len(v))
			for i, a := range v {
				arr[i] = fmt.Sprintf("%v", a)
			}
			csvEnc.Write(arr)
		case any:
			csvEnc.Write([]string{fmt.Sprintf("%v", v)})
		default:
			return fmt.Errorf("unhandled content value type %T", v)
		}
		csvEnc.Flush()
		body = buff
	}
	req, err := http.NewRequestWithContext(node.task.ctx, doer.method, doer.url, body)
	if err != nil {
		return err
	}

	for _, str := range doer.args {
		k, v, ok := strings.Cut(str, ":")
		if ok {
			k, v = strings.TrimSpace(k), strings.TrimSpace(v)
			req.Header.Add(k, v)
		}
	}

	if body != nil {
		if req.Header.Get("Content-Type") == "" {
			req.Header.Add("Content-Type", "text/csv")
		}
	}
	if req.Header.Get("User-Agent") == "" {
		req.Header.Add("User-Agent", "machbase-neo tql http doer")
	}
	if doer.client == nil {
		doer.client = node.task.NewHttpClient()
	}
	resp, err := doer.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		node.task.LogWarn("http-doer", doer.method, doer.url, resp.Status)
	} else if resp.StatusCode >= 300 {
		node.task.LogInfo("http-doer", doer.method, doer.url, resp.Status)
	} else {
		node.task.LogDebug("http-doer", doer.method, doer.url, resp.Status)
	}
	return nil
}

func (node *Node) fmDoHttp(method string, url string, body any, args ...string) *HttpDoer {
	var ret *HttpDoer
	if v, ok := node.GetValue("$httpDoer"); !ok {
		ret = &HttpDoer{}
		node.SetValue("$httpDoer", ret)
	} else {
		ret = v.(*HttpDoer)
	}
	ret.method = method
	ret.url = url
	ret.args = args
	ret.content = body
	return ret
}

type SubRoutine struct {
	code    string
	inValue []any
	node    *Node
}

func (sr *SubRoutine) Write(b []byte) (int, error) {
	if sr.node == nil || sr.node.task.logWriter == nil {
		return len(b), nil
	}
	return sr.node.task.logWriter.Write(b)
}

func (sr *SubRoutine) Do(node *Node) error {
	defer func() {
		if e := recover(); e != nil {
			node.task.LogErrorf("do: recover, %v", e)
		}
	}()
	sr.node = node
	subTask := NewTask()
	subTask.SetParams(node.task.params)
	subTask.SetConsoleLogLevel(node.task.consoleLogLevel)
	subTask.SetConsole(node.task.consoleUser, node.task.consoleId)
	subTask.SetLogWriter(sr)
	//	subTask.SetInputReader(r io.Reader)
	subTask.SetOutputWriterJson(io.Discard, true)
	subTask.SetDatabase(node.task.db)
	subTask.argValues = sr.inValue

	reader := bytes.NewBufferString(sr.code)
	if err := subTask.Compile(reader); err != nil {
		subTask.LogError("do: compile error", err.Error())
		return err
	}
	switch subTask.output.Name() {
	case "INSERT()":
	case "APPEND()":
	case "DISCARD()":
	default:
		sinkName := subTask.output.Name()
		subTask.LogWarnf("do: %s sink does not work in a sub-routine", sinkName)
	}

	var subTaskCancel context.CancelFunc
	subTask.ctx, subTaskCancel = context.WithCancel(node.task.ctx)
	defer subTaskCancel()

	result := subTask.Execute()
	if result.Err != nil {
		subTask.LogError("do: execution fail", result.Err.Error())
		return result.Err
	}
	return nil
}

func (node *Node) fmDo(args ...any) (*SubRoutine, error) {
	if len(args) == 0 {
		return nil, ErrArgs("do", len(args), "do: code is required")
	}
	code, ok := args[len(args)-1].(string)
	code = strings.TrimSpace(code)
	if !ok || code == "" {
		return nil, ErrArgs("do", len(args)-1, "do: code is required")
	}
	inValue := []any{}
	if len(args) > 1 {
		inValue = args[0 : len(args)-1]
	}
	ret := &SubRoutine{
		code:    code,
		inValue: inValue,
	}
	return ret, nil
}

type WhenDoer interface {
	Do(*Node) error
}

var (
	_ WhenDoer = &SubRoutine{}
	_ WhenDoer = LogDoer{}
	_ WhenDoer = &HttpDoer{}
)

func (node *Node) fmWhen(cond bool, action any) any {
	if !cond {
		return node.Inflight()
	}
	doer, ok := action.(WhenDoer)
	if !ok {
		node.task.LogErrorf("f(WHEN) 2nd arg is not a Doer, got %T", action)
	} else {
		defer func() {
			if e := recover(); e != nil {
				node.task.LogErrorf("f(WHEN) Doer fail recover, %v", e)
			}
		}()
		if err := doer.Do(node); err != nil {
			node.task.LogErrorf("f(WHEN) Doer fail, %s", err.Error())
		}
	}
	return node.Inflight()
}
