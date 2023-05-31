package httpd

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/machbase/neo-server/mods/service/msg"
	"github.com/machbase/neo-server/mods/tagql"
)

// POST "/tql"
func (svr *httpd) handlePostTagQL(ctx *gin.Context) {
	rsp := &msg.QueryResponse{Success: false, Reason: "not specified"}
	tick := time.Now()

	params, err := url.ParseQuery(ctx.Request.URL.RawQuery)
	if err != nil {
		svr.log.Error("tql get params error", err.Error())
		rsp.Reason = err.Error()
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}

	tql, err := tagql.ParseWithParams(ctx.Request.Body, params)
	if err != nil {
		svr.log.Error("tql parse error", err.Error())
		rsp.Reason = err.Error()
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}
	if err := tql.ExecuteHandler(ctx, svr.db, ctx.Writer); err != nil {
		svr.log.Error("tql execute error", err.Error())
		rsp.Reason = err.Error()
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
	}
}

// GET  "/tql/*path"
func (svr *httpd) handleTagQL(ctx *gin.Context) {
	rsp := &msg.QueryResponse{Success: false, Reason: "not specified"}
	tick := time.Now()

	path := ctx.Param("path")
	params, err := url.ParseQuery(ctx.Request.URL.RawQuery)
	if err != nil {
		svr.log.Error("tql parse query fail", path, err.Error())
		rsp.Reason = err.Error()
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusNotFound, rsp)
		return
	}

	if strings.HasSuffix(path, ".tql") {
		script, err := svr.tagqlLoader.Load(path)
		if err != nil {
			svr.log.Error("tql load fail", path, err.Error())
			rsp.Reason = err.Error()
			rsp.Elapse = time.Since(tick).String()
			ctx.JSON(http.StatusNotFound, rsp)
			return
		}

		tql, err := script.ParseWithParams(params)
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
		tql, err := tagql.ParseContext(ctx, params)
		if err != nil {
			rsp.Reason = err.Error()
			rsp.Elapse = time.Since(tick).String()
			ctx.JSON(http.StatusBadRequest, rsp)
			return
		}

		if err := tql.ExecuteHandler(ctx, svr.db, ctx.Writer); err != nil {
			svr.log.Error("tql execute fail", err.Error())
			rsp.Reason = err.Error()
			rsp.Elapse = time.Since(tick).String()
			ctx.JSON(http.StatusInternalServerError, rsp)
		}
	}
}
