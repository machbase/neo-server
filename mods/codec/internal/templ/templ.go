package templ

import (
	"errors"
	htmTemplate "html/template"
	"io"
	txtTemplate "text/template"

	"github.com/machbase/neo-server/v8/api"
	"github.com/machbase/neo-server/v8/mods/codec/internal"
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

func (ex *Exporter) Open() error {
	if ex.format == HTML {
		var tmpl = htmTemplate.New("html_layout")
		for _, content := range ex.templates {
			if _, err := tmpl.Parse(content); err != nil {
				return err
			}
		}
		tmpl.Funcs(map[string]any{})
		ex.tmpl = tmpl
	} else {
		var tmpl = txtTemplate.New("text_layout")
		for _, content := range ex.templates {
			if _, err := tmpl.Parse(content); err != nil {
				return err
			}
		}
		tmpl.Funcs(map[string]any{})
		ex.tmpl = tmpl
	}
	return nil
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
