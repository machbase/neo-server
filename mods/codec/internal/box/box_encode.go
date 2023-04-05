package box

import (
	"fmt"
	"strconv"
	"time"

	"github.com/jedib0t/go-pretty/v6/table"
	spi "github.com/machbase/neo-spi"
)

type Exporter struct {
	writer table.Writer
	rownum int64
	ctx    *spi.RowsEncoderContext

	Style           string
	SeparateColumns bool
	DrawBorder      bool
}

func NewEncoder(ctx *spi.RowsEncoderContext, style string, separateColumns bool, drawBorder bool) spi.RowsEncoder {
	return &Exporter{
		ctx:             ctx,
		Style:           style,
		SeparateColumns: separateColumns,
		DrawBorder:      drawBorder,
	}
}

func (ex *Exporter) ContentType() string {
	return "plain/text"
}

func (ex *Exporter) Open(cols spi.Columns) error {
	ex.writer = table.NewWriter()
	ex.writer.SetOutputMirror(ex.ctx.Output)

	style := table.StyleDefault
	switch ex.Style {
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
	style.Options.SeparateColumns = ex.SeparateColumns
	style.Options.DrawBorder = ex.DrawBorder

	ex.writer.SetStyle(style)

	colNames := cols.NamesWithTimeLocation(ex.ctx.TimeLocation)
	if ex.ctx.Heading {
		vs := make([]any, len(colNames))
		for i, h := range colNames {
			vs[i] = h
		}
		if ex.ctx.Rownum {
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
	ex.ctx.Output.Close()
}

func (ex *Exporter) Flush(heading bool) {
	ex.writer.Render()
	ex.ctx.Output.Flush()

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
		case *time.Time:
			switch ex.ctx.TimeFormat {
			case "ns":
				cols[i] = strconv.FormatInt(v.UnixNano(), 10)
			case "ms":
				cols[i] = strconv.FormatInt(v.UnixMilli(), 10)
			case "us":
				cols[i] = strconv.FormatInt(v.UnixMicro(), 10)
			case "s":
				cols[i] = strconv.FormatInt(v.Unix(), 10)
			default:
				if ex.ctx.TimeLocation == nil {
					ex.ctx.TimeLocation = time.UTC
				}
				cols[i] = v.In(ex.ctx.TimeLocation).Format(ex.ctx.TimeFormat)
			}
		case *float64:
			if ex.ctx.Precision < 0 {
				cols[i] = fmt.Sprintf("%f", *v)
			} else {
				cols[i] = fmt.Sprintf("%.*f", ex.ctx.Precision, *v)
			}
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
		default:
			cols[i] = fmt.Sprintf("%T", r)
		}
	}

	ex.rownum++

	if ex.ctx.Rownum {
		ex.writer.AppendRow(table.Row(append([]any{ex.rownum}, cols...)))
	} else {
		ex.writer.AppendRow(table.Row(cols))
	}

	return nil
}
