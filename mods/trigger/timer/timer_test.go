package timer

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/machbase/neo-server/v8/mods/logging"
	"github.com/machbase/neo-server/v8/mods/model"
	"github.com/machbase/neo-server/v8/mods/tql"
	"github.com/robfig/cron/v3"
)

func TestMain(m *testing.M) {
	tql.Init()
	code := m.Run()
	tql.Deinit()
	os.Exit(code)
}

type loaderStub struct{ err error }

func (stub loaderStub) Load(string) (*tql.Script, error) { return nil, stub.err }

type providerStub struct {
	timers      map[int64]*model.TimerDefinition
	loadErr     error
	savedScope  model.UserScope
	loadedScope model.UserScope
	lastError   string
}

type testScriptFS struct{ content string }

func (fs testScriptFS) OpenFile(string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader(fs.content)), nil
}
func (testScriptFS) Stat(string) (os.FileInfo, error) { return testFileInfo{}, nil }
func (testScriptFS) RootDir() (string, bool)          { return "", false }

type testFileInfo struct{}

func (testFileInfo) Name() string       { return "test.tql" }
func (testFileInfo) Size() int64        { return 0 }
func (testFileInfo) Mode() os.FileMode  { return 0 }
func (testFileInfo) ModTime() time.Time { return time.Time{} }
func (testFileInfo) IsDir() bool        { return false }
func (testFileInfo) Sys() any           { return nil }

type recordingProvider struct {
	*providerStub
	errors   chan string
	storeErr error
}

func (p *recordingProvider) SetTimerRuntimeError(_ context.Context, _ int64, message string) error {
	p.errors <- message
	return p.storeErr
}

type failingAfterRegistrationLoader struct {
	script *tql.Script
	calls  atomic.Int32
}

func (l *failingAfterRegistrationLoader) Load(string) (*tql.Script, error) {
	if l.calls.Add(1) == 1 {
		return l.script, nil
	}
	return nil, errors.New("load failed")
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
}

func TestTaskFailuresAreRecorded(t *testing.T) {
	for _, tc := range []struct {
		name string
		code string
	}{
		{"compile", "NOT_A_NODE()"},
		{"execute", "FAKE(1)\nCSV()"},
	} {
		for _, keepRunning := range []bool{false, true} {
			t.Run(tc.name+map[bool]string{false: "_stop", true: "_continue"}[keepRunning], func(t *testing.T) {
				resetRegistry(t)
				provider := &recordingProvider{providerStub: &providerStub{}, errors: make(chan string, 2)}
				loader := tql.NewLoader(testScriptFS{content: tc.code})
				service := newTestService(provider, loader)
				def := &model.TimerDefinition{Id: 99, Name: "test", Task: "test.tql", Schedule: "@every 1m", ContinueOnError: keepRunning}
				if err := service.Register(def); err != nil {
					t.Fatal(err)
				}
				ent := GetEntry(def.Id)
				if err := ent.Start(); err != nil {
					t.Fatal(err)
				}
				ent.doTask()
				select {
				case message := <-provider.errors:
					if message == "" {
						t.Fatal("empty error")
					}
				default:
					t.Fatal("error not recorded")
				}
				if keepRunning && ent.Status() != RUNNING || !keepRunning && ent.Status() != STOP {
					t.Fatalf("unexpected state %s", ent.Status())
				}
			})
		}
	}
}

func TestContinueOnErrorRunsNextSchedule(t *testing.T) {
	script, err := tql.NewLoader(testScriptFS{content: "FAKE(1)\nCSV()"}).Load("test.tql")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		loader tql.Loader
	}{
		{"load", &failingAfterRegistrationLoader{script: script}},
		{"compile", tql.NewLoader(testScriptFS{content: "NOT_A_NODE()"})},
		{"execute", tql.NewLoader(testScriptFS{content: "FAKE(1)\nCSV()"})},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resetRegistry(t)
			provider := &recordingProvider{providerStub: &providerStub{}, errors: make(chan string, 4)}
			service := newTestService(provider, tc.loader)
			def := &model.TimerDefinition{Id: 55, Name: "test", Task: "test.tql", Schedule: "@every 1s", ContinueOnError: true}
			if err := service.Register(def); err != nil {
				t.Fatal(err)
			}
			ent := GetEntry(def.Id)
			if err := ent.Start(); err != nil {
				t.Fatal(err)
			}
			service.crons.Start()
			t.Cleanup(func() { <-service.crons.Stop().Done() })
			for i := 0; i < 2; i++ {
				select {
				case message := <-provider.errors:
					if message == "" {
						t.Fatal("empty error")
					}
				case <-time.After(4 * time.Second):
					t.Fatal("next scheduled execution did not happen")
				}
			}
			if ent.Status() != RUNNING {
				t.Fatalf("unexpected state %s", ent.Status())
			}
			if err := ent.Stop(); err != nil {
				t.Fatal(err)
			}
			select {
			case message := <-provider.errors:
				t.Fatalf("execution after stop: %s", message)
			case <-time.After(1200 * time.Millisecond):
			}
			if ent.Status() != STOP {
				t.Fatalf("timer restarted after manual stop: %s", ent.Status())
			}
		})
	}
}

func TestOldFailureDoesNotStopRestartedTimer(t *testing.T) {
	service := newTestService(nil, loaderStub{})
	ent, err := NewTimerEntry(service, &model.TimerDefinition{Name: "test", Task: "test.tql", Schedule: "@every 1m"})
	if err != nil {
		t.Fatal(err)
	}
	if err := ent.Start(); err != nil {
		t.Fatal(err)
	}
	generation := ent.generation
	if err := ent.Stop(); err != nil {
		t.Fatal(err)
	}
	if err := ent.Start(); err != nil {
		t.Fatal(err)
	}
	ent.handleTaskFailure(generation, errors.New("old failure"))
	if ent.Status() != RUNNING {
		t.Fatal("old failure stopped the restarted timer")
	}
	if err := ent.Stop(); err != nil {
		t.Fatal(err)
	}
}

func TestTaskErrorPersistenceFailureAndStaleEntry(t *testing.T) {
	resetRegistry(t)
	provider := &recordingProvider{providerStub: &providerStub{}, errors: make(chan string, 2), storeErr: errors.New("database unavailable")}
	service := newTestService(provider, loaderStub{})
	def := &model.TimerDefinition{Id: 55, Name: "test", Task: "test.tql", Schedule: "@every 1m", ContinueOnError: true}
	if err := service.Register(def); err != nil {
		t.Fatal(err)
	}
	ent := GetEntry(def.Id)
	var output bytes.Buffer
	ent.log = logging.NewLog("timer-test", &output)
	ent.recordTaskError(errors.New(strings.Repeat("한", 500)))
	message := <-provider.errors
	if len(message) > 1024 || !utf8.ValidString(message) || message == "" {
		t.Fatalf("invalid truncated error: %q", message)
	}
	if !strings.Contains(output.String(), "database unavailable") {
		t.Fatalf("persistence failure not logged: %s", output.String())
	}
	if err := service.Register(def); err != nil {
		t.Fatal(err)
	}
	ent.recordTaskError(errors.New("stale error"))
	select {
	case message := <-provider.errors:
		t.Fatalf("replaced entry wrote error: %s", message)
	default:
	}
}

func TestRegistryReplacementAndService(t *testing.T) {
	resetRegistry(t)
	provider := &providerStub{timers: map[int64]*model.TimerDefinition{}}
	service := newTestService(provider, loaderStub{})
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
	current := GetEntry(3)
	service.tqlLoader = loaderStub{err: errors.New("missing replacement")}
	if err := service.Register(&model.TimerDefinition{Id: 3, Name: "bad replacement", Task: "missing", Schedule: "* * * * * *"}); err == nil || current.Status() != STOP {
		t.Fatal("failed replacement did not stop the old timer")
	}
	service.tqlLoader = loaderStub{}
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
	added, _ := service.Add(ctx, user, &AddRequest{Name: "timer", Task: "task", Schedule: "* * * * * *", ExecUser: "bob", ContinueOnError: true})
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
	if !got.Timer.ContinueOnError {
		t.Fatal("Get lost continueOnError")
	}
	invalid, _ := service.Update(ctx, user, &UpdateRequest{Id: 1, Schedule: "bad"})
	if invalid.Success {
		t.Fatal("invalid schedule update succeeded")
	}
	updated, _ := service.Update(ctx, sys, &UpdateRequest{Id: 1, Task: "changed", Schedule: "* * * * * *", ExecUser: "carol"})
	if !updated.Success || provider.timers[1].ExecUser != "carol" {
		t.Fatal("SYS update did not preserve exec user")
	}
	if !provider.timers[1].ContinueOnError {
		t.Fatal("omitted continueOnError was reset")
	}
	disable := false
	updated, _ = service.Update(ctx, sys, &UpdateRequest{Id: 1, Task: "changed", Schedule: "* * * * * *", ContinueOnError: &disable})
	if !updated.Success || provider.timers[1].ContinueOnError {
		t.Fatal("explicit continueOnError=false was not saved")
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
