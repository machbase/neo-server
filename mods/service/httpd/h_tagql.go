package httpd

import (
	"compress/gzip"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/machbase/neo-server/mods/codec"
	"github.com/machbase/neo-server/mods/service/msg"
	"github.com/machbase/neo-server/mods/stream"
	"github.com/machbase/neo-server/mods/stream/spec"
	"github.com/machbase/neo-server/mods/tagql"
)

// @Router      /api/tagql/:table/:tag#query [get]
func (svr *httpd) handleTagQL(ctx *gin.Context) {
	rsp := &msg.QueryResponse{Success: false, Reason: "not specified"}

	tick := time.Now()
	table := ctx.Param("table")
	tag := ctx.Param("tag")
	ql := ctx.Request.URL.RawQuery

	qlCtx := &tagql.Context{
		BaseTimeColumn: "time",
		MaxLimit:       100000,
		DefaultRange:   10 * time.Second,
		MaxRange:       1 * time.Minute,
	}
	composed := fmt.Sprintf("%s/%s", table, tag)
	if ql != "" {
		composed = fmt.Sprintf("%s?%s", composed, ql)
	}
	tql, err := tagql.ParseTagQLContext(qlCtx, composed)
	if err != nil {
		rsp.Reason = err.Error()
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}

	req := &msg.QueryRequest{Precision: -1}
	req.SqlText = tql.ToSQL()
	req.Timeformat = strString(ctx.Query("timeformat"), "ns")
	req.TimeLocation = strString(ctx.Query("tz"), "UTC")
	req.Format = strString(ctx.Query("format"), "json")
	req.Compress = ctx.Query("compress")
	req.Rownum = strBool(ctx.Query("rownum"), false)
	req.Heading = strBool(ctx.Query("heading"), true)
	req.Precision = strInt(ctx.Query("precision"), -1)

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
		codec.OutputStream(output),
		codec.TimeFormat(req.Timeformat),
		codec.Precision(req.Precision),
		codec.Rownum(req.Rownum),
		codec.Heading(req.Heading),
		codec.TimeLocation(timeLocation),
		codec.Title("TagQL chart"),
		codec.Subtitle(composed),
		codec.Delimiter(","),
		codec.BoxStyle("default"),
		codec.BoxSeparateColumns(true),
		codec.BoxDrawBorder(true),
	)

	ctx.Writer.Header().Set("Content-Type", encoder.ContentType())
	if len(req.Compress) > 0 {
		ctx.Writer.Header().Set("Content-Encoding", req.Compress)
	}

	if err := tql.Execute(ctx, svr.db, encoder); err != nil {
		svr.log.Error("query fail", err.Error())
		rsp.Reason = err.Error()
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
	}
}
