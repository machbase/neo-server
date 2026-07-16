package spi

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/machbase/neo-client/api"
	metricpkg "github.com/machbase/neo-server/v8/mods/util/metric"
	"github.com/stretchr/testify/require"
)

type stubSQLResult struct {
	rowsAffected int64
	err          error
}

type stubConn struct {
	execFunc func(ctx context.Context, sqlText string, params ...any) api.Result
	execSQLs []string
	execArgs [][]any
}

type badMetricValue struct{}

func (b badMetricValue) String() string {
	return "bad"
}

func (s stubSQLResult) LastInsertId() (int64, error) {
	return 0, nil
}

func (s stubSQLResult) RowsAffected() (int64, error) {
	return s.rowsAffected, s.err
}

func (s *stubConn) Close() error {
	return nil
}

func (s *stubConn) Exec(ctx context.Context, sqlText string, params ...any) api.Result {
	s.execSQLs = append(s.execSQLs, sqlText)
	s.execArgs = append(s.execArgs, params)
	if s.execFunc != nil {
		return s.execFunc(ctx, sqlText, params...)
	}
	return &InsertResult{rowsAffected: 1, message: "a row inserted"}
}

func (s *stubConn) Query(ctx context.Context, sqlText string, params ...any) (api.Rows, error) {
	return nil, errors.New("not implemented")
}

func (s *stubConn) QueryRow(ctx context.Context, sqlText string, params ...any) api.Row {
	return nil
}

func (s *stubConn) Prepare(ctx context.Context, query string) (api.Stmt, error) {
	return nil, errors.New("not implemented")
}

func (s *stubConn) Appender(ctx context.Context, tableName string, opts ...api.AppenderOption) (api.Appender, error) {
	return nil, errors.New("not implemented")
}

func (s *stubConn) Explain(ctx context.Context, sqlText string, full bool) (string, error) {
	return "", errors.New("not implemented")
}

func TestWrappedSqlResultMessage(t *testing.T) {
	tests := []struct {
		name    string
		sqlType SQLStatementType
		rows    int64
		err     error
		expect  string
	}{
		{name: "insert zero", sqlType: SQLStatementTypeInsert, rows: 0, expect: "no rows inserted."},
		{name: "insert one", sqlType: SQLStatementTypeInsert, rows: 1, expect: "a row inserted."},
		{name: "insert many", sqlType: SQLStatementTypeInsert, rows: 3, expect: "3 rows inserted."},
		{name: "update zero", sqlType: SQLStatementTypeUpdate, rows: 0, expect: "no rows updated."},
		{name: "update one", sqlType: SQLStatementTypeUpdate, rows: 1, expect: "a row updated."},
		{name: "update many", sqlType: SQLStatementTypeUpdate, rows: 4, expect: "4 rows updated."},
		{name: "delete zero", sqlType: SQLStatementTypeDelete, rows: 0, expect: "no rows deleted."},
		{name: "delete one", sqlType: SQLStatementTypeDelete, rows: 1, expect: "a row deleted."},
		{name: "delete many", sqlType: SQLStatementTypeDelete, rows: 2, expect: "2 rows deleted."},
		{name: "create", sqlType: SQLStatementTypeCreate, expect: "Created successfully."},
		{name: "drop", sqlType: SQLStatementTypeDrop, expect: "Dropped successfully."},
		{name: "alter", sqlType: SQLStatementTypeAlter, expect: "Altered successfully."},
		{name: "select", sqlType: SQLStatementTypeSelect, expect: "Select successfully."},
		{name: "default", sqlType: SQLStatementType(-1), expect: "executed."},
		{name: "preset error", err: errors.New("boom"), expect: "boom"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := &WrappedSqlResult{sqlType: tc.sqlType, sqlResult: stubSQLResult{rowsAffected: tc.rows}, err: tc.err}
			require.Equal(t, tc.expect, result.Message())
		})
	}

	t.Run("rows affected error", func(t *testing.T) {
		result := &WrappedSqlResult{sqlResult: stubSQLResult{err: errors.New("rows affected failed")}}
		require.Zero(t, result.RowsAffected())
		require.EqualError(t, result.Err(), "rows affected failed")
	})
}

func TestWrappedSqlRowHelpers(t *testing.T) {
	t.Run("message and rows affected", func(t *testing.T) {
		row := &WrappedSqlRow{}
		require.Equal(t, int64(0), row.RowsAffected())
		require.Equal(t, "success", row.Message())
	})

	t.Run("scan returns preset error", func(t *testing.T) {
		expectErr := errors.New("preset")
		row := &WrappedSqlRow{err: expectErr}
		require.ErrorIs(t, row.Scan(new(int)), expectErr)
	})

	t.Run("scan detects index overflow", func(t *testing.T) {
		row := &WrappedSqlRow{values: []any{int64(1)}}
		err := row.Scan(new(int64), new(string))
		require.Error(t, err)
		require.Contains(t, err.Error(), "out of range")
	})

	t.Run("scan copies values", func(t *testing.T) {
		row := &WrappedSqlRow{
			values: []any{int64(7), "neo"},
			columns: api.Columns{
				{Name: "ID", DataType: api.DataTypeInt64},
				{Name: "NAME", DataType: api.DataTypeString},
			},
		}
		var id int64
		var name string
		require.NoError(t, row.Scan(&id, &name))
		require.Equal(t, int64(7), id)
		require.Equal(t, "neo", name)

		cols, err := row.Columns()
		require.NoError(t, err)
		require.Equal(t, []string{"ID", "NAME"}, cols.Names())
	})
}

func TestWrappedSqlRowsHelpers(t *testing.T) {
	rows := &WrappedSqlRows{sqlType: SQLStatementTypeSelect}
	require.True(t, rows.IsFetchable())
	require.Equal(t, int64(0), rows.RowsAffected())
	require.Equal(t, "no rows selected.", rows.Message())

	rows.sqlType = SQLStatementTypeInsert
	rows.rowCount = 2
	require.False(t, rows.IsFetchable())
	require.Equal(t, int64(2), rows.RowsAffected())
	require.Equal(t, "2 rows inserted.", rows.Message())
}

func TestSqlBridgeBaseHelpers(t *testing.T) {
	base := &SqlBridgeBase{}

	t.Run("conn wrapper", func(t *testing.T) {
		wrapped := base.Conn(nil)
		require.NotNil(t, wrapped)
	})

	newScanTypeTests := []struct {
		reflectType      string
		databaseTypeName string
		expectType       any
	}{
		{reflectType: "sql.NullBool", expectType: new(bool)},
		{reflectType: "sql.NullByte", expectType: new(uint8)},
		{reflectType: "sql.NullFloat64", expectType: new(float64)},
		{reflectType: "sql.NullInt16", expectType: new(int16)},
		{reflectType: "sql.NullInt32", expectType: new(int32)},
		{reflectType: "sql.NullInt64", expectType: new(int64)},
		{reflectType: "sql.NullString", expectType: new(string)},
		{reflectType: "sql.NullTime", expectType: new(time.Time)},
		{reflectType: "sql.RawBytes", expectType: new([]byte)},
		{reflectType: "[]uint8", expectType: new([]byte)},
		{reflectType: "bool", expectType: new(bool)},
		{reflectType: "int32", expectType: new(int32)},
		{reflectType: "int64", expectType: new(int64)},
		{reflectType: "string", expectType: new(string)},
		{reflectType: "time.Time", expectType: new(time.Time)},
	}
	for _, tc := range newScanTypeTests {
		ret := base.NewScanType(tc.reflectType, tc.databaseTypeName)
		require.IsType(t, tc.expectType, ret)
	}
	require.Nil(t, base.NewScanType("unknown", ""))

	now := time.Unix(1700000000, 0).UTC()
	values := []any{
		sql.RawBytes("raw"),
		&sql.NullBool{Bool: true, Valid: true},
		&sql.NullBool{},
		&sql.NullByte{Byte: 9, Valid: true},
		&sql.NullFloat64{Float64: 1.5, Valid: true},
		&sql.NullInt16{Int16: 16, Valid: true},
		&sql.NullInt32{Int32: 32, Valid: true},
		&sql.NullInt64{Int64: 64, Valid: true},
		&sql.NullString{String: "str", Valid: true},
		&sql.NullTime{Time: now, Valid: true},
		&sql.NullString{},
	}

	normalized := base.NormalizeType(values)
	require.Equal(t, []byte("raw"), normalized[0])
	require.Equal(t, true, normalized[1])
	require.Nil(t, normalized[2])
	require.Equal(t, uint8(9), normalized[3])
	require.Equal(t, 1.5, normalized[4])
	require.Equal(t, int16(16), normalized[5])
	require.Equal(t, int32(32), normalized[6])
	require.Equal(t, int64(64), normalized[7])
	require.Equal(t, "str", normalized[8])
	require.Equal(t, now, normalized[9])
	require.Nil(t, normalized[10])
}

func TestInfoValueObjects(t *testing.T) {
	t.Run("table info kind and values", func(t *testing.T) {
		tests := []struct {
			info   TableInfo
			expect string
		}{
			{info: TableInfo{Type: api.TableTypeLog, Flag: api.TableFlagData}, expect: "Log Table (data)"},
			{info: TableInfo{Type: api.TableTypeFixed, Flag: api.TableFlagMeta}, expect: "Fixed Table (meta)"},
			{info: TableInfo{Type: api.TableTypeTag, Flag: api.TableFlagStat}, expect: "Tag Table (stat)"},
			{info: TableInfo{Type: api.TableType(-1)}, expect: "undef"},
		}
		for _, tc := range tests {
			require.Equal(t, tc.expect, tc.info.Kind())
		}

		info := &TableInfo{Database: "DB", User: "SYS", Name: "T", Id: 1, Type: api.TableTypeLookup, Flag: api.TableFlagRollup}
		require.Equal(t, []any{"DB", "SYS", "T", int64(1), info.Type.ShortString(), info.Flag.String()}, info.Values())
	})
}

func TestMetricsAndWatcherHelpers(t *testing.T) {
	t.Run("metrics snapshot and filter helpers", func(t *testing.T) {
		oldMetricsDest := metricsDest
		oldCollector := collector
		defer func() {
			metricsDest = oldMetricsDest
			collector = oldCollector
		}()

		all := QueryStatzFilter()
		pass, order := all("any")
		require.True(t, pass)
		require.Zero(t, order)

		filter := QueryStatzFilter("machbase:*", "exact:key")
		pass, order = filter("machbase:session:stmt")
		require.True(t, pass)
		require.Zero(t, order)
		pass, order = filter("exact:key")
		require.True(t, pass)
		require.Equal(t, 1, order)
		pass, _ = filter("miss")
		require.False(t, pass)

		collector = nil
		AddInputFunc(func(*metricpkg.Gather) error { return nil })
		AddMetrics(metricpkg.Measure{Name: "noop", Value: 1, Type: metricpkg.GaugeType(metricpkg.UnitShort)})
		StopMetrics()

		require.NoError(t, SetMetricsDestTable(""))
		require.Equal(t, "", MetricsDestTable())
		require.NoError(t, onProduct(metricpkg.Product{Name: "noop", SeriesID: SERIES_ID_FINEST, Value: badMetricValue{}}))
	})

	t.Run("watcher string and error path", func(t *testing.T) {
		watcher := &Watcher{WatcherConfig: WatcherConfig{TableName: "tag_data", TagNames: []string{"a", "b"}, Parallelism: 2}, out: make(chan any, 1)}
		require.Equal(t, "Watcher {table:tag_data, tags:[a b], parallelism:2}", watcher.String())

		expectErr := errors.New("watch error")
		watcher.handleError(expectErr)
		received := <-watcher.out
		require.ErrorIs(t, received.(error), expectErr)
	})
}

func TestWriteLineProtocol(t *testing.T) {
	ctx := t.Context()
	ts := time.Unix(1700000000, 0).UTC()
	descColumns := api.Columns{
		{Name: "NAME", DataType: api.DataTypeString},
		{Name: "TIME", DataType: api.DataTypeDatetime},
		{Name: "VALUE", DataType: api.DataTypeFloat64},
		{Name: "HOST", DataType: api.DataTypeString},
		{Name: "PORT", DataType: api.DataTypeInt32},
	}

	t.Run("insert result helpers", func(t *testing.T) {
		result := &InsertResult{err: errors.New("insert err"), rowsAffected: 7, message: "custom"}
		require.EqualError(t, result.Err(), "insert err")
		require.Equal(t, int64(7), result.RowsAffected())
		require.Equal(t, "custom", result.Message())
	})

	t.Run("skip unsupported fields", func(t *testing.T) {
		conn := &stubConn{}
		result := WriteLineProtocol(ctx, conn, "tag_data", descColumns, "cpu", map[string]any{"status": "up"}, map[string]string{"HOST": "srv-a"}, ts)
		require.NoError(t, result.Err())
		require.Equal(t, int64(0), result.RowsAffected())
		require.Equal(t, "no rows inserted", result.Message())
		require.Empty(t, conn.execSQLs)
	})

	t.Run("single row with string tag only", func(t *testing.T) {
		conn := &stubConn{}
		result := WriteLineProtocol(ctx, conn, "tag_data", descColumns, "cpu", map[string]any{"usage": 12.5}, map[string]string{"HOST": "srv-a", "PORT": "1234"}, ts)
		require.NoError(t, result.Err())
		require.Equal(t, int64(1), result.RowsAffected())
		require.Equal(t, "a row inserted", result.Message())
		require.Len(t, conn.execSQLs, 1)
		require.Equal(t, "INSERT INTO tag_data(NAME,TIME,VALUE,HOST) VALUES(?,?,?,?)", conn.execSQLs[0])
		require.Equal(t, []any{"cpu.usage", ts, 12.5, "srv-a"}, conn.execArgs[0])
	})

	t.Run("multiple rows inserted", func(t *testing.T) {
		conn := &stubConn{}
		result := WriteLineProtocol(ctx, conn, "tag_data", descColumns, "cpu", map[string]any{"usage": float32(1.5), "temp": int64(3)}, map[string]string{"HOST": "srv-b"}, ts)
		require.NoError(t, result.Err())
		require.Equal(t, int64(2), result.RowsAffected())
		require.Equal(t, "2 rows inserted", result.Message())
		require.Len(t, conn.execSQLs, 2)
		for _, sqlText := range conn.execSQLs {
			require.Equal(t, "INSERT INTO tag_data(NAME,TIME,VALUE,HOST) VALUES(?,?,?,?)", sqlText)
		}
	})

	t.Run("abort on exec error", func(t *testing.T) {
		conn := &stubConn{}
		callCount := 0
		conn.execFunc = func(ctx context.Context, sqlText string, params ...any) api.Result {
			callCount++
			if callCount == 2 {
				return &InsertResult{err: errors.New("exec failed")}
			}
			return &InsertResult{rowsAffected: 1, message: "a row inserted"}
		}

		result := WriteLineProtocol(ctx, conn, "tag_data", descColumns, "cpu", map[string]any{"usage": 1.0, "temp": 2.0}, map[string]string{"HOST": "srv-c"}, ts)
		require.EqualError(t, result.Err(), "exec failed")
		require.Equal(t, int64(1), result.RowsAffected())
		require.Equal(t, "batch inserts aborted - INSERT INTO tag_data(NAME,TIME,VALUE,HOST) VALUES(?,?,?,?)", result.Message())
		require.Len(t, conn.execSQLs, 2)
	})
}

type sqlWrapDriverScenario struct {
	execResults map[string]int64
	queryRows   map[string]*sqlWrapRowsData
	queryErrs   map[string]error
}

type sqlWrapRowsData struct {
	cols    []sqlWrapColMeta
	rows    [][]driver.Value
	nextErr error
}

type sqlWrapColMeta struct {
	name     string
	dbType   string
	scanType reflect.Type
	length   *int64
	nullable *bool
}

type sqlWrapTestDriver struct{}

type sqlWrapTestConn struct {
	scenario *sqlWrapDriverScenario
}

type sqlWrapTestRows struct {
	idx  int
	data *sqlWrapRowsData
}

var sqlWrapScenarioStore sync.Map
var sqlWrapDriverSeq uint64

func (d *sqlWrapTestDriver) Open(name string) (driver.Conn, error) {
	v, ok := sqlWrapScenarioStore.Load(name)
	if !ok {
		return nil, fmt.Errorf("scenario %s not found", name)
	}
	return &sqlWrapTestConn{scenario: v.(*sqlWrapDriverScenario)}, nil
}

func (c *sqlWrapTestConn) Prepare(query string) (driver.Stmt, error) {
	return nil, errors.New("prepare not supported")
}

func (c *sqlWrapTestConn) Close() error {
	return nil
}

func (c *sqlWrapTestConn) Begin() (driver.Tx, error) {
	return nil, errors.New("tx not supported")
}

func (c *sqlWrapTestConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	if n, ok := c.scenario.execResults[query]; ok {
		return driver.RowsAffected(n), nil
	}
	return driver.RowsAffected(0), nil
}

func (c *sqlWrapTestConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	if err, ok := c.scenario.queryErrs[query]; ok {
		return nil, err
	}
	if rows, ok := c.scenario.queryRows[query]; ok {
		return &sqlWrapTestRows{data: rows}, nil
	}
	return &sqlWrapTestRows{data: &sqlWrapRowsData{}}, nil
}

func (r *sqlWrapTestRows) Columns() []string {
	ret := make([]string, len(r.data.cols))
	for i, col := range r.data.cols {
		ret[i] = col.name
	}
	return ret
}

func (r *sqlWrapTestRows) Close() error {
	return nil
}

func (r *sqlWrapTestRows) Next(dest []driver.Value) error {
	if r.idx >= len(r.data.rows) {
		if r.data.nextErr != nil {
			return r.data.nextErr
		}
		return io.EOF
	}
	copy(dest, r.data.rows[r.idx])
	r.idx++
	return nil
}

func (r *sqlWrapTestRows) ColumnTypeDatabaseTypeName(index int) string {
	if index < 0 || index >= len(r.data.cols) {
		return ""
	}
	return r.data.cols[index].dbType
}

func (r *sqlWrapTestRows) ColumnTypeScanType(index int) reflect.Type {
	if index < 0 || index >= len(r.data.cols) {
		return reflect.TypeOf(new(any)).Elem()
	}
	if r.data.cols[index].scanType == nil {
		return reflect.TypeOf(new(any)).Elem()
	}
	return r.data.cols[index].scanType
}

func (r *sqlWrapTestRows) ColumnTypeNullable(index int) (nullable, ok bool) {
	if index < 0 || index >= len(r.data.cols) {
		return false, false
	}
	if r.data.cols[index].nullable == nil {
		return false, false
	}
	return *r.data.cols[index].nullable, true
}

func (r *sqlWrapTestRows) ColumnTypeLength(index int) (length int64, ok bool) {
	if index < 0 || index >= len(r.data.cols) {
		return 0, false
	}
	if r.data.cols[index].length == nil {
		return 0, false
	}
	return *r.data.cols[index].length, true
}

func newSQLWrapTestConn(t *testing.T, scenario *sqlWrapDriverScenario) *sql.Conn {
	t.Helper()
	id := atomic.AddUint64(&sqlWrapDriverSeq, 1)
	driverName := fmt.Sprintf("spi_sql_wrap_test_driver_%d", id)
	dsn := fmt.Sprintf("spi_sql_wrap_test_dsn_%d", id)
	sql.Register(driverName, &sqlWrapTestDriver{})
	sqlWrapScenarioStore.Store(dsn, scenario)
	t.Cleanup(func() {
		sqlWrapScenarioStore.Delete(dsn)
	})
	db, err := sql.Open(driverName, dsn)
	require.NoError(t, err)
	conn, err := db.Conn(t.Context())
	require.NoError(t, err)
	return conn
}

func TestWrappedSqlConnBridgePaths(t *testing.T) {
	trueVal := true
	falseVal := false
	strLen := int64(40)
	scenario := &sqlWrapDriverScenario{
		execResults: map[string]int64{
			"insert into t values (1)": 3,
		},
		queryRows: map[string]*sqlWrapRowsData{
			"select id, name, nick from t": {
				cols: []sqlWrapColMeta{
					{name: "ID", dbType: "INTEGER", scanType: reflect.TypeOf(sql.NullInt64{}), nullable: &falseVal},
					{name: "NAME", dbType: "VARCHAR", scanType: reflect.TypeOf(sql.NullString{}), length: &strLen, nullable: &trueVal},
					{name: "NICK", dbType: "TEXT", scanType: reflect.TypeOf(sql.NullString{}), nullable: nil},
				},
				rows: [][]driver.Value{{int64(1), "neo", "n1"}},
			},
			"select one from t": {
				cols: []sqlWrapColMeta{{name: "ONE", dbType: "INTEGER", scanType: reflect.TypeOf(sql.NullInt64{}), nullable: &falseVal}},
				rows: [][]driver.Value{{int64(1)}},
			},
			"select empty from t": {
				cols: []sqlWrapColMeta{{name: "ID", dbType: "INTEGER", scanType: reflect.TypeOf(sql.NullInt64{}), nullable: &falseVal}},
				rows: [][]driver.Value{},
			},
		},
		queryErrs: map[string]error{
			"select broken": errors.New("query broken"),
		},
	}

	wrapped := WrapSqlConn(newSQLWrapTestConn(t, scenario))

	t.Run("exec and non-fetch query path", func(t *testing.T) {
		result := wrapped.Exec(t.Context(), "insert into t values (1)")
		require.NoError(t, result.Err())
		require.Equal(t, int64(3), result.RowsAffected())
		require.Equal(t, "3 rows inserted.", result.Message())

		rows, err := wrapped.Query(t.Context(), "insert into t values (1)")
		require.NoError(t, err)
		require.False(t, rows.IsFetchable())
		require.Equal(t, int64(3), rows.RowsAffected())
		require.Equal(t, "3 rows inserted.", rows.Message())
		cols, err := rows.Columns()
		require.NoError(t, err)
		require.Nil(t, cols)
		require.False(t, rows.Next())
		require.NoError(t, rows.Scan())
		require.NoError(t, rows.Err())
		require.NoError(t, rows.Close())
	})

	t.Run("fetch query columns nullable and select flow", func(t *testing.T) {
		rows, err := wrapped.Query(t.Context(), "select id, name, nick from t")
		require.NoError(t, err)
		require.True(t, rows.IsFetchable())

		cols, err := rows.Columns()
		require.NoError(t, err)
		require.Len(t, cols, 3)
		require.False(t, cols[0].Nullable)
		require.True(t, cols[1].Nullable)
		require.False(t, cols[2].Nullable)
		require.Equal(t, 40, cols[1].Length)

		buf, err := cols.MakeBuffer()
		require.NoError(t, err)
		require.True(t, rows.Next())
		require.NoError(t, rows.Scan(buf...))
		require.False(t, rows.Next())
		require.Equal(t, int64(1), rows.RowsAffected())
		require.Equal(t, "a row selected.", rows.Message())
		require.NoError(t, rows.Err())
		require.NoError(t, rows.Close())
	})

	t.Run("query row paths", func(t *testing.T) {
		row := wrapped.QueryRow(t.Context(), "select one from t")
		require.NoError(t, row.Err())
		var one int64
		require.NoError(t, row.Scan(&one))
		require.Equal(t, int64(1), one)

		emptyRow := wrapped.QueryRow(t.Context(), "select empty from t")
		require.ErrorIs(t, emptyRow.Err(), sql.ErrNoRows)

		errRow := wrapped.QueryRow(t.Context(), "select broken")
		require.EqualError(t, errRow.Err(), "query broken")
	})

	t.Run("scan type mapping and rows err path", func(t *testing.T) {
		mapScenario := &sqlWrapDriverScenario{
			queryRows: map[string]*sqlWrapRowsData{
				"select mapping": {
					cols: []sqlWrapColMeta{
						{name: "V", dbType: "VARCHAR", scanType: reflect.TypeOf(sql.NullString{})},
						{name: "B", dbType: "BOOLEAN", scanType: reflect.TypeOf(sql.NullBool{})},
						{name: "I8", dbType: "TINYINT", scanType: reflect.TypeOf(sql.NullByte{})},
						{name: "I32", dbType: "INTEGER", scanType: reflect.TypeOf(sql.NullInt32{})},
						{name: "F32", dbType: "FLOAT", scanType: reflect.TypeOf(float32(0))},
						{name: "DT", dbType: "DATETIME", scanType: reflect.TypeOf(sql.NullTime{})},
						{name: "BIN", dbType: "BLOB", scanType: reflect.TypeOf([]byte{})},
						{name: "X", dbType: "OTHER", scanType: reflect.TypeOf(new(any)).Elem()},
					},
					rows: [][]driver.Value{{"s", true, int64(1), int64(2), float64(1.2), time.Unix(0, 0), []byte{0x01}, "x"}},
				},
				"select with err": {
					cols:    []sqlWrapColMeta{{name: "ID", dbType: "INTEGER", scanType: reflect.TypeOf(sql.NullInt64{})}},
					rows:    [][]driver.Value{},
					nextErr: errors.New("rows failed"),
				},
			},
		}
		mapped := WrapSqlConn(newSQLWrapTestConn(t, mapScenario))

		rows, err := mapped.Query(t.Context(), "select mapping")
		require.NoError(t, err)
		cols, err := rows.Columns()
		require.NoError(t, err)
		require.Equal(t, api.DataTypeString, cols[0].DataType)
		require.Equal(t, api.DataTypeBoolean, cols[1].DataType)
		require.Equal(t, api.DataTypeInt16, cols[2].DataType)
		require.Equal(t, api.DataTypeInt32, cols[3].DataType)
		require.Equal(t, api.DataTypeFloat32, cols[4].DataType)
		require.Equal(t, api.DataTypeDatetime, cols[5].DataType)
		require.Equal(t, api.DataTypeBinary, cols[6].DataType)
		require.Equal(t, api.DataTypeAny, cols[7].DataType)
		require.NoError(t, rows.Close())

		errRows, err := mapped.Query(t.Context(), "select with err")
		require.NoError(t, err)
		require.False(t, errRows.Next())
		require.EqualError(t, errRows.Err(), "rows failed")
	})

	t.Run("not implemented methods and close", func(t *testing.T) {
		require.PanicsWithValue(t, "not implemented", func() {
			_, _ = wrapped.Prepare(t.Context(), "select 1")
		})
		_, appErr := wrapped.Appender(t.Context(), "t")
		require.ErrorContains(t, appErr, "not implemented")
		_, expErr := wrapped.Explain(t.Context(), "select 1", false)
		require.ErrorContains(t, expErr, "not implemented")
	})
}

func TestWrappedSqlRowsMessageAndErrBranches(t *testing.T) {
	tests := []struct {
		name    string
		sqlType SQLStatementType
		rows    int64
		expect  string
	}{
		{name: "select many", sqlType: SQLStatementTypeSelect, rows: 3, expect: "3 rows selected."},
		{name: "describe one", sqlType: SQLStatementTypeDescribe, rows: 1, expect: "a row selected."},
		{name: "update zero", sqlType: SQLStatementTypeUpdate, rows: 0, expect: "no rows updated."},
		{name: "update one", sqlType: SQLStatementTypeUpdate, rows: 1, expect: "a row updated."},
		{name: "delete many", sqlType: SQLStatementTypeDelete, rows: 5, expect: "5 rows deleted."},
		{name: "create", sqlType: SQLStatementTypeCreate, rows: 0, expect: "Created successfully."},
		{name: "drop", sqlType: SQLStatementTypeDrop, rows: 0, expect: "Dropped successfully."},
		{name: "alter", sqlType: SQLStatementTypeAlter, rows: 0, expect: "Altered successfully."},
		{name: "other", sqlType: SQLStatementTypeOther, rows: 0, expect: "executed."},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rows := &WrappedSqlRows{sqlType: tc.sqlType, rowCount: tc.rows}
			require.Equal(t, tc.expect, rows.Message())
		})
	}

	t.Run("err priority", func(t *testing.T) {
		rows := &WrappedSqlRows{err: errors.New("preset")}
		require.EqualError(t, rows.Err(), "preset")
	})
}

var _ driver.Result = stubSQLResult{}
var _ api.Conn = (*stubConn)(nil)
