package spi

import (
	"context"
	"encoding/json"
	"expvar"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"text/template"
	"time"

	"github.com/machbase/neo-client/api"
	"github.com/machbase/neo-server/v8/jsh/viz"
	"github.com/machbase/neo-server/v8/mods/logging"
	"github.com/machbase/neo-server/v8/mods/util/glob"
	"github.com/machbase/neo-server/v8/mods/util/metric"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

const MetricQueryRowsMax = 120

var metricLog = logging.GetLog("statz")

type QueryStatzResult struct {
	Err        error
	Cols       []*api.Column
	Rows       []*QueryStatzRow
	ValueTypes []string
}

type QueryStatzRow struct {
	Timestamp time.Time
	Values    []any
}

type Statz struct {
	QueryExecHwm  uint64 `json:"queryExecHwm"`
	QueryExecAvg  uint64 `json:"queryExecAvg"`
	QueryWaitHwm  uint64 `json:"queryWaitHwm"`
	QueryWaitAvg  uint64 `json:"queryWaitAvg"`
	QueryFetchHwm uint64 `json:"queryFetchHwm"`
	QueryFetchAvg uint64 `json:"queryFetchAvg"`
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
	column    *api.Column
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
		if pass, order := filter(key); pass {
			switch kv.Value.(type) {
			case *expvar.Int:
				metricQueryKeys = append(metricQueryKeys, MetricQueryKey{key: kv.Key, column: api.MakeColumnInt64(kv.Key), valueType: "i", order: order})
			case *expvar.String:
				metricQueryKeys = append(metricQueryKeys, MetricQueryKey{key: kv.Key, column: api.MakeColumnString(kv.Key), valueType: "s", order: order})
			case *expvar.Float:
				metricQueryKeys = append(metricQueryKeys, MetricQueryKey{key: kv.Key, column: api.MakeColumnDouble(kv.Key), valueType: "f", order: order})
			case expvar.Func:
				metricQueryKeys = append(metricQueryKeys, MetricQueryKey{key: kv.Key, column: api.MakeColumnString(kv.Key), valueType: "i", order: order})
			case metric.MultiTimeSeries:
				metricQueryKeys = append(metricQueryKeys, MetricQueryKey{key: kv.Key, column: api.MakeColumnString(kv.Key), valueType: "i", order: order})
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
		switch val := kv.(type) {
		case *expvar.Int:
			for r := 0; r < rowsCount; r++ {
				ret.Rows[r].Values[colIdxOffset] = val.Value()
			}
			colIdxOffset++
		case *expvar.String:
			for r := 0; r < rowsCount; r++ {
				ret.Rows[r].Values[colIdxOffset] = val.Value()
			}
			colIdxOffset++
		case *expvar.Float:
			for r := 0; r < rowsCount; r++ {
				ret.Rows[r].Values[colIdxOffset] = val.Value()
			}
			colIdxOffset++
		case expvar.Func:
			for r := 0; r < rowsCount; r++ {
				ret.Rows[r].Values[colIdxOffset] = val()
			}
			colIdxOffset++
		case metric.MultiTimeSeries:
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
				case *metric.OdometerValue:
					ret.Rows[i].Values[colIdxOffset] = prd.Diff()
				default:
					if prd != nil {
						fmt.Printf("unknown metric type:%#v\n", prd)
					}
				}
			}
			colIdxOffset++
		default:
			fmt.Printf("unknown metric type for key:%s, value:%#v\n", queryKey.key, kv)
		}
	}
	return ret
}

var collector *metric.Collector
var prefix string = "machbase"
var metricsDest string

const SERIES_ID_FINEST = "METRIC_2H"
const SERIES_ID_FINE = "METRIC_2D12H"

func StartMetrics() {
	m2h, _ := metric.NewSeriesID(SERIES_ID_FINEST, "2h | 1m", 60*time.Second, 120)
	m2d12h, _ := metric.NewSeriesID(SERIES_ID_FINE, "2d12h | 30m", 30*time.Minute, 120)
	collector = metric.NewCollector(
		metric.WithSamplingInterval(10*time.Second),
		metric.WithSeries(m2h),
		metric.WithSeries(m2d12h),
		metric.WithPrefix(prefix),
		metric.WithInputBuffer(50),
	)
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
		conn, err := Default().Connect(ctx, api.WithAuthKey("sys", DefaultKey()))
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

func AddInput(mi metric.Input) {
	if collector == nil {
		return
	}
	collector.AddInput(mi)
}

func AddInputFunc(f func(*metric.Gather) error) {
	if collector == nil {
		return
	}
	collector.AddInputFunc(f)
}

func AddMetrics(m ...metric.Measure) {
	if collector == nil {
		return
	}
	collector.Send(m...)
}

type MetricRec struct {
	Name  string
	Time  int64
	Value float64
}

func onProduct(pd metric.Product) error {
	if metricsDest == "" {
		return nil
	}
	// insert only finest resolution
	if pd.SeriesID != SERIES_ID_FINEST {
		return nil
	}
	var result []MetricRec
	switch p := pd.Value.(type) {
	case *metric.CounterValue:
		if p.Samples == 0 {
			return nil // Skip zero counters
		}
		result = []MetricRec{{
			Name:  fmt.Sprintf("%s:%s", prefix, pd.Name),
			Time:  pd.Time.UnixNano(),
			Value: p.Value,
		}}
	case *metric.GaugeValue:
		if p.Samples == 0 {
			return nil // Skip zero gauges
		}
		result = []MetricRec{{
			Name:  fmt.Sprintf("%s:%s", prefix, pd.Name),
			Time:  pd.Time.UnixNano(),
			Value: p.Value,
		}}
	case *metric.MeterValue:
		if p.Samples == 0 {
			return nil // Skip zero meters
		}
		result = []MetricRec{
			{
				Name:  fmt.Sprintf("%s:%s:min", prefix, pd.Name),
				Time:  pd.Time.UnixNano(),
				Value: p.Min,
			},
			{
				Name:  fmt.Sprintf("%s:%s:max", prefix, pd.Name),
				Time:  pd.Time.UnixNano(),
				Value: p.Max,
			},
			{
				Name:  fmt.Sprintf("%s:%s:avg", prefix, pd.Name),
				Time:  pd.Time.UnixNano(),
				Value: p.Sum / float64(p.Samples),
			},
		}
	case *metric.HistogramValue:
		if p.Samples == 0 {
			return nil // Skip zero samples
		}
		result = append(result, MetricRec{
			Name:  fmt.Sprintf("%s:%s", prefix, pd.Name),
			Time:  pd.Time.UnixNano(),
			Value: float64(p.Samples),
		})
		for i, x := range p.P {
			pct := fmt.Sprintf("%d", int(x*1000))
			if pct[len(pct)-1] == '0' {
				pct = pct[:len(pct)-1]
			}
			result = append(result, MetricRec{
				Name:  fmt.Sprintf("%s:%s:p%s", prefix, pd.Name, pct),
				Time:  pd.Time.UnixNano(),
				Value: p.Values[i],
			})
		}
	case *metric.OdometerValue:
		if p.Samples == 0 {
			return nil // Skip zero odometers
		}
		result = append(result, MetricRec{
			Name:  fmt.Sprintf("%s:%s", prefix, pd.Name),
			Time:  pd.Time.UnixNano(),
			Value: p.Diff(),
		})
	default:
		metricLog.Errorf("metrics unknown type: %T", p)
		return nil
	}

	go func(result []MetricRec, table string) {
		if table == "" {
			return
		}
		ctx := context.Background()
		conn, err := Default().Connect(ctx, api.WithPassword("sys", "manager"))
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
	return nil
}

func DashboardHandler() http.HandlerFunc {
	dash := metric.NewDashboard(collector)
	dash.PageTitle = "MACHBASE-NEO"
	dash.ShowRemains = false
	// TODO: for offline use, download and place under /web/echarts
	// dash.Option.JsSrc = []string{"/web/echarts/echarts.min.js"}
	// dash.Option.JsSrc = append(dash.Option.JsSrc, "/web/echarts/themes/infographic.js")
	dash.SetTheme("light")
	dash.AddChart(metric.Chart{Title: "CPU/MEM", MetricNames: []string{"ps:cpu_percent", "ps:mem_percent"}, FieldNames: []string{"avg"}})
	dash.AddChart(metric.Chart{Title: "NETSTAT", MetricNames: []string{"netstat:*"}, FieldNames: []string{"last"}})
	dash.AddChart(metric.Chart{Title: "DB SYSMEM", MetricNames: []string{"sys:sysmem"}, FieldNames: []string{"last"}})
	// dash.AddChart(metric.Chart{Title: "Go Routines", MetricNames: []string{"runtime:goroutines"}, FieldNames: []string{"last"}})
	// dash.AddChart(metric.Chart{Title: "Go Heap Inuse", MetricNames: []string{"runtime:heap_inuse"}, FieldNames: []string{"last"}})
	// dash.AddChart(metric.Chart{Title: "CGO Calls", MetricNames: []string{"runtime:cgo_call"}, FieldNames: []string{"non_negative_diff"}, Type: metric.ChartTypeLine})
	dash.AddChart(metric.Chart{Title: "DB Sessions", MetricNames: []string{"sys:session:count"}, FieldNames: []string{"avg"}, Type: metric.ChartTypeLine})
	dash.AddChart(metric.Chart{Title: "DB Execute", MetricNames: []string{"sys:execute:count"}, FieldNames: []string{"non_negative_diff"}})
	dash.AddChart(metric.Chart{Title: "DB Execute Time", MetricNames: []string{"sys:execute:time:*"}, FieldNames: []string{"last"}})
	dash.AddChart(metric.Chart{Title: "DB Append", MetricNames: []string{"sys:append:data:*"}, FieldNames: []string{"non_negative_diff"}})
	dash.AddChart(metric.Chart{Title: "DB Rollup Gap", MetricNames: []string{"sys:rollup_global:gap"}, FieldNames: []string{"last"}})
	dash.AddChart(metric.Chart{Title: "DB Rollup Last Max Elapse", MetricNames: []string{"sys:rollup_global:last_elapse"}, FieldNames: []string{"last"}})
	dash.AddChart(metric.Chart{Title: "HTTP Payload", MetricNames: []string{"http:recv_bytes", "http:send_bytes"}, Type: metric.ChartTypeLine})
	dash.AddChart(metric.Chart{Title: "HTTP Status", MetricNames: []string{"http:status_[1-5]xx"}, Type: metric.ChartTypeBarStack})
	dash.AddChart(metric.Chart{Title: "HTTP Latency", MetricNames: []string{"http:latency"}})
	return dash.HandleFunc
}

func HandleStatz(w http.ResponseWriter, r *http.Request) {
	ret := map[string]any{}
	includes := r.URL.Query()["keys"]
	format := r.URL.Query().Get("format")
	interval := r.URL.Query().Get("interval")
	if interval == "" {
		interval = "1m"
	}
	dur, err := time.ParseDuration(interval)
	if err != nil {
		dur = time.Minute
	}

	stat := QueryStatzRows(dur, 1, func(key string) (bool, int) {
		return strings.HasPrefix(key, "machbase:") || slices.Contains(includes, key), 0
	})
	if stat.Err != nil {
		http.Error(w, stat.Err.Error(), http.StatusInternalServerError)
		return
	}
	for idx, col := range stat.Cols {
		value := stat.Rows[0].Values[idx]
		valueType := stat.ValueTypes[idx]
		if format == "html" {
			if value == nil {
				ret[col.Name] = "null"
				continue
			}
			printer := message.NewPrinter(language.English)
			switch col.DataType {
			case api.DataTypeInt64:
				ret[col.Name] = printer.Sprintf("%d", value)
			case api.DataTypeFloat64:
				switch valueType {
				case "dur":
					switch val := value.(type) {
					case float64:
						ret[col.Name] = printer.Sprintf("%s", time.Duration(val))
					case int64:
						ret[col.Name] = printer.Sprintf("%s", time.Duration(val))
					default:
						ret[col.Name] = printer.Sprintf("%v", value)
					}
				case "i":
					switch val := value.(type) {
					case float64:
						ret[col.Name] = printer.Sprintf("%d", int64(val))
					case int64:
						ret[col.Name] = printer.Sprintf("%d", val)
					default:
						ret[col.Name] = printer.Sprintf("%v", value)
					}
				default:
					ret[col.Name] = printer.Sprintf("%f", value)
				}
			case api.DataTypeString:
				ret[col.Name] = value
			default:
				ret[col.Name] = printer.Sprintf("%v", value)
			}
		} else {
			ret[col.Name] = value
		}
	}
	if format == "html" {
		tpl := template.New("statz").Funcs(template.FuncMap{
			"isMap": func(v any) bool {
				switch v.(type) {
				case map[string]any, map[string]float64, map[string]string, map[string]int64:
					return true
				default:
					return false
				}
			},
		})
		tpl = template.Must(tpl.Parse(tmplStatz))
		if err := tpl.ExecuteTemplate(w, "statz", ret); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	} else {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(ret); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

var tmplStatz = `
{{- define "statz" }}
<style>
  table {
    border-collapse: collapse;
  }
  tr:nth-child(even) {
    background-color: #f2f2f2;
  }
  td {
    border: 1px solid #ddd;
    padding: 8px;
  }
</style>
<table>
{{- range $key, $value := . }}
<tr>
  <td>{{ $key }}</td>
  <td>{{ $value }}</td>
</tr>
{{- end }}
</table>
{{ end }}`

func Visualizer() *VizSpecGenerator {
	return NewVizSpecGenerator(collector)
}

type VizSpecGenerator struct {
	metric.Dashboard
	collector *metric.Collector
}

func NewVizSpecGenerator(c *metric.Collector) *VizSpecGenerator {
	return &VizSpecGenerator{
		Dashboard: *metric.NewDashboard(c),
		collector: c,
	}
}

func (g VizSpecGenerator) HandleFunc(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	tsIdxStr := r.URL.Query().Get("tsIdx")
	var tsIdx int
	if _, err := fmt.Sscanf(tsIdxStr, "%d", &tsIdx); err != nil {
		tsIdx = 0
	}

	spec, err := g.Generate(id, tsIdx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(spec); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (g VizSpecGenerator) Generate(id string, tsIdx int) (*viz.Spec, error) {
	panelOpt := g.panelByID(id)

	spec := (&viz.Spec{}).Normalize()
	spec.Domain = viz.Domain{
		Kind:       viz.DomainKindTime,
		Timeformat: viz.TimeformatNano,
		TZ:         "UTC",
	}
	spec.Axes.X = viz.Axis{
		ID:    "time",
		Type:  viz.AxisTypeTime,
		Label: "Time",
	}
	spec.View = viz.View{PreferredRenderer: "echarts"}
	spec.Meta = viz.Meta{
		Producer:    "machbase-neo",
		GeneratedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}

	axisByID := map[string]struct{}{}
	var minTime time.Time
	var maxTime time.Time
	var found bool

	for _, metricName := range panelOpt.MetricNames {
		ss, ok := g.getSnapshot(metricName, tsIdx)
		if !ok || len(ss.Times) == 0 {
			continue
		}
		found = true

		axisID := axisIDForMetric(ss.Meta.MeasureName)
		axisLabel := ss.Meta.MeasureName
		if unit := string(ss.Meta.MeasureType.Unit()); unit != "" && unit != string(metric.UnitScalar) {
			axisLabel = fmt.Sprintf("%s (%s)", ss.Meta.MeasureName, unit)
		}
		if _, exists := axisByID[axisID]; !exists {
			spec.Axes.Y = append(spec.Axes.Y, viz.Axis{
				ID:    axisID,
				Type:  viz.AxisTypeLinear,
				Label: axisLabel,
				Unit:  string(ss.Meta.MeasureType.Unit()),
			})
			axisByID[axisID] = struct{}{}
		}

		seriesList := ss.Series(panelOpt)
		for _, series := range seriesList {
			vizSeries, seriesMin, seriesMax := toVizSeries(series, ss, axisID, panelOpt)
			if len(vizSeries.Data) == 0 {
				continue
			}
			spec.Series = append(spec.Series, vizSeries)
			if minTime.IsZero() || seriesMin.Before(minTime) {
				minTime = seriesMin
			}
			if maxTime.IsZero() || seriesMax.After(maxTime) {
				maxTime = seriesMax
			}
		}
	}

	if !found || len(spec.Series) == 0 {
		return nil, fmt.Errorf("no metrics found")
	}

	spec.Domain.From = minTime.UnixNano()
	spec.Domain.To = maxTime.UnixNano()
	if err := spec.Validate(); err != nil {
		return nil, err
	}
	return spec, nil
}

func (g VizSpecGenerator) panelByID(id string) metric.Chart {
	panels := g.Panels()
	if len(panels) == 0 {
		if id == "" {
			if len(g.Charts) > 0 {
				return g.Charts[0]
			}
			return metric.Chart{ID: "metrics", Title: "Metrics"}
		}
		return metric.Chart{ID: id, Title: id, MetricNames: []string{id}}
	}
	if id == "" {
		return panels[0]
	}
	for _, panel := range panels {
		if panel.ID == id {
			return panel
		}
	}
	for _, chart := range g.Charts {
		if chart.ID == id {
			return chart
		}
	}
	return metric.Chart{ID: id, Title: id, MetricNames: []string{id}}
}

func axisIDForMetric(metricName string) string {
	var sb strings.Builder
	for _, r := range metricName {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			sb.WriteRune(r)
			continue
		}
		sb.WriteByte('_')
	}
	axisID := strings.Trim(sb.String(), "_")
	if axisID == "" {
		return "y"
	}
	return axisID
}

func (g VizSpecGenerator) getSnapshot(metricName string, tsIdx int) (metric.Snapshot, bool) {
	if g.collector == nil {
		return metric.Snapshot{}, false
	}
	mts := g.collector.Timeseries(metricName)
	if mts == nil {
		return metric.Snapshot{}, false
	}
	if tsIdx < 0 || tsIdx >= len(mts) {
		return metric.Snapshot{}, false
	}
	ts := mts[tsIdx]
	times, values := ts.All()
	if len(times) == 0 {
		return metric.Snapshot{}, true
	}
	meta, _ := ts.Meta().(metric.SeriesInfo)
	return metric.Snapshot{
		PublishName: metricName,
		Times:       times,
		Values:      values,
		Interval:    ts.Interval(),
		MaxCount:    ts.MaxCount(),
		Meta:        meta,
	}, true
}

func toVizSeries(series metric.Series, ss metric.Snapshot, axisID string, chartOpt metric.Chart) (viz.Series, time.Time, time.Time) {
	data := make([]any, 0, len(series.Data))
	var minTime time.Time
	var maxTime time.Time
	for _, item := range series.Data {
		if item.Time == 0 {
			continue
		}
		tm := time.UnixMilli(item.Time)
		data = append(data, []any{tm.UnixNano(), item.Value})
		if minTime.IsZero() || tm.Before(minTime) {
			minTime = tm
		}
		if maxTime.IsZero() || tm.After(maxTime) {
			maxTime = tm
		}
	}

	vizSeries := viz.Series{
		ID:   series.Name,
		Name: series.Name,
		Axis: axisID,
		Representation: viz.Representation{
			Kind:   viz.RepresentationTimeBucketValue,
			Fields: []string{"time", "value"},
		},
		Source: viz.Source{
			Kind:        viz.SourceKindSampled,
			Resolution:  ss.Interval.String(),
			DerivedFrom: ss.PublishName,
		},
		Data: data,
		Quality: viz.Quality{
			Sampled:         true,
			RowCount:        len(data),
			EstimatedPoints: int64(len(data)),
		},
		Style: map[string]any{
			"preferredRenderer": "echarts",
		},
		Extra: map[string]any{
			"chartType":  string(series.Type),
			"stack":      series.Stack,
			"smooth":     series.Smooth,
			"showSymbol": chartOpt.ShowSymbol,
		},
	}
	return vizSeries, minTime, maxTime
}
