package tql

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	gocsv "encoding/csv"

	"github.com/machbase/neo-server/api"
	"github.com/machbase/neo-server/mods/codec/opts"
	"github.com/machbase/neo-server/mods/nums"
	"github.com/machbase/neo-server/mods/nums/kalman"
	"github.com/machbase/neo-server/mods/nums/kalman/models"
	"github.com/machbase/neo-server/mods/util"
	"github.com/machbase/neo-server/mods/util/glob"
	"github.com/pkg/errors"
	"gonum.org/v1/gonum/interp"
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

func (node *Node) fmFilterChanged(value any, args ...any) any {
	inflight := node.Inflight()
	if inflight == nil {
		return nil
	}
	var bf *BufferedFilter
	var retain *RetainDuration
	var useFirst, withLast bool

	for _, arg := range args {
		switch av := arg.(type) {
		case *RetainDuration:
			retain = av
		case BufferedFilterUseFirst:
			useFirst, withLast = true, bool(av)
		}
	}

	if v, ok := node.GetValue("filter_changed"); ok {
		bf = v.(*BufferedFilter)
	} else {
		bf = &BufferedFilter{
			last: unboxValue(value),
		}
		if retain != nil {
			bf.lastTimestamp = retain.timestamp
		}
		if withLast {
			bf.lastRecord = inflight
		}
		node.SetValue("filter_changed", bf)
		node.SetEOF(func(node *Node) {
			if withLast && bf.lastRecord != nil {
				bf.lastRecord.Tell(node.next)
			}
		})
		return inflight
	}

	val := unboxValue(value)
	if retain != nil {
		if inflight.IsEOF() || bf.last != val {
			var ret *Record
			if withLast {
				ret = bf.lastRecord
			}
			bf.last = val
			bf.lastTimestamp = retain.timestamp
			bf.lastYield = false
			bf.firstRecord = inflight
			bf.lastRecord = nil
			return ret
		}
		if !bf.lastYield && retain.timestamp.Sub(bf.lastTimestamp) >= retain.duration {
			bf.lastYield = true
			if useFirst {
				ret := bf.firstRecord
				bf.firstRecord = nil
				bf.lastRecord = inflight
				return ret
			} else {
				return inflight
			}
		}
		bf.lastRecord = inflight
	} else {
		if bf.last != val {
			bf.last = val
			bf.lastYield = true
			if withLast {
				if bf.lastRecord != nil {
					ret := []*Record{bf.lastRecord, inflight}
					bf.lastRecord = inflight
					return ret
				}
			} else {
				bf.lastRecord = nil
				return inflight
			}
		}
		bf.lastRecord = inflight
	}
	return nil
}

type BufferedFilter struct {
	last          any
	lastTimestamp time.Time
	lastYield     bool
	firstRecord   *Record
	lastRecord    *Record
}

type BufferedFilterUseFirst bool

func (node *Node) fmUseFirstWithLast(flag bool) BufferedFilterUseFirst {
	return BufferedFilterUseFirst(flag)
}

type RetainDuration struct {
	timestamp time.Time
	duration  time.Duration
}

func (node *Node) fmRetain(value any, duration any) (*RetainDuration, error) {
	ret := &RetainDuration{}
	if v, err := util.ToTime(value); err != nil {
		return nil, err
	} else {
		ret.timestamp = v
	}
	if v, err := util.ToDuration(duration); err != nil {
		return nil, err
	} else {
		ret.duration = v
	}
	return ret, nil
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
	if node.Name() == "GROUP()" {
		var ret *GroupAggregate
		if len(args) == 0 {
			ret = newAggregate("list", []any{nil})
		} else {
			ret = newAggregate("list", args[0], args[1:]...)
		}
		return ret
	}
	return args
}

func (node *Node) fmDictionary(args ...any) (any, error) {
	ret := map[string]any{}
	for i := 0; i < len(args); i += 2 {
		if i+1 >= len(args) {
			return nil, fmt.Errorf("dict() name %q doen't match with any value", args[i])
		}
		if name, ok := args[i].(string); ok {
			ret[name] = args[i+1]
		} else {
			return nil, fmt.Errorf("dict() name should be string, got args[%d] %T", i, args[i])
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
			filler:    []GroupFiller{},
			chunkMode: true,
		}
		node.SetValue("group", gr)
		node.SetEOF(gr.onEOF)
		shouldSetColumns = true
		for _, arg := range args {
			switch v := arg.(type) {
			case *GroupAggregate:
				// if has at least one aggregate, chunk mode is off
				gr.chunkMode = false
				if v.Type == GroupByTimeWindow {
					gr.byTimeWindow = true
					gr.twFrom = v.twFrom
					gr.twUntil = v.twUntil
					gr.twPeriod = v.twPeriod
				}
				gr.filler = append(gr.filler, v.newFiller())
			}
		}
	}

	for _, arg := range args {
		switch v := arg.(type) {
		case *GroupAggregate:
			columns = append(columns, v)
			if v.Type == GroupBy {
				by = v
			} else if v.Type == GroupByTimeWindow {
				by = v
			}
		case *lazyOption:
			gr.lazy = v.flag
		default:
			return ErrorRecord(fmt.Errorf("GROUP() unknown type '%T' in arguments", v))
		}
	}
	if by != nil && by.Value == nil {
		return ErrorRecord(fmt.Errorf("GROUP() has by() with NULL"))
	}
	if shouldSetColumns {
		if gr.chunkMode {
			if by == nil || by.Value == nil {
				return ErrorRecord(fmt.Errorf("GROUP() has no aggregator"))
			}
		} else {
			cols := make([]*api.Column, len(columns)+1)
			cols[0] = &api.Column{Name: "ROWNUM", Type: "int"}
			for i, c := range columns {
				resultType := c.ColumnType()
				if c.ValueType != "" {
					resultType = c.ValueType
				}
				cols[i+1] = &api.Column{
					Name: c.Name,
					Type: resultType,
				}
			}
			node.task.SetResultColumns(cols)
		}
	}
	if gr.byTimeWindow {
		// if current record's time is out of the timewindow range.
		if byTime, ok := by.Value.(time.Time); ok {
			if byTime.Before(by.twFrom) {
				return nil
			} else if byTime == by.twUntil || byTime.After(by.twUntil) {
				return nil
			}
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
	filler    []GroupFiller
	curKey    any
	chunkMode bool

	byTimeWindow bool
	twFrom       time.Time
	twUntil      time.Time
	twPeriod     time.Duration
	twCurWindow  time.Time
}

func (gr *Group) onEOF(node *Node) {
	if gr.chunkMode {
		for k, cols := range gr.buffer {
			r := cols[0].Result()
			if v, ok := r.([]any); ok {
				gr.yield(node, k, v, true)
			} else {
				gr.yield(node, k, []any{r}, true)
			}
		}
		gr.buffer = nil
	} else {
		keys := make([]any, 0, len(gr.buffer))
		for k := range gr.buffer {
			keys = append(keys, k)
		}
		keys = util.SortAny(keys)
		for _, k := range keys {
			cols := gr.buffer[k]
			v := make([]any, len(cols))
			for i, c := range cols {
				v[i] = c.Result()
			}
			gr.yield(node, k, v, true)
		}
		gr.buffer = nil
	}
}

const __group_by_all = "__group_by_all__"

func (gr *Group) pushChunk(node *Node, by *GroupAggregate) {
	var chunk *GroupColumnChunk
	if by == nil {
		if cs, ok := gr.buffer[__group_by_all]; ok {
			chunk = cs[0].(*GroupColumnChunk)
		} else {
			chunk = &GroupColumnChunk{name: "chunk"}
			gr.buffer[__group_by_all] = []GroupColumn{chunk}
		}
	} else {
		if cs, ok := gr.buffer[by.Value]; ok {
			chunk = cs[0].(*GroupColumnChunk)
		} else {
			chunk = &GroupColumnChunk{name: "chunk"}
			gr.buffer[by.Value] = []GroupColumn{chunk}
		}
	}
	inflight := node.Inflight()
	if inflight == nil {
		return
	}
	chunk.Append(inflight.Value())
	if !gr.lazy && gr.curKey != nil && gr.curKey != by.Value {
		if ret, ok := gr.buffer[gr.curKey]; ok {
			r := ret[0].Result()
			if v, ok := r.([]any); ok {
				gr.yield(node, gr.curKey, v, false)
			} else {
				gr.yield(node, gr.curKey, []any{r}, false)
			}
			delete(gr.buffer, gr.curKey)
		}
	}
	if by != nil {
		gr.curKey = by.Value
	}
}

func (gr *Group) push(node *Node, by *GroupAggregate, columns []*GroupAggregate) {
	var buffers []GroupColumn
	if by == nil {
		if cs, ok := gr.buffer[__group_by_all]; ok {
			buffers = cs
		} else {
			for _, c := range columns {
				if buff := c.NewBuffer(); buff != nil {
					buffers = append(buffers, buff)
				} else {
					node.task.LogErrorf("%s, invalid aggregate %q", node.Name(), c.Type)
					return
				}
			}
			gr.buffer[__group_by_all] = buffers
		}
	} else {
		if cs, ok := gr.buffer[by.Value]; ok {
			buffers = cs
		} else {
			for _, c := range columns {
				if buff := c.NewBuffer(); buff != nil {
					buffers = append(buffers, buff)
				} else {
					node.task.LogErrorf("%s, invalid aggregate %q", node.Name(), c.Type)
					return
				}
			}
			gr.buffer[by.Value] = buffers
		}
	}

	for i, c := range columns {
		if c.where {
			buffers[i].Append(c.Value)
		}
	}

	if !gr.lazy && gr.curKey != nil && gr.curKey != by.Value {
		if cols, ok := gr.buffer[gr.curKey]; ok {
			v := make([]any, len(cols))
			for i, c := range cols {
				v[i] = c.Result()
			}
			gr.yield(node, gr.curKey, v, false)
			delete(gr.buffer, gr.curKey)
		}
	}
	if by != nil {
		gr.curKey = by.Value
	}
}

func (gr *Group) yield(node *Node, key any, values []any, isLast bool) {
	if gr.byTimeWindow {
		recWindow := key.(time.Time)
		if gr.twCurWindow.IsZero() {
			fromWindow := time.Unix(0, (gr.twFrom.UnixNano()/int64(gr.twPeriod))*int64(gr.twPeriod))
			if isLast {
				untilWindow := time.Unix(0, (gr.twUntil.UnixNano()/int64(gr.twPeriod))*int64(gr.twPeriod))
				gr.fill(node, fromWindow, untilWindow)
			} else {
				gr.fill(node, fromWindow, recWindow)
			}
		} else {
			gr.fill(node, gr.twCurWindow.Add(gr.twPeriod), recWindow)
		}
		gr.twCurWindow = recWindow
		for i, v := range values {
			if v == nil {
				values[i] = gr.filler[i].Predict(key)
			} else {
				gr.filler[i].Fit(key, v)
			}
		}
		node.yield(key, values)
		if isLast {
			gr.fill(node, recWindow.Add(gr.twPeriod), gr.twUntil.Add(gr.twPeriod-1))
		}
	} else {
		for i, v := range values {
			if v == nil {
				values[i] = gr.filler[i].Predict(key)
			} else {
				if _, ok := key.(time.Time); ok {
					if _, ok := v.(float64); ok {
						gr.filler[i].Fit(key, v)
					}
				}
			}
		}
		node.yield(key, values)
	}
}

func (gr *Group) fill(node *Node, curWindow time.Time, nextWindow time.Time) {
	for nextWindow.Sub(curWindow) >= gr.twPeriod {
		if curWindow.Sub(gr.twFrom) >= 0 {
			ret := make([]any, len(gr.filler))
			for i, fill := range gr.filler {
				ret[i] = fill.Predict(curWindow)
			}
			node.yield(curWindow, ret)
		}
		curWindow = curWindow.Add(gr.twPeriod)
	}
}

const (
	GroupBy           = "by"
	GroupByTimeWindow = "byTimeWindow"
)

func (node *Node) fmBy(value any, args ...any) (any, error) {
	var name string
	var ret *GroupAggregate
	for _, arg := range args {
		switch v := arg.(type) {
		case string:
			name = v
		case *GroupAggregate:
			ret = v
		}
	}
	if ret == nil {
		ret = &GroupAggregate{Type: GroupBy}
	}
	ret.Name = name
	if ret.Name == "" {
		ret.Name = "GROUP"
	}

	ret.Value = unboxValue(value)
	if ret.Type == GroupByTimeWindow {
		ts, err := util.ToTime(ret.Value)
		if err != nil {
			return nil, ErrArgs("timewindow()", 0, "value should be time")
		}
		ret.Value = time.Unix(0, (ts.UnixNano()/int64(ret.twPeriod))*int64(ret.twPeriod))
	}

	return ret, nil
}

func (node *Node) fmByTimeWindow(from any, until any, duration any) (any, error) {
	ret := &GroupAggregate{Type: GroupByTimeWindow}
	fnName := "timewindow()"
	if ts, err := util.ToTime(from); err != nil {
		return nil, ErrArgs(fnName, 0, fmt.Sprintf("from is not compatible type, %T", from))
	} else {
		ret.twFrom = ts
	}
	if ts, err := util.ToTime(until); err != nil {
		return nil, ErrArgs(fnName, 1, fmt.Sprintf("until is not compatible type, %T", until))
	} else {
		ret.twUntil = ts
	}
	if d, err := util.ToDuration(duration); err != nil {
		return nil, ErrArgs(fnName, 2, fmt.Sprintf("duration is not compatible, %T", duration))
	} else if d == 0 {
		return nil, ErrArgs(fnName, 2, "duration is zero")
	} else if d < 0 {
		return nil, ErrArgs(fnName, 2, "duration is negative")
	} else {
		ret.twPeriod = d
	}
	if ret.twUntil.Sub(ret.twFrom) <= ret.twPeriod {
		return nil, ErrArgs(fnName, 0, "from ~ until should be larger than duration")
	}
	return ret, nil
}

type GroupAggregate struct {
	Type       string
	Value      any
	Name       string
	ValueType  string
	Percentile float64
	Quantile   float64
	Moment     float64
	Cumulant   stat.CumulantKind

	twFrom    time.Time
	twUntil   time.Time
	twPeriod  time.Duration
	where     WherePredicate
	nullValue any
	predict   PredictType
}

type WherePredicate bool

func (node *Node) fmWhere(predicate bool) any {
	return WherePredicate(predicate)
}

type PredictType string

func (node *Node) fmPredict(predict string) (PredictType, error) {
	typ := strings.ToLower(predict)
	switch typ {
	case "piecewiseconstant":
		return PredictType(typ), nil
	case "piecewiselinear":
		return PredictType(typ), nil
	case "akimaspline":
		return PredictType(typ), nil
	case "fritschbutland":
		return PredictType(typ), nil
	case "linearregression":
		return PredictType(typ), nil
	default:
		return "", fmt.Errorf("unknown predict %q", predict)
	}
}

type Weight float64

func (node *Node) fmWeight(v float64) Weight {
	return Weight(v)
}

func (g *GroupAggregate) newFiller() GroupFiller {
	if g.Type == GroupByTimeWindow {
		return &GroupFillerTimeWindow{}
	}
	switch g.predict {
	case "piecewiseconstant":
		return &GroupFillerPredict{predictor: &interp.PiecewiseConstant{}, fallback: g.nullValue}
	case "piecewiselinear":
		return &GroupFillerPredict{predictor: &interp.PiecewiseConstant{}, fallback: g.nullValue}
	case "akimaspline":
		return &GroupFillerPredict{predictor: &interp.PiecewiseConstant{}, fallback: g.nullValue}
	case "fritschbutland":
		return &GroupFillerPredict{predictor: &interp.PiecewiseConstant{}, fallback: g.nullValue}
	case "linearregression":
		return &GroupFillerPredict{useLinearRegression: true, fallback: g.nullValue}
	default:
		return &GroupFillerNullValue{alt: g.nullValue}
	}
}

func (ga *GroupAggregate) ColumnType() string {
	switch ga.Type {
	case GroupBy:
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
	case GroupByTimeWindow:
		return "time"
	case "chunk":
		return "array"
	}
	return "float64"
}

func (ga *GroupAggregate) NewBuffer() GroupColumn {
	switch ga.Type {
	case GroupBy:
		return &GroupColumnConst{value: ga.Value}
	case GroupByTimeWindow:
		return &GroupColumnTimeWindow{value: ga.Value}
	case "chunk", "list":
		return &GroupColumnChunk{name: ga.Type}
	case "mean", "variance", "stddev", "stderr", "entropy", "mode":
		return &GroupColumnContainer{name: ga.Type}
	case "quantile":
		return &GroupColumnContainer{name: ga.Type, percentile: ga.Percentile, cumulant: ga.Cumulant}
	case "cdf":
		return &GroupColumnContainer{name: ga.Type, quantile: ga.Quantile, cumulant: ga.Cumulant}
	case "lrs", "correlation", "covariance":
		return &GroupColumnRelation{name: ga.Type}
	case "moment":
		return &GroupColumnMoment{name: ga.Type, moment: ga.Moment}
	case "first", "last", "min", "max", "sum":
		return &GroupColumnSingle{name: ga.Type}
	case "avg", "count", "rss", "rms":
		return &GroupColumnCounter{name: ga.Type}
	default:
		return nil
	}
}

func newAggregate(typ string, value any, args ...any) *GroupAggregate {
	ret := &GroupAggregate{Type: typ, Value: value, where: WherePredicate(true)}
	for _, arg := range args {
		switch v := arg.(type) {
		case string:
			ret.Name = v
		case WherePredicate:
			ret.where = v
		case *NullValue:
			ret.nullValue = v.altValue
		case PredictType:
			ret.predict = v
		case Weight:
			if arr, ok := ret.Value.([]any); ok {
				ret.Value = append(arr, v)
			} else {
				ret.Value = []any{ret.Value, v}
			}
		}
	}
	if ret.Name == "" {
		ret.Name = strings.ToUpper(typ)
	}
	if ret.Type == "list" {
		ret.ValueType = "list"
	}
	return ret
}

func (node *Node) fmFirst(value float64, args ...any) any {
	return newAggregate("first", value, args...)
}

func (node *Node) fmLast(value float64, args ...any) any {
	return newAggregate("last", value, args...)
}

func (node *Node) fmMin(value float64, other ...any) (any, error) {
	if node.Name() == "GROUP()" {
		return newAggregate("min", value, other...), nil
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
		return newAggregate("max", value, other...), nil
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

func (node *Node) fmSum(value float64, args ...any) any {
	return newAggregate("sum", value, args...)
}

func (node *Node) fmCount(args ...any) (any, error) {
	if node.Name() == "GROUP()" {
		var value any
		var remain []any
		if len(args) > 0 {
			value = args[0]
		}
		if len(args) > 1 {
			remain = args[1:]
		}
		agg := newAggregate("count", value, remain...)
		return agg, nil
	} else {
		return nums.Count(args...)
	}
}

func (node *Node) fmMean(value float64, args ...any) any {
	return newAggregate("mean", value, args...)
}

func (node *Node) fmVariance(value float64, args ...any) any {
	return newAggregate("variance", value, args...)
}

func (node *Node) fmLRS(xval any, yval float64, args ...any) any {
	var x any
	switch xv := xval.(type) {
	case float64:
		x = xv
	case *float64:
		x = *xv
	case time.Time:
		x = float64(xv.UnixNano())
	case *time.Time:
		x = float64(xv.UnixNano())
	default:
		return ErrWrongTypeOfArgs("lrs", 0, "float or time", xv)
	}
	return newAggregate("lrs", []any{x, yval}, args...)
}

func (node *Node) fmCorrelation(x, y float64, args ...any) any {
	ret := newAggregate("correlation", []any{x, y}, args...)
	return ret
}

func (node *Node) fmCovariance(x, y float64, args ...any) any {
	ret := newAggregate("covariance", []any{x, y}, args...)
	return ret
}

func (node *Node) fmCDF(value float64, q float64, args ...any) any {
	ret := newAggregate("cdf", value, args...)
	ret.Cumulant = stat.Empirical
	ret.Quantile = q
	return ret
}

func (node *Node) fmQuantile(value float64, p float64, args ...any) any {
	ret := newAggregate("quantile", value, args...)
	ret.Cumulant = stat.Empirical
	ret.Percentile = p
	return ret
}

func (node *Node) fmQuantileInterpolated(value float64, p float64, args ...any) any {
	ret := newAggregate("quantile", value, args...)
	ret.Cumulant = stat.LinInterp
	ret.Percentile = p
	return ret
}

func (node *Node) fmMedian(value float64, args ...any) any {
	ret := newAggregate("quantile", value, args...)
	ret.Cumulant = stat.Empirical
	ret.Percentile = 0.5
	return ret
}

func (node *Node) fmMedianInterpolated(value float64, args ...any) any {
	ret := newAggregate("quantile", value, args...)
	ret.Cumulant = stat.LinInterp
	ret.Percentile = 0.5
	return ret
}

func (node *Node) fmMoment(value float64, moment float64, args ...any) any {
	ret := newAggregate("moment", value, args...)
	ret.Moment = moment
	return ret
}

func (node *Node) fmStdDev(value float64, args ...any) any {
	return newAggregate("stddev", value, args...)
}

func (node *Node) fmStdErr(value float64, args ...any) any {
	return newAggregate("stderr", value, args...)
}

func (node *Node) fmEntropy(value float64, args ...any) any {
	return newAggregate("entropy", value, args...)
}

func (node *Node) fmMode(value float64, args ...any) any {
	return newAggregate("mode", value, args...)
}

func (node *Node) fmAvg(value float64, args ...any) any {
	return newAggregate("avg", value, args...)
}

func (node *Node) fmRSS(value float64, args ...any) any {
	return newAggregate("rss", value, args...)
}

func (node *Node) fmRMS(value float64, args ...any) any {
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
	inflight := node.Inflight()
	if inflight == nil {
		return nil
	}
	key := inflight.Key()
	agg, _ := node.fmBy(key, "KEY")
	by := agg.(*GroupAggregate)
	gr.pushChunk(node, by)
	return nil
}

type GroupFiller interface {
	Fit(x, y any)
	Predict(x any) any
}

var (
	_ GroupFiller = &GroupFillerNullValue{}
	_ GroupFiller = &GroupFillerPredict{}
	_ GroupFiller = &GroupFillerTimeWindow{}
)

type GroupFillerNullValue struct {
	alt any
}

func (nv *GroupFillerNullValue) Fit(x, y any) {
}

func (nv *GroupFillerNullValue) Predict(x any) any {
	return nv.alt
}

type GroupFillerTimeWindow struct {
}

func (tf *GroupFillerTimeWindow) Fit(x, y any) {
}

func (tf *GroupFillerTimeWindow) Predict(x any) any {
	return x
}

type GroupFillerPredict struct {
	fallback            any
	predictor           interp.FittablePredictor
	useLinearRegression bool
	xs                  []float64
	ys                  []float64
}

func (p *GroupFillerPredict) unbox(v any) (float64, bool) {
	if v == nil {
		return 0, false
	}
	switch vv := v.(type) {
	case *float64:
		return *vv, true
	case float64:
		return vv, true
	case *time.Time:
		return float64(vv.UnixNano()), true
	case time.Time:
		return float64(vv.UnixNano()), true
	case int:
		return float64(vv), true
	case *int:
		return float64(*vv), true
	case int32:
		return float64(vv), true
	case *int32:
		return float64(*vv), true
	case int64:
		return float64(vv), true
	case *int64:
		return float64(*vv), true
	default:
		return 0, false
	}
}

func (p *GroupFillerPredict) Fit(x, y any) {
	if p.predictor == nil && !p.useLinearRegression {
		return
	}

	xv, ok := p.unbox(x)
	if !ok {
		return
	}
	yv, ok := p.unbox(y)
	if !ok {
		return
	}
	p.xs = append(p.xs, xv)
	p.ys = append(p.ys, yv)

	limit := 100
	if len(p.xs) > limit {
		p.xs = p.xs[len(p.xs)-limit:]
		p.ys = p.ys[len(p.ys)-limit:]
	}
}

func (p *GroupFillerPredict) Predict(x any) any {
	if p.predictor == nil && !p.useLinearRegression {
		return p.fallback
	}
	if len(p.xs) < 2 || len(p.xs) != len(p.ys) {
		return p.fallback
	}

	ret := p.fallback
	if p.useLinearRegression {
		origin := false
		// y = alpha + beta*x
		alpha, beta := stat.LinearRegression(p.xs, p.ys, nil, origin)
		if xv, ok := p.unbox(x); ok {
			ret = alpha + beta*xv
		}
	} else if p.predictor != nil {
		if err := p.predictor.Fit(p.xs, p.ys); err == nil {
			if xv, ok := p.unbox(x); ok {
				ret = p.predictor.Predict(xv)
			}
		}
	}
	return ret
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
	_ GroupColumn = &GroupColumnTimeWindow{}
	_ GroupColumn = &GroupColumnRelation{}
	_ GroupColumn = &GroupColumnMoment{}
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

// timewindow
type GroupColumnTimeWindow struct {
	value any
}

func (gt *GroupColumnTimeWindow) Result() any {
	return gt.value
}

func (gt *GroupColumnTimeWindow) Append(v any) error {
	return nil
}

// lrs, correlation, covariance
type GroupColumnRelation struct {
	name string
	x    []float64
	wv   nums.WeightedFloat64Slice
}

func (cr *GroupColumnRelation) Result() any {
	if len(cr.x) != len(cr.wv) || len(cr.x) == 0 {
		return nil
	}
	switch cr.name {
	case "lrs": // y = alpha + beta*x
		_, beta := stat.LinearRegression(cr.x, cr.wv.Values(), cr.wv.Weights(), false)
		if beta != beta { // NaN
			return nil
		}
		return beta
	case "correlation":
		return stat.Correlation(cr.x, cr.wv.Values(), cr.wv.Weights())
	case "covariance":
		return stat.Covariance(cr.x, cr.wv.Values(), cr.wv.Weights())
	}
	return nil
}

func (cr *GroupColumnRelation) Append(value any) error {
	if arr, ok := value.([]any); !ok || len(arr) < 2 {
		return nil
	} else {
		if val, err := util.ToFloat64(arr[0]); err != nil {
			return nil
		} else {
			cr.x = append(cr.x, val)
		}
		var v, w float64 = 0.0, 1.0
		if f, err := util.ToFloat64(arr[1]); err != nil {
			return nil
		} else {
			v = f
		}
		for _, elm := range arr[2:] {
			switch f := elm.(type) {
			case Weight:
				w = float64(f)
			}
		}
		cr.wv = append(cr.wv, nums.WeightedFloat64ValueWeight(v, w))
		return nil
	}
}

// "moment"
type GroupColumnMoment struct {
	name   string
	moment float64
	wv     nums.WeightedFloat64Slice
}

func (cr *GroupColumnMoment) Result() any {
	if len(cr.wv) == 0 {
		return nil
	}
	switch cr.name {
	case "moment":
		return stat.Moment(cr.moment, cr.wv.Values(), cr.wv.Weights())
	}
	return nil
}

func (cm *GroupColumnMoment) Append(value any) error {
	var v, w float64
	if arr, ok := value.([]any); ok {
		if len(arr) > 0 {
			if f, err := util.ToFloat64(arr[0]); err != nil {
				return err
			} else {
				v = f
			}
		}
		if len(arr) > 1 {
			for _, elm := range arr[1:] {
				switch f := elm.(type) {
				case Weight:
					w = float64(f)
				}
			}
		}
	} else if f, err := util.ToFloat64(value); err != nil {
		return err
	} else {
		v, w = f, 1.0
	}
	cm.wv = append(cm.wv, nums.WeightedFloat64ValueWeight(v, w))
	return nil
}

// "mean", "variance", "cdf", "quantile", "stddev", "stderr", "entropy", "mode"
type GroupColumnContainer struct {
	name       string
	wv         nums.WeightedFloat64Slice
	quantile   float64
	percentile float64
	cumulant   stat.CumulantKind
}

func (gc *GroupColumnContainer) Result() any {
	defer func() {
		gc.wv = gc.wv[0:0]
	}()
	var ret float64
	switch gc.name {
	case "cdf":
		if len(gc.wv) == 0 {
			return nil
		}
		gc.wv.Sort()
		ret = stat.CDF(gc.quantile, gc.cumulant, gc.wv.Values(), gc.wv.Weights())
	case "quantile":
		if len(gc.wv) == 0 {
			return nil
		}
		gc.wv.Sort()
		ret = stat.Quantile(gc.percentile, gc.cumulant, gc.wv.Values(), gc.wv.Weights())
	case "mean":
		if len(gc.wv) == 0 {
			return nil
		}
		ret, _ = stat.MeanStdDev(gc.wv.Values(), gc.wv.Weights())
	case "variance":
		if len(gc.wv) == 0 {
			return nil
		}
		_, ret = stat.MeanVariance(gc.wv.Values(), gc.wv.Weights())
	case "stddev":
		if len(gc.wv) < 1 {
			return nil
		}
		_, ret = stat.MeanStdDev(gc.wv.Values(), gc.wv.Weights())
	case "stderr":
		if len(gc.wv) < 1 {
			return nil
		}
		_, std := stat.MeanStdDev(gc.wv.Values(), gc.wv.Weights())
		ret = stat.StdErr(std, float64(len(gc.wv)))
	case "entropy":
		if len(gc.wv) == 0 {
			return nil
		}
		ret = stat.Entropy(gc.wv.Values())
	case "mode":
		if len(gc.wv) == 0 {
			return nil
		}
		ret, _ = stat.Mode(gc.wv.Values(), gc.wv.Weights())
	default:
		return nil
	}
	if ret != ret { // NaN
		return nil
	}
	return ret
}

func (gc *GroupColumnContainer) Append(value any) error {
	var v, w float64
	if arr, ok := value.([]any); ok {
		if len(arr) > 0 {
			if f, err := util.ToFloat64(arr[0]); err != nil {
				return err
			} else {
				v = f
			}
		}
		if len(arr) > 1 {
			for _, elm := range arr[1:] {
				switch f := elm.(type) {
				case Weight:
					w = float64(f)
				}
			}
		}
	} else if f, err := util.ToFloat64(value); err != nil {
		return err
	} else {
		v, w = f, 1.0
	}
	gc.wv = append(gc.wv, nums.WeightedFloat64ValueWeight(v, w))
	return nil
}

// count, avg, rss, rms
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
	case "count":
		ret = float64(gc.count)
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
	if gc.name == "count" {
		gc.count++
		return nil
	}

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
				cols = []*api.Column{columns[nth+1]}
				cols = append(cols, columns[1:nth+1]...)
			}
			if len(columns) >= nth+2 {
				cols = append(cols, columns[nth+2:]...)
			}
			node.task.SetResultColumns(cols)
		}
		newKey := val[nth]
		newVal := append(val[0:nth], val[nth+1:]...)
		return inflight.ReplaceKeyValue(newKey, newVal), nil
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
				ret[i] = NewRecordVars(v[0], v[1], inflight.vars)
			} else {
				ret[i] = NewRecordVars(v[0], v[1:], inflight.vars)
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
		node.task.SetResultColumns(append([]*api.Column{api.ColumnTypeOf(newKey)}, node.task.ResultColumns()...))
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
	return rec.ReplaceKeyValue(newKey, newVal), nil
}

func (node *Node) fmMapKey(newKey any) (any, error) {
	if _, ok := node.GetValue("isFirst"); !ok {
		node.SetValue("isFirst", true)
		cols := node.task.ResultColumns()
		if len(cols) > 0 {
			node.task.SetResultColumns(append([]*api.Column{api.ColumnTypeOf(newKey)}, node.task.ResultColumns()[1:]...))
		}
	}
	rec := node.Inflight()
	if rec == nil {
		return nil, nil
	}
	return rec.ReplaceKey(newKey), nil
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
			newCol := api.ColumnTypeOf(newValue)
			newCol.Name = columnName
			head := cols[0 : idx+1]
			tail := cols[idx+1:]
			updateCols := []*api.Column{}
			updateCols = append(updateCols, head...)
			updateCols = append(updateCols, newCol)
			updateCols = append(updateCols, tail...)
			node.task.SetResultColumns(updateCols)
		} else {
			for i := len(cols); i < idx; i++ {
				newCol := &api.Column{}
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
		return NewRecordVars(inflight.key, updateVal, inflight.vars), nil
	default:
		if idx <= 0 {
			return inflight.ReplaceValue([]any{newValue, val}), nil
		} else {
			return inflight.ReplaceValue([]any{val, newValue}), nil
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
		updateCols := []*api.Column{cols[0]}
		for _, idx := range includes {
			if idx+1 < len(cols) {
				updateCols = append(updateCols, cols[idx+1])
			}
		}
		node.task.SetResultColumns(updateCols)
	}

	val := inflight.value.([]any)
	updateVal := []any{}
	for _, idx := range includes {
		updateVal = append(updateVal, val[idx])
	}
	return inflight.ReplaceValue(updateVal), nil
}

func (node *Node) fmMapValue(idx int, newValue any, opts ...any) (any, error) {
	inflight := node.Inflight()
	if inflight == nil {
		return nil, nil
	}
	wherePredicate := true
	for _, opt := range opts {
		switch v := opt.(type) {
		case *NullValue:
			if newValue == nil {
				newValue = v.altValue
			}
		case WherePredicate:
			wherePredicate = bool(v)
		}
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
					if idx+1 >= len(cols) {
						for i := len(cols); i <= idx+1; i++ {
							cols = append(cols, &api.Column{Name: fmt.Sprintf("column%d", i)})
						}
					}
					cols[idx+1].Name = newName
				}
			}
		}
		if wherePredicate {
			val[idx] = newValue
		}
		return inflight.ReplaceValue(val), nil
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
		if wherePredicate {
			val = newValue
		}
		return inflight.ReplaceValue(val), nil
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
	prevTime    time.Time
	isPrevNull  bool
	abs         bool
	nonNegative bool
}

func (df *diff) diff(node *Node, idx int, value any, opts []any) (any, error) {
	if value == nil {
		df.isPrevNull = true
		return node.fmMapValue(idx, nil, opts...)
	}
	var tv time.Time
	switch val := value.(type) {
	case *time.Time:
		tv = *val
		goto timediff
	case time.Time:
		tv = val
		goto timediff
	default:
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
timediff:
	zero := time.Time{}
	if tv == zero {
		df.isPrevNull = true
		return node.fmMapValue(idx, nil, opts...)
	}
	if df.isPrevNull {
		df.prevTime = tv
		df.isPrevNull = false
		return node.fmMapValue(idx, nil, opts...)
	} else {
		ret := tv.Sub(df.prevTime)
		if df.abs && ret < 0 {
			ret = ret * -1
		} else if df.nonNegative && ret < 0 {
			ret = 0
		}
		df.prevTime = tv
		df.isPrevNull = false
		return node.fmMapValue(idx, int64(ret), opts...)
	}
}

func (node *Node) fmMapKalman(idx int, value any, opts ...any) (any, error) {
	var fv float64
	if f, err := util.ToFloat64(value); err != nil {
		return nil, err
	} else {
		fv = f
	}
	var kf *filterKalman
	if v, ok := node.GetValue("mapKalman"); ok {
		kf = v.(*filterKalman)
	} else {
		kf = &filterKalman{
			ts: time.Now(),
		}
		var simpleModelConfig *models.SimpleModelConfig
		for _, opt := range opts {
			if sm, ok := opt.(*kalmanModel); ok {
				simpleModelConfig = &models.SimpleModelConfig{
					InitialVariance:     sm.initialVariance,
					ProcessVariance:     sm.processVariance,
					ObservationVariance: sm.observationVariance,
				}
				break
			}
		}
		if simpleModelConfig == nil {
			simpleModelConfig = &models.SimpleModelConfig{
				InitialVariance:     2.0,
				ProcessVariance:     0.01,
				ObservationVariance: 2.0,
			}
		}
		model := models.NewSimpleModel(kf.ts, fv, *simpleModelConfig)
		kf.model = model
		kf.filter = kalman.NewKalmanFilter(kf.model)
		node.SetValue("mapKalman", kf)
	}
	kf.ts = kf.ts.Add(time.Second)
	kf.filter.Update(kf.ts, kf.model.NewMeasurement(fv))
	newVal := kf.model.Value(kf.filter.State())
	return node.fmMapValue(idx, newVal, opts...)
}

type filterKalman struct {
	ts     time.Time
	model  *models.SimpleModel
	filter *kalman.KalmanFilter
}

type kalmanModel struct {
	typ                 string
	initialVariance     float64 // entries for the diagonal of P_0
	processVariance     float64 // entries for the diagonal of Q_k
	observationVariance float64 // entries for the diagonal of R_k
}

func (node *Node) fmKalmanModel(args ...any) (*kalmanModel, error) {
	ret := &kalmanModel{}
	idx := 0
	if str, ok := args[idx].(string); ok {
		// model name
		// expect "simple"
		ret.typ = str
		idx++
	}
	variances := []float64{}
	for _, arg := range args[idx:] {
		f, err := util.ToFloat64(arg)
		if err != nil {
			return nil, err
		}
		variances = append(variances, f)
	}
	if len(variances) > 0 {
		ret.initialVariance = variances[0]
	}
	if len(variances) > 1 {
		ret.processVariance = variances[1]
	}
	if len(variances) > 2 {
		ret.observationVariance = variances[2]
	}
	return ret, nil
}

func (node *Node) fmMapAvg(idx int, value any, opts ...any) (any, error) {
	var fv float64
	if f, err := util.ToFloat64(value); err != nil {
		return nil, err
	} else {
		fv = f
	}
	var af *filterAvg
	if v, ok := node.GetValue("mapAvg"); ok {
		af = v.(*filterAvg)
	} else {
		af = &filterAvg{}
		node.SetValue("mapAvg", af)
	}

	af.k += 1
	if af.k == 0 {
		af.prevAvg = fv
	} else {
		alpha := 1 - 1/af.k
		af.prevAvg = alpha*af.prevAvg + (1-alpha)*fv
	}
	return node.fmMapValue(idx, af.prevAvg, opts...)
}

type filterAvg struct {
	prevAvg float64
	k       float64
}

func (node *Node) fmMapMovAvg(idx int, value any, window int, opts ...any) (any, error) {
	if window <= 1 {
		return 0, ErrArgs("MAP_MOVAVG", 1, "window should be larger than 1")
	}
	var fv *float64
	if f, err := util.ToFloat64(value); err == nil {
		fv = &f
	}
	var ma *filterMovAvg
	if v, ok := node.GetValue("movavg"); ok {
		ma = v.(*filterMovAvg)
	} else {
		ma = &filterMovAvg{}
		for _, opt := range opts {
			switch v := opt.(type) {
			case NoWait:
				ma.noWait = bool(v)
			}
		}
		node.SetValue("movavg", ma)
	}
	ma.elements = append(ma.elements, fv)
	if len(ma.elements) >= window {
		ma.elements = ma.elements[len(ma.elements)-window:]
	} else {
		if !ma.noWait {
			return node.fmMapValue(idx, nil, opts...)
		}
	}
	countMember := len(ma.elements)
	sum := 0.0
	countNil := 0
	for _, e := range ma.elements {
		if e != nil {
			sum += *e
		} else {
			countNil++
		}
	}
	if countNil == countMember {
		return node.fmMapValue(idx, nil, opts...)
	} else {
		ret := sum / float64(countMember-countNil)
		return node.fmMapValue(idx, ret, opts...)
	}
}

type filterMovAvg struct {
	elements []*float64
	noWait   bool
}

type NoWait bool

func (node *Node) fmNoWait(flag bool) NoWait {
	return NoWait(flag)
}

func (node *Node) fmMapLowPass(idx int, value any, alpha float64, opts ...any) (any, error) {
	var fv float64
	if f, err := util.ToFloat64(value); err != nil {
		return nil, err
	} else {
		fv = f
	}
	var lpf *lowPassFilter
	if v, ok := node.GetValue("mapLpf"); ok {
		lpf = v.(*lowPassFilter)
		lpf.prev = (1-alpha)*lpf.prev + alpha*fv
		return node.fmMapValue(idx, lpf.prev, opts...)
	} else {
		if alpha <= 0 || alpha >= 1 {
			return nil, errors.New("MAP_LOWPASS() should have 0 < alpha < 1 ")
		}
		lpf = &lowPassFilter{alpha: alpha}
		node.SetValue("mapLpf", lpf)
		lpf.prev = fv
		return node.fmMapValue(idx, lpf.prev, opts...)
	}
}

type lowPassFilter struct {
	prev  float64
	alpha float64
}

func (node *Node) fmGeoDistance(idx int, pt any, opts ...any) (any, error) {
	var loc *nums.LatLon
	switch v := pt.(type) {
	case *nums.LatLon:
		loc = v
	case *nums.SingleLatLon:
		loc = v.LatLon()
	default:
		return node.fmMapValue(idx, 0, opts...)
	}

	if loc == nil || (loc.Lat == 0 && loc.Lon == 0) {
		return node.fmMapValue(idx, 0, opts...)
	}
	var gd *geoDistance
	if v, ok := node.GetValue("geodistance"); ok {
		gd = v.(*geoDistance)
	} else {
		gd = &geoDistance{}
		node.SetValue("geodistance", gd)
	}
	var prev *nums.LatLon
	if gd.prev == nil {
		gd.prev = loc
		return node.fmMapValue(idx, 0, opts...)
	} else {
		prev, gd.prev = gd.prev, loc
		return node.fmMapValue(idx, loc.Distance(prev), opts...)
	}
}

type geoDistance struct {
	prev *nums.LatLon
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

	contentType string
	headers     map[string]string
}

func (doer *HttpDoer) Do(node *Node) error {
	for _, str := range doer.args {
		k, v, ok := strings.Cut(str, ":")
		if ok {
			k, v = strings.TrimSpace(k), strings.TrimSpace(v)
			doer.headers[k] = v
			if strings.ToLower(k) == "content-type" {
				if ct, _, ok := strings.Cut(v, ";"); ok {
					doer.contentType = strings.TrimSpace(ct)
				} else {
					doer.contentType = v
				}
			}
		}
	}

	var body io.Reader
	if doer.method == "POST" && doer.content != nil {
		buff := &bytes.Buffer{}
		if doer.contentType == "" {
			doer.headers["Content-Type"] = "text/csv" // default
			csvEnc := gocsv.NewWriter(buff)
			switch v := doer.content.(type) {
			case []float64:
				arr := make([]string, len(v))
				for i, a := range v {
					arr[i] = strconv.FormatFloat(a, 'f', -1, 64)
				}
				csvEnc.Write(arr)
			case float64:
				csvEnc.Write([]string{strconv.FormatFloat(v, 'f', -1, 64)})
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
		} else {
			switch v := doer.content.(type) {
			case string:
				buff.WriteString(v)
			case any:
				buff.WriteString(fmt.Sprintf("%v", v))
			}
		}
		body = buff
	}

	req, err := http.NewRequestWithContext(node.task.ctx, doer.method, doer.url, body)
	if err != nil {
		return err
	}

	for k, v := range doer.headers {
		req.Header.Add(k, v)
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

	replyLength := int(resp.ContentLength)
	if replyLength > 500 {
		replyLength = 500
	}
	replyBuff := make([]byte, replyLength)
	resp.Body.Read(replyBuff)
	reply := string(replyBuff)

	if resp.StatusCode >= 400 {
		node.task.LogWarn("http-doer", doer.method, doer.url, resp.Status, reply)
	} else if resp.StatusCode >= 300 {
		node.task.LogInfo("http-doer", doer.method, doer.url, resp.Status, reply)
	} else {
		node.task.LogDebug("http-doer", doer.method, doer.url, resp.Status, reply)
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
	ret.method = strings.ToUpper(method)
	ret.url = url
	ret.args = args
	ret.content = body
	ret.headers = map[string]string{}
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
		if inflight == nil {
			return nil, nil
		}
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
			newCols := api.Columns{cols[0]}
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
				newCols = append(newCols, &api.Column{Name: "header"})
			}
			newCols = append(newCols, &api.Column{Name: fmt.Sprintf("column%d", len(newCols)-1)})
			node.task.SetResultColumns(newCols)
		case any:
			newCols := api.Columns{cols[0]}
			if tr.header {
				tr.headerNames = []string{fmt.Sprintf("%v", vals)}
				newCols = append(newCols, &api.Column{Name: fmt.Sprintf("column%d", len(newCols)-1)})
			}
			newCols = append(newCols, &api.Column{Name: "column1"})
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
	if inflight == nil || inflight.Value() == nil {
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
