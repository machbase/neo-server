package maps

import (
	"github.com/machbase/neo-server/mods/codec"
	"github.com/machbase/neo-server/mods/codec/opts"
	"github.com/machbase/neo-server/mods/stream/spec"
)

type Encoder struct {
	format string
	opts   []opts.Option
}

func (enc *Encoder) New(out spec.OutputStream) codec.RowsEncoder {
	codecOpts := []opts.Option{
		opts.AssetHost("/web/echarts/"),
		opts.OutputStream(out),
	}
	codecOpts = append(codecOpts, enc.opts...)
	return codec.NewEncoder(enc.format, codecOpts...)
}

func newEncoder(format string, args ...any) *Encoder {
	ret := &Encoder{
		format: format,
	}
	for _, arg := range args {
		if opt, ok := arg.(opts.Option); ok {
			ret.opts = append(ret.opts, opt)
		}
	}
	return ret
}

func ToMarkdown(args ...any) *Encoder {
	return newEncoder("markdown", args...)
}

func ToCsv(args ...any) *Encoder {
	return newEncoder("csv", args...)
}

func ToJson(args ...any) *Encoder {
	return newEncoder("json", args...)
}

func ChartLine(args ...any) *Encoder {
	return newEncoder("echart.line", args...)
}

func ChartScatter(args ...any) *Encoder {
	return newEncoder("echart.scatter", args...)
}

func ChartBar(args ...any) *Encoder {
	return newEncoder("echart.bar", args...)
}

func ChartLine3D(args ...any) *Encoder {
	return newEncoder("echart.line3d", args...)
}

func ChartBar3D(args ...any) *Encoder {
	return newEncoder("echart.bar3d", args...)
}

func ChartSurface3D(args ...any) *Encoder {
	return newEncoder("echart.surface3d", args...)
}

func ChartScatter3D(args ...any) *Encoder {
	return newEncoder("echart.scatter3d", args...)
}
