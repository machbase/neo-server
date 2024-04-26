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

// func validSchedule(sched string) error {
// 	fields := strings.Fields(strings.TrimSpace(sched))
// 	fieldsLen := len(fields)
// 	schedType := ""

// 	switch fieldsLen {
// 	case 1, 2:
// 		c := sched[0]
// 		if c == '@' {
// 			schedType = "predef"
// 		}
// 	case 6:
// 		c := sched[0]
// 		if unicode.IsDigit(rune(c)) {
// 			n, _ := strconv.Atoi(string(c))
// 			if n > 59 {
// 				return fmt.Errorf("")
// 			}
// 			schedType = "cron"
// 			break
// 		}
// 		if strings.ContainsAny(string(c), "*/,-") {

// 		}
// 	default:
// 		return fmt.Errorf("")
// 	}

// 	switch schedType {
// 	case "predef":
// 		entry := []string{"@yearly", "@annually", "@monthly", "@weekly", "@daily", "@midnight", "@hourly"}
// 		if !slices.Contains(entry, fields[0]) {
// 			return fmt.Errorf("")
// 		}
// 		if fieldsLen == 1 { // @daily
// 			return nil
// 		}

// 		if !strings.ContainsAny(fields[1], "msh") {
// 			return fmt.Errorf("")
// 		}
// 		for _, c := range fields[1] {

// 		}
// 	case "cron":
// 		if fieldsLen != 6 {
// 			return fmt.Errorf("")
// 		}

// 		for i, field := range fields {
// 			switch i {
// 			case 0, 1:
// 				for _, c := range field {
// 					if unicode.IsDigit(c) {

// 					}
// 				}
// 				n, err := strconv.Atoi(field)
// 				if err != nil {
// 					return fmt.Errorf("")
// 				}

// 				if n > 60 {
// 					return fmt.Errorf("")
// 				}

// 			}

// 		}

// 	}

// 	return nil
// }

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
			if state == "STOP" {
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
