package timer

import (
	"context"
	"fmt"
	"time"

	"github.com/machbase/neo-server/v8/mods/model"
	"github.com/robfig/cron/v3"
)

type Info struct {
	Id        int64  `json:"id,omitempty"`
	UserName  string `json:"userName,omitempty"`
	ExecUser  string `json:"execUser,omitempty"`
	Name      string `json:"name,omitempty"`
	AutoStart bool   `json:"autoStart,omitempty"`
	State     string `json:"state,omitempty"`
	Task      string `json:"task,omitempty"`
	Schedule  string `json:"schedule,omitempty"`
}

type Response struct {
	Success bool   `json:"success,omitempty"`
	Reason  string `json:"reason,omitempty"`
	Elapse  string `json:"elapse,omitempty"`
	Id      int64  `json:"id,omitempty"`
}

type ListResponse struct {
	Response
	Timers []*Info `json:"timers,omitempty"`
}

type GetResponse struct {
	Response
	Timer *Info `json:"timer,omitempty"`
}

type AddRequest struct {
	Name      string `json:"name,omitempty"`
	AutoStart bool   `json:"autoStart,omitempty"`
	Task      string `json:"task,omitempty"`
	Schedule  string `json:"schedule,omitempty"`
	ExecUser  string `json:"execUser,omitempty"`
}

type UpdateRequest struct {
	Id        int64  `json:"id"`
	AutoStart bool   `json:"autoStart,omitempty"`
	Task      string `json:"task,omitempty"`
	Schedule  string `json:"schedule,omitempty"`
	ExecUser  string `json:"execUser,omitempty"`
}

func info(definition *model.TimerDefinition) *Info {
	state := UNKNOWN.String()
	if entry := GetEntry(definition.Id); entry != nil {
		state = entry.Status().String()
		if err := entry.Error(); err != nil {
			state = fmt.Sprintf("%s, %s", state, err)
		}
	}
	return &Info{Id: definition.Id, UserName: definition.UserName, ExecUser: definition.ExecUser, Name: definition.Name, AutoStart: definition.AutoStart, State: state, Task: definition.Task, Schedule: definition.Schedule}
}

func response(start time.Time, err error) *Response {
	result := &Response{Elapse: time.Since(start).String()}
	if err != nil {
		result.Reason = err.Error()
		return result
	}
	result.Success, result.Reason = true, "success"
	return result
}

func (service *Service) List(ctx context.Context, scope model.UserScope) (*ListResponse, error) {
	start := time.Now()
	definitions, err := service.models.LoadTimers(ctx, scope)
	result := &ListResponse{Response: *response(start, err), Timers: []*Info{}}
	for _, definition := range definitions {
		result.Timers = append(result.Timers, info(definition))
	}
	return result, nil
}

func (service *Service) Get(ctx context.Context, scope model.UserScope, id int64) (*GetResponse, error) {
	start := time.Now()
	definition, err := service.models.LoadTimerForUser(ctx, scope, id)
	result := &GetResponse{Response: *response(start, err)}
	if err == nil {
		result.Timer = info(definition)
	}
	return result, nil
}

func (service *Service) Add(ctx context.Context, scope model.UserScope, request *AddRequest) (*Response, error) {
	start := time.Now()
	if request.Name == "" || len(request.Name) > 40 || request.Task == "" || request.Schedule == "" {
		return response(start, fmt.Errorf("timer name, task, and schedule are required")), nil
	}
	if _, err := parseSchedule(request.Schedule); err != nil {
		return response(start, err), nil
	}
	if _, err := service.tqlLoader.Load(request.Task); err != nil {
		return response(start, err), nil
	}
	execUser := request.ExecUser
	if scope.User != "SYS" && scope.User != "sys" || execUser == "" {
		execUser = scope.User
	}
	definition := &model.TimerDefinition{Name: request.Name, ExecUser: execUser, AutoStart: request.AutoStart, Task: request.Task, Schedule: request.Schedule}
	if err := service.models.SaveTimerForUser(ctx, scope, definition); err != nil {
		return response(start, err), nil
	}
	result := response(start, service.Register(definition))
	result.Id = definition.Id
	return result, nil
}

func (service *Service) Update(ctx context.Context, scope model.UserScope, request *UpdateRequest) (*Response, error) {
	start := time.Now()
	if _, err := parseSchedule(request.Schedule); err != nil {
		return response(start, err), nil
	}
	definition, err := service.models.LoadTimerForUser(ctx, scope, request.Id)
	if err != nil {
		return response(start, err), nil
	}
	definition.AutoStart, definition.Task, definition.Schedule = request.AutoStart, request.Task, request.Schedule
	if scope.User == "SYS" || scope.User == "sys" {
		if request.ExecUser != "" {
			definition.ExecUser = request.ExecUser
		}
	}
	if err := service.models.SaveTimerForUser(ctx, scope, definition); err != nil {
		return response(start, err), nil
	}
	return response(start, service.Register(definition)), nil
}

func (service *Service) Delete(ctx context.Context, scope model.UserScope, id int64) (*Response, error) {
	start := time.Now()
	if _, err := service.models.LoadTimerForUser(ctx, scope, id); err != nil {
		return response(start, err), nil
	}
	if err := service.models.RemoveTimerForUser(ctx, scope, id); err != nil {
		return response(start, err), nil
	}
	Unregister(id)
	return response(start, nil), nil
}

func (service *Service) StartTimer(ctx context.Context, scope model.UserScope, id int64) (*Response, error) {
	start := time.Now()
	if _, err := service.models.LoadTimerForUser(ctx, scope, id); err != nil {
		return response(start, err), nil
	}
	entry := GetEntry(id)
	if entry == nil {
		return response(start, fmt.Errorf("timer id '%d' is not registered", id)), nil
	}
	return response(start, entry.Start()), nil
}

func (service *Service) StopTimer(ctx context.Context, scope model.UserScope, id int64) (*Response, error) {
	start := time.Now()
	if _, err := service.models.LoadTimerForUser(ctx, scope, id); err != nil {
		return response(start, err), nil
	}
	entry := GetEntry(id)
	if entry == nil {
		return response(start, fmt.Errorf("timer id '%d' is not registered", id)), nil
	}
	return response(start, entry.Stop()), nil
}

func parseSchedule(value string) (cron.Schedule, error) {
	parser := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	schedule, err := parser.Parse(value)
	if err != nil {
		return nil, fmt.Errorf("invalid schedule, %s", err)
	}
	return schedule, nil
}
