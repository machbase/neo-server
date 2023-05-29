package codec

import (
	"time"

	"github.com/machbase/neo-server/mods/codec/internal/box"
	"github.com/machbase/neo-server/mods/codec/internal/csv"
	"github.com/machbase/neo-server/mods/codec/internal/echart"
	"github.com/machbase/neo-server/mods/codec/internal/json"
	spi "github.com/machbase/neo-spi"
)

const BOX = "box"
const CSV = "csv"
const JSON = "json"
const ECHART_LINE = "echart.line"
const ECHART_LINE3D = "echart.line3d"
const ECHART_SURFACE3D = "echart.surface3d"
const ECHART_SCATTER3D = "echart.scatter3d"
const ECHART_BAR3D = "echart.bar3d"

type RowsEncoder interface {
	Open() error
	Close()
	AddRow(values []any) error
	Flush(heading bool)
	ContentType() string
}

type RowsDecoder interface {
	Open()
	NextRow() ([]any, error)
}

type Option func(enc any)

func NewEncoder(encoderType string, opts ...Option) RowsEncoder {
	var ret RowsEncoder
	switch encoderType {
	case BOX:
		ret = box.NewEncoder()
	case CSV:
		ret = csv.NewEncoder()
	case ECHART_LINE:
		ret = &echart.Line{}
	case ECHART_LINE3D:
		ret = &echart.Line3D{}
	case ECHART_SURFACE3D:
		ret = &echart.Surface3D{}
	case ECHART_SCATTER3D:
		ret = &echart.Scatter3D{}
	case ECHART_BAR3D:
		ret = &echart.Bar3D{}
	default: // "json"
		ret = json.NewEncoder()
	}
	for _, op := range opts {
		op(ret)
	}
	return ret
}

func NewDecoder(decoderType string, opts ...Option) RowsDecoder {
	var ret RowsDecoder
	switch decoderType {
	case CSV:
		ret = csv.NewDecoder()
	default: // "json"
		ret = json.NewDecoder()
	}
	for _, op := range opts {
		op(ret)
	}
	ret.Open()
	return ret
}

func SetEncoderColumns(encoder RowsEncoder, cols spi.Columns) {
	SetEncoderColumnsTimeLocation(encoder, cols, nil)
}

func SetEncoderColumnsTimeLocation(encoder RowsEncoder, cols spi.Columns, tz *time.Location) {
	var colNames []string
	if tz != nil {
		colNames = cols.NamesWithTimeLocation(tz)
	} else {
		for _, c := range cols {
			colNames = append(colNames, c.Name)
		}
	}
	var colTypes []string
	for _, c := range cols {
		colTypes = append(colTypes, c.Type)
	}
	if enc, ok := encoder.(CanSetColumns); ok {
		enc.SetColumns(colNames, colTypes)
	}
}

func SetDecoderColumns(decoder RowsDecoder, cols spi.Columns) {
	SetDecoderColumnsTimeLocation(decoder, cols, nil)
}

func SetDecoderColumnsTimeLocation(decoder RowsDecoder, cols spi.Columns, tz *time.Location) {
	var colNames []string
	if tz != nil {
		colNames = cols.NamesWithTimeLocation(tz)
	} else {
		for _, c := range cols {
			colNames = append(colNames, c.Name)
		}
	}
	var colTypes []string
	for _, c := range cols {
		colTypes = append(colTypes, c.Type)
	}

	Columns(colNames, colTypes)(decoder)
}
