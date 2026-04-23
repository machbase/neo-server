package ndjson

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net"
	"strconv"
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
	values   []any
	buffer   *bytes.Buffer
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
	r := appendPrecisionFloat64(make([]byte, 0, 24), float64(pf))
	return r, nil
}

func appendPrecisionFloat64(dst []byte, value float64) []byte {
	switch {
	case math.IsNaN(value):
		return append(dst, `"NaN"`...)
	case math.IsInf(value, -1):
		return append(dst, `"-Inf"`...)
	case math.IsInf(value, 1):
		return append(dst, `"+Inf"`...)
	case value == 0:
		// Keep zero formatting stable and avoid "-0".
		return append(dst, '0')
	}
	r := strconv.AppendFloat(dst, value, 'f', 6, 64)
	for len(r) > 0 && r[len(r)-1] == '0' {
		r = r[:len(r)-1]
	}
	if len(r) > 0 && r[len(r)-1] == '.' {
		r = r[:len(r)-1]
	}
	return r
}

func appendJSONValue(dst []byte, value any) ([]byte, error) {
	switch v := value.(type) {
	case nil:
		return append(dst, "null"...), nil
	case string:
		return strconv.AppendQuote(dst, v), nil
	case bool:
		return strconv.AppendBool(dst, v), nil
	case PrecisionFloat64:
		return appendPrecisionFloat64(dst, float64(v)), nil
	case float64:
		return appendPrecisionFloat64(dst, v), nil
	case float32:
		return appendPrecisionFloat64(dst, float64(v)), nil
	case int:
		return strconv.AppendInt(dst, int64(v), 10), nil
	case int8:
		return strconv.AppendInt(dst, int64(v), 10), nil
	case int16:
		return strconv.AppendInt(dst, int64(v), 10), nil
	case int32:
		return strconv.AppendInt(dst, int64(v), 10), nil
	case int64:
		return strconv.AppendInt(dst, v, 10), nil
	case uint:
		return strconv.AppendUint(dst, uint64(v), 10), nil
	case uint8:
		return strconv.AppendUint(dst, uint64(v), 10), nil
	case uint16:
		return strconv.AppendUint(dst, uint64(v), 10), nil
	case uint32:
		return strconv.AppendUint(dst, uint64(v), 10), nil
	case uint64:
		return strconv.AppendUint(dst, v, 10), nil
	case json.Number:
		return append(dst, string(v)...), nil
	default:
		encoded, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}
		return append(dst, encoded...), nil
	}
}

func (ex *Exporter) AddRow(source []any) error {
	ex.nrow++

	if cap(ex.values) < len(source) {
		ex.values = make([]any, len(source))
	} else {
		ex.values = ex.values[:len(source)]
	}
	for i, field := range source {
		switch v := field.(type) {
		case *time.Time:
			ex.values[i] = ex.timeformat.FormatEpoch(*v)
		case time.Time:
			ex.values[i] = ex.timeformat.FormatEpoch(v)
		case *float64:
			ex.values[i] = PrecisionFloat64(*v)
		case float64:
			ex.values[i] = PrecisionFloat64(v)
		case *float32:
			ex.values[i] = PrecisionFloat64(float64(*v))
		case float32:
			ex.values[i] = PrecisionFloat64(float64(v))
		case *net.IP:
			ex.values[i] = v.String()
		case net.IP:
			ex.values[i] = v.String()
		case *sql.NullBool:
			if v.Valid {
				ex.values[i] = v.Bool
			} else {
				ex.values[i] = nil
			}
		case *sql.NullByte:
			if v.Valid {
				ex.values[i] = v.Byte
			} else {
				ex.values[i] = nil
			}
		case *sql.NullFloat64:
			if v.Valid {
				ex.values[i] = PrecisionFloat64(v.Float64)
			} else {
				ex.values[i] = nil
			}
		case *sql.NullInt16:
			if v.Valid {
				ex.values[i] = v.Int16
			} else {
				ex.values[i] = nil
			}
		case *sql.NullInt32:
			if v.Valid {
				ex.values[i] = v.Int32
			} else {
				ex.values[i] = nil
			}
		case *sql.Null[float32]:
			if v.Valid {
				ex.values[i] = PrecisionFloat64(float64(v.V))
			} else {
				ex.values[i] = nil
			}
		case *sql.NullInt64:
			if v.Valid {
				ex.values[i] = v.Int64
			} else {
				ex.values[i] = nil
			}
		case *sql.NullString:
			if v.Valid {
				ex.values[i] = v.String
			} else {
				ex.values[i] = nil
			}
		case *sql.NullTime:
			if v.Valid {
				ex.values[i] = ex.timeformat.Format(v.Time)
			} else {
				ex.values[i] = nil
			}
		case *sql.Null[net.IP]:
			if v.Valid {
				ex.values[i] = v.V.String()
			} else {
				ex.values[i] = nil
			}
		default:
			ex.values[i] = field
		}
	}

	if len(ex.values) != len(ex.colNames) {
		return fmt.Errorf("rows[%d] number of columns not matched (%d); table '%s' has %d columns",
			ex.nrow, len(ex.values), ex.colNames, len(ex.colNames))
	}
	if ex.buffer == nil {
		ex.buffer = &bytes.Buffer{}
	}
	ex.buffer.Reset()
	ex.buffer.WriteByte('{')
	fieldIndex := 0
	if ex.Rownum {
		ex.buffer.WriteString(`"ROWNUM":`)
		encoded, err := appendJSONValue(ex.buffer.Bytes(), ex.nrow)
		if err != nil {
			return err
		}
		ex.buffer.Reset()
		ex.buffer.Write(encoded)
		fieldIndex = 1
	}
	for i, v := range ex.values {
		if i >= len(ex.colNames) {
			break
		}
		if fieldIndex > 0 {
			ex.buffer.WriteByte(',')
		}
		ex.buffer.WriteString(strconv.Quote(ex.colNames[i]))
		ex.buffer.WriteByte(':')
		encoded, err := appendJSONValue(ex.buffer.Bytes(), v)
		if err != nil {
			return err
		}
		ex.buffer.Reset()
		ex.buffer.Write(encoded)
		fieldIndex++
	}
	ex.buffer.WriteString("}\n")
	ex.output.Write(ex.buffer.Bytes())

	return nil
}
