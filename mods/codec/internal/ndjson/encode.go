package ndjson

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net"
	"time"

	"github.com/machbase/neo-server/v8/api"
	"github.com/machbase/neo-server/v8/mods/codec/internal"
	"github.com/machbase/neo-server/v8/mods/util"
)

type Exporter struct {
	internal.RowsEncoderBase
	tick time.Time
	nrow int

	output     io.Writer
	Rownum     bool
	Heading    bool
	precision  int
	timeformat *util.TimeFormatter

	colNames []string
	colTypes []api.DataType
}

func NewEncoder() *Exporter {
	return &Exporter{
		tick:       time.Now(),
		timeformat: util.NewTimeFormatter(),
	}
}

func (ex *Exporter) ContentType() string {
	return "application/x-ndjson"
}

func (ex *Exporter) SetOutputStream(o io.Writer) {
	ex.output = o
}

func (ex *Exporter) SetTimeformat(format string) {
	if format == "" {
		return
	}
	ex.timeformat.Set(util.Timeformat(format))
}

func (ex *Exporter) SetTimeLocation(tz *time.Location) {
	if tz == nil {
		return
	}
	ex.timeformat.Set(util.TimeLocation(tz))
}

func (ex *Exporter) SetPrecision(precision int) {
	ex.precision = precision
}

func (ex *Exporter) SetRownum(show bool) {
	ex.Rownum = show
}

func (ex *Exporter) SetHeader(show bool) {
	ex.Heading = show
}

func (ex *Exporter) SetHeading(show bool) {
	ex.Heading = show
}

func (ex *Exporter) SetColumns(labels ...string) {
	ex.colNames = labels
}

func (ex *Exporter) SetColumnTypes(types ...api.DataType) {
	ex.colTypes = types
}

func (ex *Exporter) Open() error {
	return nil
}

func (ex *Exporter) Close() {
	ex.output.Write([]byte("\n"))
	if closer, ok := ex.output.(io.Closer); ok {
		closer.Close()
	}
}

func (ex *Exporter) Flush(heading bool) {
	if flusher, ok := ex.output.(api.Flusher); ok {
		flusher.Flush()
	}
}

func (ex *Exporter) encodeFloat64(v float64) any {
	if math.IsNaN(v) {
		return "NaN"
	} else if math.IsInf(v, -1) {
		return "-Inf"
	} else if math.IsInf(v, 1) {
		return "+Inf"
	}
	return v
}

func (ex *Exporter) AddRow(source []any) error {
	ex.nrow++

	values := make([]any, len(source))
	for i, field := range source {
		switch v := field.(type) {
		case *time.Time:
			values[i] = ex.timeformat.FormatEpoch(*v)
		case time.Time:
			values[i] = ex.timeformat.FormatEpoch(v)
		case *float64:
			values[i] = ex.encodeFloat64(*v)
		case float64:
			values[i] = ex.encodeFloat64(v)
		case *float32:
			values[i] = ex.encodeFloat64(float64(*v))
		case float32:
			values[i] = ex.encodeFloat64(float64(v))
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
		case *sql.NullInt64:
			if v.Valid {
				values[i] = v.Int64
			}
		case *sql.NullString:
			if v.Valid {
				values[i] = v.String
			}
		case *sql.NullTime:
			if v.Valid {
				values[i] = ex.timeformat.Format(v.Time)
			}
		default:
			values[i] = field
		}
	}

	if len(values) != len(ex.colNames) {
		return fmt.Errorf("rows[%d] number of columns not matched (%d); table '%s' has %d columns",
			ex.nrow, len(values), ex.colNames, len(ex.colNames))
	}
	var recJson []byte
	var err error
	var vs = map[string]any{}
	if ex.Rownum {
		vs["ROWNUM"] = ex.nrow
	}
	for i, v := range values {
		if i >= len(ex.colNames) {
			break
		}
		vs[ex.colNames[i]] = v
	}
	recJson, err = json.Marshal(vs)
	if err != nil {
		return err
	}
	ex.output.Write(recJson)
	ex.output.Write([]byte("\n"))

	return nil
}
