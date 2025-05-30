package codec

import (
	"time"

	"github.com/machbase/neo-server/v8/api"
	"github.com/machbase/neo-server/v8/mods/codec/internal/box"
	"github.com/machbase/neo-server/v8/mods/codec/internal/chart"
	"github.com/machbase/neo-server/v8/mods/codec/internal/csv"
	"github.com/machbase/neo-server/v8/mods/codec/internal/geomap"
	"github.com/machbase/neo-server/v8/mods/codec/internal/html"
	"github.com/machbase/neo-server/v8/mods/codec/internal/json"
	"github.com/machbase/neo-server/v8/mods/codec/internal/markdown"
	"github.com/machbase/neo-server/v8/mods/codec/internal/ndjson"
	"github.com/machbase/neo-server/v8/mods/codec/internal/templ"
	"github.com/machbase/neo-server/v8/mods/codec/opts"
)

const DISCARD = "discard"
const BOX = "box"
const CSV = "csv"
const JSON = "json"
const NDJSON = "ndjson"
const MARKDOWN = "markdown"
const HTML = "html"
const TEXT = "text"
const ECHART = "echart"
const ECHART_LINE = "echart.line"
const ECHART_SCATTER = "echart.scatter"
const ECHART_BAR = "echart.bar"
const ECHART_LINE3D = "echart.line3d"
const ECHART_SURFACE3D = "echart.surface3d"
const ECHART_SCATTER3D = "echart.scatter3d"
const ECHART_BAR3D = "echart.bar3d"
const GEOMAP = "geomap"

type RowsEncoder interface {
	Open() error
	Close()
	AddRow(values []any) error
	Flush(heading bool)
	ContentType() string
	HttpHeaders() map[string][]string
}

var (
	_ RowsEncoder = (*chart.Chart)(nil)
	_ RowsEncoder = (*box.Exporter)(nil)
	_ RowsEncoder = (*csv.Exporter)(nil)
	_ RowsEncoder = (*markdown.Exporter)(nil)
	_ RowsEncoder = (*html.Exporter)(nil)
	_ RowsEncoder = (*geomap.GeoMap)(nil)
	_ RowsEncoder = (*ndjson.Exporter)(nil)
)

type RowsDecoder interface {
	Open()
	// NextRows returns the current row's values and column names.
	NextRow() ([]any, []string, error)
}

var (
	_ RowsDecoder = (*json.Decoder)(nil)
	_ RowsDecoder = (*csv.Decoder)(nil)
	_ RowsDecoder = (*ndjson.Decoder)(nil)
)

func NewEncoder(encoderType string, opts ...opts.Option) RowsEncoder {
	var ret RowsEncoder
	switch encoderType {
	case BOX:
		ret = box.NewEncoder()
	case CSV:
		ret = csv.NewEncoder()
	case MARKDOWN:
		ret = markdown.NewEncoder()
	case HTML:
		ret = templ.NewEncoder(templ.HTML)
	case TEXT:
		ret = templ.NewEncoder(templ.TEXT)
	case ECHART:
		ret = chart.NewChart()
	case ECHART_LINE:
		ret = chart.NewRectChart("line")
	case ECHART_SCATTER:
		ret = chart.NewRectChart("scatter")
	case ECHART_BAR:
		ret = chart.NewRectChart("bar")
	case ECHART_LINE3D:
		ret = chart.NewRectChart("line3D")
	case ECHART_SURFACE3D:
		ret = chart.NewRectChart("surface3D")
	case ECHART_SCATTER3D:
		ret = chart.NewRectChart("scatter3D")
	case ECHART_BAR3D:
		ret = chart.NewRectChart("bar3D")
	case DISCARD:
		ret = &DiscardSink{}
	case GEOMAP:
		ret = geomap.New()
	case NDJSON:
		ret = ndjson.NewEncoder()
	default: // "json"
		ret = json.NewEncoder()
	}
	for _, op := range opts {
		op(ret)
	}
	return ret
}

func NewDecoder(decoderType string, opts ...opts.Option) RowsDecoder {
	var ret RowsDecoder
	switch decoderType {
	case CSV:
		ret = csv.NewDecoder()
	case NDJSON:
		ret = ndjson.NewDecoder()
	default: // "json"
		ret = json.NewDecoder()
	}
	for _, op := range opts {
		op(ret)
	}
	ret.Open()
	return ret
}

func SetEncoderColumns(encoder RowsEncoder, cols api.Columns) {
	SetEncoderColumnsTimeLocation(encoder, cols, nil)
}

func SetEncoderColumnsTimeLocation(encoder RowsEncoder, cols api.Columns, tz *time.Location) {
	var colNames []string
	if tz != nil {
		colNames = cols.NamesWithTimeLocation(tz)
	} else {
		for _, c := range cols {
			colNames = append(colNames, c.Name)
		}
	}
	var colTypes []api.DataType
	for _, c := range cols {
		colTypes = append(colTypes, c.DataType)
	}
	if enc, ok := encoder.(opts.CanSetColumns); ok {
		enc.SetColumns(colNames...)
	}
	if enc, ok := encoder.(opts.CanSetColumnTypes); ok {
		enc.SetColumnTypes(colTypes...)
	}
}

type DiscardSink struct {
}

func (ds *DiscardSink) Open() error {
	return nil
}

func (ds *DiscardSink) Close() {
}

func (ds *DiscardSink) AddRow([]any) error {
	return nil
}

func (ds *DiscardSink) Flush(heading bool) {
}

func (ds *DiscardSink) ContentType() string {
	return "text/plain"
}

func (ds *DiscardSink) HttpHeaders() map[string][]string {
	return nil
}
