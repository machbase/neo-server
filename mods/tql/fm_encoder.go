package tql

import (
	"fmt"
	"slices"
	"time"

	"github.com/machbase/neo-server/v8/mods/codec/opts"
)

func newEncoder(format string, args ...any) (*Encoder, error) {
	ret := &Encoder{
		format: format,
	}
	for _, arg := range args {
		switch v := arg.(type) {
		case *time.Location:
			arg = opts.TimeLocation(v)
		case *CacheOption:
			if slices.Contains([]string{"json", "csv", "ndjson"}, format) {
				ret.cacheOption = v
				continue
			} else {
				return nil, fmt.Errorf("encoder '%s' does not support cache", format)
			}
		}

		if opt, ok := arg.(opts.Option); ok {
			ret.opts = append(ret.opts, opt)
		} else {
			return nil, fmt.Errorf("encoder '%s' invalid option %v (%T)", format, arg, arg)
		}
	}
	return ret, nil
}

func (node *Node) fmHtml(args ...any) (*Encoder, error) {
	return newEncoder("html", args...)
}

func (node *Node) fmMarkdown(args ...any) (*Encoder, error) {
	return newEncoder("markdown", args...)
}

func (node *Node) fmJson(args ...any) (*Encoder, error) {
	return newEncoder("json", args...)
}

func (node *Node) fmNDJson(args ...any) (*Encoder, error) {
	return newEncoder("ndjson", args...)
}

func (node *Node) fmGeoMap(args ...any) (*Encoder, error) {
	return newEncoder("geomap", args...)
}
func (node *Node) fmChart(args ...any) (*Encoder, error) {
	return newEncoder("echart", args...)
}

func (node *Node) fmDiscard(args ...any) (*Encoder, error) {
	return newEncoder("discard", args...)
}

func (node *Node) fmChartLine(args ...any) (*Encoder, error) {
	return newEncoder("echart.line", args...)
}

func (node *Node) fmChartScatter(args ...any) (*Encoder, error) {
	return newEncoder("echart.scatter", args...)
}

func (node *Node) fmChartBar(args ...any) (*Encoder, error) {
	return newEncoder("echart.bar", args...)
}

func (node *Node) fmChartLine3D(args ...any) (*Encoder, error) {
	return newEncoder("echart.line3d", args...)
}

func (node *Node) fmChartBar3D(args ...any) (*Encoder, error) {
	return newEncoder("echart.bar3d", args...)
}

func (node *Node) fmChartSurface3D(args ...any) (*Encoder, error) {
	return newEncoder("echart.surface3d", args...)
}

func (node *Node) fmChartScatter3D(args ...any) (*Encoder, error) {
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
