package model

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"io"
	"os"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

const fakeScheduleDriverName = "model_schedule_fake"

var fakeScheduleDriverOnce sync.Once
var fakeScheduleStores sync.Map

type fakeScheduleStore struct {
	mu           sync.Mutex
	nextTimerID  int64
	nextSubID    int64
	timers       map[int64]fakeTimerRow
	subs         map[int64]fakeSubscriberRow
	createTables int
}

type fakeTimerRow struct {
	id, autoStart                              int64
	name, execUser, task, schedule, attributes string
}

type fakeSubscriberRow struct {
	id, autoStart, qos                                                     int64
	name, execUser, task, bridge, topic, queueName, streamName, attributes string
}

type fakeScheduleDriver struct{}

func (d *fakeScheduleDriver) Open(string) (driver.Conn, error) {
	return nil, errors.New("use OpenConnector")
}
func (d *fakeScheduleDriver) OpenConnector(name string) (driver.Connector, error) {
	store, ok := fakeScheduleStores.Load(name)
	if !ok {
		return nil, errors.New("fake schedule store not found")
	}
	return &fakeScheduleConnector{store: store.(*fakeScheduleStore)}, nil
}

type fakeScheduleConnector struct{ store *fakeScheduleStore }

func (c *fakeScheduleConnector) Connect(context.Context) (driver.Conn, error) {
	return &fakeScheduleConn{store: c.store}, nil
}
func (c *fakeScheduleConnector) Driver() driver.Driver { return &fakeScheduleDriver{} }

type fakeScheduleConn struct{ store *fakeScheduleStore }

func (c *fakeScheduleConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("prepare is not supported")
}
func (c *fakeScheduleConn) Close() error { return nil }
func (c *fakeScheduleConn) Begin() (driver.Tx, error) {
	return nil, errors.New("transactions are not supported")
}

func (c *fakeScheduleConn) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	q := strings.ToUpper(strings.TrimSpace(query))
	switch {
	case strings.HasPrefix(q, "CREATE TABLE"):
		c.store.mu.Lock()
		c.store.createTables++
		c.store.mu.Unlock()
		return fakeScheduleResult{}, nil
	case strings.HasPrefix(q, "INSERT INTO _NEO_TIMER_DEF"):
		if len(args) != 6 {
			return nil, errors.New("invalid timer insert args")
		}
		c.store.mu.Lock()
		defer c.store.mu.Unlock()
		id := c.store.nextTimerID
		c.store.nextTimerID++
		c.store.timers[id] = fakeTimerRow{id: id, name: stringValue(args[0]), execUser: stringValue(args[1]), autoStart: intValue(args[2]), task: stringValue(args[3]), schedule: stringValue(args[4]), attributes: stringValue(args[5])}
		return fakeScheduleResult{lastInsertID: id, rowsAffected: 1}, nil
	case strings.HasPrefix(q, "INSERT INTO _NEO_SUBSCRIBER_DEF"):
		if len(args) != 10 {
			return nil, errors.New("invalid subscriber insert args")
		}
		c.store.mu.Lock()
		defer c.store.mu.Unlock()
		id := c.store.nextSubID
		c.store.nextSubID++
		c.store.subs[id] = fakeSubscriberRow{id: id, name: stringValue(args[0]), execUser: stringValue(args[1]), autoStart: intValue(args[2]), task: stringValue(args[3]), bridge: stringValue(args[4]), topic: stringValue(args[5]), qos: intValue(args[6]), queueName: stringValue(args[7]), streamName: stringValue(args[8]), attributes: stringValue(args[9])}
		return fakeScheduleResult{lastInsertID: id, rowsAffected: 1}, nil
	case strings.HasPrefix(q, "UPDATE _NEO_TIMER_DEF"):
		if len(args) != 6 {
			return nil, errors.New("invalid timer update args")
		}
		c.store.mu.Lock()
		defer c.store.mu.Unlock()
		id := intValue(args[5])
		row, ok := c.store.timers[id]
		if !ok {
			return fakeScheduleResult{}, nil
		}
		row.name, row.autoStart, row.task, row.schedule, row.attributes = stringValue(args[0]), intValue(args[1]), stringValue(args[2]), stringValue(args[3]), stringValue(args[4])
		c.store.timers[id] = row
		return fakeScheduleResult{rowsAffected: 1}, nil
	case strings.HasPrefix(q, "UPDATE _NEO_SUBSCRIBER_DEF"):
		if len(args) != 10 {
			return nil, errors.New("invalid subscriber update args")
		}
		c.store.mu.Lock()
		defer c.store.mu.Unlock()
		id := intValue(args[9])
		row, ok := c.store.subs[id]
		if !ok {
			return fakeScheduleResult{}, nil
		}
		row.name, row.autoStart, row.task, row.bridge, row.topic, row.qos, row.queueName, row.streamName, row.attributes = stringValue(args[0]), intValue(args[1]), stringValue(args[2]), stringValue(args[3]), stringValue(args[4]), intValue(args[5]), stringValue(args[6]), stringValue(args[7]), stringValue(args[8])
		c.store.subs[id] = row
		return fakeScheduleResult{rowsAffected: 1}, nil
	case strings.HasPrefix(q, "DELETE FROM _NEO_TIMER_DEF"):
		c.store.mu.Lock()
		defer c.store.mu.Unlock()
		var deleted bool
		if strings.Contains(q, "WHERE ID") {
			_, deleted = c.store.timers[intValue(args[0])]
			delete(c.store.timers, intValue(args[0]))
		} else {
			for id, row := range c.store.timers {
				if row.name == stringValue(args[0]) {
					delete(c.store.timers, id)
					deleted = true
				}
			}
		}
		if deleted {
			return fakeScheduleResult{rowsAffected: 1}, nil
		}
		return fakeScheduleResult{}, nil
	case strings.HasPrefix(q, "DELETE FROM _NEO_SUBSCRIBER_DEF"):
		c.store.mu.Lock()
		defer c.store.mu.Unlock()
		var deleted bool
		if strings.Contains(q, "WHERE ID") {
			_, deleted = c.store.subs[intValue(args[0])]
			delete(c.store.subs, intValue(args[0]))
		} else {
			for id, row := range c.store.subs {
				if row.name == stringValue(args[0]) {
					delete(c.store.subs, id)
					deleted = true
				}
			}
		}
		if deleted {
			return fakeScheduleResult{rowsAffected: 1}, nil
		}
		return fakeScheduleResult{}, nil
	default:
		return nil, errors.New("unexpected exec: " + query)
	}
}

func (c *fakeScheduleConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	q := strings.ToUpper(strings.TrimSpace(query))
	c.store.mu.Lock()
	defer c.store.mu.Unlock()
	switch {
	case strings.Contains(q, "SELECT COUNT(*) FROM _NEO_TIMER_DEF"):
		name := stringValue(args[0])
		count := int64(0)
		for _, row := range c.store.timers {
			if row.name == name {
				count++
			}
		}
		return &fakeCountRows{count: count}, nil
	case strings.Contains(q, "SELECT COUNT(*) FROM _NEO_SUBSCRIBER_DEF"):
		name := stringValue(args[0])
		count := int64(0)
		for _, row := range c.store.subs {
			if row.name == name {
				count++
			}
		}
		return &fakeCountRows{count: count}, nil
	case strings.Contains(q, "FROM _NEO_TIMER_DEF WHERE NAME"):
		name := stringValue(args[0])
		for _, row := range c.store.timers {
			if row.name == name {
				return timerRows(row), nil
			}
		}
		return &fakeScheduleRows{columns: timerColumns}, nil
	case strings.Contains(q, "FROM _NEO_TIMER_DEF WHERE ID"):
		row, ok := c.store.timers[intValue(args[0])]
		if !ok {
			return &fakeScheduleRows{columns: timerColumns}, nil
		}
		return timerRows(row), nil
	case strings.Contains(q, "FROM _NEO_SUBSCRIBER_DEF WHERE NAME"):
		name := stringValue(args[0])
		for _, row := range c.store.subs {
			if row.name == name {
				return subscriberRows(row), nil
			}
		}
		return &fakeScheduleRows{columns: subscriberColumns}, nil
	case strings.Contains(q, "FROM _NEO_SUBSCRIBER_DEF WHERE ID"):
		row, ok := c.store.subs[intValue(args[0])]
		if !ok {
			return &fakeScheduleRows{columns: subscriberColumns}, nil
		}
		return subscriberRows(row), nil
	case strings.Contains(q, "FROM _NEO_TIMER_DEF ORDER BY ID"):
		rows := make([]fakeTimerRow, 0, len(c.store.timers))
		for _, row := range c.store.timers {
			rows = append(rows, row)
		}
		sort.Slice(rows, func(i, j int) bool { return rows[i].id < rows[j].id })
		return timerListRows(rows), nil
	case strings.Contains(q, "FROM _NEO_SUBSCRIBER_DEF ORDER BY ID"):
		rows := make([]fakeSubscriberRow, 0, len(c.store.subs))
		for _, row := range c.store.subs {
			rows = append(rows, row)
		}
		sort.Slice(rows, func(i, j int) bool { return rows[i].id < rows[j].id })
		return subscriberListRows(rows), nil
	default:
		return nil, errors.New("unexpected query: " + query)
	}
}

var timerColumns = []string{"ID", "NAME", "EXEC_USER", "AUTO_START", "TASK", "SCHEDULE", "ATTRIBUTES"}
var subscriberColumns = []string{"ID", "NAME", "EXEC_USER", "AUTO_START", "TASK", "BRIDGE", "TOPIC", "QOS", "QUEUE_NAME", "STREAM_NAME", "ATTRIBUTES"}

type fakeScheduleRows struct {
	columns []string
	values  [][]driver.Value
	idx     int
}

func (r *fakeScheduleRows) Columns() []string { return r.columns }
func (r *fakeScheduleRows) Close() error      { return nil }
func (r *fakeScheduleRows) Next(dest []driver.Value) error {
	if r.idx >= len(r.values) {
		return io.EOF
	}
	copy(dest, r.values[r.idx])
	r.idx++
	return nil
}
func timerRows(row fakeTimerRow) driver.Rows {
	return &fakeScheduleRows{columns: timerColumns, values: [][]driver.Value{{row.id, row.name, row.execUser, row.autoStart, row.task, row.schedule, row.attributes}}}
}
func timerListRows(rows []fakeTimerRow) driver.Rows {
	r := &fakeScheduleRows{columns: timerColumns}
	for _, row := range rows {
		r.values = append(r.values, []driver.Value{row.id, row.name, row.execUser, row.autoStart, row.task, row.schedule, row.attributes})
	}
	return r
}
func subscriberRows(row fakeSubscriberRow) driver.Rows {
	return &fakeScheduleRows{columns: subscriberColumns, values: [][]driver.Value{{row.id, row.name, row.execUser, row.autoStart, row.task, row.bridge, row.topic, row.qos, row.queueName, row.streamName, row.attributes}}}
}
func subscriberListRows(rows []fakeSubscriberRow) driver.Rows {
	r := &fakeScheduleRows{columns: subscriberColumns}
	for _, row := range rows {
		r.values = append(r.values, []driver.Value{row.id, row.name, row.execUser, row.autoStart, row.task, row.bridge, row.topic, row.qos, row.queueName, row.streamName, row.attributes})
	}
	return r
}

type fakeCountRows struct {
	count int64
	done  bool
}

func (r *fakeCountRows) Columns() []string { return []string{"COUNT(*)"} }
func (r *fakeCountRows) Close() error      { return nil }
func (r *fakeCountRows) Next(dest []driver.Value) error {
	if r.done {
		return io.EOF
	}
	dest[0] = r.count
	r.done = true
	return nil
}

type fakeScheduleResult struct{ lastInsertID, rowsAffected int64 }

func (r fakeScheduleResult) LastInsertId() (int64, error) { return r.lastInsertID, nil }
func (r fakeScheduleResult) RowsAffected() (int64, error) { return r.rowsAffected, nil }

func registerFakeScheduleDriver() {
	fakeScheduleDriverOnce.Do(func() { sql.Register(fakeScheduleDriverName, &fakeScheduleDriver{}) })
}
func stringValue(v driver.NamedValue) string {
	if v.Value == nil {
		return ""
	}
	return v.Value.(string)
}
func intValue(v driver.NamedValue) int64 {
	switch value := v.Value.(type) {
	case int64:
		return value
	case int:
		return int64(value)
	default:
		return 0
	}
}

func newFakeScheduleProvider(t *testing.T) (*Provider, *fakeScheduleStore, func()) {
	t.Helper()
	registerFakeScheduleDriver()
	store := &fakeScheduleStore{nextTimerID: 1, nextSubID: 1, timers: map[int64]fakeTimerRow{}, subs: map[int64]fakeSubscriberRow{}}
	dsn := t.Name()
	fakeScheduleStores.Store(dsn, store)
	db, err := sql.Open(fakeScheduleDriverName, dsn)
	require.NoError(t, err)
	provider := NewProvider()
	provider.connect = func(ctx context.Context, _ string) (*sql.Conn, error) { return db.Conn(ctx) }
	return provider, store, func() { require.NoError(t, db.Close()); fakeScheduleStores.Delete(dsn) }
}

func TestScheduleTablePersistence(t *testing.T) {
	provider, store, cleanup := newFakeScheduleProvider(t)
	defer cleanup()
	ctx := context.Background()
	attributes := json.RawMessage(`{"retry":3,"headers":{"x":"y"}}`)
	timer := &TimerDefinition{Name: "timer-a", ExecUser: "alice", Task: "timer.tql", Schedule: "@every 1m", Attributes: attributes}
	require.NoError(t, provider.SaveTimer(ctx, timer))
	require.Equal(t, int64(1), timer.Id)
	loadedTimer, err := provider.LoadTimerByID(ctx, timer.Id)
	require.NoError(t, err)
	require.Equal(t, attributes, loadedTimer.Attributes)
	loadedTimer.Task = "timer-updated.tql"
	require.NoError(t, provider.SaveTimer(ctx, loadedTimer))
	loadedTimer, err = provider.LoadTimer(ctx, "timer-a")
	require.NoError(t, err)
	require.Equal(t, "timer-updated.tql", loadedTimer.Task)
	timers, err := provider.LoadAllTimers(ctx)
	require.NoError(t, err)
	require.Len(t, timers, 1)
	timerID := timer.Id

	subscriber := &SubscriberDefinition{Name: "sub-a", ExecUser: "alice", Task: "append.tql", Bridge: "mqtt", Topic: "events", QoS: 1, QueueName: "q", StreamName: "s", Attributes: json.RawMessage(`{"batch":10}`)}
	require.NoError(t, provider.SaveSubscriber(ctx, subscriber))
	require.Equal(t, int64(1), subscriber.Id)
	loadedSub, err := provider.LoadSubscriberByID(ctx, subscriber.Id)
	require.NoError(t, err)
	require.Equal(t, subscriber.Attributes, loadedSub.Attributes)
	require.Equal(t, "q", loadedSub.QueueName)
	subs, err := provider.LoadAllSubscribers(ctx)
	require.NoError(t, err)
	require.Len(t, subs, 1)
	require.NoError(t, provider.RemoveTimerByID(ctx, timerID))
	require.NoError(t, provider.RemoveSubscriberByID(ctx, subscriber.Id))
	subscriber = &SubscriberDefinition{Name: "sub-b", ExecUser: "alice", Task: "append.tql", Bridge: "mqtt", Topic: "events"}
	require.NoError(t, provider.SaveSubscriber(ctx, subscriber))
	require.NoError(t, provider.RemoveSubscriber(ctx, subscriber.Name))
	require.Empty(t, store.timers)
	require.Empty(t, store.subs)
	require.Equal(t, 2, store.createTables)
}

func TestScheduleNameUniquenessAndNotFound(t *testing.T) {
	provider, _, cleanup := newFakeScheduleProvider(t)
	defer cleanup()
	ctx := context.Background()
	def := &TimerDefinition{Name: "same", ExecUser: "sys", Task: "a", Schedule: "@every 1m"}
	require.NoError(t, provider.SaveTimer(ctx, def))
	sub := &SubscriberDefinition{Name: "same", ExecUser: "sys", Task: "a", Bridge: "b", Topic: "t"}
	require.Error(t, provider.SaveSubscriber(ctx, sub))
	require.NoError(t, provider.RemoveTimer(ctx, "same"))
	require.ErrorIs(t, provider.RemoveTimerByID(ctx, 999), os.ErrNotExist)
}
