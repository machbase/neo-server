package spi

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"io"
	"net"
	"reflect"
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

type testColumnMeta struct {
	name     string
	dbType   string
	scanType reflect.Type
	nullable *bool
}

type testColumnDriver struct{}

var (
	testColumnDriverOnce sync.Once
	testColumnDriverMu   sync.Mutex
	testColumnDriverMeta = map[string][]testColumnMeta{}
)

func registerTestColumnDriver(t *testing.T) {
	t.Helper()
	testColumnDriverOnce.Do(func() {
		sql.Register("spi_test_column_driver", &testColumnDriver{})
	})
}

func (d *testColumnDriver) Open(name string) (driver.Conn, error) {
	return &testColumnConn{dsn: name}, nil
}

type testColumnConn struct {
	dsn string
}

func (c *testColumnConn) Prepare(string) (driver.Stmt, error) {
	return nil, api.ErrNotImplemented("Prepare")
}

func (c *testColumnConn) Close() error { return nil }

func (c *testColumnConn) Begin() (driver.Tx, error) {
	return nil, api.ErrNotImplemented("Begin")
}

func (c *testColumnConn) QueryContext(_ context.Context, _ string, _ []driver.NamedValue) (driver.Rows, error) {
	testColumnDriverMu.Lock()
	metas, ok := testColumnDriverMeta[c.dsn]
	testColumnDriverMu.Unlock()
	if !ok {
		return nil, fmt.Errorf("column metadata not found for dsn %q", c.dsn)
	}
	return &testColumnRows{metas: metas}, nil
}

type testColumnRows struct {
	metas []testColumnMeta
}

func (r *testColumnRows) Columns() []string {
	ret := make([]string, len(r.metas))
	for i, m := range r.metas {
		ret[i] = m.name
	}
	return ret
}

func (r *testColumnRows) Close() error { return nil }

func (r *testColumnRows) Next(_ []driver.Value) error { return io.EOF }

func (r *testColumnRows) ColumnTypeDatabaseTypeName(index int) string {
	return r.metas[index].dbType
}

func (r *testColumnRows) ColumnTypeScanType(index int) reflect.Type {
	return r.metas[index].scanType
}

func (r *testColumnRows) ColumnTypeNullable(index int) (nullable, ok bool) {
	if r.metas[index].nullable == nil {
		return false, false
	}
	return *r.metas[index].nullable, true
}

func makeColumnTypesForTest(t *testing.T, metas []testColumnMeta) []*sql.ColumnType {
	t.Helper()
	registerTestColumnDriver(t)

	dsn := fmt.Sprintf("%s/%s", t.Name(), time.Now().Format(time.RFC3339Nano))
	testColumnDriverMu.Lock()
	testColumnDriverMeta[dsn] = metas
	testColumnDriverMu.Unlock()
	t.Cleanup(func() {
		testColumnDriverMu.Lock()
		delete(testColumnDriverMeta, dsn)
		testColumnDriverMu.Unlock()
	})

	db, err := sql.Open("spi_test_column_driver", dsn)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, db.Close())
	})

	rows, err := db.QueryContext(context.Background(), "SELECT 1")
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, rows.Close())
	})

	colTypes, err := rows.ColumnTypes()
	require.NoError(t, err)
	return colTypes
}

func boolPtr(v bool) *bool {
	return &v
}

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

func TestSQLStatementTypeString(t *testing.T) {
	tcs := []struct {
		name string
		in   SQLStatementType
		want string
	}{
		{name: "select", in: SQLStatementTypeSelect, want: "SELECT"},
		{name: "insert", in: SQLStatementTypeInsert, want: "INSERT"},
		{name: "update", in: SQLStatementTypeUpdate, want: "UPDATE"},
		{name: "delete", in: SQLStatementTypeDelete, want: "DELETE"},
		{name: "create", in: SQLStatementTypeCreate, want: "CREATE"},
		{name: "drop", in: SQLStatementTypeDrop, want: "DROP"},
		{name: "alter", in: SQLStatementTypeAlter, want: "ALTER"},
		{name: "describe", in: SQLStatementTypeDescribe, want: "DESCRIBE"},
		{name: "cte", in: SQLStatementTypeCommonTableExpression, want: "CTE"},
		{name: "explain", in: SQLStatementTypeExplain, want: "EXPLAIN"},
		{name: "show", in: SQLStatementTypeShow, want: "SHOW"},
		{name: "other", in: SQLStatementTypeOther, want: "OTHER"},
		{name: "unknown", in: SQLStatementType(-1), want: "OTHER"},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, tc.in.String())
		})
	}
}

func TestColumnTypesToDataTypes(t *testing.T) {
	colTypes := makeColumnTypesForTest(t, []testColumnMeta{
		{name: "c1", dbType: "SHORT", scanType: reflect.TypeOf(int16(0))},
		{name: "c2", dbType: "UINT32", scanType: reflect.TypeOf(uint32(0))},
		{name: "c3", dbType: "VARCHAR", scanType: reflect.TypeOf("")},
		{name: "c4", dbType: "DATETIME", scanType: reflect.TypeOf(time.Time{})},
		{name: "c5", dbType: "BINARY", scanType: reflect.TypeOf([]byte{})},
		{name: "c6", dbType: "JSON", scanType: reflect.TypeOf(api.JSONString(""))},
		{name: "c7", dbType: "IPV4", scanType: reflect.TypeOf(net.IP{})},
		{name: "c8", dbType: "IPV6", scanType: reflect.TypeOf(net.IP{})},
		{name: "c9", dbType: "CUSTOM", scanType: reflect.TypeOf("")},
	})

	got := ColumnTypesToDataTypes(colTypes)
	require.Equal(t, []api.DataType{
		api.DataTypeInt16,
		api.DataTypeUInt32,
		api.DataTypeString,
		api.DataTypeDatetime,
		api.DataTypeBinary,
		api.DataTypeJSON,
		api.DataTypeIPv4,
		api.DataTypeIPv6,
		api.DataType("CUSTOM"),
	}, got)
}

func TestMakeBuffer(t *testing.T) {
	colTypes := makeColumnTypesForTest(t, []testColumnMeta{
		{name: "n_int16", dbType: "INT16", scanType: reflect.TypeOf(int16(0)), nullable: boolPtr(true)},
		{name: "int16", dbType: "INT16", scanType: reflect.TypeOf(int16(0)), nullable: boolPtr(false)},
		{name: "n_int32", dbType: "INT32", scanType: reflect.TypeOf(int32(0)), nullable: boolPtr(true)},
		{name: "n_str", dbType: "VARCHAR", scanType: reflect.TypeOf(""), nullable: boolPtr(true)},
		{name: "str", dbType: "VARCHAR", scanType: reflect.TypeOf(""), nullable: boolPtr(false)},
		{name: "n_json", dbType: "JSON", scanType: reflect.TypeOf(api.JSONString("")), nullable: boolPtr(true)},
		{name: "json", dbType: "JSON", scanType: reflect.TypeOf(api.JSONString("")), nullable: boolPtr(false)},
		{name: "bytes", dbType: "BINARY", scanType: reflect.TypeOf([]uint8{})},
		{name: "ip", dbType: "IPV4", scanType: reflect.TypeOf(net.IP{})},
		{name: "null_f64", dbType: "DOUBLE", scanType: reflect.TypeOf(sql.NullFloat64{})},
		{name: "def_int", dbType: "INT", scanType: reflect.TypeOf(struct{}{})},
		{name: "def_bool", dbType: "BOOLEAN", scanType: reflect.TypeOf(struct{}{})},
		{name: "def_time", dbType: "DATE", scanType: reflect.TypeOf(struct{}{})},
		{name: "def_any", dbType: "MYSTERY", scanType: reflect.TypeOf(struct{}{})},
	})

	got := MakeBuffer(colTypes)
	require.Len(t, got, len(colTypes))

	require.IsType(t, &sql.NullInt16{}, got[0])
	//require.IsType(t, new(int16), got[1])
	require.IsType(t, &sql.NullInt16{}, got[1])
	require.IsType(t, &sql.NullInt32{}, got[2])
	require.IsType(t, &sql.NullString{}, got[3])
	//require.IsType(t, new(string), got[4])
	require.IsType(t, &sql.NullString{}, got[4])
	require.IsType(t, &sql.Null[api.JSONString]{}, got[5])
	//require.IsType(t, new(api.JSONString), got[6])
	require.IsType(t, &sql.Null[[]byte]{}, got[7])
	require.IsType(t, &sql.Null[net.IP]{}, got[8])
	require.IsType(t, &sql.NullFloat64{}, got[9])
	require.IsType(t, &sql.NullInt64{}, got[10])
	require.IsType(t, &sql.NullBool{}, got[11])
	require.IsType(t, &sql.NullTime{}, got[12])
	require.IsType(t, new(interface{}), got[13])
}

func TestMakeUserMessage(t *testing.T) {
	tcs := []struct {
		name      string
		smtType   SQLStatementType
		rowsCount int64
		want      string
	}{
		{name: "select zero", smtType: SQLStatementTypeSelect, rowsCount: 0, want: "no rows selected."},
		{name: "select one", smtType: SQLStatementTypeSelect, rowsCount: 1, want: "a row selected."},
		{name: "select many", smtType: SQLStatementTypeSelect, rowsCount: 2, want: "2 rows selected."},
		{name: "insert", smtType: SQLStatementTypeInsert, rowsCount: 3, want: "3 rows inserted."},
		{name: "update", smtType: SQLStatementTypeUpdate, rowsCount: 4, want: "4 rows updated."},
		{name: "delete", smtType: SQLStatementTypeDelete, rowsCount: 5, want: "5 rows deleted."},
		{name: "create", smtType: SQLStatementTypeCreate, rowsCount: 0, want: "Created successfully."},
		{name: "drop", smtType: SQLStatementTypeDrop, rowsCount: 0, want: "Dropped successfully."},
		{name: "alter", smtType: SQLStatementTypeAlter, rowsCount: 0, want: "Altered successfully."},
		{name: "other", smtType: SQLStatementTypeOther, rowsCount: 0, want: "executed."},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, MakeUserMessage(tc.smtType, tc.rowsCount))
		})
	}
}
