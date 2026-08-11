package server

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/machbase/neo-server/v8/mods/scheduler"
)

func (svr *httpd) handleTimer(ctx *gin.Context) {
	tick := time.Now()
	rsp := gin.H{"success": false, "reason": "not specified"}

	name := ctx.Param("name")
	getRsp, err := svr.authServer.schedSvc.GetSchedule(ctx, &scheduler.GetScheduleRequest{
		Name: name,
	})
	if err != nil {
		rsp["reason"] = err.Error()
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}
	if !getRsp.Success {
		rsp["reason"] = getRsp.Reason
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}

	rsp["success"] = true
	rsp["reason"] = "success"
	rsp["data"] = getRsp.Schedule
	rsp["elapse"] = time.Since(tick).String()
	ctx.JSON(http.StatusOK, rsp)
}

func (svr *httpd) handleTimersUpdate(ctx *gin.Context) {
	tick := time.Now()
	rsp := gin.H{"success": false, "reason": "not specified"}
	req := struct {
		AutoStart bool   `json:"autoStart"`
		Schedule  string `json:"schedule"`
		Path      string `json:"path"`
	}{}

	name := ctx.Param("name")
	if name == "" {
		rsp["reason"] = "no name specified"
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}

	err := ctx.ShouldBind(&req)
	if err != nil {
		rsp["reason"] = err.Error()
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}

	getRsp, err := svr.authServer.schedSvc.GetSchedule(ctx, &scheduler.GetScheduleRequest{
		Name: name,
	})
	if err != nil {
		rsp["reason"] = err.Error()
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}
	if !getRsp.Success {
		rsp["reason"] = getRsp.Reason
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}

	updateRsp, err := svr.authServer.schedSvc.UpdateSchedule(ctx, &scheduler.UpdateScheduleRequest{
		Name:      name,
		AutoStart: req.AutoStart,
		Schedule:  req.Schedule,
		Task:      req.Path,
	})
	if err != nil {
		rsp["reason"] = err.Error()
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}
	if !updateRsp.Success {
		rsp["reason"] = updateRsp.Reason
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}

	rsp["success"] = true
	rsp["reason"] = "success"
	rsp["elapse"] = time.Since(tick).String()
	ctx.JSON(http.StatusOK, rsp)
}

func (svr *httpd) handleSubscriber(ctx *gin.Context) {
	tick := time.Now()
	rsp := gin.H{"success": false, "reason": "not specified"}

	name := ctx.Param("name")
	getRsp, err := svr.authServer.schedSvc.GetSchedule(ctx, &scheduler.GetScheduleRequest{Name: name})
	if err != nil {
		rsp["reason"] = err.Error()
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}
	if !getRsp.Success {
		rsp["reason"] = getRsp.Reason
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}

	rsp["success"] = true
	rsp["reason"] = "success"
	rsp["data"] = getRsp.Schedule
	rsp["elapse"] = time.Since(tick).String()
	ctx.JSON(http.StatusOK, rsp)
}
