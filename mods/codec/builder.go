package codec

import (
	"time"

	"github.com/machbase/neo-server/mods/codec/internal/box"
	"github.com/machbase/neo-server/mods/codec/internal/csv"
	"github.com/machbase/neo-server/mods/codec/internal/json"
	"github.com/machbase/neo-server/mods/transcoder"
	"github.com/machbase/neo-server/mods/util"
	spi "github.com/machbase/neo-spi"
)

const BOX = "box"
const CSV = "csv"
const JSON = "json"

type EncoderBuilder interface {
	Build() spi.RowsEncoder
	SetOutputStream(s spi.OutputStream) EncoderBuilder
	SetTimeLocation(tz *time.Location) EncoderBuilder
	SetTimeFormat(f string) EncoderBuilder
	SetPrecision(p int) EncoderBuilder
	SetRownum(flag bool) EncoderBuilder
	SetHeading(flag bool) EncoderBuilder
	// CSV only
	SetCsvDelimieter(del string) EncoderBuilder
	// BOX only
	SetBoxStyle(style string) EncoderBuilder
	SetBoxSeparateColumns(flag bool) EncoderBuilder
	SetBoxDrawBorder(flag bool) EncoderBuilder
}

type encBuilder struct {
	*spi.RowsEncoderContext
	encoderType        string
	csvDelimiter       string
	boxStyle           string
	boxSeparateColumns bool
	boxDrawBorder      bool
}

func NewEncoderBuilder(encoderType string) EncoderBuilder {
	return &encBuilder{
		RowsEncoderContext: &spi.RowsEncoderContext{},
		encoderType:        encoderType,
		csvDelimiter:       ",",
		boxStyle:           "default",
		boxSeparateColumns: true,
		boxDrawBorder:      true,
	}
}

func (b *encBuilder) Build() spi.RowsEncoder {
	switch b.encoderType {
	case BOX:
		return box.NewEncoder(b.RowsEncoderContext, b.boxStyle, b.boxSeparateColumns, b.boxDrawBorder)
	case CSV:
		return csv.NewEncoder(b.RowsEncoderContext, b.csvDelimiter)
	default: // "json"
		return json.NewEncoder(b.RowsEncoderContext)
	}
}

func (b *encBuilder) SetOutputStream(s spi.OutputStream) EncoderBuilder {
	b.Output = s
	return b
}

func (b *encBuilder) SetTimeLocation(tz *time.Location) EncoderBuilder {
	b.TimeLocation = tz
	return b
}

func (b *encBuilder) SetTimeFormat(f string) EncoderBuilder {
	b.TimeFormat = util.GetTimeformat(f)
	return b
}

func (b *encBuilder) SetPrecision(p int) EncoderBuilder {
	b.Precision = p
	return b
}

func (b *encBuilder) SetRownum(flag bool) EncoderBuilder {
	b.Rownum = flag
	return b
}

func (b *encBuilder) SetHeading(flag bool) EncoderBuilder {
	b.Heading = flag
	return b
}

func (b *encBuilder) SetCsvDelimieter(del string) EncoderBuilder {
	b.csvDelimiter = del
	return b
}

func (b *encBuilder) SetBoxStyle(style string) EncoderBuilder {
	b.boxStyle = style
	return b
}
func (b *encBuilder) SetBoxSeparateColumns(flag bool) EncoderBuilder {
	b.boxSeparateColumns = flag
	return b
}

func (b *encBuilder) SetBoxDrawBorder(flag bool) EncoderBuilder {
	b.boxDrawBorder = flag
	return b
}

type DecoderBuilder interface {
	Build() spi.RowsDecoder
	SetInputStream(reader spi.InputStream) DecoderBuilder
	SetTimeFormat(f string) DecoderBuilder
	SetTimeLocation(tz *time.Location) DecoderBuilder
	SetColumns(columns spi.Columns) DecoderBuilder
	SetCsvHeading(heading bool) DecoderBuilder
	SetCsvDelimieter(del string) DecoderBuilder
	SetTranscoder(trans transcoder.Transcoder) DecoderBuilder
}

type decBuilder struct {
	*spi.RowsDecoderContext
	decoderType  string
	csvDelimiter string
	csvHeading   bool
	transcoder   transcoder.Transcoder
}

func NewDecoderBuilder(decoderType string) DecoderBuilder {
	return &decBuilder{
		RowsDecoderContext: &spi.RowsDecoderContext{},
		decoderType:        decoderType,
		csvDelimiter:       ",",
	}
}

func (b *decBuilder) Build() spi.RowsDecoder {
	switch b.decoderType {
	case CSV:
		return csv.NewDecoder(b.RowsDecoderContext, b.csvDelimiter, b.csvHeading, b.transcoder)
	default: // "json"
		return json.NewDecoder(b.RowsDecoderContext)
	}
}

func (b *decBuilder) SetInputStream(reader spi.InputStream) DecoderBuilder {
	b.Reader = reader
	return b
}

func (b *decBuilder) SetTimeFormat(f string) DecoderBuilder {
	b.RowsDecoderContext.TimeFormat = f
	return b
}

func (b *decBuilder) SetTimeLocation(tz *time.Location) DecoderBuilder {
	b.RowsDecoderContext.TimeLocation = tz
	return b
}

func (b *decBuilder) SetColumns(columns spi.Columns) DecoderBuilder {
	b.RowsDecoderContext.Columns = columns
	return b
}

func (b *decBuilder) SetCsvDelimieter(del string) DecoderBuilder {
	b.csvDelimiter = del
	return b
}

func (b *decBuilder) SetCsvHeading(heading bool) DecoderBuilder {
	b.csvHeading = heading
	return b
}

func (b *decBuilder) SetTranscoder(trans transcoder.Transcoder) DecoderBuilder {
	b.transcoder = trans
	return b
}
