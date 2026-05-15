package scheduler

import (
	"errors"
	"testing"

	"github.com/machbase/neo-server/v8/mods/bridge"
	"github.com/machbase/neo-server/v8/mods/logging"
	"github.com/machbase/neo-server/v8/mods/model"
	"github.com/machbase/neo-server/v8/mods/tql"
	"github.com/machbase/neo-server/v8/mods/util"
	"github.com/nats-io/nats.go"
	"github.com/robfig/cron/v3"
	"github.com/stretchr/testify/require"
)

type schedulerLoaderStub struct {
	err error
}

func (l schedulerLoaderStub) Load(string) (*tql.Script, error) {
	return nil, l.err
}

type schedulerEntryStub struct {
	BaseEntry
	startCount int
	stopCount  int
	startErr   error
	stopErr    error
}

type schedulerSubscriptionStub struct{}

func (schedulerSubscriptionStub) Unsubscribe() error {
	return nil
}

func (schedulerSubscriptionStub) AddAppended(uint64) {}

func (schedulerSubscriptionStub) AddInserted(uint64) {}

func newSchedulerEntryStub(name string, state State, autoStart bool) *schedulerEntryStub {
	return &schedulerEntryStub{BaseEntry: NewBaseEntry(name, state, autoStart)}
}

func (e *schedulerEntryStub) Start() error {
	e.startCount++
	if e.startErr != nil {
		return e.startErr
	}
	e.setState(RUNNING)
	return nil
}

func (e *schedulerEntryStub) Stop() error {
	e.stopCount++
	if e.stopErr != nil {
		return e.stopErr
	}
	e.setState(STOP)
	return nil
}

func TestBaseEntryStateAndError(t *testing.T) {
	ent := NewBaseEntry("entry", STARTING, true)
	require.Equal(t, "entry", ent.Name())
	require.True(t, ent.AutoStart())
	require.Equal(t, STARTING, ent.Status())
	require.EqualError(t, ent.Start(), "Start() is not implemented")
	require.EqualError(t, ent.Stop(), "Stop() is not implemented")

	err := errors.New("failed")
	ent.setError(err)
	require.ErrorIs(t, ent.Error(), err)
	ent.setStateError(FAILED, err)
	state, gotErr := ent.statusError()
	require.Equal(t, FAILED, state)
	require.ErrorIs(t, gotErr, err)
}

func TestRegistryHelpers(t *testing.T) {
	registryLock.Lock()
	registry = map[string]Entry{}
	entry := newSchedulerEntryStub("mixed_case", RUNNING, false)
	registry["MIXED_CASE"] = entry
	registryLock.Unlock()
	t.Cleanup(UnregisterAll)

	require.Same(t, entry, GetEntry("mixed_case"))
	Unregister("mixed_CASE")
	require.Equal(t, 1, entry.stopCount)
	require.Nil(t, GetEntry("mixed_case"))
}

func TestRegisterTimerAndSubscriber(t *testing.T) {
	registryLock.Lock()
	registry = map[string]Entry{}
	registryLock.Unlock()
	t.Cleanup(UnregisterAll)

	svc := &Service{crons: cron.New(), tqlLoader: schedulerLoaderStub{}}
	timer := &model.ScheduleDefinition{
		Name:     "timer_one",
		Type:     model.SCHEDULE_TIMER,
		Task:     "timer.tql",
		Schedule: "*/5 * * * *",
	}
	require.NoError(t, Register(svc, timer))
	require.NotNil(t, GetEntry("TIMER_ONE"))

	subscriber := &model.ScheduleDefinition{
		Name:   "subscriber_one",
		Type:   model.SCHEDULE_SUBSCRIBER,
		Task:   "db/append/table",
		Bridge: "missing",
		Topic:  "topic/a",
	}
	require.NoError(t, Register(svc, subscriber))
	require.NotNil(t, GetEntry("subscriber_one"))

	unknown := &model.ScheduleDefinition{Name: "bad", Type: "bad"}
	require.EqualError(t, Register(svc, unknown), "undefined schedule type")
}

func TestRegisterTimerLoadFailure(t *testing.T) {
	registryLock.Lock()
	registry = map[string]Entry{}
	registryLock.Unlock()
	t.Cleanup(UnregisterAll)

	svc := &Service{crons: cron.New(), tqlLoader: schedulerLoaderStub{err: errors.New("load failed")}}
	def := &model.ScheduleDefinition{
		Name:     "timer_fail",
		Type:     model.SCHEDULE_TIMER,
		Task:     "missing.tql",
		Schedule: "*/5 * * * *",
	}
	require.EqualError(t, Register(svc, def), "load failed")
	require.Equal(t, FAILED, GetEntry("timer_fail").Status())
}

func TestTimerEntryValidationAndStop(t *testing.T) {
	svc := &Service{crons: cron.New()}

	missingSchedule, err := NewTimerEntry(svc, &model.ScheduleDefinition{Name: "missing_schedule", Task: "task.tql"})
	require.NoError(t, err)
	require.EqualError(t, missingSchedule.Start(), "invalid configure - missing Schedule")
	require.Equal(t, FAILED, missingSchedule.Status())

	missingTask, err := NewTimerEntry(svc, &model.ScheduleDefinition{Name: "missing_task", Schedule: "*/5 * * * *"})
	require.NoError(t, err)
	require.EqualError(t, missingTask.Start(), "invalid configure - missing Task")
	require.Equal(t, FAILED, missingTask.Status())

	valid, err := NewTimerEntry(svc, &model.ScheduleDefinition{Name: "valid", Task: "task.tql", Schedule: "*/5 * * * *"})
	require.NoError(t, err)
	require.NoError(t, valid.Start())
	require.Equal(t, RUNNING, valid.Status())
	require.NoError(t, valid.Stop())
	require.Equal(t, STOP, valid.Status())
}

func TestTimerEntryDoTaskLoadFailure(t *testing.T) {
	svc := &Service{crons: cron.New(), tqlLoader: schedulerLoaderStub{err: errors.New("load failed")}}
	ent, err := NewTimerEntry(svc, &model.ScheduleDefinition{Name: "task_fail", Task: "task.tql", Schedule: "*/5 * * * *"})
	require.NoError(t, err)
	require.NoError(t, ent.Start())
	require.Equal(t, RUNNING, ent.Status())

	ent.doTask()
	require.Equal(t, STOP, ent.Status())
}

func TestSubscriberEntryStartStopValidation(t *testing.T) {
	bridge.UnregisterAll()
	t.Cleanup(bridge.UnregisterAll)

	ent, err := NewSubscriberEntry(&Service{}, &model.ScheduleDefinition{
		Name:   "subscriber",
		Task:   "db/append/table",
		Bridge: "missing",
		Topic:  "topic/a",
	})
	require.NoError(t, err)
	require.EqualError(t, ent.Start(), "undefined bridge name 'missing'")
	require.Equal(t, FAILED, ent.Status())
	require.Error(t, ent.Error())

	mqtt := bridge.NewMqttBridge("mqtt_sub", "")
	require.NoError(t, mqtt.BeforeRegister())
	bridge.UnregisterAll()
	require.NoError(t, bridge.Register(&model.BridgeDefinition{Type: model.BRIDGE_MQTT, Name: "mqtt_sub"}))
	emptyTopic, err := NewSubscriberEntry(&Service{}, &model.ScheduleDefinition{
		Name:   "empty_topic",
		Task:   "db/append/table",
		Bridge: "mqtt_sub",
	})
	require.NoError(t, err)
	require.EqualError(t, emptyTopic.Start(), "empty topic is not allowed, subscribe to bridge 'mqtt_sub' (mqtt)")
	require.Equal(t, FAILED, emptyTopic.Status())
	require.NoError(t, emptyTopic.Stop())
	require.Equal(t, STOP, emptyTopic.Status())

	stopMissingBridge, err := NewSubscriberEntry(&Service{}, &model.ScheduleDefinition{
		Name:   "stop_missing_bridge",
		Task:   "db/append/table",
		Bridge: "missing_stop",
		Topic:  "topic/a",
	})
	require.NoError(t, err)
	stopMissingBridge.subscription = schedulerSubscriptionStub{}
	require.EqualError(t, stopMissingBridge.Stop(), "undefined bridge name 'missing_stop'")
	require.Equal(t, FAILED, stopMissingBridge.Status())

	require.NoError(t, bridge.Register(&model.BridgeDefinition{Type: model.BRIDGE_MQTT, Name: "mqtt_wait"}))
	waitingMqtt, err := NewSubscriberEntry(&Service{}, &model.ScheduleDefinition{
		Name:   "waiting_mqtt",
		Task:   "db/append/table",
		Bridge: "mqtt_wait",
		Topic:  "topic/a",
	})
	require.NoError(t, err)
	require.NoError(t, waitingMqtt.Start())
	require.Equal(t, STARTING, waitingMqtt.Status())
	require.NoError(t, waitingMqtt.Stop())
	require.Equal(t, STOP, waitingMqtt.Status())

	require.NoError(t, bridge.Register(&model.BridgeDefinition{Type: model.BRIDGE_NATS, Name: "nats_sub"}))
	natsEntry, err := NewSubscriberEntry(&Service{}, &model.ScheduleDefinition{
		Name:      "nats_entry",
		Task:      "db/append/table",
		Bridge:    "nats_sub",
		Topic:     "topic/a",
		QueueName: "workers",
	})
	require.NoError(t, err)
	require.NoError(t, natsEntry.Start())
	require.Equal(t, FAILED, natsEntry.Status())
	require.Error(t, natsEntry.Error())
}

func TestSubscriberEntryTasksHandleTqlLoadError(t *testing.T) {
	wd, err := util.NewWriteDescriptor("task.tql")
	require.NoError(t, err)

	newEntry := func() *SubscriberEntry {
		return &SubscriberEntry{
			BaseEntry: NewBaseEntry("subscriber_task", RUNNING, false),
			TaskTql:   "task.tql",
			s:         &Service{tqlLoader: schedulerLoaderStub{err: errors.New("load failed")}},
			log:       logging.GetLog("subscriber-task-test"),
			wd:        wd,
		}
	}

	mqttEntry := newEntry()
	mqttRsp := &Reason{Reason: "not specified"}
	mqttEntry.doMqttTask("topic/a", []byte("payload"), 7, true, false)
	require.Equal(t, STOP, mqttEntry.Status())
	mqttEntry.doTql([]byte("payload"), map[string][]string{"TOPIC": {"topic/a"}}, mqttRsp)
	require.Equal(t, STOP, mqttEntry.Status())

	natsEntry := newEntry()
	natsEntry.doNatsTask(&nats.Msg{Subject: "topic/a", Data: []byte("payload")})
	require.Equal(t, STOP, natsEntry.Status())
}

func TestSubscriberEntryMqttOnConnectUnavailable(t *testing.T) {
	entry, err := NewSubscriberEntry(&Service{}, &model.ScheduleDefinition{
		Name:   "mqtt_connect",
		Task:   "db/append/table",
		Bridge: "mqtt",
		Topic:  "topic/a",
	})
	require.NoError(t, err)
	mqttBridge := bridge.NewMqttBridge("mqtt", "")

	entry.shouldSubscribe = false
	entry.doMqttOnConnect(mqttBridge)
	require.Equal(t, STOP, entry.Status())

	entry.shouldSubscribe = true
	entry.doMqttOnConnect(mqttBridge)
	require.Equal(t, FAILED, entry.Status())
	require.EqualError(t, entry.Error(), "mqtt connection is unavailable")
}
