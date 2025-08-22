package api

import (
	"bytes"
	"encoding/json"
	"expvar"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	"github.com/OutOfBedlam/metric"
	"github.com/machbase/neo-server/v8/api/mgmt"
	"github.com/machbase/neo-server/v8/mods/util/glob"
)

const (
	MetricShortTerm = 1 * time.Minute
	MetricMidTerm   = 5 * time.Minute
	MetricLongTerm  = 15 * time.Minute
)

const MetricMeasurePeriod = 500 * time.Millisecond

var MetricTimeFrames = []string{"2h1m", "10h5m", "30h15m"}

const MetricQueryRowsMax = 120

var (
	// session:conn
	metricConnsInUse atomic.Int64

	// session:stmt
	metricStmts      atomic.Int64
	metricStmtsInUse atomic.Int64

	// session:appender
	metricAppenders      atomic.Int64
	metricAppendersInUse atomic.Int64

	// session:query
	metricQueryCount atomic.Int64

	// longest query
	metricQueryHwmSqlText       = expvar.NewString("machbase:session:query:hwm:sql_text")
	metricQueryHwmSqlArgs       = expvar.NewString("machbase:session:query:hwm:sql_args")
	metricQueryHwmElapse        = expvar.NewInt("machbase:session:query:hwm:elapse")
	metricQueryHwmExecuteElapse = expvar.NewInt("machbase:session:query:hwm:exec_time")
	metricQueryHwmLimitWait     = expvar.NewInt("machbase:session:query:hwm:wait_time")
	metricQueryHwmFetchElapse   = expvar.NewInt("machbase:session:query:hwm:fetch_time")
)

func AllocConn(connWaitTime time.Duration) {
	metricConnsInUse.Add(1)
	AddMetrics(metric.Measurement{Name: "session", Fields: []metric.Field{
		{Name: "conn:wait_time", Value: float64(connWaitTime), Unit: metric.UnitDuration, Type: metric.FieldTypeHistogram(10, 0.5, 0.99, 0.999)},
	}})
}

func FreeConn(connUseTime time.Duration) {
	metricConnsInUse.Add(-1)
	AddMetrics(metric.Measurement{Name: "session", Fields: []metric.Field{
		{Name: "conn:use_time", Value: float64(connUseTime), Unit: metric.UnitDuration, Type: metric.FieldTypeHistogram(10, 0.5, 0.99, 0.999)},
	}})
}

func AllocStmt() {
	metricStmts.Add(1)
	metricStmtsInUse.Add(1)
}

func FreeStmt() {
	metricStmtsInUse.Add(-1)
}

func AllocAppender() {
	metricAppenders.Add(1)
	metricAppendersInUse.Add(1)
}

func FreeAppender() {
	metricAppendersInUse.Add(-1)
}

func QueryExecTime(d time.Duration) {
	AddMetrics(metric.Measurement{Name: "session", Fields: []metric.Field{
		{Name: "query:exec_time", Value: float64(d), Unit: metric.UnitDuration, Type: metric.FieldTypeHistogram(10, 0.5, 0.99, 0.999)},
	}})
}

func QueryWaitTime(d time.Duration) {
	AddMetrics(metric.Measurement{Name: "session", Fields: []metric.Field{
		{Name: "query:wait_time", Value: float64(d), Unit: metric.UnitDuration, Type: metric.FieldTypeHistogram(10, 0.5, 0.99, 0.999)},
	}})
}

func QueryFetchTime(d time.Duration) {
	AddMetrics(metric.Measurement{Name: "session", Fields: []metric.Field{
		{Name: "query:fetch_time", Value: float64(d), Unit: metric.UnitDuration, Type: metric.FieldTypeHistogram(10, 0.5, 0.99, 0.999)},
	}})
}

var RawConns func() int

func ResetQueryStatz() {
	queryElapseHwm.Store(0)
}

type QueryStatzResult struct {
	Err        error
	Cols       []*Column
	Rows       []*QueryStatzRow
	ValueTypes []string
}

type QueryStatzRow struct {
	Timestamp time.Time
	Values    []any
}

func elapseAvgMax(key string) (uint64, uint64, int64) {
	if value, ok := expvar.Get(key).(metric.MultiTimeSeries); ok && len(value) > 0 {
		_, val := value[0].Last()
		prd, ok := val.(*metric.HistogramProduct)
		if !ok {
			return 0, 0, 0
		}
		values := prd.Values
		return uint64(values[0]), uint64(values[len(values)-1]), int64(prd.Count)
	}
	return 0, 0, 0
}

func StatzSnapshot() *mgmt.Statz {
	ret := &mgmt.Statz{}
	ret.ConnWaitTime, _, _ = elapseAvgMax("machbase:session:conn:wait_time")
	ret.ConnUseTime, _, ret.Conns = elapseAvgMax("machbase:session:conn:use_time")
	ret.ConnsInUse = int32(metricConnsInUse.Load())
	ret.Stmts = metricStmts.Load()
	ret.StmtsInUse = int32(metricStmtsInUse.Load())
	ret.Appenders = metricAppenders.Load()
	ret.AppendersInUse = int32(metricAppendersInUse.Load())
	if RawConns != nil {
		ret.RawConns = int32(RawConns())
	}
	ret.QueryExecAvg, ret.QueryExecHwm, _ = elapseAvgMax("machbase:session:query:exec_time")
	ret.QueryWaitAvg, ret.QueryWaitHwm, _ = elapseAvgMax("machbase:session:query:wait_time")
	ret.QueryFetchAvg, ret.QueryFetchHwm, _ = elapseAvgMax("machbase:session:query:fetch_time")
	ret.QueryHwmSql = metricQueryHwmSqlText.Value()
	ret.QueryHwmSqlArg = metricQueryHwmSqlArgs.Value()
	ret.QueryHwm = uint64(metricQueryHwmElapse.Value())
	ret.QueryHwmExec = uint64(metricQueryHwmExecuteElapse.Value())
	ret.QueryHwmWait = uint64(metricQueryHwmLimitWait.Value())
	ret.QueryHwmFetch = uint64(metricQueryHwmFetchElapse.Value())
	return ret
}

func QueryStatzFilter(filters []string) func(key string) (bool, int) {
	return func(key string) (bool, int) {
		if len(filters) == 0 {
			return true, 0
		} else {
			for idx, filter := range filters {
				if glob.IsGlob(filter) {
					if ok, _ := glob.Match(filter, key); ok {
						return true, idx
					}
				} else {
					if filter == key {
						return true, idx
					}
				}
			}
		}
		return false, 0
	}
}

type MetricQueryKey struct {
	key       string
	field     string
	column    *Column
	valueType string
	order     int
}

func QueryStatz(interval time.Duration, filter func(key string) (bool, int)) *QueryStatzResult {
	return QueryStatzRows(interval, MetricQueryRowsMax, filter)
}

func QueryStatzRows(interval time.Duration, rowsCount int, filter func(key string) (bool, int)) *QueryStatzResult {
	ret := &QueryStatzResult{}
	if filter == nil {
		filter = func(key string) (bool, int) { return true, 0 }
	}
	metricQueryKeys := []MetricQueryKey{}
	expvar.Do(func(kv expvar.KeyValue) {
		key := kv.Key
		if _, ok := kv.Value.(*expvar.Int); ok {
			if pass, order := filter(key); pass {
				metricQueryKeys = append(metricQueryKeys, MetricQueryKey{key: kv.Key, column: MakeColumnInt64(kv.Key), valueType: "i", order: order})
			}
		} else if _, ok := kv.Value.(*expvar.String); ok {
			if pass, order := filter(key); pass {
				metricQueryKeys = append(metricQueryKeys, MetricQueryKey{key: kv.Key, column: MakeColumnString(kv.Key), valueType: "s", order: order})
			}
		} else if _, ok := kv.Value.(*expvar.Float); ok {
			if pass, order := filter(key); pass {
				metricQueryKeys = append(metricQueryKeys, MetricQueryKey{key: kv.Key, column: MakeColumnDouble(kv.Key), valueType: "f", order: order})
			}
		} else if _, ok := kv.Value.(expvar.Func); ok {
			if pass, order := filter(key); pass {
				metricQueryKeys = append(metricQueryKeys, MetricQueryKey{key: kv.Key, column: MakeColumnString(kv.Key), valueType: "i", order: order})
			}
			// } else if v, ok := kv.Value.(metric.ExpVar); ok {
			// 	switch v.MetricType() {
			// 	case "c":
			// 		if pass, order := filter(key); pass {
			// 			metricQueryKeys = append(metricQueryKeys, MetricQueryKey{key: kv.Key, column: MakeColumnInt64(kv.Key), valueType: v.ValueType(), order: order})
			// 		}
			// 	case "g":
			// 		for _, field := range []string{"avg", "max", "min"} {
			// 			subKey := fmt.Sprintf("%s_%s", kv.Key, field)
			// 			if pass, order := filter(subKey); pass {
			// 				metricQueryKeys = append(metricQueryKeys, MetricQueryKey{key: kv.Key, field: field, column: MakeColumnDouble(subKey), valueType: v.ValueType(), order: order})
			// 			}
			// 		}
			// 	case "h":
			// 		for _, field := range []string{"p50", "p90", "p99"} {
			// 			subKey := fmt.Sprintf("%s_%s", kv.Key, field)
			// 			if pass, order := filter(subKey); pass {
			// 				metricQueryKeys = append(metricQueryKeys, MetricQueryKey{key: kv.Key, field: field, column: MakeColumnDouble(subKey), valueType: v.ValueType(), order: order})
			// 			}
			// 		}
			// 	default:
			// 		if pass, order := filter(key); pass {
			// 			metricQueryKeys = append(metricQueryKeys, MetricQueryKey{key: kv.Key, column: MakeColumnString(kv.Key), valueType: v.ValueType(), order: order})
			// 		}
			// 	}
		}
	})
	if len(metricQueryKeys) == 0 {
		ret.Err = fmt.Errorf("no metrics found")
		return ret
	}

	slices.SortFunc(metricQueryKeys, func(a, b MetricQueryKey) int {
		if a.order == b.order {
			return strings.Compare(a.key+a.field, b.key+b.field)
		}
		return a.order - b.order
	})

	for _, queryKey := range metricQueryKeys {
		ret.Cols = append(ret.Cols, queryKey.column)
		ret.ValueTypes = append(ret.ValueTypes, queryKey.valueType)
	}

	if rowsCount > MetricQueryRowsMax {
		rowsCount = MetricQueryRowsMax
	}

	now := time.Now()

	ret.Rows = make([]*QueryStatzRow, rowsCount)
	for r := 0; r < rowsCount; r++ {
		timestamp := now.Add(-1 * interval * time.Duration(r))
		ret.Rows[r] = &QueryStatzRow{
			Timestamp: timestamp,
			Values:    make([]any, len(ret.Cols)),
		}
	}

	colIdxOffset := 0
	for _, queryKey := range metricQueryKeys {
		kv := expvar.Get(queryKey.key)
		if val, ok := kv.(*expvar.Int); ok {
			for r := 0; r < rowsCount; r++ {
				ret.Rows[r].Values[colIdxOffset] = val.Value()
			}
			colIdxOffset++
		} else if val, ok := kv.(*expvar.String); ok {
			for r := 0; r < rowsCount; r++ {
				ret.Rows[r].Values[colIdxOffset] = val.Value()
			}
			colIdxOffset++
		} else if val, ok := kv.(*expvar.Float); ok {
			for r := 0; r < rowsCount; r++ {
				ret.Rows[r].Values[colIdxOffset] = val.Value()
			}
			colIdxOffset++
		} else if val, ok := kv.(expvar.Func); ok {
			for r := 0; r < rowsCount; r++ {
				ret.Rows[r].Values[colIdxOffset] = val()
			}
			colIdxOffset++
			// } else if val, ok := kv.(metric.ExpVar); ok {
			// 	met := val.Metric()
			// 	for r := 0; r < rowsCount; r++ {
			// 		var colIdx = colIdxOffset
			// 		var row = ret.Rows[r]
			// 		var timeseries *metric.Timeseries
			// 		var valueMetric metric.Metric

			// 		if mts, ok := met.(metric.MultiTimeseries); ok {
			// 			for _, ts := range mts {
			// 				if ts.Interval() == interval {
			// 					timeseries = ts
			// 				}
			// 			}
			// 		} else if ts, ok := met.(*metric.Timeseries); ok {
			// 			timeseries = ts
			// 		} else {
			// 			valueMetric = met
			// 		}

			// 		if timeseries != nil {
			// 			if samples := timeseries.Samples(); r < len(samples) {
			// 				valueMetric = samples[r]
			// 			}
			// 		}

			// 		if valueMetric != nil {
			// 			if counter, ok := valueMetric.(*metric.Counter); ok {
			// 				row.Values[colIdx] = counter.Value()
			// 				colIdx++
			// 			} else if gauge, ok := valueMetric.(*metric.Gauge); ok {
			// 				g := gauge.Value()
			// 				row.Values[colIdx] = g[queryKey.field]
			// 				colIdx++
			// 			} else if histogram, ok := valueMetric.(*metric.Histogram); ok {
			// 				h := histogram.Value()
			// 				row.Values[colIdx] = h[queryKey.field]
			// 				colIdx++
			// 			}
			// 		} else {
			// 			row.Values[colIdx] = nil
			// 			colIdx++
			// 		}
			// 		if r == rowsCount-1 {
			// 			colIdxOffset = colIdx
			// 		}
			// 	}
		}
	}
	return ret
}

var collector *metric.Collector
var prefix string = "machbase"

func StartMetrics() {
	collector = metric.NewCollector(2*time.Second,
		metric.WithSeriesListener("30min/10s", 10*time.Second, 180, onProduct),
		metric.WithExpvarPrefix(prefix+":"),
		metric.WithReceiverSize(20),
	)
	collector.AddInputFunc(collect_runtime)
	collector.AddInputFunc(collect_metrics)
	collector.Start()
}

func StopMetrics() {
	if collector == nil {
		return
	}
	collector.Stop()
	collector = nil
}

func AddMetricsFunc(f func() (metric.Measurement, error)) {
	if collector == nil {
		return
	}
	collector.AddInputFunc(f)
}

func AddMetrics(m metric.Measurement) {
	if collector == nil {
		return
	}
	collector.SendEvent(m)
}

func collect_runtime() (metric.Measurement, error) {
	m := metric.Measurement{Name: "runtime"}
	m.AddField(
		metric.Field{Name: "goroutines", Value: float64(runtime.NumGoroutine()), Unit: metric.UnitShort, Type: metric.FieldTypeGauge},
		// metric.Field{Name: "heap_inuse", Value: float64(runtime.NumGoroutine()), Unit: metric.UnitShort, Type: metric.FieldTypeGauge},
		metric.Field{Name: "cgo_call", Value: float64(runtime.NumCgoCall()), Unit: metric.UnitShort, Type: metric.FieldTypeGauge},
	)
	return m, nil
}

func collect_metrics() (metric.Measurement, error) {
	m := metric.Measurement{Name: "session"}
	m.AddField(
		metric.Field{Name: "conn:in_use", Value: float64(metricConnsInUse.Load()), Unit: metric.UnitShort, Type: metric.FieldTypeGauge},
		metric.Field{Name: "stmt:count", Value: float64(metricStmts.Load()), Unit: metric.UnitShort, Type: metric.FieldTypeGauge},
		metric.Field{Name: "stmt:in_use", Value: float64(metricStmtsInUse.Load()), Unit: metric.UnitShort, Type: metric.FieldTypeGauge},
		metric.Field{Name: "append:count", Value: float64(metricAppenders.Load()), Unit: metric.UnitShort, Type: metric.FieldTypeGauge},
		metric.Field{Name: "append:in_use", Value: float64(metricAppendersInUse.Load()), Unit: metric.UnitShort, Type: metric.FieldTypeGauge},
		metric.Field{Name: "query:count", Value: float64(metricQueryCount.Load()), Unit: metric.UnitShort, Type: metric.FieldTypeGauge},
	)
	return m, nil
}
func onProduct(tb metric.TimeBin, field metric.FieldInfo) {
	var result []any
	switch p := tb.Value.(type) {
	case *metric.CounterProduct:
		if p.Count == 0 {
			return // Skip zero counters
		}
		result = []any{
			map[string]any{
				"NAME":  fmt.Sprintf("%s:%s:%s", prefix, field.Measure, field.Name),
				"TIME":  tb.Time.UnixNano(),
				"VALUE": p.Value,
			},
		}
	case *metric.GaugeProduct:
		if p.Count == 0 {
			return // Skip zero gauges
		}
		result = []any{
			map[string]any{
				"NAME":  fmt.Sprintf("%s:%s:%s", prefix, field.Measure, field.Name),
				"TIME":  tb.Time.UnixNano(),
				"VALUE": p.Value,
			},
		}
	case *metric.MeterProduct:
		if p.Count == 0 {
			return // Skip zero meters
		}
		result = []any{
			map[string]any{
				"NAME":  fmt.Sprintf("%s:%s:%s:max", prefix, field.Measure, field.Name),
				"TIME":  tb.Time.UnixNano(),
				"VALUE": p.Max,
			},
			map[string]any{
				"NAME":  fmt.Sprintf("%s:%s:%s:avg", prefix, field.Measure, field.Name),
				"TIME":  tb.Time.UnixNano(),
				"VALUE": p.Sum / float64(p.Count),
			},
		}
	case *metric.HistogramProduct:
		if p.Count == 0 {
			return // Skip zero meters
		}
		result = append(result, map[string]any{
			"NAME":  fmt.Sprintf("%s:%s:%s", prefix, field.Measure, field.Name),
			"TIME":  tb.Time.UnixNano(),
			"VALUE": p.Count,
		})
		for i, x := range p.P {
			pct := fmt.Sprintf("%d", int(x*1000))
			if pct[len(pct)-1] == '0' {
				pct = pct[:len(pct)-1]
			}
			result = append(result, map[string]any{
				"NAME":  fmt.Sprintf("%s:%s:%s:p%s", prefix, field.Measure, field.Name, pct),
				"TIME":  tb.Time.UnixNano(),
				"VALUE": p.Values[i],
			})
		}
	default:
		fmt.Printf("metrics unknown type: %T\n", p)
		return
	}

	// temporary go routine: to avoid blocking by recursive http call : actual service calls and metric collecting calls
	go func(result []any) {
		out := &bytes.Buffer{}
		for _, m := range result {
			b, err := json.Marshal(m)
			if err != nil {
				fmt.Printf("metrics marshaling: %v\n", err)
				return
			}
			out.Write(b)
			out.Write([]byte("\n"))
		}
		out.Write([]byte("\n"))

		rsp, err := http.DefaultClient.Post(
			"http://127.0.0.1:5654/db/write/TAG",
			"application/x-ndjson", out)
		if err != nil {
			fmt.Printf("metrics sending: %v\n", err)
			return
		}
		defer rsp.Body.Close()
		if rsp.StatusCode != http.StatusOK {
			msg, _ := io.ReadAll(rsp.Body)
			fmt.Printf("metrics writing: %s\n", msg)
			return
		}
	}(result)
}
