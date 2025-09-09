package api

import (
	"context"
	"expvar"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	"github.com/machbase/neo-server/v8/api/mgmt"
	"github.com/machbase/neo-server/v8/mods/logging"
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

var metricLog = logging.GetLog("statz")

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
		{Name: "conn:wait_time", Value: float64(connWaitTime), Type: metric.HistogramType(metric.UnitDuration)},
	}})
}

func FreeConn(connUseTime time.Duration) {
	metricConnsInUse.Add(-1)
	AddMetrics(metric.Measurement{Name: "session", Fields: []metric.Field{
		{Name: "conn:use_time", Value: float64(connUseTime), Type: metric.HistogramType(metric.UnitDuration)},
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
		{Name: "query:exec_time", Value: float64(d), Type: metric.HistogramType(metric.UnitDuration)},
	}})
}

func QueryWaitTime(d time.Duration) {
	AddMetrics(metric.Measurement{Name: "session", Fields: []metric.Field{
		{Name: "query:wait_time", Value: float64(d), Type: metric.HistogramType(metric.UnitDuration)},
	}})
}

func QueryFetchTime(d time.Duration) {
	AddMetrics(metric.Measurement{Name: "session", Fields: []metric.Field{
		{Name: "query:fetch_time", Value: float64(d), Type: metric.HistogramType(metric.UnitDuration)},
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
		prd, ok := val.(*metric.HistogramValue)
		if !ok {
			return 0, 0, 0
		}
		values := prd.Values
		return uint64(values[0]), uint64(values[len(values)-1]), int64(prd.Samples)
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
		} else if _, ok := kv.Value.(metric.MultiTimeSeries); ok {
			if pass, order := filter(key); pass {
				metricQueryKeys = append(metricQueryKeys, MetricQueryKey{key: kv.Key, column: MakeColumnString(kv.Key), valueType: "i", order: order})
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
		} else if val, ok := kv.(expvar.Func); ok {
			for r := 0; r < rowsCount; r++ {
				ret.Rows[r].Values[colIdxOffset] = val()
			}
			colIdxOffset++
		} else if val, ok := kv.(metric.MultiTimeSeries); ok {
			tss, prds := val[0].LastN(rowsCount)
			for i := range tss {
				ret.Rows[i].Timestamp = tss[i]
				switch prd := prds[i].(type) {
				case *metric.CounterValue:
					ret.Rows[i].Values[colIdxOffset] = prd.Value
				case *metric.GaugeValue:
					ret.Rows[i].Values[colIdxOffset] = prd.Value
				case *metric.MeterValue:
					ret.Rows[i].Values[colIdxOffset] = prd.Last
				case *metric.HistogramValue:
					ret.Rows[i].Values[colIdxOffset] = prd.Values[0]
				default:
					fmt.Printf("unknown metric type:%#v\n", prd)
				}
			}
			colIdxOffset++
		}
	}
	return ret
}

var collector *metric.Collector
var prefix string = "machbase"
var metricsDest string

func StartMetrics() {
	collector = metric.NewCollector(
		metric.WithSamplingInterval(10*time.Second),
		metric.WithSeries("2h", 60*time.Second, 120),
		metric.WithSeries("2d12h", 30*time.Minute, 120),
		metric.WithPrefix(prefix),
		metric.WithInputBuffer(50),
	)
	collector.AddInput(&SessionInput{})
	collector.AddOutputFunc(onProduct)
	collector.Start()
}

func StopMetrics() {
	if collector == nil {
		return
	}
	collector.Stop()
	collector = nil
}

func MetricsDestTable() string {
	return metricsDest
}

func SetMetricsDestTable(destTable string) error {
	destTable = strings.ToUpper(strings.TrimSpace(destTable))
	if destTable != "" {
		ctx := context.Background()
		conn, err := defaultDatabase.Connect(ctx, WithTrustUser("sys"))
		if err != nil {
			metricLog.Errorf("metrics connect: %v", err)
			return nil
		}
		defer conn.Close()
		ddl := fmt.Sprintf("CREATE TAG TABLE IF NOT EXISTS %s ("+
			"NAME VARCHAR(200) primary key, "+
			"TIME DATETIME basetime, "+
			"VALUE DOUBLE)", destTable)
		r := conn.Exec(ctx, ddl)
		if r.Err() != nil {
			metricLog.Errorf("metrics creating table: %v", r.Err())
			return nil
		}
	}

	metricsDest = destTable
	return nil
}

func AddMetricsInput(in metric.Input) {
	if collector == nil {
		return
	}
	collector.AddInput(in)
}

func AddMetricsFunc(f func(metric.Gather)) {
	if collector == nil {
		return
	}
	collector.AddInputFunc(f)
}

func AddMetrics(m metric.Measurement) {
	if collector == nil {
		return
	}
	collector.Send(m)
}

type SessionInput struct{}

var _ metric.Input = (*SessionInput)(nil)

func (si *SessionInput) Init() error {
	return nil
}

func (si *SessionInput) Gather(g metric.Gather) {
	m := metric.Measurement{Name: "session"}
	m.AddField(
		metric.Field{Name: "conn:in_use", Value: float64(metricConnsInUse.Load()), Type: metric.GaugeType(metric.UnitShort)},
		metric.Field{Name: "stmt:in_use", Value: float64(metricStmtsInUse.Load()), Type: metric.GaugeType(metric.UnitShort)},
		metric.Field{Name: "append:in_use", Value: float64(metricAppendersInUse.Load()), Type: metric.GaugeType(metric.UnitShort)},
		metric.Field{Name: "stmt:count", Value: float64(metricStmts.Load()), Type: metric.OdometerType(metric.UnitShort)},
		metric.Field{Name: "query:count", Value: float64(metricQueryCount.Load()), Type: metric.OdometerType(metric.UnitShort)},
		metric.Field{Name: "append:count", Value: float64(metricAppenders.Load()), Type: metric.OdometerType(metric.UnitShort)},
	)
	g.AddMeasurement(m)
}

type MetricRec struct {
	Name  string
	Time  int64
	Value float64
}

func onProduct(pd metric.Product) {
	if metricsDest == "" {
		return
	}
	var result []MetricRec
	switch p := pd.Value.(type) {
	case *metric.CounterValue:
		if p.Samples == 0 {
			return // Skip zero counters
		}
		result = []MetricRec{{
			Name:  fmt.Sprintf("%s:%s:%s", prefix, pd.Measure, pd.Field),
			Time:  pd.Time.UnixNano(),
			Value: p.Value,
		}}
	case *metric.GaugeValue:
		if p.Samples == 0 {
			return // Skip zero gauges
		}
		result = []MetricRec{{
			Name:  fmt.Sprintf("%s:%s:%s", prefix, pd.Measure, pd.Field),
			Time:  pd.Time.UnixNano(),
			Value: p.Value,
		}}
	case *metric.MeterValue:
		if p.Samples == 0 {
			return // Skip zero meters
		}
		result = []MetricRec{
			{
				Name:  fmt.Sprintf("%s:%s:%s:min", prefix, pd.Measure, pd.Field),
				Time:  pd.Time.UnixNano(),
				Value: p.Min,
			},
			{
				Name:  fmt.Sprintf("%s:%s:%s:max", prefix, pd.Measure, pd.Field),
				Time:  pd.Time.UnixNano(),
				Value: p.Max,
			},
			{
				Name:  fmt.Sprintf("%s:%s:%s:avg", prefix, pd.Measure, pd.Field),
				Time:  pd.Time.UnixNano(),
				Value: p.Sum / float64(p.Samples),
			},
		}
	case *metric.HistogramValue:
		if p.Samples == 0 {
			return // Skip zero samples
		}
		result = append(result, MetricRec{
			Name:  fmt.Sprintf("%s:%s:%s", prefix, pd.Measure, pd.Field),
			Time:  pd.Time.UnixNano(),
			Value: float64(p.Samples),
		})
		for i, x := range p.P {
			pct := fmt.Sprintf("%d", int(x*1000))
			if pct[len(pct)-1] == '0' {
				pct = pct[:len(pct)-1]
			}
			result = append(result, MetricRec{
				Name:  fmt.Sprintf("%s:%s:%s:p%s", prefix, pd.Measure, pd.Field, pct),
				Time:  pd.Time.UnixNano(),
				Value: p.Values[i],
			})
		}
	case *metric.OdometerValue:
		if p.Samples == 0 {
			return // Skip zero odometers
		}
		result = append(result, MetricRec{
			Name:  fmt.Sprintf("%s:%s:%s", prefix, pd.Measure, pd.Field),
			Time:  pd.Time.UnixNano(),
			Value: p.Diff(),
		})
	default:
		metricLog.Errorf("metrics unknown type: %T", p)
		return
	}

	go func(result []MetricRec, table string) {
		if table == "" {
			return
		}
		ctx := context.Background()
		conn, err := defaultDatabase.Connect(ctx, WithTrustUser("sys"))
		if err != nil {
			metricLog.Errorf("metrics connect: %v", err)
			return
		}
		defer conn.Close()
		sqlText := fmt.Sprintf("INSERT INTO %s (NAME, TIME, VALUE) VALUES (?, ?, ?)", table)
		for _, m := range result {
			r := conn.Exec(ctx, sqlText, m.Name, m.Time, m.Value)
			if r.Err() != nil {
				metricLog.Errorf("metrics writing: %v", r.Err())
				return
			}
		}
	}(result, metricsDest)
}

func DashboardHandler() http.HandlerFunc {
	httpStatusFilter := metric.MustCompile([]string{"machbase:http:status_[1-5]xx"}, ':')
	tqlCacheFilter := metric.MustCompile([]string{"machbase:tql:cache:*"}, ':')
	dbCountFilter := metric.MustCompile([]string{"machbase:session:*:count"}, ':')
	dbInuseFilter := metric.MustCompile([]string{"machbase:session:*:in_use"}, ':')
	dbMachSvrFilter := metric.MustCompile([]string{"machbase:machsvr:*"}, ':')
	netstatFilter := metric.MustCompile([]string{"machbase:netstat:*"}, ':')

	avgFilter := metric.MustCompile([]string{"*(avg)"})
	lastFilter := metric.MustCompile([]string{"*(last)"})

	dash := metric.NewDashboard(collector)
	dash.PageTitle = "MACHBASE-NEO"
	dash.ShowRemains = false
	dash.SetTheme("light")
	dash.AddChart(metric.Chart{Title: "CPU", MetricNames: []string{"machbase:ps:cpu_percent"}, ValueSelector: avgFilter})
	dash.AddChart(metric.Chart{Title: "MEM", MetricNames: []string{"machbase:ps:mem_percent"}, ValueSelector: avgFilter})
	dash.AddChart(metric.Chart{Title: "NETSTAT", MetricNameFilter: netstatFilter, ValueSelector: lastFilter})
	dash.AddChart(metric.Chart{Title: "DB SYSMEM", MetricNames: []string{"machbase:sys:sysmem"}, ValueSelector: lastFilter})
	dash.AddChart(metric.Chart{Title: "Go Routines", MetricNames: []string{"machbase:runtime:goroutines"}, ValueSelector: lastFilter})
	dash.AddChart(metric.Chart{Title: "Go Heap Inuse", MetricNames: []string{"machbase:runtime:heap_inuse"}, ValueSelector: lastFilter})
	dash.AddChart(metric.Chart{Title: "CGO Calls", MetricNames: []string{"machbase:runtime:cgo_call"}, Type: metric.ChartTypeBar})
	dash.AddChart(metric.Chart{Title: "HTTP Payload", MetricNames: []string{"machbase:http:recv_bytes", "machbase:http:send_bytes"}, Type: metric.ChartTypeLine})
	dash.AddChart(metric.Chart{Title: "HTTP Status", MetricNameFilter: httpStatusFilter, Type: metric.ChartTypeBarStack})
	dash.AddChart(metric.Chart{Title: "HTTP Latency", MetricNames: []string{"machbase:http:latency"}})
	dash.AddChart(metric.Chart{Title: "DB Use Count", MetricNameFilter: dbCountFilter})
	dash.AddChart(metric.Chart{Title: "DB Inuse", MetricNameFilter: dbInuseFilter, ValueSelector: lastFilter})
	dash.AddChart(metric.Chart{Title: "DB Native Inuse", MetricNameFilter: dbMachSvrFilter, ValueSelector: lastFilter})
	dash.AddChart(metric.Chart{Title: "DB Wait Time", MetricNames: []string{"machbase:session:conn:wait_time"}})
	dash.AddChart(metric.Chart{Title: "DB Use Time", MetricNames: []string{"machbase:session:conn:use_time"}})
	dash.AddChart(metric.Chart{Title: "TQL Cache", MetricNameFilter: tqlCacheFilter, ValueSelector: avgFilter})
	return dash.HandleFunc
}
