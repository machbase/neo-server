package csv

import (
	"encoding/csv"
	"fmt"
	"strconv"
	"time"
	"unicode/utf8"

	spi "github.com/machbase/neo-spi"
)

type Exporter struct {
	rownum int64

	writer *csv.Writer
	Comma  rune

	ctx *spi.RowsEncoderContext
}

func NewEncoder(ctx *spi.RowsEncoderContext, delimiter string) spi.RowsEncoder {
	rr := &Exporter{ctx: ctx}
	rr.SetDelimiter(delimiter)
	return rr
}

func (ex *Exporter) SetDelimiter(delimiter string) {
	delmiter, _ := utf8.DecodeRuneInString(delimiter)
	ex.Comma = delmiter
}

func (ex *Exporter) ContentType() string {
	return "text/csv"
}

func (ex *Exporter) Open(cols spi.Columns) error {
	ex.writer = csv.NewWriter(ex.ctx.Output)

	if ex.Comma != 0 {
		ex.writer.Comma = ex.Comma
	}

	colNames := cols.Names()
	if ex.ctx.Heading {
		// TODO check if write() returns error, when csvWritter.Comma is not valid
		if ex.ctx.Rownum {
			ex.writer.Write(append([]string{"ROWNUM"}, colNames...))
		} else {
			ex.writer.Write(colNames)
		}
	}

	return nil
}

func (ex *Exporter) Close() {
	ex.writer.Flush()
	ex.ctx.Output.Close()
}

func (ex *Exporter) Flush(heading bool) {
	ex.writer.Flush()
	ex.ctx.Output.Flush()
}

func (ex *Exporter) AddRow(values []any) error {
	var cols = make([]string, len(values))

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
		case float64:
			if ex.ctx.Precision < 0 {
				cols[i] = fmt.Sprintf("%f", v)
			} else {
				cols[i] = fmt.Sprintf("%.*f", ex.ctx.Precision, v)
			}
		case *int:
			cols[i] = strconv.FormatInt(int64(*v), 10)
		case int:
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
		return ex.writer.Write(append([]string{strconv.FormatInt(ex.rownum, 10)}, cols...))
	} else {
		return ex.writer.Write(cols)
	}
}
