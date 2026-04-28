package api

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"testing"
	"time"

	clientapi "github.com/machbase/neo-client/api"
	metricpkg "github.com/machbase/neo-server/v8/mods/util/metric"
	"github.com/stretchr/testify/require"
)

type stubSQLResult struct {
	rowsAffected int64
	err          error
}

type stubConn struct {
	execFunc func(ctx context.Context, sqlText string, params ...any) clientapi.Result
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

func (s *stubConn) Exec(ctx context.Context, sqlText string, params ...any) clientapi.Result {
	s.execSQLs = append(s.execSQLs, sqlText)
	s.execArgs = append(s.execArgs, params)
	if s.execFunc != nil {
		return s.execFunc(ctx, sqlText, params...)
	}
	return &InsertResult{rowsAffected: 1, message: "a row inserted"}
}

func (s *stubConn) Query(ctx context.Context, sqlText string, params ...any) (clientapi.Rows, error) {
	return nil, errors.New("not implemented")
}

func (s *stubConn) QueryRow(ctx context.Context, sqlText string, params ...any) clientapi.Row {
	return nil
}

func (s *stubConn) Prepare(ctx context.Context, query string) (clientapi.Stmt, error) {
	return nil, errors.New("not implemented")
}

func (s *stubConn) Appender(ctx context.Context, tableName string, opts ...clientapi.AppenderOption) (clientapi.Appender, error) {
	return nil, errors.New("not implemented")
}

func (s *stubConn) Explain(ctx context.Context, sqlText string, full bool) (string, error) {
	return "", errors.New("not implemented")
}

func TestWrappedSqlResultMessage(t *testing.T) {
	tests := []struct {
		name    string
		sqlType clientapi.SQLStatementType
		rows    int64
		err     error
		expect  string
	}{
		{name: "insert zero", sqlType: clientapi.SQLStatementTypeInsert, rows: 0, expect: "no rows inserted."},
		{name: "insert one", sqlType: clientapi.SQLStatementTypeInsert, rows: 1, expect: "a row inserted."},
		{name: "insert many", sqlType: clientapi.SQLStatementTypeInsert, rows: 3, expect: "3 rows inserted."},
		{name: "update zero", sqlType: clientapi.SQLStatementTypeUpdate, rows: 0, expect: "no rows updated."},
		{name: "update one", sqlType: clientapi.SQLStatementTypeUpdate, rows: 1, expect: "a row updated."},
		{name: "update many", sqlType: clientapi.SQLStatementTypeUpdate, rows: 4, expect: "4 rows updated."},
		{name: "delete zero", sqlType: clientapi.SQLStatementTypeDelete, rows: 0, expect: "no rows deleted."},
		{name: "delete one", sqlType: clientapi.SQLStatementTypeDelete, rows: 1, expect: "a row deleted."},
		{name: "delete many", sqlType: clientapi.SQLStatementTypeDelete, rows: 2, expect: "2 rows deleted."},
		{name: "create", sqlType: clientapi.SQLStatementTypeCreate, expect: "Created successfully."},
		{name: "drop", sqlType: clientapi.SQLStatementTypeDrop, expect: "Dropped successfully."},
		{name: "alter", sqlType: clientapi.SQLStatementTypeAlter, expect: "Altered successfully."},
		{name: "select", sqlType: clientapi.SQLStatementTypeSelect, expect: "Select successfully."},
		{name: "default", sqlType: clientapi.SQLStatementType(-1), expect: "executed."},
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
			columns: clientapi.Columns{
				{Name: "ID", DataType: clientapi.DataTypeInt64},
				{Name: "NAME", DataType: clientapi.DataTypeString},
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
	rows := &WrappedSqlRows{}
	require.True(t, rows.IsFetchable())
	require.Equal(t, int64(0), rows.RowsAffected())
	require.Equal(t, "success", rows.Message())
}

func TestSqlBridgeBaseHelpers(t *testing.T) {
	base := &SqlBridgeBase{}

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
	t.Run("table name helpers", func(t *testing.T) {
		require.Equal(t, "SYS.EXAMPLE", TableName("sys.example").String())
		db, user, table := TableName("metrics").SplitOr("db0", "user0")
		require.Equal(t, "db0", db)
		require.Equal(t, "user0", user)
		require.Equal(t, "METRICS", table)

		db, user, table = TableName("user1.metrics").SplitOr("db0", "user0")
		require.Equal(t, "db0", db)
		require.Equal(t, "USER1", user)
		require.Equal(t, "METRICS", table)

		db, user, table = TableName("db1.user1.metrics").SplitOr("db0", "user0")
		require.Equal(t, "DB1", db)
		require.Equal(t, "USER1", user)
		require.Equal(t, "METRICS", table)
	})

	t.Run("table info kind and values", func(t *testing.T) {
		tests := []struct {
			info   TableInfo
			expect string
		}{
			{info: TableInfo{Type: clientapi.TableTypeLog, Flag: clientapi.TableFlagData}, expect: "Log Table (data)"},
			{info: TableInfo{Type: clientapi.TableTypeFixed, Flag: clientapi.TableFlagMeta}, expect: "Fixed Table (meta)"},
			{info: TableInfo{Type: clientapi.TableTypeTag, Flag: clientapi.TableFlagStat}, expect: "Tag Table (stat)"},
			{info: TableInfo{Type: clientapi.TableType(-1)}, expect: "undef"},
		}
		for _, tc := range tests {
			require.Equal(t, tc.expect, tc.info.Kind())
		}

		info := &TableInfo{Database: "DB", User: "SYS", Name: "T", Id: 1, Type: clientapi.TableTypeLookup, Flag: clientapi.TableFlagRollup, err: errors.New("table err")}
		cols := info.Columns()
		require.Equal(t, []string{"DATABASE", "USER", "NAME", "ID", "TYPE", "FLAG"}, cols.Names())
		require.Equal(t, []any{"DB", "SYS", "T", int64(1), info.Type.ShortString(), info.Flag.String()}, info.Values())
		require.EqualError(t, info.Err(), "table err")
	})

	t.Run("misc info values", func(t *testing.T) {
		now := time.Unix(1700000000, 0).UTC()
		license := &LicenseInfo{Id: "id", Type: "dev", Customer: "cust", Project: "proj", CountryCode: "KR", InstallDate: "20240101", IssueDate: "20240102", LicenseStatus: "active"}
		require.Len(t, license.Columns(), 8)
		require.Equal(t, []any{"ID", "id", "TYPE", "dev", "CUSTOMER", "cust", "PROJECT", "proj", "COUNTRY_CODE", "KR", "INSTALL_DATE", "20240101", "ISSUE_DATE", "20240102", "LICENSE_STATUS", "active"}, license.Values())

		tag := &TagInfo{Database: "DB", User: "SYS", Table: "TAG_DATA", Name: "name", Id: 2, Summarized: true}
		require.Equal(t, []any{"DB", "SYS", "TAG_DATA", "name", int64(2), true}, tag.Values())

		tagStat := &TagStatInfo{Database: "DB", User: "SYS", Table: "TAG_DATA", Name: "name", RowCount: 3, MinTime: now, MaxTime: now, MinValue: 1.2, MinValueTime: now, MaxValue: 3.4, MaxValueTime: now, RecentRowTime: now}
		require.Len(t, tagStat.Columns(), 12)
		require.Equal(t, []any{"DB", "SYS", "TAG_DATA", "name", int64(3), now, now, 1.2, now, 3.4, now, now}, tagStat.Values())

		nonTagIndexGap := &IndexGapInfo{ID: 1, TableName: "T", IndexName: "IDX", Gap: 2, err: errors.New("gap err")}
		require.Equal(t, []string{"ID", "TABLE", "INDEX", "GAP"}, nonTagIndexGap.Columns().Names())
		require.Equal(t, []any{int64(1), "T", "IDX", int64(2)}, nonTagIndexGap.Values())
		require.EqualError(t, nonTagIndexGap.Err(), "gap err")

		tagIndexGap := &IndexGapInfo{IsTagIndex: true, ID: 2, Status: "OK", DiskGap: 3, MemoryGap: 4}
		require.Equal(t, []string{"ID", "STATUS", "DISK_GAP", "MEMORY_GAP"}, tagIndexGap.Columns().Names())
		require.Equal(t, []any{int64(2), "OK", int64(3), int64(4)}, tagIndexGap.Values())

		rollup := &RollupGapInfo{SrcTable: "SRC", RollupTable: "ROLL", SrcEndRID: 1, RollupEndRID: 2, Gap: 3, LastElapsed: time.Second, err: errors.New("rollup err")}
		require.Equal(t, []any{"SRC", "ROLL", int64(1), int64(2), int64(3), time.Second}, rollup.Values())
		require.EqualError(t, rollup.Err(), "rollup err")

		storage := &StorageInfo{TableName: "T", DataSize: 10, IndexSize: 20, TotalSize: 30, err: errors.New("storage err")}
		require.Equal(t, []any{"T", int64(10), int64(20), int64(30)}, storage.Values())
		require.EqualError(t, storage.Err(), "storage err")

		usage := &TableUsageInfo{TableName: "T", StorageUsage: 40, err: errors.New("usage err")}
		require.Equal(t, []any{"T", int64(40)}, usage.Values())
		require.EqualError(t, usage.Err(), "usage err")
	})

	t.Run("statement and session branch values", func(t *testing.T) {
		stmt := &StatementInfo{ID: 1, SessionID: 2, State: "RUN", Query: "select 1", RecordSize: 128}
		require.Equal(t, []any{int64(1), int64(2), "RUN", "", int64(128), nil, nil, "select 1"}, stmt.Values())

		stmtNeo := &StatementInfo{ID: 3, SessionID: 4, State: "APPEND", Query: "insert", IsNeo: true, AppendSuccessCount: 5, AppendFailureCount: 6, err: errors.New("stmt err")}
		require.Equal(t, []any{int64(3), int64(4), "APPEND", "neo", nil, int64(5), int64(6), "insert"}, stmtNeo.Values())
		require.EqualError(t, stmtNeo.Err(), "stmt err")

		now := time.Unix(1700000000, 0).UTC()
		sess := &SessionInfo{ID: 1, UserID: 2, UserName: "sys", LoginTime: now, MaxQPXMem: 64}
		require.Equal(t, []any{int64(1), int64(2), "sys", "", now, int64(64), nil}, sess.Values())

		sessNeo := &SessionInfo{ID: 3, UserID: 4, UserName: "neo", IsNeo: true, StmtCount: 7, err: errors.New("session err")}
		require.Equal(t, []any{int64(3), int64(4), "neo", "neo", nil, nil, int64(7)}, sessNeo.Values())
		require.EqualError(t, sessNeo.Err(), "session err")
	})
}

func TestMetricsAndWatcherHelpers(t *testing.T) {
	t.Run("metrics snapshot and filter helpers", func(t *testing.T) {
		oldRawConns := RawConns
		oldMetricsDest := metricsDest
		oldCollector := collector
		defer func() {
			RawConns = oldRawConns
			metricsDest = oldMetricsDest
			collector = oldCollector
			metricConnsInUse.Store(0)
			metricStmts.Store(0)
			metricStmtsInUse.Store(0)
			metricAppenders.Store(0)
			metricAppendersInUse.Store(0)
			metricQueryHwmSqlText.Set("")
			metricQueryHwmSqlArgs.Set("")
			metricQueryHwmElapse.Set(0)
			metricQueryHwmExecuteElapse.Set(0)
			metricQueryHwmLimitWait.Set(0)
			metricQueryHwmFetchElapse.Set(0)
			queryElapseHwm.Store(0)
		}()

		metricConnsInUse.Store(2)
		metricStmts.Store(3)
		metricStmtsInUse.Store(1)
		metricAppenders.Store(4)
		metricAppendersInUse.Store(2)
		metricQueryHwmSqlText.Set("select 1")
		metricQueryHwmSqlArgs.Set("[1]")
		metricQueryHwmElapse.Set(111)
		metricQueryHwmExecuteElapse.Set(22)
		metricQueryHwmLimitWait.Set(33)
		metricQueryHwmFetchElapse.Set(44)
		queryElapseHwm.Store(999)
		RawConns = func() int { return 7 }

		ResetQueryStatz()
		require.Zero(t, queryElapseHwm.Load())

		snapshot := StatzSnapshot()
		require.Equal(t, int64(3), snapshot.Stmts)
		require.Equal(t, int32(2), snapshot.ConnsInUse)
		require.Equal(t, int32(1), snapshot.StmtsInUse)
		require.Equal(t, int32(2), snapshot.AppendersInUse)
		require.Equal(t, int32(7), snapshot.RawConns)
		require.Equal(t, "select 1", snapshot.QueryHwmSql)
		require.Equal(t, "[1]", snapshot.QueryHwmSqlArg)
		require.Equal(t, uint64(111), snapshot.QueryHwm)
		require.Equal(t, uint64(22), snapshot.QueryHwmExec)
		require.Equal(t, uint64(33), snapshot.QueryHwmWait)
		require.Equal(t, uint64(44), snapshot.QueryHwmFetch)

		all := QueryStatzFilter(nil)
		pass, order := all("any")
		require.True(t, pass)
		require.Zero(t, order)

		filter := QueryStatzFilter([]string{"machbase:*", "exact:key"})
		pass, order = filter("machbase:session:stmt")
		require.True(t, pass)
		require.Zero(t, order)
		pass, order = filter("exact:key")
		require.True(t, pass)
		require.Equal(t, 1, order)
		pass, _ = filter("miss")
		require.False(t, pass)

		collector = nil
		AddMetricsFunc(func(*metricpkg.Gather) error { return nil })
		AddMetrics(metricpkg.Measure{Name: "noop", Value: 1, Type: metricpkg.GaugeType(metricpkg.UnitShort)})
		StopMetrics()

		require.NoError(t, SetMetricsDestTable(""))
		require.Equal(t, "", MetricsDestTable())
		require.NoError(t, (&SessionInput{}).Init())
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
	ctx := context.Background()
	ts := time.Unix(1700000000, 0).UTC()
	descColumns := clientapi.Columns{
		{Name: "NAME", DataType: clientapi.DataTypeString},
		{Name: "TIME", DataType: clientapi.DataTypeDatetime},
		{Name: "VALUE", DataType: clientapi.DataTypeFloat64},
		{Name: "HOST", DataType: clientapi.DataTypeString},
		{Name: "PORT", DataType: clientapi.DataTypeInt32},
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
		conn.execFunc = func(ctx context.Context, sqlText string, params ...any) clientapi.Result {
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

var _ driver.Result = stubSQLResult{}
var _ clientapi.Conn = (*stubConn)(nil)
