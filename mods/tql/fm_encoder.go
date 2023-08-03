package tql

import (
	"github.com/machbase/neo-server/mods/codec/opts"
)

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

func (node *Node) fmMarkdown(args ...any) *Encoder {
	return newEncoder("markdown", args...)
}

func (node *Node) fmJson(args ...any) *Encoder {
	return newEncoder("json", args...)
}

func (node *Node) fmChartLine(args ...any) *Encoder {
	return newEncoder("echart.line", args...)
}

func (node *Node) fmChartScatter(args ...any) *Encoder {
	return newEncoder("echart.scatter", args...)
}

func (node *Node) fmChartBar(args ...any) *Encoder {
	return newEncoder("echart.bar", args...)
}

func (node *Node) fmChartLine3D(args ...any) *Encoder {
	return newEncoder("echart.line3d", args...)
}

func (node *Node) fmChartBar3D(args ...any) *Encoder {
	return newEncoder("echart.bar3d", args...)
}

func (node *Node) fmChartSurface3D(args ...any) *Encoder {
	return newEncoder("echart.surface3d", args...)
}

func (node *Node) fmChartScatter3D(args ...any) *Encoder {
	return newEncoder("echart.scatter3d", args...)
}

func (node *Node) fmMarkArea(args ...any) (any, error) {
	if len(args) < 2 {
		return nil, ErrInvalidNumOfArgs("markArea", 2, len(args))
	}
	var err error
	coord0 := args[0]
	coord1 := args[1]
	label := ""
	color := ""
	opacity := 1.0
	if len(args) >= 3 {
		if label, err = convString(args, 2, "markArea", "label"); err != nil {
			return nil, err
		}
	}
	if len(args) >= 4 {
		if color, err = convString(args, 3, "markArea", "color"); err != nil {
			return nil, err
		}
	}
	if len(args) >= 5 {
		if opacity, err = convFloat64(args, 4, "markArea", "opacity"); err != nil {
			return nil, err
		}
	}
	return opts.MarkAreaNameCoord(coord0, coord1, label, color, opacity), nil
}
