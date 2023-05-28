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
		case *echart.Line:
			e.Output = s
		case *echart.Line3D:
			e.Output = s
		case *echart.Surface3D:
			e.Output = s
		case *echart.Scatter3D:
			e.Output = s
		case *echart.Bar3D:
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
		case *echart.Line:
			e.TimeFormat = f
		case *echart.Line3D:
			e.TimeFormat = f
		case *echart.Surface3D:
			e.TimeFormat = f
		case *echart.Scatter3D:
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
		case *echart.Line3D:
			e.Precision = p
		case *echart.Surface3D:
			e.Precision = p
		case *echart.Scatter3D:
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
		case *echart.Line3D:
			e.Rownum = b
		case *echart.Surface3D:
			e.Rownum = b
		case *echart.Scatter3D:
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
		case *echart.Line3D:
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
		case *echart.Line:
			e.TimeLocation = tz
		case *echart.Line3D:
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
		case *echart.Line:
			e.Title = title
		case *echart.Line3D:
			e.Title = title
		case *echart.Bar3D:
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
		case *echart.Line:
			e.Subtitle = subtitle
		case *echart.Line3D:
			e.Subtitle = subtitle
		case *echart.Bar3D:
			e.Subtitle = subtitle
		case *json.Exporter:
		}
	}
}

func Series(idx int, label string) Option {
	return func(one any) {
		switch e := one.(type) {
		case *echart.Line:
			e.SetSeries(idx, label)
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
		case *echart.Line:
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
		case *echart.Line:
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
		case *echart.Line:
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
		case *echart.Line:
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

func Size(width string, height string) Option {
	return func(one any) {
		switch e := one.(type) {
		case *echart.Bar3D:
			e.Width = width
			e.Height = height
		case *echart.Line:
			e.Width = width
			e.Height = height
		}
	}
}
