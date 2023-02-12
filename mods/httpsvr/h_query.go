package httpsvr

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/machbase/neo-server/mods/msg"
)

func (svr *Server) handleQuery(ctx *gin.Context) {
	req := &msg.QueryRequest{}
	rsp := &msg.QueryResponse{Success: false, Reason: "not specified"}
	tick := time.Now()

	var err error
	var timeformat string
	var format string
	var compress string
	if ctx.Request.Method == http.MethodPost {
		contentType := ctx.ContentType()
		if contentType == "application/json" {
			if err = ctx.Bind(req); err != nil {
				rsp.Reason = err.Error()
				rsp.Elapse = time.Since(tick).String()
				ctx.JSON(http.StatusBadRequest, rsp)
				return
			}
			timeformat = req.Timeformat
			format = req.Format
			compress = req.Compress
		} else if contentType == "application/x-www-form-urlencoded" {
			req.SqlText = ctx.PostForm("q")
			timeformat = ctx.PostForm("timeformat")
			format = ctx.PostForm("format")
			compress = ctx.PostForm("compress")
		} else {
			rsp.Reason = fmt.Sprintf("unsupported content-type: %s", contentType)
			rsp.Elapse = time.Since(tick).String()
			ctx.JSON(http.StatusBadRequest, rsp)
			return
		}
	} else if ctx.Request.Method == http.MethodGet {
		req.SqlText = ctx.Query("q")
		timeformat = ctx.Query("timeformat")
		format = ctx.Query("format")
		compress = ctx.Query("compress")
	}

	if len(req.SqlText) == 0 {
		rsp.Reason = "empty sql"
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}

	if len(timeformat) == 0 {
		timeformat = "ns"
	}

	switch format {
	case "csv":
	default:
		format = "json"
	}
	switch compress {
	case "gzip":
	default:
		compress = ""
	}

	req.Timeformat = timeformat
	req.Format = format
	req.Compress = compress

	// TODO
	// queryCtx := &spi.QueryContext{
	// 	DB: svr.db,
	// }
	// spi.DoQuery()
	msg.Query(svr.db, req, rsp)
	rsp.Elapse = time.Since(tick).String()

	if !rsp.Success {
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}

	if rsp.ContentType == "application/json" {
		ctx.JSON(http.StatusOK, rsp)
	} else {
		if len(rsp.ContentEncoding) > 0 {
			ctx.Header("Content-Encoding", rsp.ContentEncoding)
		}
		ctx.Data(http.StatusOK, rsp.ContentType, rsp.Content)
	}
}
