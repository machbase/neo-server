package chart

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/machbase/neo-server/mods/codec/facility"
	"github.com/machbase/neo-server/mods/stream/spec"
	"github.com/machbase/neo-server/mods/util/snowflake"
)

type Chart struct {
	output       spec.OutputStream
	toJsonOutput bool
	option       string
	data         [][]any
	plugins      []string

	logger             facility.Logger
	volatileFileWriter facility.VolatileFileWriter

	// Common template
	ChartID string
	Width   string
	Height  string
	Theme   string
	// HTML template
	PageTitle      string
	JSAssets       []string
	CSSAssets      []string
	JSCodes        []string
	JSCodeAssets   []string
	DispatchAction string
}

type ChartActions struct {
}

var idGen, _ = snowflake.NewNode(time.Now().Unix() % 1024)

func NewChart() *Chart {
	var id = "snowyDayCharty"
	if idGen != nil {
		id = strings.TrimSuffix(idGen.Generate().Base64(), "=")
	}
	return &Chart{
		ChartID: id,
		Width:   "600px",
		Height:  "600px",
	}
}

func (c *Chart) ContentType() string {
	if c.toJsonOutput {
		return "application/json"
	}
	return "text/html"
}

func (c *Chart) SetLogger(l facility.Logger) {
	c.logger = l
}

func (c *Chart) SetVolatileFileWriter(w facility.VolatileFileWriter) {
	c.volatileFileWriter = w
}

func (c *Chart) SetOutputStream(o spec.OutputStream) {
	c.output = o
}

func (c *Chart) SetChartId(id string) {
	c.ChartID = id
}

func (c *Chart) SetSize(width, height string) {
	c.Width = width
	c.Height = height
}

func (c *Chart) SetTheme(theme string) {
	c.Theme = theme
}

func (c *Chart) SetChartJson(flag bool) {
	c.toJsonOutput = flag
}

func (c *Chart) SetChartOption(opt string) {
	opt = strings.TrimSpace(opt)
	if !strings.HasPrefix(opt, "{") {
		opt = "{" + opt + "}"
	}
	c.option = opt
}

func (c *Chart) SetPlugins(plugins ...string) {
	c.plugins = append(c.plugins, plugins...)
}

func (c *Chart) SetChartAssets(args ...string) {
	for _, url := range args {
		if strings.HasSuffix(url, ".css") {
			c.JSAssets = append(c.CSSAssets, url)
		} else {
			c.JSAssets = append(c.JSAssets, url)
		}
	}
}

func (c *Chart) SetChartJSCode(js string) {
	c.JSCodes = append(c.JSCodes, js)
}

func (c *Chart) SetChartDispatchAction(action string) {
	action = strings.TrimSpace(action)
	if !strings.HasPrefix(action, "{") {
		action = "{" + action + "}"
	}
	c.DispatchAction = action
}

func (c *Chart) JSAssetsNoEscaped() template.HTML {
	lst := []string{}
	for _, itm := range c.JSAssets {
		lst = append(lst, fmt.Sprintf("%q", itm))
	}
	return template.HTML("[" + strings.Join(lst, ",") + "]")
}

func (c *Chart) CSSAssetsNoEscaped() template.HTML {
	lst := []string{}
	for _, itm := range c.CSSAssets {
		lst = append(lst, fmt.Sprintf("%q", itm))
	}
	return template.HTML("[" + strings.Join(lst, ",") + "]")
}

func (c *Chart) JSCodeAssetsNoEscaped() template.HTML {
	lst := []string{}
	for _, itm := range c.JSCodeAssets {
		lst = append(lst, fmt.Sprintf("%q", itm))
	}
	return template.HTML("[" + strings.Join(lst, ",") + "]")
}

func (c *Chart) Open() error {
	return nil
}

func (c *Chart) Flush(heading bool) {
}

func (c *Chart) AddRow(values []any) error {
	if c.data == nil {
		c.data = [][]any{}
	}
	for i, val := range values {
		if len(c.data) < i+1 {
			c.data = append(c.data, []any{})
		}
		c.data[i] = append(c.data[i], convValue(val))
	}
	return nil
}

func convValue(val any) (ret any) {
	switch v := val.(type) {
	case []any:
		for i, elm := range v {
			v[i] = convValue(elm)
		}
		ret = v
	case *time.Time:
		// t := v.UnixNano()
		// ret = float64(t/int64(time.Millisecond)) + float64(t%int64(time.Millisecond))/float64(time.Millisecond)
		ret = v.UnixMilli()
	case time.Time:
		// t := v.UnixNano()
		// ret = float64(t/int64(time.Millisecond)) + float64(t%int64(time.Millisecond))/float64(time.Millisecond)
		ret = v.UnixMilli()
	default:
		ret = v
	}
	return
}

var themeNames = map[string]bool{
	"white":          true,
	"dark":           true,
	"essos":          true,
	"chalk":          true,
	"purple-passion": true,
	"romantic":       true,
	"walden":         true,
	"westeros":       true,
	"wonderland":     true,
	"vintage":        true,
	"macarons":       true,
	"infographic":    true,
	"shine":          true,
	"roma":           true,
}

var pluginNames = map[string]bool{
	"liquidfill": true,
	"wordcloud":  true,
}

func (c *Chart) Close() {
	if c.output == nil {
		return
	}
	if c.Theme == "" {
		c.Theme = "white"
	}
	if c.option != "" {
		for i, d := range c.data {
			jsonData, err := json.Marshal(d)
			if err != nil {
				jsonData = []byte(err.Error())
			}
			exp := getValueRegexp(i)
			c.option = exp.ReplaceAllString(c.option, string(jsonData))
		}
	}
	if len(c.JSAssets) == 0 {
		c.JSAssets = append(c.JSAssets, "/web/echarts/echarts.min.js")
	}
	if _, ok := themeNames[c.Theme]; ok {
		if c.Theme != "white" {
			c.JSAssets = append(c.JSAssets, fmt.Sprintf("/web/echarts/themes/%s.js", c.Theme))
		}
	} else {
		if strings.HasPrefix(c.Theme, "http://") || strings.HasPrefix(c.Theme, "https://") {
			c.JSAssets = append(c.JSAssets, c.Theme)
		}
	}
	for _, plugin := range c.plugins {
		if _, ok := pluginNames[plugin]; ok {
			c.JSAssets = append(c.JSAssets, fmt.Sprintf("/web/echarts/echarts-%s.min.js", plugin))
		} else {
			c.JSAssets = append(c.JSAssets, plugin)
		}
	}
	if c.volatileFileWriter != nil {
		prefix := c.volatileFileWriter.VolatileFilePrefix()
		path := fmt.Sprintf("%s/%s.js", strings.TrimSuffix(prefix, "/"), c.ChartID)

		codes := []string{}
		codes = append(codes, `"use strict";`)
		codes = append(codes, fmt.Sprintf(`let chart = echarts.init(document.getElementById('%s'), "%s");`, c.ChartID, c.Theme))
		codes = append(codes, fmt.Sprintf(`chart.setOption(%s);`, c.option))
		if c.DispatchAction == "" {
			codes = append(codes, `chart.dispatchAction({"areas": {}, "type": ""});`)
		} else {
			codes = append(codes, fmt.Sprintf(`chart.dispatchAction(%s);`, c.DispatchAction))
		}
		codes = append(codes, c.JSCodes...)
		jscode := fmt.Sprintf("(()=>{\n%s\n})();", strings.Join(codes, "\n"))
		c.volatileFileWriter.VolatileFileWrite(path, []byte(jscode), time.Now().Add(30*time.Second))
		c.JSCodeAssets = append(c.JSCodeAssets, path)
	}
	if c.toJsonOutput {
		c.RenderJSON()
	} else {
		c.Render()
	}
}

// isSet check if the field exist in the chart instance
// Shamed copy from https://stackoverflow.com/questions/44675087/golang-template-variable-isset
func isSet(name string, data interface{}) bool {
	v := reflect.ValueOf(data)

	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		return false
	}

	return v.FieldByName(name).IsValid()
}

func (c *Chart) RenderJSON() {
	contents := []string{ChartJsonTemplate}
	tpl := template.New("chart")
	tpl = template.Must(tpl.Parse(contents[0]))

	if err := tpl.ExecuteTemplate(c.output, "chart", c); err != nil {
		c.output.Write([]byte(err.Error()))
	}
}

func (c *Chart) Render() {
	contents := []string{HeaderTemplate, BaseTemplate, ChartTemplate}
	tpl := template.New("chart").Funcs(template.FuncMap{
		"safeJS": func(s interface{}) template.JS {
			return template.JS(fmt.Sprint(s))
		},
		"isSet": isSet,
	})
	tpl = template.Must(tpl.Parse(contents[0]))
	for _, cont := range contents[1:] {
		tpl = template.Must(tpl.Parse(cont))
	}

	var buf bytes.Buffer
	if err := tpl.ExecuteTemplate(&buf, "chart", c); err != nil {
		buf.WriteString(err.Error())
	}

	content := pat.ReplaceAll(buf.Bytes(), []byte(""))
	if _, err := c.output.Write(content); err != nil {
		c.output.Write([]byte(err.Error()))
	}
}

var (
	pat = regexp.MustCompile(`(__f__")|("__f__)|(__f__)`)
)

var valueRegexpCache = map[int]*regexp.Regexp{}

func init() {
	for i := 0; i < 20; i++ {
		_ = getValueRegexp(i)
	}
}

func getValueRegexp(idx int) *regexp.Regexp {
	if r, ok := valueRegexpCache[idx]; !ok {
		pattern := fmt.Sprintf(`(column\s*\(\s*%d\s*\))`, idx)
		r = regexp.MustCompile(pattern)
		valueRegexpCache[idx] = r
		return r
	} else {
		return r
	}
}
