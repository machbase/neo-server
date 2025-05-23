package templ

import (
	"errors"
	"html/template"
	"io"

	"github.com/machbase/neo-server/v8/mods/codec/internal"
)

type Exporter struct {
	internal.RowsEncoderBase
	output   io.Writer
	template string
	tmpl     *template.Template
	record   *TemplObj
	rownum   int
	colNames []string
}

func NewEncoder() *Exporter {
	rr := &Exporter{}
	return rr
}

func (ex *Exporter) ContentType() string {
	return "application/xhtml+xml"
}

func (ex *Exporter) SetOutputStream(o io.Writer) {
	ex.output = o
}

func (ex *Exporter) SetTemplate(template string) {
	ex.template = template
}

func (ex *Exporter) SetColumns(colNames ...string) {
	ex.colNames = colNames
}

func (ex *Exporter) Open() error {
	tmpl, err := template.New("row").Parse(ex.template)
	if err != nil {
		return err
	}
	ex.tmpl = tmpl
	return nil
}

func (ex *Exporter) Close() {
	if ex.record != nil {
		ex.record.IsLast = true
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
	ex.record = &TemplObj{
		ROW:     make(map[string]any),
		Values:  values,
		ROWNUM:  ex.rownum,
		IsFirst: ex.rownum == 1,
	}
	if len(values) == 1 {
		ex.record.Values = values[0]
	}
	for i := 0; i < len(values) && i < len(ex.colNames); i++ {
		ex.record.ROW[ex.colNames[i]] = values[i]
	}
	return nil
}

type TemplObj struct {
	ROW     map[string]any
	ROWNUM  int
	Values  any
	IsFirst bool
	IsLast  bool
}
