package httpsvr

import (
	"compress/gzip"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/machbase/neo-server/mods/msg"
	"github.com/machbase/neo-shell/codec"
	"github.com/machbase/neo-shell/do"
	"github.com/machbase/neo-shell/stream"
	spi "github.com/machbase/neo-spi"
)

func (svr *Server) handleQuery(ctx *gin.Context) {
	rsp := &msg.QueryResponse{Success: false, Reason: "not specified"}
	tick := time.Now()
	defer func() {
		rsp.Elapse = time.Since(tick).String()
	}()

	var err error
	req := &msg.QueryRequest{Precision: -1}
	if ctx.Request.Method == http.MethodPost {
		contentType := ctx.ContentType()
		if contentType == "application/json" {
			req.Timeformat = "ns"
			req.TimeLocation = "UTC"
			req.Format = "json"
			req.Rownum = false
			req.Heading = true
			req.Precision = -1
			if err = ctx.Bind(req); err != nil {
				rsp.Reason = err.Error()
				ctx.JSON(http.StatusBadRequest, rsp)
				return
			}
		} else if contentType == "application/x-www-form-urlencoded" {
			req.SqlText = ctx.PostForm("q")
			req.Timeformat = strString(ctx.PostForm("timeformat"), "ns")
			req.TimeLocation = strString(ctx.PostForm("tz"), "UTC")
			req.Format = strString(ctx.PostForm("format"), "json")
			req.Compress = ctx.PostForm("compress")
			req.Rownum = strBool(ctx.PostForm("rownum"), false)
			req.Heading = strBool(ctx.PostForm("heading"), true)
			req.Precision = strInt(ctx.PostForm("precision"), -1)
		} else {
			rsp.Reason = fmt.Sprintf("unsupported content-type: %s", contentType)
			ctx.JSON(http.StatusBadRequest, rsp)
			return
		}
	} else if ctx.Request.Method == http.MethodGet {
		req.SqlText = ctx.Query("q")
		req.Timeformat = strString(ctx.Query("timeformat"), "ns")
		req.TimeLocation = strString(ctx.Query("tz"), "UTC")
		req.Format = strString(ctx.Query("format"), "json")
		req.Compress = ctx.Query("compress")
		req.Rownum = strBool(ctx.Query("rownum"), false)
		req.Heading = strBool(ctx.Query("heading"), true)
		req.Precision = strInt(ctx.Query("precision"), -1)
	}

	if len(req.SqlText) == 0 {
		rsp.Reason = "empty sql"
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}

	var timeLocation = strTimeLocation(req.TimeLocation, time.UTC)

	var output spi.OutputStream
	switch req.Compress {
	case "gzip":
		output = &stream.WriterOutputStream{Writer: gzip.NewWriter(ctx.Writer)}
	default:
		req.Compress = ""
		output = &stream.WriterOutputStream{Writer: ctx.Writer}
	}

	encoder := codec.NewEncoderBuilder(req.Format).
		SetOutputStream(output).
		SetTimeLocation(timeLocation).
		SetTimeFormat(req.Timeformat).
		SetPrecision(req.Precision).
		SetRownum(req.Rownum).
		SetHeading(req.Heading).
		SetBoxStyle("default").
		SetBoxSeparateColumns(true).
		SetBoxDrawBorder(true).
		SetCsvDelimieter(",").
		Build()

	queryCtx := &do.QueryContext{
		DB: svr.db,
		OnFetchStart: func(cols spi.Columns) {
			ctx.Writer.Header().Set("Content-Type", encoder.ContentType())
			if len(req.Compress) > 0 {
				ctx.Writer.Header().Set("Content-Encoding", req.Compress)
			}
			encoder.Open(cols)
		},
		OnFetch: func(nrow int64, values []any) bool {
			err := encoder.AddRow(values)
			if err != nil {
				// report error to client?
				svr.log.Errorf("render", err.Error())
				return false
			}
			return true
		},
		OnFetchEnd: func() {
			encoder.Close()
		},
	}
	if err := do.Query(queryCtx, req.SqlText); err != nil {
		rsp.Reason = err.Error()
		ctx.JSON(http.StatusInternalServerError, rsp)
	}
}
