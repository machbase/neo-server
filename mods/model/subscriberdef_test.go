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
	"time"

	"github.com/stretchr/testify/require"
)

const fakeSubscriberDriverName = "model_subscriber_fake"

var fakeSubscriberDriverOnce sync.Once
var fakeSubscriberStores sync.Map

type fakeSubscriberStore struct {
	mu              sync.Mutex
	nextID          int64
	subs            map[int64]fakeSubscriberRow
	dropTables      int
	missingUserName bool
}

type fakeSubscriberRow struct {
	id, autoStart, disabled, qos                                                                int64
	userName, name, execUser, lastError, task, bridge, topic, queueName, streamName, attributes string
	lastErrorAt                                                                                 time.Time
}

type fakeSubscriberDriver struct{}

func (d *fakeSubscriberDriver) Open(string) (driver.Conn, error) {
	return nil, errors.New("use OpenConnector")
}
func (d *fakeSubscriberDriver) OpenConnector(name string) (driver.Connector, error) {
	store, ok := fakeSubscriberStores.Load(name)
	if !ok {
		return nil, errors.New("fake subscriber store not found")
	}
	return &fakeSubscriberConnector{store: store.(*fakeSubscriberStore)}, nil
}

type fakeSubscriberConnector struct{ store *fakeSubscriberStore }

func (c *fakeSubscriberConnector) Connect(context.Context) (driver.Conn, error) {
	return &fakeSubscriberConn{store: c.store}, nil
}
func (c *fakeSubscriberConnector) Driver() driver.Driver { return &fakeSubscriberDriver{} }

type fakeSubscriberConn struct{ store *fakeSubscriberStore }

func (c *fakeSubscriberConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("prepare is not supported")
}
func (c *fakeSubscriberConn) Close() error { return nil }
func (c *fakeSubscriberConn) Begin() (driver.Tx, error) {
	return nil, errors.New("transactions are not supported")
}

func (c *fakeSubscriberConn) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	q := strings.ToUpper(strings.TrimSpace(query))
	switch {
	case strings.HasPrefix(q, "CREATE TABLE"):
		return fakeSubscriberResult{}, nil
	case strings.HasPrefix(q, "DROP TABLE _NEO_TIMER_DEF"):
		// scheduleConn ensures both tables on every open; this fake only
		// tracks subscriber state, so the sibling table's drop is a no-op.
		return fakeSubscriberResult{}, nil
	case strings.HasPrefix(q, "DROP TABLE _NEO_SUBSCRIBER_DEF"):
		c.store.mu.Lock()
		c.store.dropTables++
		c.store.missingUserName = false
		c.store.subs = map[int64]fakeSubscriberRow{}
		c.store.mu.Unlock()
		return fakeSubscriberResult{}, nil
	case strings.HasPrefix(q, "INSERT INTO _NEO_SUBSCRIBER_DEF"):
		if len(args) != 14 {
			return nil, errors.New("invalid subscriber insert args")
		}
		c.store.mu.Lock()
		defer c.store.mu.Unlock()
		id := c.store.nextID
		c.store.nextID++
		c.store.subs[id] = fakeSubscriberRow{id: id, userName: subscriberStringValue(args[0]), name: subscriberStringValue(args[1]), execUser: subscriberStringValue(args[2]), autoStart: subscriberIntValue(args[3]), disabled: subscriberIntValue(args[4]), lastError: subscriberStringValue(args[5]), lastErrorAt: subscriberTimeValue(args[6]), task: subscriberStringValue(args[7]), bridge: subscriberStringValue(args[8]), topic: subscriberStringValue(args[9]), qos: subscriberIntValue(args[10]), queueName: subscriberStringValue(args[11]), streamName: subscriberStringValue(args[12]), attributes: subscriberStringValue(args[13])}
		return fakeSubscriberResult{lastInsertID: id, rowsAffected: 1}, nil
	case strings.HasPrefix(q, "UPDATE _NEO_SUBSCRIBER_DEF SET LAST_ERROR"):
		c.store.mu.Lock()
		defer c.store.mu.Unlock()
		id := subscriberIntValue(args[2])
		row, ok := c.store.subs[id]
		if !ok {
			return fakeSubscriberResult{}, nil
		}
		row.lastError, row.lastErrorAt = subscriberStringValue(args[0]), subscriberTimeValue(args[1])
		c.store.subs[id] = row
		return fakeSubscriberResult{rowsAffected: 1}, nil
	case strings.HasPrefix(q, "UPDATE _NEO_SUBSCRIBER_DEF"):
		if len(args) != 15 {
			return nil, errors.New("invalid subscriber update args")
		}
		c.store.mu.Lock()
		defer c.store.mu.Unlock()
		id := subscriberIntValue(args[13])
		row, ok := c.store.subs[id]
		if !ok || row.userName != subscriberStringValue(args[14]) {
			return fakeSubscriberResult{}, nil
		}
		row.name, row.execUser, row.autoStart, row.disabled, row.lastError, row.lastErrorAt, row.task, row.bridge, row.topic, row.qos, row.queueName, row.streamName, row.attributes = subscriberStringValue(args[0]), subscriberStringValue(args[1]), subscriberIntValue(args[2]), subscriberIntValue(args[3]), subscriberStringValue(args[4]), subscriberTimeValue(args[5]), subscriberStringValue(args[6]), subscriberStringValue(args[7]), subscriberStringValue(args[8]), subscriberIntValue(args[9]), subscriberStringValue(args[10]), subscriberStringValue(args[11]), subscriberStringValue(args[12])
		c.store.subs[id] = row
		return fakeSubscriberResult{rowsAffected: 1}, nil
	case strings.HasPrefix(q, "DELETE FROM _NEO_SUBSCRIBER_DEF"):
		c.store.mu.Lock()
		defer c.store.mu.Unlock()
		var deleted bool
		if strings.Contains(q, "WHERE ID") {
			row, ok := c.store.subs[subscriberIntValue(args[0])]
			deleted = ok && (len(args) == 1 || row.userName == subscriberStringValue(args[1]))
			if deleted {
				delete(c.store.subs, subscriberIntValue(args[0]))
			}
		} else {
			for id, row := range c.store.subs {
				if row.name == subscriberStringValue(args[0]) {
					delete(c.store.subs, id)
					deleted = true
				}
			}
		}
		if deleted {
			return fakeSubscriberResult{rowsAffected: 1}, nil
		}
		return fakeSubscriberResult{}, nil
	default:
		return nil, errors.New("unexpected exec: " + query)
	}
}

func (c *fakeSubscriberConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	q := strings.ToUpper(strings.TrimSpace(query))
	c.store.mu.Lock()
	defer c.store.mu.Unlock()
	switch {
	case strings.HasPrefix(q, "SELECT USER_NAME FROM _NEO_SUBSCRIBER_DEF WHERE 1=0"):
		if c.store.missingUserName {
			return nil, errors.New("unknown column USER_NAME")
		}
		return &fakeSubscriberRows{columns: []string{"USER_NAME"}}, nil
	case strings.HasPrefix(q, "SELECT USER_NAME FROM _NEO_TIMER_DEF WHERE 1=0"):
		return &fakeSubscriberRows{columns: []string{"USER_NAME"}}, nil
	case strings.Contains(q, "FROM _NEO_SUBSCRIBER_DEF WHERE USER_NAME = ?"):
		user := subscriberStringValue(args[0])
		rows := make([]fakeSubscriberRow, 0, len(c.store.subs))
		for _, row := range c.store.subs {
			if row.userName == user {
				rows = append(rows, row)
			}
		}
		sort.Slice(rows, func(i, j int) bool { return rows[i].id < rows[j].id })
		return subscriberListRows(rows), nil
	case strings.Contains(q, "FROM _NEO_SUBSCRIBER_DEF WHERE ID = ? AND USER_NAME = ?"):
		row, ok := c.store.subs[subscriberIntValue(args[0])]
		if !ok || row.userName != subscriberStringValue(args[1]) {
			return &fakeSubscriberRows{columns: subscriberColumns}, nil
		}
		return subscriberRows(row), nil
	case strings.Contains(q, "FROM _NEO_SUBSCRIBER_DEF WHERE NAME = ? AND USER_NAME = ?"):
		name := subscriberStringValue(args[0])
		user := subscriberStringValue(args[1])
		for _, row := range c.store.subs {
			if row.name == name && row.userName == user {
				return subscriberRows(row), nil
			}
		}
		return &fakeSubscriberRows{columns: subscriberColumns}, nil
	case strings.Contains(q, "FROM _NEO_SUBSCRIBER_DEF WHERE NAME"):
		name := subscriberStringValue(args[0])
		for _, row := range c.store.subs {
			if row.name == name {
				return subscriberRows(row), nil
			}
		}
		return &fakeSubscriberRows{columns: subscriberColumns}, nil
	case strings.Contains(q, "FROM _NEO_SUBSCRIBER_DEF WHERE ID"):
		row, ok := c.store.subs[subscriberIntValue(args[0])]
		if !ok {
			return &fakeSubscriberRows{columns: subscriberColumns}, nil
		}
		return subscriberRows(row), nil
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

var subscriberColumns = []string{"ID", "USER_NAME", "NAME", "EXEC_USER", "AUTO_START", "DISABLED", "LAST_ERROR", "LAST_ERROR_AT", "TASK", "BRIDGE", "TOPIC", "QOS", "QUEUE_NAME", "STREAM_NAME", "ATTRIBUTES"}

type fakeSubscriberRows struct {
	columns []string
	values  [][]driver.Value
	idx     int
}

func (r *fakeSubscriberRows) Columns() []string { return r.columns }
func (r *fakeSubscriberRows) Close() error      { return nil }
func (r *fakeSubscriberRows) Next(dest []driver.Value) error {
	if r.idx >= len(r.values) {
		return io.EOF
	}
	copy(dest, r.values[r.idx])
	r.idx++
	return nil
}
func subscriberRows(row fakeSubscriberRow) driver.Rows {
	return &fakeSubscriberRows{columns: subscriberColumns, values: [][]driver.Value{{row.id, row.userName, row.name, row.execUser, row.autoStart, row.disabled, row.lastError, row.lastErrorAt, row.task, row.bridge, row.topic, row.qos, row.queueName, row.streamName, row.attributes}}}
}
func subscriberListRows(rows []fakeSubscriberRow) driver.Rows {
	r := &fakeSubscriberRows{columns: subscriberColumns}
	for _, row := range rows {
		r.values = append(r.values, []driver.Value{row.id, row.userName, row.name, row.execUser, row.autoStart, row.disabled, row.lastError, row.lastErrorAt, row.task, row.bridge, row.topic, row.qos, row.queueName, row.streamName, row.attributes})
	}
	return r
}

type fakeSubscriberResult struct{ lastInsertID, rowsAffected int64 }

func (r fakeSubscriberResult) LastInsertId() (int64, error) { return r.lastInsertID, nil }
func (r fakeSubscriberResult) RowsAffected() (int64, error) { return r.rowsAffected, nil }

func registerFakeSubscriberDriver() {
	fakeSubscriberDriverOnce.Do(func() { sql.Register(fakeSubscriberDriverName, &fakeSubscriberDriver{}) })
}

func subscriberStringValue(v driver.NamedValue) string {
	if v.Value == nil {
		return ""
	}
	return v.Value.(string)
}
func subscriberIntValue(v driver.NamedValue) int64 {
	switch value := v.Value.(type) {
	case int64:
		return value
	case int:
		return int64(value)
	default:
		return 0
	}
}
func subscriberTimeValue(v driver.NamedValue) time.Time {
	value, _ := v.Value.(time.Time)
	return value
}

func newFakeSubscriberProvider(t *testing.T) (*Provider, *fakeSubscriberStore, func()) {
	t.Helper()
	registerFakeSubscriberDriver()
	store := &fakeSubscriberStore{nextID: 1, subs: map[int64]fakeSubscriberRow{}}
	dsn := t.Name()
	fakeSubscriberStores.Store(dsn, store)
	db, err := sql.Open(fakeSubscriberDriverName, dsn)
	require.NoError(t, err)
	provider := NewProvider()
	provider.connect = func(ctx context.Context, _ string) (*sql.Conn, error) { return db.Conn(ctx) }
	return provider, store, func() { require.NoError(t, db.Close()); fakeSubscriberStores.Delete(dsn) }
}

func TestSubscriberDefinitionScopedNameLookups(t *testing.T) {
	provider, _, cleanup := newFakeSubscriberProvider(t)
	defer cleanup()
	ctx := context.Background()
	alice := UserScope{User: "alice"}
	bob := UserScope{User: "bob"}

	subscriber := &SubscriberDefinition{Name: "subscriber-name", ExecUser: "alice", Task: "task", Bridge: "mqtt", Topic: "topic"}
	require.NoError(t, provider.SaveSubscriberForUser(ctx, alice, subscriber))

	loaded, err := provider.LoadSubscriberByNameForUser(ctx, alice, "SUBSCRIBER-NAME")
	require.NoError(t, err)
	require.Equal(t, subscriber.Id, loaded.Id)
	_, err = provider.LoadSubscriberByNameForUser(ctx, bob, "SUBSCRIBER-NAME")
	require.EqualError(t, err, "subscriber 'SUBSCRIBER-NAME' not found")

	require.ErrorIs(t, provider.RemoveSubscriberByID(ctx, 999), os.ErrNotExist)
}

func TestSubscriberDefinitionPersistence(t *testing.T) {
	provider, store, cleanup := newFakeSubscriberProvider(t)
	defer cleanup()
	ctx := context.Background()
	subscriber := &SubscriberDefinition{UserName: "alice", Name: "sub-a", ExecUser: "alice", Disabled: true, LastError: "subscriber failed", LastErrorAt: time.Now(), Task: "append.tql", Bridge: "mqtt", Topic: "events", QoS: 1, QueueName: "q", StreamName: "s", Attributes: json.RawMessage(`{"batch":10}`)}
	require.NoError(t, provider.SaveSubscriber(ctx, subscriber))
	require.Equal(t, int64(1), subscriber.Id)
	loaded, err := provider.LoadSubscriberByID(ctx, subscriber.Id)
	require.NoError(t, err)
	require.Equal(t, subscriber.Attributes, loaded.Attributes)
	require.Equal(t, "q", loaded.QueueName)
	require.True(t, loaded.Disabled)
	require.Equal(t, "subscriber failed", loaded.LastError)
	require.NoError(t, provider.SetSubscriberRuntimeError(ctx, subscriber.Id, ""))
	subscribers, err := provider.LoadAllSubscribers(ctx)
	require.NoError(t, err)
	require.Len(t, subscribers, 1)
	require.NoError(t, provider.RemoveSubscriberByID(ctx, subscriber.Id))
	require.Empty(t, store.subs)
	subscriber = &SubscriberDefinition{UserName: "alice", Name: "sub-b", ExecUser: "alice", Task: "append.tql", Bridge: "mqtt", Topic: "events"}
	require.NoError(t, provider.SaveSubscriber(ctx, subscriber))
	require.NoError(t, provider.RemoveSubscriber(ctx, subscriber.Name))
	require.Empty(t, store.subs)
}

// NAME duplicates are allowed both within the subscriber table and against
// a timer sharing the same name; only ID identifies a definition.
func TestSubscriberDefinitionAllowsDuplicateNames(t *testing.T) {
	provider, _, cleanup := newFakeSubscriberProvider(t)
	defer cleanup()
	ctx := context.Background()
	firstSubscriber := &SubscriberDefinition{UserName: "alice", Name: "same", ExecUser: "alice", Task: "task", Bridge: "mqtt", Topic: "topic"}
	secondSubscriber := &SubscriberDefinition{UserName: "bob", Name: "same", ExecUser: "bob", Task: "task", Bridge: "mqtt", Topic: "topic"}
	require.NoError(t, provider.SaveSubscriber(ctx, firstSubscriber))
	require.NoError(t, provider.SaveSubscriber(ctx, secondSubscriber))
	require.NotEqual(t, firstSubscriber.Id, secondSubscriber.Id)

	timerProvider, _, timerCleanup := newFakeTimerProvider(t)
	defer timerCleanup()
	timer := &TimerDefinition{UserName: "sys", Name: "same", ExecUser: "sys", Task: "task", Schedule: "@every 1m"}
	require.NoError(t, timerProvider.SaveTimer(ctx, timer))
}

func TestSubscriberDefinitionTableResetAndOwnership(t *testing.T) {
	provider, store, cleanup := newFakeSubscriberProvider(t)
	defer cleanup()
	store.missingUserName = true
	ctx := context.Background()
	alice := UserScope{User: "alice"}
	bob := UserScope{User: "bob"}
	subscriber := &SubscriberDefinition{Name: "subscriber-name", ExecUser: "ALICE", Task: "append.tql", Bridge: "mqtt", Topic: "events"}
	require.NoError(t, provider.SaveSubscriberForUser(ctx, alice, subscriber))
	require.Equal(t, "ALICE", subscriber.UserName)
	require.Equal(t, 1, store.dropTables)
	subscribers, err := provider.LoadSubscribers(ctx, alice)
	require.NoError(t, err)
	require.Len(t, subscribers, 1)
	subscribers, err = provider.LoadSubscribers(ctx, bob)
	require.NoError(t, err)
	require.Empty(t, subscribers)
	_, err = provider.LoadSubscriberForUser(ctx, bob, subscriber.Id)
	require.Error(t, err)
	require.ErrorIs(t, provider.RemoveSubscriberForUser(ctx, bob, subscriber.Id), sql.ErrNoRows)
	require.NoError(t, provider.RemoveSubscriberForUser(ctx, alice, subscriber.Id))
}
