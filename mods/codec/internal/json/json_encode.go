package json

import (
	gojson "encoding/json"
	"fmt"
	"time"

	spi "github.com/machbase/neo-spi"
)

type Exporter struct {
	tick time.Time
	nrow int
	ctx  *spi.RowsEncoderContext
}

func NewEncoder(ctx *spi.RowsEncoderContext) spi.RowsEncoder {
	return &Exporter{ctx: ctx, tick: time.Now()}
}

func (ex *Exporter) ContentType() string {
	return "application/json"
}

func (ex *Exporter) Open(cols spi.Columns) error {
	names := cols.Names()
	types := cols.Types()
	if ex.ctx.Rownum {
		names = append([]string{"ROWNUM"}, names...)
		types = append([]string{"string"}, types...)
	}

	columnsJson, _ := gojson.Marshal(names)
	typesJson, _ := gojson.Marshal(types)

	header := fmt.Sprintf(`{"data":{"columns":%s,"types":%s,"rows":[`,
		string(columnsJson), string(typesJson))
	ex.ctx.Output.Write([]byte(header))

	return nil
}

func (ex *Exporter) Close() {
	footer := fmt.Sprintf(`]}, "success":true, "reason":"success", "elapse":"%s"}`, time.Since(ex.tick).String())
	ex.ctx.Output.Write([]byte(footer))
	ex.ctx.Output.Close()
}

func (ex *Exporter) Flush(heading bool) {
	ex.ctx.Output.Flush()
}

func (ex *Exporter) AddRow(source []any) error {
	ex.nrow++

	if ex.ctx.TimeLocation == nil {
		ex.ctx.TimeLocation = time.UTC
	}

	values := make([]any, len(source))
	for i, field := range source {
		values[i] = field
		if v, ok := field.(*time.Time); ok {
			switch ex.ctx.TimeFormat {
			case "ns":
				values[i] = v.UnixNano()
			case "ms":
				values[i] = v.UnixMilli()
			case "us":
				values[i] = v.UnixMicro()
			case "s":
				values[i] = v.Unix()
			default:
				if ex.ctx.TimeLocation == nil {
					ex.ctx.TimeLocation = time.UTC
				}
				values[i] = v.In(ex.ctx.TimeLocation).Format(ex.ctx.TimeFormat)
			}
			continue
		}
	}
	var recJson []byte
	if ex.ctx.Rownum {
		vs := append([]any{ex.nrow}, values...)
		recJson, _ = gojson.Marshal(vs)
	} else {
		recJson, _ = gojson.Marshal(values)
	}

	if ex.nrow > 1 {
		ex.ctx.Output.Write([]byte(","))
	}
	ex.ctx.Output.Write(recJson)

	return nil
}
