package httpsvr

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/machbase/dbms-mach-go/server/msg"
)

func (svr *Server) handleQuery(ctx *gin.Context) {
	req := &msg.QueryRequest{}
	rsp := &msg.QueryResponse{Success: false, Reason: "not specified"}
	tick := time.Now()

	var err error
	var timeformat string
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
		} else if contentType == "application/x-www-form-urlencoded" {
			req.SqlText = ctx.PostForm("q")
			timeformat = ctx.PostForm("timeformat")
		} else {
			rsp.Reason = fmt.Sprintf("unsupported content-type: %s", contentType)
			rsp.Elapse = time.Since(tick).String()
			ctx.JSON(http.StatusBadRequest, rsp)
			return
		}
	} else if ctx.Request.Method == http.MethodGet {
		req.SqlText = ctx.Query("q")
		timeformat = ctx.PostForm("timeformat")
	}
	if len(req.SqlText) == 0 {
		rsp.Reason = "empty sql"
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}

	if len(timeformat) == 0 {
		timeformat = "epoch"
	}

	req.Timeformat = timeformat

	msg.Query(svr.db, req, rsp)
	rsp.Elapse = time.Since(tick).String()

	if rsp.Success {
		ctx.JSON(http.StatusOK, rsp)
	} else {
		ctx.JSON(http.StatusInternalServerError, rsp)
	}
}
