package tql

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"slices"
	"time"

	"github.com/machbase/neo-server/v8/mods/codec/opts"
	"github.com/machbase/neo-server/v8/mods/util/restclient"
	"github.com/machbase/neo-server/v8/mods/util/ssfs"
)

func newEncoder(format string, args ...any) (*Encoder, error) {
	ret := &Encoder{
		format: format,
	}
	for _, arg := range args {
		switch v := arg.(type) {
		case *time.Location:
			arg = opts.TimeLocation(v)
		case *CacheParam:
			if slices.Contains([]string{"json", "csv", "ndjson", "text", "html"}, format) {
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

func toTemplateOption(arg any) (opts.Option, error) {
	switch v := arg.(type) {
	case string:
		return opts.Template(v), nil
	case *FilePath:
		if v.AbsPath != "" {
			if content, err := os.ReadFile(v.AbsPath); err != nil {
				return nil, fmt.Errorf("template file '%s' not found: %w", v.Path, err)
			} else {
				return opts.Template(string(content)), nil
			}
		} else {
			return nil, fmt.Errorf("template file '%s' not found", v.Path)
		}
	}
	return nil, nil
}

func (node *Node) fmHtml(args ...any) (*Encoder, error) {
	for i := range args {
		if o, err := toTemplateOption(args[i]); err != nil {
			return nil, err
		} else if o != nil {
			args[i] = o
		}
	}
	return newEncoder("html", args...)
}

func (node *Node) fmText(args ...any) (*Encoder, error) {
	for i := range args {
		if o, err := toTemplateOption(args[i]); err != nil {
			return nil, err
		} else if o != nil {
			args[i] = o
		}
	}
	return newEncoder("text", args...)
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

func (node *Node) fmHttp(args ...any) (any, error) {
	if len(args) < 1 {
		return nil, ErrInvalidNumOfArgs("HTTP", 1, len(args))
	}
	content, err := convString(args, 0, "HTTP", "content")
	if err != nil {
		return nil, err
	}
	rcli, err := restclient.Parse(content)
	if err != nil {
		return nil, fmt.Errorf("HTTP parse error: %w", err)
	}
	rcli.SetFileLoader(func(path string) (io.ReadCloser, error) {
		def := ssfs.Default()
		ent, err := def.Get(path)
		if err != nil {
			return nil, err
		}

		return io.NopCloser(bytes.NewBuffer(ent.Content)), nil

	})
	result := rcli.Do()
	NewRecord(0, result).Tell(node.next)
	return nil, nil
}
