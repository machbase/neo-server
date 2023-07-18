package echart

import (
	"bytes"
	"io"
	"regexp"
	"text/template"

	"github.com/go-echarts/go-echarts/v2/render"
)

var (
	pat = regexp.MustCompile(`(__f__")|("__f__)|(__f__)`)
)

type chartRender struct {
	c      any
	before []func()
}

func newChartRender(c any, before ...func()) render.Renderer {
	return &chartRender{c: c, before: before}
}

func (r *chartRender) Render(w io.Writer) error {
	for _, fn := range r.before {
		fn()
	}

	contents := []string{HeaderTpl, BaseTpl, ChartTpl}
	tpl := render.MustTemplate(render.ModChart, contents)
	var buf bytes.Buffer
	if err := tpl.ExecuteTemplate(&buf, render.ModChart, r.c); err != nil {
		return err
	}

	content := pat.ReplaceAll(buf.Bytes(), []byte(""))
	_, err := w.Write(content)
	return err
}

type jsonRender struct {
	c      any
	before []func()
}

func newJsonRender(c any, before ...func()) render.Renderer {
	return &jsonRender{c: c, before: before}
}

func (r *jsonRender) Render(w io.Writer) error {
	for _, fn := range r.before {
		fn()
	}

	tpl := template.Must(template.New(render.ModChart).Parse(ChartJsonTpl))
	var buf bytes.Buffer
	if err := tpl.ExecuteTemplate(&buf, render.ModChart, r.c); err != nil {
		return err
	}

	content := pat.ReplaceAll(buf.Bytes(), []byte(""))
	_, err := w.Write(content)
	return err
}

//lint:ignore U1000 type pageRender is unused
type pageRender struct {
	c      interface{}
	before []func()
}

//lint:ignore U1000 func newPageRender is unused
func newPageRender(c any, before ...func()) render.Renderer {
	return &pageRender{c: c, before: before}
}

//lint:ignore U1000 func (*pageRender).Render is unused
func (r *pageRender) Render(w io.Writer) error {
	for _, fn := range r.before {
		fn()
	}

	contents := []string{HeaderTpl, BaseTpl, PageTpl}
	tpl := render.MustTemplate(render.ModPage, contents)

	var buf bytes.Buffer
	if err := tpl.ExecuteTemplate(&buf, render.ModPage, r.c); err != nil {
		return err
	}

	content := pat.ReplaceAll(buf.Bytes(), []byte(""))

	_, err := w.Write(content)
	return err
}
