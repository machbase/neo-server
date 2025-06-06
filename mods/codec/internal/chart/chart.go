package chart

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/machbase/neo-server/v8/mods/codec/facility"
	"github.com/machbase/neo-server/v8/mods/codec/internal"
	"github.com/machbase/neo-server/v8/mods/util/snowflake"
)

type Chart struct {
	internal.RowsEncoderBase
	output       io.Writer
	toJsonOutput bool
	option       string
	data         [][]any
	plugins      []string

	logger             facility.Logger
	volatileFileWriter facility.VolatileFileWriter
	typeHint           map[int]string

	// Common template
	ChartID string
	Width   string
	Height  string
	Theme   string
	// HTML template
	PageTitle      string
	JSAssets       []string
	CSSAssets      []string
	JSCodeAssets   []string
	DispatchAction string

	jsCodesPre  []string
	jsCodesPost []string

	isCompatibleMode bool
}

type ChartActions struct {
}

func NewChart() *Chart {
	return &Chart{
		ChartID: snowflake.Generate(),
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

func (c *Chart) SetOutputStream(o io.Writer) {
	c.output = o
}

func (c *Chart) SetChartID(id string) {
	c.ChartID = id
}

// deprecated, use SetChartID
// keep this version for the compatibility
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
	if c.option == "" {
		c.jsCodesPre = append(c.jsCodesPre, js)
	} else {
		c.jsCodesPost = append(c.jsCodesPost, js)
	}
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
	if len(values) == 1 {
		if v, ok := values[0].(map[string]any); ok {
			opt, err := json.Marshal(v)
			if err != nil {
				return err
			}
			c.option = string(opt)
			return nil
		}
	}
	if c.data == nil {
		c.data = [][]any{}
	}
	for i, val := range values {
		if len(c.data) < i+1 {
			c.data = append(c.data, []any{})
		}
		convVal, hint := convValueType(val)
		c.data[i] = append(c.data[i], convVal)
		if hint != "" {
			if c.typeHint == nil {
				c.typeHint = map[int]string{}
			}
			c.typeHint[i] = hint
		}
	}
	return nil
}

func convValue(val any) (ret any) {
	v, _ := convValueType(val)
	return v
}

func convValueType(val any) (ret any, typeHint string) {
	switch v := val.(type) {
	case []any:
		for i, elm := range v {
			v[i] = convValue(elm)
		}
		ret = v
		typeHint = ""
	case *time.Time:
		ret = float64(v.UnixMicro()) / 1000
		typeHint = "time"
	case time.Time:
		ret = float64(v.UnixMicro()) / 1000
		typeHint = "time"
	default:
		ret = v
		typeHint = ""
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

var pluginNames = map[string]string{
	"liquidfill": "/web/echarts/echarts-liquidfill.min.js",
	"wordcloud":  "/web/echarts/echarts-wordcloud.min.js",
	"gl":         "/web/echarts/echarts-gl.min.js",
}

func (c *Chart) Close() {
	if c.output == nil {
		return
	}
	if c.Theme == "" {
		c.Theme = "white"
	}

	if !c.isCompatibleMode {
		columnVarNames := make([]string, len(c.data))
		for i, d := range c.data {
			columnVal := `""`
			if jsonData, err := json.Marshal(d); err != nil {
				columnVal = fmt.Sprintf("%q", err.Error())
			} else {
				columnVal = string(jsonData)
			}
			columnVarNames[i] = fmt.Sprintf("_column_%d", i)

			c.jsCodesPre = append(c.jsCodesPre, fmt.Sprintf("const %s=%s;", columnVarNames[i], columnVal))
		}
		c.jsCodesPre = append(c.jsCodesPre, fmt.Sprintf("const _columns=[%s];", strings.Join(columnVarNames, ",")))
		c.jsCodesPre = append(c.jsCodesPre, `function column(idx) { return _columns[idx]; }`)
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
		if path, ok := pluginNames[plugin]; ok {
			c.JSAssets = append(c.JSAssets, path)
		} else {
			c.JSAssets = append(c.JSAssets, plugin)
		}
	}
	if c.volatileFileWriter != nil {
		prefix := c.volatileFileWriter.VolatileFilePrefix()
		path := fmt.Sprintf("%s/%s.js", strings.TrimSuffix(prefix, "/"), c.ChartID)

		codes := []string{}
		codes = append(codes, fmt.Sprintf(`let _chartID = '%s';`, c.ChartID))
		codes = append(codes, fmt.Sprintf(`let _chart = echarts.init(document.getElementById(_chartID), "%s");`, c.Theme))
		if c.option != "" {
			codes = append(codes, fmt.Sprintf(`let _chartOption = %s;`, c.option))
			codes = append(codes, `_chart.setOption(_chartOption);`)
		}
		if c.DispatchAction == "" {
			codes = append(codes, `_chart.dispatchAction({"areas": {}, "type": ""});`)
		} else {
			codes = append(codes, fmt.Sprintf(`_chart.dispatchAction(%s);`, c.DispatchAction))
		}
		codes = append(c.jsCodesPre, codes...)
		codes = append(codes, c.jsCodesPost...)
		strings.Join(codes, "\n")
		jscode := fmt.Sprintf("(()=>{\n\"use strict\";\n%s\n})();", strings.Join(codes, "\n"))
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
