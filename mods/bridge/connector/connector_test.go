package connector

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/machbase/neo-server/v8/mods/bridge/connector/sqlite"
	"github.com/stretchr/testify/require"
)

func closeBridgedDatabase(t *testing.T, db any) {
	t.Helper()
	bridged, ok := db.(*BridgedDatabase)
	if !ok || bridged == nil || bridged.db == nil {
		return
	}
	require.NoError(t, bridged.db.Close())
}

func resetDatabasesForTest(t *testing.T) {
	t.Helper()
	databasesLock.Lock()
	prev := databases
	databases = map[string]*BridgedDatabase{}
	databasesLock.Unlock()
	t.Cleanup(func() {
		for _, db := range databases {
			if db != nil && db.db != nil {
				require.NoError(t, db.db.Close())
			}
		}
		databasesLock.Lock()
		databases = prev
		databasesLock.Unlock()
	})
}

func sqliteDataSource(t *testing.T) string {
	t.Helper()
	return "file:" + filepath.Join(t.TempDir(), "connector.db") + "?cache=shared"
}

func TestNewCachesAndConnectsSqlite(t *testing.T) {
	resetDatabasesForTest(t)

	ctx := context.Background()
	name := "sqlite," + sqliteDataSource(t)

	first, err := New(name)
	require.NoError(t, err)
	require.NotNil(t, first)
	t.Cleanup(func() { closeBridgedDatabase(t, first) })

	second, err := New(name)
	require.NoError(t, err)
	require.Same(t, first, second)

	bridged := first.(*BridgedDatabase)
	conn, err := bridged.Connect(ctx)
	require.NoError(t, err)
	require.NoError(t, conn.Close())

	_, err = bridged.Ping(ctx)
	require.NoError(t, err)

	ok, reason, err := bridged.UserAuth(ctx, "user", "password")
	require.NoError(t, err)
	require.True(t, ok)
	require.Empty(t, reason)

	_, err = New("unknown,dsn")
	require.EqualError(t, err, "unknown database type: unknown,dsn")
}

func TestNewWithDataSourceAndSetDatabase(t *testing.T) {
	resetDatabasesForTest(t)

	dataSource := sqliteDataSource(t)
	db, opts, err := NewWithDataSource("sqlite", dataSource)
	require.NoError(t, err)
	require.NotNil(t, db)
	require.Empty(t, opts)
	t.Cleanup(func() { closeBridgedDatabase(t, db) })

	_, _, err = NewWithDataSource("postgresql", "postgres://user:pass@127.0.0.1/db")
	require.NoError(t, err)

	_, _, err = NewWithDataSource("mysql", "user:pass@tcp(127.0.0.1:3306)/db")
	require.NoError(t, err)

	_, _, err = NewWithDataSource("mssql", "sqlserver://user:pass@127.0.0.1:1433?database=db")
	require.NoError(t, err)

	_, _, err = NewWithDataSource("unknown", "dsn")
	require.EqualError(t, err, "unknown database type: unknown")

	sqlDB, err := sqlite.Connect(dataSource)
	require.NoError(t, err)
	t.Cleanup(func() { sqlDB.Close() })

	SetDatabase("preloaded", sqlDB, "sqlite", dataSource)
	preloaded, err := New("preloaded")
	require.NoError(t, err)
	require.NotNil(t, preloaded)

	require.PanicsWithValue(t, "db is nil", func() {
		SetDatabase("panic", nil, "sqlite", dataSource)
	})
}

func TestBridgedDatabasePingFailure(t *testing.T) {
	db, err := sqlite.Connect(sqliteDataSource(t))
	require.NoError(t, err)
	require.NoError(t, db.Close())

	bridged := &BridgedDatabase{db: db}
	_, err = bridged.Ping(context.Background())
	require.Error(t, err)
}
