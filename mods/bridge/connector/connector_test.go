package connector

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

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

func TestResultAndScanTypeHelpers(t *testing.T) {
	base := &SqlBridgeBase{}
	require.IsType(t, new(bool), base.NewScanType("sql.NullBool", ""))
	require.IsType(t, new(uint8), base.NewScanType("sql.NullByte", ""))
	require.IsType(t, new(float64), base.NewScanType("sql.NullFloat64", ""))
	require.IsType(t, new(int16), base.NewScanType("sql.NullInt16", ""))
	require.IsType(t, new(int32), base.NewScanType("sql.NullInt32", ""))
	require.IsType(t, new(int64), base.NewScanType("sql.NullInt64", ""))
	require.IsType(t, new(string), base.NewScanType("sql.NullString", ""))
	require.IsType(t, new(sql.NullTime), base.NewScanType("sql.NullTime", ""))
	require.IsType(t, new([]byte), base.NewScanType("sql.RawBytes", ""))
	require.IsType(t, new([]byte), base.NewScanType("[]uint8", ""))
	require.IsType(t, new(bool), base.NewScanType("bool", ""))
	require.IsType(t, new(int32), base.NewScanType("int32", ""))
	require.IsType(t, new(int64), base.NewScanType("int64", ""))
	require.IsType(t, new(string), base.NewScanType("string", ""))
	require.IsType(t, new(sql.NullTime), base.NewScanType("time.Time", ""))
	require.Nil(t, base.NewScanType("unknown", ""))

	nullBool := &sql.NullBool{Bool: true, Valid: true}
	nullByte := &sql.NullByte{Byte: 2, Valid: true}
	nullFloat := &sql.NullFloat64{Float64: 1.25, Valid: true}
	nullInt16 := &sql.NullInt16{Int16: 16, Valid: true}
	nullInt32 := &sql.NullInt32{Int32: 32, Valid: true}
	nullInt64 := &sql.NullInt64{Int64: 64, Valid: true}
	nullString := &sql.NullString{String: "text", Valid: true}
	nullTime := &sql.NullTime{Time: time.Unix(0, 10), Valid: true}
	raw := sql.RawBytes("bytes")

	normalized := base.NormalizeType([]any{
		raw,
		nullBool,
		nullByte,
		nullFloat,
		nullInt16,
		nullInt32,
		nullInt64,
		nullString,
		nullTime,
		&sql.NullString{},
	})
	require.Equal(t, []byte("bytes"), normalized[0])
	require.Equal(t, true, normalized[1])
	require.EqualValues(t, 2, normalized[2])
	require.Equal(t, 1.25, normalized[3])
	require.EqualValues(t, 16, normalized[4])
	require.EqualValues(t, 32, normalized[5])
	require.EqualValues(t, 64, normalized[6])
	require.Equal(t, "text", normalized[7])
	require.Equal(t, nullTime.Time, normalized[8])
	require.Nil(t, normalized[9])
}
