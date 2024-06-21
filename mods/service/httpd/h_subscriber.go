package httpd

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/machbase/neo-server/api/bridge"
	"github.com/machbase/neo-server/api/schedule"
)

func (svr *httpd) handleSubscriber(ctx *gin.Context) {
	tick := time.Now()
	rsp := gin.H{"success": false, "reason": "not specified"}

	name := ctx.Param("name")
	getRsp, err := svr.schedMgmtImpl.GetSchedule(ctx, &schedule.GetScheduleRequest{Name: name})
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

func (svr *httpd) handleSubscribers(ctx *gin.Context) {
	tick := time.Now()
	rsp := gin.H{"success": false, "reason": "not specified"}

	schedRsp, err := svr.schedMgmtImpl.ListSchedule(ctx, &schedule.ListScheduleRequest{})
	if err != nil {
		rsp["reason"] = err.Error()
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}
	if !schedRsp.Success {
		rsp["reason"] = schedRsp.Reason
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}

	lst := []*schedule.Schedule{}
	for _, c := range schedRsp.Schedules {
		typ := strings.ToUpper(c.Type)
		if typ != "SUBSCRIBER" {
			continue
		}
		lst = append(lst, c)
	}

	if len(lst) > 0 {
		sort.Slice(lst, func(i, j int) bool { return lst[i].Name < lst[j].Name })
	}

	rsp["success"] = true
	rsp["reason"] = "success"
	rsp["data"] = lst
	rsp["elapse"] = time.Since(tick).String()
	ctx.JSON(http.StatusOK, rsp)
}

func (svr *httpd) handleSubscribersAdd(ctx *gin.Context) {
	tick := time.Now()
	rsp := gin.H{"success": false, "reason": "not specified"}

	req := struct {
		Name      string `json:"name"`
		AutoStart bool   `json:"autoStart"`
		Bridge    string `json:"bridge"`
		Topic     string `json:"topic"`
		Task      string `json:"task"`
		QoS       int    `json:"QoS"`
		Queue     string `json:"quque"`
		Stream    string `json:"stream"`
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
		rsp["reason"] = "duplicate name"
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}

	bridgeRsp, err := svr.bridgeMgmtImpl.GetBridge(ctx, &bridge.GetBridgeRequest{Name: req.Bridge})
	if err != nil {
		rsp["reason"] = err.Error()
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}
	if !bridgeRsp.Success {
		rsp["reason"] = bridgeRsp.Reason
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}
	bridgeType := bridgeRsp.Bridge.Type

	addSchedReq := &schedule.AddScheduleRequest{
		Name:      strings.ToLower(req.Name),
		Type:      "subscriber",
		AutoStart: req.AutoStart,
		Bridge:    req.Bridge,
		Task:      req.Task,
	}

	switch bridgeType {
	case "mqtt":
		addSchedReq.Opt = &schedule.AddScheduleRequest_Mqtt{
			Mqtt: &schedule.MqttOption{
				Topic: req.Topic,
				QoS:   int32(req.QoS),
			},
		}
	case "nats":
		addSchedReq.Opt = &schedule.AddScheduleRequest_Nats{
			Nats: &schedule.NatsOption{
				Subject:    req.Topic,
				QueueName:  req.Queue,
				StreamName: req.Stream,
			},
		}
	default:
		rsp["reason"] = fmt.Sprintf("unknown birdge type %q", bridgeType)
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}

	schedRsp, err := svr.schedMgmtImpl.AddSchedule(ctx, addSchedReq)
	if err != nil {
		rsp["reason"] = err.Error()
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}
	if !schedRsp.Success {
		rsp["reason"] = schedRsp.Reason
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}

	rsp["success"] = true
	rsp["reason"] = "success"
	rsp["elapse"] = time.Since(tick).String()
	ctx.JSON(http.StatusOK, rsp)
}

func (svr *httpd) handleSubscribersDel(ctx *gin.Context) {
	tick := time.Now()
	rsp := gin.H{"success": false, "reason": "not specified"}
	name := ctx.Param("name")

	schedRsp, err := svr.schedMgmtImpl.DelSchedule(ctx, &schedule.DelScheduleRequest{
		Name: name,
	})
	if err != nil {
		rsp["reason"] = err.Error()
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}
	if !schedRsp.Success {
		rsp["reason"] = schedRsp.Reason
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}

	rsp["success"] = true
	rsp["reason"] = "success"
	rsp["elapse"] = time.Since(tick).String()
	ctx.JSON(http.StatusOK, rsp)
}

func (svr *httpd) handleSubscribersState(ctx *gin.Context) {
	tick := time.Now()
	rsp := gin.H{"success": false, "reason": "not specified"}
	name := ctx.Param("name")

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
		schedRsp, err := svr.schedMgmtImpl.StartSchedule(ctx, &schedule.StartScheduleRequest{
			Name: name,
		})
		if err != nil {
			rsp["reason"] = err.Error()
			rsp["elapse"] = time.Since(tick).String()
			ctx.JSON(http.StatusInternalServerError, rsp)
			return
		}
		if !schedRsp.Success {
			rsp["reason"] = schedRsp.Reason
			rsp["elapse"] = time.Since(tick).String()
			ctx.JSON(http.StatusInternalServerError, rsp)
			return
		}
	case "STOP":
		schedRsp, err := svr.schedMgmtImpl.StopSchedule(ctx, &schedule.StopScheduleRequest{
			Name: name,
		})
		if err != nil {
			rsp["reason"] = err.Error()
			rsp["elapse"] = time.Since(tick).String()
			ctx.JSON(http.StatusInternalServerError, rsp)
			return
		}
		if !schedRsp.Success {
			rsp["reason"] = schedRsp.Reason
			rsp["elapse"] = time.Since(tick).String()
			ctx.JSON(http.StatusInternalServerError, rsp)
			return
		}
	default:
		rsp["reason"] = "invalid state"
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}

	rsp["success"] = true
	rsp["reason"] = "success"
	rsp["elapse"] = time.Since(tick).String()
	ctx.JSON(http.StatusOK, rsp)
}
