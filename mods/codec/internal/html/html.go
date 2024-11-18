package html

import (
	"encoding/base64"
	"fmt"

	"github.com/machbase/neo-server/v8/mods/codec/internal"
	"github.com/machbase/neo-server/v8/mods/stream/spec"
)

type Exporter struct {
	internal.RowsEncoderBase
	imageType string
	output    spec.OutputStream
}

func NewEncoder() *Exporter {
	return &Exporter{
		imageType: "png",
	}
}

func (ex *Exporter) ContentType() string {
	return "application/xhtml+xml" // "text/html"
}

func (ex *Exporter) Open() error {
	return nil
}

func (ex *Exporter) Close() {
}

func (ex *Exporter) AddRow(values []any) error {
	if len(values) == 2 {
		var head string
		switch values[0] {
		case "image/png":
			head = "data:image/png;base64,"
		case "image/jpeg":
			head = "data:image/jpeg;base64,"
		}
		if head != "" {
			switch v := values[1].(type) {
			case []byte:
				enc := base64.NewEncoder(base64.StdEncoding, ex.output)
				ex.output.Write([]byte(fmt.Sprintf(`<div><img src="%s`, head)))
				enc.Write(v)
				ex.output.Write([]byte(`"/></div>`))
			default:
				return fmt.Errorf("invalid image data type (%T)", v)
			}
		}
	}
	return nil
}

func (ex *Exporter) Flush(heading bool) {
}

func (ex *Exporter) SetOutputStream(o spec.OutputStream) {
	ex.output = o
}
