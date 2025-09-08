package metric

import (
	_ "embed"
	"encoding/json"
	"expvar"
	"fmt"
	"html/template"
	"net/http"
	"runtime/debug"
	"slices"
	"strings"
	"time"
)

func NewDashboard(c *Collector) *Dashboard {
	d := &Dashboard{
		Option:           DefaultDashboardOption(),
		Timeseries:       c.Series(),
		SamplingInterval: c.SamplingInterval(),
		nameProvider:     c.PublishNames,
	}
	return d
}

var _ http.Handler = (*Dashboard)(nil)

type Dashboard struct {
	Option           DashboardOption
	PanelOptions     []Chart
	Timeseries       []CollectorSeries
	SeriesIdx        int
	ShowRemains      bool
	SamplingInterval time.Duration
	nameProvider     func() []string
}

type Chart struct {
	MetricNames      []string
	MetricNameFilter Filter // pattern to match metric names, e.g., "net:*"
	ValueSelector    Filter // pattern to match field names, e.g., "*(avg)"
	ID               string
	Title            string
	SubTitle         string
	Type             ChartType // e.g., line, bar
}

// multiple metric names can be added in one place to group them together
// those series are shown together in one graph
func (d *Dashboard) AddChart(co ...Chart) {
	for _, c := range co {
		if c.ID == "" {
			c.ID = fmt.Sprintf("@%d", len(d.PanelOptions)+1)
		}
		if c.Title == "" {
			c.Title = strings.Join(c.MetricNames, ", ")
		}
		d.PanelOptions = append(d.PanelOptions, c)
	}
}

func (d Dashboard) Panels() []Chart {
	lst := d.nameProvider()
	slices.Sort(lst)

	ret := []Chart{}
	for idx := range d.PanelOptions {
		po := &d.PanelOptions[idx]
		if po.MetricNameFilter != nil {
			d.refreshPanel(po)
			// remove matched names from lst
			for _, name := range po.MetricNames {
				if i := slices.Index(lst, name); i >= 0 {
					lst = append(lst[:i], lst[i+1:]...)
				}
			}
		} else if len(po.MetricNames) > 0 {
			for _, name := range po.MetricNames {
				if i := slices.Index(lst, name); i >= 0 {
					lst = append(lst[:i], lst[i+1:]...)
				}
			}
		}
		ret = append(ret, *po)
	}
	if d.ShowRemains {
		for _, name := range lst {
			ret = append(ret, Chart{ID: name, Title: name})
		}
	}
	return ret
}

func (d Dashboard) refreshPanel(po *Chart) {
	if po.MetricNameFilter == nil {
		return
	}
	lst := d.nameProvider()
	slices.Sort(lst)
	for _, name := range lst {
		if po.MetricNameFilter.Match(name) {
			if !slices.Contains(po.MetricNames, name) {
				po.MetricNames = append(po.MetricNames, name)
			}
		}
	}
}

type DashboardOption struct {
	BasePath string
	JsSrc    []string
	Style    []CSSStyle
}

func DefaultDashboardOption() DashboardOption {
	return DashboardOption{
		JsSrc: []string{
			"https://cdn.jsdelivr.net/npm/echarts@6.0.0/dist/echarts.min.js",
		},
		Style: []CSSStyle{
			{
				Selector: "body",
				Styles: map[string]string{
					"background": "rgb(38,40,49)",
				},
			},
			{
				Selector: ".container",
				Styles: map[string]string{
					"display":         "flex",       // Enables Flexbox
					"flex-wrap":       "wrap",       // Allows wrapping to the next line
					"gap":             "10px",       // Adds spacing between panels
					"justify-content": "flex-start", // Aligns panels to the left
					// "justify-content": "space-between" // Distributes panels evenly with space between
				},
			},
			{
				Selector: ".panel",
				Styles: map[string]string{
					"flex":          "0 0 400px", // Each panel takes up 400px width
					"height":        "300px",     // Fixed height for each panel
					"border-radius": "4px",
					"padding":       "0px",
					"box-shadow":    "2px 2px 5px rgba(0,0,0,0.1)",
				},
			},
			{
				Selector: ".series-tabs",
				Styles: map[string]string{
					"display":       "flex",
					"gap":           "4px",
					"margin-bottom": "0.2em",
					"margin-left":   "4px",
				},
			},
			{
				Selector: ".series-tabs .tab",
				Styles: map[string]string{
					"padding":         "6px 16px",
					"border":          "1px solid #888",
					"border-radius":   "6px 6px 0 0",
					"background":      "#222",
					"color":           "#eee",
					"text-decoration": "none",
					"cursor":          "pointer",
					"transition":      "background 0.2s",
				},
			},
			{
				Selector: ".series-tabs .tab.active",
				Styles: map[string]string{
					"background":    "#444",
					"font-weight":   "bold",
					"border-bottom": "2px solid #fff",
				},
			},
			{
				Selector: ".series-tabs .tab:hover",
				Styles: map[string]string{
					"background": "#333",
				},
			},
		},
	}
}

func (opt DashboardOption) StyleCSS() template.CSS {
	var sb strings.Builder
	for _, style := range opt.Style {
		sb.WriteString(style.Selector)
		sb.WriteString(" {")
		for k, v := range style.Styles {
			sb.WriteString(k)
			sb.WriteString(": ")
			sb.WriteString(v)
			sb.WriteString("; ")
		}
		sb.WriteString("}\n")
	}
	return template.CSS(sb.String())
}

type CSSStyle struct {
	Selector string
	Styles   map[string]string
}

func (d Dashboard) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	d.HandleFunc(w, r)
}

func (d Dashboard) HandleFunc(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if rec := recover(); rec != nil {
			fmt.Println("Recovered in Dashboard.Handle:", rec)
			debug.PrintStack()
			http.Error(w, fmt.Sprintf("Internal server error: %v", rec), http.StatusInternalServerError)
		}
	}()

	if id := r.URL.Query().Get("id"); id == "" {
		d.HandleIndex(w, r)
	} else {
		d.HandleData(w, r)
	}
}

func (d Dashboard) HandleIndex(w http.ResponseWriter, r *http.Request) {
	// BasePath
	d.Option.BasePath = r.URL.Path
	// tsIdx
	tsIdxStr := r.URL.Query().Get("tsIdx")
	if _, err := fmt.Sscanf(tsIdxStr, "%d", &d.SeriesIdx); err != nil {
		d.SeriesIdx = 0
	}
	// showRemains
	if r.URL.Query().Has("showRemains") {
		showRemains := r.URL.Query().Get("showRemains")
		if showRemains == "0" || strings.ToLower(showRemains) == "false" {
			d.ShowRemains = false
		} else {
			d.ShowRemains = true
		}
	}
	w.Header().Set("Content-Type", "text/html")
	err := tmplIndex.Execute(w, d)
	if err != nil {
		http.Error(w, "Error rendering template: "+err.Error(), http.StatusInternalServerError)
		return
	}
}

func (d Dashboard) HandleData(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	id := query.Get("id")
	tsIdxStr := query.Get("tsIdx")

	var tsIdx int
	if _, err := fmt.Sscanf(tsIdxStr, "%d", &tsIdx); err != nil {
		tsIdx = 0
	}
	var panelOpt Chart
	for _, po := range d.PanelOptions {
		if po.ID == id {
			panelOpt = po
			break
		}
	}
	if panelOpt.ID == "" {
		// id not found, which means it is one of the remains
		// create a new panel option for it with default settings
		panelOpt = Chart{
			ID:          id,
			MetricNames: []string{id},
		}
	}
	if panelOpt.MetricNameFilter != nil {
		d.refreshPanel(&panelOpt)
	}

	var series []Series
	var meta *FieldInfo
	var seriesMaxCount int
	var seriesInterval time.Duration
	var notFound bool = true
	for _, metricName := range panelOpt.MetricNames {
		ss, ssExists := getSnapshot(metricName, tsIdx)

		if !ssExists {
			panelOpt.SubTitle = "Metric not found"
			continue
		}
		notFound = false
		series = append(series, ss.Series(panelOpt)...)

		if meta == nil {
			meta = &ss.Meta
			seriesInterval = ss.Interval
			seriesMaxCount = ss.MaxCount
		}
		if panelOpt.Title == "" {
			panelOpt.Title = ss.PublishName
		}
	}
	if notFound {
		http.Error(w, "Metric not found", http.StatusNotFound)
		return
	}
	var seriesSingleOrArray any
	if len(series) == 1 {
		seriesSingleOrArray = series[0]
	} else {
		seriesSingleOrArray = series
	}

	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	err := enc.Encode(H{
		"chartOption": H{
			"series": seriesSingleOrArray,
			"title": H{
				"text":    panelOpt.Title,
				"subtext": panelOpt.SubTitle,
			},
			"legend": H{},
			"tooltip": H{
				"trigger": "axis",
			},
			"xAxis": H{
				"type":      "time",
				"axisLabel": H{"hideOverlap": true},
			},
			"yAxis":     H{},
			"animation": false,
		},
		"interval": seriesInterval.Milliseconds(),
		"maxCount": seriesMaxCount,
		"meta": H{
			"measure": meta.Measure,
			"field":   meta.Name,
			"series":  meta.Series,
			"unit":    meta.Unit,
			"type":    meta.Type,
		},
	})
	if err != nil {
		http.Error(w, "Error encoding JSON: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

type H map[string]any

type Item struct {
	Time  int64
	Value any
}

func (itm Item) MarshalJSON() ([]byte, error) {
	if arr, ok := itm.Value.([]any); ok {
		return json.Marshal(append([]any{itm.Time}, arr...))
	}
	return json.Marshal([2]any{itm.Time, itm.Value})
}

type Series struct {
	Name       string         `json:"name"`
	Data       []Item         `json:"data"`
	Type       string         `json:"type"`                // e.g. 'line',
	Stack      any            `json:"stack,omitempty"`     // nil or stack-name
	Smooth     bool           `json:"smooth"`              //  true,
	ShowSymbol bool           `json:"showSymbol"`          // showSymbol: true,
	AreaStyle  map[string]any `json:"areaStyle,omitempty"` // {}
}

type ChartType string

const (
	ChartTypeLine        ChartType = "line"
	ChartTypeLineStack   ChartType = "line-stack"
	ChartTypeBar         ChartType = "bar"
	ChartTypeBarStack    ChartType = "bar-stack"
	ChartTypeCandlestick ChartType = "candlestick"
)

func (ct ChartType) TypeAndStack(fallback string) (string, any) {
	switch ct {
	case ChartTypeLineStack:
		return "line", "total"
	case ChartTypeBarStack:
		return "bar", "total"
	default:
		if ct != "" {
			return string(ct), nil
		}
	}
	return fallback, nil
}

func (ss Snapshot) Series(opt Chart) []Series {
	var series []Series
	switch ss.Meta.Type {
	case "counter":
		typ, stack := opt.Type.TypeAndStack("bar")
		series = []Series{
			{
				Name:       ss.Meta.Name,
				Type:       typ,
				Data:       make([]Item, len(ss.Times)),
				Stack:      stack,
				Smooth:     true,
				ShowSymbol: false,
				AreaStyle: H{
					"opacity": 0.5,
				},
			},
		}
		for i, t := range ss.Times {
			series[0].Data[i].Time = t.UnixMilli()
			if v, ok := ss.Values[i].(*CounterValue); ok && v.Samples > 0 {
				series[0].Data[i].Value = v.Value
			}
		}
	case "gauge":
		// for gauge, show avg and last value
		seriesNames := []string{ss.Meta.Name + "(avg)", ss.Meta.Name + "(last)"}
		seriesFlags := map[string]int{}
		for i, seriesName := range seriesNames {
			if opt.ValueSelector != nil && !opt.ValueSelector.Match(seriesName) {
				continue
			}
			switch i {
			case 0:
				seriesFlags["avg"] = len(series)
			case 1:
				seriesFlags["last"] = len(series)
			}
			series = append(series, Series{
				Name:       seriesName,
				Type:       "line",
				Data:       make([]Item, len(ss.Times)),
				Smooth:     true,
				ShowSymbol: false,
			})
		}
		for i, tm := range ss.Times {
			for s := range series {
				series[s].Data[i].Time = tm.UnixMilli()
			}
			if v, ok := ss.Values[i].(*GaugeValue); ok && v.Samples > 0 {
				if idx, ok := seriesFlags["avg"]; ok {
					series[idx].Data[i].Value = v.Sum / float64(v.Samples)
				}
				if idx, ok := seriesFlags["last"]; ok {
					series[idx].Data[i].Value = v.Value
				}
			}
		}
	case "meter":
		series = []Series{
			{
				Name:       ss.Meta.Name,
				Type:       "candlestick",
				Data:       make([]Item, len(ss.Times)),
				Smooth:     true,
				ShowSymbol: false,
			},
		}
		for i, t := range ss.Times {
			series[0].Data[i].Time = t.UnixMilli()
			if v, ok := ss.Values[i].(*MeterValue); ok && v.Samples > 0 {
				// data order [open, close, lowest, highest]
				series[0].Data[i].Value = []any{v.First, v.Last, v.Min, v.Max}
			}
		}
	case "histogram":
		last := ss.Values[len(ss.Values)-1].(*HistogramValue)
		for _, p := range last.P {
			pName := fmt.Sprintf("p%d", int(p*1000))
			if pName[len(pName)-1] == '0' {
				pName = pName[:len(pName)-1]
			}
			series = append(series, Series{
				Name:       ss.Meta.Name + "(" + pName + ")",
				Type:       "line",
				Data:       make([]Item, len(ss.Times)),
				Smooth:     true,
				ShowSymbol: false,
				AreaStyle: H{
					"opacity": 0.3,
				},
			})
		}

		for i, t := range ss.Times {
			for s := range series {
				series[s].Data[i].Time = t.UnixMilli()
			}
			if v, ok := ss.Values[i].(*HistogramValue); ok && v.Samples > 0 {
				for pIdx, pVal := range v.Values {
					series[pIdx].Data[i].Value = pVal
				}
			}
		}
	}
	return series
}

//go:embed dashboard.tmpl
var tmplIndexHtml string

var tmplIndex = template.Must(template.New("index").Funcs(tmplFuncMap).Parse(tmplIndexHtml))

var tmplFuncMap = template.FuncMap{
	"sub": func(a, b int) int {
		return a - b
	},
	"seriesTitle": func(s CollectorSeries) string {
		title := s.Name + " | " + s.Period.String()
		if strings.HasSuffix(title, "m0s") {
			title = strings.TrimSuffix(title, "0s")
		}
		if strings.HasSuffix(title, "h0m") {
			title = strings.TrimSuffix(title, "0m")
		}
		return title
	},
}

type Snapshot struct {
	PublishName string
	Times       []time.Time
	Values      []Value
	Interval    time.Duration
	MaxCount    int
	Meta        FieldInfo
}

func getSnapshot(expvarKey string, tsIdx int) (Snapshot, bool) {
	var ret Snapshot
	if g := expvar.Get(expvarKey); g != nil {
		mts := g.(MultiTimeSeries)
		if tsIdx < 0 || tsIdx >= len(mts) {
			return ret, false
		}
		ts := mts[tsIdx]
		times, values := ts.All()
		if len(times) > 0 {
			ret = Snapshot{
				PublishName: expvarKey,
				Times:       times,
				Values:      values,
				Interval:    ts.Interval(),
				MaxCount:    ts.MaxCount(),
				Meta:        ts.Meta().(FieldInfo),
			}
		}
		return ret, true
	}
	return ret, false
}
