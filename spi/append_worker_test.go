package spi

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	client "github.com/machbase/neo-client/v2"
	"github.com/machbase/neo-server/v8/mods/logging"
	"github.com/stretchr/testify/require"
)

func newAppendWorkerForTest(tableName string) *AppendWorker {
	ctx, cancel := context.WithCancel(context.Background())
	return &AppendWorker{
		ctx:       ctx,
		ctxCancel: cancel,
		appender:  &client.Appender{},
		tableDesc: &TableDescription{Name: tableName},
		lastTime:  time.Now(),
		log:       logging.GetLog("append-worker-test"),
	}
}

func TestAppendWorkerRegistryStopsByLowerCaseName(t *testing.T) {
	StartAppendWorkers()
	t.Cleanup(StopAppendWorkers)

	worker := newAppendWorkerForTest("sensor")
	appendersLock.Lock()
	appenders["machbasedb.sys.sensor"] = worker
	appendersLock.Unlock()

	ack := StopAppendWorker("", "", "SENSOR")
	select {
	case <-ack:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for append worker stop ack")
	}

	appendersLock.Lock()
	_, exists := appenders["machbasedb.sys.sensor"]
	appendersLock.Unlock()
	require.False(t, exists)
}

func TestFlushAppendWorkersMatchesNamesCaseInsensitively(t *testing.T) {
	StartAppendWorkers()
	t.Cleanup(StopAppendWorkers)

	sensor := newAppendWorkerForTest("sensor")
	metric := newAppendWorkerForTest("metric")
	appendersLock.Lock()
	appenders["machbasedb.sys.sensor"] = sensor
	appenders["machbasedb.sys.metric"] = metric
	appendersLock.Unlock()

	FlushAppendWorkers("", "", "SENSOR")

	appendersLock.Lock()
	_, sensorExists := appenders["machbasedb.sys.sensor"]
	_, metricExists := appenders["machbasedb.sys.metric"]
	appendersLock.Unlock()
	require.False(t, sensorExists)
	require.True(t, metricExists)

	FlushAppendWorkers("", "")
	require.Empty(t, appenders)
}

func TestGetAppendWorkerReusesRegisteredWorkerCaseInsensitively(t *testing.T) {
	StartAppendWorkers()
	t.Cleanup(StopAppendWorkers)

	worker := newAppendWorkerForTest("sensor")
	appendersLock.Lock()
	appenders["machbasedb.sys.sensor"] = worker
	appendersLock.Unlock()

	got, err := GetAppendWorker(context.Background(), "", "", "SENSOR")
	require.NoError(t, err)
	require.Same(t, worker, got)
	require.Equal(t, int32(1), atomic.LoadInt32(&got.refCount))
}

func TestAppendWorkerKeyNormalization(t *testing.T) {
	require.Equal(t, "machbasedb.sys.sensor", appendWorkerKey("", "", "sensor"))
	require.Equal(t, "machbasedb.sys.sensor", appendWorkerKey("", "", "SYS.SENSOR"))
	require.Equal(t, "machbasedb.alice.sensor", appendWorkerKey("", "", "MACHBASEDB.ALICE.SENSOR"))
	require.Equal(t, "machbasedb.alice.sensor", appendWorkerKey("", "alice", "sensor"))
	require.Equal(t, "testdb.alice.sensor", appendWorkerKey("testdb", "", "alice.sensor"))
	// explicit db/user take precedence over any qualifier embedded in table
	require.Equal(t, "machbasedb.bob.sensor", appendWorkerKey("", "bob", "alice.sensor"))

	db, user, table := splitAppendWorkerName("testdb.alice.sensor")
	require.Equal(t, "testdb", db)
	require.Equal(t, "alice", user)
	require.Equal(t, "sensor", table)

	db, user, table = splitAppendWorkerName("alice.sensor")
	require.Empty(t, db)
	require.Equal(t, "alice", user)
	require.Equal(t, "sensor", table)

	db, user, table = splitAppendWorkerName("sensor")
	require.Empty(t, db)
	require.Empty(t, user)
	require.Equal(t, "sensor", table)
}

func TestAppenderWithWorkerInputColumnsUpperCase(t *testing.T) {
	worker := newAppendWorkerForTest("sensor")

	wrapped := worker.WithInputColumns("value", "name")
	withWorker, ok := wrapped.(*AppenderWithWorker)
	require.True(t, ok)
	require.Len(t, withWorker.inputColumns, 2)
	require.Equal(t, "VALUE", withWorker.inputColumns[0].Name)
	require.Equal(t, "NAME", withWorker.inputColumns[1].Name)
	require.Equal(t, -1, withWorker.inputColumns[0].Idx)
	require.Equal(t, -1, withWorker.inputColumns[1].Idx)
}

func TestAppendWorkerAppendLogTimeRequiresLogTable(t *testing.T) {
	worker := newAppendWorkerForTest("sensor")
	worker.appendC = make(chan []interface{}, 1)
	ts := time.Unix(1, 2)
	err := worker.AppendLogTime(ts, "temperature", 3.14)
	require.EqualError(t, err, " is not a log table, use Append() instead")
}

func TestAppendWorkerAppenderAccessorsAndNoopOptions(t *testing.T) {
	worker := newAppendWorkerForTest("sensor")

	require.Equal(t, "", worker.TableName())
	require.Equal(t, client.TableType(-1), worker.TableType())
	columns := worker.Columns()
	require.Nil(t, columns)
	require.Same(t, worker, worker.WithInputFormats("json"))
	require.Same(t, worker, worker.WithBatchMaxRows(100))
	require.Same(t, worker, worker.WithBatchMaxBytes(1024))
	require.Same(t, worker, worker.WithBatchMaxDelay(time.Second))

	success, fail, err := worker.Close()
	require.NoError(t, err)
	require.Zero(t, success)
	require.Zero(t, fail)
	require.Equal(t, int32(-1), atomic.LoadInt32(&worker.refCount))
}

func TestAppendWorkerStartAndAppenderWithWorkerAppendLogTime(t *testing.T) {
	worker := newAppendWorkerForTest("sensor")

	worker.Start()
	require.NoError(t, worker.Append("temperature", 3.14))
	worker.Stop()
	require.Nil(t, worker.appendC)

	worker2 := newAppendWorkerForTest("sensor")
	worker2.appendC = make(chan []interface{}, 1)
	wrapped := worker2.WithInputColumns("NAME", "VALUE")
	sts := time.Unix(10, 0)
	err := wrapped.AppendLogTime(sts, "tagB", 11.0)
	require.EqualError(t, err, " is not a log table, use Append() instead")
}
