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

const (
	MetricShortTerm = 10 * time.Second
	MetricMidTerm   = time.Minute
	MetricLongTerm  = 15 * time.Minute
)

var MetricTimeFrames = []string{"20m10s", "2h1m", "30h15m"}

const MetricQueryRowsMax = 120

var (
	// session:conn
	metricConns        = metric.NewExpVarIntCounter("machbase:session:conn:count", MetricTimeFrames...)
	metricConnsCounter = metric.NewCounter()
	metricConnsInUse   = metric.NewExpVarIntCounter("machbase:session:conn:in_use", MetricTimeFrames...)
	metricConnWaitTime = metric.NewExpVarDurationGauge("machbase:session:conn:wait_time", MetricTimeFrames...)
	metricConnUseTime  = metric.NewExpVarDurationGauge("machbase:session:conn:use_time", MetricTimeFrames...)

	// session:stmt
	metricStmts        = metric.NewExpVarIntCounter("machbase:session:stmt:count", MetricTimeFrames...)
	metricStmtsCounter = metric.NewCounter()
	metricStmtsInUse   = metric.NewExpVarIntCounter("machbase:session:stmt:in_use", MetricTimeFrames...)

	// session:appender
	metricAppenders        = metric.NewExpVarIntCounter("machbase:session:append:count", MetricTimeFrames...)
	metricAppendersCounter = metric.NewCounter()
	metricAppendersInUse   = metric.NewExpVarIntCounter("machbase:session:append:in_use", MetricTimeFrames...)

	// session:query
	metricQueryCount         = metric.NewExpVarIntCounter("machbase:session:query:count", MetricTimeFrames...)
	metricQueryExecuteElapse = metric.NewExpVarDurationGauge("machbase:session:query:exec_time", MetricTimeFrames...)
	metricQueryLimitWait     = metric.NewExpVarDurationGauge("machbase:session:query:wait_time", MetricTimeFrames...)
	metricQueryFetchElapse   = metric.NewExpVarDurationGauge("machbase:session:query:fetch_time", MetricTimeFrames...)

	// longest query
	metricQueryHwmSqlText       = expvar.NewString("machbase:session:query:hwm:sql_text")
	metricQueryHwmSqlArgs       = expvar.NewString("machbase:session:query:hwm:sql_args")
	metricQueryHwmElapse        = expvar.NewInt("machbase:session:query:hwm:elapse")
	metricQueryHwmExecuteElapse = expvar.NewInt("machbase:session:query:hwm:exec_time")
	metricQueryHwmLimitWait     = expvar.NewInt("machbase:session:query:hwm:wait_time")
	metricQueryHwmFetchElapse   = expvar.NewInt("machbase:session:query:hwm:fetch_time")
)

func init() {
	numGoRoutine := metric.NewExpVarIntGauge("go:num_goroutine", MetricTimeFrames...)
	numCGoCall := metric.NewExpVarIntGauge("go:num_cgo_call", MetricTimeFrames...)

	go func() {
		for range time.Tick(100 * time.Millisecond) {
			numGoRoutine.Add(int64(runtime.NumGoroutine()))
			numCGoCall.Add(runtime.NumCgoCall())
			metricConnsInUse.Add(int64(metricConnsCounter.(*metric.Counter).Value()))
			metricStmtsInUse.Add(int64(metricStmtsCounter.(*metric.Counter).Value()))
			metricAppendersInUse.Add(int64(metricAppendersCounter.(*metric.Counter).Value()))
		}
	}()
}

func AllocConn(connWaitTime time.Duration) {
	metricConns.Add(1)
	metricConnsCounter.Add(1)
	metricConnWaitTime.Add(connWaitTime)
}

func FreeConn(connUseTime time.Duration) {
	metricConnsCounter.Add(-1)
	metricConnUseTime.Add(connUseTime)
}

func AllocStmt() {
	metricStmts.Add(1)
	metricStmtsCounter.Add(1)
}

func FreeStmt() {
	metricStmtsCounter.Add(-1)
}

func AllocAppender() {
	metricAppenders.Add(1)
	metricAppendersCounter.Add(1)
}

func FreeAppender() {
	metricAppendersCounter.Add(-1)
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

func StatzSnapshot() *mgmt.Statz {
	ret := &mgmt.Statz{}
	ret.Conns = metricConns.Value()
	ret.ConnsInUse = int32(metricConnsCounter.(*metric.Counter).Value())
	ret.ConnWaitTime = uint64(metricConnWaitTime.Value())
	ret.ConnUseTime = uint64(metricConnUseTime.Value())
	ret.Stmts = metricStmts.Value()
	ret.StmtsInUse = int32(metricStmtsCounter.(*metric.Counter).Value())
	ret.Appenders = metricAppenders.Value()
	ret.AppendersInUse = int32(metricAppendersCounter.(*metric.Counter).Value())
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

func QueryStatz(interval time.Duration, filter func(kv expvar.KeyValue) bool) *QueryStatzResult {
	return QueryStatzRows(interval, MetricQueryRowsMax, filter)
}

func QueryStatzRows(interval time.Duration, rowsCount int, filter func(kv expvar.KeyValue) bool) *QueryStatzResult {
	ret := &QueryStatzResult{}
	metricKeys := []string{}
	expvar.Do(func(kv expvar.KeyValue) {
		if filter != nil && !filter(kv) {
			return
		}
		if _, ok := kv.Value.(*expvar.Int); ok {
			ret.Cols = append(ret.Cols, MakeColumnInt64(kv.Key))
			ret.ValueTypes = append(ret.ValueTypes, "i")
			metricKeys = append(metricKeys, kv.Key)
		} else if _, ok := kv.Value.(*expvar.String); ok {
			ret.Cols = append(ret.Cols, MakeColumnString(kv.Key))
			ret.ValueTypes = append(ret.ValueTypes, "s")
			metricKeys = append(metricKeys, kv.Key)
		} else if _, ok := kv.Value.(*expvar.Float); ok {
			ret.Cols = append(ret.Cols, MakeColumnDouble(kv.Key))
			ret.ValueTypes = append(ret.ValueTypes, "f")
			metricKeys = append(metricKeys, kv.Key)
		} else if v, ok := kv.Value.(metric.ExpVar); ok {
			switch v.MetricType() {
			case "c":
				ret.Cols = append(ret.Cols, MakeColumnInt64(kv.Key))
				ret.ValueTypes = append(ret.ValueTypes, v.ValueType())
				metricKeys = append(metricKeys, kv.Key)
			case "g":
				ret.Cols = append(ret.Cols, MakeColumnDouble(kv.Key+"_avg"), MakeColumnDouble(kv.Key+"_min"), MakeColumnDouble(kv.Key+"_max"))
				ret.ValueTypes = append(ret.ValueTypes, v.ValueType(), v.ValueType(), v.ValueType())
				metricKeys = append(metricKeys, kv.Key)
			case "h":
				ret.Cols = append(ret.Cols, MakeColumnDouble(kv.Key+"_p50"), MakeColumnDouble(kv.Key+"_p90"), MakeColumnDouble(kv.Key+"_p99"))
				ret.ValueTypes = append(ret.ValueTypes, v.ValueType(), v.ValueType(), v.ValueType())
				metricKeys = append(metricKeys, kv.Key)
			default:
				ret.Cols = append(ret.Cols, MakeColumnString(kv.Key))
				ret.ValueTypes = append(ret.ValueTypes, v.ValueType())
				metricKeys = append(metricKeys, kv.Key)
			}
		}
	})
	if len(ret.Cols) == 0 {
		ret.Err = fmt.Errorf("no metrics found")
		return ret
	}
	slices.SortFunc(ret.Cols, func(a, b *Column) int {
		return strings.Compare(a.Name, b.Name)
	})

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
	for _, metricKey := range metricKeys {
		kv := expvar.Get(metricKey)
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
		} else if val, ok := kv.(metric.ExpVar); ok {
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
	}
	return ret
}
