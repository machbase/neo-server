package httpd

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/machbase/neo-server/api/schedule"
)

func (svr *httpd) handleTimer(ctx *gin.Context) {
	tick := time.Now()
	rsp := gin.H{"success": false, "reason": "not specified"}

	name := ctx.Param("name")
	getRsp, err := svr.schedMgmtImpl.GetSchedule(ctx, &schedule.GetScheduleRequest{
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

func (svr *httpd) handleTimers(ctx *gin.Context) {
	tick := time.Now()
	rsp := gin.H{"success": false, "reason": "not specified"}

	listRsp, err := svr.schedMgmtImpl.ListSchedule(ctx, &schedule.ListScheduleRequest{})
	if err != nil {
		rsp["reason"] = err.Error()
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}
	if !listRsp.Success {
		rsp["reason"] = listRsp.Reason
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}

	list := []*schedule.Schedule{}
	for _, c := range listRsp.Schedules {
		typ := strings.ToUpper(c.Type)
		if typ != "TIMER" {
			continue
		}
		list = append(list, c)
	}

	rsp["success"] = true
	rsp["reason"] = "success"
	rsp["data"] = list
	rsp["elapse"] = time.Since(tick).String()
	ctx.JSON(http.StatusOK, rsp)
}

func (svr *httpd) handleTimersAdd(ctx *gin.Context) {
	tick := time.Now()
	rsp := gin.H{"success": false, "reason": "not specified"}
	req := struct {
		Name      string `json:"name"`
		AutoStart bool   `json:"autoStart"`
		Schedule  string `json:"schedule"`
		Path      string `json:"path"`
	}{}

	err := ctx.ShouldBind(&req)
	if err != nil {
		rsp["reason"] = err.Error()
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}

	getRsp, err := svr.schedMgmtImpl.GetSchedule(ctx, &schedule.GetScheduleRequest{Name: req.Name})
	if err != nil {
		rsp["reason"] = err.Error()
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}
	if getRsp.Success {
		rsp["reason"] = fmt.Sprintf("'%s' is duplicate name.", req.Name)
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}

	addRsp, err := svr.schedMgmtImpl.AddSchedule(ctx, &schedule.AddScheduleRequest{
		Name:      strings.ToLower(req.Name),
		Type:      "timer",
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
	if !addRsp.Success {
		rsp["reason"] = addRsp.Reason
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}

	rsp["success"] = true
	rsp["reason"] = "success"
	rsp["elapse"] = time.Since(tick).String()
	ctx.JSON(http.StatusOK, rsp)
}

func (svr *httpd) handleTimersState(ctx *gin.Context) {
	tick := time.Now()
	rsp := gin.H{"success": false, "reason": "not specified"}

	name := ctx.Param("name")
	if name == "" {
		rsp["reason"] = "no name specified"
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}

	req := struct {
		State string `json:"state"`
	}{}
	err := ctx.ShouldBind(&req)
	if err != nil {
		rsp["reason"] = err.Error()
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}

	switch strings.ToUpper(req.State) {
	case "START":
		startRsp, err := svr.schedMgmtImpl.StartSchedule(ctx, &schedule.StartScheduleRequest{
			Name: name,
		})
		if err != nil {
			rsp["reason"] = err.Error()
			rsp["elapse"] = time.Since(tick).String()
			ctx.JSON(http.StatusInternalServerError, rsp)
			return
		}
		if !startRsp.Success {
			rsp["reason"] = startRsp.Reason
			rsp["elapse"] = time.Since(tick).String()
			ctx.JSON(http.StatusInternalServerError, rsp)
			return
		}
	case "STOP":
		stopRsp, err := svr.schedMgmtImpl.StopSchedule(ctx, &schedule.StopScheduleRequest{
			Name: name,
		})
		if err != nil {
			rsp["reason"] = err.Error()
			rsp["elapse"] = time.Since(tick).String()
			ctx.JSON(http.StatusInternalServerError, rsp)
			return
		}
		if !stopRsp.Success {
			rsp["reason"] = stopRsp.Reason
			rsp["elapse"] = time.Since(tick).String()
			ctx.JSON(http.StatusInternalServerError, rsp)
			return
		}
	default:
		rsp["reason"] = fmt.Sprintf("no state specified: '%s'", req.State)
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}

	rsp["success"] = true
	rsp["reason"] = "success"
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

	getRsp, err := svr.schedMgmtImpl.GetSchedule(ctx, &schedule.GetScheduleRequest{
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

	updateRsp, err := svr.schedMgmtImpl.UpdateSchedule(ctx, &schedule.UpdateScheduleRequest{
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

func (svr *httpd) handleTimersDel(ctx *gin.Context) {
	tick := time.Now()
	rsp := gin.H{"success": false, "reason": "not specified"}

	name := ctx.Param("name")
	if name == "" {
		rsp["reason"] = "no name specified"
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}

	getRsp, err := svr.schedMgmtImpl.GetSchedule(ctx, &schedule.GetScheduleRequest{
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
	if getRsp.Schedule.State == "RUNNING" {
		stopRsp, err := svr.schedMgmtImpl.StopSchedule(ctx, &schedule.StopScheduleRequest{
			Name: name,
		})
		if err != nil {
			rsp["reason"] = err.Error()
			rsp["elapse"] = time.Since(tick).String()
			ctx.JSON(http.StatusInternalServerError, rsp)
			return
		}
		if !stopRsp.Success {
			rsp["reason"] = stopRsp.Reason
			rsp["elapse"] = time.Since(tick).String()
			ctx.JSON(http.StatusInternalServerError, rsp)
			return
		}
	}

	deleteRsp, err := svr.schedMgmtImpl.DelSchedule(ctx, &schedule.DelScheduleRequest{
		Name: name,
	})
	if err != nil {
		rsp["reason"] = err.Error()
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}
	if !deleteRsp.Success {
		rsp["reason"] = deleteRsp.Reason
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}

	rsp["success"] = true
	rsp["reason"] = "success"
	rsp["elapse"] = time.Since(tick).String()
	ctx.JSON(http.StatusOK, rsp)
}
