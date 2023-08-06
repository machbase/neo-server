package html

import (
	"encoding/base64"
	"fmt"

	"github.com/machbase/neo-server/mods/stream/spec"
)

type Exporter struct {
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
				base64Encoding := head + base64.StdEncoding.EncodeToString(v)
				ex.output.Write([]byte(fmt.Sprintf(`<div><img src="%s"/></div>`, base64Encoding)))
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
