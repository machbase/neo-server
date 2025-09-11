package metric

import (
	_ "embed"
	"encoding/json"
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
		Option:             DefaultDashboardOption(),
		Timeseries:         c.Series(),
		SamplingInterval:   c.SamplingInterval(),
		nameProvider:       c.MetricNames,
		timeseriesProvider: c.Timeseries,
		PageTitle:          "Metrics",
	}
	return d
}

var _ http.Handler = (*Dashboard)(nil)

type Dashboard struct {
	Option             DashboardOption
	Charts             []Chart
	Timeseries         []CollectorSeries
	SeriesIdx          int
	ShowRemains        bool
	SamplingInterval   time.Duration
	PageTitle          string
	nameProvider       func() []string
	timeseriesProvider func(string) MultiTimeSeries
}

type Chart struct {
	MetricNames    []string
	SeriesSelector Filter // pattern to match field names, e.g., "*(avg)"
	ID             string
	Title          string
	SubTitle       string
	Type           ChartType // e.g., line, bar

	metricNameFilter Filter
}

// multiple metric names can be added in one place to group them together
// those series are shown together in one graph
func (d *Dashboard) AddChart(co ...Chart) error {
	for _, c := range co {
		if c.ID == "" {
			c.ID = fmt.Sprintf("@%d", len(d.Charts)+1)
		}
		if c.Title == "" {
			c.Title = fmt.Sprintf("NoTitle-%s", c.ID)
		}
		hasPattern := false
		for _, n := range c.MetricNames {
			if IsFilterPattern(n) {
				hasPattern = true
				break
			}
		}
		if hasPattern {
			if f, err := Compile(c.MetricNames, ':'); err != nil {
				return fmt.Errorf("error compiling metric name filter %v: %w", c.MetricNames, err)
			} else {
				c.metricNameFilter = f
			}
		}
		d.Charts = append(d.Charts, c)
	}
	return nil
}

func (d Dashboard) Panels() []Chart {
	lst := d.nameProvider()
	slices.Sort(lst)

	ret := []Chart{}
	for idx := range d.Charts {
		po := &d.Charts[idx]
		if po.metricNameFilter != nil {
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
	if po.metricNameFilter == nil {
		return
	}
	lst := d.nameProvider()
	slices.Sort(lst)
	for _, name := range lst {
		if po.metricNameFilter.Match(name) {
			if !slices.Contains(po.MetricNames, name) {
				po.MetricNames = append(po.MetricNames, name)
			}
		}
	}
}

type DashboardOption struct {
	BasePath string
	Theme    string // "light" or "dark"
	JsSrc    []string
	Style    map[string]CSSStyle
}

func DefaultDashboardOption() DashboardOption {
	return DashboardOption{
		JsSrc: []string{
			"https://cdn.jsdelivr.net/npm/echarts@6.0.0/dist/echarts.min.js",
		},
		Theme: "dark",
		Style: map[string]CSSStyle{
			"body": {
				"background": "rgb(38,40,49)",
			},
			".container": {
				"display":         "flex",       // Enables Flexbox
				"flex-wrap":       "wrap",       // Allows wrapping to the next line
				"gap":             "10px",       // Adds spacing between panels
				"justify-content": "flex-start", // Aligns panels to the left
			},
			".panel": {
				"flex":          "1 1 400px", // Each panel takes up 400px width
				"min-width":     "400px",     // Minimum width for each panel
				"max-width":     "640px",     // Maximum width for each panel
				"height":        "300px",     // Fixed height for each panel
				"border-radius": "4px",
				"padding":       "0px",
				"border":        "1px solid rgba(0,0,0,0.1)",
				"box-shadow":    "2px 2px 5px rgba(0,0,0,0.1)",
			},
			".header-row": {
				"display":         "flex",
				"justify-content": "space-between",
				"align-items":     "center",
				"width":           "100%",
				"margin-bottom":   "0em",
			},
			".page-title": {
				"font-family":  "'Segoe UI', 'Arial', 'Helvetica Neue', Helvetica, Arial, sans-serif",
				"font-weight":  "bold",
				"font-size":    "1.8em",
				"margin":       "0",
				"padding-left": "0.5em",
			},
			".series-tabs": {
				"display":      "flex",
				"gap":          "4px",
				"font-family":  "'Segoe UI', 'Arial', 'Helvetica Neue', Helvetica, Arial, sans-serif",
				"margin-right": "4px",
			},
			".series-tabs .tab": {
				"padding":         "6px 16px",
				"border":          "1px solid #888",
				"border-radius":   "6px 6px 0 0",
				"background":      "#222",
				"color":           "#eee",
				"text-decoration": "none",
				"cursor":          "pointer",
				"transition":      "background 0.2s",
			},
			".series-tabs .tab.active": {
				"background":    "#444",
				"font-weight":   "bold",
				"border-bottom": "2px solid #fff",
			},
			".series-tabs .tab:hover": {
				"background": "#333",
			},
		},
	}
}

// SetTheme sets the dashboard theme to either "light" or "dark"
func (d *Dashboard) SetTheme(theme string) {
	switch theme {
	case "light":
		d.Option.Style["body"]["background"] = "rgb(255, 255, 255)"
		d.Option.Style[".page-title"]["color"] = "#222"
		d.Option.Style[".series-tabs .tab.active"]["border-bottom"] = "2px solid #c83707ff"
		d.Option.Style[".series-tabs .tab.active"]["background"] = "#bbb"
		d.Option.Style[".series-tabs .tab"]["background"] = "#eee"
		d.Option.Style[".series-tabs .tab"]["color"] = "#222"
		d.Option.Style[".series-tabs .tab:hover"]["background"] = "#ddd"
	case "dark":
		d.Option.Style["body"]["background"] = "rgb(38,40,49)"
		d.Option.Style[".page-title"]["color"] = "#eee"
		d.Option.Style[".series-tabs .tab.active"]["border-bottom"] = "2px solid #fff"
		d.Option.Style[".series-tabs .tab.active"]["background"] = "#444"
		d.Option.Style[".series-tabs .tab"]["background"] = "#222"
		d.Option.Style[".series-tabs .tab"]["color"] = "#eee"
		d.Option.Style[".series-tabs .tab:hover"]["background"] = "#333"
	default:
		return
	}
	d.Option.Theme = theme
}

func (d *Dashboard) SetPanelHeight(height int) {
	// "height":        "300px",     // Fixed height for each panel
	d.Option.Style[".panel"]["height"] = fmt.Sprintf("%dpx", height)
}

func (d *Dashboard) SetPanelMinWidth(width int) {
	// "flex":          "1 1 400px", // Each panel takes up 400px width
	// "min-width":     "400px",     // Minimum width for each panel
	d.Option.Style[".panel"]["flex"] = fmt.Sprintf("1 1 %dpx", width)
	d.Option.Style[".panel"]["min-width"] = fmt.Sprintf("%dpx", width)
}

func (d *Dashboard) SetPanelMaxWidth(width int) {
	// "max-width":     "640px",     // Maximum width for each panel
	d.Option.Style[".panel"]["max-width"] = fmt.Sprintf("%dpx", width)
}

func (opt DashboardOption) StyleCSS() template.CSS {
	var sb strings.Builder
	for selector, style := range opt.Style {
		sb.WriteString(selector)
		sb.WriteString(" {")
		for k, v := range style {
			sb.WriteString(k)
			sb.WriteString(": ")
			sb.WriteString(v)
			sb.WriteString("; ")
		}
		sb.WriteString("}\n")
	}
	return template.CSS(sb.String())
}

type CSSStyle map[string]string

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
	for _, po := range d.Charts {
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
	if panelOpt.metricNameFilter != nil {
		d.refreshPanel(&panelOpt)
	}

	var series []Series
	var meta *SeriesInfo
	var seriesMaxCount int
	var seriesInterval time.Duration
	var notFound bool = true
	var notFoundNames []string
	for _, metricName := range panelOpt.MetricNames {
		ss, ssExists := d.getSnapshot(metricName, tsIdx)

		if !ssExists {
			notFoundNames = append(notFoundNames, metricName)
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
	var seriesSingleOrArray any
	if notFound {
		// TODO: show not found message in the chart area instead of returning 404
		_ = notFoundNames
		// http.Error(w, "Metric not found", http.StatusNotFound)
		// return
	}
	trimSeriesNames(series)
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
			"legend": H{"type": "scroll", "width": "80%", "bottom": 4, "textStyle": H{"fontSize": 11}},
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
		"meta":     meta.H(),
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

// trimSeriesNames trims the series names to remove common prefixes and suffixes of all series' names
// the suffix and prefix are determined by longest common prefix/suffix of all series' names
// that has at least one colon(':') separator before and after it
// For example,
// e.g., "cpu:cpu_user:avg", "cpu:cpu_system:avg" => "user", "system"
// e.g.,  "go:goroutines", "go:threads" => "goroutines", "threads"
func trimSeriesNames(series []Series) {
	trimSeriesNamesSeparator(series, ":")
	trimSeriesNamesSeparator(series, "#")
}

func trimSeriesNamesSeparator(series []Series, separator string) {
	if len(series) <= 0 {
		return
	}
	names := make([]string, len(series))
	for i, s := range series {
		names[i] = s.Name
	}

	// Find common prefix (colon-separated)
	prefixParts := strings.Split(names[0], separator)
	for i := 1; i < len(names); i++ {
		parts := strings.Split(names[i], separator)
		max := len(prefixParts)
		if len(parts) < max {
			max = len(parts)
		}
		j := 0
		for ; j < max; j++ {
			if parts[j] != prefixParts[j] {
				break
			}
		}
		prefixParts = prefixParts[:j]
	}
	prefix := strings.Join(prefixParts, separator)
	if prefix != "" {
		prefix += separator
	}

	// Find common suffix (colon-separated)
	suffixParts := strings.Split(names[0], separator)
	for i := 1; i < len(names); i++ {
		parts := strings.Split(names[i], separator)
		max := len(suffixParts)
		if len(parts) < max {
			max = len(parts)
		}
		j := 0
		for ; j < max; j++ {
			if parts[len(parts)-1-j] != suffixParts[len(suffixParts)-1-j] {
				break
			}
		}
		suffixParts = suffixParts[len(suffixParts)-j:]
	}
	suffix := strings.Join(suffixParts, separator)
	if suffix != "" {
		suffix = separator + suffix
	}

	for i, s := range series {
		name := s.Name
		// Remove prefix if it ends with ':' and is not empty
		if prefix != "" && strings.HasPrefix(name, prefix) {
			name = name[len(prefix):]
		}
		// Remove suffix if it starts with ':' and is not empty
		if suffix != "" && strings.HasSuffix(name, suffix) {
			name = name[:len(name)-len(suffix)]
		}
		name = strings.Trim(name, separator+"_ ")
		series[i].Name = name
	}
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
		seriesNames := []string{ss.Meta.Name + "#avg", ss.Meta.Name + "#last"}
		seriesFlags := map[string]int{}
		for i, seriesName := range seriesNames {
			if opt.SeriesSelector != nil && !opt.SeriesSelector.Match(seriesName) {
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
	case "odometer":
		typ, stack := opt.Type.TypeAndStack("bar")
		series = []Series{
			{
				Name:       ss.Meta.Name,
				Type:       typ,
				Stack:      stack,
				Data:       make([]Item, len(ss.Times)),
				Smooth:     true,
				ShowSymbol: false,
			},
		}
		for i, t := range ss.Times {
			series[0].Data[i].Time = t.UnixMilli()
			if v, ok := ss.Values[i].(*OdometerValue); ok && v.Samples > 0 {
				series[0].Data[i].Value = v.Diff()
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
				Name:       ss.Meta.Name + "#" + pName,
				Type:       "line",
				Data:       make([]Item, len(ss.Times)),
				Smooth:     true,
				ShowSymbol: false,
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
	Meta        SeriesInfo
}

func (d Dashboard) getSnapshot(expvarKey string, tsIdx int) (Snapshot, bool) {
	var ret Snapshot
	mts := d.timeseriesProvider(expvarKey)
	if mts == nil {
		return ret, false
	}
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
			Meta:        ts.Meta().(SeriesInfo),
		}
	}
	return ret, true
}
