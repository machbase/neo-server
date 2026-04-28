package scheduler

import (
	"context"
	"fmt"
	"time"

	"github.com/machbase/neo-server/v8/mods/model"
	"github.com/robfig/cron/v3"
)

type ListScheduleResponse struct {
	Success   bool        `json:"success,omitempty"`
	Reason    string      `json:"reason,omitempty"`
	Elapse    string      `json:"elapse,omitempty"`
	Schedules []*Schedule `json:"schedules,omitempty"`
}

type Schedule struct {
	Name      string `json:"name,omitempty"`
	Type      string `json:"type,omitempty"`
	AutoStart bool   `json:"autoStart,omitempty"`
	State     string `json:"state,omitempty"`
	Task      string `json:"task,omitempty"`
	Schedule  string `json:"schedule,omitempty"`
	Bridge    string `json:"bridge,omitempty"`
	Topic     string `json:"topic,omitempty"`
	QoS       int32  `json:"QoS,omitempty"`
}

func (s *Service) ListSchedule(context.Context) (*ListScheduleResponse, error) {
	tick := time.Now()
	rsp := &ListScheduleResponse{}
	defer func() {
		rsp.Elapse = time.Since(tick).String()
	}()
	lst, err := s.models.LoadAllSchedules()
	if err != nil {
		rsp.Reason = err.Error()
		return rsp, nil
	}
	for _, define := range lst {
		sched := &Schedule{
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

type GetScheduleRequest struct {
	Name string `json:"name,omitempty"`
}

type GetScheduleResponse struct {
	Success  bool      `json:"success,omitempty"`
	Reason   string    `json:"reason,omitempty"`
	Elapse   string    `json:"elapse,omitempty"`
	Schedule *Schedule `json:"schedule,omitempty"`
}

func (s *Service) GetSchedule(ctx context.Context, req *GetScheduleRequest) (*GetScheduleResponse, error) {
	tick := time.Now()
	rsp := &GetScheduleResponse{}
	defer func() {
		rsp.Elapse = time.Since(tick).String()
	}()
	if define, err := s.models.LoadSchedule(req.Name); err != nil {
		rsp.Reason = err.Error()
	} else {
		rsp.Schedule = &Schedule{
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

type AddScheduleRequest struct {
	Name      string            `json:"name,omitempty"`
	Type      string            `json:"type,omitempty"`
	AutoStart bool              `json:"autoStart,omitempty"`
	Task      string            `json:"task,omitempty"`
	Schedule  string            `json:"schedule,omitempty"`
	Bridge    string            `json:"bridge,omitempty"`
	Opt       AddScheduleOption `json:"opt"`
}

type AddScheduleOption struct {
	Mqtt *MqttOption `json:"mqtt,omitempty"`
	Nats *NatsOption `json:"nats,omitempty"`
}

type MqttOption struct {
	Topic string `json:"Topic,omitempty"`
	QoS   int32  `json:"QoS,omitempty"`
}

type NatsOption struct {
	Subject    string `json:"Subject,omitempty"`
	QueueName  string `json:"QueueName,omitempty"`
	StreamName string `json:"StreamName,omitempty"`
}

type AddScheduleResponse struct {
	Success bool   `json:"success"`
	Reason  string `json:"reason"`
	Elapse  string `json:"elapse"`
}

func (s *Service) AddSchedule(ctx context.Context, req *AddScheduleRequest) (*AddScheduleResponse, error) {
	tick := time.Now()
	rsp := &AddScheduleResponse{Reason: "not specified"}
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
	if req.Opt.Mqtt != nil {
		def.Topic = req.Opt.Mqtt.Topic
		def.QoS = int(req.Opt.Mqtt.QoS)
	} else if req.Opt.Nats != nil {
		def.Topic = req.Opt.Nats.Subject
		def.QueueName = req.Opt.Nats.QueueName
		def.StreamName = req.Opt.Nats.StreamName
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

type DelScheduleRequest struct {
	Name string `json:"name,omitempty"`
}

type DelScheduleResponse struct {
	Success bool   `json:"success,omitempty"`
	Reason  string `json:"reason,omitempty"`
	Elapse  string `json:"elapse,omitempty"`
}

func (s *Service) DelSchedule(ctx context.Context, req *DelScheduleRequest) (*DelScheduleResponse, error) {
	tick := time.Now()
	rsp := &DelScheduleResponse{}
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

type UpdateScheduleRequest struct {
	Name      string `json:"name,omitempty"`
	AutoStart bool   `json:"autoStart,omitempty"`
	Task      string `json:"task,omitempty"`
	Schedule  string `json:"schedule,omitempty"`
	Bridge    string `json:"bridge,omitempty"`
	Topic     string `json:"topic,omitempty"`
	QoS       int32  `json:"QoS,omitempty"`
}

type UpdateScheduleResponse struct {
	Success bool   `json:"success,omitempty"`
	Reason  string `json:"reason,omitempty"`
	Elapse  string `json:"elapse,omitempty"`
}

func (s *Service) UpdateSchedule(ctx context.Context, req *UpdateScheduleRequest) (*UpdateScheduleResponse, error) {
	tick := time.Now()
	rsp := &UpdateScheduleResponse{}
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

type StartScheduleRequest struct {
	Name string `json:"name,omitempty"`
}

type StartScheduleResponse struct {
	Success bool   `json:"success,omitempty"`
	Reason  string `json:"reason,omitempty"`
	Elapse  string `json:"elapse,omitempty"`
}

func (s *Service) StartSchedule(ctx context.Context, req *StartScheduleRequest) (*StartScheduleResponse, error) {
	tick := time.Now()
	rsp := &StartScheduleResponse{}
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

type StopScheduleRequest struct {
	Name string `json:"name,omitempty"`
}

type StopScheduleResponse struct {
	Success bool   `json:"success,omitempty"`
	Reason  string `json:"reason,omitempty"`
	Elapse  string `json:"elapse,omitempty"`
}

func (s *Service) StopSchedule(ctx context.Context, req *StopScheduleRequest) (*StopScheduleResponse, error) {
	tick := time.Now()
	rsp := &StopScheduleResponse{}
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
