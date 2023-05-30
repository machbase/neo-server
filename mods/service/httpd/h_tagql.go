package httpd

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/machbase/neo-server/mods/service/msg"
	"github.com/machbase/neo-server/mods/tagql"
)

// "/tagql/*path"
func (svr *httpd) handleTagQL(ctx *gin.Context) {
	rsp := &msg.QueryResponse{Success: false, Reason: "not specified"}
	tick := time.Now()

	path := ctx.Param("path")

	if strings.HasSuffix(path, ".tql") {
		script, err := svr.tagqlLoader.Load(path)
		if err != nil {
			svr.log.Error("tql load fail", path, err.Error())
			rsp.Reason = err.Error()
			rsp.Elapse = time.Since(tick).String()
			ctx.JSON(http.StatusNotFound, rsp)
			return
		}

		tql, err := script.Parse()
		if err != nil {
			svr.log.Error("tql parse fail", path, err.Error())
			rsp.Reason = err.Error()
			rsp.Elapse = time.Since(tick).String()
			ctx.JSON(http.StatusInternalServerError, rsp)
			return
		}

		if err := tql.ExecuteHandler(ctx, svr.db, ctx.Writer); err != nil {
			svr.log.Error("tql execute fail", path, err.Error())
			rsp.Reason = err.Error()
			rsp.Elapse = time.Since(tick).String()
			ctx.JSON(http.StatusInternalServerError, rsp)
		}
	} else {
		ql := fmt.Sprintf("?%s", ctx.Request.URL.RawQuery)
		tql, err := tagql.ParseURIContext(ctx, ql)
		if err != nil {
			rsp.Reason = err.Error()
			rsp.Elapse = time.Since(tick).String()
			ctx.JSON(http.StatusBadRequest, rsp)
			return
		}

		if err := tql.ExecuteHandler(ctx, svr.db, ctx.Writer); err != nil {
			svr.log.Error("tagql execute fail", err.Error())
			rsp.Reason = err.Error()
			rsp.Elapse = time.Since(tick).String()
			ctx.JSON(http.StatusInternalServerError, rsp)
		}
	}
}
