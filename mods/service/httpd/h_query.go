package httpd

import (
	"compress/gzip"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/machbase/neo-server/mods/codec"
	"github.com/machbase/neo-server/mods/codec/opts"
	"github.com/machbase/neo-server/mods/do"
	"github.com/machbase/neo-server/mods/service/msg"
	"github.com/machbase/neo-server/mods/stream"
	"github.com/machbase/neo-server/mods/stream/spec"
	spi "github.com/machbase/neo-spi"
)

// Execute machbase SQL query
//
// @Summary     Execute query
// @Description execute query
// @Param       q           query   string true "sql query text" default(select * from example limit 3)
// @Success     200  {object}  msg.QueryResponse
// @Failure     400  {object}  msg.QueryResponse
// @Failure     500  {object}  msg.QueryResponse
// @Router      /db/query [get]
func (svr *httpd) handleQuery(ctx *gin.Context) {
	rsp := &msg.QueryResponse{Success: false, Reason: "not specified"}
	tick := time.Now()

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
				rsp.Elapse = time.Since(tick).String()
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
			rsp.Elapse = time.Since(tick).String()
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
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}

	var timeLocation = strTimeLocation(req.TimeLocation, time.UTC)

	var output spec.OutputStream
	switch req.Compress {
	case "gzip":
		output = &stream.WriterOutputStream{Writer: gzip.NewWriter(ctx.Writer)}
	default:
		req.Compress = ""
		output = &stream.WriterOutputStream{Writer: ctx.Writer}
	}

	encoder := codec.NewEncoder(req.Format,
		opts.OutputStream(output),
		opts.Timeformat(req.Timeformat),
		opts.Precision(req.Precision),
		opts.Rownum(req.Rownum),
		opts.Heading(req.Heading),
		opts.TimeLocation(timeLocation),
		opts.Delimiter(","),
		opts.BoxStyle("default"),
		opts.BoxSeparateColumns(true),
		opts.BoxDrawBorder(true),
	)

	conn, err := svr.getTrustConnection(ctx)
	if err != nil {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	defer conn.Close()

	queryCtx := &do.QueryContext{
		Conn: conn,
		Ctx:  ctx,
		OnFetchStart: func(cols spi.Columns) {
			ctx.Writer.Header().Set("Content-Type", encoder.ContentType())
			if len(req.Compress) > 0 {
				ctx.Writer.Header().Set("Content-Encoding", req.Compress)
			}
			codec.SetEncoderColumns(encoder, cols)
			encoder.Open()
		},
		OnFetch: func(nrow int64, values []any) bool {
			err := encoder.AddRow(values)
			if err != nil {
				// report error to client?
				svr.log.Error("render", err.Error())
				return false
			}
			return true
		},
		OnFetchEnd: func() {
			encoder.Close()
		},
		OnExecuted: func(userMessage string, rowsAffected int64) {
			rsp.Success, rsp.Reason = true, userMessage
			rsp.Elapse = time.Since(tick).String()
			ctx.JSON(http.StatusOK, rsp)
		},
	}
	if _, err := do.Query(queryCtx, req.SqlText); err != nil {
		svr.log.Error("query fail", err.Error())
		rsp.Reason = err.Error()
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
	}
}
