package csv

import (
	"encoding/csv"
	"fmt"
	"io"
	"net"
	"runtime/debug"
	"strconv"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/machbase/neo-client/api"
	"github.com/machbase/neo-server/v8/mods/codec/internal"
	"github.com/machbase/neo-server/v8/mods/nums"
	"github.com/machbase/neo-server/v8/mods/util"
)

type Exporter struct {
	internal.RowsEncoderBase
	rownum int64

	writer *csv.Writer
	comma  rune

	output          io.Writer
	showRownum      bool
	precision       int
	nullAlternative any
	timeformat      *util.TimeFormatter
	binaryFormatter *util.BinaryFormatter

	heading  bool
	colNames []string
	colTypes []api.DataType

	closeOnce sync.Once
}

func NewEncoder() *Exporter {
	rr := &Exporter{
		precision:       -1,
		nullAlternative: "NULL",
		timeformat:      util.NewTimeFormatter(),
		binaryFormatter: util.NewBinaryFormatter("hex"),
	}
	return rr
}

func (ex *Exporter) ContentType() string {
	return "text/csv; charset=utf-8"
}

func (ex *Exporter) SetOutputStream(o io.Writer) {
	ex.output = o
}

func (ex *Exporter) SetTimeformat(format string) {
	ex.timeformat.Set(util.Timeformat(format))
}

func (ex *Exporter) SetTimeLocation(tz *time.Location) {
	ex.timeformat.Set(util.TimeLocation(tz))
}

func (ex *Exporter) SetBinaryformat(format string) {
	ex.binaryFormatter.SetFormat(format)
}

func (ex *Exporter) SetPrecision(precision int) {
	ex.precision = precision
}

func (ex *Exporter) SetRownum(show bool) {
	ex.showRownum = show
}

// Deprecated use SetHeader()
func (ex *Exporter) SetHeading(show bool) {
	ex.heading = show
}

func (ex *Exporter) SetHeader(show bool) {
	ex.heading = show
}

func (ex *Exporter) SetDelimiter(delimiter string) {
	comma, _ := utf8.DecodeRuneInString(delimiter)
	ex.comma = comma
}

func (ex *Exporter) SetColumns(labels ...string) {
	ex.colNames = labels
}

func (ex *Exporter) SetColumnTypes(types ...api.DataType) {
	ex.colTypes = types
}

func (ex *Exporter) SetSubstituteNull(alternative any) {
	ex.nullAlternative = alternative
}

func (ex *Exporter) Open() error {
	ex.writer = csv.NewWriter(ex.output)

	if ex.comma != 0 {
		ex.writer.Comma = ex.comma
	}

	if ex.heading {
		// TODO check if write() returns error, when csvWriter.Comma is not valid
		if ex.showRownum {
			ex.writer.Write(append([]string{"ROWNUM"}, ex.colNames...))
		} else {
			ex.writer.Write(ex.colNames)
		}
	}

	return nil
}

func (ex *Exporter) Close() {
	ex.closeOnce.Do(func() {
		ex.writer.Flush()
		ex.output.Write([]byte("\n"))
		if closer, ok := ex.output.(io.Closer); ok {
			closer.Close()
		}
	})
}

func (ex *Exporter) Flush(heading bool) {
	ex.writer.Flush()
	if flusher, ok := ex.output.(api.Flusher); ok {
		flusher.Flush()
	}
}

func (ex *Exporter) AddRow(values []any) error {
	defer func() {
		o := recover()
		if o != nil {
			fmt.Println("PANIC (csv)", o)
			debug.PrintStack()
		}
	}()

	var cols = make([]string, len(values))

	for i, value := range values {
		treatIntValueAsFloat := false
		if ex.precision > 0 && i < len(ex.colTypes) && (ex.colTypes[i] == api.DataTypeFloat64 || ex.colTypes[i] == api.DataTypeFloat32) {
			treatIntValueAsFloat = true
		}
		val := api.Unbox(value)
		if val == nil {
			val = ex.nullAlternative
		}
		switch val := val.(type) {
		case bool:
			cols[i] = strconv.FormatBool(val)
		case net.IP:
			cols[i] = val.String()
		case string:
			cols[i] = val
		case time.Time:
			cols[i] = ex.timeformat.Format(val)
		case time.Duration:
			cols[i] = strconv.FormatInt(val.Nanoseconds(), 10)
		case float64:
			cols[i] = internal.FormatPrecisionFloat64(val, ex.precision, false)
		case float32:
			cols[i] = internal.FormatPrecisionFloat64(float64(val), ex.precision, false)
		case int:
			if treatIntValueAsFloat {
				cols[i] = internal.FormatPrecisionFloat64(float64(val), ex.precision, false)
			} else {
				cols[i] = strconv.FormatInt(int64(val), 10)
			}
		case uint:
			if treatIntValueAsFloat {
				cols[i] = internal.FormatPrecisionFloat64(float64(val), ex.precision, false)
			} else {
				cols[i] = strconv.FormatUint(uint64(val), 10)
			}
		case int8:
			if treatIntValueAsFloat {
				cols[i] = internal.FormatPrecisionFloat64(float64(val), ex.precision, false)
			} else {
				cols[i] = strconv.FormatInt(int64(val), 10)
			}
		case uint8:
			cols[i] = strconv.FormatInt(int64(val), 10)
		case int16:
			if treatIntValueAsFloat {
				cols[i] = internal.FormatPrecisionFloat64(float64(val), ex.precision, false)
			} else {
				cols[i] = strconv.FormatInt(int64(val), 10)
			}
		case uint16:
			if treatIntValueAsFloat {
				cols[i] = internal.FormatPrecisionFloat64(float64(val), ex.precision, false)
			} else {
				cols[i] = strconv.FormatUint(uint64(val), 10)
			}
		case int32:
			if treatIntValueAsFloat {
				cols[i] = internal.FormatPrecisionFloat64(float64(val), ex.precision, false)
			} else {
				cols[i] = strconv.FormatInt(int64(val), 10)
			}
		case uint32:
			if treatIntValueAsFloat {
				cols[i] = internal.FormatPrecisionFloat64(float64(val), ex.precision, false)
			} else {
				cols[i] = strconv.FormatUint(uint64(val), 10)
			}
		case int64:
			if treatIntValueAsFloat {
				cols[i] = internal.FormatPrecisionFloat64(float64(val), ex.precision, false)
			} else {
				cols[i] = strconv.FormatInt(val, 10)
			}
		case uint64:
			if treatIntValueAsFloat {
				cols[i] = internal.FormatPrecisionFloat64(float64(val), ex.precision, false)
			} else {
				cols[i] = strconv.FormatUint(val, 10)
			}
		case []byte:
			cols[i] = ex.binaryFormatter.Format(val)
		case *nums.LatLon:
			cols[i] = fmt.Sprintf("[%v,%v]", val.Lat, val.Lon)
		case *nums.SingleLatLon:
			if coord := val.Coordinates(); len(coord) == 1 && len(coord[0]) == 2 {
				cols[i] = fmt.Sprintf("[%v,%v]", coord[0][0], coord[0][1])
			} else {
				cols[i] = ""
			}
		case api.JSONString:
			cols[i] = string(val)
		default:
			cols[i] = fmt.Sprintf("%T", val)

		}
	}

	ex.rownum++

	if ex.showRownum {
		return ex.writer.Write(append([]string{strconv.FormatInt(ex.rownum, 10)}, cols...))
	} else {
		return ex.writer.Write(cols)
	}
}
