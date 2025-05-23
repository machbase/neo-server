package templ

import (
	"database/sql"
	"errors"
	htmTemplate "html/template"
	"io"
	"net"
	txtTemplate "text/template"

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
	output   io.Writer
	format   Format
	template string
	tmpl     Engine
	record   *TemplObj
	rownum   int
	colNames []string
}

func NewEncoder(format Format) *Exporter {
	rr := &Exporter{format: format}
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
	var err error
	if ex.format == HTML {
		ex.tmpl, err = htmTemplate.New("row").Parse(ex.template)
	} else {
		ex.tmpl, err = txtTemplate.New("row").Parse(ex.template)
	}
	return err
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
	for i, val := range values {
		switch v := val.(type) {
		case *float64:
			values[i] = *v
		case float64:
			values[i] = v
		case *float32:
			values[i] = float64(*v)
		case float32:
			values[i] = float64(v)
		case *int:
			values[i] = *v
		case int:
			values[i] = v
		case *int8:
			values[i] = int(*v)
		case int8:
			values[i] = int(v)
		case *int16:
			values[i] = int(*v)
		case int16:
			values[i] = int(v)
		case *int32:
			values[i] = int(*v)
		case int32:
			values[i] = int(v)
		case *int64:
			values[i] = int(*v)
		case int64:
			values[i] = int(v)
		case *net.IP:
			values[i] = v.String()
		case net.IP:
			values[i] = v.String()
		case *sql.NullBool:
			if v.Valid {
				values[i] = v.Bool
			}
		case *sql.NullByte:
			if v.Valid {
				values[i] = v.Byte
			}
		case *sql.NullFloat64:
			if v.Valid {
				values[i] = v.Float64
			}
		case *sql.NullInt16:
			if v.Valid {
				values[i] = v.Int16
			}
		case *sql.NullInt32:
			if v.Valid {
				values[i] = v.Int32
			}
		case *sql.Null[float32]:
			if v.Valid {
				values[i] = v.V
			}
		case *sql.NullInt64:
			if v.Valid {
				values[i] = v.Int64
			}
		case *sql.NullString:
			if v.Valid {
				values[i] = v.String
			}
			// case *sql.NullTime:
			// 	if v.Valid {
			// 		values[i] = ex.timeformatter.Format(v.Time)
			// 	}
			// case *sql.Null[net.IP]:
			// 	if v.Valid {
			// 		ex.values[i] = v.V.String()
			// 	}
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
