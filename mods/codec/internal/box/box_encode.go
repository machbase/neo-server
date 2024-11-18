package box

import (
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/machbase/neo-server/v8/mods/codec/internal"
	"github.com/machbase/neo-server/v8/mods/stream/spec"
	"github.com/machbase/neo-server/v8/mods/util"
)

type Exporter struct {
	internal.RowsEncoderBase
	writer table.Writer
	rownum int64

	style           string
	separateColumns bool
	drawBorder      bool
	output          spec.OutputStream
	showRownum      bool
	heading         bool
	precision       int
	timeformatter   *util.TimeFormatter
	binaryFormatter *util.BinaryFormatter
	colNames        []string
}

func NewEncoder() *Exporter {
	return &Exporter{
		style:           "default",
		separateColumns: true,
		drawBorder:      true,
		precision:       -1,
		timeformatter:   util.NewTimeFormatter(),
		binaryFormatter: util.NewBinaryFormatter(),
	}
}

func (ex *Exporter) ContentType() string {
	return "plain/text"
}

func (ex *Exporter) SetOutputStream(o spec.OutputStream) {
	ex.output = o
}

func (ex *Exporter) SetTimeformat(format string) {
	ex.timeformatter.Set(util.Timeformat(format))
}

func (ex *Exporter) SetTimeLocation(tz *time.Location) {
	ex.timeformatter.Set(util.TimeLocation(tz))
}

func (ex *Exporter) SetPrecision(precision int) {
	ex.precision = precision
}

func (ex *Exporter) SetRownum(show bool) {
	ex.showRownum = show
}

func (ex *Exporter) SetHeader(show bool) {
	ex.heading = show
}

func (ex *Exporter) SetHeading(show bool) {
	ex.heading = show
}

func (ex *Exporter) SetBoxStyle(style string) {
	ex.style = style
}

func (ex *Exporter) SetBoxSeparateColumns(flag bool) {
	ex.separateColumns = flag
}

func (ex *Exporter) SetBoxDrawBorder(flag bool) {
	ex.drawBorder = flag
}

func (ex *Exporter) SetColumns(names ...string) {
	ex.colNames = names
}

func (ex *Exporter) Open() error {
	ex.writer = table.NewWriter()
	ex.writer.SetOutputMirror(ex.output)

	style := table.StyleDefault
	switch ex.style {
	case "bold":
		style = table.StyleBold
	case "double":
		style = table.StyleDouble
	case "light":
		style = table.StyleLight
	case "round":
		style = table.StyleRounded
	default:
		style = table.StyleDefault
	}
	style.Options.SeparateColumns = ex.separateColumns
	style.Options.DrawBorder = ex.drawBorder

	ex.writer.SetStyle(style)

	if ex.heading {
		vs := make([]any, len(ex.colNames))
		for i, h := range ex.colNames {
			vs[i] = h
		}
		if ex.showRownum {
			ex.writer.AppendHeader(table.Row(append([]any{"ROWNUM"}, vs...)))
		} else {
			ex.writer.AppendHeader(table.Row(vs))
		}
	}

	return nil
}

func (ex *Exporter) Close() {
	if ex.writer.Length() > 0 {
		ex.writer.Render()
		ex.writer.ResetRows()
	}
	ex.output.Close()
}

func (ex *Exporter) Flush(heading bool) {
	ex.writer.Render()
	ex.output.Flush()

	ex.writer.ResetRows()
	if !heading {
		ex.writer.ResetHeaders()
	}
}

func (ex *Exporter) AddRow(values []any) error {
	var cols = make([]any, len(values))

	for i, r := range values {
		if r == nil {
			cols[i] = "NULL"
			continue
		}
		switch v := r.(type) {
		case *string:
			cols[i] = *v
		case string:
			cols[i] = v
		case *[]byte:
			cols[i] = ex.binaryFormatter.Format(*v)
		case []byte:
			cols[i] = ex.binaryFormatter.Format(v)
		case *time.Time:
			cols[i] = ex.timeformatter.Format(*v)
		case time.Time:
			cols[i] = ex.timeformatter.Format(v)
		case *float32:
			cols[i] = strconv.FormatFloat(float64(*v), 'f', ex.precision, 32)
		case float32:
			cols[i] = strconv.FormatFloat(float64(v), 'f', ex.precision, 32)
		case *float64:
			cols[i] = strconv.FormatFloat(*v, 'f', ex.precision, 64)
		case float64:
			cols[i] = strconv.FormatFloat(v, 'f', ex.precision, 64)
		case *int:
			cols[i] = strconv.FormatInt(int64(*v), 10)
		case int:
			cols[i] = strconv.FormatInt(int64(v), 10)
		case *int8:
			cols[i] = strconv.FormatInt(int64(*v), 10)
		case int8:
			cols[i] = strconv.FormatInt(int64(v), 10)
		case *int16:
			cols[i] = strconv.FormatInt(int64(*v), 10)
		case int16:
			cols[i] = strconv.FormatInt(int64(v), 10)
		case *int32:
			cols[i] = strconv.FormatInt(int64(*v), 10)
		case int32:
			cols[i] = strconv.FormatInt(int64(v), 10)
		case *int64:
			cols[i] = strconv.FormatInt(*v, 10)
		case int64:
			cols[i] = strconv.FormatInt(v, 10)
		case *net.IP:
			cols[i] = v.String()
		case net.IP:
			cols[i] = v.String()
		default:
			cols[i] = fmt.Sprintf("%T", r)
		}
	}

	ex.rownum++

	if ex.showRownum {
		ex.writer.AppendRow(table.Row(append([]any{ex.rownum}, cols...)))
	} else {
		ex.writer.AppendRow(table.Row(cols))
	}

	return nil
}
