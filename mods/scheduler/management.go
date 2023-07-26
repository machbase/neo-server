package scheduler

import (
	"context"
	"time"

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
			Name: define.Name,
			Type: string(define.Type),
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
			Name: define.Name,
			Type: string(define.Type),
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

	if len(req.Name) > 40 {
		rsp.Reason = "name is too long, should be shorter than 40 characters"
		return rsp, nil
	} else {
		def.Name = req.Name
	}

	if err := Register(def); err != nil {
		rsp.Reason = err.Error()
		return rsp, nil
	}

	if err := s.saveConfig(def); err != nil {
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
