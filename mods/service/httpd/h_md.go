package httpd

import (
	"context"
	"fmt"
	"io"
	"net/http"

	d2lang "github.com/FurqanSoftware/goldmark-d2"
	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/gin-gonic/gin"
	pikchr "github.com/jchenry/goldmark-pikchr"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"
	"go.abhg.dev/goldmark/mermaid"
	"oss.terrastruct.com/d2/d2graph"
	"oss.terrastruct.com/d2/d2layouts/d2dagrelayout"
	"oss.terrastruct.com/d2/d2themes/d2themescatalog"
)

// POST "/md"
// POST "/md?darkMode=true"
func (svr *httpd) handleMarkdown(ctx *gin.Context) {
	darkMode := strBool(ctx.Query("darkMode"), false)
	highlightingStyle := "onesenterprise"
	d2theme := d2themescatalog.NeutralDefault.ID
	if darkMode {
		highlightingStyle = "catppuccin-macchiato"
		d2theme = d2themescatalog.DarkMauve.ID
	}
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			&mermaid.Extender{RenderMode: mermaid.RenderModeClient, NoScript: true},
			&pikchr.Extender{DarkMode: darkMode},
			highlighting.NewHighlighting(
				highlighting.WithStyle(highlightingStyle),
				highlighting.WithFormatOptions(
					chromahtml.WithLineNumbers(true),
					chromahtml.WrapLongLines(true),
				),
			),
			&d2lang.Extender{
				Layout: func(ctx context.Context, gr *d2graph.Graph) error {
					return d2dagrelayout.Layout(ctx, gr, &d2dagrelayout.DefaultOpts)
				},
				ThemeID: d2theme,
				Sketch:  false,
			},
		),
		goldmark.WithRendererOptions(
			html.WithXHTML(),
		),
	)
	src, err := io.ReadAll(ctx.Request.Body)
	if err != nil {
		ctx.String(http.StatusBadRequest, err.Error())
		return
	}
	ctx.Writer.Header().Set("Content-Type", "text/html")
	err = md.Convert(src, ctx.Writer)
	if err != nil {
		ctx.String(http.StatusInternalServerError, fmt.Sprintf(`<p>%s</p>`, err.Error()))
	}
}
