package tql

import (
	"github.com/machbase/neo-server/mods/codec/opts"
	"github.com/pkg/errors"
)

func newEncoder(format string, args ...any) (*Encoder, error) {
	ret := &Encoder{
		format: format,
	}
	for _, arg := range args {
		if opt, ok := arg.(opts.Option); ok {
			ret.opts = append(ret.opts, opt)
		} else {
			return nil, errors.New("invalid option")
		}
	}
	return ret, nil
}

func (node *Node) fmHtml(args ...any) *Encoder {
	enc, _ := newEncoder("html", args...)
	return enc
}

func (node *Node) fmMarkdown(args ...any) *Encoder {
	enc, _ := newEncoder("markdown", args...)
	return enc
}

func (node *Node) fmJson(args ...any) *Encoder {
	enc, _ := newEncoder("json", args...)
	return enc
}

func (node *Node) fmChartLine(args ...any) *Encoder {
	enc, _ := newEncoder("echart.line", args...)
	return enc
}

func (node *Node) fmChartScatter(args ...any) *Encoder {
	enc, _ := newEncoder("echart.scatter", args...)
	return enc
}

func (node *Node) fmChartBar(args ...any) *Encoder {
	enc, _ := newEncoder("echart.bar", args...)
	return enc
}

func (node *Node) fmChartLine3D(args ...any) *Encoder {
	enc, _ := newEncoder("echart.line3d", args...)
	return enc
}

func (node *Node) fmChartBar3D(args ...any) *Encoder {
	enc, _ := newEncoder("echart.bar3d", args...)
	return enc
}

func (node *Node) fmChartSurface3D(args ...any) *Encoder {
	enc, _ := newEncoder("echart.surface3d", args...)
	return enc
}

func (node *Node) fmChartScatter3D(args ...any) *Encoder {
	enc, _ := newEncoder("echart.scatter3d", args...)
	return enc
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
