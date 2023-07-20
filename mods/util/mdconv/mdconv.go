package mdconv

import (
	"io"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	pikchr "github.com/jchenry/goldmark-pikchr"
	"github.com/machbase/neo-server/mods/util/mdconv/d2ext"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"
	"go.abhg.dev/goldmark/mermaid"
	"oss.terrastruct.com/d2/d2layouts/d2dagrelayout"
	"oss.terrastruct.com/d2/d2themes/d2themescatalog"
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
	d2theme := d2themescatalog.NeutralDefault.ID
	if c.darkMode {
		highlightingStyle = "catppuccin-macchiato"
		d2theme = d2themescatalog.DarkMauve.ID
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
			// waiting for PR merged in "github.com/FurqanSoftware/goldmark-d2"
			&d2ext.Extender{
				Layout:  d2dagrelayout.DefaultLayout,
				ThemeID: d2theme,
				Sketch:  false,
			},
		),
		goldmark.WithRendererOptions(
			html.WithXHTML(),
		),
	)
	return md.Convert(src, w)
}
