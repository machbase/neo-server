package connector

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

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
	require.NotNil(t, br.u)
	require.Equal(t, "sqlserver", br.u.Scheme)
	require.Equal(t, "127.0.0.1:1433", br.u.Host)

	q := br.u.Query()
	require.Equal(t, "sa", q.Get("user id"))
	require.Equal(t, "pw", q.Get("password"))
	require.Equal(t, "master", q.Get("database"))
	require.Equal(t, "disable", q.Get("encrypt"))
	require.Equal(t, "3", q.Get("dial timeout"))
	require.Equal(t, "5", q.Get("connection timeout"))
	require.Equal(t, "neo-bridge", q.Get("app name"))

	require.NoError(t, br.AfterUnregister())
}
