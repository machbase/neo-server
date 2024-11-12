package csv

import (
	"database/sql"
	"encoding/csv"
	"fmt"
	"net"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/machbase/neo-server/mods/codec/internal"
	"github.com/machbase/neo-server/mods/expression"
	"github.com/machbase/neo-server/mods/nums"
	"github.com/machbase/neo-server/mods/stream/spec"
	"github.com/machbase/neo-server/mods/util"
)

type Exporter struct {
	internal.RowsEncoderBase
	rownum int64

	writer *csv.Writer
	comma  rune

	output          spec.OutputStream
	showRownum      bool
	precision       int
	nullAlternative any
	timeformat      *util.TimeFormatter

	heading  bool
	colNames []string

	closeOnce sync.Once
}

func NewEncoder() *Exporter {
	rr := &Exporter{
		precision:       -1,
		nullAlternative: "NULL",
		timeformat:      util.NewTimeFormatter(),
	}
	return rr
}

func (ex *Exporter) ContentType() string {
	return "text/csv; charset=utf-8"
}

func (ex *Exporter) SetOutputStream(o spec.OutputStream) {
	ex.output = o
}

func (ex *Exporter) SetTimeformat(format string) {
	ex.timeformat.Set(util.Timeformat(format))
}

func (ex *Exporter) SetTimeLocation(tz *time.Location) {
	ex.timeformat.Set(util.TimeLocation(tz))
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
	delmiter, _ := utf8.DecodeRuneInString(delimiter)
	ex.comma = delmiter
}

func (ex *Exporter) SetColumns(labels ...string) {
	ex.colNames = labels
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
		// TODO check if write() returns error, when csvWritter.Comma is not valid
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
		ex.output.Close()
	})
}

func (ex *Exporter) Flush(heading bool) {
	ex.writer.Flush()
	ex.output.Flush()
}

func (ex *Exporter) AddRow(values []any) error {
	defer func() {
		o := recover()
		if o != nil {
			fmt.Println("PANIC (csvexporter)", o)
			debug.PrintStack()
		}
	}()

	var cols = make([]string, len(values))

	for i, r := range values {
		if r == nil || r == expression.NullValue {
			r = ex.nullAlternative
		}
		switch sqlVal := r.(type) {
		case *sql.NullBool:
			if sqlVal.Valid {
				r = sqlVal.Bool
			} else {
				r = ex.nullAlternative
			}
		case *sql.NullByte:
			if sqlVal.Valid {
				r = sqlVal.Byte
			} else {
				r = ex.nullAlternative
			}
		case *sql.NullFloat64:
			if sqlVal.Valid {
				r = sqlVal.Float64
			} else {
				r = ex.nullAlternative
			}
		case *sql.NullInt16:
			if sqlVal.Valid {
				r = sqlVal.Int16
			} else {
				r = ex.nullAlternative
			}
		case *sql.NullInt32:
			if sqlVal.Valid {
				r = sqlVal.Int32
			} else {
				r = ex.nullAlternative
			}
		case *sql.NullInt64:
			if sqlVal.Valid {
				r = sqlVal.Int64
			} else {
				r = ex.nullAlternative
			}
		case *sql.NullString:
			if sqlVal.Valid {
				r = sqlVal.String
			} else {
				r = ex.nullAlternative
			}
		case *sql.NullTime:
			if sqlVal.Valid {
				r = ex.timeformat.Format(sqlVal.Time)
			} else {
				r = ex.nullAlternative
			}
		case *sql.Null[float32]:
			if sqlVal.Valid {
				r = sqlVal.V
			} else {
				r = ex.nullAlternative
			}
		case *sql.Null[net.IP]:
			if sqlVal.Valid {
				r = sqlVal.V.String()
			} else {
				r = ex.nullAlternative
			}
		}
		switch v := r.(type) {
		case *string:
			cols[i] = *v
		case string:
			cols[i] = v
		case *time.Time:
			cols[i] = ex.timeformat.Format(*v)
		case time.Time:
			cols[i] = ex.timeformat.Format(v)
		case *float64:
			cols[i] = strconv.FormatFloat(*v, 'f', ex.precision, 64)
		case float64:
			cols[i] = strconv.FormatFloat(v, 'f', ex.precision, 64)
		case *float32:
			cols[i] = strconv.FormatFloat(float64(*v), 'f', ex.precision, 32)
		case float32:
			cols[i] = strconv.FormatFloat(float64(v), 'f', ex.precision, 32)
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
		case *bool:
			cols[i] = strconv.FormatBool(*v)
		case bool:
			cols[i] = strconv.FormatBool(v)
		case *net.IP:
			cols[i] = v.String()
		case net.IP:
			cols[i] = v.String()
		case uint8:
			cols[i] = strconv.FormatInt(int64(v), 10)
		case *[]uint8:
			strs := []string{}
			for _, c := range *v {
				strs = append(strs, fmt.Sprintf("\\x%02X", c))
			}
			cols[i] = strings.Join(strs, "")
		case []uint8:
			strs := []string{}
			for _, c := range v {
				strs = append(strs, fmt.Sprintf("\\x%02X", c))
			}
			cols[i] = strings.Join(strs, "")
		case *nums.LatLon:
			cols[i] = fmt.Sprintf("[%v,%v]", v.Lat, v.Lon)
		case *nums.SingleLatLon:
			if coord := v.Coordinates(); len(coord) == 1 && len(coord[0]) == 2 {
				cols[i] = fmt.Sprintf("[%v,%v]", coord[0][0], coord[0][1])
			} else {
				cols[i] = ""
			}
		default:
			cols[i] = fmt.Sprintf("%T", r)
		}
	}

	ex.rownum++

	if ex.showRownum {
		return ex.writer.Write(append([]string{strconv.FormatInt(ex.rownum, 10)}, cols...))
	} else {
		return ex.writer.Write(cols)
	}
}
