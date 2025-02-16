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
	"github.com/machbase/neo-server/v8/mods/util/glob"
	"github.com/machbase/neo-server/v8/mods/util/metric"
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
		for range time.Tick(MetricMeasurePeriod) {
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
		} else if v, ok := kv.Value.(metric.ExpVar); ok {
			switch v.MetricType() {
			case "c":
				if pass, order := filter(key); pass {
					metricQueryKeys = append(metricQueryKeys, MetricQueryKey{key: kv.Key, column: MakeColumnInt64(kv.Key), valueType: v.ValueType(), order: order})
				}
			case "g":
				for _, field := range []string{"avg", "max", "min"} {
					subKey := fmt.Sprintf("%s_%s", kv.Key, field)
					if pass, order := filter(subKey); pass {
						metricQueryKeys = append(metricQueryKeys, MetricQueryKey{key: kv.Key, field: field, column: MakeColumnDouble(subKey), valueType: v.ValueType(), order: order})
					}
				}
			case "h":
				for _, field := range []string{"p50", "p90", "p99"} {
					subKey := fmt.Sprintf("%s_%s", kv.Key, field)
					if pass, order := filter(subKey); pass {
						metricQueryKeys = append(metricQueryKeys, MetricQueryKey{key: kv.Key, field: field, column: MakeColumnDouble(subKey), valueType: v.ValueType(), order: order})
					}
				}
			default:
				if pass, order := filter(key); pass {
					metricQueryKeys = append(metricQueryKeys, MetricQueryKey{key: kv.Key, column: MakeColumnString(kv.Key), valueType: v.ValueType(), order: order})
				}
			}
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
						row.Values[colIdx] = g[queryKey.field]
						colIdx++
					} else if histogram, ok := valueMetric.(*metric.Histogram); ok {
						h := histogram.Value()
						row.Values[colIdx] = h[queryKey.field]
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
