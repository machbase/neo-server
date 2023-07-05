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
func (svr *httpd) handleMarkdown(ctx *gin.Context) {
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			&mermaid.Extender{NoScript: true},
			&pikchr.Extender{},
			highlighting.NewHighlighting(
				highlighting.WithStyle("catppuccin-macchiato"),
				highlighting.WithFormatOptions(
					chromahtml.WithLineNumbers(true),
				),
			),
		),
		goldmark.WithRendererOptions(
			html.WithHardWraps(),
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
