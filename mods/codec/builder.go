package codec

import (
	"github.com/machbase/neo-server/mods/codec/internal/box"
	"github.com/machbase/neo-server/mods/codec/internal/csv"
	"github.com/machbase/neo-server/mods/codec/internal/echart"
	"github.com/machbase/neo-server/mods/codec/internal/json"
	spi "github.com/machbase/neo-spi"
)

const BOX = "box"
const CSV = "csv"
const JSON = "json"
const ECHART = "echart"

type RowsEncoder interface {
	Open(columns spi.Columns) error
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
	case ECHART:
		ret = echart.NewEncoder()
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
