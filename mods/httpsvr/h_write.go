package httpsvr

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/machbase/dbms-mach-go/server/msg"
)

func (svr *Server) handleWrite(ctx *gin.Context) {
	tick := time.Now()
	req := &msg.WriteRequest{}
	rsp := &msg.WriteResponse{Reason: "not specified"}

	err := ctx.Bind(req)
	if err != nil {
		rsp.Reason = err.Error()
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}

	// post body로 전달되는 table name이 우선한다.
	if len(req.Table) == 0 {
		// table명이 path param으로 입력될 수도 있고
		req.Table = ctx.Param("table")
	}

	if len(req.Table) == 0 {
		rsp.Reason = "table is not specified"
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}

	msg.Write(svr.db, req, rsp)
	rsp.Elapse = time.Since(tick).String()

	if rsp.Success {
		ctx.JSON(http.StatusOK, rsp)
	} else {
		ctx.JSON(http.StatusInternalServerError, rsp)
	}
}
