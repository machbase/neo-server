package timer

import (
	"context"
	"errors"
	"testing"

	"github.com/machbase/neo-server/v8/mods/model"
	"github.com/machbase/neo-server/v8/mods/tql"
	"github.com/robfig/cron/v3"
)

type loaderStub struct{ err error }

func (stub loaderStub) Load(string) (*tql.Script, error) { return nil, stub.err }

type providerStub struct {
	timers      map[int64]*model.TimerDefinition
	loadErr     error
	savedScope  model.UserScope
	loadedScope model.UserScope
	lastError   string
}

func (stub *providerStub) LoadTimers(_ context.Context, scope model.UserScope) ([]*model.TimerDefinition, error) {
	stub.loadedScope = scope
	if stub.loadErr != nil {
		return nil, stub.loadErr
	}
	result := make([]*model.TimerDefinition, 0, len(stub.timers))
	for _, definition := range stub.timers {
		result = append(result, definition)
	}
	return result, nil
}
func (stub *providerStub) LoadAllTimers(context.Context) ([]*model.TimerDefinition, error) {
	if stub.loadErr != nil {
		return nil, stub.loadErr
	}
	result := make([]*model.TimerDefinition, 0, len(stub.timers))
	for _, definition := range stub.timers {
		result = append(result, definition)
	}
	return result, nil
}
func (stub *providerStub) LoadTimerForUser(_ context.Context, scope model.UserScope, id int64) (*model.TimerDefinition, error) {
	stub.loadedScope = scope
	definition := stub.timers[id]
	if definition == nil {
		return nil, errors.New("not found")
	}
	return definition, nil
}
func (stub *providerStub) SaveTimerForUser(_ context.Context, scope model.UserScope, definition *model.TimerDefinition) error {
	stub.savedScope = scope
	if definition.Id == 0 {
		definition.Id = int64(len(stub.timers) + 1)
	}
	stub.timers[definition.Id] = definition
	return nil
}
func (stub *providerStub) RemoveTimerForUser(_ context.Context, scope model.UserScope, id int64) error {
	stub.loadedScope = scope
	if stub.timers[id] == nil {
		return errors.New("not found")
	}
	delete(stub.timers, id)
	return nil
}
func (stub *providerStub) SetTimerRuntimeError(_ context.Context, _ int64, message string) error {
	stub.lastError = message
	return nil
}

func resetRegistry(t *testing.T) {
	t.Helper()
	unregisterAll()
	t.Cleanup(unregisterAll)
}

func newTestService(provider Provider, loader tql.Loader) *Service {
	return NewService(WithProvider(provider), WithTqlLoader(loader))
}

func TestTimerEntryValidationAndTaskLoadFailure(t *testing.T) {
	service := &Service{crons: cron.New(cron.WithSeconds()), tqlLoader: loaderStub{err: errors.New("load failed")}}
	for _, definition := range []*model.TimerDefinition{{Name: "schedule", Task: "task"}, {Name: "task", Schedule: "* * * * * *"}} {
		entry, err := NewTimerEntry(service, definition)
		if err != nil || entry.Start() == nil || entry.Status() != FAILED {
			t.Fatalf("invalid timer was accepted: %#v", definition)
		}
	}
	entry, _ := NewTimerEntry(service, &model.TimerDefinition{Name: "run", Task: "task", Schedule: "* * * * * *"})
	if err := entry.Start(); err != nil || entry.Status() != RUNNING {
		t.Fatalf("Start() = %v, %s", err, entry.Status())
	}
	entry.doTask()
	if entry.Status() != STOP {
		t.Fatalf("doTask load failure state = %s", entry.Status())
	}
}

func TestRegistryAndService(t *testing.T) {
	resetRegistry(t)
	provider := &providerStub{timers: map[int64]*model.TimerDefinition{}}
	service := newTestService(provider, loaderStub{})
	disabled := &model.TimerDefinition{Id: 1, Disabled: true}
	if err := service.Register(disabled); err != nil || GetEntry(1) != nil {
		t.Fatal("disabled timer was registered")
	}
	failing := &model.TimerDefinition{Id: 2, Name: "bad", Task: "bad", Schedule: "* * * * * *"}
	service.tqlLoader = loaderStub{err: errors.New("missing")}
	if err := service.Register(failing); err == nil || GetEntry(2).Status() != FAILED || provider.lastError == "" {
		t.Fatal("load failure was not recorded")
	}
	service.tqlLoader = loaderStub{}
	first := &model.TimerDefinition{Id: 3, Name: "one", Task: "task", Schedule: "* * * * * *"}
	if err := service.Register(first); err != nil || GetEntry(3) == nil {
		t.Fatalf("Register() = %v", err)
	}
	old := GetEntry(3)
	old.SetState(RUNNING)
	if err := service.Register(&model.TimerDefinition{Id: 3, Name: "two", Task: "task", Schedule: "* * * * * *"}); err != nil || GetEntry(3) == old {
		t.Fatal("running entry was not replaced")
	}
	Unregister(3)
	if GetEntry(3) != nil {
		t.Fatal("Unregister did not remove entry")
	}
	provider.timers[4] = &model.TimerDefinition{Id: 4, Name: "start", Task: "task", Schedule: "* * * * * *"}
	if err := service.Start(); err != nil {
		t.Fatalf("service Start() = %v", err)
	}
	service.Stop()
	service.Info("quiet", "entry", cron.EntryID(1))
	service.verbose = true
	service.Info("verbose", "entry", cron.EntryID(1), "next", service.crons.Entry(1).Next, "key", "value")
	service.Error(errors.New("expected"), "error")
}

func TestManagementUsesOwnerScopeAndSysExecUser(t *testing.T) {
	resetRegistry(t)
	provider := &providerStub{timers: map[int64]*model.TimerDefinition{}}
	service := newTestService(provider, loaderStub{})
	ctx := context.Background()
	user := model.UserScope{User: "alice"}
	bad, _ := service.Add(ctx, user, &AddRequest{})
	if bad.Success {
		t.Fatal("invalid add succeeded")
	}
	service.tqlLoader = loaderStub{err: errors.New("missing task")}
	missing, _ := service.Add(ctx, user, &AddRequest{Name: "missing", Task: "missing.tql", Schedule: "* * * * * *"})
	if missing.Success || len(provider.timers) != 0 {
		t.Fatal("missing task add was persisted")
	}
	service.tqlLoader = loaderStub{}
	added, _ := service.Add(ctx, user, &AddRequest{Name: "timer", Task: "task", Schedule: "* * * * * *", ExecUser: "bob"})
	if !added.Success || provider.timers[added.Id].ExecUser != "alice" || provider.savedScope != user {
		t.Fatal("user add did not retain owner scope")
	}
	sys := model.UserScope{User: "SYS"}
	added, _ = service.Add(ctx, sys, &AddRequest{Name: "sys", Task: "task", Schedule: "* * * * * *", ExecUser: "bob"})
	if provider.timers[added.Id].ExecUser != "bob" {
		t.Fatal("SYS exec user exception was lost")
	}
	list, _ := service.List(ctx, user)
	if !list.Success || provider.loadedScope != user {
		t.Fatal("List did not use scope")
	}
	got, _ := service.Get(ctx, user, 1)
	if !got.Success || got.Timer == nil {
		t.Fatal("Get failed")
	}
	invalid, _ := service.Update(ctx, user, &UpdateRequest{Id: 1, Schedule: "bad"})
	if invalid.Success {
		t.Fatal("invalid schedule update succeeded")
	}
	updated, _ := service.Update(ctx, sys, &UpdateRequest{Id: 1, Task: "changed", Schedule: "* * * * * *", ExecUser: "carol"})
	if !updated.Success || provider.timers[1].ExecUser != "carol" {
		t.Fatal("SYS update did not preserve exec user")
	}
	started, _ := service.StartTimer(ctx, user, 1)
	if !started.Success {
		t.Fatalf("StartTimer: %s", started.Reason)
	}
	stopped, _ := service.StopTimer(ctx, user, 1)
	if !stopped.Success {
		t.Fatalf("StopTimer: %s", stopped.Reason)
	}
	deleted, _ := service.Delete(ctx, user, 1)
	if !deleted.Success || GetEntry(1) != nil {
		t.Fatal("Delete failed")
	}
}
