package json

import (
	"bytes"
	"database/sql"
	gojson "encoding/json"
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

	output        io.Writer
	Rownum        bool
	Heading       bool
	precision     int
	timeformatter *util.TimeFormatter

	colNames []string
	colTypes []api.DataType

	transpose   bool
	rowsFlatten bool
	rowsArray   bool
	series      [][]any
	values      []any
	jsonEncoder *gojson.Encoder
	buffer      *bytes.Buffer
}

func NewEncoder() *Exporter {
	return &Exporter{
		tick:          time.Now(),
		timeformatter: util.NewTimeFormatter(),
	}
}

func (ex *Exporter) ContentType() string {
	return "application/json"
}

func (ex *Exporter) SetOutputStream(o io.Writer) {
	ex.output = o
}

func (ex *Exporter) SetTimeformat(format string) {
	if format == "" {
		return
	}
	ex.timeformatter.Set(util.Timeformat(format))
}

func (ex *Exporter) SetTimeLocation(tz *time.Location) {
	if tz == nil {
		return
	}
	ex.timeformatter.Set(util.TimeLocation(tz))
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

func (ex *Exporter) SetColumnTypes(columnTypes ...api.DataType) {
	ex.colTypes = columnTypes
}

func (ex *Exporter) SetTranspose(flag bool) {
	ex.transpose = flag
}

func (ex *Exporter) SetRowsFlatten(flag bool) {
	ex.rowsFlatten = flag
}

func (ex *Exporter) SetRowsArray(flag bool) {
	ex.rowsArray = flag
}

func (ex *Exporter) Open() error {
	var columnNames []string
	var columnTypes []api.DataType
	if ex.Rownum && !ex.transpose { // rownum does not effective in transpose mode
		columnNames = append([]string{"ROWNUM"}, ex.colNames...)
		columnTypes = append([]api.DataType{api.DataTypeInt64}, ex.colTypes...)
	} else {
		columnNames = ex.colNames
		columnTypes = ex.colTypes
	}

	columnsJson, _ := gojson.Marshal(columnNames)
	typesJson, _ := gojson.Marshal(columnTypes)

	if ex.transpose && !ex.rowsArray {
		header := fmt.Sprintf(`{"data":{"columns":%s,"types":%s,"cols":[`,
			string(columnsJson), string(typesJson))
		ex.output.Write([]byte(header))
	} else {
		header := fmt.Sprintf(`{"data":{"columns":%s,"types":%s,"rows":[`,
			string(columnsJson), string(typesJson))
		ex.output.Write([]byte(header))
	}

	return nil
}

func (ex *Exporter) Close() {
	if ex.transpose && !ex.rowsArray {
		for n, ser := range ex.series {
			recJson, err := gojson.Marshal(ser)
			if err != nil {
				// TODO how to report error?
				break
			}
			if n > 0 {
				ex.output.Write([]byte(","))
			}
			ex.output.Write(recJson)
		}
	}
	footer := fmt.Sprintf(`]},"success":true,"reason":"success","elapse":"%s"}`, time.Since(ex.tick).String())
	ex.output.Write([]byte(footer))
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

	if cap(ex.values) < len(source) {
		ex.values = make([]any, len(source))
	} else {
		ex.values = ex.values[:len(source)]
	}
	for i, field := range source {
		switch v := field.(type) {
		case *time.Time:
			ex.values[i] = ex.timeformatter.FormatEpoch(*v)
		case time.Time:
			ex.values[i] = ex.timeformatter.FormatEpoch(v)
		case *float64:
			ex.values[i] = ex.encodeFloat64(*v)
		case float64:
			ex.values[i] = ex.encodeFloat64(v)
		case *float32:
			ex.values[i] = ex.encodeFloat64(float64(*v))
		case float32:
			ex.values[i] = ex.encodeFloat64(float64(v))
		case *net.IP:
			ex.values[i] = v.String()
		case net.IP:
			ex.values[i] = v.String()
		case *sql.NullBool:
			if v.Valid {
				ex.values[i] = v.Bool
			}
		case *sql.NullByte:
			if v.Valid {
				ex.values[i] = v.Byte
			}
		case *sql.NullFloat64:
			if v.Valid {
				ex.values[i] = v.Float64
			}
		case *sql.NullInt16:
			if v.Valid {
				ex.values[i] = v.Int16
			}
		case *sql.NullInt32:
			if v.Valid {
				ex.values[i] = v.Int32
			}
		case *sql.Null[float32]:
			if v.Valid {
				ex.values[i] = v.V
			}
		case *sql.NullInt64:
			if v.Valid {
				ex.values[i] = v.Int64
			}
		case *sql.NullString:
			if v.Valid {
				ex.values[i] = v.String
			}
		case *sql.NullTime:
			if v.Valid {
				ex.values[i] = ex.timeformatter.Format(v.Time)
			}
		case *sql.Null[net.IP]:
			if v.Valid {
				ex.values[i] = v.V.String()
			}
		default:
			ex.values[i] = field
		}
	}

	if ex.rowsArray {
		var vs = map[string]any{}
		if ex.Rownum {
			vs["ROWNUM"] = ex.nrow
		}
		for i, v := range ex.values {
			if i >= len(ex.colNames) {
				break
			}
			vs[ex.colNames[i]] = v
		}
		recJson, err := gojson.Marshal(vs)
		if err != nil {
			return err
		}
		if ex.nrow > 1 {
			ex.output.Write([]byte(","))
		}
		ex.output.Write(recJson)
	} else if ex.transpose {
		if ex.series == nil {
			ex.series = make([][]any, len(ex.values)-1)
		}
		if len(ex.series) < len(ex.values) {
			for i := 0; i < len(ex.values)-len(ex.series); i++ {
				ex.series = append(ex.series, []any{})
			}
		}
		for n, v := range ex.values {
			ex.series[n] = append(ex.series[n], v)
		}
	} else if ex.rowsFlatten {
		var recJson []byte
		var err error
		vs := ex.values
		if ex.Rownum {
			vs = append([]any{ex.nrow}, ex.values...)
		}
		for i, v := range vs {
			recJson, err = gojson.Marshal(v)
			if err != nil {
				return err
			}
			if ex.nrow > 1 || i > 0 {
				ex.output.Write([]byte(","))
			}
			ex.output.Write(recJson)
		}
	} else {
		if ex.buffer == nil {
			ex.buffer = &bytes.Buffer{}
			ex.jsonEncoder = gojson.NewEncoder(ex.buffer)
		}
		ex.buffer.Reset()

		var err error
		if ex.Rownum {
			vs := append([]any{ex.nrow}, ex.values...)
			err = ex.jsonEncoder.Encode(vs)
		} else {
			err = ex.jsonEncoder.Encode(ex.values)
		}
		if err != nil {
			return err
		}

		if ex.nrow > 1 {
			ex.output.Write([]byte(","))
		}
		str := ex.buffer.Bytes() // trim the last newline '\n'
		ex.output.Write(str[0 : len(str)-1])
	}

	return nil
}
