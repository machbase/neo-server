package httpd

import (
	"bytes"
	"io"
	"net/http"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/gin-gonic/gin"
	pikchr "github.com/jchenry/goldmark-pikchr"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"
	"go.abhg.dev/goldmark/mermaid"
)

// POST "/md"
// POST "/md?darkMode=true"
func (svr *httpd) handleMarkdown(ctx *gin.Context) {
	darkMode := strBool(ctx.Query("darkMode"), false)
	highlightingStyle := "onesenterprise"
	if darkMode {
		highlightingStyle = "catppuccin-macchiato"
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
	var dst bytes.Buffer
	err = md.Convert(src, &dst)
	if err != nil {
		ctx.String(http.StatusInternalServerError, err.Error())
		return
	}
	ctx.Writer.Header().Set("Content-Type", "text/html")
	ctx.Writer.Write(dst.Bytes())
}
