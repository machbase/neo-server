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

const fakeShellDriverName = "model_shell_fake"

var fakeShellDriverOnce sync.Once
var fakeShellStores sync.Map

func registerFakeShellDriver() {
	fakeShellDriverOnce.Do(func() {
		sql.Register(fakeShellDriverName, &fakeShellDriver{})
	})
}

type fakeShellStore struct {
	mu           sync.Mutex
	nextID       int64
	rows         map[int64]fakeShellRow
	connectUsers []string
}

type fakeShellRow struct {
	ID         int64
	UserName   string
	Type       string
	Icon       string
	Label      string
	Theme      string
	Command    string
	Attributes string
}

type fakeShellDriver struct{}

func (d *fakeShellDriver) Open(name string) (driver.Conn, error) {
	return nil, errors.New("use OpenConnector")
}

func (d *fakeShellDriver) OpenConnector(name string) (driver.Connector, error) {
	storeAny, ok := fakeShellStores.Load(name)
	if !ok {
		return nil, errors.New("fake shell store not found")
	}
	return &fakeShellConnector{store: storeAny.(*fakeShellStore)}, nil
}

type fakeShellConnector struct {
	store *fakeShellStore
}

func (c *fakeShellConnector) Connect(ctx context.Context) (driver.Conn, error) {
	return &fakeShellConn{store: c.store}, nil
}

func (c *fakeShellConnector) Driver() driver.Driver { return &fakeShellDriver{} }

type fakeShellConn struct {
	store *fakeShellStore
}

func (c *fakeShellConn) Prepare(query string) (driver.Stmt, error) {
	return nil, errors.New("prepare is not supported")
}
func (c *fakeShellConn) Close() error              { return nil }
func (c *fakeShellConn) Begin() (driver.Tx, error) { return nil, errors.New("tx is not supported") }

func (c *fakeShellConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	normalizedQuery := strings.ToUpper(strings.TrimSpace(query))
	switch {
	case strings.HasPrefix(normalizedQuery, "CREATE TABLE"):
		return fakeShellResult{}, nil
	case strings.HasPrefix(normalizedQuery, "INSERT INTO _NEO_SHELL_DEF"):
		return c.insert(args)
	case strings.HasPrefix(normalizedQuery, "UPDATE _NEO_SHELL_DEF"):
		return c.update(args)
	case strings.HasPrefix(normalizedQuery, "DELETE FROM _NEO_SHELL_DEF"):
		return c.delete(args)
	default:
		return nil, errors.New("unexpected exec: " + query)
	}
}

func (c *fakeShellConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	normalizedQuery := strings.ToUpper(strings.TrimSpace(query))
	switch {
	case strings.Contains(normalizedQuery, "WHERE USER_NAME = ? AND ID = ?"):
		return c.queryByID(args)
	case strings.Contains(normalizedQuery, "WHERE USER_NAME = ? ORDER BY ID"):
		return c.queryByUser(args)
	default:
		return nil, errors.New("unexpected query: " + query)
	}
}

func (c *fakeShellConn) insert(args []driver.NamedValue) (driver.Result, error) {
	if len(args) != 7 {
		return nil, errors.New("invalid insert args")
	}
	c.store.mu.Lock()
	defer c.store.mu.Unlock()
	id := c.store.nextID
	c.store.nextID++
	c.store.rows[id] = fakeShellRow{
		ID:         id,
		UserName:   toString(args[0].Value),
		Type:       toString(args[1].Value),
		Icon:       toString(args[2].Value),
		Label:      toString(args[3].Value),
		Theme:      toString(args[4].Value),
		Command:    toString(args[5].Value),
		Attributes: toString(args[6].Value),
	}
	return fakeShellResult{lastInsertID: id, rowsAffected: 1}, nil
}

func (c *fakeShellConn) update(args []driver.NamedValue) (driver.Result, error) {
	if len(args) != 8 {
		return nil, errors.New("invalid update args")
	}
	user := toString(args[6].Value)
	id := toInt64(args[7].Value)
	c.store.mu.Lock()
	defer c.store.mu.Unlock()
	row, ok := c.store.rows[id]
	if !ok || row.UserName != user {
		return fakeShellResult{rowsAffected: 0}, nil
	}
	row.Type = toString(args[0].Value)
	row.Icon = toString(args[1].Value)
	row.Label = toString(args[2].Value)
	row.Theme = toString(args[3].Value)
	row.Command = toString(args[4].Value)
	row.Attributes = toString(args[5].Value)
	c.store.rows[id] = row
	return fakeShellResult{rowsAffected: 1}, nil
}

func (c *fakeShellConn) delete(args []driver.NamedValue) (driver.Result, error) {
	if len(args) != 2 {
		return nil, errors.New("invalid delete args")
	}
	user := toString(args[0].Value)
	id := toInt64(args[1].Value)
	c.store.mu.Lock()
	defer c.store.mu.Unlock()
	row, ok := c.store.rows[id]
	if !ok || row.UserName != user {
		return fakeShellResult{rowsAffected: 0}, nil
	}
	delete(c.store.rows, id)
	return fakeShellResult{rowsAffected: 1}, nil
}

func (c *fakeShellConn) queryByID(args []driver.NamedValue) (driver.Rows, error) {
	if len(args) != 2 {
		return nil, errors.New("invalid select by id args")
	}
	user := toString(args[0].Value)
	id := toInt64(args[1].Value)
	c.store.mu.Lock()
	defer c.store.mu.Unlock()
	row, ok := c.store.rows[id]
	if !ok || row.UserName != user {
		return &fakeShellRows{}, nil
	}
	return &fakeShellRows{rows: []fakeShellRow{row}}, nil
}

func (c *fakeShellConn) queryByUser(args []driver.NamedValue) (driver.Rows, error) {
	if len(args) != 1 {
		return nil, errors.New("invalid select by user args")
	}
	user := toString(args[0].Value)
	c.store.mu.Lock()
	defer c.store.mu.Unlock()
	rows := []fakeShellRow{}
	for _, row := range c.store.rows {
		if row.UserName == user {
			rows = append(rows, row)
		}
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].ID < rows[j].ID })
	return &fakeShellRows{rows: rows}, nil
}

type fakeShellResult struct {
	lastInsertID int64
	rowsAffected int64
}

func (r fakeShellResult) LastInsertId() (int64, error) { return r.lastInsertID, nil }
func (r fakeShellResult) RowsAffected() (int64, error) { return r.rowsAffected, nil }

type fakeShellRows struct {
	rows []fakeShellRow
	idx  int
}

func (r *fakeShellRows) Columns() []string {
	return []string{"ID", "TYPE", "ICON", "LABEL", "THEME", "COMMAND", "ATTRIBUTES"}
}

func (r *fakeShellRows) Close() error { return nil }

func (r *fakeShellRows) Next(dest []driver.Value) error {
	if r.idx >= len(r.rows) {
		return io.EOF
	}
	row := r.rows[r.idx]
	r.idx++
	dest[0] = row.ID
	dest[1] = row.Type
	dest[2] = row.Icon
	dest[3] = row.Label
	dest[4] = row.Theme
	dest[5] = row.Command
	dest[6] = row.Attributes
	return nil
}

func toString(value any) string {
	if value == nil {
		return ""
	}
	return value.(string)
}

func toInt64(value any) int64 {
	switch v := value.(type) {
	case int64:
		return v
	case int:
		return int64(v)
	default:
		return 0
	}
}

func newFakeShellProvider(t *testing.T) (*Provider, *fakeShellStore, func()) {
	t.Helper()
	registerFakeShellDriver()
	store := &fakeShellStore{nextID: 1, rows: map[int64]fakeShellRow{}}
	dsn := t.Name()
	fakeShellStores.Store(dsn, store)
	db, err := sql.Open(fakeShellDriverName, dsn)
	require.NoError(t, err)
	provider := NewProvider()
	provider.connect = func(ctx context.Context, user string) (*sql.Conn, error) {
		store.mu.Lock()
		store.connectUsers = append(store.connectUsers, user)
		store.mu.Unlock()
		return db.Conn(ctx)
	}
	return provider, store, func() {
		require.NoError(t, db.Close())
		fakeShellStores.Delete(dsn)
	}
}

func TestShellAttributesRoundTrip(t *testing.T) {
	attrs := &ShellAttributes{Removable: true, Cloneable: true, Editable: true}
	data, err := json.Marshal(attrs)
	require.NoError(t, err)
	require.JSONEq(t, `[ {"removable": true}, {"cloneable": true}, {"editable": true} ]`, string(data))
	decoded := &ShellAttributes{}
	require.NoError(t, json.Unmarshal(data, decoded))
	require.True(t, decoded.Removable)
	require.True(t, decoded.Cloneable)
	require.True(t, decoded.Editable)

	require.NoError(t, json.Unmarshal([]byte(`{"removable": true, "cloneable": true, "editable": true}`), decoded))
	require.True(t, decoded.Removable)
	require.True(t, decoded.Cloneable)
	require.True(t, decoded.Editable)
}

func TestShellAttributesDBJSON(t *testing.T) {
	attrs := &ShellAttributes{Removable: true, Cloneable: true, Editable: true}
	data, err := marshalShellAttributesForDB(attrs)
	require.NoError(t, err)
	require.JSONEq(t, `{"removable": true, "cloneable": true, "editable": true}`, data)
	require.NotContains(t, data, "[")

	decoded, err := unmarshalShellAttributesFromDB(data)
	require.NoError(t, err)
	require.NotNil(t, decoded)
	require.True(t, decoded.Removable)
	require.True(t, decoded.Cloneable)
	require.True(t, decoded.Editable)
}

func TestShellDefinitionTablePersistence(t *testing.T) {
	provider, store, cleanup := newFakeShellProvider(t)
	defer cleanup()
	ctx := context.Background()
	alice := UserScope{User: "alice"}
	bob := UserScope{User: "bob"}
	exe, err := os.Executable()
	require.NoError(t, err)

	def := &ShellDefinition{
		Type:    SHELL_TERM,
		Label:   "custom",
		Command: exe,
		Attributes: &ShellAttributes{
			Removable: true,
			Cloneable: true,
			Editable:  true,
		},
	}
	require.NoError(t, provider.SaveShell(ctx, alice, def))
	require.Equal(t, "1", def.Id)

	loaded, err := provider.GetShell(ctx, alice, def.Id)
	require.NoError(t, err)
	require.NotNil(t, loaded)
	require.Equal(t, "custom", loaded.Label)
	require.Equal(t, exe, loaded.Command)
	require.True(t, loaded.Attributes.Removable)
	require.True(t, loaded.Attributes.Cloneable)
	require.True(t, loaded.Attributes.Editable)

	otherUser, err := provider.GetShell(ctx, bob, def.Id)
	require.NoError(t, err)
	require.Nil(t, otherUser)

	all, err := provider.GetAllShells(ctx, alice, false)
	require.NoError(t, err)
	require.Len(t, all, 1)
	require.Equal(t, def.Id, all[0].Id)

	def.Label = "updated"
	require.NoError(t, provider.SaveShell(ctx, alice, def))
	loaded, err = provider.GetShell(ctx, alice, def.Id)
	require.NoError(t, err)
	require.Equal(t, "updated", loaded.Label)

	require.ErrorIs(t, provider.RemoveShell(ctx, bob, def.Id), os.ErrNotExist)
	require.NoError(t, provider.RemoveShell(ctx, alice, def.Id))
	loaded, err = provider.GetShell(ctx, alice, def.Id)
	require.NoError(t, err)
	require.Nil(t, loaded)

	store.mu.Lock()
	connectUsers := append([]string{}, store.connectUsers...)
	store.mu.Unlock()
	require.NotEmpty(t, connectUsers)
	for _, user := range connectUsers {
		require.Equal(t, "sys", user)
	}
}

func TestShellDefinitionInvalidIds(t *testing.T) {
	_, err := parseCustomShellId("")
	require.Error(t, err)
	_, err = parseCustomShellId("missing-shell")
	require.Error(t, err)
	_, err = parseCustomShellId(SHELLID_SQL)
	require.Error(t, err)
	id, err := parseCustomShellId("42")
	require.NoError(t, err)
	require.Equal(t, int64(42), id)
	require.Equal(t, "42", formatCustomShellId(id))
}

func TestShellDefinitionReservedLabelRejected(t *testing.T) {
	provider, _, cleanup := newFakeShellProvider(t)
	defer cleanup()
	exe, err := os.Executable()
	require.NoError(t, err)
	err = provider.SaveShell(context.Background(), UserScope{User: "sys"}, &ShellDefinition{
		Type:    SHELL_TERM,
		Label:   "sql",
		Command: exe,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "not allowed")
}
