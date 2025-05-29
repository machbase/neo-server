package templ

import (
	"errors"
	"fmt"
	htmlTemplate "html/template"
	"io"
	"strings"
	txtTemplate "text/template"
	"time"

	"github.com/machbase/neo-server/v8/api"
	"github.com/machbase/neo-server/v8/mods/codec/internal"
	"github.com/machbase/neo-server/v8/mods/util"
)

type Format string

const (
	HTML Format = "html"
	TEXT Format = "text"
)

type Engine interface {
	Execute(wr io.Writer, data any) error
}

type Exporter struct {
	internal.RowsEncoderBase
	output      io.Writer
	format      Format
	contentType string
	templates   []string
	tmpl        Engine
	record      *Record
	rownum      int
	colNames    []string
	params      map[string][]string
}

func NewEncoder(format Format) *Exporter {
	rr := &Exporter{format: format}
	if format == TEXT {
		rr.contentType = "text/plain"
	} else {
		rr.contentType = "application/xhtml+xml"
	}
	return rr
}

func (ex *Exporter) ContentType() string {
	return ex.contentType
}

func (ex *Exporter) SetOutputStream(o io.Writer) {
	ex.output = o
}

func (ex *Exporter) SetContentType(contentType string) {
	ex.contentType = contentType
}

func (ex *Exporter) SetTemplate(templates ...string) {
	ex.templates = append(ex.templates, templates...)
}

func (ex *Exporter) SetColumns(colNames ...string) {
	ex.colNames = colNames
}

func (ex *Exporter) ExportParams(params map[string][]string) {
	if ex.params == nil {
		ex.params = make(map[string][]string)
	}
	for k, v := range params {
		ex.params[k] = make([]string, len(v))
		copy(ex.params[k], v)
	}
}

func (ex *Exporter) Open() error {
	if ex.format == HTML {
		var tmpl = htmlTemplate.New("html_layout").Funcs(tmplFuncs(ex.params))
		for _, content := range ex.templates {
			if _, err := tmpl.Parse(content); err != nil {
				return err
			}
		}
		ex.tmpl = tmpl
	} else {
		var tmpl = txtTemplate.New("text_layout").Funcs(tmplFuncs(ex.params))
		for _, content := range ex.templates {
			if _, err := tmpl.Parse(content); err != nil {
				return err
			}
		}
		ex.tmpl = tmpl
	}
	return nil
}

func tmplFuncs(params map[string][]string) htmlTemplate.FuncMap {
	return htmlTemplate.FuncMap{
		"timeformat": func(s any, z any, v any) string {
			tz, err := util.ParseTimeLocation(fmt.Sprint(z), time.Local)
			if err != nil {
				return fmt.Sprintf("Invalid timezone: %v", err)
			}
			ts, err := util.ToTime(v)
			if err != nil {
				return fmt.Sprintf("Invalid time: %v", err)
			}
			tf := util.NewTimeFormatter(util.Timeformat(fmt.Sprint(s)), util.TimeLocation(tz))
			return tf.Format(ts)
		},
		"format": func(format string, v any) string {
			return fmt.Sprintf(format, v)
		},
		"param": func(name string) string {
			if params == nil {
				return ""
			}
			if value, ok := params[name]; ok {
				return value[0]
			}
			return ""
		},
		"paramDefault": func(name, def string) string {
			if params == nil {
				return def
			}
			if value, ok := params[name]; ok {
				return value[0]
			}
			return def
		},
		"toUpper": func(s string) string {
			return strings.ToUpper(s)
		},
		"toLower": func(s string) string {
			return strings.ToLower(s)
		},
	}
}

func (ex *Exporter) Close() {
	if ex.record != nil {
		ex.record.IsLast = true
		ex.tmpl.Execute(ex.output, ex.record)
	} else if ex.rownum == 0 {
		// If no rows were added, we still need to execute the template
		ex.record = &Record{
			Num:      0,
			IsFirst:  true,
			IsLast:   true,
			IsEmpty:  true,
			colNames: ex.colNames,
		}
		ex.tmpl.Execute(ex.output, ex.record)
	}

	if ex.output != nil {
		if w, ok := ex.output.(io.Closer); ok {
			w.Close()
		}
	}
}

func (ex *Exporter) Flush(heading bool) {
}

func (ex *Exporter) AddRow(values []any) error {
	ex.rownum++
	if ex.tmpl == nil {
		return errors.New("template is not set")
	}
	if ex.record != nil {
		err := ex.tmpl.Execute(ex.output, ex.record)
		if err != nil {
			return err
		}
	}
	for i, val := range values {
		values[i] = api.Unbox(val)
	}
	ex.record = &Record{
		values:   values,
		Num:      ex.rownum,
		IsFirst:  ex.rownum == 1,
		IsEmpty:  len(values) == 0,
		colNames: ex.colNames,
	}
	return nil
}

type Record struct {
	Num      int
	IsFirst  bool
	IsLast   bool
	IsEmpty  bool
	colNames []string
	values   []any
	v        map[string]any
}

func (to *Record) ValueHTML(idx int) htmlTemplate.HTML {
	return htmlTemplate.HTML(to.ValueString(idx))
}

func (to *Record) ValueHTMLAttr(idx int) htmlTemplate.HTMLAttr {
	return htmlTemplate.HTMLAttr(to.ValueString(idx))
}

func (to *Record) ValueCSS(idx int) htmlTemplate.CSS {
	return htmlTemplate.CSS(to.ValueString(idx))
}

func (to *Record) ValueJS(idx int) htmlTemplate.JS {
	return htmlTemplate.JS(to.ValueString(idx))
}

func (to *Record) ValueURL(idx int) htmlTemplate.URL {
	return htmlTemplate.URL(to.ValueString(idx))
}

func (to *Record) ValueString(idx int) string {
	return fmt.Sprint(to.Value(idx))
}

func (to *Record) Value(idx int) any {
	if idx < 0 || idx >= len(to.values) {
		return nil
	}
	return to.values[idx]
}

func (to *Record) Values() []any {
	return to.values
}

func (to *Record) V() map[string]any {
	if to.v == nil {
		to.v = make(map[string]any)
		for i := 0; i < len(to.values) && i < len(to.colNames); i++ {
			to.v[to.colNames[i]] = to.values[i]
		}
	}
	return to.v
}
