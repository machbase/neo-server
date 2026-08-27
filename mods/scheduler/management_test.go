package scheduler

import (
	"context"
	"errors"
	"testing"

	"github.com/machbase/neo-server/v8/mods/model"
	"github.com/machbase/neo-server/v8/mods/tql"
	"github.com/stretchr/testify/require"
)

type managementProviderStub struct {
	timers map[int64]*model.TimerDefinition
	subs   map[int64]*model.SubscriberDefinition
}

func (p *managementProviderStub) LoadAllTimers(context.Context) ([]*model.TimerDefinition, error) {
	ret := make([]*model.TimerDefinition, 0, len(p.timers))
	for _, def := range p.timers {
		ret = append(ret, def)
	}
	return ret, nil
}
func (p *managementProviderStub) LoadTimer(_ context.Context, name string) (*model.TimerDefinition, error) {
	for _, def := range p.timers {
		if def.Name == name {
			return def, nil
		}
	}
	return nil, errors.New("timer not found")
}
func (p *managementProviderStub) LoadTimerByID(_ context.Context, id int64) (*model.TimerDefinition, error) {
	def, ok := p.timers[id]
	if !ok {
		return nil, errors.New("timer not found")
	}
	return def, nil
}
func (p *managementProviderStub) SaveTimer(_ context.Context, def *model.TimerDefinition) error {
	if def.Id == 0 {
		def.Id = int64(len(p.timers) + 1)
	}
	p.timers[def.Id] = def
	return nil
}
func (p *managementProviderStub) RemoveTimer(_ context.Context, name string) error {
	for id, def := range p.timers {
		if def.Name == name {
			delete(p.timers, id)
			return nil
		}
	}
	return errors.New("timer not found")
}
func (p *managementProviderStub) RemoveTimerByID(_ context.Context, id int64) error {
	if _, ok := p.timers[id]; !ok {
		return errors.New("timer not found")
	}
	delete(p.timers, id)
	return nil
}
func (p *managementProviderStub) LoadAllSubscribers(context.Context) ([]*model.SubscriberDefinition, error) {
	ret := make([]*model.SubscriberDefinition, 0, len(p.subs))
	for _, def := range p.subs {
		ret = append(ret, def)
	}
	return ret, nil
}
func (p *managementProviderStub) LoadSubscriber(_ context.Context, name string) (*model.SubscriberDefinition, error) {
	for _, def := range p.subs {
		if def.Name == name {
			return def, nil
		}
	}
	return nil, errors.New("subscriber not found")
}
func (p *managementProviderStub) LoadSubscriberByID(_ context.Context, id int64) (*model.SubscriberDefinition, error) {
	def, ok := p.subs[id]
	if !ok {
		return nil, errors.New("subscriber not found")
	}
	return def, nil
}
func (p *managementProviderStub) SaveSubscriber(_ context.Context, def *model.SubscriberDefinition) error {
	if def.Id == 0 {
		def.Id = int64(len(p.subs) + 1)
	}
	p.subs[def.Id] = def
	return nil
}
func (p *managementProviderStub) RemoveSubscriber(_ context.Context, name string) error {
	for id, def := range p.subs {
		if def.Name == name {
			delete(p.subs, id)
			return nil
		}
	}
	return errors.New("subscriber not found")
}
func (p *managementProviderStub) RemoveSubscriberByID(_ context.Context, id int64) error {
	if _, ok := p.subs[id]; !ok {
		return errors.New("subscriber not found")
	}
	delete(p.subs, id)
	return nil
}

func newManagementService(provider *managementProviderStub) *Service {
	return NewService(
		WithProvider(provider),
		WithTqlLoader(managementTqlLoader{}),
	)
}

type managementTqlLoader struct{}

func (managementTqlLoader) Load(name string) (*tql.Script, error) { return nil, nil }

func TestManagementScheduleLifecycle(t *testing.T) {
	provider := &managementProviderStub{timers: map[int64]*model.TimerDefinition{}, subs: map[int64]*model.SubscriberDefinition{}}
	service := newManagementService(provider)
	ctx := context.Background()

	addTimer, err := service.AddSchedule(ctx, &AddScheduleRequest{
		Name: "timer-one", Type: "timer", ExecUser: "SYS", Task: "timer.tql", Schedule: "@every 1m",
	})
	require.NoError(t, err)
	require.True(t, addTimer.Success)
	require.Equal(t, int64(1), addTimer.Id)

	addSub, err := service.AddSchedule(ctx, &AddScheduleRequest{
		Name: "sub-one", Type: "subscriber", ExecUser: "SYS", Task: "append.tql", Bridge: "mqtt",
		Opt: AddScheduleOption{Mqtt: &MqttOption{Topic: "events", QoS: 1}},
	})
	require.NoError(t, err)
	require.True(t, addSub.Success)
	require.Equal(t, int64(1), addSub.Id)

	list, err := service.ListSchedule(ctx)
	require.NoError(t, err)
	require.True(t, list.Success)
	require.Len(t, list.Schedules, 2)
	require.Equal(t, int64(1), list.Schedules[0].Id)

	gotTimer, err := service.GetTimer(ctx, &GetTimerRequest{Id: addTimer.Id})
	require.NoError(t, err)
	require.True(t, gotTimer.Success)
	require.Equal(t, int64(1), gotTimer.Schedule.Id)

	gotSub, err := service.GetSubscriber(ctx, &GetSubscriberRequest{Id: addSub.Id})
	require.NoError(t, err)
	require.True(t, gotSub.Success)
	require.Equal(t, "events", gotSub.Schedule.Topic)

	updated, err := service.UpdateTimer(ctx, &UpdateTimerRequest{Id: addTimer.Id, Task: "updated.tql", Schedule: "@every 2m"})
	require.NoError(t, err)
	require.True(t, updated.Success)
	require.Equal(t, "updated.tql", provider.timers[1].Task)

	deleted, err := service.DeleteTimer(ctx, &DeleteTimerRequest{Id: addTimer.Id})
	require.NoError(t, err)
	require.True(t, deleted.Success)
	deleted, err = service.DeleteSubscriber(ctx, &DeleteSubscriberRequest{Id: addSub.Id})
	require.NoError(t, err)
	require.True(t, deleted.Success)
}

func TestScheduleTypeParsing(t *testing.T) {
	require.Equal(t, SCHEDULE_TIMER, ParseScheduleType("timer"))
	require.Equal(t, SCHEDULE_SUBSCRIBER, ParseScheduleType("SUBSCRIBER"))
	require.Equal(t, SCHEDULE_UNDEFINED, ParseScheduleType("other"))
	require.Equal(t, "TIMER", SCHEDULE_TIMER.String())
	require.Equal(t, "SUBSCRIBER", SCHEDULE_SUBSCRIBER.String())
	require.Equal(t, "UNDEFINED", ScheduleType("unknown").String())
}

func TestNewServiceAndScheduleState(t *testing.T) {
	cronService := NewService()
	require.NotNil(t, cronService)
	require.Equal(t, UNKNOWN.String(), scheduleState("missing"))
	cronService.crons.Stop()
}

func TestScheduleCompatibilityManagement(t *testing.T) {
	provider := &managementProviderStub{
		timers: map[int64]*model.TimerDefinition{1: {Id: 1, Name: "timer", ExecUser: "SYS", Task: "task", Schedule: "@every 1m"}},
		subs:   map[int64]*model.SubscriberDefinition{1: {Id: 1, Name: "subscriber", ExecUser: "SYS", Task: "task", Bridge: "mqtt", Topic: "topic"}},
	}
	service := newManagementService(provider)
	ctx := context.Background()
	registryLock.Lock()
	registry = map[string]Entry{}
	registryLock.Unlock()
	t.Cleanup(UnregisterAll)
	require.NoError(t, RegisterTimer(service, provider.timers[1]))

	list, err := service.ListTimer(ctx)
	require.NoError(t, err)
	require.True(t, list.Success)
	require.Len(t, list.Schedules, 1)
	list, err = service.ListSubscriber(ctx)
	require.NoError(t, err)
	require.True(t, list.Success)
	require.Len(t, list.Schedules, 1)

	got, err := service.GetSchedule(ctx, &GetScheduleRequest{Name: "timer"})
	require.NoError(t, err)
	require.True(t, got.Success)
	require.Equal(t, int64(1), got.Schedule.Id)

	updated, err := service.UpdateSchedule(ctx, &UpdateScheduleRequest{Name: "timer", Task: "updated", Schedule: "@every 2m"})
	require.NoError(t, err)
	require.True(t, updated.Success)

	started, err := service.StartSchedule(ctx, &StartScheduleRequest{Name: "timer"})
	require.NoError(t, err)
	require.True(t, started.Success)
	stopped, err := service.StopSchedule(ctx, &StopScheduleRequest{Name: "timer"})
	require.NoError(t, err)
	require.True(t, stopped.Success)

	deleted, err := service.DelSchedule(ctx, &DelScheduleRequest{Name: "timer"})
	require.NoError(t, err)
	require.True(t, deleted.Success)
	deleted, err = service.DelSchedule(ctx, &DelScheduleRequest{Name: "subscriber"})
	require.NoError(t, err)
	require.True(t, deleted.Success)
}

func TestSchedulerLifecycleAndLogging(t *testing.T) {
	provider := &managementProviderStub{
		timers: map[int64]*model.TimerDefinition{1: {Id: 1, Name: "timer", ExecUser: "SYS", Task: "task", Schedule: "@every 1m"}},
		subs:   map[int64]*model.SubscriberDefinition{1: {Id: 1, Name: "subscriber", ExecUser: "SYS", Task: "task", Bridge: "mqtt", Topic: "topic"}},
	}
	service := newManagementService(provider)
	registryLock.Lock()
	registry = map[string]Entry{}
	registryLock.Unlock()
	t.Cleanup(UnregisterAll)

	require.NoError(t, service.Start())
	service.Stop()

	entry := newSchedulerEntryStub("manual", STOP, true)
	require.NoError(t, service.AddEntry(entry))
	require.Equal(t, RUNNING, entry.Status())
	service.Info("info", "entry", 1, "ignored", "value")
	service.Info("info", "now", "ignored")
	service.Error(errors.New("expected"), "error")
}
