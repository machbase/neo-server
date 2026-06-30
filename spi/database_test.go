package spi

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/machbase/neo-client/api"
	"github.com/stretchr/testify/require"
)

type poolStubDatabase struct {
	connectCount int
}

func (s *poolStubDatabase) Connect(ctx context.Context, options ...api.ConnectOption) (api.Conn, error) {
	s.connectCount++
	return &poolStubConn{}, nil
}

func (s *poolStubDatabase) UserAuth(ctx context.Context, user string, password string) (bool, string, error) {
	return true, "", nil
}

func (s *poolStubDatabase) Ping(ctx context.Context) (time.Duration, error) {
	return 0, nil
}

type poolStubConn struct{}

func (c *poolStubConn) Close() error { return nil }

func (c *poolStubConn) Exec(ctx context.Context, sqlText string, params ...any) api.Result {
	return &InsertResult{rowsAffected: 1, message: "a row inserted."}
}

func (c *poolStubConn) Query(ctx context.Context, sqlText string, params ...any) (api.Rows, error) {
	// DefaultPool() validates connector availability via database/sql Ping() -> SELECT 1.
	return &poolStubRows{}, nil
}

func (c *poolStubConn) QueryRow(ctx context.Context, sqlText string, params ...any) api.Row {
	return &WrappedSqlRow{err: api.ErrNotImplemented("QueryRow")}
}

func (c *poolStubConn) Prepare(ctx context.Context, query string) (api.Stmt, error) {
	return nil, api.ErrNotImplemented("Prepare")
}

func (c *poolStubConn) Appender(ctx context.Context, tableName string, opts ...api.AppenderOption) (api.Appender, error) {
	return nil, api.ErrNotImplemented("Appender")
}

func (c *poolStubConn) Explain(ctx context.Context, sqlText string, full bool) (string, error) {
	return "", api.ErrNotImplemented("Explain")
}

type poolStubRows struct{}

func (r *poolStubRows) Next() bool                    { return false }
func (r *poolStubRows) Scan(cols ...any) error        { return nil }
func (r *poolStubRows) Close() error                  { return nil }
func (r *poolStubRows) Err() error                    { return nil }
func (r *poolStubRows) IsFetchable() bool             { return true }
func (r *poolStubRows) RowsAffected() int64           { return 0 }
func (r *poolStubRows) Message() string               { return "success" }
func (r *poolStubRows) Columns() (api.Columns, error) { return api.Columns{}, nil }

func resetDefaultPoolForTest(t *testing.T) {
	t.Helper()
	defaultPoolOnce = sync.Once{}
	defaultPoolDB = nil
	defaultPoolErr = nil
}

func resetDefaultPoolConfigForTest(t *testing.T) {
	t.Helper()
	maxOpenConn = 20
	maxIdleConn = 2
	connMaxLifetime = 10 * time.Minute
	connMaxIdleTime = 1 * time.Minute
}

func setDefaultForTest(t *testing.T, db api.Database, key crypto.PrivateKey) {
	t.Helper()
	defaultDatabase = db
	defaultDatabaseKey = key
}

func newTestAuthKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	return key
}

func TestIssueTokenAndVerifyToken(t *testing.T) {
	token := IssueToken()
	require.NotEmpty(t, token)
	require.Contains(t, token, ":")

	valid := VerifyToken(token, 10*time.Second)
	require.True(t, valid)
}

func TestVerifyTokenMalformedToken(t *testing.T) {
	require.False(t, VerifyToken("not-a-token", 10*time.Second))
	require.False(t, VerifyToken("1234567890:", 10*time.Second))
}

func TestVerifyTokenTamperedSignature(t *testing.T) {
	token := IssueToken()
	require.NotEmpty(t, token)

	parts := strings.SplitN(token, ":", 2)
	require.Len(t, parts, 2)

	tampered := parts[0] + ":" + parts[1] + "a"
	require.False(t, VerifyToken(tampered, 10*time.Second))
}

func TestVerifyTokenExpired(t *testing.T) {
	token := IssueToken()
	require.NotEmpty(t, token)

	time.Sleep(5 * time.Millisecond)
	require.False(t, VerifyToken(token, 1*time.Millisecond))
}

func TestDefaultPoolDatabaseNotConfigured(t *testing.T) {
	oldDB := defaultDatabase
	oldKey := defaultDatabaseKey
	t.Cleanup(func() {
		defaultDatabase = oldDB
		defaultDatabaseKey = oldKey
		resetDefaultPoolForTest(t)
	})

	setDefaultForTest(t, nil, nil)
	resetDefaultPoolForTest(t)

	pool, err := DefaultPool()
	require.Error(t, err)
	require.Nil(t, pool)
	require.ErrorContains(t, err, "default database is not configured")
}

func TestDefaultPoolConnectFailsWhenKeyMissing(t *testing.T) {
	oldDB := defaultDatabase
	oldKey := defaultDatabaseKey
	t.Cleanup(func() {
		defaultDatabase = oldDB
		defaultDatabaseKey = oldKey
		resetDefaultPoolForTest(t)
	})

	stubDB := &poolStubDatabase{}
	setDefaultForTest(t, stubDB, nil)
	resetDefaultPoolForTest(t)

	pool, err := DefaultPool()
	require.Error(t, err)
	require.Nil(t, pool)
	require.ErrorContains(t, err, "default key is not configured")
	require.Equal(t, 0, stubDB.connectCount)
}

func TestDefaultPoolSuccessAndCachedInstance(t *testing.T) {
	oldDB := defaultDatabase
	oldKey := defaultDatabaseKey
	t.Cleanup(func() {
		defaultDatabase = oldDB
		defaultDatabaseKey = oldKey
		resetDefaultPoolForTest(t)
		resetDefaultPoolConfigForTest(t)
	})

	stubDB := &poolStubDatabase{}
	setDefaultForTest(t, stubDB, newTestAuthKey(t))
	resetDefaultPoolForTest(t)

	pool1, err := DefaultPool()
	require.NoError(t, err)
	require.NotNil(t, pool1)
	t.Cleanup(func() {
		require.NoError(t, pool1.Close())
	})

	conn, err := pool1.Conn(context.Background())
	require.NoError(t, err)
	require.NotNil(t, conn)
	require.NoError(t, conn.Close())
	require.GreaterOrEqual(t, stubDB.connectCount, 1)

	pool2, err := DefaultPool()
	require.NoError(t, err)
	require.Same(t, pool1, pool2)
}

func TestDefaultPoolUsesConfiguredPoolSettings(t *testing.T) {
	oldDB := defaultDatabase
	oldKey := defaultDatabaseKey
	t.Cleanup(func() {
		defaultDatabase = oldDB
		defaultDatabaseKey = oldKey
		resetDefaultPoolForTest(t)
		resetDefaultPoolConfigForTest(t)
	})

	stubDB := &poolStubDatabase{}
	setDefaultForTest(t, stubDB, newTestAuthKey(t))
	resetDefaultPoolForTest(t)

	wantMaxOpen := 31
	wantMaxIdle := 7
	wantConnMaxLifetime := 3 * time.Minute
	wantConnMaxIdleTime := 45 * time.Second
	SetDefaultPoolConfig(wantMaxOpen, wantMaxIdle, wantConnMaxLifetime, wantConnMaxIdleTime)

	pool, err := DefaultPool()
	require.NoError(t, err)
	require.NotNil(t, pool)
	t.Cleanup(func() {
		require.NoError(t, pool.Close())
	})

	stats := pool.Stats()
	require.Equal(t, wantMaxOpen, stats.MaxOpenConnections)
	require.Equal(t, wantMaxOpen, maxOpenConn)
	require.Equal(t, wantMaxIdle, maxIdleConn)
	require.Equal(t, wantConnMaxLifetime, connMaxLifetime)
	require.Equal(t, wantConnMaxIdleTime, connMaxIdleTime)
	require.GreaterOrEqual(t, stubDB.connectCount, 1)
}

func TestDefaultPoolErrorIsCachedByOnce(t *testing.T) {
	oldDB := defaultDatabase
	oldKey := defaultDatabaseKey
	t.Cleanup(func() {
		defaultDatabase = oldDB
		defaultDatabaseKey = oldKey
		resetDefaultPoolForTest(t)
	})

	setDefaultForTest(t, nil, nil)
	resetDefaultPoolForTest(t)

	pool, err := DefaultPool()
	require.Error(t, err)
	require.Nil(t, pool)
	require.ErrorContains(t, err, "default database is not configured")

	setDefaultForTest(t, &poolStubDatabase{}, newTestAuthKey(t))
	pool2, err2 := DefaultPool()
	require.Error(t, err2)
	require.Nil(t, pool2)
	require.ErrorContains(t, err2, "default database is not configured")
}
