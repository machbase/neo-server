package model

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"os"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

const fakeBridgeDriverName = "model_bridge_fake"

var fakeBridgeDriverOnce sync.Once
var fakeBridgeStores sync.Map

type fakeBridgeStore struct {
	mu           sync.Mutex
	nextID       int64
	rows         map[int64]fakeBridgeRow
	connectUsers []string
}

type fakeBridgeRow struct {
	ID          int64
	Owner       string
	IsPublic    int64
	AllowedUser string
	Name        string
	Type        string
	Path        string
}

type fakeBridgeDriver struct{}

func (d *fakeBridgeDriver) Open(name string) (driver.Conn, error) {
	return nil, errors.New("use OpenConnector")
}

func (d *fakeBridgeDriver) OpenConnector(name string) (driver.Connector, error) {
	storeAny, ok := fakeBridgeStores.Load(name)
	if !ok {
		return nil, errors.New("fake bridge store not found")
	}
	return &fakeBridgeConnector{store: storeAny.(*fakeBridgeStore)}, nil
}

type fakeBridgeConnector struct {
	store *fakeBridgeStore
}

func (c *fakeBridgeConnector) Connect(ctx context.Context) (driver.Conn, error) {
	return &fakeBridgeConn{store: c.store}, nil
}

func (c *fakeBridgeConnector) Driver() driver.Driver { return &fakeBridgeDriver{} }

type fakeBridgeConn struct {
	store *fakeBridgeStore
}

func (c *fakeBridgeConn) Prepare(query string) (driver.Stmt, error) {
	return nil, errors.New("prepare is not supported")
}

func (c *fakeBridgeConn) Close() error { return nil }

func (c *fakeBridgeConn) Begin() (driver.Tx, error) { return nil, errors.New("tx is not supported") }

func (c *fakeBridgeConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	normalizedQuery := strings.ToUpper(strings.TrimSpace(query))
	switch {
	case strings.HasPrefix(normalizedQuery, "CREATE TABLE"):
		return fakeBridgeResult{}, nil
	case strings.HasPrefix(normalizedQuery, "INSERT INTO _NEO_BRIDGE_DEF"):
		return c.insert(args)
	case strings.HasPrefix(normalizedQuery, "UPDATE _NEO_BRIDGE_DEF"):
		return c.update(args)
	case strings.HasPrefix(normalizedQuery, "DELETE FROM _NEO_BRIDGE_DEF"):
		return c.delete(args)
	default:
		return nil, errors.New("unexpected exec: " + query)
	}
}

func (c *fakeBridgeConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	normalizedQuery := strings.ToUpper(strings.TrimSpace(query))
	switch {
	case strings.Contains(normalizedQuery, "SELECT ID FROM _NEO_BRIDGE_DEF WHERE OWNER_NAME = ? AND NAME = ?"):
		return c.queryByOwnerAndName(args)
	case strings.Contains(normalizedQuery, "WHERE NAME = ? AND (OWNER_NAME = ? OR IS_PUBLIC = 1 OR ALLOWED_USER = ?)"):
		return c.queryByName(args)
	case strings.Contains(normalizedQuery, "WHERE OWNER_NAME = ? OR IS_PUBLIC = 1 OR ALLOWED_USER = ?"):
		return c.queryVisible(args)
	case strings.Contains(normalizedQuery, "FROM _NEO_BRIDGE_DEF ORDER BY ID"):
		return c.queryAll(args)
	default:
		return nil, errors.New("unexpected query: " + query)
	}
}

func (c *fakeBridgeConn) insert(args []driver.NamedValue) (driver.Result, error) {
	if len(args) != 6 {
		return nil, errors.New("invalid insert args")
	}
	c.store.mu.Lock()
	defer c.store.mu.Unlock()
	id := c.store.nextID
	c.store.nextID++
	c.store.rows[id] = fakeBridgeRow{
		ID:          id,
		Owner:       bridgeToString(args[0].Value),
		IsPublic:    bridgeToInt64(args[1].Value),
		AllowedUser: bridgeToString(args[2].Value),
		Name:        bridgeToString(args[3].Value),
		Type:        bridgeToString(args[4].Value),
		Path:        bridgeToString(args[5].Value),
	}
	return fakeBridgeResult{lastInsertID: id, rowsAffected: 1}, nil
}

func (c *fakeBridgeConn) update(args []driver.NamedValue) (driver.Result, error) {
	if len(args) != 7 {
		return nil, errors.New("invalid update args")
	}
	owner := bridgeToString(args[5].Value)
	id := bridgeToInt64(args[6].Value)
	c.store.mu.Lock()
	defer c.store.mu.Unlock()
	row, ok := c.store.rows[id]
	if !ok || row.Owner != owner {
		return fakeBridgeResult{rowsAffected: 0}, nil
	}
	row.IsPublic = bridgeToInt64(args[0].Value)
	row.AllowedUser = bridgeToString(args[1].Value)
	row.Name = bridgeToString(args[2].Value)
	row.Type = bridgeToString(args[3].Value)
	row.Path = bridgeToString(args[4].Value)
	c.store.rows[id] = row
	return fakeBridgeResult{rowsAffected: 1}, nil
}

func (c *fakeBridgeConn) delete(args []driver.NamedValue) (driver.Result, error) {
	if len(args) != 2 {
		return nil, errors.New("invalid delete args")
	}
	owner := bridgeToString(args[0].Value)
	name := bridgeToString(args[1].Value)
	c.store.mu.Lock()
	defer c.store.mu.Unlock()
	for id, row := range c.store.rows {
		if row.Owner == owner && row.Name == name {
			delete(c.store.rows, id)
			return fakeBridgeResult{rowsAffected: 1}, nil
		}
	}
	return fakeBridgeResult{rowsAffected: 0}, nil
}

func (c *fakeBridgeConn) queryByOwnerAndName(args []driver.NamedValue) (driver.Rows, error) {
	if len(args) != 2 {
		return nil, errors.New("invalid select by owner+name args")
	}
	owner := bridgeToString(args[0].Value)
	name := bridgeToString(args[1].Value)
	c.store.mu.Lock()
	defer c.store.mu.Unlock()
	for _, row := range c.store.rows {
		if row.Owner == owner && row.Name == name {
			return &fakeBridgeSingleColumnRows{rows: []int64{row.ID}}, nil
		}
	}
	return &fakeBridgeSingleColumnRows{}, nil
}

type fakeBridgeSingleColumnRows struct {
	rows []int64
	idx  int
}

func (r *fakeBridgeSingleColumnRows) Columns() []string { return []string{"ID"} }
func (r *fakeBridgeSingleColumnRows) Close() error      { return nil }
func (r *fakeBridgeSingleColumnRows) Next(dest []driver.Value) error {
	if r.idx >= len(r.rows) {
		return io.EOF
	}
	dest[0] = r.rows[r.idx]
	r.idx++
	return nil
}

func (c *fakeBridgeConn) queryByName(args []driver.NamedValue) (driver.Rows, error) {
	if len(args) != 3 {
		return nil, errors.New("invalid select by name args")
	}
	name := bridgeToString(args[0].Value)
	user := bridgeToString(args[1].Value)
	_ = bridgeToString(args[2].Value)
	c.store.mu.Lock()
	defer c.store.mu.Unlock()
	for _, row := range c.store.rows {
		if row.Name == name && (row.Owner == user || row.IsPublic != 0 || row.AllowedUser == user) {
			return &fakeBridgeRows{rows: []fakeBridgeRow{row}}, nil
		}
	}
	return &fakeBridgeRows{}, nil
}

func (c *fakeBridgeConn) queryVisible(args []driver.NamedValue) (driver.Rows, error) {
	if len(args) != 2 {
		return nil, errors.New("invalid visible bridge args")
	}
	user := bridgeToString(args[0].Value)
	_ = bridgeToString(args[1].Value)
	c.store.mu.Lock()
	defer c.store.mu.Unlock()
	rows := []fakeBridgeRow{}
	for _, row := range c.store.rows {
		if row.Owner == user || row.IsPublic != 0 || row.AllowedUser == user {
			rows = append(rows, row)
		}
	}
	sortBridgeRows(rows)
	return &fakeBridgeRows{rows: rows}, nil
}

func (c *fakeBridgeConn) queryAll(args []driver.NamedValue) (driver.Rows, error) {
	_ = args
	c.store.mu.Lock()
	defer c.store.mu.Unlock()
	rows := []fakeBridgeRow{}
	for _, row := range c.store.rows {
		rows = append(rows, row)
	}
	sortBridgeRows(rows)
	return &fakeBridgeRows{rows: rows}, nil
}

func sortBridgeRows(rows []fakeBridgeRow) {
	sort.Slice(rows, func(i, j int) bool { return rows[i].ID < rows[j].ID })
}

type fakeBridgeResult struct {
	lastInsertID int64
	rowsAffected int64
}

func (r fakeBridgeResult) LastInsertId() (int64, error) { return r.lastInsertID, nil }
func (r fakeBridgeResult) RowsAffected() (int64, error) { return r.rowsAffected, nil }

type fakeBridgeRows struct {
	rows []fakeBridgeRow
	idx  int
}

func (r *fakeBridgeRows) Columns() []string {
	return []string{"ID", "OWNER_NAME", "IS_PUBLIC", "ALLOWED_USER", "NAME", "TYPE", "PATH"}
}

func (r *fakeBridgeRows) Close() error { return nil }

func (r *fakeBridgeRows) Next(dest []driver.Value) error {
	if r.idx >= len(r.rows) {
		return io.EOF
	}
	row := r.rows[r.idx]
	r.idx++
	dest[0] = row.ID
	dest[1] = row.Owner
	dest[2] = row.IsPublic
	dest[3] = row.AllowedUser
	dest[4] = row.Name
	dest[5] = row.Type
	dest[6] = row.Path
	return nil
}

func registerFakeBridgeDriver() {
	fakeBridgeDriverOnce.Do(func() {
		sql.Register(fakeBridgeDriverName, &fakeBridgeDriver{})
	})
}

func newFakeBridgeProvider(t *testing.T) (*Provider, *fakeBridgeStore, func()) {
	t.Helper()
	registerFakeBridgeDriver()
	store := &fakeBridgeStore{nextID: 1, rows: map[int64]fakeBridgeRow{}}
	dsn := t.Name()
	fakeBridgeStores.Store(dsn, store)
	db, err := sql.Open(fakeBridgeDriverName, dsn)
	require.NoError(t, err)
	provider := NewProvider(WithConfigDirPath(t.TempDir()))
	provider.connect = func(ctx context.Context, user string) (*sql.Conn, error) {
		store.mu.Lock()
		store.connectUsers = append(store.connectUsers, user)
		store.mu.Unlock()
		return db.Conn(ctx)
	}
	require.NoError(t, provider.Start())
	return provider, store, func() {
		provider.Stop()
		require.NoError(t, db.Close())
		fakeBridgeStores.Delete(dsn)
	}
}

func TestBridgeDefinitionTablePersistence(t *testing.T) {
	provider, store, cleanup := newFakeBridgeProvider(t)
	defer cleanup()

	ctx := context.Background()
	alice := UserScope{User: "alice"}
	bob := UserScope{User: "bob"}
	charlie := UserScope{User: "charlie"}

	def := &BridgeDefinition{
		Type: BRIDGE_SQLITE,
		Name: "alice_sqlite",
		Path: "file::memory:?cache=shared",
	}
	require.NoError(t, provider.SaveBridge(ctx, alice, def))
	require.NotZero(t, def.Id)

	loaded, err := provider.LoadBridge(ctx, alice, def.Name)
	require.NoError(t, err)
	require.Equal(t, def.Id, loaded.Id)
	require.Equal(t, strings.ToUpper(alice.User), loaded.Owner)
	require.Equal(t, def.Name, loaded.Name)
	require.Equal(t, def.Path, loaded.Path)
	require.False(t, loaded.IsPublic)
	require.Empty(t, loaded.AllowedUser)

	def.IsPublic = true
	def.AllowedUser = charlie.User
	require.NoError(t, provider.SaveBridge(ctx, alice, def))

	publicLoaded, err := provider.LoadBridge(ctx, bob, def.Name)
	require.NoError(t, err)
	require.True(t, publicLoaded.IsPublic)
	require.Equal(t, charlie.User, publicLoaded.AllowedUser)

	charlieLoaded, err := provider.LoadBridge(ctx, charlie, def.Name)
	require.NoError(t, err)
	require.Equal(t, def.Name, charlieLoaded.Name)

	all, err := provider.LoadAllBridges(ctx, bob)
	require.NoError(t, err)
	require.Len(t, all, 1)
	require.Equal(t, def.Name, all[0].Name)

	bootstrapAll, err := provider.LoadAllBridgesForBootstrap(ctx)
	require.NoError(t, err)
	require.Len(t, bootstrapAll, 1)
	require.Equal(t, def.Name, bootstrapAll[0].Name)

	require.ErrorIs(t, provider.RemoveBridge(ctx, bob, def.Name), os.ErrNotExist)
	require.NoError(t, provider.RemoveBridge(ctx, alice, def.Name))
	_, err = provider.LoadBridge(ctx, alice, def.Name)
	require.ErrorContains(t, err, "not found")

	store.mu.Lock()
	connectUsers := append([]string{}, store.connectUsers...)
	store.mu.Unlock()
	require.NotEmpty(t, connectUsers)
	for _, user := range connectUsers {
		require.Equal(t, "sys", user)
	}
}

func TestBridgeDefinitionValidationAndOwnership(t *testing.T) {
	provider, _, cleanup := newFakeBridgeProvider(t)
	defer cleanup()

	ctx := context.Background()
	alice := UserScope{User: "alice"}

	require.ErrorContains(t, provider.SaveBridge(ctx, alice, nil), "bridge definition not specified")
	require.ErrorContains(t, provider.SaveBridge(ctx, alice, &BridgeDefinition{Name: "", Type: BRIDGE_SQLITE, Path: "x"}), "bridge name not specified")
	require.ErrorContains(t, provider.SaveBridge(ctx, alice, &BridgeDefinition{Name: "bad", Type: BridgeType("bad"), Path: "x"}), "unsupported bridge type")
	require.ErrorContains(t, provider.SaveBridge(ctx, alice, &BridgeDefinition{Name: "bad", Type: BRIDGE_SQLITE, Path: ""}), "bridge path not specified")

	def := &BridgeDefinition{Type: BRIDGE_SQLITE, Name: "dupe", Path: "file::memory:?cache=shared"}
	require.NoError(t, provider.SaveBridge(ctx, alice, def))
	dup := &BridgeDefinition{Type: BRIDGE_SQLITE, Name: "dupe", Path: "file::memory:?cache=shared"}
	require.ErrorContains(t, provider.SaveBridge(ctx, alice, dup), "already exists")

	other := UserScope{User: "mallory"}
	require.ErrorIs(t, provider.SaveBridge(ctx, other, &BridgeDefinition{Id: def.Id, Name: "dupe", Type: BRIDGE_SQLITE, Path: "file::memory:?cache=shared"}), os.ErrNotExist)

	_, err := provider.LoadBridge(ctx, alice, "missing")
	require.ErrorContains(t, err, "not found")
	require.NoError(t, provider.SaveBridge(ctx, alice, &BridgeDefinition{Id: def.Id, Name: "dupe", Type: BRIDGE_SQLITE, Path: "updated::memory", IsPublic: false}))
	updated, err := provider.LoadBridge(ctx, alice, "dupe")
	require.NoError(t, err)
	require.Equal(t, "updated::memory", updated.Path)
}

func TestBridgeDefinitionHelpers(t *testing.T) {
	require.Equal(t, int64(1), boolToShort(true))
	require.Equal(t, int64(0), boolToShort(false))
	require.Nil(t, nullIfEmpty(" "))
	require.Equal(t, "alice", nullIfEmpty("alice"))
	_, err := ParseBridgeType("sqlite3")
	require.NoError(t, err)
	_, err = ParseBridgeType("postgresql")
	require.NoError(t, err)
	_, err = ParseBridgeType("bad")
	require.ErrorContains(t, err, "unsupported bridge type")
}

func bridgeToString(value any) string {
	if value == nil {
		return ""
	}
	return value.(string)
}

func bridgeToInt64(value any) int64 {
	switch v := value.(type) {
	case int64:
		return v
	case int:
		return int64(v)
	case int32:
		return int64(v)
	case int16:
		return int64(v)
	case int8:
		return int64(v)
	case uint64:
		return int64(v)
	case uint:
		return int64(v)
	case uint32:
		return int64(v)
	case uint16:
		return int64(v)
	case uint8:
		return int64(v)
	case nil:
		return 0
	default:
		return 0
	}
}
