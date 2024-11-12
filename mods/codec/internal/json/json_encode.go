package json

import (
	"database/sql"
	gojson "encoding/json"
	"fmt"
	"math"
	"net"
	"time"

	"github.com/machbase/neo-server/api"
	"github.com/machbase/neo-server/mods/codec/internal"
	"github.com/machbase/neo-server/mods/stream/spec"
	"github.com/machbase/neo-server/mods/util"
)

type Exporter struct {
	internal.RowsEncoderBase
	tick time.Time
	nrow int

	output        spec.OutputStream
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

func (ex *Exporter) SetOutputStream(o spec.OutputStream) {
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
	ex.output.Close()
}

func (ex *Exporter) Flush(heading bool) {
	ex.output.Flush()
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
			values[i] = ex.timeformatter.FormatEpoch(*v)
		case time.Time:
			values[i] = ex.timeformatter.FormatEpoch(v)
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
		case *sql.NullTime:
			if v.Valid {
				values[i] = ex.timeformatter.Format(v.Time)
			}
		case *sql.Null[net.IP]:
			if v.Valid {
				values[i] = v.V.String()
			}
		default:
			values[i] = field
		}
	}

	if ex.rowsArray {
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
			ex.series = make([][]any, len(values)-1)
		}
		if len(ex.series) < len(values) {
			for i := 0; i < len(values)-len(ex.series); i++ {
				ex.series = append(ex.series, []any{})
			}
		}
		for n, v := range values {
			ex.series[n] = append(ex.series[n], v)
		}
	} else if ex.rowsFlatten {
		var recJson []byte
		var err error
		vs := values
		if ex.Rownum {
			vs = append([]any{ex.nrow}, values...)
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
		var recJson []byte
		var err error
		if ex.Rownum {
			vs := append([]any{ex.nrow}, values...)
			recJson, err = gojson.Marshal(vs)
		} else {
			recJson, err = gojson.Marshal(values)
		}
		if err != nil {
			return err
		}

		if ex.nrow > 1 {
			ex.output.Write([]byte(","))
		}
		ex.output.Write(recJson)
	}

	return nil
}
