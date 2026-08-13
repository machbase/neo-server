package spi

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"net"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	client "github.com/machbase/neo-client/v2"
	"github.com/machbase/neo-client/v2/api"
	"github.com/stretchr/testify/require"
)

type poolStubDatabase struct {
	connectCount int
}

func (s *poolStubDatabase) Connect(ctx context.Context) (*poolStubConn, error) {
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

func (c *poolStubConn) Exec(ctx context.Context, sqlText string, params ...any) *InsertResult {
	return &InsertResult{rowsAffected: 1, message: "a row inserted."}
}

func (c *poolStubConn) Query(ctx context.Context, sqlText string, params ...any) (*poolStubRows, error) {
	// DefaultPool() validates connector availability via database/sql Ping() -> SELECT 1.
	return &poolStubRows{}, nil
}

func (c *poolStubConn) QueryRow(ctx context.Context, sqlText string, params ...any) *poolStubRow {
	return &poolStubRow{err: errors.New("not implemented QueryRow")}
}

func (c *poolStubConn) Prepare(ctx context.Context, query string) (any, error) {
	return nil, errors.New("not implemented Prepare")
}

func (c *poolStubConn) Appender(ctx context.Context, tableName string) (any, error) {
	return nil, errors.New("not implemented Appender")
}

func (c *poolStubConn) Explain(ctx context.Context, sqlText string, full bool) (string, error) {
	return "", errors.New("not implemented Explain")
}

type poolStubRows struct{}

func (r *poolStubRows) Next() bool                       { return false }
func (r *poolStubRows) Scan(cols ...any) error           { return nil }
func (r *poolStubRows) Close() error                     { return nil }
func (r *poolStubRows) Err() error                       { return nil }
func (r *poolStubRows) IsFetchable() bool                { return true }
func (r *poolStubRows) RowsAffected() int64              { return 0 }
func (r *poolStubRows) Message() string                  { return "success" }
func (r *poolStubRows) Columns() (client.Columns, error) { return client.Columns{}, nil }

type poolStubRow struct {
	err          error
	values       []any
	columns      client.Columns
	columnsErr   error
	timeLocation *time.Location
}

func (r *poolStubRow) Err() error {
	return r.err
}

func (r *poolStubRow) RowsAffected() int64 {
	return 0
}

func (r *poolStubRow) Message() string {
	// TODO: implement
	return "success"
}

func (r *poolStubRow) Scan(values ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(values) > len(r.values) {
		return fmt.Errorf("scan column %d is out of range %d", len(values), len(r.values))
	}
	for i := range values {
		if r.values[i] == nil {
			values[i] = nil
			continue
		}
		if err := client.Scan(r.values[i], values[i], r.timeLocation); err != nil {
			return err
		}
	}
	return nil
}

func (r *poolStubRow) Columns() (client.Columns, error) {
	return r.columns, nil
}

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
	return nil, errors.New("not implemented Prepare")
}

func (c *testColumnConn) Close() error { return nil }

func (c *testColumnConn) Begin() (driver.Tx, error) {
	return nil, errors.New("not implemented Begin")
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

func TestSetDefaultHttpEndpointSelectsLoopbackFriendlyURLs(t *testing.T) {
	tests := []struct {
		name     string
		addrs    []string
		expected string
	}{
		{
			name:     "ipv4 wildcard",
			addrs:    []string{"tcp://0.0.0.0:5655"},
			expected: "http://127.0.0.1:5655",
		},
		{
			name:     "ipv6 wildcard",
			addrs:    []string{"tcp://[::]:5655"},
			expected: "http://[::1]:5655",
		},
		{
			name:     "ipv6 loopback",
			addrs:    []string{"tcp://[::1]:5655"},
			expected: "http://[::1]:5655",
		},
		{
			name:     "explicit host",
			addrs:    []string{"tcp://192.168.0.10:5655"},
			expected: "http://192.168.0.10:5655",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetDefaultHttpEndpoint(tt.addrs)
			require.Equal(t, tt.expected, DefaultHttpEndpoint())
		})
	}
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

	require.IsType(t, new(int16), got[1])
	require.IsType(t, &sql.NullInt32{}, got[2])
	require.IsType(t, &sql.NullString{}, got[3])
	require.IsType(t, new(string), got[4])
	require.IsType(t, &sql.Null[api.JSONString]{}, got[5])
	require.IsType(t, new(api.JSONString), got[6])
	require.IsType(t, &[]byte{}, got[7])
	require.IsType(t, &net.IP{}, got[8])
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

type catalogTestConn struct {
	legacy bool
}

type catalogTestConnDriver struct{}

type catalogTestConnDriverConn struct {
	scenario catalogTestConn
}

type catalogTestConnScenario struct {
	legacy bool
}

type catalogTestConnRows struct {
	columns []string
	rows    [][]driver.Value
	index   int
	err     error
}

var (
	catalogTestConnDriverOnce sync.Once
	catalogTestConnDriverMu   sync.Mutex
	catalogTestConnScenarios  = map[string]catalogTestConnScenario{}
)

func (c catalogTestConn) legacyMode() bool { return c.legacy }

func registerCatalogTestConnDriver(t *testing.T) {
	t.Helper()
	catalogTestConnDriverOnce.Do(func() {
		sql.Register("spi_catalog_test_driver", &catalogTestConnDriver{})
	})
}

func openCatalogTestConn(t *testing.T, scenario catalogTestConn) *sql.Conn {
	t.Helper()
	registerCatalogTestConnDriver(t)

	dsn := fmt.Sprintf("%s/%s/%t", t.Name(), time.Now().Format(time.RFC3339Nano), scenario.legacyMode())
	catalogTestConnDriverMu.Lock()
	catalogTestConnScenarios[dsn] = catalogTestConnScenario{legacy: scenario.legacyMode()}
	catalogTestConnDriverMu.Unlock()
	t.Cleanup(func() {
		catalogTestConnDriverMu.Lock()
		delete(catalogTestConnScenarios, dsn)
		catalogTestConnDriverMu.Unlock()
	})

	db, err := sql.Open("spi_catalog_test_driver", dsn)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, db.Close())
	})

	conn, err := db.Conn(t.Context())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, conn.Close())
	})

	return conn
}

func (d *catalogTestConnDriver) Open(name string) (driver.Conn, error) {
	catalogTestConnDriverMu.Lock()
	scenario, ok := catalogTestConnScenarios[name]
	catalogTestConnDriverMu.Unlock()
	if !ok {
		return nil, fmt.Errorf("catalog test scenario not found for dsn %q", name)
	}
	return &catalogTestConnDriverConn{scenario: catalogTestConn{legacy: scenario.legacy}}, nil
}

func (c *catalogTestConnDriverConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("not implemented Prepare")
}

func (c *catalogTestConnDriverConn) Close() error { return nil }

func (c *catalogTestConnDriverConn) Begin() (driver.Tx, error) {
	return nil, errors.New("not implemented Begin")
}

func (c *catalogTestConnDriverConn) QueryContext(_ context.Context, sqlText string, args []driver.NamedValue) (driver.Rows, error) {
	params := make([]any, len(args))
	for i, arg := range args {
		params[i] = arg.Value
	}

	if c.scenario.legacyMode() && strings.Contains(sqlText, "from V$DATABASES") {
		return nil, fmt.Errorf("legacy server does not support V$DATABASES")
	}

	switch {
	case strings.Contains(sqlText, "from V$DATABASES"):
		name := "DB_A"
		if len(params) == 1 {
			name = strings.ToUpper(params[0].(string))
		}
		return catalogTestConnRowsFrom([]string{"NAME", "DATABASE_ID"}, [][]driver.Value{{name, catalogTestDatabaseID(name)}}), nil
	case strings.Contains(sqlText, "from V$STORAGE_MOUNT_DATABASES"):
		if len(params) != 1 || params[0] != "ARCHIVE_DB" {
			return nil, fmt.Errorf("unexpected mounted database params: %v", params)
		}
		return catalogTestConnRowsFrom([]string{"BACKUP_TBSID"}, [][]driver.Value{{int64(303)}}), nil
	case strings.Contains(sqlText, "select count(*) from M$SYS_TABLES"):
		if len(params) != 3 || params[0] != "SYS" || params[2] != "SHARED_TABLE" {
			return nil, fmt.Errorf("unexpected table existence params: %v", params)
		}
		dbID := params[1].(int64)
		count := int64(0)
		if dbID == -1 || dbID == 101 || dbID == 202 || dbID == 303 {
			count = 1
		}
		return catalogTestConnRowsFrom([]string{"COUNT"}, [][]driver.Value{{count}}), nil
	case strings.Contains(sqlText, "j.COLCOUNT as TABLE_COLCOUNT"):
		if len(params) != 3 || params[0] != "SYS" || params[2] != "SHARED_TABLE" {
			return nil, fmt.Errorf("unexpected table description params: %v", params)
		}
		return catalogTestConnRowsFrom([]string{"TABLE_ID", "TABLE_TYPE", "TABLE_FLAG", "TABLE_COLCOUNT"}, [][]driver.Value{{int64(77), int64(client.TableTypeLog), int64(client.TableFlagNone), int64(1)}}), nil
	case strings.Contains(sqlText, "from M$SYS_COLUMNS"):
		dbID, err := catalogTestDBParam(params, 2, 1)
		if err != nil {
			return nil, err
		}
		return catalogTestConnRowsFrom([]string{"NAME", "TYPE", "LENGTH", "ID", "FLAG"}, [][]driver.Value{{catalogTestPrefix(dbID) + "_VALUE", int64(api.ColumnTypeInteger), int64(11), int64(0), int64(api.ColumnFlag(0))}}), nil
	case strings.Contains(sqlText, "M$SYS_INDEXES"):
		dbID, err := catalogTestDBParam(params, 3, 1)
		if err != nil {
			return nil, err
		}
		if params[2].(int64) != dbID {
			return nil, fmt.Errorf("database ID was not applied to both index tables: %v", params)
		}
		return catalogTestConnRowsFrom([]string{"NAME", "TYPE", "ID", "KEY_COMPRESS", "MAX_LEVEL", "PART_VALUE_COUNT", "BITMAP_ENCODE"}, [][]driver.Value{{catalogTestPrefix(dbID) + "_IDX", int64(client.IndexTypeRedBlack), int64(88), int64(0), int64(0), int64(0), "EQUAL"}}), nil
	case strings.Contains(sqlText, "M$SYS_INDEX_COLUMNS"):
		dbID, err := catalogTestDBParam(params, 2, 1)
		if err != nil {
			return nil, err
		}
		return catalogTestConnRowsFrom([]string{"NAME"}, [][]driver.Value{{catalogTestPrefix(dbID) + "_VALUE"}}), nil
	default:
		return nil, fmt.Errorf("unexpected Query: %s", sqlText)
	}
}

func catalogTestConnRowsFrom(columns []string, rows [][]driver.Value) driver.Rows {
	return &catalogTestConnRows{columns: columns, rows: rows}
}

func (r *catalogTestConnRows) Columns() []string { return r.columns }

func (r *catalogTestConnRows) Close() error { return nil }

func (r *catalogTestConnRows) Next(dest []driver.Value) error {
	if r.err != nil {
		return r.err
	}
	if r.index >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.index])
	r.index++
	return nil
}

func catalogTestDBParam(params []any, count, index int) (int64, error) {
	if len(params) != count {
		return 0, fmt.Errorf("unexpected params: %v", params)
	}
	dbID, ok := params[index].(int64)
	if !ok || (dbID != -1 && dbID != 101 && dbID != 202 && dbID != 303) {
		return 0, fmt.Errorf("unexpected database ID: %v", params[index])
	}
	return dbID, nil
}

func catalogTestDatabaseID(name string) int64 {
	if name == "DB_B" {
		return 202
	}
	return 101
}

func catalogTestPrefix(dbID int64) string {
	switch dbID {
	case -1:
		return "LEGACY"
	case 202:
		return "B"
	case 303:
		return "ARCHIVE"
	default:
		return "A"
	}
}

func TestCatalogTablesAreIsolatedByDatabase(t *testing.T) {
	ctx := context.Background()
	conn := openCatalogTestConn(t, catalogTestConn{})

	for _, name := range []string{"SHARED_TABLE", "DB_B.SYS.SHARED_TABLE"} {
		exists, err := ExistsTable(ctx, conn, name)
		if err != nil {
			t.Fatalf("ExistsTable(%q): %v", name, err)
		}
		if !exists {
			t.Fatalf("ExistsTable(%q) = false", name)
		}
	}

	tests := []struct {
		name       string
		database   string
		columnName string
		indexName  string
	}{
		{name: "SHARED_TABLE", database: "DB_A", columnName: "A_VALUE", indexName: "A_IDX"},
		{name: "DB_B.SYS.SHARED_TABLE", database: "DB_B", columnName: "B_VALUE", indexName: "B_IDX"},
	}
	for _, tt := range tests {
		desc, err := DescribeTable(ctx, conn, tt.name, false)
		if err != nil {
			t.Fatalf("DescribeTable(%q): %v", tt.name, err)
		}
		if desc.Database != tt.database || len(desc.Columns) != 1 || desc.Columns[0].Name != tt.columnName {
			t.Fatalf("DescribeTable(%q) returned wrong table: %#v", tt.name, desc)
		}
		if len(desc.Indexes) != 1 || desc.Indexes[0].Name != tt.indexName || !reflect.DeepEqual(desc.Indexes[0].Cols, []string{tt.columnName}) {
			t.Fatalf("DescribeTable(%q) returned wrong index: %#v", tt.name, desc.Indexes)
		}
	}
}

func TestDatabaseIDFallsBackForLegacyServer(t *testing.T) {
	ctx := context.Background()
	conn := openCatalogTestConn(t, catalogTestConn{legacy: true})

	currentID, err := DatabaseID(ctx, conn, "")
	if err != nil || currentID != -1 {
		t.Fatalf("current legacy database ID = %d, %v; want -1", currentID, err)
	}
	mountedID, err := DatabaseID(ctx, conn, "archive_db")
	if err != nil || mountedID != 303 {
		t.Fatalf("mounted legacy database ID = %d, %v; want 303", mountedID, err)
	}
}

func TestCatalogTablesFallBackForLegacyServer(t *testing.T) {
	ctx := context.Background()
	conn := openCatalogTestConn(t, catalogTestConn{legacy: true})
	tests := []struct {
		name       string
		database   string
		columnName string
		indexName  string
	}{
		{name: "SHARED_TABLE", database: "MACHBASEDB", columnName: "LEGACY_VALUE", indexName: "LEGACY_IDX"},
		{name: "ARCHIVE_DB.SYS.SHARED_TABLE", database: "ARCHIVE_DB", columnName: "ARCHIVE_VALUE", indexName: "ARCHIVE_IDX"},
	}
	for _, tt := range tests {
		desc, err := DescribeTable(ctx, conn, tt.name, false)
		if err != nil {
			t.Fatalf("DescribeTable(%q): %v", tt.name, err)
		}
		if desc.Database != tt.database || len(desc.Columns) != 1 || desc.Columns[0].Name != tt.columnName {
			t.Fatalf("DescribeTable(%q) returned wrong legacy table: %#v", tt.name, desc)
		}
		if len(desc.Indexes) != 1 || desc.Indexes[0].Name != tt.indexName {
			t.Fatalf("DescribeTable(%q) returned wrong legacy index: %#v", tt.name, desc.Indexes)
		}
	}
}

func TestParseProxyUserName(t *testing.T) {
	tests := []struct {
		name              string
		input             string
		expectedLoginName string
		expectedProxyUser string
		expectedAllowed   bool
		expectedString    string
	}{
		{
			name:              "no proxy user",
			input:             "alice",
			expectedLoginName: "alice",
			expectedProxyUser: "",
			expectedAllowed:   false,
			expectedString:    "alice",
		},
		{
			name:              "with proxy user",
			input:             "Sys As proxy",
			expectedLoginName: "Sys",
			expectedProxyUser: "proxy",
			expectedAllowed:   true,
			expectedString:    "sys as proxy",
		},
		{
			name:              "proxy user same as login",
			input:             "sys as sys",
			expectedLoginName: "sys",
			expectedProxyUser: "",
			expectedAllowed:   false,
			expectedString:    "sys",
		},
		{
			name:              "non-sys login with proxy format",
			input:             "Proxy as other",
			expectedLoginName: "Proxy",
			expectedProxyUser: "other",
			expectedAllowed:   false,
			expectedString:    "proxy as other",
		},
		{
			name:              "invalid format",
			input:             "PROXY other",
			expectedLoginName: "PROXY other",
			expectedProxyUser: "",
			expectedAllowed:   false,
			expectedString:    "proxy other",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			un := &UserName{}
			allowed := un.Parse(tt.input)
			require.Equal(t, tt.expectedLoginName, un.Login)
			require.Equal(t, tt.expectedProxyUser, un.Proxy)
			require.Equal(t, tt.expectedAllowed, allowed)
			require.Equal(t, tt.expectedString, un.String())
		})
	}
}

func TestParseTableName(t *testing.T) {
	tests := []struct {
		name            string
		input           string
		expectedDB      string
		expectedUser    string
		expectedTable   string
		expectedOrDB    string
		expectedOrUser  string
		expectedOrTable string
	}{
		{
			name:            "table_only",
			input:           "example",
			expectedDB:      "MACHBASEDB",
			expectedUser:    "SYS",
			expectedTable:   "EXAMPLE",
			expectedOrDB:    "db0",
			expectedOrUser:  "user0",
			expectedOrTable: "EXAMPLE",
		},
		{
			name:            "user.table",
			input:           "sys.example",
			expectedDB:      "MACHBASEDB",
			expectedUser:    "SYS",
			expectedTable:   "EXAMPLE",
			expectedOrDB:    "db0",
			expectedOrUser:  "SYS",
			expectedOrTable: "EXAMPLE",
		},
		{
			name:            "db.user.table",
			input:           "testdb.sys.example",
			expectedDB:      "TESTDB",
			expectedUser:    "SYS",
			expectedTable:   "EXAMPLE",
			expectedOrDB:    "TESTDB",
			expectedOrUser:  "SYS",
			expectedOrTable: "EXAMPLE",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, user, table := TableName(tt.input).Split()
			require.Equal(t, tt.expectedDB, db)
			require.Equal(t, tt.expectedUser, user)
			require.Equal(t, tt.expectedTable, table)
			db, user, table = TableName(tt.input).SplitOr("db0", "user0")
			require.Equal(t, tt.expectedOrDB, db)
			require.Equal(t, tt.expectedOrUser, user)
			require.Equal(t, tt.expectedOrTable, table)
		})
	}
}
