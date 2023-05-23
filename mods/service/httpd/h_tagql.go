package httpd

import (
	"compress/gzip"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/machbase/neo-server/mods/codec"
	"github.com/machbase/neo-server/mods/do"
	"github.com/machbase/neo-server/mods/service/msg"
	"github.com/machbase/neo-server/mods/stream"
	"github.com/machbase/neo-server/mods/tagql"
	spi "github.com/machbase/neo-spi"
)

// @Router      /api/tagql/:table/:tag#query [get]
func (svr *httpd) handleTagQL(ctx *gin.Context) {
	tick := time.Now()
	table := ctx.Param("table")
	tag := ctx.Param("tag")
	ql := ctx.Request.URL.Fragment
	rsp := &msg.QueryResponse{Success: false, Reason: "not specified"}

	if len(ql) > 0 {
		ql = "#" + ql
	}

	qlCtx := &tagql.Context{
		BaseTimeColumn: "time",
		MaxLimit:       100000,
		DefaultRange:   10 * time.Second,
		MaxRange:       1 * time.Minute,
	}
	composed := fmt.Sprintf("%s/%s%s?%s", table, tag, ql, ctx.Request.URL.RawQuery)
	parsed, err := tagql.ParseTagQLContext(qlCtx, composed)
	svr.log.Info("composed", composed, "parsed", parsed.ToSQL())
	if err != nil {
		rsp.Reason = err.Error()
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}

	req := &msg.QueryRequest{Precision: -1}
	req.SqlText = parsed.ToSQL()
	req.Timeformat = strString(ctx.Query("timeformat"), "ns")
	req.TimeLocation = strString(ctx.Query("tz"), "UTC")
	req.Format = strString(ctx.Query("format"), "json")
	req.Compress = ctx.Query("compress")
	req.Rownum = strBool(ctx.Query("rownum"), false)
	req.Heading = strBool(ctx.Query("heading"), true)
	req.Precision = strInt(ctx.Query("precision"), -1)

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
