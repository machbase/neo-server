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

	"github.com/machbase/neo-server/mods/codec/logger"
	"github.com/machbase/neo-server/mods/stream/spec"
	"github.com/machbase/neo-server/mods/util/snowflake"
)

type Chart struct {
	logger       logger.Logger
	output       spec.OutputStream
	toJsonOutput bool
	option       string
	data         [][]any

	// Common template
	ChartID string
	Width   string
	Height  string
	Theme   string
	// HTML template
	AssetsHost   string
	PageTitle    string
	JSAssets     []string
	CSSAssets    []string
	JSFunctions  []string
	ChartActions *ChartActions
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
		JSAssets: []string{
			"/web/echarts/echarts.min.js",
		},
		ChartActions: &ChartActions{},
	}
}

func (c *Chart) ContentType() string {
	if c.toJsonOutput {
		return "application/json"
	}
	return "text/html"
}

func (c *Chart) SetLogger(l logger.Logger) {
	c.logger = l
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

func (c *Chart) SetAssetHost(path string) {
	c.AssetsHost = path
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

func (c *Chart) ChartOptionNoEscaped() template.HTML {
	return template.HTML(c.option)
}

func (c *Chart) ChartActionsNoEscaped() template.HTML {
	return template.HTML(`{"areas": {}, "type": ""}`)
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

func (c *Chart) Close() {
	if c.output == nil {
		return
	}
	if c.Theme == "" {
		c.Theme = "white"
	}
	if c.option != "" {
		for i, d := range c.data {
			name := fmt.Sprintf("value(%d)", i)
			if !strings.Contains(c.option, name) {
				continue
			}
			jsonData, err := json.Marshal(d)
			if err != nil {
				jsonData = []byte(err.Error())
			}
			c.option = strings.ReplaceAll(c.option, name, string(jsonData))
		}
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
