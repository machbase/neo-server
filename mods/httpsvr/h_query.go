package httpsvr

import (
	"compress/gzip"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/machbase/neo-server/mods/msg"
	"github.com/machbase/neo-shell/renderer/boxrenderer"
	"github.com/machbase/neo-shell/renderer/csvrenderer"
	"github.com/machbase/neo-shell/renderer/jsonrenderer"
	"github.com/machbase/neo-shell/sink"
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
			if err = ctx.Bind(req); err != nil {
				rsp.Reason = err.Error()
				ctx.JSON(http.StatusBadRequest, rsp)
				return
			}
		} else if contentType == "application/x-www-form-urlencoded" {
			req.SqlText = ctx.PostForm("q")
			req.Timeformat = ctx.PostForm("timeformat")
			req.TimeLocation = ctx.PostForm("tz")
			req.Format = ctx.PostForm("format")
			req.Compress = ctx.PostForm("compress")
			req.Rownum = strings.ToLower(ctx.PostForm("rownum")) == "true"
			req.Heading = strings.ToLower(ctx.PostForm("heading")) == "true"
			req.Precision, _ = strconv.Atoi(ctx.PostForm("precision"))
		} else {
			rsp.Reason = fmt.Sprintf("unsupported content-type: %s", contentType)
			ctx.JSON(http.StatusBadRequest, rsp)
			return
		}
	} else if ctx.Request.Method == http.MethodGet {
		req.SqlText = ctx.Query("q")
		req.Timeformat = ctx.Query("timeformat")
		req.TimeLocation = ctx.Query("tz")
		req.Format = ctx.Query("format")
		req.Compress = ctx.Query("compress")
		req.Rownum = strings.ToLower(ctx.Query("rownum")) == "true"
		req.Heading = strings.ToLower(ctx.Query("heading")) == "true"
		req.Precision, _ = strconv.Atoi(ctx.Query("precision"))
	}

	if len(req.SqlText) == 0 {
		rsp.Reason = "empty sql"
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}

	if len(req.Timeformat) == 0 {
		req.Timeformat = "ns"
	}

	var timeLocation *time.Location
	if tz, err := time.LoadLocation(req.TimeLocation); err == nil {
		timeLocation = tz
	} else {
		timeLocation = time.UTC
	}

	var rspSink spi.Sink
	switch req.Compress {
	case "gzip":
		rspSink = &sink.WriterSink{Writer: gzip.NewWriter(ctx.Writer)}
	default:
		req.Compress = ""
		rspSink = &sink.WriterSink{Writer: ctx.Writer}
	}

	var renderer spi.RowsRenderer
	var rendererCtx = &spi.RowsRendererContext{
		Sink:         rspSink,
		TimeLocation: timeLocation,
		TimeFormat:   spi.GetTimeformat(req.Timeformat),
		Precision:    req.Precision,
		Rownum:       req.Rownum,
		Heading:      req.Heading,
	}
	var contentType string
	switch req.Format {
	case "box":
		contentType = "plain/text"
		renderer = boxrenderer.NewRowsRenderer("default", true, true)
	case "csv":
		contentType = "text/csv"
		renderer = csvrenderer.NewRowsRenderer(",")
	default: // "json":
		contentType = "application/json"
		renderer = jsonrenderer.NewRowsRenderer()
	}

	queryCtx := &spi.QueryContext{
		DB: svr.db,
		OnFetchStart: func(cols spi.Columns) {
			ctx.Writer.Header().Set("Content-Type", contentType)
			if len(req.Compress) > 0 {
				ctx.Writer.Header().Set("Content-Encoding", req.Compress)
			}
			rendererCtx.ColumnNames = cols.NamesWithTimeLocation(timeLocation)
			rendererCtx.ColumnTypes = cols.Types()
			renderer.OpenRender(rendererCtx)
		},
		OnFetch: func(nrow int64, values []any) bool {
			err := renderer.RenderRow(values)
			if err != nil {
				// report error to client?
				svr.log.Errorf("render", err.Error())
				return false
			}
			return true
		},
		OnFetchEnd: func() {
			renderer.CloseRender()
		},
	}
	if err := spi.DoQuery(queryCtx, req.SqlText); err != nil {
		rsp.Reason = err.Error()
		ctx.JSON(http.StatusInternalServerError, rsp)
	}
}
