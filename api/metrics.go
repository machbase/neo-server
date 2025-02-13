package api

import (
	"encoding/json"
	"expvar"
	"fmt"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/machbase/neo-server/v8/api/mgmt"
	"github.com/machbase/neo-server/v8/mods/util/metric"
)

// 2m1s   -> 1s  : 2 * 60 = 120
// 60m30s -> 30s : 2 * 60 = 120
// 10h5m  -> 5m  : 10 * 12 = 120
var MetricTimeFrames = []string{"2m1s", "60m30s", "10h5m"}

const MetricQueryRowsMax = 120

var (
	// sessions
	metricConns          = metric.NewExpVarIntCounter("machbase:session:conns", MetricTimeFrames...)
	metricConnsInUse     = metric.NewExpVarIntCounter("machbase:session:conns_in_use", MetricTimeFrames...)
	metricConnWaitTime   = metric.NewExpVarDurationGauge("machbase:session:conn_wait_time", MetricTimeFrames...)
	metricConnUseTime    = metric.NewExpVarDurationGauge("machbase:session:conn_use_time", MetricTimeFrames...)
	metricStmts          = metric.NewExpVarIntCounter("machbase:session:stmts", MetricTimeFrames...)
	metricStmtsInUse     = metric.NewExpVarIntCounter("machbase:session:stmts_in_use", MetricTimeFrames...)
	metricAppenders      = metric.NewExpVarIntCounter("machbase:session:appenders", MetricTimeFrames...)
	metricAppendersInUse = metric.NewExpVarIntCounter("machbase:session:appenders_in_use", MetricTimeFrames...)

	// query api
	metricQueryCount         = metric.NewExpVarIntCounter("machbase:query:count", MetricTimeFrames...)
	metricQueryExecuteElapse = metric.NewExpVarDurationGauge("machbase:query:execute_elapse", MetricTimeFrames...)
	metricQueryLimitWait     = metric.NewExpVarDurationGauge("machbase:query:limit_wait", MetricTimeFrames...)
	metricQueryFetchElapse   = metric.NewExpVarDurationGauge("machbase:query:fetch_elapse", MetricTimeFrames...)

	// longest query
	metricQueryHwmSqlText       = expvar.NewString("machbase:query:hwm_sql_text")
	metricQueryHwmSqlArgs       = expvar.NewString("machbase:query:hwm_sql_args")
	metricQueryHwmElapse        = expvar.NewInt("machbase:query:hwm_elapse")
	metricQueryHwmExecuteElapse = expvar.NewInt("machbase:query:hwm_execute_elapse")
	metricQueryHwmLimitWait     = expvar.NewInt("machbase:query:hwm_limit_wait")
	metricQueryHwmFetchElapse   = expvar.NewInt("machbase:query:hwm_fetch_elapse")
)

func init() {
	// Example for internal metrics
	numGoRoutine := metric.NewExpVarIntGauge("go:num_goroutine", MetricTimeFrames...)
	numCGoCall := metric.NewExpVarIntGauge("go:num_cgo_call", MetricTimeFrames...)
	// goAlloc := metric.NewExpVarIntGauge("go:alloc", MetricTimeFrames...)
	// goAllocTotal := metric.NewExpVarIntGauge("go:alloc_total", MetricTimeFrames...)

	go func() {
		for range time.Tick(100 * time.Millisecond) {
			// m := &runtime.MemStats{}
			// runtime.ReadMemStats(m)
			// goAlloc.Add(int64(float64(m.Alloc) / 1000000))
			// goAllocTotal.Add(int64(float64(m.TotalAlloc) / 1000000))
			numGoRoutine.Add(int64(runtime.NumGoroutine()))
			numCGoCall.Add(runtime.NumCgoCall())
		}
	}()
}

func AllocConn(connWaitTime time.Duration) {
	metricConns.Add(1)
	metricConnsInUse.Add(1)
	metricConnWaitTime.Add(connWaitTime)
}

func FreeConn(connUseTime time.Duration) {
	metricConnsInUse.Add(-1)
	metricConnUseTime.Add(connUseTime)
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

var RawConns func() int

func StatzSnapshot() *mgmt.Statz {
	ret := &mgmt.Statz{}
	ret.Conns = metricConns.Value()
	ret.ConnsInUse = int32(metricConnsInUse.Value())
	ret.ConnWaitTime = uint64(metricConnWaitTime.Value())
	ret.ConnUseTime = uint64(metricConnUseTime.Value())
	ret.Stmts = metricStmts.Value()
	ret.StmtsInUse = int32(metricStmtsInUse.Value())
	ret.Appenders = metricAppenders.Value()
	ret.AppendersInUse = int32(metricAppendersInUse.Value())
	if RawConns != nil {
		ret.RawConns = int32(RawConns())
	}

	elapseAvgMax := func(p *metric.ExpVarMetric[time.Duration]) (uint64, uint64) {
		if mm, ok := p.Metric().(metric.MultiTimeseries); ok {
			b, _ := json.Marshal(mm[0])
			m := map[string]interface{}{}
			json.Unmarshal(b, &m)
			total := m["total"].(map[string]interface{})
			max := int64(total["max"].(float64))
			avg := int64(total["avg"].(float64))
			return uint64(avg), uint64(max)
		}
		return 0, 0
	}
	ret.QueryExecAvg, ret.QueryExecHwm = elapseAvgMax(metricQueryExecuteElapse)
	ret.QueryWaitAvg, ret.QueryWaitHwm = elapseAvgMax(metricQueryLimitWait)
	ret.QueryFetchAvg, ret.QueryFetchHwm = elapseAvgMax(metricQueryFetchElapse)
	ret.QueryHwmSql = metricQueryHwmSqlText.Value()
	ret.QueryHwmSqlArg = metricQueryHwmSqlArgs.Value()
	ret.QueryHwm = uint64(metricQueryHwmElapse.Value())
	ret.QueryHwmExec = uint64(metricQueryHwmExecuteElapse.Value())
	ret.QueryHwmWait = uint64(metricQueryHwmLimitWait.Value())
	ret.QueryHwmFetch = uint64(metricQueryHwmFetchElapse.Value())

	return ret
}

func ResetQueryStatz() {
	queryElapseHwm.Store(0)
}

type QueryStatzResult struct {
	Err  error
	Cols []*Column
	Rows []*QueryStatzRow
}

type QueryStatzRow struct {
	Timestamp time.Time
	Values    []any
}

func QueryStatz(interval time.Duration, filter func(key string, value metric.ExpVar) bool) *QueryStatzResult {
	ret := &QueryStatzResult{}
	metricKeys := []string{}
	expvar.Do(func(kv expvar.KeyValue) {
		var value metric.ExpVar
		if v, ok := kv.Value.(metric.ExpVar); !ok {
			return
		} else {
			value = v
		}
		if value == nil {
			return
		}
		if filter != nil && !filter(kv.Key, value) {
			return
		}
		var cols []*Column
		switch value.MetricType() {
		case "c":
			cols = []*Column{MakeColumnInt64(kv.Key)}
		case "g":
			cols = []*Column{
				MakeColumnDouble(kv.Key + "_avg"),
				MakeColumnDouble(kv.Key + "_min"),
				MakeColumnDouble(kv.Key + "_max"),
			}
		case "h":
			cols = []*Column{
				MakeColumnDouble(kv.Key + "_p50"),
				MakeColumnDouble(kv.Key + "_p90"),
				MakeColumnDouble(kv.Key + "_p99"),
			}
		default:
			cols = []*Column{MakeColumnString(kv.Key)}
		}
		ret.Cols = append(ret.Cols, cols...)
		metricKeys = append(metricKeys, kv.Key)
	})
	if len(ret.Cols) == 0 {
		ret.Err = fmt.Errorf("no metrics found")
		return ret
	}
	slices.SortFunc(ret.Cols, func(a, b *Column) int {
		return strings.Compare(a.Name, b.Name)
	})
	rowsCount := MetricQueryRowsMax
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
	for _, metricKey := range metricKeys {
		val := expvar.Get(metricKey).(metric.ExpVar)
		met := val.Metric()
		for r := 0; r < rowsCount; r++ {
			var colIdx = colIdxOffset
			var row = ret.Rows[r]
			var timeseries *metric.Timeseries
			var valueMetric metric.Metric

			if mts, ok := met.(metric.MultiTimeseries); ok {
				for _, ts := range mts {
					if ts.Interval() == interval {
						timeseries = ts
					}
				}
			} else if ts, ok := met.(*metric.Timeseries); ok {
				timeseries = ts
			} else {
				valueMetric = met
			}

			if timeseries != nil {
				if samples := timeseries.Samples(); r < len(samples) {
					valueMetric = samples[r]
				}
			}

			if valueMetric != nil {
				if counter, ok := valueMetric.(*metric.Counter); ok {
					row.Values[colIdx] = counter.Value()
					colIdx++
				} else if gauge, ok := valueMetric.(*metric.Gauge); ok {
					g := gauge.Value()
					row.Values[colIdx] = g["avg"]
					colIdx++
					row.Values[colIdx] = g["max"]
					colIdx++
					row.Values[colIdx] = g["min"]
					colIdx++
				} else if histogram, ok := valueMetric.(*metric.Histogram); ok {
					h := histogram.Value()
					row.Values[colIdx] = h["p50"]
					colIdx++
					row.Values[colIdx] = h["p90"]
					colIdx++
					row.Values[colIdx] = h["p99"]
					colIdx++
				}
			} else {
				row.Values[colIdx] = nil
				colIdx++
			}
			if r == rowsCount-1 {
				colIdxOffset = colIdx
			}
		}
	}
	return ret
}
