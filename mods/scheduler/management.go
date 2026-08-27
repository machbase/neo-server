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
	Id        int64  `json:"id,omitempty"`
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

func scheduleState(name string) string {
	ent := GetEntry(name)
	if ent == nil {
		return UNKNOWN.String()
	}
	if err := ent.Error(); err != nil {
		return fmt.Sprintf("%s, %s", ent.Status().String(), err.Error())
	}
	return ent.Status().String()
}

func timerToSchedule(define *model.TimerDefinition) *Schedule {
	return &Schedule{
		Id:        define.Id,
		Name:      define.Name,
		Type:      SCHEDULE_TIMER.String(),
		AutoStart: define.AutoStart,
		State:     scheduleState(define.Name),
		Task:      define.Task,
		Schedule:  define.Schedule,
	}
}

func subscriberToSchedule(define *model.SubscriberDefinition) *Schedule {
	return &Schedule{
		Id:        define.Id,
		Name:      define.Name,
		Type:      SCHEDULE_SUBSCRIBER.String(),
		AutoStart: define.AutoStart,
		State:     scheduleState(define.Name),
		Task:      define.Task,
		Bridge:    define.Bridge,
		Topic:     define.Topic,
		QoS:       int32(define.QoS),
	}
}

func (s *Service) ListSchedule(ctx context.Context) (*ListScheduleResponse, error) {
	tick := time.Now()
	rsp := &ListScheduleResponse{}
	defer func() {
		rsp.Elapse = time.Since(tick).String()
	}()
	timers, err := s.models.LoadAllTimers(ctx)
	if err != nil {
		rsp.Reason = err.Error()
		return rsp, nil
	}
	for _, define := range timers {
		rsp.Schedules = append(rsp.Schedules, timerToSchedule(define))
	}
	subs, err := s.models.LoadAllSubscribers(ctx)
	if err != nil {
		rsp.Reason = err.Error()
		return rsp, nil
	}
	for _, define := range subs {
		rsp.Schedules = append(rsp.Schedules, subscriberToSchedule(define))
	}
	rsp.Success, rsp.Reason = true, "success"
	return rsp, nil
}

// ListTimer returns only timer schedules.
func (s *Service) ListTimer(ctx context.Context) (*ListScheduleResponse, error) {
	tick := time.Now()
	rsp := &ListScheduleResponse{}
	defer func() {
		rsp.Elapse = time.Since(tick).String()
	}()
	timers, err := s.models.LoadAllTimers(ctx)
	if err != nil {
		rsp.Reason = err.Error()
		return rsp, nil
	}
	for _, define := range timers {
		rsp.Schedules = append(rsp.Schedules, timerToSchedule(define))
	}
	rsp.Success, rsp.Reason = true, "success"
	return rsp, nil
}

// ListSubscriber returns only subscriber schedules.
func (s *Service) ListSubscriber(ctx context.Context) (*ListScheduleResponse, error) {
	tick := time.Now()
	rsp := &ListScheduleResponse{}
	defer func() {
		rsp.Elapse = time.Since(tick).String()
	}()
	subs, err := s.models.LoadAllSubscribers(ctx)
	if err != nil {
		rsp.Reason = err.Error()
		return rsp, nil
	}
	for _, define := range subs {
		rsp.Schedules = append(rsp.Schedules, subscriberToSchedule(define))
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

// GetSchedule looks up a schedule by name. Timer and subscriber names share
// a single global namespace, so at most one of the two lookups can succeed.
func (s *Service) GetSchedule(ctx context.Context, req *GetScheduleRequest) (*GetScheduleResponse, error) {
	tick := time.Now()
	rsp := &GetScheduleResponse{}
	defer func() {
		rsp.Elapse = time.Since(tick).String()
	}()
	if define, err := s.models.LoadTimer(ctx, req.Name); err == nil {
		rsp.Schedule = timerToSchedule(define)
		rsp.Success, rsp.Reason = true, "success"
		return rsp, nil
	}
	if define, err := s.models.LoadSubscriber(ctx, req.Name); err == nil {
		rsp.Schedule = subscriberToSchedule(define)
		rsp.Success, rsp.Reason = true, "success"
		return rsp, nil
	}
	rsp.Reason = fmt.Sprintf("schedule '%s' is not found", req.Name)
	return rsp, nil
}

// GetTimerRequest identifies a timer schedule by its auto increment ID.
type GetTimerRequest struct {
	Id int64 `json:"id"`
}

// GetTimer looks up a timer schedule by ID.
func (s *Service) GetTimer(ctx context.Context, req *GetTimerRequest) (*GetScheduleResponse, error) {
	tick := time.Now()
	rsp := &GetScheduleResponse{}
	defer func() {
		rsp.Elapse = time.Since(tick).String()
	}()
	define, err := s.models.LoadTimerByID(ctx, req.Id)
	if err != nil {
		rsp.Reason = err.Error()
		return rsp, nil
	}
	rsp.Schedule = timerToSchedule(define)
	rsp.Success, rsp.Reason = true, "success"
	return rsp, nil
}

// GetSubscriberRequest identifies a subscriber schedule by its auto
// increment ID.
type GetSubscriberRequest struct {
	Id int64 `json:"id"`
}

// GetSubscriber looks up a subscriber schedule by ID.
func (s *Service) GetSubscriber(ctx context.Context, req *GetSubscriberRequest) (*GetScheduleResponse, error) {
	tick := time.Now()
	rsp := &GetScheduleResponse{}
	defer func() {
		rsp.Elapse = time.Since(tick).String()
	}()
	define, err := s.models.LoadSubscriberByID(ctx, req.Id)
	if err != nil {
		rsp.Reason = err.Error()
		return rsp, nil
	}
	rsp.Schedule = subscriberToSchedule(define)
	rsp.Success, rsp.Reason = true, "success"
	return rsp, nil
}

type AddScheduleRequest struct {
	Name      string            `json:"name,omitempty"`
	Type      string            `json:"type,omitempty"`
	AutoStart bool              `json:"autoStart,omitempty"`
	Task      string            `json:"task,omitempty"`
	Schedule  string            `json:"schedule,omitempty"`
	Bridge    string            `json:"bridge,omitempty"`
	ExecUser  string            `json:"execUser,omitempty"`
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
	Id      int64  `json:"id,omitempty"`
}

func (s *Service) AddSchedule(ctx context.Context, req *AddScheduleRequest) (*AddScheduleResponse, error) {
	tick := time.Now()
	rsp := &AddScheduleResponse{Reason: "not specified"}
	defer func() {
		rsp.Elapse = time.Since(tick).String()
	}()

	if len(req.Name) > 40 {
		rsp.Reason = "name is too long, should be shorter than 40 characters"
		return rsp, nil
	}

	switch ParseScheduleType(req.Type) {
	default:
		rsp.Reason = fmt.Sprintf("schedule type '%s' is undefined", req.Type)
		return rsp, nil
	case SCHEDULE_TIMER:
		def := &model.TimerDefinition{
			Name:      req.Name,
			ExecUser:  req.ExecUser,
			AutoStart: req.AutoStart,
			Task:      req.Task,
			Schedule:  req.Schedule,
		}
		if def.Schedule == "" {
			rsp.Reason = "schedule of timer type should be specified with timer spec"
			return rsp, nil
		}
		if def.Task == "" {
			rsp.Reason = "destination task (tql path) is not specified"
			return rsp, nil
		}
		if _, err := parseSchedule(def.Schedule); err != nil {
			rsp.Reason = err.Error()
			return rsp, nil
		}
		// Upsert by name: re-adding a schedule with an existing name reuses
		// its ID (update) instead of failing on a name collision, matching
		// the original file-based "overwrite" behavior.
		if existing, err := s.models.LoadTimer(ctx, def.Name); err == nil {
			def.Id = existing.Id
		}
		if err := s.models.SaveTimer(ctx, def); err != nil {
			rsp.Reason = err.Error()
			return rsp, nil
		}
		if err := RegisterTimer(s, def); err != nil {
			rsp.Reason = err.Error()
			return rsp, nil
		}
		rsp.Id = def.Id
	case SCHEDULE_SUBSCRIBER:
		def := &model.SubscriberDefinition{
			Name:      req.Name,
			ExecUser:  req.ExecUser,
			AutoStart: req.AutoStart,
			Task:      req.Task,
			Bridge:    req.Bridge,
		}
		if req.Opt.Mqtt != nil {
			def.Topic = req.Opt.Mqtt.Topic
			def.QoS = int(req.Opt.Mqtt.QoS)
		} else if req.Opt.Nats != nil {
			def.Topic = req.Opt.Nats.Subject
			def.QueueName = req.Opt.Nats.QueueName
			def.StreamName = req.Opt.Nats.StreamName
		}
		if def.Bridge == "" || def.Topic == "" {
			rsp.Reason = "schedule of subscriber type should be specified with bridge and topic"
			return rsp, nil
		}
		if def.Task == "" {
			rsp.Reason = "destination task (tql path) is not specified"
			return rsp, nil
		}
		// Upsert by name: see the corresponding comment in the timer branch.
		if existing, err := s.models.LoadSubscriber(ctx, def.Name); err == nil {
			def.Id = existing.Id
		}
		if err := s.models.SaveSubscriber(ctx, def); err != nil {
			rsp.Reason = err.Error()
			return rsp, nil
		}
		if err := RegisterSubscriber(s, def); err != nil {
			rsp.Reason = err.Error()
			return rsp, nil
		}
		rsp.Id = def.Id
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

// DelSchedule removes a schedule by name. Since timer and subscriber names
// share a single global namespace, it tries the timer table first and
// falls back to the subscriber table.
func (s *Service) DelSchedule(ctx context.Context, req *DelScheduleRequest) (*DelScheduleResponse, error) {
	tick := time.Now()
	rsp := &DelScheduleResponse{}
	defer func() {
		rsp.Elapse = time.Since(tick).String()
	}()

	if err := s.models.RemoveTimer(ctx, req.Name); err != nil {
		if err := s.models.RemoveSubscriber(ctx, req.Name); err != nil {
			rsp.Reason = err.Error()
			return rsp, nil
		}
	}

	Unregister(req.Name)

	rsp.Success, rsp.Reason = true, "success"
	return rsp, nil

}

// DeleteTimerRequest identifies a timer schedule by its auto increment ID.
type DeleteTimerRequest struct {
	Id int64 `json:"id"`
}

// DeleteTimer removes a timer schedule by ID.
func (s *Service) DeleteTimer(ctx context.Context, req *DeleteTimerRequest) (*DelScheduleResponse, error) {
	tick := time.Now()
	rsp := &DelScheduleResponse{}
	defer func() {
		rsp.Elapse = time.Since(tick).String()
	}()
	def, err := s.models.LoadTimerByID(ctx, req.Id)
	if err != nil {
		rsp.Reason = err.Error()
		return rsp, nil
	}
	if err := s.models.RemoveTimerByID(ctx, req.Id); err != nil {
		rsp.Reason = err.Error()
		return rsp, nil
	}
	Unregister(def.Name)
	rsp.Success, rsp.Reason = true, "success"
	return rsp, nil
}

// DeleteSubscriberRequest identifies a subscriber schedule by its auto
// increment ID.
type DeleteSubscriberRequest struct {
	Id int64 `json:"id"`
}

// DeleteSubscriber removes a subscriber schedule by ID.
func (s *Service) DeleteSubscriber(ctx context.Context, req *DeleteSubscriberRequest) (*DelScheduleResponse, error) {
	tick := time.Now()
	rsp := &DelScheduleResponse{}
	defer func() {
		rsp.Elapse = time.Since(tick).String()
	}()
	def, err := s.models.LoadSubscriberByID(ctx, req.Id)
	if err != nil {
		rsp.Reason = err.Error()
		return rsp, nil
	}
	if err := s.models.RemoveSubscriberByID(ctx, req.Id); err != nil {
		rsp.Reason = err.Error()
		return rsp, nil
	}
	Unregister(def.Name)
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

// UpdateSchedule updates an existing timer schedule. It loads the current
// definition first so the immutable ExecUser (creator) is preserved.
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

	def, err := s.models.LoadTimer(ctx, req.Name)
	if err != nil {
		rsp.Reason = err.Error()
		return rsp, nil
	}
	def.Task = req.Task
	def.Schedule = req.Schedule
	def.AutoStart = req.AutoStart
	if err := s.models.SaveTimer(ctx, def); err != nil {
		rsp.Reason = err.Error()
		return rsp, nil
	}

	if err := RegisterTimer(s, def); err != nil {
		rsp.Reason = err.Error()
		return rsp, nil
	}

	rsp.Success, rsp.Reason = true, "success"
	return rsp, nil
}

// UpdateTimerRequest identifies the timer schedule to update by its auto
// increment ID.
type UpdateTimerRequest struct {
	Id        int64  `json:"id"`
	AutoStart bool   `json:"autoStart,omitempty"`
	Task      string `json:"task,omitempty"`
	Schedule  string `json:"schedule,omitempty"`
}

// UpdateTimer updates an existing timer schedule by ID. It loads the
// current definition first so the immutable ExecUser (creator) is
// preserved.
func (s *Service) UpdateTimer(ctx context.Context, req *UpdateTimerRequest) (*UpdateScheduleResponse, error) {
	tick := time.Now()
	rsp := &UpdateScheduleResponse{}
	defer func() {
		rsp.Elapse = time.Since(tick).String()
	}()

	if _, err := parseSchedule(req.Schedule); err != nil {
		rsp.Reason = err.Error()
		return rsp, nil
	}

	def, err := s.models.LoadTimerByID(ctx, req.Id)
	if err != nil {
		rsp.Reason = err.Error()
		return rsp, nil
	}
	if ent := GetEntry(def.Name); ent == nil {
		rsp.Reason = fmt.Sprintf("timer id '%d' is not found", req.Id)
		return rsp, nil
	}
	def.Task = req.Task
	def.Schedule = req.Schedule
	def.AutoStart = req.AutoStart
	if err := s.models.SaveTimer(ctx, def); err != nil {
		rsp.Reason = err.Error()
		return rsp, nil
	}

	if err := RegisterTimer(s, def); err != nil {
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
