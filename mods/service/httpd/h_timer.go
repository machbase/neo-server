package httpd

import (
	"fmt"
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
	req := struct {
		Name      string `json:"name"`
		AutoStart bool   `json:"autoStart"`
		Spec      string `json:"spec"`
		Path      string `json:"path"`
	}{}

	err := ctx.ShouldBind(&req)
	if err != nil {
		rsp["reason"] = err.Error()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}

	// validSchedule(req.Spec)

	// 중복 검사
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

	for _, c := range listRsp.Schedules {
		if c.Name == req.Name {
			rsp["reason"] = fmt.Sprintf("'%s' is duplicate name.", req.Name)
			ctx.JSON(http.StatusBadRequest, rsp)
			return
		}
	}

	addRsp, err := svr.schedMgmtImpl.AddSchedule(ctx, &schedule.AddScheduleRequest{
		Name:      strings.ToLower(req.Name),
		Type:      "timer",
		AutoStart: req.AutoStart,
		Schedule:  req.Spec,
		Task:      req.Path,
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

func (svr *httpd) handleStateTimer(ctx *gin.Context) {
	tick := time.Now()
	rsp := gin.H{"success": false, "reason": "not specified"}

	name := ctx.Param("name")
	if name == "" {
		rsp["reason"] = "no name specified"
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}

	req := struct {
		State string `json:"state"`
	}{}

	err := ctx.ShouldBind(&req)
	if err != nil {
		rsp["reason"] = err.Error()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}

	switch strings.ToLower(req.State) {
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

func (svr *httpd) handleUpdateTimer(ctx *gin.Context) {
	tick := time.Now()
	rsp := gin.H{"success": false, "reason": "not specified"}
	req := struct {
		AutoStart bool   `json:"autoStart"`
		Spec      string `json:"spec"`
		Path      string `json:"path"`
	}{}

	name := ctx.Param("name")
	if name == "" {
		rsp["reason"] = "no name specified"
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}

	err := ctx.ShouldBind(&req)
	if err != nil {
		rsp["reason"] = err.Error()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}

	// 타이머가 START 상태인지 확인
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
	for _, c := range listRsp.Schedules {
		if c.Name == name {
			state := strings.ToUpper(c.State)
			if state == "START" {
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
				break
			}
		}
	}

	// TIMER 업데이트
	updateRsp, err := svr.schedMgmtImpl.UpdateSchedule(ctx, &schedule.UpdateScheduleRequest{
		Name:      name,
		AutoStart: req.AutoStart,
		Schedule:  req.Spec,
		Task:      req.Path,
	})
	if err != nil {
		rsp["reason"] = err.Error()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}
	if !updateRsp.Success {
		rsp["reason"] = updateRsp.Reason
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}

	// 업데이트 된 TIMER 시작
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

	for _, c := range listRsp.Schedules {
		if c.Name == name {
			state := strings.ToUpper(c.State)
			if state == "START" {
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
				break
			}
		}
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
