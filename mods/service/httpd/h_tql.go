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
	"github.com/golang-jwt/jwt/v4"
	"github.com/machbase/neo-server/mods/service/msg"
	"github.com/machbase/neo-server/mods/tql"
	"github.com/machbase/neo-server/mods/util"
)

const TqlHeaderChartType = "X-Chart-Type"
const TqlHeaderConsoleId = "X-Console-Id"

// POST "/tql"
func (svr *httpd) handlePostTagQL(ctx *gin.Context) {
	rsp := &msg.QueryResponse{Success: false, Reason: "not specified"}
	tick := time.Now()

	var consoleId string
	var consoleLogLevel tql.Level
	var logLevel tql.Level
	var claim *jwt.RegisteredClaims
	if val, exists := ctx.Get("jwt-claim"); exists {
		claim = val.(*jwt.RegisteredClaims)
	}
	consoleId = ctx.GetHeader(TqlHeaderConsoleId)
	if fields := util.SplitFields(consoleId, true); len(fields) > 1 {
		consoleId = fields[0]
		for _, field := range fields[1:] {
			kvpair := strings.SplitN(field, "=", 2)
			if len(kvpair) == 2 {
				switch strings.ToLower(kvpair[0]) {
				case "console-log-level":
					consoleLogLevel = tql.ParseLogLevel(kvpair[1])
				case "log-level":
					logLevel = tql.ParseLogLevel(kvpair[1])
				}
			}
		}
	}

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
	task.SetLogLevel(logLevel)
	task.SetConsoleLogLevel(consoleLogLevel)
	if claim != nil && consoleId != "" {
		task.SetConsole(claim.Subject, consoleId)
	}
	task.SetOutputWriterJson(ctx.Writer, true)
	task.SetDatabase(svr.db)
	if err := task.Compile(input); err != nil {
		svr.log.Error("tql parse error", err.Error())
		rsp.Reason = err.Error()
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}
	ctx.Writer.Header().Set("Content-Type", task.OutputContentType())
	ctx.Writer.Header().Set("Content-Encoding", task.OutputContentEncoding())
	if chart := task.OutputChartType(); len(chart) > 0 {
		ctx.Writer.Header().Set(TqlHeaderChartType, chart)
	}
	if err := task.Execute(); err != nil {
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
		contentType := contentTypeOfFile(path)
		if contentType != "" && ctx.Request.Method == http.MethodGet {
			if ent, err := svr.serverFs.Get(path); err == nil && !ent.IsDir {
				ctx.Data(http.StatusOK, contentType, ent.Content)
				return
			}
		}
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
	task.SetDatabase(svr.db)
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

	contentType := task.OutputContentType()
	switch contentType {
	case "application/xhtml+xml":
		ctx.Writer.Header().Set("Content-Type", "text/html")
	default:
		ctx.Writer.Header().Set("Content-Type", contentType)
	}
	ctx.Writer.Header().Set("Content-Encoding", task.OutputContentEncoding())
	if chart := task.OutputChartType(); len(chart) > 0 {
		ctx.Writer.Header().Set(TqlHeaderChartType, chart)
	}
	if err := task.Execute(); err != nil {
		svr.log.Error("tql execute fail", path, err.Error())
		rsp.Reason = err.Error()
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
	}
}
