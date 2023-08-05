package csv

import (
	"encoding/csv"
	"fmt"
	"net"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/machbase/neo-server/mods/stream/spec"
)

type Exporter struct {
	rownum int64

	writer *csv.Writer
	comma  rune

	timeLocation *time.Location
	output       spec.OutputStream
	showRownum   bool
	timeformat   string
	precision    int

	heading  bool
	colNames []string

	closeOnce sync.Once
}

func NewEncoder() *Exporter {
	rr := &Exporter{
		precision:  -1,
		timeformat: "ns",
	}
	return rr
}

func (ex *Exporter) ContentType() string {
	return "text/csv"
}

func (ex *Exporter) SetOutputStream(o spec.OutputStream) {
	ex.output = o
}

func (ex *Exporter) SetTimeformat(format string) {
	ex.timeformat = format
}

func (ex *Exporter) SetTimeLocation(tz *time.Location) {
	ex.timeLocation = tz
}

func (ex *Exporter) SetPrecision(precision int) {
	ex.precision = precision
}

func (ex *Exporter) SetRownum(show bool) {
	ex.showRownum = show
}

func (ex *Exporter) SetHeading(show bool) {
	ex.heading = show
}

func (ex *Exporter) SetDelimiter(delimiter string) {
	delmiter, _ := utf8.DecodeRuneInString(delimiter)
	ex.comma = delmiter
}

func (ex *Exporter) SetColumns(labels ...string) {
	ex.colNames = labels
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
		ex.output.Close()
	})
}

func (ex *Exporter) Flush(heading bool) {
	ex.writer.Flush()
	ex.output.Flush()
}

func (ex *Exporter) encodeTime(v time.Time) string {
	switch ex.timeformat {
	case "ns":
		return strconv.FormatInt(v.UnixNano(), 10)
	case "ms":
		return strconv.FormatInt(v.UnixMilli(), 10)
	case "us":
		return strconv.FormatInt(v.UnixMicro(), 10)
	case "s":
		return strconv.FormatInt(v.Unix(), 10)
	default:
		if ex.timeLocation == nil {
			ex.timeLocation = time.UTC
		}
		return v.In(ex.timeLocation).Format(ex.timeformat)
	}
}

func (ex *Exporter) encodeFloat64(v float64) string {
	if ex.precision < 0 {
		return fmt.Sprintf("%f", v)
	} else {
		return fmt.Sprintf("%.*f", ex.precision, v)
	}
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
			cols[i] = ex.encodeTime(*v)
		case time.Time:
			cols[i] = ex.encodeTime(v)
		case *float64:
			cols[i] = ex.encodeFloat64(*v)
		case float64:
			cols[i] = ex.encodeFloat64(v)
		case *float32:
			cols[i] = ex.encodeFloat64(float64(*v))
		case float32:
			cols[i] = ex.encodeFloat64(float64(v))
		case *int:
			cols[i] = strconv.FormatInt(int64(*v), 10)
		case int:
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
		case []uint8:
			strs := []string{}
			for _, c := range v {
				strs = append(strs, fmt.Sprintf("\\x%02X", c))
			}
			cols[i] = strings.Join(strs, "")
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
