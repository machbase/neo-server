package json

import (
	gojson "encoding/json"
	"fmt"
	"math"
	"net"
	"time"

	"github.com/machbase/neo-server/mods/stream/spec"
)

type Exporter struct {
	tick time.Time
	nrow int

	TimeLocation *time.Location
	output       spec.OutputStream
	Rownum       bool
	Heading      bool
	timeformat   string
	precision    int

	colNames []string
	colTypes []string

	transpose bool
	series    [][]any
}

func NewEncoder() *Exporter {
	return &Exporter{tick: time.Now()}
}

func (ex *Exporter) ContentType() string {
	return "application/json"
}

func (ex *Exporter) SetOutputStream(o spec.OutputStream) {
	ex.output = o
}

func (ex *Exporter) SetTimeformat(format string) {
	ex.timeformat = format
}

func (ex *Exporter) SetTimeLocation(tz *time.Location) {
	ex.TimeLocation = tz
}

func (ex *Exporter) SetPrecision(precision int) {
	ex.precision = precision
}

func (ex *Exporter) SetRownum(show bool) {
	ex.Rownum = show
}

func (ex *Exporter) SetHeading(show bool) {
	ex.Heading = show
}

func (ex *Exporter) SetColumns(labels []string, types []string) {
	ex.colNames = labels
	ex.colTypes = types
}

func (ex *Exporter) SetTranspose(flag bool) {
	ex.transpose = flag
}

func (ex *Exporter) Open() error {
	var names []string
	var types []string
	if ex.Rownum {
		names = append([]string{"ROWNUM"}, ex.colNames...)
		types = append([]string{"string"}, ex.colTypes...)
	} else {
		names = ex.colNames
		types = ex.colTypes
	}

	columnsJson, _ := gojson.Marshal(names)
	typesJson, _ := gojson.Marshal(types)

	if ex.transpose {
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
	if ex.transpose {
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
	footer := fmt.Sprintf(`]}, "success":true, "reason":"success", "elapse":"%s"}`, time.Since(ex.tick).String())
	ex.output.Write([]byte(footer))
	ex.output.Close()
}

func (ex *Exporter) Flush(heading bool) {
	ex.output.Flush()
}

func (ex *Exporter) encodeTime(t time.Time) any {
	switch ex.timeformat {
	case "":
		fallthrough
	case "ns":
		return t.UnixNano()
	case "ms":
		return t.UnixMilli()
	case "us":
		return t.UnixMicro()
	case "s":
		return t.Unix()
	default:
		if ex.TimeLocation == nil {
			ex.TimeLocation = time.UTC
		}
		return t.In(ex.TimeLocation).Format(ex.timeformat)
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

	if ex.TimeLocation == nil {
		ex.TimeLocation = time.UTC
	}

	values := make([]any, len(source))
	for i, field := range source {
		switch v := field.(type) {
		default:
			values[i] = field
		case *time.Time:
			values[i] = ex.encodeTime(*v)
		case time.Time:
			values[i] = ex.encodeTime(v)
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

		}
	}

	if ex.transpose {
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
