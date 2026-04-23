package ndjson

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net"
	"time"

	"github.com/machbase/neo-client/api"
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

type PrecisionFloat64 float64

func (pf PrecisionFloat64) MarshalJSON() ([]byte, error) {
	v := float64(pf)
	switch {
	case math.IsNaN(v):
		return []byte(`"NaN"`), nil
	case math.IsInf(v, -1):
		return []byte(`"-Inf"`), nil
	case math.IsInf(v, 1):
		return []byte(`"+Inf"`), nil
	case v == 0:
		// Keep zero formatting stable and avoid "-0".
		return []byte("0"), nil
	}
	r := []byte(fmt.Sprintf("%f", v))
	for len(r) > 0 && r[len(r)-1] == '0' {
		r = r[:len(r)-1]
	}
	if len(r) > 0 && r[len(r)-1] == '.' {
		r = r[:len(r)-1]
	}
	return r, nil
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
			values[i] = PrecisionFloat64(*v)
		case float64:
			values[i] = PrecisionFloat64(v)
		case *float32:
			values[i] = PrecisionFloat64(float64(*v))
		case float32:
			values[i] = PrecisionFloat64(float64(v))
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
				values[i] = PrecisionFloat64(v.Float64)
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
				values[i] = PrecisionFloat64(float64(v.V))
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
		case *sql.Null[net.IP]:
			if v.Valid {
				values[i] = v.V.String()
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
