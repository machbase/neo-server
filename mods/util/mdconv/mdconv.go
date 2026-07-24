package mdconv

import (
	"context"
	"fmt"
	"io"
	"regexp"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	pikchr "github.com/jchenry/goldmark-pikchr"
	"github.com/machbase/neo-server/v8/mods/util/mdconv/chartext"
	"github.com/machbase/neo-server/v8/mods/util/mdconv/d2ext"
	"github.com/machbase/neo-server/v8/mods/util/mdconv/geomapext"
	"github.com/machbase/neo-server/v8/mods/util/mdconv/httpext"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"
	"go.abhg.dev/goldmark/mermaid"
	"oss.terrastruct.com/d2/d2graph"
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

func (c *Converter) Convert(src []byte, w io.Writer) (retErr error) {
	defer func() {
		if r := recover(); r != nil {
			// Handle the panic and return an error instead of crashing the program
			err, ok := r.(error)
			if !ok {
				err = fmt.Errorf("panic: %v", r)
			}
			retErr = err
		}
	}()
	highlightingStyle := "onesenterprise"
	if c.darkMode {
		highlightingStyle = "catppuccin-macchiato"
	}

	// Preprocess: map jsh/jsh-run code fences to javascript
	src = c.mapCodeFenceLanguage(src)

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
			&d2ext.Extender{
				Layout: func(ctx context.Context, g *d2graph.Graph) error {
					return d2dagrelayout.Layout(ctx, g, &d2dagrelayout.ConfigurableOpts{
						NodeSep: 20,
						EdgeSep: 20,
					})
				},
				ThemeID: &d2themescatalog.CoolClassics.ID,
				Sketch:  false,
			},
			&chartext.Extender{DarkMode: c.darkMode},
			&geomapext.Extender{DarkMode: c.darkMode},
			&httpext.Extender{},
		),
		goldmark.WithRendererOptions(
			html.WithXHTML(),
		),
	)
	return md.Convert(src, w)
}

// mapCodeFenceLanguage converts jsh/jsh-run code fences to javascript for syntax highlighting
func (c *Converter) mapCodeFenceLanguage(src []byte) []byte {
	// Pattern to match code fences with jsh or jsh-run language
	// Note: jsh-run must come before jsh in alternation to match correctly
	pattern := regexp.MustCompile("(?m)^```(?:jsh-run|jsh)(.*)$")

	return pattern.ReplaceAll(src, []byte("```javascript$1"))
}
