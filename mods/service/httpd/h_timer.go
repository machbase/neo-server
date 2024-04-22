package httpd

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/machbase/neo-server/api/schedule"
)

func (svr *httpd) handleListTimers(ctx *gin.Context) {
	tick := time.Now()
	rsp := gin.H{"success": false, "reason": "not specified"}

	listRsp, err := svr.schedMgmtImpl.ListSchedule(ctx, &schedule.ListScheduleRequest{})
	if err != nil {
		rsp["reason"] = err.Error()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}
	if !listRsp.Success {
		rsp["reason"] = listRsp.Reason
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}

	lst := []*schedule.Schedule{}
	for _, c := range listRsp.Schedules {
		typ := strings.ToUpper(c.Type)
		if typ != "TIMER" {
			continue
		}
		lst = append(lst, c)
	}

	rsp["success"] = true
	rsp["reason"] = "success"
	rsp["list"] = lst
	rsp["elapse"] = time.Since(tick).String()
	ctx.JSON(http.StatusOK, rsp)
}

func (svr *httpd) handleAddTimer(ctx *gin.Context) {
	tick := time.Now()
	rsp := gin.H{"success": false, "reason": "not specified"}

	name := ctx.Param("name")
	if name == "" {
		rsp["reason"] = "no name specified"
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}

	req := struct {
		AutoStart bool   `json:"autoStart"`
		Spec      string `json:"spec"`
		TqlPath   string `json:"tqlPath"`
	}{}

	err := ctx.ShouldBind(&req)
	if err != nil {
		rsp["reason"] = err.Error()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}

	addRsp, err := svr.schedMgmtImpl.AddSchedule(ctx, &schedule.AddScheduleRequest{
		Name:      strings.ToLower(name),
		Type:      "timer",
		AutoStart: req.AutoStart,
		Schedule:  req.Spec,
		Task:      req.TqlPath,
	})
	if err != nil {
		rsp["reason"] = err.Error()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}
	if !addRsp.Success {
		rsp["reason"] = addRsp.Reason
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}

	rsp["success"] = true
	rsp["reason"] = "success"
	rsp["elapse"] = time.Since(tick).String()
	ctx.JSON(http.StatusOK, rsp)
}

func (svr *httpd) handleDoTimer(ctx *gin.Context) {
	tick := time.Now()
	rsp := gin.H{"success": false, "reason": "not specified"}

	name := ctx.Param("name")
	if name == "" {
		rsp["reason"] = "no name specified"
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}

	req := struct {
		Action string `json:"action"`
	}{}

	err := ctx.ShouldBind(&req)
	if err != nil {
		rsp["reason"] = "no action specified"
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}

	switch strings.ToLower(req.Action) {
	case "start":
		startRsp, err := svr.schedMgmtImpl.StartSchedule(ctx, &schedule.StartScheduleRequest{
			Name: name,
		})
		if err != nil {
			rsp["reason"] = err.Error()
			ctx.JSON(http.StatusInternalServerError, rsp)
			return
		}
		if !startRsp.Success {
			rsp["reason"] = startRsp.Reason
			ctx.JSON(http.StatusInternalServerError, rsp)
			return
		}
	case "stop":
		stopRsp, err := svr.schedMgmtImpl.StopSchedule(ctx, &schedule.StopScheduleRequest{
			Name: name,
		})
		if err != nil {
			rsp["reason"] = err.Error()
			ctx.JSON(http.StatusInternalServerError, rsp)
			return
		}
		if !stopRsp.Success {
			rsp["reason"] = stopRsp.Reason
			ctx.JSON(http.StatusInternalServerError, rsp)
			return
		}
	default:
		rsp["reason"] = "no action specified"
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}

	rsp["success"] = true
	rsp["reason"] = "success"
	rsp["elapse"] = time.Since(tick).String()
	ctx.JSON(http.StatusOK, rsp)
}

func (svr *httpd) handleDeleteTimer(ctx *gin.Context) {
	tick := time.Now()
	rsp := gin.H{"success": false, "reason": "not specified"}

	name := ctx.Param("name")
	if name == "" {
		rsp["reason"] = "no name specified"
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}

	deleteRsp, err := svr.schedMgmtImpl.DelSchedule(ctx, &schedule.DelScheduleRequest{
		Name: name,
	})
	if err != nil {
		rsp["reason"] = err.Error()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}
	if !deleteRsp.Success {
		rsp["reason"] = deleteRsp.Reason
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}

	rsp["success"] = true
	rsp["reason"] = "success"
	rsp["elapse"] = time.Since(tick).String()
	ctx.JSON(http.StatusOK, rsp)
}
