package httpd

import (
	"bytes"
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"
)

var mdHtmlHeader = `
<html lang="en">
<head>
   <meta charset="UTF-8">
   <meta http-equiv="X-UA-Compatible" content="IE=edge">
   <meta name="viewport" content="width=device-width, initial-scale=1.0">
   <link rel="stylesheet" href="/web/assets/github-markdown.css">
   <link rel="stylesheet" href="/web/assets/github-markdown-%s.css">
   <title>%s</title>
</head>
<body>
<br>
<article class="markdown-body">`

var mdHtmlFooter = []byte(`
</article>
<br>
</body>
</html>`)

// POST "/md"
func (svr *httpd) handleMarkdown(ctx *gin.Context) {
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
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
	addHeader := strBool(ctx.Query("header"), true)
	title := strString(ctx.Query("title"), "")
	lightOrDark := "light"
	if strBool(ctx.Query("dark"), false) {
		lightOrDark = "dark"
	}
	ctx.Writer.Header().Set("Content-Type", "text/html")
	if addHeader {
		ctx.Writer.Write([]byte(fmt.Sprintf(mdHtmlHeader, lightOrDark, title)))
	}
	ctx.Writer.Write(dst.Bytes())
	if addHeader {
		ctx.Writer.Write(mdHtmlFooter)
	}
}
