package renderer

import (
	"github.com/machbase/neo-server/mods/renderer/internal/csvchart"
	"github.com/machbase/neo-server/mods/renderer/internal/jschart"
	"github.com/machbase/neo-server/mods/renderer/internal/termchart"
)

type Option func(r Renderer)

func New(format string, opts ...Option) Renderer {
	var ret Renderer
	switch format {
	case "json":
		ret = jschart.NewJsonRenderer()
	case "html":
		ret = jschart.NewHtmlRenderer(jschart.HtmlOptions{})
	case "term":
		ret = termchart.NewRenderer()
	case "csv":
		ret = csvchart.NewRenderer()
	default:
		return nil
	}
	for _, op := range opts {
		op(ret)
	}
	return ret
}

func Title(title string) Option {
	return func(one Renderer) {
		switch r := one.(type) {
		case *jschart.HtmlRenderer:
			r.Options.Title = title
		}
	}
}

func Subtitle(subtitle string) Option {
	return func(one Renderer) {
		switch r := one.(type) {
		case *jschart.HtmlRenderer:
			r.Options.Subtitle = subtitle
		}
	}
}

func Size(width, height string) Option {
	return func(one Renderer) {
		switch r := one.(type) {
		case *jschart.HtmlRenderer:
			r.Options.Width = width
			r.Options.Height = height
		}
	}
}
