package scheduler

import (
	"context"
	"fmt"
	"time"

	schedrpc "github.com/machbase/neo-server/v8/api/schedule"
	"github.com/machbase/neo-server/v8/mods/model"
	"github.com/robfig/cron/v3"
)

func (s *svr) ListSchedule(context.Context, *schedrpc.ListScheduleRequest) (*schedrpc.ListScheduleResponse, error) {
	tick := time.Now()
	rsp := &schedrpc.ListScheduleResponse{}
	defer func() {
		rsp.Elapse = time.Since(tick).String()
	}()
	lst, err := s.models.LoadAllSchedules()
	if err != nil {
		rsp.Reason = err.Error()
		return rsp, nil
	}
	for _, define := range lst {
		sched := &schedrpc.Schedule{
			Name:      define.Name,
			Type:      define.Type.String(),
			AutoStart: define.AutoStart,
			State:     UNKNOWN.String(),
			Task:      define.Task,
			Schedule:  define.Schedule,
			Bridge:    define.Bridge,
			Topic:     define.Topic,
			QoS:       int32(define.QoS),
		}
		if ent := GetEntry(define.Name); ent != nil {
			if err := ent.Error(); err != nil {
				sched.State = fmt.Sprintf("%s, %s", ent.Status().String(), err.Error())
			} else {
				sched.State = ent.Status().String()
			}
		}
		rsp.Schedules = append(rsp.Schedules, sched)
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
	if define, err := s.models.LoadSchedule(req.Name); err != nil {
		rsp.Reason = err.Error()
	} else {
		rsp.Schedule = &schedrpc.Schedule{
			Name:      define.Name,
			Type:      define.Type.String(),
			AutoStart: define.AutoStart,
			State:     UNKNOWN.String(),
			Task:      define.Task,
			Schedule:  define.Schedule,
			Bridge:    define.Bridge,
			Topic:     define.Topic,
			QoS:       int32(define.QoS),
		}
		if ent := GetEntry(define.Name); ent != nil {
			rsp.Schedule.State = ent.Status().String()
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

	def := &model.ScheduleDefinition{}
	if len(req.Name) > 40 {
		rsp.Reason = "name is too long, should be shorter than 40 characters"
		return rsp, nil
	} else {
		def.Name = req.Name
	}

	def.AutoStart = req.AutoStart
	def.Bridge = req.Bridge
	def.Schedule = req.Schedule
	def.Task = req.Task
	def.Type = model.ParseScheduleType(req.Type)
	if opt, ok := req.Opt.(*schedrpc.AddScheduleRequest_Mqtt); ok {
		def.Topic = opt.Mqtt.Topic
		def.QoS = int(opt.Mqtt.QoS)
	} else if opt, ok := req.Opt.(*schedrpc.AddScheduleRequest_Nats); ok {
		def.Topic = opt.Nats.Subject
		def.QueueName = opt.Nats.QueueName
		def.StreamName = opt.Nats.StreamName
	}

	switch def.Type {
	case model.SCHEDULE_UNDEFINED:
		rsp.Reason = fmt.Sprintf("schedule type '%s' is undefined", req.Type)
		return rsp, nil
	case model.SCHEDULE_TIMER:
		if def.Schedule == "" {
			rsp.Reason = "schedule of timer type should be specified with timer spec"
			return rsp, nil
		}
		if def.Task == "" {
			rsp.Reason = "destination task (tql path) is not specified"
			return rsp, nil
		}
		if _, err := parseSchedule(req.Schedule); err != nil {
			rsp.Reason = err.Error()
			return rsp, nil
		}
	case model.SCHEDULE_SUBSCRIBER:
		if def.Bridge == "" || def.Topic == "" {
			rsp.Reason = "schedule of subscriber type should be specified with bridge and topic"
			return rsp, nil
		}
		if def.Task == "" {
			rsp.Reason = "destination task (tql path) is not specified"
			return rsp, nil
		}
	}
	if err := s.models.SaveSchedule(def); err != nil {
		rsp.Reason = err.Error()
		return rsp, nil
	}

	if err := Register(s, def); err != nil {
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

	if err := s.models.RemoveSchedule(req.Name); err != nil {
		rsp.Reason = err.Error()
		return rsp, nil
	}

	Unregister(req.Name)

	rsp.Success, rsp.Reason = true, "success"
	return rsp, nil

}

func (s *svr) UpdateSchedule(ctx context.Context, req *schedrpc.UpdateScheduleRequest) (*schedrpc.UpdateScheduleResponse, error) {
	tick := time.Now()
	rsp := &schedrpc.UpdateScheduleResponse{}
	defer func() {
		rsp.Elapse = time.Since(tick).String()
	}()

	if ent := GetEntry(req.Name); ent == nil {
		rsp.Reason = fmt.Sprintf("schedule '%s' is not found", req.Name)
		return rsp, nil
	}

	if _, err := parseSchedule(req.Schedule); err != nil {
		rsp.Reason = err.Error()
		return rsp, nil
	}

	sd := &model.ScheduleDefinition{
		Name:      req.Name,
		Task:      req.Task,
		Schedule:  req.Schedule,
		AutoStart: req.AutoStart,
		Type:      model.SCHEDULE_TIMER,
	}
	if err := s.models.UpdateSchedule(sd); err != nil {
		rsp.Reason = err.Error()
		return rsp, nil
	}

	if err := Register(s, sd); err != nil {
		rsp.Reason = err.Error()
		return rsp, nil
	}

	rsp.Success, rsp.Reason = true, "success"
	return rsp, nil
}

func (s *svr) StartSchedule(ctx context.Context, req *schedrpc.StartScheduleRequest) (*schedrpc.StartScheduleResponse, error) {
	tick := time.Now()
	rsp := &schedrpc.StartScheduleResponse{}
	defer func() {
		rsp.Elapse = time.Since(tick).String()
	}()

	if ent := GetEntry(req.Name); ent == nil {
		rsp.Reason = fmt.Sprintf("schedule '%s' is not found", req.Name)
	} else {
		if err := ent.Start(); err != nil {
			rsp.Reason = fmt.Sprintf("schedule '%s' fail to start; %s", req.Name, err.Error())
		} else {
			rsp.Success, rsp.Reason = true, "success"
		}
	}
	return rsp, nil
}

func (s *svr) StopSchedule(ctx context.Context, req *schedrpc.StopScheduleRequest) (*schedrpc.StopScheduleResponse, error) {
	tick := time.Now()
	rsp := &schedrpc.StopScheduleResponse{}
	defer func() {
		rsp.Elapse = time.Since(tick).String()
	}()

	if ent := GetEntry(req.Name); ent == nil {
		rsp.Reason = fmt.Sprintf("schedule '%s' is not found", req.Name)
	} else {
		if err := ent.Stop(); err != nil {
			rsp.Reason = fmt.Sprintf("schedule '%s' fail to stop; %s", req.Name, err.Error())
		} else {
			rsp.Success, rsp.Reason = true, "success"
		}
	}
	return rsp, nil
}

func parseSchedule(schedule string) (cron.Schedule, error) {
	scheduleParser := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	if s, err := scheduleParser.Parse(schedule); err != nil {
		return nil, fmt.Errorf("invalid schedule, %s", err.Error())
	} else {
		return s, err
	}
}
