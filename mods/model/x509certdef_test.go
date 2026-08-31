package model

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const fakeX509CertDriverName = "model_x509cert_fake"

var fakeX509CertDriverOnce sync.Once
var fakeX509CertStores sync.Map

type fakeX509CertStore struct {
	mu           sync.Mutex
	nextID       int64
	certificates map[int64][]driver.Value
	connectUsers []string
}

type fakeX509CertDriver struct{}

func (d *fakeX509CertDriver) Open(string) (driver.Conn, error) {
	return nil, errors.New("use OpenConnector")
}
func (d *fakeX509CertDriver) OpenConnector(name string) (driver.Connector, error) {
	store, ok := fakeX509CertStores.Load(name)
	if !ok {
		return nil, errors.New("x509 cert store not found")
	}
	return &fakeX509CertConnector{store: store.(*fakeX509CertStore)}, nil
}

type fakeX509CertConnector struct{ store *fakeX509CertStore }

func (c *fakeX509CertConnector) Connect(context.Context) (driver.Conn, error) {
	return &fakeX509CertConn{store: c.store}, nil
}
func (c *fakeX509CertConnector) Driver() driver.Driver { return &fakeX509CertDriver{} }

type fakeX509CertConn struct{ store *fakeX509CertStore }

func (c *fakeX509CertConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("prepare is not supported")
}
func (c *fakeX509CertConn) Close() error { return nil }
func (c *fakeX509CertConn) Begin() (driver.Tx, error) {
	return nil, errors.New("transactions are not supported")
}

func (c *fakeX509CertConn) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	q := strings.ToUpper(strings.TrimSpace(query))
	c.store.mu.Lock()
	defer c.store.mu.Unlock()
	switch {
	case strings.HasPrefix(q, "CREATE TABLE"):
		return fakeX509CertResult{}, nil
	case strings.HasPrefix(q, "INSERT INTO _NEO_X509_CERT"):
		id := c.store.nextID
		c.store.nextID++
		c.store.certificates[id] = append([]driver.Value{id}, x509CertNamedValues(args)...)
		return fakeX509CertResult{lastInsertID: id, rowsAffected: 1}, nil
	case strings.HasPrefix(q, "UPDATE _NEO_X509_CERT SET LAST_USED_AT"):
		id := x509CertIntValue(args[1].Value)
		row, ok := c.store.certificates[id]
		if !ok {
			return fakeX509CertResult{}, nil
		}
		row[9] = args[0].Value
		c.store.certificates[id] = row
		return fakeX509CertResult{rowsAffected: 1}, nil
	case strings.HasPrefix(q, "DELETE FROM _NEO_X509_CERT"):
		id := x509CertIntValue(args[0].Value)
		row, ok := c.store.certificates[id]
		if !ok || x509CertStringValue(row[2]) != x509CertStringValue(args[1].Value) {
			return fakeX509CertResult{}, nil
		}
		delete(c.store.certificates, id)
		return fakeX509CertResult{rowsAffected: 1}, nil
	}
	return nil, errors.New("unexpected exec: " + query)
}

func (c *fakeX509CertConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	q := strings.ToUpper(strings.TrimSpace(query))
	c.store.mu.Lock()
	defer c.store.mu.Unlock()
	switch {
	case strings.Contains(q, "SELECT CERT_HASH FROM _NEO_X509_CERT WHERE ID"):
		var rows [][]driver.Value
		if row, ok := c.store.certificates[x509CertIntValue(args[0].Value)]; ok && x509CertStringValue(row[2]) == x509CertStringValue(args[1].Value) {
			rows = append(rows, []driver.Value{row[4]})
		}
		return &fakeX509CertRows{values: rows, width: 1}, nil
	case strings.Contains(q, "FROM _NEO_X509_CERT WHERE CERT_HASH"):
		var rows [][]driver.Value
		for _, row := range c.store.certificates {
			if x509CertStringValue(row[4]) == x509CertStringValue(args[0].Value) {
				rows = append(rows, row)
				break
			}
		}
		return &fakeX509CertRows{values: rows, width: 11}, nil
	case strings.Contains(q, "FROM _NEO_X509_CERT WHERE USER_NAME"):
		var rows [][]driver.Value
		for _, row := range c.store.certificates {
			if x509CertStringValue(row[2]) == x509CertStringValue(args[0].Value) {
				rows = append(rows, row)
			}
		}
		return &fakeX509CertRows{values: rows, width: 11}, nil
	default:
		return nil, errors.New("unexpected query: " + query)
	}
}

type fakeX509CertResult struct{ lastInsertID, rowsAffected int64 }

func (r fakeX509CertResult) LastInsertId() (int64, error) { return r.lastInsertID, nil }
func (r fakeX509CertResult) RowsAffected() (int64, error) { return r.rowsAffected, nil }

// fakeX509CertRows' column width varies: 1 for the CERT_HASH-only lookup, 11 for full-row SELECTs.
type fakeX509CertRows struct {
	values [][]driver.Value
	index  int
	width  int
}

func (r *fakeX509CertRows) Columns() []string { return make([]string, r.width) }
func (r *fakeX509CertRows) Close() error      { return nil }
func (r *fakeX509CertRows) Next(dest []driver.Value) error {
	if r.index >= len(r.values) {
		return io.EOF
	}
	copy(dest, r.values[r.index])
	r.index++
	return nil
}

func x509CertNamedValues(values []driver.NamedValue) []driver.Value {
	result := make([]driver.Value, len(values))
	for index := range values {
		result[index] = values[index].Value
	}
	return result
}
func x509CertStringValue(value any) string {
	if value == nil {
		return ""
	}
	return value.(string)
}
func x509CertIntValue(value any) int64 {
	switch v := value.(type) {
	case int64:
		return v
	case int:
		return int64(v)
	}
	return 0
}

func newFakeX509CertProvider(t *testing.T) (*Provider, *fakeX509CertStore, func()) {
	t.Helper()
	fakeX509CertDriverOnce.Do(func() { sql.Register(fakeX509CertDriverName, &fakeX509CertDriver{}) })
	store := &fakeX509CertStore{nextID: 1, certificates: map[int64][]driver.Value{}}
	dsn := t.Name()
	fakeX509CertStores.Store(dsn, store)
	db, err := sql.Open(fakeX509CertDriverName, dsn)
	require.NoError(t, err)
	provider := NewProvider()
	provider.connect = func(ctx context.Context, user string) (*sql.Conn, error) {
		store.mu.Lock()
		store.connectUsers = append(store.connectUsers, user)
		store.mu.Unlock()
		return db.Conn(ctx)
	}
	return provider, store, func() { require.NoError(t, db.Close()); fakeX509CertStores.Delete(dsn) }
}

func TestX509CertDefinitionTablePersistence(t *testing.T) {
	provider, store, cleanup := newFakeX509CertProvider(t)
	defer cleanup()
	ctx, alice, bob := context.Background(), UserScope{User: "alice"}, UserScope{User: "bob"}
	now := time.Now().Round(0)
	definition := &X509CertDefinition{Name: "device-1", CertPEM: "pem", CertHash: "hash", KeyType: "ecdsa", NotBefore: now, NotAfter: now.Add(time.Hour), CreatedAt: now, Comment: "test"}
	require.NoError(t, provider.SaveX509Cert(ctx, alice, definition))
	require.Equal(t, int64(1), definition.Id)
	require.Equal(t, "ALICE", definition.UserName)
	// duplicate name is allowed; only CERT_HASH must be unique per issued certificate
	duplicate := &X509CertDefinition{Name: "device-1", CertPEM: "pem2", CertHash: "hash2", KeyType: "ecdsa", NotBefore: now, NotAfter: now.Add(time.Hour)}
	require.NoError(t, provider.SaveX509Cert(ctx, alice, duplicate))
	require.NotEqual(t, definition.Id, duplicate.Id)

	loaded, err := provider.GetX509CertByHash(ctx, definition.CertHash)
	require.NoError(t, err)
	require.Equal(t, definition.CertPEM, loaded.CertPEM)
	require.Equal(t, definition.Comment, loaded.Comment)
	require.True(t, loaded.LastUsedAt.IsZero())
	_, err = provider.GetX509CertByHash(ctx, "missing-hash")
	require.ErrorIs(t, err, sql.ErrNoRows)

	list, err := provider.GetAllX509Certs(ctx, alice)
	require.NoError(t, err)
	require.Len(t, list, 2)
	list, err = provider.GetAllX509Certs(ctx, bob)
	require.NoError(t, err)
	require.Empty(t, list)

	require.NoError(t, provider.TouchX509Cert(ctx, definition.Id, now))
	loaded, err = provider.GetX509CertByHash(ctx, definition.CertHash)
	require.NoError(t, err)
	require.Equal(t, now, loaded.LastUsedAt)

	_, err = provider.RemoveX509Cert(ctx, bob, definition.Id)
	require.ErrorIs(t, err, sql.ErrNoRows)
	removedHash, err := provider.RemoveX509Cert(ctx, alice, definition.Id)
	require.NoError(t, err)
	require.Equal(t, definition.CertHash, removedHash)
	_, err = provider.GetX509CertByHash(ctx, definition.CertHash)
	require.ErrorIs(t, err, sql.ErrNoRows)

	require.Error(t, provider.SaveX509Cert(ctx, alice, nil))
	require.Error(t, provider.SaveX509Cert(ctx, UserScope{}, &X509CertDefinition{Name: "x", CertPEM: "pem", CertHash: "hash"}))
	require.Error(t, provider.SaveX509Cert(ctx, alice, &X509CertDefinition{}))
	store.mu.Lock()
	defer store.mu.Unlock()
	for _, user := range store.connectUsers {
		require.Equal(t, "sys", user)
	}
}
