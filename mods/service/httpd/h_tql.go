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
	"github.com/machbase/neo-server/v8/mods/service/msg"
	"github.com/machbase/neo-server/v8/mods/tql"
	"github.com/machbase/neo-server/v8/mods/util"
)

const TqlHeaderChartType = "X-Chart-Type"
const TqlHeaderChartOutput = "X-Chart-Output"
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

const TQL_SCRIPT_PARAM = "$"
const TQL_TOKEN_PARAM = "$token"

// POST "/tql/tql-exec" accepts the access token in the query parameter
func (svr *httpd) handleTqlQueryExec(ctx *gin.Context) {
	if token := ctx.Query(TQL_TOKEN_PARAM); token != "" {
		ctx.Request.Header.Set("Authorization", "Bearer "+token)
	}
	svr.handleJwtToken(ctx)
	if ctx.IsAborted() {
		return
	}
	svr.handleTqlQuery(ctx)
}

// POST "/tql"
// POST "/tql?$=...."
// GET  "/tql?$=...."
func (svr *httpd) handleTqlQuery(ctx *gin.Context) {
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

	var codeReader io.Reader
	var input io.Reader
	var debug = false
	if ctx.Request.Method == http.MethodPost {
		if script := ctx.Query(TQL_SCRIPT_PARAM); script == "" {
			if debug {
				b, _ := io.ReadAll(ctx.Request.Body)
				fmt.Println("...", string(b), "...")
				codeReader = bytes.NewBuffer(b)
			} else {
				codeReader = ctx.Request.Body
			}
		} else {
			codeReader = bytes.NewBufferString(script)
			if debug {
				fmt.Println("...", script, "...")
			}
			params.Del(TQL_SCRIPT_PARAM)
			params.Del(TQL_TOKEN_PARAM)
			input = ctx.Request.Body
		}
	} else if ctx.Request.Method == http.MethodGet {
		if script := ctx.Query(TQL_SCRIPT_PARAM); script != "" {
			codeReader = bytes.NewBufferString(script)
			params.Del(TQL_SCRIPT_PARAM)
			params.Del(TQL_TOKEN_PARAM)
		} else {
			rsp.Reason = "script not found"
			rsp.Elapse = time.Since(tick).String()
			ctx.JSON(http.StatusBadRequest, rsp)
			return
		}
	} else {
		rsp.Reason = "unsupported method"
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusMethodNotAllowed, rsp)
		return
	}

	task := tql.NewTaskContext(ctx)
	task.SetParams(params)
	task.SetInputReader(input)
	task.SetLogLevel(consoleInfo.logLevel)
	task.SetConsoleLogLevel(consoleInfo.consoleLogLevel)
	if claim != nil && consoleInfo.consoleId != "" {
		if svr.authServer == nil {
			task.SetConsole(claim.Subject, consoleInfo.consoleId, "")
		} else {
			otp, _ := svr.authServer.GenerateOtp(claim.Subject)
			task.SetConsole(claim.Subject, consoleInfo.consoleId, "$otp$:"+otp)
		}
	}
	task.SetOutputWriterJson(ctx.Writer, true)
	task.SetDatabase(svr.db)
	if err := task.Compile(codeReader); err != nil {
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
	if headers := task.OutputHttpHeaders(); len(headers) > 0 {
		for k, vs := range headers {
			for _, v := range vs {
				ctx.Writer.Header().Set(k, v)
			}
		}
	}
	go func() {
		<-ctx.Request.Context().Done()
		task.Cancel()
	}()

	result := task.Execute()
	if result == nil {
		svr.log.Error("tql execute return nil")
		rsp.Reason = "task result is empty"
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
	} else if result.IsDbSink {
		ctx.JSON(http.StatusOK, result)
	} else if !ctx.Writer.Written() {
		// clear headers for the json result
		ctx.Writer.Header().Set("Content-Type", "application/json")
		ctx.Writer.Header().Del("Content-Encoding")
		ctx.Writer.Header().Del(TqlHeaderChartType)
		ctx.JSON(http.StatusOK, result)
	}
}

// tql as RESTful API
//
// GET  "/tql/*path"
// POST "/tql/*path"
func (svr *httpd) handleTqlFile(ctx *gin.Context) {
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
		rsp.Reason = "tql not found"
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
	task.SetParams(params)
	if ctx.Request.Header.Get(TqlHeaderChartOutput) == "json" {
		task.SetOutputWriterJson(ctx.Writer, true)
	} else {
		task.SetOutputWriter(ctx.Writer)
	}
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
	if headers := task.OutputHttpHeaders(); len(headers) > 0 {
		for k, vs := range headers {
			for _, v := range vs {
				ctx.Writer.Header().Set(k, v)
			}
		}
	}
	go func() {
		<-ctx.Request.Context().Done()
		task.Cancel()
	}()

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
