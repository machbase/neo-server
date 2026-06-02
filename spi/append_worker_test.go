package spi

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	clientapi "github.com/machbase/neo-client/api"
	"github.com/machbase/neo-server/v8/mods/logging"
	"github.com/stretchr/testify/require"
)

type appendWorkerTestConn struct {
	closed int32
}

func (c *appendWorkerTestConn) Close() error {
	atomic.AddInt32(&c.closed, 1)
	return nil
}

func (c *appendWorkerTestConn) Exec(context.Context, string, ...any) clientapi.Result {
	return nil
}

func (c *appendWorkerTestConn) Query(context.Context, string, ...any) (clientapi.Rows, error) {
	return nil, nil
}

func (c *appendWorkerTestConn) QueryRow(context.Context, string, ...any) clientapi.Row {
	return nil
}

func (c *appendWorkerTestConn) Prepare(context.Context, string) (clientapi.Stmt, error) {
	return nil, nil
}

func (c *appendWorkerTestConn) Appender(context.Context, string, ...clientapi.AppenderOption) (clientapi.Appender, error) {
	return nil, nil
}

func (c *appendWorkerTestConn) Explain(context.Context, string, bool) (string, error) {
	return "", nil
}

type appendWorkerTestAppender struct {
	tableName  string
	tableType  clientapi.TableType
	columns    clientapi.Columns
	appendRows [][]any
	closed     int32
}

func (a *appendWorkerTestAppender) TableName() string {
	return a.tableName
}

func (a *appendWorkerTestAppender) Append(values ...any) error {
	a.appendRows = append(a.appendRows, values)
	return nil
}

func (a *appendWorkerTestAppender) AppendLogTime(ts time.Time, values ...any) error {
	row := append([]any{ts}, values...)
	a.appendRows = append(a.appendRows, row)
	return nil
}

func (a *appendWorkerTestAppender) Close() (int64, int64, error) {
	atomic.AddInt32(&a.closed, 1)
	return int64(len(a.appendRows)), 0, nil
}

func (a *appendWorkerTestAppender) Columns() (clientapi.Columns, error) {
	return a.columns, nil
}

func (a *appendWorkerTestAppender) TableType() clientapi.TableType {
	return a.tableType
}

func (a *appendWorkerTestAppender) WithInputColumns(...string) clientapi.Appender {
	return a
}

func (a *appendWorkerTestAppender) WithInputFormats(...string) clientapi.Appender {
	return a
}

func (a *appendWorkerTestAppender) WithBatchMaxRows(int) clientapi.Appender {
	return a
}

func (a *appendWorkerTestAppender) WithBatchMaxBytes(int) clientapi.Appender {
	return a
}

func (a *appendWorkerTestAppender) WithBatchMaxDelay(time.Duration) clientapi.Appender {
	return a
}

func newAppendWorkerForTest(tableName string) (*AppendWorker, *appendWorkerTestAppender, *appendWorkerTestConn) {
	ctx, cancel := context.WithCancel(context.Background())
	appender := &appendWorkerTestAppender{
		tableName: tableName,
		tableType: clientapi.TableTypeLog,
		columns: clientapi.Columns{
			{Name: "NAME", DataType: clientapi.DataTypeString},
			{Name: "VALUE", DataType: clientapi.DataTypeFloat64},
		},
	}
	conn := &appendWorkerTestConn{}
	return &AppendWorker{
		ctx:       ctx,
		ctxCancel: cancel,
		conn:      conn,
		appender:  appender,
		tableDesc: &clientapi.TableDescription{Name: tableName},
		lastTime:  time.Now(),
		log:       logging.GetLog("append-worker-test"),
	}, appender, conn
}

func TestAppendWorkerRegistryStopsByLowerCaseName(t *testing.T) {
	StartAppendWorkers()
	t.Cleanup(StopAppendWorkers)

	worker, appender, conn := newAppendWorkerForTest("sensor")
	appendersLock.Lock()
	appenders["sensor"] = worker
	appendersLock.Unlock()

	ack := StopAppendWorker("SENSOR")
	select {
	case <-ack:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for append worker stop ack")
	}

	appendersLock.Lock()
	_, exists := appenders["sensor"]
	appendersLock.Unlock()
	require.False(t, exists)
	require.Equal(t, int32(1), atomic.LoadInt32(&appender.closed))
	require.Equal(t, int32(1), atomic.LoadInt32(&conn.closed))
}

func TestFlushAppendWorkersMatchesNamesCaseInsensitively(t *testing.T) {
	StartAppendWorkers()
	t.Cleanup(StopAppendWorkers)

	sensor, sensorAppender, _ := newAppendWorkerForTest("sensor")
	metric, metricAppender, _ := newAppendWorkerForTest("metric")
	appendersLock.Lock()
	appenders["sensor"] = sensor
	appenders["metric"] = metric
	appendersLock.Unlock()

	FlushAppendWorkers("SENSOR")

	appendersLock.Lock()
	_, sensorExists := appenders["sensor"]
	_, metricExists := appenders["metric"]
	appendersLock.Unlock()
	require.False(t, sensorExists)
	require.True(t, metricExists)
	require.Equal(t, int32(1), atomic.LoadInt32(&sensorAppender.closed))
	require.Equal(t, int32(0), atomic.LoadInt32(&metricAppender.closed))

	FlushAppendWorkers()
	require.Equal(t, int32(1), atomic.LoadInt32(&metricAppender.closed))
	require.Empty(t, appenders)
}

func TestGetAppendWorkerReusesRegisteredWorkerCaseInsensitively(t *testing.T) {
	StartAppendWorkers()
	t.Cleanup(StopAppendWorkers)

	worker, _, _ := newAppendWorkerForTest("sensor")
	appendersLock.Lock()
	appenders["sensor"] = worker
	appendersLock.Unlock()

	got, err := GetAppendWorker(context.Background(), "SENSOR")
	require.NoError(t, err)
	require.Same(t, worker, got)
	require.Equal(t, int32(1), atomic.LoadInt32(&got.refCount))
}

func TestAppenderWithWorkerMapsInputColumns(t *testing.T) {
	worker, _, _ := newAppendWorkerForTest("sensor")
	worker.appendC = make(chan []interface{}, 1)

	wrapped := worker.WithInputColumns("value", "name")
	require.NoError(t, wrapped.Append(3.14, "temperature"))
	require.Equal(t, []interface{}{"temperature", 3.14}, <-worker.appendC)

	require.EqualError(t, worker.WithInputColumns().Append("only-name"), "value count 1, table 'sensor' requires 2 columns to append")
}

func TestAppendWorkerAppendLogTimeRequiresLogTable(t *testing.T) {
	worker, appender, _ := newAppendWorkerForTest("sensor")
	worker.appendC = make(chan []interface{}, 1)
	ts := time.Unix(1, 2)
	require.NoError(t, worker.AppendLogTime(ts, "temperature", 3.14))
	require.Equal(t, []interface{}{ts, "temperature", 3.14}, <-worker.appendC)

	appender.tableType = clientapi.TableTypeFixed
	err := worker.AppendLogTime(ts, "temperature", 3.14)
	require.EqualError(t, err, "sensor is not a log table, use Append() instead")
}
