package connector

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	client "github.com/machbase/neo-client/v2"
	"github.com/machbase/neo-server/v8/mods/bridge/connector/sqlite"
	"github.com/stretchr/testify/require"
)

func TestBridgedDatabaseConnectAndAuth(t *testing.T) {
	db, err := sqlite.Connect(sqliteDataSource(t))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	bridged := &BridgedDatabase{db: db}

	conn, err := bridged.Connect(context.Background())
	require.NoError(t, err)
	require.NoError(t, conn.Close())

	ok, reason, err := bridged.UserAuth(context.Background(), "sys", "manager")
	require.NoError(t, err)
	require.True(t, ok)
	require.Empty(t, reason)
}

func TestConnDatabaseAndUnsetHelpers(t *testing.T) {
	resetDatabasesForTest(t)

	db, err := sqlite.Connect(sqliteDataSource(t))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	SetDatabase("cached-sqlite", db, "sqlite", "unused")

	conn, err := Conn(context.Background(), "cached-sqlite")
	require.NoError(t, err)
	require.NoError(t, conn.Close())

	fetched, err := Database("cached-sqlite")
	require.NoError(t, err)
	require.Same(t, db, fetched)

	UnsetDatabase("cached-sqlite")
	_, err = Database("cached-sqlite")
	require.EqualError(t, err, "unknown database type: cached-sqlite")

	_, err = Database("sqlite," + sqliteDataSource(t))
	require.NoError(t, err)

	_, err = Database("postgres,host=127.0.0.1 port=5432 dbname=test user=test password=test sslmode=disable")
	require.NoError(t, err)

	_, err = Database("mysql,user:pass@tcp(127.0.0.1:3306)/db")
	require.NoError(t, err)

	_, err = Database("mssql,server=127.0.0.1:1433 user=sa password=pw database=master encrypt=disable")
	require.NoError(t, err)
}

func TestNewWithDataSourceMachbaseParsing(t *testing.T) {
	resetDatabasesForTest(t)

	db1, err := NewWithDataSource("machbase", "host=127.0.0.1;port=5656;user=sys;password=manager")
	require.NoError(t, err)
	require.NotNil(t, db1)
	t.Cleanup(func() { require.NoError(t, db1.Close()) })

	db2, err := NewWithDataSource("machbase", "server=tcp://sys:manager@127.0.0.1:5656")
	require.NoError(t, err)
	require.NotNil(t, db2)
	t.Cleanup(func() { require.NoError(t, db2.Close()) })
}

func resetDatabasesForTest(t *testing.T) {
	t.Helper()
	databasesLock.Lock()
	prev := databases
	databases = map[string]*sql.DB{}
	databasesLock.Unlock()
	t.Cleanup(func() {
		for _, db := range databases {
			if db != nil {
				require.NoError(t, db.Close())
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
	t.Cleanup(func() { require.NoError(t, first.Close()) })

	second, err := New(name)
	require.NoError(t, err)
	require.Same(t, first, second)

	conn, err := first.Conn(ctx)
	require.NoError(t, err)
	require.NoError(t, conn.Close())

	_, err = New("unknown,dsn")
	require.EqualError(t, err, "unknown database type: unknown,dsn")
}

func TestNewWithDataSourceAndSetDatabase(t *testing.T) {
	resetDatabasesForTest(t)

	dataSource := sqliteDataSource(t)
	db, err := NewWithDataSource("sqlite", dataSource)
	require.NoError(t, err)
	require.NotNil(t, db)
	t.Cleanup(func() { require.NoError(t, db.Close()) })

	_, err = NewWithDataSource("postgresql", "postgres://user:pass@127.0.0.1/db")
	require.NoError(t, err)

	_, err = NewWithDataSource("mysql", "user:pass@tcp(127.0.0.1:3306)/db")
	require.NoError(t, err)

	_, err = NewWithDataSource("mssql", "sqlserver://user:pass@127.0.0.1:1433?database=db")
	require.NoError(t, err)

	_, err = NewWithDataSource("unknown", "dsn")
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
	nullBool := &sql.NullBool{Bool: true, Valid: true}
	nullByte := &sql.NullByte{Byte: 2, Valid: true}
	nullFloat := &sql.NullFloat64{Float64: 1.25, Valid: true}
	nullInt16 := &sql.NullInt16{Int16: 16, Valid: true}
	nullInt32 := &sql.NullInt32{Int32: 32, Valid: true}
	nullInt64 := &sql.NullInt64{Int64: 64, Valid: true}
	nullString := &sql.NullString{String: "text", Valid: true}
	nullTime := &sql.NullTime{Time: time.Unix(0, 10), Valid: true}
	raw := sql.RawBytes("bytes")

	normalized := client.NormalizeTypes([]any{
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
	}, time.UTC)
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

func TestSqliteBridgeLifecycleAndBasics(t *testing.T) {
	dsn := "file:" + filepath.Join(t.TempDir(), "sqlite_bridge.db") + "?cache=shared"
	br := NewSqliteBridge("sqlite_test", dsn)

	require.Equal(t, "bridge 'sqlite_test' (sqlite3)", br.String())
	require.Equal(t, "sqlite_test", br.Name())
	require.Equal(t, "sqlite", br.Type())
	require.True(t, br.SupportLastInsertId())
	require.Equal(t, "?", br.ParameterMarker(0))
	require.Nil(t, br.DB())
	require.IsType(t, new([]byte), br.NewScanType("sql.RawBytes", "BLOB"))
	require.IsType(t, new(sql.NullBool), br.NewScanType("sql.NullBool", ""))
	require.IsType(t, new(sql.NullByte), br.NewScanType("sql.NullByte", ""))
	require.IsType(t, new(sql.NullFloat64), br.NewScanType("sql.NullFloat64", ""))
	require.IsType(t, new(sql.NullInt16), br.NewScanType("sql.NullInt16", ""))
	require.IsType(t, new(sql.NullInt32), br.NewScanType("sql.NullInt32", ""))
	require.IsType(t, new(sql.NullInt64), br.NewScanType("sql.NullInt64", ""))
	require.IsType(t, new(sql.NullString), br.NewScanType("sql.NullString", ""))
	require.IsType(t, new(sql.NullTime), br.NewScanType("sql.NullTime", "DATETIME"))
	require.IsType(t, new(string), br.NewScanType("*interface {}", ""))

	_, err := br.Connect(context.Background())
	require.EqualError(t, err, "bridge 'sqlite_test' is not initialized")

	require.NoError(t, br.BeforeRegister())
	require.NotNil(t, br.DB())

	conn, err := br.Connect(context.Background())
	require.NoError(t, err)
	require.NoError(t, conn.Close())

	require.NoError(t, br.AfterUnregister())
}

func TestPostgresBridgeBasics(t *testing.T) {
	br := NewPostgresBridge("pg_test", "host=127.0.0.1 port=5432 dbname=test user=test password=test sslmode=disable")

	require.Equal(t, "bridge 'pg_test' (postgres)", br.String())
	require.Equal(t, "pg_test", br.Name())
	require.Equal(t, "postgres", br.Type())
	require.False(t, br.SupportLastInsertId())
	require.Equal(t, "$3", br.ParameterMarker(2))
	require.Nil(t, br.DB())

	require.IsType(t, new(float32), br.NewScanType("interface {}", "FLOAT4"))
	require.IsType(t, new(sql.NullString), br.NewScanType("interface {}", "UUID"))
	require.IsType(t, new(sql.NullBool), br.NewScanType("bool", ""))
	require.IsType(t, new(sql.NullInt32), br.NewScanType("int32", ""))
	require.IsType(t, new(sql.NullInt64), br.NewScanType("int64", ""))
	require.IsType(t, new(sql.NullString), br.NewScanType("string", ""))
	require.IsType(t, new(sql.NullTime), br.NewScanType("time.Time", ""))

	_, err := br.Connect(context.Background())
	require.EqualError(t, err, "bridge 'pg_test' is not initialized")

	require.NoError(t, br.BeforeRegister())
	require.NotNil(t, br.DB())
	require.NoError(t, br.AfterUnregister())
}

func TestMySQLBridgeBasics(t *testing.T) {
	br := NewMySQLBridge("my_test", "user:pass@tcp(127.0.0.1:3306)/db")

	require.Equal(t, "bridge 'my_test' (mysql)", br.String())
	require.Equal(t, "my_test", br.Name())
	require.Equal(t, "mysql", br.Type())
	require.True(t, br.SupportLastInsertId())
	require.Equal(t, "?", br.ParameterMarker(0))
	require.Nil(t, br.DB())

	require.IsType(t, new(sql.NullString), br.NewScanType("sql.RawBytes", "VARCHAR"))
	require.IsType(t, new([]byte), br.NewScanType("sql.RawBytes", "BLOB"))
	require.IsType(t, new(sql.NullBool), br.NewScanType("sql.NullBool", ""))
	require.IsType(t, new(sql.NullByte), br.NewScanType("sql.NullByte", ""))
	require.IsType(t, new(sql.NullFloat64), br.NewScanType("sql.NullFloat64", ""))
	require.IsType(t, new(sql.NullInt16), br.NewScanType("sql.NullInt16", ""))
	require.IsType(t, new(sql.NullInt32), br.NewScanType("sql.NullInt32", ""))
	require.IsType(t, new(sql.NullInt64), br.NewScanType("sql.NullInt64", ""))
	require.IsType(t, new(sql.NullString), br.NewScanType("sql.NullString", ""))
	require.IsType(t, new(sql.NullString), br.NewScanType("sql.NullTime", "DATE"))
	require.IsType(t, new(sql.NullTime), br.NewScanType("sql.NullTime", "DATETIME"))

	_, err := br.Connect(context.Background())
	require.EqualError(t, err, "bridge 'my_test' is not initialized")

	err = br.BeforeRegister()
	require.Error(t, err)
	require.NoError(t, br.AfterUnregister())
}

func TestMSSQLBridgeBasicsAndOptionParsing(t *testing.T) {
	path := "server=127.0.0.1:1433 user=sa password=pw database=master encrypt=disable"
	br := NewMSSQLBridge("ms_test", path)

	require.Equal(t, "bridge 'ms_test' (mssql)", br.String())
	require.Equal(t, "ms_test", br.Name())
	require.Equal(t, "mssql", br.Type())
	require.False(t, br.SupportLastInsertId())
	require.Equal(t, "@p3", br.ParameterMarker(2))
	require.Nil(t, br.DB())

	require.IsType(t, new(sql.NullInt64), br.NewScanType("", "INT"))
	require.IsType(t, new(sql.NullFloat64), br.NewScanType("", "DECIMAL"))
	require.IsType(t, new(sql.NullBool), br.NewScanType("", "BIT"))
	require.IsType(t, new(sql.NullString), br.NewScanType("", "VARCHAR"))
	require.IsType(t, new(sql.NullTime), br.NewScanType("", "DATETIME"))
	require.IsType(t, new(string), br.NewScanType("string", "UNKNOWN"))

	_, err := br.Connect(context.Background())
	require.EqualError(t, err, "bridge 'ms_test' is not initialized")

	require.NoError(t, br.BeforeRegister())
	require.NotNil(t, br.DB())
	require.NotNil(t, br.ParsedURL())
	require.Equal(t, "sqlserver", br.ParsedURL().Scheme)
	require.Equal(t, "127.0.0.1:1433", br.ParsedURL().Host)

	q := br.ParsedURL().Query()
	require.Equal(t, "sa", q.Get("user id"))
	require.Equal(t, "pw", q.Get("password"))
	require.Equal(t, "master", q.Get("database"))
	require.Equal(t, "disable", q.Get("encrypt"))
	require.Equal(t, "3", q.Get("dial timeout"))
	require.Equal(t, "5", q.Get("connection timeout"))
	require.Equal(t, "neo-bridge", q.Get("app name"))

	require.NoError(t, br.AfterUnregister())
}
