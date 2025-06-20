package mdconv

import (
	"io"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	pikchr "github.com/jchenry/goldmark-pikchr"
	"github.com/machbase/neo-server/v8/mods/util/mdconv/d2ext"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"
	"go.abhg.dev/goldmark/mermaid"
)

type Converter struct {
	darkMode bool
}

type Option func(*Converter)

func WithDarkMode(flag bool) Option {
	return func(c *Converter) {
		c.darkMode = flag
	}
}

func New(opts ...Option) *Converter {
	ret := &Converter{}
	for _, o := range opts {
		o(ret)
	}
	return ret
}

func (c *Converter) ConvertString(src string, w io.Writer) error {
	return c.Convert([]byte(src), w)
}

func (c *Converter) Convert(src []byte, w io.Writer) error {
	highlightingStyle := "onesenterprise"
	if c.darkMode {
		highlightingStyle = "catppuccin-macchiato"
	}

	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			&mermaid.Extender{RenderMode: mermaid.RenderModeClient, NoScript: true},
			&pikchr.Extender{DarkMode: c.darkMode},
			highlighting.NewHighlighting(
				highlighting.WithStyle(highlightingStyle),
				highlighting.WithFormatOptions(
					chromahtml.WithLineNumbers(true),
					chromahtml.WrapLongLines(true),
				),
			),
			&d2ext.Extender{},
		),
		goldmark.WithRendererOptions(
			html.WithXHTML(),
		),
	)
	return md.Convert(src, w)
}
