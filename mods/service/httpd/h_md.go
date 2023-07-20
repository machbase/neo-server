package httpd

import (
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/machbase/neo-server/mods/util/mdconv"
)

// POST "/md"
// POST "/md?darkMode=true"
func (svr *httpd) handleMarkdown(ctx *gin.Context) {
	src, err := io.ReadAll(ctx.Request.Body)
	if err != nil {
		ctx.String(http.StatusBadRequest, err.Error())
		return
	}
	ctx.Writer.Header().Set("Content-Type", "text/html")
	conv := mdconv.New(mdconv.WithDarkMode(strBool(ctx.Query("darkMode"), false)))
	err = conv.Convert(src, ctx.Writer)
	if err != nil {
		ctx.String(http.StatusInternalServerError, fmt.Sprintf(`<p>%s</p>`, err.Error()))
	}
}
