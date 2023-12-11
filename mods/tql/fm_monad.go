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

	"github.com/machbase/neo-server/mods/codec/opts"
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

func (node *Node) fmThrottle(tps float64) *Record {
	var th *Throttle
	if v, ok := node.GetValue("throttle"); ok {
		th = v.(*Throttle)
	} else {
		dur := float64(time.Second) / tps
		th = &Throttle{
			minDuration: time.Duration(dur),
			last:        time.Now(),
		}
		node.SetValue("throttle", th)
	}
	inflight := node.Inflight()
	if inflight == nil {
		return inflight
	}

	since := time.Since(th.last)
	if since >= th.minDuration {
		th.last = time.Now()
		return inflight
	} else {
		time.Sleep(th.minDuration - since)
		th.last = time.Now()
		return inflight
	}
}

type Throttle struct {
	minDuration time.Duration
	last        time.Time
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

func (node *Node) fmList(args ...any) any {
	ret := []any{}
	ret = append(ret, args...)
	return ret
}

func (node *Node) fmDictionary(args ...any) (any, error) {
	ret := map[string]any{}
	for i := 0; i < len(args); i += 2 {
		if i+1 >= len(args) {
			break
		}
		if name, ok := args[i].(string); ok {
			ret[name] = args[i+1]
		} else {
			return nil, fmt.Errorf("dictionary name should be string, got args[%d] %T", i, args[i])
		}
	}
	return ret, nil
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
			buffer:    map[any][]GroupColumn{},
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
	buffer    map[any][]GroupColumn
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
	var chunk *GroupColumnChunk
	if cs, ok := gr.buffer[by.Value]; ok {
		chunk = cs[0].(*GroupColumnChunk)
	} else {
		chunk = &GroupColumnChunk{name: "chunk"}
		gr.buffer[by.Value] = []GroupColumn{chunk}
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
	var cols []GroupColumn
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

func (ga *GroupAggregate) NewBuffer() GroupColumn {
	switch ga.Type {
	case "by":
		return &GroupColumnConst{value: ga.Value}
	case "chunk":
		return &GroupColumnChunk{name: ga.Type}
	case "mean", "median", "median-interpolated", "stddev", "stderr", "entropy", "mode":
		return &GroupColumnContainer{name: ga.Type}
	case "first", "last", "min", "max", "sum":
		return &GroupColumnSingle{name: ga.Type}
	case "avg", "rss", "rms":
		return &GroupColumnCounter{name: ga.Type}
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
			return newAggregate("min", value, name), nil
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
			return newAggregate("max", value, name), nil
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
	var gr *Group
	if obj, ok := node.GetValue("group"); ok {
		gr = obj.(*Group)
	} else {
		gr = &Group{
			buffer:    map[any][]GroupColumn{},
			chunkMode: true,
		}
		node.SetValue("group", gr)
		node.SetEOF(gr.onEOF)
		for _, arg := range args {
			switch v := arg.(type) {
			case *lazyOption:
				gr.lazy = v.flag
			}
		}
	}
	key := node.Inflight().Key()
	by := node.fmBy(key, "KEY").(*GroupAggregate)
	gr.pushChunk(node, by)
	return nil
}

type GroupColumn interface {
	Append(any) error
	Result() any
}

var (
	_ GroupColumn = &GroupColumnSingle{}
	_ GroupColumn = &GroupColumnContainer{}
	_ GroupColumn = &GroupColumnCounter{}
	_ GroupColumn = &GroupColumnChunk{}
	_ GroupColumn = &GroupColumnConst{}
)

// chunk
type GroupColumnChunk struct {
	name   string
	values []any
}

func (gc *GroupColumnChunk) Result() any {
	ret := gc.values
	gc.values = []any{}
	return ret
}

func (gc *GroupColumnChunk) Append(v any) error {
	gc.values = append(gc.values, v)
	return nil
}

// const
type GroupColumnConst struct {
	value any
}

func (gc *GroupColumnConst) Result() any {
	return gc.value
}

func (gc *GroupColumnConst) Append(v any) error {
	return nil
}

// "mean", "median", "median-interpolated", "stddev", "stderr", "entropy", "mode"
type GroupColumnContainer struct {
	name   string
	values []float64
}

func (gc *GroupColumnContainer) Result() any {
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

func (gc *GroupColumnContainer) Append(v any) error {
	f, err := util.ToFloat64(v)
	if err != nil {
		return err
	}
	gc.values = append(gc.values, f)
	return nil
}

// avg, rss, rms
type GroupColumnCounter struct {
	name  string
	value float64
	count int
}

func (gc *GroupColumnCounter) Result() any {
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

func (gc *GroupColumnCounter) Append(v any) error {
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
type GroupColumnSingle struct {
	name     string
	value    any
	hasValue bool
}

func (gc *GroupColumnSingle) Result() any {
	if gc.hasValue {
		ret := gc.value
		gc.value, gc.hasValue = 0, false
		return ret
	}
	return nil
}

func (gc *GroupColumnSingle) Append(v any) error {
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

func (node *Node) fmAbsDiff(idx int, value any, args ...any) (any, error) {
	var df *diff
	if v, ok := node.GetValue("_abs_diff"); ok {
		df = v.(*diff)
	} else {
		df = &diff{isPrevNull: true, abs: true}
		node.SetValue("_abs_diff", df)
	}
	return df.diff(node, idx, value, args)
}

func (node *Node) fmNonNegativeDiff(idx int, value any, args ...any) (any, error) {
	var df *diff
	if v, ok := node.GetValue("_non_negative_diff"); ok {
		df = v.(*diff)
	} else {
		df = &diff{isPrevNull: true, nonNegative: true}
		node.SetValue("_non_negative_diff", df)
	}
	return df.diff(node, idx, value, args)
}

func (node *Node) fmDiff(idx int, value any, args ...any) (any, error) {
	var df *diff
	if v, ok := node.GetValue("_diff"); ok {
		df = v.(*diff)
	} else {
		df = &diff{isPrevNull: true}
		node.SetValue("_diff", df)
	}
	return df.diff(node, idx, value, args)
}

type diff struct {
	prev        float64
	isPrevNull  bool
	abs         bool
	nonNegative bool
}

func (df *diff) diff(node *Node, idx int, value any, opts []any) (any, error) {
	if value == nil {
		df.isPrevNull = true
		return node.fmMapValue(idx, nil, opts...)
	}

	var fv float64
	if f, err := util.ToFloat64(value); err == nil {
		fv = f
	} else {
		df.isPrevNull = true
		return node.fmMapValue(idx, nil, opts...)
	}

	if df.isPrevNull {
		df.prev = fv
		df.isPrevNull = false
		return node.fmMapValue(idx, nil, opts...)
	} else {
		ret := fv - df.prev
		if df.abs {
			ret = math.Abs(ret)
		} else if df.nonNegative && ret < 0 {
			ret = 0
		}
		df.prev = fv
		df.isPrevNull = false
		return node.fmMapValue(idx, ret, opts...)
	}
}

func (node *Node) fmMovAvg(idx int, value any, lag int, opts ...any) (any, error) {
	if lag <= 0 {
		return 0, ErrArgs("movavg", 1, "lag should be larger than 0")
	}
	var fv *float64
	if f, err := util.ToFloat64(value); err == nil {
		fv = &f
	}
	var ma *movavg
	if v, ok := node.GetValue("movavg"); ok {
		ma = v.(*movavg)
	} else {
		ma = &movavg{}
		node.SetValue("movavg", ma)
	}
	ma.elements = append(ma.elements, fv)
	if len(ma.elements) > lag {
		ma.elements = ma.elements[len(ma.elements)-lag:]
	}
	if len(ma.elements) == lag {
		sum := 0.0
		countNil := 0
		for _, e := range ma.elements {
			if e != nil {
				sum += *e
			} else {
				countNil++
			}
		}
		if countNil == lag {
			return node.fmMapValue(idx, nil, opts...)
		} else {
			ret := sum / float64(lag-countNil)
			return node.fmMapValue(idx, ret, opts...)
		}
	} else {
		return node.fmMapValue(idx, nil, opts...)
	}
}

type movavg struct {
	elements []*float64
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

func (node *Node) fmTranspose(args ...any) (any, error) {
	var tr *Transposer
	if v, ok := node.GetValue("transposer"); ok {
		tr = v.(*Transposer)
	} else {
		tr = &Transposer{}
		node.SetValue("transposer", tr)
		if len(args) > 0 {
			for _, arg := range args {
				switch argv := arg.(type) {
				case opts.Option:
					argv(tr)
				case float64:
					tr.transposedIndexes = append(tr.transposedIndexes, int(argv))
				default:
					return nil, ErrArgs("TRANSPOSE", 0, fmt.Sprintf("unknown type of argument %T", argv))
				}
			}
		}
		if len(tr.fixedIndexes) > 0 && len(tr.transposedIndexes) > 0 {
			return nil, ErrArgs("TRANSPOSE", 1, "cannot use 'fixed columns' and 'transposed columns' together")
		}

		cols := node.task.ResultColumns()
		inflight := node.Inflight()
		switch vals := inflight.Value().(type) {
		case []any:
			if tr.header {
				for _, v := range vals {
					switch str := v.(type) {
					case string:
						tr.headerNames = append(tr.headerNames, str)
					case *string:
						tr.headerNames = append(tr.headerNames, *str)
					default:
						tr.headerNames = append(tr.headerNames, fmt.Sprintf("%v", str))
					}
				}
			}
			fixed, _ := tr.fixedAndTransposed(vals)
			newCols := spi.Columns{cols[0]}
			for i, n := range fixed {
				if len(tr.headerNames) > n {
					cols[n+1].Name = tr.headerNames[n]
					cols[n+1].Type = ""
				} else {
					cols[n+1].Name = fmt.Sprintf("column%d", i)
					cols[n+1].Type = ""
				}
				newCols = append(newCols, cols[n+1])
			}
			if tr.header {
				newCols = append(newCols, &spi.Column{Name: "header"})
			}
			newCols = append(newCols, &spi.Column{Name: fmt.Sprintf("column%d", len(newCols)-1)})
			node.task.SetResultColumns(newCols)
		case any:
			newCols := spi.Columns{cols[0]}
			if tr.header {
				tr.headerNames = []string{fmt.Sprintf("%v", vals)}
				newCols = append(newCols, &spi.Column{Name: fmt.Sprintf("column%d", len(newCols)-1)})
			}
			newCols = append(newCols, &spi.Column{Name: "column1"})
			node.task.SetResultColumns(newCols)
		}
		if tr.header {
			return nil, nil
		}
	}

	return tr.do(node)
}

func (node *Node) fmFixed(args ...int) opts.Option {
	return func(obj any) {
		if tr, ok := obj.(*Transposer); ok {
			tr.fixedIndexes = append(tr.fixedIndexes, args...)
		}
	}
}

type Transposer struct {
	fixedIndexes      []int
	transposedIndexes []int
	headerNames       []string
	header            bool

	fixed      []int
	transposed []int
}

func (tr *Transposer) SetHeader(flag bool) {
	tr.header = flag
}

func (tr *Transposer) contains(list []int, i int) bool {
	for _, v := range list {
		if v == i {
			return true
		}
	}
	return false
}

func (tr *Transposer) fixedAndTransposed(values []any) ([]int, []int) {
	if tr.fixed != nil && tr.transposed != nil {
		return tr.fixed, tr.transposed
	}
	fixed := []int{}
	transposed := []int{}
	if len(tr.transposedIndexes) == 0 && len(tr.fixedIndexes) == 0 {
		for i := range values {
			transposed = append(transposed, i)
		}
	} else if len(tr.transposedIndexes) > 0 {
		for i := range values {
			if tr.contains(tr.transposedIndexes, i) {
				transposed = append(transposed, i)
			} else {
				fixed = append(fixed, i)
			}
		}
	} else {
		for i := range values {
			if tr.contains(tr.fixedIndexes, i) {
				fixed = append(fixed, i)
			} else {
				transposed = append(transposed, i)
			}
		}
	}
	tr.fixed, tr.transposed = fixed, transposed
	return fixed, transposed
}

func (tr *Transposer) do(node *Node) (any, error) {
	inflight := node.Inflight()
	if inflight.Value() == nil {
		return nil, nil
	}
	inflightValue := inflight.Value()

	var values []any
	if v, ok := inflightValue.([]any); !ok {
		if tr.header && len(tr.headerNames) > 0 {
			return NewRecord(inflight.Key(), []any{tr.headerNames[0], v}), nil
		} else {
			return inflight, nil
		}
	} else {
		values = v
	}

	fixed, transposed := tr.fixedAndTransposed(values)
	fixedVals := []any{}
	for _, n := range fixed {
		fixedVals = append(fixedVals, values[n])
	}
	if len(transposed) == 0 {
		return NewRecord(inflight.Key(), fixedVals), nil
	}
	var arr []*Record
	for _, n := range transposed {
		newVals := make([]any, len(fixedVals))
		copy(newVals, fixedVals)
		if tr.header {
			if n < len(tr.headerNames) {
				newVals = append(newVals, tr.headerNames[n])
			} else {
				newVals = append(newVals, "no-header")
			}
		}
		newVals = append(newVals, values[n])
		arr = append(arr, NewRecord(inflight.Key(), newVals))
	}
	return arr, nil
}
