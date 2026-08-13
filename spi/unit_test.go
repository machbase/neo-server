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

	"github.com/machbase/neo-client/v2/api"
	metricpkg "github.com/machbase/neo-server/v8/mods/util/metric"
	"github.com/stretchr/testify/require"
)

type badMetricValue struct{}

func (b badMetricValue) String() string {
	return "bad"
}

type stubSQLResult struct {
	rowsAffected int64
	err          error
}

func (s stubSQLResult) LastInsertId() (int64, error) {
	return 0, nil
}

func (s stubSQLResult) RowsAffected() (int64, error) {
	return s.rowsAffected, s.err
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
		scenario := &sqlWrapDriverScenario{}
		conn := newSQLWrapTestConn(t, scenario)
		defer conn.Close()

		result := WriteLineProtocol(ctx, conn, "tag_data", descColumns, "cpu", map[string]any{"status": "up"}, map[string]string{"HOST": "srv-a"}, ts)
		require.NoError(t, result.Err())
		require.Equal(t, int64(0), result.RowsAffected())
		require.Equal(t, "no rows inserted", result.Message())
		require.Empty(t, scenario.execCalls)
	})

	t.Run("single row with string tag only", func(t *testing.T) {
		scenario := &sqlWrapDriverScenario{execResults: map[string]int64{
			"INSERT INTO tag_data(NAME,TIME,VALUE,HOST) VALUES(?,?,?,?)": 1,
		}}
		conn := newSQLWrapTestConn(t, scenario)
		defer conn.Close()

		result := WriteLineProtocol(ctx, conn, "tag_data", descColumns, "cpu", map[string]any{"usage": 12.5}, map[string]string{"HOST": "srv-a", "PORT": "1234"}, ts)
		require.NoError(t, result.Err())
		require.Equal(t, int64(1), result.RowsAffected())
		require.Equal(t, "a row inserted.", result.Message())
		require.Len(t, scenario.execCalls, 1)
		require.Equal(t, "INSERT INTO tag_data(NAME,TIME,VALUE,HOST) VALUES(?,?,?,?)", scenario.execCalls[0].query)
		require.Equal(t, []any{"cpu.usage", ts, 12.5, "srv-a"}, scenario.execCalls[0].args)
	})

	t.Run("multiple rows inserted", func(t *testing.T) {
		scenario := &sqlWrapDriverScenario{execResults: map[string]int64{
			"INSERT INTO tag_data(NAME,TIME,VALUE,HOST) VALUES(?,?,?,?)": 1,
		}}
		conn := newSQLWrapTestConn(t, scenario)
		defer conn.Close()

		result := WriteLineProtocol(ctx, conn, "tag_data", descColumns, "cpu", map[string]any{"usage": float32(1.5), "temp": int64(3)}, map[string]string{"HOST": "srv-b"}, ts)
		require.NoError(t, result.Err())
		require.Equal(t, int64(2), result.RowsAffected())
		require.Equal(t, "2 rows inserted.", result.Message())
		require.Len(t, scenario.execCalls, 2)
		for _, call := range scenario.execCalls {
			require.Equal(t, "INSERT INTO tag_data(NAME,TIME,VALUE,HOST) VALUES(?,?,?,?)", call.query)
		}
	})

	t.Run("abort on exec error", func(t *testing.T) {
		scenario := &sqlWrapDriverScenario{}
		callCount := 0
		scenario.execFunc = func(query string, args []driver.NamedValue) (driver.Result, error) {
			callCount++
			if callCount == 2 {
				return nil, errors.New("exec failed")
			}
			return driver.RowsAffected(1), nil
		}
		conn := newSQLWrapTestConn(t, scenario)
		defer conn.Close()

		result := WriteLineProtocol(ctx, conn, "tag_data", descColumns, "cpu", map[string]any{"usage": 1.0, "temp": 2.0}, map[string]string{"HOST": "srv-c"}, ts)
		require.EqualError(t, result.Err(), "exec failed")
		require.Equal(t, int64(1), result.RowsAffected())
		require.Equal(t, "batch inserts aborted - INSERT INTO tag_data(NAME,TIME,VALUE,HOST) VALUES(?,?,?,?)", result.Message())
		require.Len(t, scenario.execCalls, 2)
	})
}

type sqlWrapDriverScenario struct {
	execResults map[string]int64
	execFunc    func(query string, args []driver.NamedValue) (driver.Result, error)
	execCalls   []sqlWrapExecCall
	queryRows   map[string]*sqlWrapRowsData
	queryErrs   map[string]error
}

type sqlWrapExecCall struct {
	query string
	args  []any
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
	callArgs := make([]any, len(args))
	for i, arg := range args {
		callArgs[i] = arg.Value
	}
	c.scenario.execCalls = append(c.scenario.execCalls, sqlWrapExecCall{query: query, args: callArgs})
	if c.scenario.execFunc != nil {
		return c.scenario.execFunc(query, args)
	}
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
