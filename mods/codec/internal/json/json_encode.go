package json

import (
	gojson "encoding/json"
	"fmt"
	"math"
	"time"

	spi "github.com/machbase/neo-spi"
)

type Exporter struct {
	tick time.Time
	nrow int

	TimeLocation *time.Location
	Output       spi.OutputStream
	Rownum       bool
	Heading      bool
	TimeFormat   string
	Precision    int
}

func NewEncoder() *Exporter {
	return &Exporter{tick: time.Now()}
}

func (ex *Exporter) ContentType() string {
	return "application/json"
}

func (ex *Exporter) Open(cols spi.Columns) error {
	names := cols.Names()
	types := cols.Types()
	if ex.Rownum {
		names = append([]string{"ROWNUM"}, names...)
		types = append([]string{"string"}, types...)
	}

	columnsJson, _ := gojson.Marshal(names)
	typesJson, _ := gojson.Marshal(types)

	header := fmt.Sprintf(`{"data":{"columns":%s,"types":%s,"rows":[`,
		string(columnsJson), string(typesJson))
	ex.Output.Write([]byte(header))

	return nil
}

func (ex *Exporter) Close() {
	footer := fmt.Sprintf(`]}, "success":true, "reason":"success", "elapse":"%s"}`, time.Since(ex.tick).String())
	ex.Output.Write([]byte(footer))
	ex.Output.Close()
}

func (ex *Exporter) Flush(heading bool) {
	ex.Output.Flush()
}

func (ex *Exporter) AddRow(source []any) error {
	ex.nrow++

	if ex.TimeLocation == nil {
		ex.TimeLocation = time.UTC
	}

	values := make([]any, len(source))
	for i, field := range source {
		values[i] = field
		if v, ok := field.(*time.Time); ok {
			switch ex.TimeFormat {
			case "ns":
				values[i] = v.UnixNano()
			case "ms":
				values[i] = v.UnixMilli()
			case "us":
				values[i] = v.UnixMicro()
			case "s":
				values[i] = v.Unix()
			default:
				if ex.TimeLocation == nil {
					ex.TimeLocation = time.UTC
				}
				values[i] = v.In(ex.TimeLocation).Format(ex.TimeFormat)
			}
			continue
		} else if v, ok := field.(*float64); ok {
			if math.IsNaN(*v) {
				values[i] = "NaN"
			} else if math.IsInf(*v, -1) {
				values[i] = "-Inf"
			} else if math.IsInf(*v, 1) {
				values[i] = "+Inf"
			}
		}
	}
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
		ex.Output.Write([]byte(","))
	}
	ex.Output.Write(recJson)

	return nil
}
