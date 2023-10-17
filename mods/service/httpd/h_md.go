package httpd

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strings"

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
	var referer string
	if dec, err := base64.StdEncoding.DecodeString(ctx.GetHeader("X-Referer")); err != nil {
		ctx.String(http.StatusBadRequest, err.Error())
		return
	} else {
		referer = string(dec)
	}
	// referer := "http://127.0.0.1:5654/web/api/tql/sample_image.wrk" // if file has been saved
	// referer := "http://127.0.0.1:5654/web/ui" // file is not saved
	var filePath, fileName, fileDir string
	if u, err := url.Parse(referer); err == nil {
		// {{ file_path }} => /web/api/tql/path/to/file.wrk
		// {{ file_name }} => file.wrk
		// {{ file_dir }}  => /web/api/tql/path/to
		filePath = u.Path
		fileName = path.Base(filePath)
		fileDir = path.Dir(filePath)
	}
	// {{ file_root }} => /web/api/tql
	fileRoot := path.Join(strings.TrimSuffix(ctx.Request.URL.Path, "/md"), "tql")
	src = regexp.MustCompile(`{{\s*file_root\s*}}`).ReplaceAll(src, []byte(fileRoot))
	src = regexp.MustCompile(`{{\s*file_path\s*}}`).ReplaceAll(src, []byte(filePath))
	src = regexp.MustCompile(`{{\s*file_name\s*}}`).ReplaceAll(src, []byte(fileName))
	src = regexp.MustCompile(`{{\s*file_dir\s*}}`).ReplaceAll(src, []byte(fileDir))

	ctx.Writer.Header().Set("Content-Type", "application/xhtml+xml")
	conv := mdconv.New(mdconv.WithDarkMode(strBool(ctx.Query("darkMode"), false)))
	ctx.Writer.Write([]byte("<div>"))
	err = conv.Convert(src, ctx.Writer)
	if err != nil {
		ctx.String(http.StatusInternalServerError, fmt.Sprintf(`<p>%s</p>`, err.Error()))
	}
	ctx.Writer.Write([]byte("</div>"))
}
