package codec

import (
	"time"

	"github.com/machbase/neo-server/mods/codec/internal/box"
	"github.com/machbase/neo-server/mods/codec/internal/csv"
	"github.com/machbase/neo-server/mods/codec/internal/echart"
	"github.com/machbase/neo-server/mods/codec/internal/json"
	"github.com/machbase/neo-server/mods/stream/spec"
	"github.com/machbase/neo-server/mods/transcoder"
	spi "github.com/machbase/neo-spi"
)

func OutputStream(s spec.OutputStream) Option {
	return func(one any) {
		switch e := one.(type) {
		case *box.Exporter:
			e.Output = s
		case *csv.Exporter:
			e.Output = s
		case *echart.Exporter:
			e.Output = s
		case *json.Exporter:
			e.Output = s
		}
	}
}

func TimeFormat(f string) Option {
	return func(one any) {
		switch e := one.(type) {
		case *box.Exporter:
			e.TimeFormat = f
		case *csv.Exporter:
			e.TimeFormat = f
		case *csv.Decoder:
			e.TimeFormat = f
		case *echart.Exporter:
			e.TimeFormat = f
		case *json.Exporter:
			e.TimeFormat = f
		case *json.Decoder:
			e.TimeFormat = f
		}
	}
}
func Precision(p int) Option {
	return func(one any) {
		switch e := one.(type) {
		case *box.Exporter:
			e.Precision = p
		case *csv.Exporter:
			e.Precision = p
		case *echart.Exporter:
			e.Precision = p
		case *json.Exporter:
			e.Precision = p
		}
	}
}

func Rownum(b bool) Option {
	return func(one any) {
		switch e := one.(type) {
		case *box.Exporter:
			e.Rownum = b
		case *csv.Exporter:
			e.Rownum = b
		case *echart.Exporter:
			e.Rownum = b
		case *json.Exporter:
			e.Rownum = b
		}
	}
}

func Heading(b bool) Option {
	return func(one any) {
		switch e := one.(type) {
		case *box.Exporter:
			e.Heading = b
		case *csv.Exporter:
			e.Heading = b
		case *csv.Decoder:
			e.Heading = b
		case *echart.Exporter:
			e.Heading = b
		case *json.Exporter:
			e.Heading = b
		}
	}
}

func TimeLocation(tz *time.Location) Option {
	return func(one any) {
		switch e := one.(type) {
		case *box.Exporter:
			e.TimeLocation = tz
		case *csv.Exporter:
			e.TimeLocation = tz
		case *csv.Decoder:
			e.TimeLocation = tz
		case *echart.Exporter:
			e.TimeLocation = tz
		case *json.Exporter:
			e.TimeLocation = tz
		case *json.Decoder:
			e.TimeLocation = tz
		}
	}
}
func Title(title string) Option {
	return func(one any) {
		switch e := one.(type) {
		case *box.Exporter:
		case *csv.Exporter:
		case *echart.Exporter:
			e.Title = title
		case *json.Exporter:
		}
	}
}

func Subtitle(subtitle string) Option {
	return func(one any) {
		switch e := one.(type) {
		case *box.Exporter:
		case *csv.Exporter:
		case *echart.Exporter:
			e.Subtitle = subtitle
		case *json.Exporter:
		}
	}
}

func Delimiter(delimiter string) Option {
	return func(one any) {
		switch e := one.(type) {
		case *box.Exporter:
		case *csv.Exporter:
			e.SetDelimiter(delimiter)
		case *csv.Decoder:
			e.SetDelimiter(delimiter)
		case *echart.Exporter:
		case *json.Exporter:
		}
	}
}

// BOX only
func BoxStyle(style string) Option {
	return func(one any) {
		switch e := one.(type) {
		case *box.Exporter:
			e.Style = style
		case *csv.Exporter:
		case *echart.Exporter:
		case *json.Exporter:
		}
	}
}

func BoxSeparateColumns(flag bool) Option {
	return func(one any) {
		switch e := one.(type) {
		case *box.Exporter:
			e.SeparateColumns = flag
		case *csv.Exporter:
		case *echart.Exporter:
		case *json.Exporter:
		}
	}
}

func BoxDrawBorder(flag bool) Option {
	return func(one any) {
		switch e := one.(type) {
		case *box.Exporter:
			e.DrawBorder = flag
		case *csv.Exporter:
		case *echart.Exporter:
		case *json.Exporter:
		}
	}
}

// Decoder only
func InputStream(in spec.InputStream) Option {
	return func(one any) {
		switch e := one.(type) {
		case *csv.Decoder:
			e.Input = in
		case *json.Decoder:
			e.Input = in
		}
	}
}

func Table(table string) Option {
	return func(one any) {
		switch e := one.(type) {
		case *csv.Decoder:
			e.TableName = table
		case *json.Decoder:
			e.TableName = table
		}
	}
}

func Transcoder(trans transcoder.Transcoder) Option {
	return func(one any) {
		switch e := one.(type) {
		case *csv.Decoder:
			e.Translator = trans
		case *json.Decoder:
		}
	}
}

func Columns(cols spi.Columns) Option {
	return func(one any) {
		switch e := one.(type) {
		case *csv.Decoder:
			e.Columns = cols
		case *json.Decoder:
			e.Columns = cols
		}
	}
}
