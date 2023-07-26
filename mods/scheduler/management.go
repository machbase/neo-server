package scheduler

import (
	"context"
	"time"

	"github.com/gofrs/uuid"
	schedrpc "github.com/machbase/neo-grpc/schedule"
)

func (s *svr) ListSchedule(context.Context, *schedrpc.ListScheduleRequest) (*schedrpc.ListScheduleResponse, error) {
	tick := time.Now()
	rsp := &schedrpc.ListScheduleResponse{}
	defer func() {
		rsp.Elapse = time.Since(tick).String()
	}()
	err := s.iterateConfigs(func(define *Define) bool {
		rsp.Schedules = append(rsp.Schedules, &schedrpc.Schedule{
			Id:        define.Id,
			Name:      define.Name,
			Type:      define.Type,
			AutoStart: define.AutoStart,
			State:     UNKNOWN.String(),
			Task:      define.Task,
			Schedule:  define.Schedule,
			Bridge:    define.Bridge,
			Topic:     define.Topic,
			QoS:       int32(define.QoS),
		})
		return true
	})
	if err != nil {
		rsp.Reason = err.Error()
		return rsp, nil
	}
	rsp.Success, rsp.Reason = true, "success"
	return rsp, nil
}

func (s *svr) GetSchedule(ctx context.Context, req *schedrpc.GetScheduleRequest) (*schedrpc.GetScheduleResponse, error) {
	tick := time.Now()
	rsp := &schedrpc.GetScheduleResponse{}
	defer func() {
		rsp.Elapse = time.Since(tick).String()
	}()
	if define, err := s.loadConfig(req.Id); err != nil {
		rsp.Reason = err.Error()
	} else {
		rsp.Schedule = &schedrpc.Schedule{
			Id:        define.Id,
			Name:      define.Name,
			Type:      define.Type,
			AutoStart: define.AutoStart,
			State:     UNKNOWN.String(),
			Task:      define.Task,
			Schedule:  define.Schedule,
			Bridge:    define.Bridge,
			Topic:     define.Topic,
			QoS:       int32(define.QoS),
		}
		rsp.Success, rsp.Reason = true, "success"
	}
	return rsp, nil
}

func (s *svr) AddSchedule(ctx context.Context, req *schedrpc.AddScheduleRequest) (*schedrpc.AddScheduleResponse, error) {
	tick := time.Now()
	rsp := &schedrpc.AddScheduleResponse{Reason: "not specified"}
	defer func() {
		rsp.Elapse = time.Since(tick).String()
	}()

	def := &Define{}
	if uid, err := uuid.DefaultGenerator.NewV4(); err != nil {
		return nil, err
	} else {
		def.Id = uid.String()
	}
	if len(req.Name) > 40 {
		rsp.Reason = "name is too long, should be shorter than 40 characters"
		return rsp, nil
	} else {
		def.Name = req.Name
	}

	def.AutoStart = req.AutoStart
	def.Bridge = req.Bridge
	def.QoS = int(req.QoS)
	def.Schedule = req.Schedule
	def.Task = req.Task
	def.Topic = req.Topic
	def.Type = req.Type

	if err := s.saveConfig(def); err != nil {
		rsp.Reason = err.Error()
		return rsp, nil
	}

	if err := Register(def); err != nil {
		rsp.Reason = err.Error()
		return rsp, nil
	}

	rsp.Success, rsp.Reason = true, "success"
	return rsp, nil
}

func (s *svr) DelSchedule(ctx context.Context, req *schedrpc.DelScheduleRequest) (*schedrpc.DelScheduleResponse, error) {
	tick := time.Now()
	rsp := &schedrpc.DelScheduleResponse{}
	defer func() {
		rsp.Elapse = time.Since(tick).String()
	}()

	if err := s.removeConfig(req.Id); err != nil {
		rsp.Reason = err.Error()
		return rsp, nil
	}

	Unregister(req.Id)

	rsp.Success, rsp.Reason = true, "success"
	return rsp, nil

}
