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
	"github.com/machbase/neo-server/mods/util"
)

const TqlHeaderChartType = "X-Chart-Type"
const TqlHeaderConsoleId = "X-Console-Id"

type ConsoleInfo struct {
	consoleId       string
	consoleLogLevel tql.Level
	logLevel        tql.Level
}

func parseConsoleId(ctx *gin.Context) *ConsoleInfo {
	ret := &ConsoleInfo{}
	ret.consoleId = ctx.GetHeader(TqlHeaderConsoleId)
	if fields := util.SplitFields(ret.consoleId, true); len(fields) > 1 {
		ret.consoleId = fields[0]
		for _, field := range fields[1:] {
			kvpair := strings.SplitN(field, "=", 2)
			if len(kvpair) == 2 {
				switch strings.ToLower(kvpair[0]) {
				case "console-log-level":
					ret.consoleLogLevel = tql.ParseLogLevel(kvpair[1])
				case "log-level":
					ret.logLevel = tql.ParseLogLevel(kvpair[1])
				}
			}
		}
	}
	return ret
}

// POST "/tql"
func (svr *httpd) handlePostTagQL(ctx *gin.Context) {
	rsp := &msg.QueryResponse{Success: false, Reason: "not specified"}
	tick := time.Now()

	claim, _ := svr.getJwtClaim(ctx)
	consoleInfo := parseConsoleId(ctx)

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
	task.SetLogLevel(consoleInfo.logLevel)
	task.SetConsoleLogLevel(consoleInfo.consoleLogLevel)
	if claim != nil && consoleInfo.consoleId != "" {
		task.SetConsole(claim.Subject, consoleInfo.consoleId)
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
	task.SetVolatileAssetsProvider(svr.memoryFs)
	ctx.Writer.Header().Set("Content-Type", task.OutputContentType())
	ctx.Writer.Header().Set("Content-Encoding", task.OutputContentEncoding())
	if chart := task.OutputChartType(); len(chart) > 0 {
		ctx.Writer.Header().Set(TqlHeaderChartType, chart)
	}
	result := task.Execute()
	if result == nil {
		svr.log.Error("tql execute return nil")
		rsp.Reason = "task result is empty"
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
	} else if result.IsDbSink {
		ctx.JSON(http.StatusOK, result)
	} else if !ctx.Writer.Written() {
		ctx.JSON(http.StatusOK, result)
	}
	// TODO handling error while processing TQL
	// It could be another content-type than json.
	// } else {
	// }
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
				ctx.Header("Content-Type", contentType)
				ctx.Writer.Write(ent.Content)
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
	result := task.Execute()
	if result == nil {
		svr.log.Error("tql execute return nil")
		rsp.Reason = "task result is empty"
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
	} else if result.IsDbSink {
		ctx.JSON(http.StatusOK, result)
	} else if !ctx.Writer.Written() {
		ctx.JSON(http.StatusOK, result)
	}
}
