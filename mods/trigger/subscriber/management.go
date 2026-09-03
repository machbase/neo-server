package subscriber

import (
	"context"
	"fmt"
	"time"

	"github.com/machbase/neo-server/v8/mods/model"
	"github.com/machbase/neo-server/v8/mods/util"
)

type Info struct {
	Id        int64  `json:"id,omitempty"`
	UserName  string `json:"userName,omitempty"`
	ExecUser  string `json:"execUser,omitempty"`
	Name      string `json:"name,omitempty"`
	AutoStart bool   `json:"autoStart,omitempty"`
	State     string `json:"state,omitempty"`
	Task      string `json:"task,omitempty"`
	Bridge    string `json:"bridge,omitempty"`
	Topic     string `json:"topic,omitempty"`
	QoS       int32  `json:"qos,omitempty"`
}

type Response struct {
	Success bool   `json:"success,omitempty"`
	Reason  string `json:"reason,omitempty"`
	Elapse  string `json:"elapse,omitempty"`
	Id      int64  `json:"id,omitempty"`
}

type ListResponse struct {
	Response
	Subscribers []*Info `json:"subscribers,omitempty"`
}

type GetResponse struct {
	Response
	Subscriber *Info `json:"subscriber,omitempty"`
}

type AddRequest struct {
	Name      string `json:"name,omitempty"`
	AutoStart bool   `json:"autoStart,omitempty"`
	Task      string `json:"task,omitempty"`
	Bridge    string `json:"bridge,omitempty"`
	Topic     string `json:"topic,omitempty"`
	QoS       int32  `json:"qos,omitempty"`
	ExecUser  string `json:"execUser,omitempty"`
}

type UpdateRequest struct {
	Id        int64  `json:"id"`
	AutoStart bool   `json:"autoStart,omitempty"`
	Task      string `json:"task,omitempty"`
	Bridge    string `json:"bridge,omitempty"`
	Topic     string `json:"topic,omitempty"`
	QoS       int32  `json:"qos,omitempty"`
	ExecUser  string `json:"execUser,omitempty"`
}

func info(definition *model.SubscriberDefinition) *Info {
	state := UNKNOWN.String()
	if entry := GetEntry(definition.Id); entry != nil {
		state = entry.Status().String()
		if err := entry.Error(); err != nil {
			state = fmt.Sprintf("%s, %s", state, err)
		}
	}
	return &Info{Id: definition.Id, UserName: definition.UserName, ExecUser: definition.ExecUser, Name: definition.Name, AutoStart: definition.AutoStart, State: state, Task: definition.Task, Bridge: definition.Bridge, Topic: definition.Topic, QoS: int32(definition.QoS)}
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
	definitions, err := service.models.LoadSubscribers(ctx, scope)
	result := &ListResponse{Response: *response(start, err), Subscribers: []*Info{}}
	for _, definition := range definitions {
		result.Subscribers = append(result.Subscribers, info(definition))
	}
	return result, nil
}
func (service *Service) Get(ctx context.Context, scope model.UserScope, id int64) (*GetResponse, error) {
	start := time.Now()
	definition, err := service.models.LoadSubscriberForUser(ctx, scope, id)
	result := &GetResponse{Response: *response(start, err)}
	if err == nil {
		result.Subscriber = info(definition)
	}
	return result, nil
}
func (service *Service) Add(ctx context.Context, scope model.UserScope, request *AddRequest) (*Response, error) {
	start := time.Now()
	if request.Name == "" || len(request.Name) > 40 || request.Task == "" || request.Bridge == "" || request.Topic == "" {
		return response(start, fmt.Errorf("subscriber name, task, bridge, and topic are required")), nil
	}
	writeDescriptor, err := util.NewWriteDescriptor(request.Task)
	if err != nil {
		return response(start, err), nil
	}
	if writeDescriptor.IsTqlDestination() {
		if _, err := service.tqlLoader.Load(request.Task); err != nil {
			return response(start, err), nil
		}
	}
	execUser := request.ExecUser
	if scope.User != "SYS" && scope.User != "sys" || execUser == "" {
		execUser = scope.User
	}
	definition := &model.SubscriberDefinition{Name: request.Name, ExecUser: execUser, AutoStart: request.AutoStart, Task: request.Task, Bridge: request.Bridge, Topic: request.Topic, QoS: int(request.QoS)}
	if err := service.models.SaveSubscriberForUser(ctx, scope, definition); err != nil {
		return response(start, err), nil
	}
	result := response(start, service.Register(definition))
	result.Id = definition.Id
	return result, nil
}
func (service *Service) Update(ctx context.Context, scope model.UserScope, request *UpdateRequest) (*Response, error) {
	start := time.Now()
	definition, err := service.models.LoadSubscriberForUser(ctx, scope, request.Id)
	if err != nil {
		return response(start, err), nil
	}
	definition.AutoStart, definition.Task, definition.Bridge, definition.Topic, definition.QoS = request.AutoStart, request.Task, request.Bridge, request.Topic, int(request.QoS)
	if (scope.User == "SYS" || scope.User == "sys") && request.ExecUser != "" {
		definition.ExecUser = request.ExecUser
	}
	if err := service.models.SaveSubscriberForUser(ctx, scope, definition); err != nil {
		return response(start, err), nil
	}
	return response(start, service.Register(definition)), nil
}
func (service *Service) Delete(ctx context.Context, scope model.UserScope, id int64) (*Response, error) {
	start := time.Now()
	if _, err := service.models.LoadSubscriberForUser(ctx, scope, id); err != nil {
		return response(start, err), nil
	}
	if err := service.models.RemoveSubscriberForUser(ctx, scope, id); err != nil {
		return response(start, err), nil
	}
	Unregister(id)
	return response(start, nil), nil
}
func (service *Service) StartSubscriber(ctx context.Context, scope model.UserScope, id int64) (*Response, error) {
	start := time.Now()
	if _, err := service.models.LoadSubscriberForUser(ctx, scope, id); err != nil {
		return response(start, err), nil
	}
	entry := GetEntry(id)
	if entry == nil {
		return response(start, fmt.Errorf("subscriber id '%d' is not registered", id)), nil
	}
	return response(start, entry.Start()), nil
}
func (service *Service) StopSubscriber(ctx context.Context, scope model.UserScope, id int64) (*Response, error) {
	start := time.Now()
	if _, err := service.models.LoadSubscriberForUser(ctx, scope, id); err != nil {
		return response(start, err), nil
	}
	entry := GetEntry(id)
	if entry == nil {
		return response(start, fmt.Errorf("subscriber id '%d' is not registered", id)), nil
	}
	return response(start, entry.Stop()), nil
}
