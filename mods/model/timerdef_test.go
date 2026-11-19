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

const fakeTimerDriverName = "model_timer_fake"

var fakeTimerDriverOnce sync.Once
var fakeTimerStores sync.Map

type fakeTimerStore struct {
	mu                     sync.Mutex
	nextID                 int64
	timers                 map[int64]fakeTimerRow
	dropTables             int
	missingUserName        bool
	missingContinueOnError bool
	checkError             error
	alterError             error
	alterCount             int
}

type fakeTimerRow struct {
	id, autoStart, continueOnError, disabled                        int64
	continueOnErrorNull                                             bool
	userName, name, execUser, lastError, task, schedule, attributes string
	lastErrorAt                                                     time.Time
}

type fakeTimerDriver struct{}

func (d *fakeTimerDriver) Open(string) (driver.Conn, error) {
	return nil, errors.New("use OpenConnector")
}
func (d *fakeTimerDriver) OpenConnector(name string) (driver.Connector, error) {
	store, ok := fakeTimerStores.Load(name)
	if !ok {
		return nil, errors.New("fake timer store not found")
	}
	return &fakeTimerConnector{store: store.(*fakeTimerStore)}, nil
}

type fakeTimerConnector struct{ store *fakeTimerStore }

func (c *fakeTimerConnector) Connect(context.Context) (driver.Conn, error) {
	return &fakeTimerConn{store: c.store}, nil
}
func (c *fakeTimerConnector) Driver() driver.Driver { return &fakeTimerDriver{} }

type fakeTimerConn struct{ store *fakeTimerStore }

func (c *fakeTimerConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("prepare is not supported")
}
func (c *fakeTimerConn) Close() error { return nil }
func (c *fakeTimerConn) Begin() (driver.Tx, error) {
	return nil, errors.New("transactions are not supported")
}

func (c *fakeTimerConn) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	q := strings.ToUpper(strings.TrimSpace(query))
	switch {
	case strings.HasPrefix(q, "CREATE TABLE"):
		return fakeTimerResult{}, nil
	case strings.HasPrefix(q, "ALTER TABLE _NEO_TIMER_DEF ADD COLUMN"):
		c.store.mu.Lock()
		defer c.store.mu.Unlock()
		if c.store.alterError != nil {
			return nil, c.store.alterError
		}
		c.store.missingContinueOnError = false
		for id, row := range c.store.timers {
			row.continueOnErrorNull = true
			c.store.timers[id] = row
		}
		c.store.alterCount++
		return fakeTimerResult{}, nil
	case strings.HasPrefix(q, "DROP TABLE _NEO_TIMER_DEF"):
		c.store.mu.Lock()
		c.store.dropTables++
		c.store.missingUserName = false
		c.store.timers = map[int64]fakeTimerRow{}
		c.store.mu.Unlock()
		return fakeTimerResult{}, nil
	case strings.HasPrefix(q, "DROP TABLE _NEO_SUBSCRIBER_DEF"):
		// scheduleConn ensures both tables on every open; this fake only
		// tracks timer state, so the sibling table's drop is a no-op.
		return fakeTimerResult{}, nil
	case strings.HasPrefix(q, "INSERT INTO _NEO_TIMER_DEF"):
		if len(args) != 11 {
			return nil, errors.New("invalid timer insert args")
		}
		c.store.mu.Lock()
		defer c.store.mu.Unlock()
		id := c.store.nextID
		c.store.nextID++
		c.store.timers[id] = fakeTimerRow{id: id, userName: timerStringValue(args[0]), name: timerStringValue(args[1]), execUser: timerStringValue(args[2]), autoStart: timerIntValue(args[3]), continueOnError: timerIntValue(args[4]), disabled: timerIntValue(args[5]), lastError: timerStringValue(args[6]), lastErrorAt: timerTimeValue(args[7]), task: timerStringValue(args[8]), schedule: timerStringValue(args[9]), attributes: timerStringValue(args[10])}
		return fakeTimerResult{lastInsertID: id, rowsAffected: 1}, nil
	case strings.HasPrefix(q, "UPDATE _NEO_TIMER_DEF SET LAST_ERROR"):
		c.store.mu.Lock()
		defer c.store.mu.Unlock()
		id := timerIntValue(args[2])
		row, ok := c.store.timers[id]
		if !ok {
			return fakeTimerResult{}, nil
		}
		row.lastError, row.lastErrorAt = timerStringValue(args[0]), timerTimeValue(args[1])
		c.store.timers[id] = row
		return fakeTimerResult{rowsAffected: 1}, nil
	case strings.HasPrefix(q, "UPDATE _NEO_TIMER_DEF"):
		if len(args) != 10 {
			return nil, errors.New("invalid timer update args")
		}
		c.store.mu.Lock()
		defer c.store.mu.Unlock()
		id := timerIntValue(args[8])
		row, ok := c.store.timers[id]
		if !ok || row.userName != timerStringValue(args[9]) {
			return fakeTimerResult{}, nil
		}
		row.name, row.execUser, row.autoStart, row.continueOnError, row.disabled, row.task, row.schedule, row.attributes = timerStringValue(args[0]), timerStringValue(args[1]), timerIntValue(args[2]), timerIntValue(args[3]), timerIntValue(args[4]), timerStringValue(args[5]), timerStringValue(args[6]), timerStringValue(args[7])
		row.continueOnErrorNull = false
		c.store.timers[id] = row
		return fakeTimerResult{rowsAffected: 1}, nil
	case strings.HasPrefix(q, "DELETE FROM _NEO_TIMER_DEF"):
		c.store.mu.Lock()
		defer c.store.mu.Unlock()
		var deleted bool
		if strings.Contains(q, "WHERE ID") {
			row, ok := c.store.timers[timerIntValue(args[0])]
			deleted = ok && (len(args) == 1 || row.userName == timerStringValue(args[1]))
			if deleted {
				delete(c.store.timers, timerIntValue(args[0]))
			}
		} else {
			for id, row := range c.store.timers {
				if row.name == timerStringValue(args[0]) {
					delete(c.store.timers, id)
					deleted = true
				}
			}
		}
		if deleted {
			return fakeTimerResult{rowsAffected: 1}, nil
		}
		return fakeTimerResult{}, nil
	default:
		return nil, errors.New("unexpected exec: " + query)
	}
}

func (c *fakeTimerConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	q := strings.ToUpper(strings.TrimSpace(query))
	c.store.mu.Lock()
	defer c.store.mu.Unlock()
	switch {
	case strings.HasPrefix(q, "SELECT USER_NAME FROM _NEO_TIMER_DEF WHERE 1=0"):
		if c.store.missingUserName {
			return nil, errors.New("invalid column USER_NAME")
		}
		return &fakeTimerRows{columns: []string{"USER_NAME"}}, nil
	case strings.HasPrefix(q, "SELECT USER_NAME FROM _NEO_SUBSCRIBER_DEF WHERE 1=0"):
		return &fakeTimerRows{columns: []string{"USER_NAME"}}, nil
	case strings.HasPrefix(q, "SELECT CONTINUE_ON_ERROR FROM _NEO_TIMER_DEF WHERE 1=0"):
		if c.store.checkError != nil {
			return nil, c.store.checkError
		}
		if c.store.missingContinueOnError {
			return nil, errors.New("column name CONTINUE_ON_ERROR not found")
		}
		return &fakeTimerRows{columns: []string{"CONTINUE_ON_ERROR"}}, nil
	case strings.Contains(q, "FROM _NEO_TIMER_DEF WHERE USER_NAME = ?"):
		user := timerStringValue(args[0])
		rows := make([]fakeTimerRow, 0, len(c.store.timers))
		for _, row := range c.store.timers {
			if row.userName == user {
				rows = append(rows, row)
			}
		}
		sort.Slice(rows, func(i, j int) bool { return rows[i].id < rows[j].id })
		return timerListRows(rows), nil
	case strings.Contains(q, "FROM _NEO_TIMER_DEF WHERE ID = ? AND USER_NAME = ?"):
		row, ok := c.store.timers[timerIntValue(args[0])]
		if !ok || row.userName != timerStringValue(args[1]) {
			return &fakeTimerRows{columns: timerColumns}, nil
		}
		return timerRows(row), nil
	case strings.Contains(q, "FROM _NEO_TIMER_DEF WHERE NAME = ? AND USER_NAME = ?"):
		name := timerStringValue(args[0])
		user := timerStringValue(args[1])
		for _, row := range c.store.timers {
			if row.name == name && row.userName == user {
				return timerRows(row), nil
			}
		}
		return &fakeTimerRows{columns: timerColumns}, nil
	case strings.Contains(q, "FROM _NEO_TIMER_DEF WHERE NAME"):
		name := timerStringValue(args[0])
		for _, row := range c.store.timers {
			if row.name == name {
				return timerRows(row), nil
			}
		}
		return &fakeTimerRows{columns: timerColumns}, nil
	case strings.Contains(q, "FROM _NEO_TIMER_DEF WHERE ID"):
		row, ok := c.store.timers[timerIntValue(args[0])]
		if !ok {
			return &fakeTimerRows{columns: timerColumns}, nil
		}
		return timerRows(row), nil
	case strings.Contains(q, "FROM _NEO_TIMER_DEF ORDER BY ID"):
		rows := make([]fakeTimerRow, 0, len(c.store.timers))
		for _, row := range c.store.timers {
			rows = append(rows, row)
		}
		sort.Slice(rows, func(i, j int) bool { return rows[i].id < rows[j].id })
		return timerListRows(rows), nil
	default:
		return nil, errors.New("unexpected query: " + query)
	}
}

var timerColumns = []string{"ID", "USER_NAME", "NAME", "EXEC_USER", "AUTO_START", "CONTINUE_ON_ERROR", "DISABLED", "LAST_ERROR", "LAST_ERROR_AT", "TASK", "SCHEDULE", "ATTRIBUTES"}

type fakeTimerRows struct {
	columns []string
	values  [][]driver.Value
	idx     int
}

func (r *fakeTimerRows) Columns() []string { return r.columns }
func (r *fakeTimerRows) Close() error      { return nil }
func (r *fakeTimerRows) Next(dest []driver.Value) error {
	if r.idx >= len(r.values) {
		return io.EOF
	}
	copy(dest, r.values[r.idx])
	r.idx++
	return nil
}
func timerRows(row fakeTimerRow) driver.Rows {
	return &fakeTimerRows{columns: timerColumns, values: [][]driver.Value{{row.id, row.userName, row.name, row.execUser, row.autoStart, timerContinueValue(row), row.disabled, row.lastError, row.lastErrorAt, row.task, row.schedule, row.attributes}}}
}
func timerListRows(rows []fakeTimerRow) driver.Rows {
	r := &fakeTimerRows{columns: timerColumns}
	for _, row := range rows {
		r.values = append(r.values, []driver.Value{row.id, row.userName, row.name, row.execUser, row.autoStart, timerContinueValue(row), row.disabled, row.lastError, row.lastErrorAt, row.task, row.schedule, row.attributes})
	}
	return r
}

func timerContinueValue(row fakeTimerRow) driver.Value {
	if row.continueOnErrorNull {
		return nil
	}
	return row.continueOnError
}

type fakeTimerResult struct{ lastInsertID, rowsAffected int64 }

func (r fakeTimerResult) LastInsertId() (int64, error) { return r.lastInsertID, nil }
func (r fakeTimerResult) RowsAffected() (int64, error) { return r.rowsAffected, nil }

func registerFakeTimerDriver() {
	fakeTimerDriverOnce.Do(func() { sql.Register(fakeTimerDriverName, &fakeTimerDriver{}) })
}

func timerStringValue(v driver.NamedValue) string {
	if v.Value == nil {
		return ""
	}
	return v.Value.(string)
}
func timerIntValue(v driver.NamedValue) int64 {
	switch value := v.Value.(type) {
	case int64:
		return value
	case int:
		return int64(value)
	default:
		return 0
	}
}
func timerTimeValue(v driver.NamedValue) time.Time {
	value, _ := v.Value.(time.Time)
	return value
}

func newFakeTimerProvider(t *testing.T) (*Provider, *fakeTimerStore, func()) {
	t.Helper()
	registerFakeTimerDriver()
	store := &fakeTimerStore{nextID: 1, timers: map[int64]fakeTimerRow{}}
	dsn := t.Name()
	fakeTimerStores.Store(dsn, store)
	db, err := sql.Open(fakeTimerDriverName, dsn)
	require.NoError(t, err)
	provider := NewProvider()
	provider.connect = func(ctx context.Context, _ string) (*sql.Conn, error) { return db.Conn(ctx) }
	return provider, store, func() { require.NoError(t, db.Close()); fakeTimerStores.Delete(dsn) }
}

func TestTimerDefinitionScopedNameLookups(t *testing.T) {
	provider, _, cleanup := newFakeTimerProvider(t)
	defer cleanup()
	ctx := context.Background()
	alice := UserScope{User: "alice"}
	bob := UserScope{User: "bob"}

	timer := &TimerDefinition{Name: "timer-name", ExecUser: "alice", Task: "task", Schedule: "@every 1m"}
	require.NoError(t, provider.SaveTimerForUser(ctx, alice, timer))

	loaded, err := provider.LoadTimerByNameForUser(ctx, alice, "TIMER-NAME")
	require.NoError(t, err)
	require.Equal(t, timer.Id, loaded.Id)
	_, err = provider.LoadTimerByNameForUser(ctx, bob, "TIMER-NAME")
	require.EqualError(t, err, "timer 'TIMER-NAME' not found")

	require.ErrorIs(t, provider.RemoveTimerByID(ctx, 999), os.ErrNotExist)
}

func TestTimerDefinitionPersistence(t *testing.T) {
	provider, store, cleanup := newFakeTimerProvider(t)
	defer cleanup()
	ctx := context.Background()
	attributes := json.RawMessage(`{"retry":3}`)
	timer := &TimerDefinition{UserName: "alice", Name: "timer-a", ExecUser: "alice", ContinueOnError: true, Disabled: true, LastError: "timer failed", LastErrorAt: time.Now(), Task: "timer.tql", Schedule: "@every 1m", Attributes: attributes}
	require.NoError(t, provider.SaveTimer(ctx, timer))
	require.Equal(t, int64(1), timer.Id)
	loaded, err := provider.LoadTimerByID(ctx, timer.Id)
	require.NoError(t, err)
	require.Equal(t, attributes, loaded.Attributes)
	require.True(t, loaded.Disabled)
	require.True(t, loaded.ContinueOnError)
	require.Equal(t, "timer failed", loaded.LastError)
	require.NoError(t, provider.SetTimerRuntimeError(ctx, timer.Id, "new failure"))
	loaded.Task = "timer-updated.tql"
	loaded.ContinueOnError = false
	require.NoError(t, provider.SaveTimer(ctx, loaded))
	loaded, err = provider.LoadTimer(ctx, "timer-a")
	require.NoError(t, err)
	require.Equal(t, "timer-updated.tql", loaded.Task)
	require.False(t, loaded.ContinueOnError)
	require.Equal(t, "new failure", loaded.LastError)
	timers, err := provider.LoadAllTimers(ctx)
	require.NoError(t, err)
	require.Len(t, timers, 1)
	require.NoError(t, provider.SetTimerRuntimeError(ctx, timer.Id, ""))
	require.NoError(t, provider.RemoveTimerByID(ctx, timer.Id))
	require.Empty(t, store.timers)
}

func TestTimerDefinitionUpgradePreservesRowsAndRetries(t *testing.T) {
	provider, store, cleanup := newFakeTimerProvider(t)
	defer cleanup()
	store.missingContinueOnError = true
	store.timers[7] = fakeTimerRow{id: 7, userName: "ALICE", name: "OLD", execUser: "ALICE", task: "old.tql", schedule: "@every 1m", lastError: "old failure"}
	store.checkError = errors.New("connection failed")
	_, err := provider.LoadAllTimers(context.Background())
	require.ErrorContains(t, err, "connection failed")
	require.Zero(t, store.alterCount)
	store.checkError = nil
	store.alterError = errors.New("alter denied")
	_, err = provider.LoadAllTimers(context.Background())
	require.ErrorContains(t, err, "alter denied")
	require.False(t, provider.timerTableReady)
	store.alterError = nil
	timers, err := provider.LoadAllTimers(context.Background())
	require.NoError(t, err)
	require.Len(t, timers, 1)
	require.True(t, timers[0].ContinueOnError)
	require.Equal(t, "old failure", timers[0].LastError)
	require.Equal(t, "old.tql", timers[0].Task)
	require.Equal(t, 1, store.alterCount)
	_, err = provider.LoadAllTimers(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, store.alterCount)
}

func TestTimerDefinitionTableResetAndOwnership(t *testing.T) {
	provider, store, cleanup := newFakeTimerProvider(t)
	defer cleanup()
	store.missingUserName = true
	ctx := context.Background()
	alice := UserScope{User: "alice"}
	bob := UserScope{User: "bob"}
	timer := &TimerDefinition{Name: "shared-name", ExecUser: "ALICE", Task: "timer.tql", Schedule: "@every 1m"}
	require.NoError(t, provider.SaveTimerForUser(ctx, alice, timer))
	require.Equal(t, "ALICE", timer.UserName)
	require.Equal(t, 1, store.dropTables)
	timers, err := provider.LoadTimers(ctx, alice)
	require.NoError(t, err)
	require.Len(t, timers, 1)
	require.Empty(t, mustLoadTimers(t, provider, bob))
	_, err = provider.LoadTimerForUser(ctx, bob, timer.Id)
	require.Error(t, err)
	require.ErrorIs(t, provider.RemoveTimerForUser(ctx, bob, timer.Id), sql.ErrNoRows)
	require.NoError(t, provider.RemoveTimerForUser(ctx, alice, timer.Id))
}

// NAME duplicates are allowed both within the timer table and against a
// subscriber sharing the same name; only ID identifies a definition.
func TestTimerDefinitionAllowsDuplicateNames(t *testing.T) {
	provider, _, cleanup := newFakeTimerProvider(t)
	defer cleanup()
	ctx := context.Background()
	firstTimer := &TimerDefinition{UserName: "alice", Name: "same", ExecUser: "alice", Task: "task", Schedule: "@every 1m"}
	secondTimer := &TimerDefinition{UserName: "bob", Name: "same", ExecUser: "bob", Task: "task", Schedule: "@every 1m"}
	require.NoError(t, provider.SaveTimer(ctx, firstTimer))
	require.NoError(t, provider.SaveTimer(ctx, secondTimer))
	require.NotEqual(t, firstTimer.Id, secondTimer.Id)

	subscriberProvider, _, subscriberCleanup := newFakeSubscriberProvider(t)
	defer subscriberCleanup()
	subscriber := &SubscriberDefinition{UserName: "sys", Name: "same", ExecUser: "sys", Task: "task", Bridge: "mqtt", Topic: "topic"}
	require.NoError(t, subscriberProvider.SaveSubscriber(ctx, subscriber))
}

func mustLoadTimers(t *testing.T, provider *Provider, scope UserScope) []*TimerDefinition {
	t.Helper()
	definitions, err := provider.LoadTimers(context.Background(), scope)
	require.NoError(t, err)
	return definitions
}
