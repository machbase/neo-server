package model

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const fakeApiTokenDriverName = "model_apitoken_fake"

var fakeApiTokenDriverOnce sync.Once
var fakeApiTokenStores sync.Map

type fakeApiTokenStore struct {
	mu           sync.Mutex
	nextID       int64
	tokens       map[int64][]driver.Value
	connectUsers []string
}

type fakeApiTokenDriver struct{}

func (d *fakeApiTokenDriver) Open(string) (driver.Conn, error) {
	return nil, errors.New("use OpenConnector")
}
func (d *fakeApiTokenDriver) OpenConnector(name string) (driver.Connector, error) {
	store, ok := fakeApiTokenStores.Load(name)
	if !ok {
		return nil, errors.New("api token store not found")
	}
	return &fakeApiTokenConnector{store: store.(*fakeApiTokenStore)}, nil
}

type fakeApiTokenConnector struct{ store *fakeApiTokenStore }

func (c *fakeApiTokenConnector) Connect(context.Context) (driver.Conn, error) {
	return &fakeApiTokenConn{store: c.store}, nil
}
func (c *fakeApiTokenConnector) Driver() driver.Driver { return &fakeApiTokenDriver{} }

type fakeApiTokenConn struct{ store *fakeApiTokenStore }

func (c *fakeApiTokenConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("prepare is not supported")
}
func (c *fakeApiTokenConn) Close() error { return nil }
func (c *fakeApiTokenConn) Begin() (driver.Tx, error) {
	return nil, errors.New("transactions are not supported")
}

func (c *fakeApiTokenConn) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	q := strings.ToUpper(strings.TrimSpace(query))
	c.store.mu.Lock()
	defer c.store.mu.Unlock()
	switch {
	case strings.HasPrefix(q, "CREATE TABLE"):
		return fakeApiTokenResult{}, nil
	case strings.HasPrefix(q, "INSERT INTO _NEO_API_TOKEN"):
		id := c.store.nextID
		c.store.nextID++
		c.store.tokens[id] = append([]driver.Value{id}, apiTokenNamedValues(args)...)
		return fakeApiTokenResult{lastInsertID: id, rowsAffected: 1}, nil
	case strings.HasPrefix(q, "UPDATE _NEO_API_TOKEN SET LAST_USED_AT"):
		id := apiTokenIntValue(args[1].Value)
		row, ok := c.store.tokens[id]
		if !ok {
			return fakeApiTokenResult{}, nil
		}
		row[8] = args[0].Value
		c.store.tokens[id] = row
		return fakeApiTokenResult{rowsAffected: 1}, nil
	case strings.HasPrefix(q, "UPDATE _NEO_API_TOKEN SET TOKEN_HINT"):
		id := apiTokenIntValue(args[1].Value)
		row, ok := c.store.tokens[id]
		if !ok || apiTokenStringValue(row[1]) != apiTokenStringValue(args[2].Value) {
			return fakeApiTokenResult{}, nil
		}
		row[4] = args[0].Value
		c.store.tokens[id] = row
		return fakeApiTokenResult{rowsAffected: 1}, nil
	case strings.HasPrefix(q, "DELETE FROM _NEO_API_TOKEN"):
		id := apiTokenIntValue(args[0].Value)
		row, ok := c.store.tokens[id]
		if !ok || apiTokenStringValue(row[1]) != apiTokenStringValue(args[1].Value) {
			return fakeApiTokenResult{}, nil
		}
		delete(c.store.tokens, id)
		return fakeApiTokenResult{rowsAffected: 1}, nil
	}
	return nil, errors.New("unexpected exec: " + query)
}

func (c *fakeApiTokenConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	q := strings.ToUpper(strings.TrimSpace(query))
	c.store.mu.Lock()
	defer c.store.mu.Unlock()
	switch {
	case strings.Contains(q, "FROM _NEO_API_TOKEN WHERE ID"):
		var rows [][]driver.Value
		if row, ok := c.store.tokens[apiTokenIntValue(args[0].Value)]; ok {
			rows = append(rows, row)
		}
		return &fakeApiTokenRows{values: rows}, nil
	case strings.Contains(q, "FROM _NEO_API_TOKEN WHERE USER_NAME"):
		var rows [][]driver.Value
		for _, row := range c.store.tokens {
			if apiTokenStringValue(row[1]) == apiTokenStringValue(args[0].Value) {
				rows = append(rows, row)
			}
		}
		return &fakeApiTokenRows{values: rows}, nil
	default:
		return nil, errors.New("unexpected query: " + query)
	}
}

type fakeApiTokenResult struct{ lastInsertID, rowsAffected int64 }

func (r fakeApiTokenResult) LastInsertId() (int64, error) { return r.lastInsertID, nil }
func (r fakeApiTokenResult) RowsAffected() (int64, error) { return r.rowsAffected, nil }

// fakeApiTokenRows always reports 10 columns, matching _NEO_API_TOKEN's SELECT list.
type fakeApiTokenRows struct {
	values [][]driver.Value
	index  int
}

func (r *fakeApiTokenRows) Columns() []string { return make([]string, 10) }
func (r *fakeApiTokenRows) Close() error      { return nil }
func (r *fakeApiTokenRows) Next(dest []driver.Value) error {
	if r.index >= len(r.values) {
		return io.EOF
	}
	copy(dest, r.values[r.index])
	r.index++
	return nil
}

func apiTokenNamedValues(values []driver.NamedValue) []driver.Value {
	result := make([]driver.Value, len(values))
	for index := range values {
		result[index] = values[index].Value
	}
	return result
}
func apiTokenStringValue(value any) string {
	if value == nil {
		return ""
	}
	return value.(string)
}
func apiTokenIntValue(value any) int64 {
	switch v := value.(type) {
	case int64:
		return v
	case int:
		return int64(v)
	}
	return 0
}

func newFakeApiTokenProvider(t *testing.T) (*Provider, *fakeApiTokenStore, func()) {
	t.Helper()
	fakeApiTokenDriverOnce.Do(func() { sql.Register(fakeApiTokenDriverName, &fakeApiTokenDriver{}) })
	store := &fakeApiTokenStore{nextID: 1, tokens: map[int64][]driver.Value{}}
	dsn := t.Name()
	fakeApiTokenStores.Store(dsn, store)
	db, err := sql.Open(fakeApiTokenDriverName, dsn)
	require.NoError(t, err)
	provider := NewProvider()
	provider.connect = func(ctx context.Context, user string) (*sql.Conn, error) {
		store.mu.Lock()
		store.connectUsers = append(store.connectUsers, user)
		store.mu.Unlock()
		return db.Conn(ctx)
	}
	return provider, store, func() { require.NoError(t, db.Close()); fakeApiTokenStores.Delete(dsn) }
}

func TestApiTokenDefinitionTablePersistence(t *testing.T) {
	provider, store, cleanup := newFakeApiTokenProvider(t)
	defer cleanup()
	ctx, alice, bob := context.Background(), UserScope{User: "alice"}, UserScope{User: "bob"}
	now := time.Now().Round(0)
	definition := &ApiTokenDefinition{Name: "CI", TokenHash: "hash", TokenHint: "hint", CreatedAt: now, NotBefore: now, NotAfter: now.Add(time.Hour), Attributes: json.RawMessage(`{"scope":"read"}`)}
	require.NoError(t, provider.SaveApiToken(ctx, alice, definition))
	require.Equal(t, int64(1), definition.Id)
	require.Equal(t, "ALICE", definition.UserName)
	loaded, err := provider.GetApiToken(ctx, definition.Id)
	require.NoError(t, err)
	require.Equal(t, definition.Name, loaded.Name)
	require.JSONEq(t, string(definition.Attributes), string(loaded.Attributes))
	require.True(t, loaded.LastUsedAt.IsZero())
	list, err := provider.GetAllApiTokens(ctx, alice)
	require.NoError(t, err)
	require.Len(t, list, 1)
	list, err = provider.GetAllApiTokens(ctx, bob)
	require.NoError(t, err)
	require.Empty(t, list)
	require.NoError(t, provider.UpdateApiTokenHint(ctx, alice, definition.Id, "new-hint"))
	require.NoError(t, provider.TouchApiToken(ctx, definition.Id, now))
	loaded, err = provider.GetApiToken(ctx, definition.Id)
	require.NoError(t, err)
	require.Equal(t, "new-hint", loaded.TokenHint)
	require.Equal(t, now, loaded.LastUsedAt)
	require.ErrorIs(t, provider.RemoveApiToken(ctx, bob, definition.Id), sql.ErrNoRows)
	require.NoError(t, provider.RemoveApiToken(ctx, alice, definition.Id))
	_, err = provider.GetApiToken(ctx, definition.Id)
	require.ErrorIs(t, err, sql.ErrNoRows)
	require.Error(t, provider.SaveApiToken(ctx, alice, nil))
	require.Error(t, provider.SaveApiToken(ctx, UserScope{}, definition))
	require.Error(t, provider.SaveApiToken(ctx, alice, &ApiTokenDefinition{}))
	store.mu.Lock()
	defer store.mu.Unlock()
	for _, user := range store.connectUsers {
		require.Equal(t, "sys", user)
	}
}
