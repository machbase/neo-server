package subscriber

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/machbase/neo-server/v8/mods/bridge"
	"github.com/machbase/neo-server/v8/mods/logging"
	"github.com/machbase/neo-server/v8/mods/model"
	"github.com/machbase/neo-server/v8/mods/tql"
	"github.com/machbase/neo-server/v8/mods/util"
	"github.com/machbase/neo-server/v8/mods/util/ssfs"
	"github.com/machbase/neo-server/v8/spi"
	"github.com/machbase/neo-server/v8/spi/machsvr"
	"github.com/nats-io/nats.go"
)

type subscriberLoaderStub struct{ err error }

func (stub subscriberLoaderStub) Load(string) (*tql.Script, error) { return nil, stub.err }

type subscriptionStub struct{}

func (subscriptionStub) Unsubscribe() error { return nil }
func (subscriptionStub) AddAppended(uint64) {}
func (subscriptionStub) AddInserted(uint64) {}

type failingSubscriptionStub struct{}

func (failingSubscriptionStub) Unsubscribe() error { return errors.New("unsubscribe failed") }
func (failingSubscriptionStub) AddAppended(uint64) {}
func (failingSubscriptionStub) AddInserted(uint64) {}

var subscriberTestServer *machsvr.TestServer

func TestMain(main *testing.M) {
	subscriberTestServer = &machsvr.TestServer{}
	subscriberTestServer.StartServer("./testsuite_tmp")
	tql.Init()
	code := main.Run()
	tql.Deinit()
	subscriberTestServer.StopServer()
	os.Exit(code)
}

type subscriberProviderStub struct {
	definitions map[int64]*model.SubscriberDefinition
	loadErr     error
	saveErr     error
	removeErr   error
	savedScope  model.UserScope
	loadedScope model.UserScope
	lastError   string
}

func (stub *subscriberProviderStub) LoadSubscribers(_ context.Context, scope model.UserScope) ([]*model.SubscriberDefinition, error) {
	stub.loadedScope = scope
	if stub.loadErr != nil {
		return nil, stub.loadErr
	}
	result := make([]*model.SubscriberDefinition, 0, len(stub.definitions))
	for _, definition := range stub.definitions {
		result = append(result, definition)
	}
	return result, nil
}
func (stub *subscriberProviderStub) LoadAllSubscribers(context.Context) ([]*model.SubscriberDefinition, error) {
	if stub.loadErr != nil {
		return nil, stub.loadErr
	}
	result := make([]*model.SubscriberDefinition, 0, len(stub.definitions))
	for _, definition := range stub.definitions {
		result = append(result, definition)
	}
	return result, nil
}
func (stub *subscriberProviderStub) LoadSubscriberForUser(_ context.Context, scope model.UserScope, id int64) (*model.SubscriberDefinition, error) {
	stub.loadedScope = scope
	definition := stub.definitions[id]
	if definition == nil {
		return nil, errors.New("not found")
	}
	return definition, nil
}
func (stub *subscriberProviderStub) SaveSubscriberForUser(_ context.Context, scope model.UserScope, definition *model.SubscriberDefinition) error {
	stub.savedScope = scope
	if stub.saveErr != nil {
		return stub.saveErr
	}
	if definition.Id == 0 {
		definition.Id = int64(len(stub.definitions) + 1)
	}
	stub.definitions[definition.Id] = definition
	return nil
}
func (stub *subscriberProviderStub) RemoveSubscriberForUser(_ context.Context, scope model.UserScope, id int64) error {
	stub.loadedScope = scope
	if stub.removeErr != nil {
		return stub.removeErr
	}
	if stub.definitions[id] == nil {
		return errors.New("not found")
	}
	delete(stub.definitions, id)
	return nil
}
func (stub *subscriberProviderStub) SetSubscriberRuntimeError(_ context.Context, _ int64, message string) error {
	stub.lastError = message
	return nil
}

type bridgeProviderStub struct {
	definitions map[string]*model.BridgeDefinition
	scope       model.UserScope
}

func (stub *bridgeProviderStub) LoadBridge(_ context.Context, scope model.UserScope, name string) (*model.BridgeDefinition, error) {
	stub.scope = scope
	definition := stub.definitions[name]
	if definition == nil {
		return nil, errors.New("bridge not found")
	}
	return definition, nil
}
func (*bridgeProviderStub) LoadAllBridgesForBootstrap(context.Context) ([]*model.BridgeDefinition, error) {
	return nil, nil
}
func (*bridgeProviderStub) LoadAllBridges(context.Context, model.UserScope) ([]*model.BridgeDefinition, error) {
	return nil, nil
}
func (*bridgeProviderStub) SaveBridge(context.Context, model.UserScope, *model.BridgeDefinition) error {
	return nil
}
func (*bridgeProviderStub) RemoveBridge(context.Context, model.UserScope, string) error { return nil }

func resetSubscriberRegistry(t *testing.T) {
	t.Helper()
	unregisterAll()
	t.Cleanup(unregisterAll)
}

func TestSubscriberEntryValidationAndTaskFailures(t *testing.T) {
	resetSubscriberRegistry(t)
	bridge.UnregisterAll()
	t.Cleanup(bridge.UnregisterAll)
	bridgeProvider := &bridgeProviderStub{definitions: map[string]*model.BridgeDefinition{}}
	bridge.SetBridgeProvider(bridgeProvider)
	service := NewService(WithTqlLoader(subscriberLoaderStub{err: errors.New("missing task")}))
	missing, _ := NewSubscriberEntry(service, &model.SubscriberDefinition{Name: "missing", Task: "task.tql", Bridge: "none", Topic: "topic"})
	if missing.Start() == nil || missing.Status() != FAILED || bridgeProvider.scope.User != "sys" {
		t.Fatal("missing bridge did not fail under legacy sys scope")
	}
	definition := &model.BridgeDefinition{Id: 20, Name: "mqtt", Type: model.BRIDGE_MQTT}
	bridgeProvider.definitions["mqtt"] = definition
	if err := bridge.RegisterByID(definition); err != nil {
		t.Fatal(err)
	}
	empty, _ := NewSubscriberEntry(service, &model.SubscriberDefinition{Name: "empty", Task: "task.tql", Bridge: "mqtt"})
	if empty.Start() == nil || empty.Status() != FAILED {
		t.Fatal("empty topic did not fail")
	}
	invalid, _ := NewSubscriberEntry(service, &model.SubscriberDefinition{Name: "invalid", Task: "unsupported", Bridge: "mqtt", Topic: "topic"})
	if invalid.Start() == nil {
		t.Fatal("invalid descriptor did not fail")
	}
	entry, _ := NewSubscriberEntry(service, &model.SubscriberDefinition{Name: "task", Task: "task.tql", Bridge: "mqtt", Topic: "topic", ExecUser: "alice"})
	entry.wd, _ = newWriteDescriptor(t, "task.tql")
	entry.doTql([]byte("payload"), map[string][]string{"TOPIC": {"topic"}}, &Reason{})
	if entry.Status() != STOP {
		t.Fatal("TQL load failure did not stop entry")
	}
	if err := entry.Stop(); err != nil || entry.Status() != STOP {
		t.Fatal("stop without subscription failed")
	}
}

func newWriteDescriptor(t *testing.T, task string) (*util.WriteDescriptor, error) {
	return util.NewWriteDescriptor(task)
}

func TestSubscriberRegistryServiceAndManagement(t *testing.T) {
	resetSubscriberRegistry(t)
	provider := &subscriberProviderStub{definitions: map[int64]*model.SubscriberDefinition{}}
	service := NewService(WithProvider(provider), WithTqlLoader(subscriberLoaderStub{}))
	if err := service.Register(&model.SubscriberDefinition{Id: 1, Disabled: true}); err != nil || GetEntry(1) != nil {
		t.Fatal("disabled subscriber was registered")
	}
	definition := &model.SubscriberDefinition{Id: 2, Name: "registered", Task: "task.tql", Bridge: "missing", Topic: "topic"}
	if err := service.Register(definition); err != nil || GetEntry(2) == nil {
		t.Fatalf("Register() = %v", err)
	}
	Unregister(2)
	if GetEntry(2) != nil {
		t.Fatal("Unregister did not remove entry")
	}
	provider.definitions[3] = &model.SubscriberDefinition{Id: 3, Name: "startup", Task: "task.tql", Bridge: "missing", Topic: "topic"}
	if err := service.Start(); err != nil {
		t.Fatalf("Start() = %v", err)
	}
	service.Stop()
	ctx := context.Background()
	user := model.UserScope{User: "alice"}
	bad, _ := service.Add(ctx, user, &AddRequest{})
	if bad.Success {
		t.Fatal("invalid add succeeded")
	}
	added, _ := service.Add(ctx, user, &AddRequest{Name: "sub", Task: "task.tql", Bridge: "missing", Topic: "topic", ExecUser: "bob"})
	if !added.Success || provider.definitions[added.Id].ExecUser != "alice" || provider.savedScope != user {
		t.Fatal("user scope was not preserved")
	}
	userID := added.Id
	sys := model.UserScope{User: "SYS"}
	added, _ = service.Add(ctx, sys, &AddRequest{Name: "sys", Task: "task.tql", Bridge: "missing", Topic: "topic", ExecUser: "bob"})
	if provider.definitions[added.Id].ExecUser != "bob" {
		t.Fatal("SYS exec user exception was lost")
	}
	list, _ := service.List(ctx, user)
	if !list.Success || provider.loadedScope != user {
		t.Fatal("List did not pass scope")
	}
	got, _ := service.Get(ctx, user, userID)
	if !got.Success || got.Subscriber == nil {
		t.Fatal("Get failed")
	}
	updated, _ := service.Update(ctx, sys, &UpdateRequest{Id: userID, Task: "new.tql", Bridge: "missing", Topic: "new", ExecUser: "carol"})
	if !updated.Success || provider.definitions[userID].ExecUser != "carol" {
		t.Fatal("SYS update failed")
	}
	started, _ := service.StartSubscriber(ctx, user, userID)
	if started.Success || started.Reason == "" {
		t.Fatal("StartSubscriber did not report the missing bridge")
	}
	stopped, _ := service.StopSubscriber(ctx, user, userID)
	if !stopped.Success {
		t.Fatalf("StopSubscriber: %s", stopped.Reason)
	}
	deleted, _ := service.Delete(ctx, user, userID)
	if !deleted.Success || GetEntry(userID) != nil {
		t.Fatal("Delete failed")
	}
}

func TestSubscriberWritesUseExecUser(t *testing.T) {
	ctx := t.Context()
	username := fmt.Sprintf("subscriber_test_%d", time.Now().UnixNano())
	table := "SUBSCRIBER_TEST"
	sysConn, err := spi.Connect(ctx, "sys")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = sysConn.ExecContext(ctx, fmt.Sprintf("CREATE USER %s IDENTIFIED BY 'password'", username)); err != nil {
		t.Fatal(err)
	}
	_ = sysConn.Close()
	t.Cleanup(func() {
		conn, err := spi.Connect(context.Background(), "sys")
		if err != nil {
			return
		}
		defer conn.Close()
		_, _ = conn.ExecContext(context.Background(), fmt.Sprintf("DROP TABLE %s.%s", username, table))
		_, _ = conn.ExecContext(context.Background(), "DROP USER "+username)
	})
	ownerConn, err := spi.Connect(ctx, username)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = ownerConn.ExecContext(ctx, fmt.Sprintf("CREATE TAG TABLE %s (NAME VARCHAR(40) PRIMARY KEY, TIME DATETIME BASETIME, VALUE DOUBLE SUMMARIZED)", table)); err != nil {
		t.Fatal(err)
	}
	_ = ownerConn.Close()

	insertDescriptor, err := util.NewWriteDescriptor("db/write/" + username + "." + table)
	if err != nil {
		t.Fatal(err)
	}
	insertEntry := &SubscriberEntry{BaseEntry: NewBaseEntry("insert", RUNNING, false), ExecUser: username, ctx: ctx, log: logging.GetLog("subscriber-insert-test"), wd: insertDescriptor, subscription: subscriptionStub{}}
	insertResponse := &Reason{}
	insertEntry.doInsert([]byte(fmt.Sprintf(`{"data":{"columns":["NAME","TIME","VALUE"],"rows":[["insert",%d,1.5]]}}`, time.Now().UnixNano())), insertResponse)
	if !insertResponse.Success {
		t.Fatal(insertResponse.Reason)
	}
	if insertEntry.conn == nil {
		t.Fatal("insert did not retain a connection")
	}
	_ = insertEntry.conn.Close()

	appendDescriptor, err := util.NewWriteDescriptor("db/append/" + username + "." + table)
	if err != nil {
		t.Fatal(err)
	}
	appendEntry := &SubscriberEntry{BaseEntry: NewBaseEntry("append", RUNNING, false), ExecUser: username, ctx: ctx, log: logging.GetLog("subscriber-append-test"), wd: appendDescriptor, subscription: subscriptionStub{}}
	appendResponse := &Reason{}
	appendEntry.doAppend([]byte(fmt.Sprintf(`{"data":{"rows":[["append",%d,2.5]]}}`, time.Now().UnixNano())), appendResponse)
	if !appendResponse.Success {
		t.Fatal(appendResponse.Reason)
	}
	if appendEntry.appender == nil {
		t.Fatal("append did not create an appender")
	}
	if err := appendEntry.appenderClose(); err != nil {
		t.Fatal(err)
	}

	defaultDescriptor, err := util.NewWriteDescriptor("db/write/" + username + "." + table)
	if err != nil {
		t.Fatal(err)
	}
	defaultEntry := &SubscriberEntry{BaseEntry: NewBaseEntry("default", RUNNING, false), ExecUser: username, ctx: ctx, log: logging.GetLog("subscriber-default-test"), wd: defaultDescriptor, subscription: subscriptionStub{}}
	defaultResponse := &Reason{}
	defaultEntry.doInsert([]byte(fmt.Sprintf(`{"data":{"rows":[["default",%d,3.5]]}}`, time.Now().UnixNano())), defaultResponse)
	if !defaultResponse.Success {
		t.Fatal(defaultResponse.Reason)
	}
	_ = defaultEntry.conn.Close()

	gzipDescriptor, err := util.NewWriteDescriptor("db/write/" + username + "." + table + ":json:gzip")
	if err != nil {
		t.Fatal(err)
	}
	gzipEntry := &SubscriberEntry{BaseEntry: NewBaseEntry("gzip", RUNNING, false), ExecUser: username, ctx: ctx, log: logging.GetLog("subscriber-gzip-test"), wd: gzipDescriptor, subscription: subscriptionStub{}}
	gzipResponse := &Reason{}
	gzipEntry.doInsert([]byte("not gzip"), gzipResponse)
	if gzipResponse.Reason == "" {
		t.Fatal("invalid gzip input was accepted")
	}
	_ = gzipEntry.conn.Close()

	appendGzipDescriptor, err := util.NewWriteDescriptor("db/append/" + username + "." + table + ":json:gzip")
	if err != nil {
		t.Fatal(err)
	}
	appendGzipEntry := &SubscriberEntry{BaseEntry: NewBaseEntry("append-gzip", RUNNING, false), ExecUser: username, ctx: ctx, log: logging.GetLog("subscriber-append-gzip-test"), wd: appendGzipDescriptor, subscription: subscriptionStub{}}
	appendGzipResponse := &Reason{}
	appendGzipEntry.doAppend([]byte("not gzip"), appendGzipResponse)
	if appendGzipResponse.Reason == "" {
		t.Fatal("invalid append gzip input was accepted")
	}
	if appendGzipEntry.appender != nil {
		_ = appendGzipEntry.appenderClose()
	}

	invalidColumnsEntry := &SubscriberEntry{BaseEntry: NewBaseEntry("columns", RUNNING, false), ExecUser: username, ctx: ctx, log: logging.GetLog("subscriber-columns-test"), wd: insertDescriptor, subscription: subscriptionStub{}}
	invalidColumnsResponse := &Reason{}
	invalidColumnsEntry.doInsert([]byte(`{"data":{"columns":["MISSING"],"rows":[[1]]}}`), invalidColumnsResponse)
	if invalidColumnsResponse.Reason == "" {
		t.Fatal("unknown column was accepted")
	}
	_ = invalidColumnsEntry.conn.Close()

	gzipPayload := gzipData(t, []byte(fmt.Sprintf(`{"data":{"rows":[["gzip",%d,4.5]]}}`, time.Now().UnixNano())))
	gzipSuccessEntry := &SubscriberEntry{BaseEntry: NewBaseEntry("gzip-success", RUNNING, false), ExecUser: username, ctx: ctx, log: logging.GetLog("subscriber-gzip-success-test"), wd: gzipDescriptor, subscription: subscriptionStub{}}
	gzipSuccessResponse := &Reason{}
	gzipSuccessEntry.doInsert(gzipPayload, gzipSuccessResponse)
	if !gzipSuccessResponse.Success {
		t.Fatal(gzipSuccessResponse.Reason)
	}
	_ = gzipSuccessEntry.conn.Close()

	appendGzipSuccessEntry := &SubscriberEntry{BaseEntry: NewBaseEntry("append-gzip-success", RUNNING, false), ExecUser: username, ctx: ctx, log: logging.GetLog("subscriber-append-gzip-success-test"), wd: appendGzipDescriptor, subscription: subscriptionStub{}}
	appendGzipSuccessResponse := &Reason{}
	appendGzipSuccessEntry.doAppend(gzipPayload, appendGzipSuccessResponse)
	if !appendGzipSuccessResponse.Success {
		t.Fatal(appendGzipSuccessResponse.Reason)
	}
	if err := appendGzipSuccessEntry.appenderClose(); err != nil {
		t.Fatal(err)
	}

	unsupportedInsertDescriptor := *insertDescriptor
	unsupportedInsertDescriptor.Format = "unsupported"
	unsupportedInsertEntry := &SubscriberEntry{BaseEntry: NewBaseEntry("unsupported-insert", RUNNING, false), ExecUser: username, ctx: ctx, log: logging.GetLog("subscriber-unsupported-insert-test"), wd: &unsupportedInsertDescriptor, subscription: subscriptionStub{}}
	unsupportedInsertResponse := &Reason{}
	unsupportedInsertEntry.doInsert([]byte(`[]`), unsupportedInsertResponse)
	if unsupportedInsertResponse.Reason == "" {
		t.Fatal("unsupported insert codec was accepted")
	}
	_ = unsupportedInsertEntry.conn.Close()

	unsupportedAppendDescriptor := *appendDescriptor
	unsupportedAppendDescriptor.Format = "unsupported"
	unsupportedAppendEntry := &SubscriberEntry{BaseEntry: NewBaseEntry("unsupported-append", RUNNING, false), ExecUser: username, ctx: ctx, log: logging.GetLog("subscriber-unsupported-append-test"), wd: &unsupportedAppendDescriptor, subscription: subscriptionStub{}}
	unsupportedAppendResponse := &Reason{}
	unsupportedAppendEntry.doAppend([]byte(`[]`), unsupportedAppendResponse)
	if unsupportedAppendResponse.Reason == "" {
		t.Fatal("unsupported append codec was accepted")
	}
	if unsupportedAppendEntry.appender != nil {
		_ = unsupportedAppendEntry.appenderClose()
	}

	missingTableDescriptor, err := util.NewWriteDescriptor("db/write/" + username + ".MISSING")
	if err != nil {
		t.Fatal(err)
	}
	missingTableEntry := &SubscriberEntry{BaseEntry: NewBaseEntry("missing-table", RUNNING, false), ExecUser: username, ctx: ctx, log: logging.GetLog("subscriber-missing-table-test"), wd: missingTableDescriptor, subscription: subscriptionStub{}}
	missingTableResponse := &Reason{}
	missingTableEntry.doInsert([]byte(`[]`), missingTableResponse)
	if missingTableResponse.Reason == "" {
		t.Fatal("missing table was accepted")
	}
	_ = missingTableEntry.conn.Close()

	checkConn, err := spi.Connect(ctx, "sys")
	if err != nil {
		t.Fatal(err)
	}
	defer checkConn.Close()
	var count int
	if err := checkConn.QueryRowContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s.%s", username, table)).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 5 {
		t.Fatalf("row count = %d, want 5", count)
	}
}

func gzipData(t *testing.T, data []byte) []byte {
	t.Helper()
	var output bytes.Buffer
	writer := gzip.NewWriter(&output)
	if _, err := writer.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

func TestSubscriberTqlAndColumnHelpers(t *testing.T) {
	if columns := extractColumns([]byte(`{"data":{"columns":["one","two"]}}`)); strings.Join(columns, ",") != "one,two" {
		t.Fatalf("columns = %v", columns)
	}
	if columns := extractColumns([]byte(`{}`)); columns != nil {
		t.Fatalf("columns = %v", columns)
	}
	descriptor, err := util.NewWriteDescriptor("task.tql")
	if err != nil {
		t.Fatal(err)
	}
	if !descriptor.IsTqlDestination() {
		t.Fatal("TQL destination was not recognized")
	}
}

func TestSubscriberTaskWrappersAndSubscriptionStates(t *testing.T) {
	bridge.UnregisterAll()
	t.Cleanup(bridge.UnregisterAll)
	bridgeProvider := &bridgeProviderStub{definitions: map[string]*model.BridgeDefinition{"mqtt": {Id: 30, Name: "mqtt", Type: model.BRIDGE_MQTT}}}
	bridge.SetBridgeProvider(bridgeProvider)
	if err := bridge.RegisterByID(bridgeProvider.definitions["mqtt"]); err != nil {
		t.Fatal(err)
	}
	loader := subscriberLoaderStub{err: errors.New("missing")}
	descriptor, err := util.NewWriteDescriptor("task.tql")
	if err != nil {
		t.Fatal(err)
	}
	entry := &SubscriberEntry{BaseEntry: NewBaseEntry("wrapper", RUNNING, false), TaskTql: "task.tql", Bridge: "mqtt", Topic: "topic", ctx: context.Background(), log: logging.GetLog("subscriber-wrapper-test"), wd: descriptor, s: NewService(WithTqlLoader(loader))}
	entry.doMqttTask("topic", []byte("payload"), 1, true, false)
	if entry.Status() != STOP {
		t.Fatal("MQTT TQL failure did not stop entry")
	}
	entry.SetState(RUNNING)
	entry.doNatsTask(&nats.Msg{Subject: "topic", Data: []byte("payload")})
	if entry.Status() != STOP {
		t.Fatal("NATS TQL failure did not stop entry")
	}
	mqtt, err := bridge.GetMqttBridge(context.Background(), model.UserScope{User: "sys"}, "mqtt")
	if err != nil {
		t.Fatal(err)
	}
	entry.SetState(STOP)
	entry.shouldSubscribe = false
	entry.doMqttOnConnect(mqtt)
	if entry.Status() != STOP {
		t.Fatal("inactive MQTT entry changed state")
	}
	entry.shouldSubscribe = true
	entry.doMqttOnConnect(mqtt)
	if entry.Status() != FAILED || entry.Error() == nil {
		t.Fatal("unavailable MQTT connection was not reported")
	}
	entry.subscription = subscriptionStub{}
	if err := entry.Stop(); err != nil || entry.Status() != STOP {
		t.Fatalf("Stop() = %v, %s", err, entry.Status())
	}
	disconnected, _ := NewSubscriberEntry(NewService(), &model.SubscriberDefinition{Name: "disconnected", Task: "task.tql", Bridge: "mqtt", Topic: "topic"})
	if err := disconnected.startMqtt(mqtt); err != nil {
		t.Fatalf("startMqtt() = %v", err)
	}
	natsEntry, _ := NewSubscriberEntry(NewService(), &model.SubscriberDefinition{Name: "nats", Task: "task.tql", Bridge: "nats", Topic: "topic"})
	if err := natsEntry.startNats(bridge.NewNatsBridge("nats", "")); err != nil || natsEntry.Status() != FAILED {
		t.Fatalf("startNats() = %v, %s", err, natsEntry.Status())
	}
}

func TestSubscriberStopAndRegisterErrors(t *testing.T) {
	resetSubscriberRegistry(t)
	bridge.UnregisterAll()
	t.Cleanup(bridge.UnregisterAll)
	provider := &subscriberProviderStub{definitions: map[int64]*model.SubscriberDefinition{}}
	service := NewService(WithProvider(provider))

	missingBridge := &SubscriberEntry{BaseEntry: NewBaseEntry("missing", RUNNING, false), Bridge: "missing", ctx: context.Background(), log: logging.GetLog("subscriber-stop-missing-test"), subscription: subscriptionStub{}}
	if err := missingBridge.Stop(); err == nil || missingBridge.Status() != FAILED {
		t.Fatal("Stop succeeded without its bridge")
	}

	definition := &model.BridgeDefinition{Id: 31, Name: "mqtt", Type: model.BRIDGE_MQTT}
	if err := bridge.RegisterByID(definition); err != nil {
		t.Fatal(err)
	}
	failingUnsubscribe := &SubscriberEntry{BaseEntry: NewBaseEntry("unsubscribe", RUNNING, false), Bridge: "mqtt", ctx: context.Background(), log: logging.GetLog("subscriber-stop-unsubscribe-test"), subscription: failingSubscriptionStub{}}
	if err := failingUnsubscribe.Stop(); err == nil || failingUnsubscribe.Status() != FAILED {
		t.Fatal("Stop ignored an unsubscribe failure")
	}

	old := &SubscriberEntry{BaseEntry: NewBaseEntry("old", RUNNING, false), Bridge: "mqtt", ctx: context.Background(), log: logging.GetLog("subscriber-register-old-test"), subscription: subscriptionStub{}}
	service.replace(42, old)
	if err := service.Register(&model.SubscriberDefinition{Id: 42, Name: "new", Task: "task.tql", Bridge: "missing", Topic: "topic"}); err == nil {
		t.Fatal("Register succeeded when replacing a running subscriber with a missing bridge")
	}
	if provider.lastError == "" {
		t.Fatal("Register did not record its runtime error")
	}
}

func TestSubscriberManagementErrorsAndDatabaseTaskWrappers(t *testing.T) {
	resetSubscriberRegistry(t)
	provider := &subscriberProviderStub{definitions: map[int64]*model.SubscriberDefinition{1: {Id: 1}}, loadErr: errors.New("load failed")}
	service := NewService(WithProvider(provider))
	ctx := context.Background()
	if response, _ := service.List(ctx, model.UserScope{}); response.Success || response.Reason == "" {
		t.Fatal("List did not report provider failure")
	}
	if response, _ := service.Get(ctx, model.UserScope{}, 99); response.Success || response.Reason == "" {
		t.Fatal("Get did not report provider failure")
	}
	if err := service.Start(); err == nil {
		t.Fatal("Start did not return provider failure")
	}

	provider.loadErr = nil
	provider.saveErr = errors.New("save failed")
	if response, _ := service.Add(ctx, model.UserScope{User: "alice"}, &AddRequest{Name: "sub", Task: "task.tql", Bridge: "bridge", Topic: "topic"}); response.Success || response.Reason == "" {
		t.Fatal("Add did not report save failure")
	}
	provider.saveErr = nil
	provider.removeErr = errors.New("remove failed")
	if response, _ := service.Delete(ctx, model.UserScope{}, 1); response.Success || response.Reason == "" {
		t.Fatal("Delete did not report remove failure")
	}
	if response, _ := service.StartSubscriber(ctx, model.UserScope{}, 99); response.Success || response.Reason == "" {
		t.Fatal("StartSubscriber did not report lookup failure")
	}
	if response, _ := service.StopSubscriber(ctx, model.UserScope{}, 99); response.Success || response.Reason == "" {
		t.Fatal("StopSubscriber did not report lookup failure")
	}
	if response, _ := service.StartSubscriber(ctx, model.UserScope{}, 1); response.Success || response.Reason == "" {
		t.Fatal("StartSubscriber did not report an unregistered subscriber")
	}
	if response, _ := service.StopSubscriber(ctx, model.UserScope{}, 1); response.Success || response.Reason == "" {
		t.Fatal("StopSubscriber did not report an unregistered subscriber")
	}
	provider.removeErr = nil
	provider.saveErr = errors.New("update failed")
	if response, _ := service.Update(ctx, model.UserScope{}, &UpdateRequest{Id: 1}); response.Success || response.Reason == "" {
		t.Fatal("Update did not report save failure")
	}

	baseEntry := NewBaseEntry("runtime", STOP, false)
	baseEntry.setError(errors.New("runtime failure"))
	if baseEntry.Error() == nil {
		t.Fatal("setError did not preserve the runtime error")
	}

	insertDescriptor, err := util.NewWriteDescriptor("db/write/MISSING")
	if err != nil {
		t.Fatal(err)
	}
	insertEntry := &SubscriberEntry{BaseEntry: NewBaseEntry("mqtt-write", RUNNING, false), TaskTql: "db/write/MISSING", ExecUser: "not-a-user", ctx: ctx, log: logging.GetLog("subscriber-wrapper-write-test"), wd: insertDescriptor}
	insertEntry.doMqttTask("topic", []byte(`[]`), 9, true, true)

	appendDescriptor, err := util.NewWriteDescriptor("db/append/MISSING")
	if err != nil {
		t.Fatal(err)
	}
	appendEntry := &SubscriberEntry{BaseEntry: NewBaseEntry("nats-append", RUNNING, false), TaskTql: "db/append/MISSING", ExecUser: "not-a-user", ctx: ctx, log: logging.GetLog("subscriber-wrapper-append-test"), wd: appendDescriptor}
	appendEntry.doNatsTask(&nats.Msg{Subject: "topic", Data: []byte(`[]`)})
}

func TestSubscriberTqlCompileFailure(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "bad.tql"), []byte("THIS IS NOT TQL"), 0644); err != nil {
		t.Fatal(err)
	}
	fsys, err := ssfs.NewServerSideFileSystem([]string{"/=" + dir})
	if err != nil {
		t.Fatal(err)
	}
	previous := ssfs.Default()
	ssfs.SetDefault(fsys)
	t.Cleanup(func() { ssfs.SetDefault(previous) })
	descriptor, err := util.NewWriteDescriptor("bad.tql")
	if err != nil {
		t.Fatal(err)
	}
	entry := &SubscriberEntry{BaseEntry: NewBaseEntry("bad", RUNNING, false), TaskTql: "bad.tql", ctx: context.Background(), log: logging.GetLog("subscriber-compile-test"), wd: descriptor, s: NewService(WithTqlLoader(tql.NewLoader()))}
	entry.doTql([]byte("payload"), map[string][]string{}, &Reason{})
}
