package httpd

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/machbase/neo-server/mods/service/msg"
	"github.com/machbase/neo-server/mods/tql"
)

// POST "/tql"
func (svr *httpd) handlePostTagQL(ctx *gin.Context) {
	rsp := &msg.QueryResponse{Success: false, Reason: "not specified"}
	tick := time.Now()

	params, err := url.ParseQuery(ctx.Request.URL.RawQuery)
	if err != nil {
		svr.log.Error("tql params error", err.Error())
		rsp.Reason = err.Error()
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}

	var input io.Reader
	var debug = false
	if debug {
		b, _ := io.ReadAll(ctx.Request.Body)
		fmt.Println("...", string(b), "...")
		input = bytes.NewBuffer(b)
	} else {
		input = ctx.Request.Body
	}

	task := tql.NewTaskContext(ctx)
	task.SetParams(params)
	task.SetOutputWriterJson(ctx.Writer, true)
	if err := task.Compile(input); err != nil {
		svr.log.Error("tql parse error", err.Error())
		rsp.Reason = err.Error()
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}
	if err := task.ExecuteHandler(svr.db, ctx.Writer); err != nil {
		svr.log.Error("tql execute error", err.Error())
		rsp.Reason = err.Error()
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
	}
}

// tql as RESTful API
//
// GET  "/tql/*path"
// POST "/tql/*path"
func (svr *httpd) handleTagQL(ctx *gin.Context) {
	rsp := &msg.QueryResponse{Success: false, Reason: "not specified"}
	tick := time.Now()

	path := ctx.Param("path")
	if !strings.HasSuffix(path, ".tql") {
		rsp.Reason = "no tql found"
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusNotFound, rsp)
		return
	}
	params, err := url.ParseQuery(ctx.Request.URL.RawQuery)
	if err != nil {
		svr.log.Error("tql params error", path, err.Error())
		rsp.Reason = err.Error()
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusNotFound, rsp)
		return
	}

	script, err := svr.tqlLoader.Load(path)
	if err != nil {
		svr.log.Error("tql load fail", path, err.Error())
		rsp.Reason = err.Error()
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusNotFound, rsp)
		return
	}

	task := tql.NewTaskContext(ctx)
	task.SetInputReader(ctx.Request.Body)
	task.SetOutputWriter(ctx.Writer)
	task.SetParams(params)
	if err := task.CompileScript(script); err != nil {
		svr.log.Error("tql parse fail", path, err.Error())
		rsp.Reason = err.Error()
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}

	if err := task.ExecuteHandler(svr.db, ctx.Writer); err != nil {
		svr.log.Error("tql execute fail", path, err.Error())
		rsp.Reason = err.Error()
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
	}
}
