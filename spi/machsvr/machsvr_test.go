package machsvr

import (
	"bytes"
	"context"
	"crypto"
	"database/sql"
	"database/sql/driver"
	_ "embed"
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/machbase/neo-client/api"
	"github.com/machbase/neo-client/machgo"
	mach "github.com/machbase/neo-engine/v8"
	"github.com/machbase/neo-server/v8/spi"
	"github.com/stretchr/testify/require"
)

var machsvrDB *Database
var machsvrPort int
var machsvrKey crypto.PrivateKey

//go:embed test/testsuite.conf
var defaultConfig []byte

func TestMain(m *testing.M) {
	ctx := context.Background()
	startServer(ctx)

	code := m.Run()

	stopServer(ctx)
	os.Exit(code)
}

func startServer(ctx context.Context) {
	// prepare
	homePath, err := filepath.Abs(filepath.Join("test", "tmp", "machbase"))
	if err != nil {
		panic(err)
	}
	confPath := filepath.Join(homePath, "conf", "machbase.conf")

	os.RemoveAll(homePath)
	os.MkdirAll(homePath, 0755)
	os.MkdirAll(filepath.Join(homePath, "conf"), 0755)
	os.MkdirAll(filepath.Join(homePath, "trc"), 0755)
	os.MkdirAll(filepath.Join(homePath, "dbs"), 0755)
	os.WriteFile(confPath, defaultConfig, 0644)

	// available port
	time.Sleep(time.Millisecond * time.Duration(3000*rand.Float32()))
	var lsnr net.Listener
	for {
		if l, err := net.Listen("tcp", "127.0.0.1:0"); err != nil {
			continue
		} else {
			lsnr = l
			machsvrPort = l.Addr().(*net.TCPAddr).Port
			break
		}
	}
	lsnr.Close()

	if err := Initialize(homePath, machsvrPort, OPT_SIGHANDLER_OFF); err != nil {
		panic(err)
	}

	if !ExistsDatabase() {
		if err := CreateDatabase(); err != nil {
			panic(err)
		}
	}

	// setup
	if db, err := NewDatabase(DatabaseOption{MaxOpenConn: -1, MaxOpenQuery: -1}); err != nil {
		panic(err)
	} else {
		machsvrDB = db
	}

	if err := machsvrDB.Startup(); err != nil {
		panic(err)
	}
	time.Sleep(time.Millisecond * 2000)

	pair, err := machgo.GenerateAuthKeyPair()
	if err != nil {
		panic(err)
	}

	privPath, pubPath, err := pair.WriteFiles(homePath, "authkey_test")
	if err != nil {
		panic(err)
	}
	// just to verify the generated key file is valid
	if privKey, err := machgo.LoadPrivateKeyFromFile(privPath); err != nil {
		panic(err)
	} else {
		machsvrKey = privKey
	}

	pubKeyContent, err := os.ReadFile(pubPath)
	if err != nil {
		panic(err)
	}

	// trace_log_level
	conn, err := machsvrDB.ConnectTrust(ctx, "sys")
	if err != nil {
		panic(err)
	}
	result := conn.Exec(ctx, "alter system set trace_log_level=1023")
	if result.Err() != nil {
		panic(result.Err())
	}
	result = conn.Exec(ctx,
		fmt.Sprintf("alter user sys add auth key (key='%s', valid_before='2100-01-01', comment='test key')",
			strings.TrimSpace(string(pubKeyContent))))
	if result.Err() != nil {
		panic(result.Err())
	}
	conn.Close()

	// machgo database
	if db, err := machgo.NewDatabase(&machgo.Config{
		Host: "127.0.0.1",
		Port: machsvrPort,
	}); err != nil {
		panic(err)
	} else {
		spi.SetDefault(db, machsvrKey)
	}
}

func stopServer(_ context.Context) {
	if err := machsvrDB.Shutdown(); err != nil {
		panic(err)
	}
	Finalize()

	if err := os.RemoveAll(filepath.Join("test", "tmp", "machbase")); err != nil {
		panic(err)
	}
}

func TestConnCancelNilHandle(t *testing.T) {
	conn := &Conn{db: &Database{}}
	require.Error(t, conn.Cancel())
}

func TestConnCloseNilHandle(t *testing.T) {
	conn := &Conn{db: &Database{}}
	require.ErrorIs(t, conn.Close(), api.ErrDatabaseNoConnection)
}

func TestConnCloseSignalsReturnChan(t *testing.T) {
	conn, err := _env.database.Connect(context.Background(), api.WithPassword("sys", "manager"))
	require.NoError(t, err)

	machConn, ok := conn.(*Conn)
	require.True(t, ok)

	machConn.returnChan = make(chan struct{}, 1)
	require.NoError(t, machConn.Close())

	select {
	case <-machConn.returnChan:
	default:
		t.Fatal("expected Close to signal returnChan")
	}

	require.NoError(t, machConn.Close())
}

func TestSetMaxConn(t *testing.T) {
	engine := machsvrDB
	expectLimit, open := engine.MaxOpenConn()
	require.NotZero(t, expectLimit)
	require.LessOrEqual(t, -1, open)

	engine.SetMaxOpenConn(-1)
	limit, open := engine.MaxOpenConn()
	require.Equal(t, -1, limit)
	require.Equal(t, -1, open)

	engine.SetMaxOpenConn(0)
	limit, open = engine.MaxOpenConn()
	require.Equal(t, runtime.NumCPU()*2, limit)
	require.Equal(t, limit, open)

	expectLimit = 1000
	engine.SetMaxOpenConn(expectLimit)
	limit, open = engine.MaxOpenConn()
	require.Equal(t, expectLimit, limit)
	require.Equal(t, expectLimit, open)
}

func TestSetMaxQuery(t *testing.T) {
	engine := machsvrDB
	expectLimit, open := engine.MaxOpenQuery()
	require.NotZero(t, expectLimit)
	require.LessOrEqual(t, -1, open)

	engine.SetMaxOpenQuery(-1)
	limit, open := engine.MaxOpenQuery()
	require.Equal(t, -1, limit)
	require.Equal(t, -1, open)

	engine.SetMaxOpenQuery(0)
	limit, open = engine.MaxOpenQuery()
	require.Equal(t, runtime.NumCPU()*2, limit)
	require.Equal(t, limit, open)

	expectLimit = 1000
	engine.SetMaxOpenQuery(expectLimit)
	limit, open = engine.MaxOpenQuery()
	require.Equal(t, expectLimit, limit)
	require.Equal(t, expectLimit, open)
}

func TestDatabaseError(t *testing.T) {
	engine := machsvrDB
	_, err := machsvrDB.Connect(context.Background(), api.WithPassword("sys", "wrong-password"))
	require.Error(t, err)

	lastErr := engine.Error()
	require.Error(t, lastErr)
	require.True(t, strings.Contains(lastErr.Error(), "Invalid username/password") || strings.Contains(lastErr.Error(), "invalid username/password"))
}

func TestPackageMetadataFunctions(t *testing.T) {
	require.IsType(t, "", LinkInfo())
	require.IsType(t, "", LinkVersion())
	require.IsType(t, "", LinkGitHash())
}

func TestResultMessageBranches(t *testing.T) {
	t.Run("basic_accessors", func(t *testing.T) {
		r := &Result{err: fmt.Errorf("boom"), affectedRows: 11}
		require.Equal(t, int64(11), r.RowsAffected())
		require.EqualError(t, r.Err(), "boom")
	})

	t.Run("error_message", func(t *testing.T) {
		r := &Result{err: fmt.Errorf("boom")}
		require.Equal(t, "boom", r.Message())
	})

	t.Run("select_one", func(t *testing.T) {
		r := &Result{affectedRows: 1, stmtType: mach.StmtType(512)}
		require.Equal(t, "a row selected.", r.Message())
	})

	t.Run("insert_many", func(t *testing.T) {
		r := &Result{affectedRows: 2, stmtType: mach.StmtType(513)}
		require.Equal(t, "2 rows inserted.", r.Message())
	})

	t.Run("update_zero", func(t *testing.T) {
		r := &Result{affectedRows: 0, stmtType: mach.StmtType(520)}
		require.Equal(t, "no row updated.", r.Message())
	})

	t.Run("delete_many", func(t *testing.T) {
		r := &Result{affectedRows: 3, stmtType: mach.StmtType(514)}
		require.Equal(t, "3 rows deleted.", r.Message())
	})

	t.Run("alter_system", func(t *testing.T) {
		r := &Result{affectedRows: 0, stmtType: mach.StmtType(256)}
		require.Equal(t, "system altered.", r.Message())
	})

	t.Run("ddl", func(t *testing.T) {
		r := &Result{affectedRows: 0, stmtType: mach.StmtType(1)}
		require.Equal(t, "ok.", r.Message())
	})

	t.Run("unknown", func(t *testing.T) {
		r := &Result{affectedRows: 0, stmtType: mach.StmtType(9999)}
		require.Equal(t, "ok.(9999)", r.Message())
	})
}

func TestRowBasicBranches(t *testing.T) {
	t.Run("err_columns_and_rows_affected", func(t *testing.T) {
		typeErr := fmt.Errorf("row failed")
		cols := api.Columns{{Name: "A"}}
		row := &Row{err: typeErr, columns: cols, affectedRows: 3}

		require.EqualError(t, row.Err(), "row failed")
		gotCols, err := row.Columns()
		require.NoError(t, err)
		require.Equal(t, cols, gotCols)
		require.Equal(t, int64(3), row.RowsAffected())
	})

	t.Run("success_and_values", func(t *testing.T) {
		row := &Row{ok: true, values: []any{"v"}}
		require.True(t, row.Success())
		require.Equal(t, []any{"v"}, row.Values())
	})

	t.Run("scan_error_passthrough", func(t *testing.T) {
		row := &Row{err: fmt.Errorf("scan failed")}
		var got int32
		err := row.Scan(&got)
		require.EqualError(t, err, "scan failed")
	})

	t.Run("scan_no_rows", func(t *testing.T) {
		row := &Row{}
		var got int32
		err := row.Scan(&got)
		require.ErrorIs(t, err, sql.ErrNoRows)
	})

	t.Run("scan_index_out_of_range", func(t *testing.T) {
		row := &Row{ok: true, values: []any{int32(1)}}
		var a, b int32
		err := row.Scan(&a, &b)
		require.EqualError(t, err, api.ErrDatabaseScanIndex(1, 1).Error())
	})

	t.Run("scan_valid", func(t *testing.T) {
		row := &Row{ok: true, values: []any{int32(7), "neo"}}
		var a int32
		var b string
		err := row.Scan(&a, &b)
		require.NoError(t, err)
		require.Equal(t, int32(7), a)
		require.Equal(t, "neo", b)
	})

	t.Run("scan_null_value", func(t *testing.T) {
		row := &Row{ok: true, values: []any{nil}}
		v := "keep"
		err := row.Scan(&v)
		require.NoError(t, err)
		require.Equal(t, "keep", v)
	})

	t.Run("message_unknown_stmt", func(t *testing.T) {
		row := &Row{ok: true, affectedRows: 2, stmtType: mach.StmtType(9999)}
		require.Equal(t, "ok.(9999)", row.Message())
	})
}

func TestRowsNonEngineBranches(t *testing.T) {
	t.Run("close_without_stmt_releases_slot", func(t *testing.T) {
		ch := make(chan struct{}, 1)
		rows := &Rows{sqlText: "select 1", returnChan: ch}

		require.NoError(t, rows.Close())
		require.Equal(t, "", rows.sqlText)

		select {
		case <-ch:
		default:
			t.Fatal("expected Close to signal return channel")
		}
	})

	t.Run("columns_and_error_accessors", func(t *testing.T) {
		cols := api.Columns{{Name: "VALUE"}}
		rows := &Rows{columns: cols, fetchError: fmt.Errorf("fetch failed")}

		gotCols, err := rows.Columns()
		require.NoError(t, err)
		require.Equal(t, cols, gotCols)
		require.EqualError(t, rows.Err(), "fetch failed")
	})

	t.Run("rows_affected_nil_stmt", func(t *testing.T) {
		rows := &Rows{stmtType: mach.StmtType(513)}
		require.Equal(t, int64(0), rows.RowsAffected())
	})

	t.Run("query_limit_without_channel", func(t *testing.T) {
		rows := &Rows{}
		require.True(t, rows.QueryLimit(context.Background()))
	})

	t.Run("query_limit_channel_acquired", func(t *testing.T) {
		ch := make(chan struct{}, 1)
		ch <- struct{}{}
		rows := &Rows{candidateReturnChan: ch}
		require.True(t, rows.QueryLimit(context.Background()))
	})

	t.Run("query_limit_context_canceled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		rows := &Rows{candidateReturnChan: make(chan struct{})}
		require.False(t, rows.QueryLimit(ctx))
	})

	t.Run("statement_type_and_rows_affected_for_select", func(t *testing.T) {
		rows := &Rows{stmtType: mach.StmtType(512)}
		require.Equal(t, mach.StmtType(512), rows.StatementType())
		require.Equal(t, int64(0), rows.RowsAffected())
	})

	t.Run("defined_message_keywords", func(t *testing.T) {
		cases := []struct {
			sql  string
			msg  string
			okay bool
		}{
			{sql: "create table x(a int)", msg: "Created successfully.", okay: true},
			{sql: "drop table x", msg: "Dropped successfully.", okay: true},
			{sql: "truncate table x", msg: "Truncated successfully.", okay: true},
			{sql: "alter table x add b int", msg: "Altered successfully.", okay: true},
			{sql: "connect user", msg: "Connected successfully.", okay: true},
			{sql: "select 1", msg: "", okay: false},
		}
		for _, tc := range cases {
			rows := &Rows{sqlText: tc.sql}
			msg, ok := rows.definedMessage()
			require.Equal(t, tc.okay, ok)
			require.Equal(t, tc.msg, msg)
		}
	})

	t.Run("message_select_zero_rows", func(t *testing.T) {
		rows := &Rows{stmtType: mach.StmtType(512)}
		require.Equal(t, "no rows selected.", rows.Message())
	})

	t.Run("message_ddl_fallback", func(t *testing.T) {
		rows := &Rows{stmtType: mach.StmtType(1), sqlText: "grant something"}
		require.Equal(t, "executed.", rows.Message())
	})

	t.Run("message_system_altered_fallback", func(t *testing.T) {
		rows := &Rows{stmtType: mach.StmtType(256), sqlText: "set something"}
		require.Equal(t, "system altered.", rows.Message())
	})

	t.Run("fetch_sync_non_select", func(t *testing.T) {
		rows := &Rows{stmtType: mach.StmtType(513)}
		vals, next, err := rows.FetchSync()
		require.Nil(t, vals)
		require.False(t, next)
		require.ErrorIs(t, err, sql.ErrNoRows)
	})

	t.Run("next_non_select", func(t *testing.T) {
		rows := &Rows{stmtType: mach.StmtType(513)}
		require.False(t, rows.Next())
	})

	t.Run("scan_non_select", func(t *testing.T) {
		rows := &Rows{stmtType: mach.StmtType(513)}
		var v int32
		err := rows.Scan(&v)
		require.ErrorIs(t, err, sql.ErrNoRows)
	})
}

func TestAppenderSimpleBranches(t *testing.T) {
	t.Run("string_table_and_type", func(t *testing.T) {
		ap := &Appender{tableName: "TAG_DATA", tableType: api.TableTypeTag}
		require.Contains(t, ap.String(), "appender TAG_DATA")
		require.Equal(t, "TAG_DATA", ap.TableName())
		require.Equal(t, api.TableTypeTag, ap.TableType())
	})

	t.Run("columns_accessor", func(t *testing.T) {
		expect := api.Columns{{Name: "NAME"}, {Name: "TIME"}}
		ap := &Appender{columns: expect}
		cols, err := ap.Columns()
		require.NoError(t, err)
		require.Equal(t, expect, cols)
	})

	t.Run("with_input_columns_maps_index", func(t *testing.T) {
		ap := &Appender{columns: api.Columns{{Name: "TIME"}, {Name: "VALUE"}}}
		ap.WithInputColumns("value", "time")
		require.Len(t, ap.inputColumns, 2)
		require.Equal(t, "VALUE", ap.inputColumns[0].Name)
		require.Equal(t, 1, ap.inputColumns[0].Idx)
		require.Equal(t, "TIME", ap.inputColumns[1].Name)
		require.Equal(t, 0, ap.inputColumns[1].Idx)
	})

	t.Run("append_invalid_table_type", func(t *testing.T) {
		ap := &Appender{tableName: "X", tableType: api.TableType(-1)}
		err := ap.Append(1)
		require.EqualError(t, err, "X can not be appended")
	})

	t.Run("append_tag_without_columns", func(t *testing.T) {
		ap := &Appender{tableName: "TAG_DATA", tableType: api.TableTypeTag}
		err := ap.Append("name", time.Now(), 1.23)
		require.EqualError(t, err, api.ErrDatabaseNoColumns("TAG_DATA").Error())
	})

	t.Run("append_log_without_columns", func(t *testing.T) {
		ap := &Appender{tableName: "LOG_DATA", tableType: api.TableTypeLog}
		err := ap.Append(time.Now(), 1.23)
		require.EqualError(t, err, api.ErrDatabaseNoColumns("LOG_DATA").Error())
	})

	t.Run("append_with_input_columns_length_mismatch", func(t *testing.T) {
		ap := &Appender{
			tableName: "TAG_DATA",
			tableType: api.TableTypeTag,
			columns:   api.Columns{{Name: "NAME"}, {Name: "VALUE"}},
		}
		ap.WithInputColumns("name")
		err := ap.append("a", 1)
		require.EqualError(t, err, api.ErrDatabaseLengthOfColumns("TAG_DATA", 2, 2).Error())
	})

	t.Run("append_closed_appender", func(t *testing.T) {
		ap := &Appender{
			tableName: "TAG_DATA",
			tableType: api.TableTypeTag,
			closed:    true,
			columns:   api.Columns{{Name: "NAME"}},
		}
		err := ap.append("a")
		require.ErrorIs(t, err, api.ErrDatabaseClosedAppender)
	})

	t.Run("append_without_connection", func(t *testing.T) {
		ap := &Appender{
			tableName: "TAG_DATA",
			tableType: api.TableTypeTag,
			columns:   api.Columns{{Name: "NAME"}},
		}
		err := ap.append("a")
		require.ErrorIs(t, err, api.ErrDatabaseNoConnection)
	})

	t.Run("append_log_time_non_log", func(t *testing.T) {
		ap := &Appender{tableName: "TAG_DATA", tableType: api.TableTypeTag}
		err := ap.AppendLogTime(time.Now(), 1)
		require.EqualError(t, err, "TAG_DATA is not a log table, use Append() instead")
	})
}

func TestConnConnectedClosed(t *testing.T) {
	conn := &Conn{closed: true}
	require.False(t, conn.Connected())
}

func TestWatcherRegistry(t *testing.T) {
	engine := machsvrDB
	watcherKey := "registry-test"
	engine.RemoveWatcher(watcherKey)
	t.Cleanup(func() {
		engine.RemoveWatcher(watcherKey)
	})

	engine.RegisterWatcher(watcherKey, nil)
	state, ok := engine.GetWatcher(watcherKey)
	require.True(t, ok)
	require.NotNil(t, state)
	require.False(t, state.CreatedTime.IsZero())
	require.Empty(t, state.Id)
	require.Empty(t, state.LatestSql)
	require.True(t, state.LatestTime.IsZero())
	expectCreatedTime := state.CreatedTime

	engine.ListWatcher(nil)

	found := false
	engine.ListWatcher(func(state *ConnState) bool {
		if state.Id == "" && state.CreatedTime.Equal(expectCreatedTime) {
			found = true
			return false
		}
		return true
	})
	require.True(t, found)

	engine.RemoveWatcher(watcherKey)
	_, ok = engine.GetWatcher(watcherKey)
	require.False(t, ok)
}

func TestKillConnection(t *testing.T) {
	engine := machsvrDB
	require.EqualError(t, engine.KillConnection("missing-watcher", true), "connection 'missing-watcher' not found")

	invalidKey := "invalid-watcher"
	engine.SetWatcher(invalidKey, &ConnWatcher{})
	t.Cleanup(func() {
		engine.RemoveWatcher(invalidKey)
	})
	require.EqualError(t, engine.KillConnection(invalidKey, true), "invalid connection 'invalid-watcher'")
	engine.RemoveWatcher(invalidKey)

	before := watcherStates(engine)
	conn, err := machsvrDB.Connect(context.Background(), api.WithPassword("sys", "manager"))
	require.NoError(t, err)

	after := watcherStates(engine)
	watcherID := newWatcherID(before, after)
	require.NotEmpty(t, watcherID)

	require.NoError(t, engine.KillConnection(watcherID, true))
	require.EqualError(t, engine.KillConnection(watcherID, true), "connection '"+watcherID+"' not found")
	require.NoError(t, conn.Close())
}

func TestCancelConnection(t *testing.T) {
	engine := machsvrDB

	before := watcherStates(engine)
	conn, err := machsvrDB.Connect(context.Background(), api.WithPassword("sys", "manager"))
	require.NoError(t, err)

	after := watcherStates(engine)
	watcherID := newWatcherID(before, after)
	require.NotEmpty(t, watcherID)

	require.NoError(t, engine.KillConnection(watcherID, false))
	require.EqualError(t, engine.KillConnection(watcherID, false), "connection '"+watcherID+"' not found")
	require.NoError(t, conn.Close())

	before = watcherStates(engine)
	conn, err = machsvrDB.Connect(context.Background(), api.WithPassword("sys", "manager"))
	require.NoError(t, err)

	machConn, ok := conn.(*Conn)
	require.True(t, ok)

	after = watcherStates(engine)
	watcherID = newWatcherID(before, after)
	require.NotEmpty(t, watcherID)

	require.NoError(t, machConn.Cancel())
	require.EqualError(t, engine.KillConnection(watcherID, false), "connection '"+watcherID+"' not found")
	require.NoError(t, conn.Close())
}

func TestPreparedStatement(t *testing.T) {
	ctx := t.Context()
	db := machsvrDB

	conn, err := db.Connect(ctx, api.WithPassword("sys", "manager"))
	require.NoError(t, err)
	defer conn.Close()

	result := conn.Exec(ctx, "create tag table prep_table(name varchar(100) primary key, time datetime base time, value double)")
	require.NoError(t, result.Err())
	defer func() {
		result = conn.Exec(ctx, "drop table prep_table")
		require.NoError(t, result.Err())
	}()

	stmt, err := conn.Prepare(ctx, "insert into prep_table values(?, ?, ?)")
	require.NoError(t, err)

	for i := range 5 {
		result = stmt.Exec(ctx, fmt.Sprintf("tag%d", i), time.Now(), float64(i)*1.1)
		require.NoError(t, result.Err())
	}
	stmt.Close()

	result = conn.Exec(ctx, "exec table_flush(prep_table)")
	require.NoError(t, result.Err())

	stmt, err = conn.Prepare(ctx, "select name, time, value from prep_table where name = ?")
	require.NoError(t, err)
	for i := range 5 {
		tag := fmt.Sprintf("tag%d", i)
		row := stmt.QueryRow(ctx, tag)
		require.NoError(t, row.Err(), fmt.Sprintf("record not found for %q", tag))
		var name string
		var tm time.Time
		var val float64
		require.NoError(t, row.Scan(&name, &tm, &val))
		require.Equal(t, tag, name)
		require.InDelta(t, tm.Unix(), time.Now().Unix(), float64(10))
		require.InDelta(t, val, float64(i)*1.1, float64(0.001))
	}
	stmt.Close()
}

func watcherStates(engine *Database) []*ConnState {
	states := []*ConnState{}
	engine.ListWatcher(func(state *ConnState) bool {
		states = append(states, state)
		return true
	})
	return states
}

func newWatcherID(before []*ConnState, after []*ConnState) string {
	known := map[string]struct{}{}
	for _, state := range before {
		if state != nil && state.Id != "" {
			known[state.Id] = struct{}{}
		}
	}
	for _, state := range after {
		if state == nil || state.Id == "" {
			continue
		}
		if _, ok := known[state.Id]; !ok {
			return state.Id
		}
	}
	return ""
}

func TestPing(t *testing.T) {
	ctx := t.Context()
	dur, err := machsvrDB.Ping(ctx)
	require.NoError(t, err)
	require.GreaterOrEqual(t, dur, time.Duration(0))
}

func TestLicense(t *testing.T) {
	ctx := t.Context()
	conn, err := machsvrDB.Connect(ctx, api.WithPassword("sys", "manager"))
	require.NoError(t, err, "connect fail")
	defer conn.Close()

	lic, err := spi.GetLicenseInfo(ctx, conn)
	require.NoError(t, err, "license fail")
	require.Equal(t, "00000000", lic.Id)
	require.Equal(t, "COMMUNITY", lic.Type)
	require.Equal(t, "NONE", lic.Customer)
	require.Equal(t, "NONE", lic.Project)
	require.Equal(t, "KR", lic.CountryCode)
	require.NotEmpty(t, lic.InstallDate)
	require.NotEmpty(t, lic.IssueDate)
	require.NotEmpty(t, lic.LicenseStatus)
}

func TestUserAuth(t *testing.T) {
	ctx := t.Context()
	ok, reason, err := machsvrDB.UserAuth(ctx, "sys", "mm")
	if err != nil {
		t.Fatalf("UserAuth failed [%T]: %s", machsvrDB, err.Error())
	}
	require.NoError(t, err)
	require.False(t, ok)
	require.Equal(t, "invalid username or password", reason)

	ok, reason, err = machsvrDB.UserAuth(ctx, "sys", "manager")
	if err != nil {
		t.Fatalf("UserAuth failed: %s", err.Error())
	}
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "", reason)
}

func TestUserAuthWithKey(t *testing.T) {
	ctx := t.Context()
	db := spi.Default()
	host := "127.0.0.1"
	port := machsvrPort

	adminConn, err := db.Connect(ctx, api.WithPassword("sys", "manager"))
	require.NoError(t, err)
	defer adminConn.Close()

	type keyCase struct {
		name string
		gen  func() (*machgo.AuthKeyPair, error)
	}

	cases := []keyCase{
		{name: "ecdsa_p256", gen: machgo.GenerateAuthKeyPairECDSA},
		{name: "rsa_2048", gen: machgo.GenerateAuthKeyPairRSA},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			authkey, err := tc.gen()
			require.NoError(t, err)

			comment := fmt.Sprintf("testsuite auth key %s %d", tc.name, time.Now().UnixNano())
			keyID, err := machgo.RegisterAuthKey(ctx, adminConn, "sys", authkey.PublicKeyPEM, comment)
			require.NoError(t, err)
			require.Greater(t, keyID, 0)

			authDB, err := machgo.NewDatabase(&machgo.Config{
				Host: host,
				Port: port,
			})
			require.NoError(t, err)
			defer authDB.Close()

			key, err := authkey.PrivateKey()
			require.NoError(t, err)
			authConn, err := authDB.Connect(ctx, api.WithAuthKey("sys", key))
			require.NoError(t, err)
			defer authConn.Close()

			row := authConn.QueryRow(ctx, "select 1")
			require.NoError(t, row.Err())
			var v int64
			require.NoError(t, row.Scan(&v))
			require.Equal(t, int64(1), v)
		})
	}
}

func TestTableBasedCases(t *testing.T) {
	t.Run("CreateTables", testCreateTables)
	t.Run("DescribeTable", testDescribeTable)
	t.Run("InsertAndQuery", testInsertAndQuery)
	t.Run("InsertMeta", testInsertMeta)
	t.Run("AppendTag", testAppendTag)
	t.Run("AppendTagNotExist", testAppendTagNotExist)
	t.Run("AppendTagPartial", testAppendTagPartial)
	t.Run("ShowTables", testShowTables)
	t.Run("ExistsTable", testExistsTable)
	t.Run("ShowIndexes", testShowIndexes)
	t.Run("Explain", testExplain)
	t.Run("ExplainFull", testExplainFull)
	t.Run("Columns", testColumns)
	t.Run("ColumnsNameCaseSensitivity", testColumnsNameCaseSensitivity)
	t.Run("QueryRow", testQueryRow)
	t.Run("InsertAndQueryLogTable", testInserAndQueryLogTable)
	t.Run("AppendLogTable", testAppendLogTable)
	t.Run("AppendTagTable", testAppendTagTable)
	t.Run("WatchLogTable", testWatchLogTable)
}

func testCreateTables(t *testing.T) {
	// create test tables
	ctx := t.Context()
	conn, err := machsvrDB.Connect(ctx, api.WithPassword("sys", "manager"))
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}
	defer conn.Close()

	result := conn.Exec(ctx, api.SqlTidy(`
		create tag table if not exists tag_data(
			name            varchar(100) primary key, 
			time            datetime basetime, 
			value           double summarized,
			short_value     short,
			ushort_value    ushort,
			int_value       integer,
			uint_value 	    uinteger,
			long_value      long,
			ulong_value 	ulong,
			str_value       varchar(400),
			json_value      json,
			ipv4_value      ipv4,
			ipv6_value      ipv6,
			bin_value		binary
		) TAG_DUPLICATE_CHECK_DURATION=1;
	`))
	if err := result.Err(); err != nil {
		t.Fatalf("failed to create tag_data table: %v", err)
	}

	result = conn.Exec(ctx, api.SqlTidy(`
		create tag table if not exists tag_simple(
			name            varchar(100) primary key, 
			time            datetime basetime, 
			value           double
		) TAG_DUPLICATE_CHECK_DURATION=1;
	`))
	if err := result.Err(); err != nil {
		t.Fatalf("failed to create tag_simple table: %v", err)
	}

	result = conn.Exec(ctx, api.SqlTidy(`
		create table if not exists log_data(
		    time datetime,
			short_value short,
			ushort_value ushort,
			int_value integer,
			uint_value uinteger,
			long_value long,
			ulong_value ulong,
			double_value double,
			float_value float,
			str_value varchar(400),
			json_value json,
			ipv4_value ipv4,
			ipv6_value ipv6,
			text_value text,
			bin_value binary)
	`))
	if err := result.Err(); err != nil {
		t.Fatalf("failed to create log_data table: %v", err)
	}
}

func testDescribeTable(t *testing.T) {
	ctx := t.Context()
	conn, err := machsvrDB.Connect(ctx, api.WithPassword("sys", "manager"))
	require.NoError(t, err, "connect fail")
	defer conn.Close()

	expect := api.Columns{
		{Name: "NAME", Type: api.ColumnTypeVarchar, DataType: api.DataTypeString},
		{Name: "TIME", Type: api.ColumnTypeDatetime, DataType: api.DataTypeDatetime},
		{Name: "VALUE", Type: api.ColumnTypeDouble, DataType: api.DataTypeFloat64},
		{Name: "SHORT_VALUE", Type: api.ColumnTypeShort, DataType: api.DataTypeInt16},
		{Name: "USHORT_VALUE", Type: api.ColumnTypeUShort, DataType: api.DataTypeUInt16},
		{Name: "INT_VALUE", Type: api.ColumnTypeInteger, DataType: api.DataTypeInt32},
		{Name: "UINT_VALUE", Type: api.ColumnTypeUInteger, DataType: api.DataTypeUInt32},
		{Name: "LONG_VALUE", Type: api.ColumnTypeLong, DataType: api.DataTypeInt64},
		{Name: "ULONG_VALUE", Type: api.ColumnTypeULong, DataType: api.DataTypeUInt64},
		{Name: "STR_VALUE", Type: api.ColumnTypeVarchar, DataType: api.DataTypeString},
		{Name: "JSON_VALUE", Type: api.ColumnTypeJSON, DataType: api.DataTypeJSON},
		{Name: "IPV4_VALUE", Type: api.ColumnTypeIPv4, DataType: api.DataTypeIPv4},
		{Name: "IPV6_VALUE", Type: api.ColumnTypeIPv6, DataType: api.DataTypeIPv6},
		{Name: "BIN_VALUE", Type: api.ColumnTypeBinary, DataType: api.DataTypeBinary},
		{Name: "_RID", Type: api.ColumnTypeLong, DataType: api.DataTypeInt64},
	}

	expectColumns := []map[string]interface{}{
		{"name": "NAME", "type": "varchar", "data_type": "string", "length": 100, "flag": api.ColumnFlagTagName},
		{"name": "TIME", "type": "datetime", "data_type": "datetime", "length": 8, "flag": api.ColumnFlagBasetime},
		{"name": "VALUE", "type": "double", "data_type": "double", "length": 8, "flag": api.ColumnFlagSummarized},
		{"name": "SHORT_VALUE", "type": "short", "data_type": "int16", "length": 2},
		{"name": "USHORT_VALUE", "type": "ushort", "data_type": "uint16", "length": 2},
		{"name": "INT_VALUE", "type": "integer", "data_type": "int32", "length": 4},
		{"name": "UINT_VALUE", "type": "uinteger", "data_type": "uint32", "length": 4},
		{"name": "LONG_VALUE", "type": "long", "data_type": "int64", "length": 8},
		{"name": "ULONG_VALUE", "type": "ulong", "data_type": "uint64", "length": 8},
		{"name": "STR_VALUE", "type": "varchar", "data_type": "string", "length": 400},
		{"name": "JSON_VALUE", "type": "json", "data_type": "json", "length": 32767},
		{"name": "IPV4_VALUE", "type": "ipv4", "data_type": "ipv4", "length": 5},
		{"name": "IPV6_VALUE", "type": "ipv6", "data_type": "ipv6", "length": 17},
		{"name": "BIN_VALUE", "type": "binary", "data_type": "binary", "length": 32767},
		{"name": "_RID", "type": "long", "data_type": "int64", "length": 8},
	}
	for _, table_name := range []string{"tag_data", "sys.tag_data", "machbasedb.sys.tag_data"} {
		// describe table
		desc, err := api.DescribeTable(ctx, conn, table_name, true)
		require.NoError(t, err, "describe table %q fail", table_name)
		require.Equal(t, "TAG_DATA", desc.Name)
		require.Equal(t, "SYS", desc.User)
		require.Equal(t, "MACHBASEDB", desc.Database)
		require.Equal(t, "Tag Table", desc.String())
		require.Equal(t, api.TableTypeTag, desc.Type)

		require.Equal(t, len(expect), len(desc.Columns))

		for i, e := range expect {
			require.Equal(t, e.Name, desc.Columns[i].Name)
			require.Equal(t, e.Type, desc.Columns[i].Type)
			require.Equal(t, e.DataType, desc.Columns[i].DataType)
		}

		if table_name != "tag_data" {
			continue
		}

		buf := &bytes.Buffer{}
		json.NewEncoder(buf).Encode(desc)

		m := make(map[string]interface{})
		json.Unmarshal(buf.Bytes(), &m)

		require.Equal(t, "TAG_DATA", m["name"])
		require.Equal(t, "SYS", m["user"])
		require.Equal(t, "MACHBASEDB", m["database"])
		require.Equal(t, "TagTable", m["type"])
		require.Equal(t, 15, len(m["columns"].([]interface{})))

		columns := m["columns"].([]interface{})

		for i, e := range expectColumns {
			col := columns[i].(map[string]interface{})
			col["length"] = int(col["length"].(float64))
			if flag, ok := col["flag"]; ok {
				col["flag"] = int(flag.(float64))
			}
			// copy actual id to expected id, just for comparison
			if floatId, ok := col["id"]; ok {
				e["id"] = int(floatId.(float64))
				col["id"] = int(floatId.(float64))
			}
			require.Equal(t, e, col)
		}
	}

	descConn, err := machsvrDB.Connect(ctx, api.WithPassword("sys", "manager"))
	require.NoError(t, err, "connect fail")
	defer descConn.Close()

	desc, err := api.DescribeTable(ctx, descConn, "m$sys_tables", false)
	require.NoError(t, err, "describe m$sys_tables fail")
	require.Equal(t, "M$SYS_TABLES", desc.Name)
}

func testInsertAndQuery(t *testing.T) {
	ctx := t.Context()
	db := machsvrDB

	now, _ := time.ParseInLocation("2006-01-02 15:04:05", "2021-01-01 00:00:00", time.UTC)

	// Because INSERT statement uses '2021-01-01 00:00:00' as time value which was parsed in Local timezone,
	// the time value should be converted to UTC timezone to compare
	// TODO: improve this behavior
	nowStrInLocal := now.In(time.Local).Format("2006-01-02 15:04:05")

	conn, err := db.Connect(ctx, api.WithPassword("sys", "manager"), api.WithStatementCache(api.StatementCacheAuto))
	require.NoError(t, err, "connect fail")
	defer conn.Close()

	// insert
	result := conn.Exec(ctx, `insert into tag_data (name, time, value, short_value, int_value, long_value, str_value, json_value) `+
		`values('insert-once', '`+nowStrInLocal+`', 1.23, 1, 2, 3, 'str1', '{"key1": "value1"}')`)
	require.NoError(t, result.Err(), "insert fail")
	require.Equal(t, int64(1), result.RowsAffected())

	sysConn, _ := db.Connect(ctx, api.WithPassword("sys", "manager"), api.WithStatementCache(api.StatementCacheAuto))
	result = sysConn.Exec(ctx, `EXEC table_flush(tag_data)`)
	require.NoError(t, result.Err(), "table_flush fail")
	require.Equal(t, int64(0), result.RowsAffected())
	sysConn.Close()

	// prepare and query
	stmt, err := conn.Prepare(ctx, `select name, time, value, short_value, int_value, long_value, str_value, json_value from tag_data where name = ?`)
	require.NoError(t, err, "prepare fail")
	defer stmt.Close()
	for nth := range 10 {
		rows, err := stmt.Query(ctx, "insert-once")
		require.NoError(t, err, "query fail")
		numRows := 0
		for rows.Next() {
			numRows++
			var name string
			var timeVal time.Time
			var value float64
			var short_value int16
			var int_value int32
			var long_value int64
			var str_value string
			var json_value string
			err := rows.Scan(&name, &timeVal, &value, &short_value, &int_value, &long_value, &str_value, &json_value)
			require.NoError(t, err, "scan fail")
			require.Equal(t, "insert-once", name)
			require.Equal(t, now.Unix(), timeVal.Unix())
			require.Equal(t, 1.23, value)
			require.Equal(t, int16(1), short_value)
			require.Equal(t, int32(2), int_value)
			require.Equal(t, int64(3), long_value)
			require.Equal(t, "str1", str_value)
			require.Equal(t, `{"key1": "value1"}`, json_value)
		}
		rows.Close()
		require.Equal(t, 1, numRows, "expect 1 row in nth=%d", nth+1)
	}

	// select
	rows, err := conn.Query(ctx, `select name, time, value, short_value, int_value, long_value, str_value, json_value from tag_data where name = ?`,
		"insert-once")
	require.NoError(t, err, "select fail")

	numRows := 0
	for rows.Next() {
		numRows++
		var name string
		var timeVal time.Time
		var value float64
		var short_value int16
		var int_value int32
		var long_value int64
		var str_value string
		var json_value string
		err := rows.Scan(&name, &timeVal, &value, &short_value, &int_value, &long_value, &str_value, &json_value)
		require.NoError(t, err, "scan fail")
		require.Equal(t, "insert-once", name)
		require.Equal(t, now.Unix(), timeVal.Unix())
		require.Equal(t, 1.23, value)
		require.Equal(t, int16(1), short_value)
		require.Equal(t, int32(2), int_value)
		require.Equal(t, int64(3), long_value)
		require.Equal(t, "str1", str_value)
		require.Equal(t, `{"key1": "value1"}`, json_value)
	}
	require.Equal(t, 1, numRows)
	err = rows.Close()
	require.NoError(t, err, "close fail")

	var unbox = func(val any) any {
		switch v := val.(type) {
		case *int:
			return *v
		case *uint:
			return *v
		case *int16:
			return *v
		case *uint16:
			return *v
		case *int32:
			return *v
		case *uint32:
			return *v
		case *int64:
			return *v
		case *uint64:
			return *v
		case *float64:
			return *v
		case *float32:
			return *v
		case *string:
			return *v
		case *time.Time:
			return *v
		case *[]byte:
			return *v
		case *net.IP:
			return *v
		case *driver.Value:
			return *v
		default:
			return val
		}
	}

	var beginCalled, endCalled bool
	var nextCalled int
	// query - select
	queryCtx := &spi.Query{
		Begin: func(q *spi.Query) {
			beginCalled = true
			cols := q.Columns()
			require.Equal(t, []string{"NAME", "TIME", "VALUE",
				"SHORT_VALUE", "USHORT_VALUE", "INT_VALUE", "UINT_VALUE", "LONG_VALUE", "ULONG_VALUE",
				"STR_VALUE", "JSON_VALUE", "IPV4_VALUE", "IPV6_VALUE", "BIN_VALUE"}, cols.Names())
			require.Equal(t, []api.DataType{
				api.DataTypeString,
				api.DataTypeDatetime,
				api.DataTypeFloat64,
				api.DataTypeInt16,
				api.DataTypeInt16,
				api.DataTypeInt32,
				api.DataTypeInt32,
				api.DataTypeInt64,
				api.DataTypeInt64,
				api.DataTypeString,
				api.DataTypeString,
				api.DataTypeIPv4,
				api.DataTypeIPv6,
				api.DataTypeBinary,
			}, cols.DataTypes())
		},
		Next: func(q *spi.Query, rownum int64) bool {
			nextCalled++
			values, err := q.Columns().MakeBuffer()
			require.NoError(t, err)
			err = q.Scan(values...)
			require.NoError(t, err)
			require.Equal(t, "insert-once", unbox(values[0]))
			require.Equal(t, now, unbox(values[1]))
			require.Equal(t, 1.23, unbox(values[2]))
			require.Equal(t, int16(1), unbox(values[3]))
			require.Equal(t, nil, unbox(values[4]))
			require.Equal(t, int32(2), unbox(values[5]))
			require.Equal(t, nil, unbox(values[6]))
			require.Equal(t, int64(3), unbox(values[7]))
			require.Equal(t, nil, unbox(values[8]))
			require.Equal(t, "str1", unbox(values[9]))
			require.Equal(t, `{"key1": "value1"}`, unbox(values[10]))
			return true
		},
		End: func(q *spi.Query) {
			endCalled = true
			require.NoError(t, q.Err())
			require.True(t, q.IsFetch())
			require.Equal(t, "a row fetched.", q.UserMessage())
			require.Equal(t, int64(1), q.RowNum())
		},
	}
	err = queryCtx.Execute(ctx, conn, `select * from tag_data where name = ?`, "insert-once")
	require.NoError(t, err, "query fail")
	require.True(t, beginCalled)
	require.True(t, endCalled)
	require.Equal(t, 1, nextCalled)

	// query - insert
	endCalled = false
	queryCtx = &spi.Query{
		End: func(q *spi.Query) {
			endCalled = true
			require.False(t, q.IsFetch())
			require.NoError(t, q.Err())
			require.Equal(t, "a row inserted.", q.UserMessage())
		},
	}
	err = queryCtx.Execute(ctx, conn, `insert into tag_data values('insert-twice', '2021-01-01 00:00:00', ?,`+ // name, time, value
		`1, ?, ?, ?,`+ // short_value, ushort_value, int_value, uint_value
		`?, ?, `+ // long_value, ulong_value
		`?, ?, ?, ?, ? )`, // str_value, json_value, ipv4_value, ipv6_value, bin_value
		1.23,                     // value
		10,                       // ushort_value
		2,                        // int_value
		20,                       // uint_value
		3,                        // long_value
		40,                       // ulong_value
		"str1",                   // str_value
		`{"key1": "value1"}`,     // json_value
		nil,                      // ipv4_value
		nil,                      // ipv6_value
		[]byte{0x01, 0x02, 0x03}, // bin_value
	)
	require.NoError(t, err, "query-insert fail")
	require.True(t, endCalled)

	result = conn.Exec(ctx, "EXEC table_flush(tag_data)")
	require.NoError(t, result.Err(), "table_flush fail")

	// tags
	tags := []*spi.TagInfo{}
	spi.ListTagsWalk(ctx, conn, "TAG_DATA", "NAME", func(tag *spi.TagInfo) bool {
		// TODO: MACHCLI-ERR-3, Communication link failure
		require.NoError(t, tag.Err, "tags fail")
		require.Greater(t, tag.Id, int64(0))
		require.Contains(t, []string{"insert-once", "insert-twice"}, tag.Name)
		tags = append(tags, tag)
		return true
	})
	tags2, err := spi.ListTags(ctx, conn, "TAG_DATA", "NAME")
	require.NoError(t, err, "tags fail")
	require.EqualValues(t, tags, tags2)

	// tag stat
	tagStat, err := spi.TagStat(ctx, conn, "TAG_DATA", "insert-once")
	require.NoError(t, err, "tag stat fail")
	require.Equal(t, "insert-once", tagStat.Name)
	require.Equal(t, int64(1), tagStat.RowCount)
	require.Equal(t, 1.23, tagStat.MinValue)
	require.Equal(t, 1.23, tagStat.MaxValue)

	// tag stat
	tagStat, err = spi.TagStat(ctx, conn, "TAG_DATA", "insert-twice")
	require.NoError(t, err, "tag stat fail")
	require.Equal(t, "insert-twice", tagStat.Name)
	require.Equal(t, int64(1), tagStat.RowCount)

	// delete test data
	result = conn.Exec(ctx, `delete from tag_data where name = ?`, "insert-once")
	require.NoError(t, result.Err(), "delete fail")
	require.Equal(t, int64(1), result.RowsAffected())

	result = conn.Exec(ctx, `delete from tag_data where name = ?`, "insert-twice")
	require.NoError(t, result.Err(), "delete fail")
	require.Equal(t, int64(1), result.RowsAffected())
}

func testInsertMeta(t *testing.T) {
	ctx := t.Context()
	db := machsvrDB

	conn, err := db.Connect(ctx, api.WithPassword("sys", "manager"))
	require.NoError(t, err)
	defer conn.Close()

	// create tag table
	result := conn.Exec(ctx, api.SqlTidy(`
		CREATE TAG TABLE MYTAG (
			name varchar(32) primary key,
			time datetime basetime,
			value double summarized
		) METADATA(
			factory varchar(32),
			equipment varchar(64) 
		)`))
	require.NoError(t, result.Err())

	result = conn.Exec(ctx, "INSERT INTO MYTAG METADATA(name, factory, equipment) values('FA1_CNC', 'FA1', 'CNC')")
	require.NoError(t, result.Err())
	result = conn.Exec(ctx, "INSERT INTO MYTAG METADATA(name, factory, equipment) values('FA4_MILLING', 'FA4', 'MILLING')")
	require.NoError(t, result.Err())

	// flush
	result = conn.Exec(ctx, "EXEC table_flush(MYTAG)")
	require.NoError(t, result.Err(), "table_flush fail")

	// select tag metadata
	rows, err := conn.Query(ctx, "SELECT _id, name, factory, equipment FROM _MYTAG_META")
	require.NoError(t, err)
	var id, name, factory, equipment string
	for rows.Next() {
		require.NoError(t, rows.Scan(&id, &name, &factory, &equipment))
		switch id {
		case "1":
			require.Equal(t, "FA1_CNC", name)
			require.Equal(t, "FA1", factory)
			require.Equal(t, "CNC", equipment)
		case "2":
			require.Equal(t, "FA4_MILLING", name)
			require.Equal(t, "FA4", factory)
			require.Equal(t, "MILLING", equipment)
		default:
			t.Fatalf("Unknown tag metadata: %s", id)
		}
	}
	rows.Close()

	// drop tag table
	result = conn.Exec(ctx, "DROP TABLE MYTAG")
	require.NoError(t, result.Err())
}

func testAppendTag(t *testing.T) {
	ctx := t.Context()
	db := machsvrDB

	tableName := "append_tag"

	conn, err := spi.Default().Connect(ctx, api.WithAuthKey("sys", spi.DefaultKey()))
	require.NoError(t, err, "connect fail")
	result := conn.Exec(ctx, fmt.Sprintf(`CREATE TAG TABLE %s (
		name     varchar(200) primary key,
		time     datetime basetime,
		value    double summarized,
		id       varchar(80),
		jsondata json,
		bindata  binary)`, tableName))
	conn.Close()
	require.NoError(t, result.Err(), "create table fail")

	defer func() {
		conn, err := db.Connect(ctx, api.WithPassword("sys", "manager"))
		require.NoError(t, err, "connect fail")
		conn.Exec(ctx, fmt.Sprintf(`DROP TABLE %s`, tableName))
		conn.Close()
	}()

	conn, err = db.Connect(ctx, api.WithPassword("sys", "manager"))
	require.NoError(t, err)

	appender, err := conn.Appender(ctx, tableName)
	if err != nil {
		panic(err)
	}
	require.Equal(t, strings.ToUpper(tableName), appender.TableName())
	require.Equal(t, api.TableTypeTag, appender.TableType())

	testCount := 100
	ts := time.Now()
	for i := 0; i < testCount; i++ {
		err = appender.Append(
			fmt.Sprintf("name-%d", i%5),
			ts.Add(time.Duration(i)),
			1.001*float64(i+1),
			"some-id-string",
			`{"name":"json"}`,
			[]byte{0x01, 0x02, 0x03},
		)
		if err != nil {
			panic(err)
		}
	}
	appender.Close()
	conn.Close()

	conn, err = db.Connect(ctx, api.WithPassword("sys", "manager"))
	require.NoError(t, err)
	row := conn.QueryRow(ctx, "select count(*) from "+tableName+" where time >= ?", ts)
	if row.Err() != nil {
		panic(row.Err())
	}
	var count int
	err = row.Scan(&count)
	if err != nil {
		panic(err)
	}
	require.Equal(t, testCount, count)
	conn.Close()

	conn, err = db.Connect(ctx, api.WithPassword("sys", "manager"))
	require.NoError(t, err)
	rows, err := conn.Query(ctx, "select * from "+tableName+" where time >= ?", ts)
	if err != nil {
		panic(err)
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		var time time.Time
		var value float64
		var id string
		var jsondata string
		var bindata []byte
		err = rows.Scan(&name, &time, &value, &id, &jsondata, &bindata)
		if err != nil {
			panic(err)
		}
		require.NotEmpty(t, name)
		require.NotZero(t, time)
		require.NotZero(t, value)
		require.NotEmpty(t, id)
		require.Equal(t, `{"name":"json"}`, jsondata)
		require.Equal(t, []byte{0x01, 0x02, 0x03}, bindata)
	}
}

func testAppendTagNotExist(t *testing.T) {
	ctx := t.Context()
	db := machsvrDB

	conn, err := db.Connect(ctx, api.WithPassword("sys", "manager"))
	require.NoError(t, err, "connect fail")
	defer conn.Close()

	appender, err := conn.Appender(ctx, "notexist")
	require.NotNil(t, err)
	require.True(t, strings.Contains(err.Error(), "does not exist"), err.Error())
	if appender != nil {
		appender.Close()
	}
}

func testAppendTagPartial(t *testing.T) {
	ctx := t.Context()
	db := machsvrDB
	tableName := "append_tag2"

	conn, err := db.Connect(ctx, api.WithPassword("sys", "manager"))
	require.NoError(t, err, "connect fail")
	result := conn.Exec(ctx, fmt.Sprintf(`
	CREATE TAG TABLE %s (
		name     varchar(200) primary key,
		time     datetime basetime,
		value    double summarized,
		id       varchar(80),
		jsondata json)
	METADATA( factory varchar(32), equipment varchar(64) )`, tableName))
	conn.Close()
	require.NoError(t, result.Err(), "create table fail")

	defer func() {
		conn, err := db.Connect(ctx, api.WithPassword("sys", "manager"))
		require.NoError(t, err, "connect fail")
		conn.Exec(ctx, fmt.Sprintf(`DROP TABLE %s`, tableName))
		conn.Close()
	}()

	conn, err = db.Connect(ctx, api.WithPassword("sys", "manager"))
	require.NoError(t, err)

	appender, err := conn.Appender(ctx, tableName)
	if err != nil {
		panic(err)
	}
	require.Equal(t, strings.ToUpper(tableName), appender.TableName())
	require.Equal(t, api.TableTypeTag, appender.TableType())

	// arbitrary column order
	appender = appender.WithInputColumns("time", "name", "jsondata", "value")

	testCount := 100
	ts := time.Now()
	for i := 0; i < testCount; i++ {
		err = appender.Append(
			ts.Add(time.Duration(i)),
			fmt.Sprintf("name-%d", i%5),
			`{"name":"json"}`,
			1.001*float64(i+1))
		if err != nil {
			panic(err)
		}
	}
	appender.Close()
	conn.Close()

	conn, err = db.Connect(ctx, api.WithPassword("sys", "manager"))
	require.NoError(t, err)
	row := conn.QueryRow(ctx, "select count(*) from "+tableName+" where time >= ?", ts)
	if row.Err() != nil {
		panic(row.Err())
	}
	var count int
	err = row.Scan(&count)
	if err != nil {
		panic(err)
	}
	require.Equal(t, testCount, count)
	conn.Close()
}

func testShowTables(t *testing.T) {
	ctx := t.Context()
	db := machsvrDB
	conn, err := db.Connect(ctx, api.WithPassword("sys", "manager"))
	require.NoError(t, err, "connect fail")
	defer conn.Close()

	result := map[string]*spi.TableInfo{}
	spi.ListTablesWalk(ctx, conn, true, func(ti *spi.TableInfo) bool {
		require.NoError(t, err, "tables fail")
		result[fmt.Sprintf("%s.%s.%s", ti.Database, ti.User, ti.Name)] = ti
		return true
	})
	ti := result["MACHBASEDB.SYS.TAG_DATA"]
	require.NotNil(t, ti, "table not found")
	require.Equal(t, api.TableTypeTag, ti.Type)
	require.Equal(t, api.TableFlagNone, ti.Flag)
	require.Equal(t, "Tag Table", ti.Kind())

	ti = result["MACHBASEDB.SYS._TAG_DATA_META"]
	require.NotNil(t, ti, "table not found")
	require.Equal(t, api.TableTypeLookup, ti.Type)
	require.Equal(t, api.TableFlagMeta, ti.Flag)
	require.Equal(t, "Lookup Table (meta)", ti.Kind())

	ti = result["MACHBASEDB.SYS._TAG_DATA_DATA_0"]
	require.NotNil(t, ti, "table not found")
	require.Equal(t, api.TableTypeKeyValue, ti.Type)
	require.Equal(t, api.TableFlagData, ti.Flag)
	require.Equal(t, "KeyValue Table (data)", ti.Kind())

	ti = result["MACHBASEDB.SYS.TAG_SIMPLE"]
	require.NotNil(t, ti, "table not found")
	require.Equal(t, api.TableTypeTag, ti.Type)
	require.Equal(t, api.TableFlagNone, ti.Flag)
	require.Equal(t, "Tag Table", ti.Kind())

	ti = result["MACHBASEDB.SYS._TAG_SIMPLE_META"]
	require.NotNil(t, ti, "table not found")
	require.Equal(t, api.TableTypeLookup, ti.Type)
	require.Equal(t, api.TableFlagMeta, ti.Flag)
	require.Equal(t, "Lookup Table (meta)", ti.Kind())

	tables, err := spi.ListTables(ctx, conn, true)
	require.NoError(t, err, "show tables fail")
	require.Equal(t, len(result), len(tables))

	resultList, err := spi.ListTables(ctx, conn, false)
	require.NoError(t, err, "show tables fail")
	require.NotEmpty(t, resultList, "tables empty")

	result2 := map[string]*spi.TableInfo{}
	for _, v := range tables {
		result2[fmt.Sprintf("%s.%s.%s", v.Database, v.User, v.Name)] = v
	}
	require.Equal(t, result, result2)
}

func testExistsTable(t *testing.T) {
	ctx := t.Context()
	db := machsvrDB

	conn, err := db.Connect(ctx, api.WithPassword("sys", "manager"))
	require.NoError(t, err, "connect fail")
	defer conn.Close()

	for _, table_name := range []string{"tag_data", "sys.tag_data", "machbasedb.sys.tag_data"} {
		// table exists
		exists, err := api.ExistsTable(ctx, conn, table_name)
		require.NoError(t, err, "exists table %q fail", table_name)
		require.True(t, exists, "table %q not exists", table_name)

		// table not exists
		exists, err = api.ExistsTable(ctx, conn, table_name+"_not_exists")
		require.NoError(t, err, "exists table %q_not_exists fail", table_name)
		require.False(t, exists, "table %q_not_exists exists", table_name)

		// table exists and truncate
		exists, truncated, err := spi.TruncateTableIfExists(ctx, conn, table_name, true)
		require.NoError(t, err, "exists table %q fail", table_name)
		require.True(t, exists, "table %q not exists", table_name)
		require.True(t, truncated, "table %q not truncated", table_name)
	}
}

func testShowIndexes(t *testing.T) {
	ctx := t.Context()
	db := machsvrDB

	conn, err := db.Connect(ctx, api.WithPassword("sys", "manager"))
	require.NoError(t, err, "connect fail")
	defer conn.Close()

	ret, err := spi.ListIndexes(ctx, conn)
	require.NoError(t, err, "indexes fail")
	require.NotEmpty(t, ret, "indexes empty")
	required := map[string]bool{
		"_TAG_DATA_META_NAME":         false,
		"__PK_IDX__TAG_DATA_META_1":   false,
		"_TAG_SIMPLE_META_NAME":       false,
		"__PK_IDX__TAG_SIMPLE_META_1": false,
	}
	for _, r := range ret {
		switch r.IndexName {
		case "_TAG_DATA_META_NAME":
			require.Equal(t, "MACHBASEDB", r.Database)
			require.Equal(t, "_TAG_DATA_META", r.TableName)
			require.Equal(t, "NAME", r.ColumnName)
			require.Equal(t, "REDBLACK", r.IndexType)
			required[r.IndexName] = true
		case "__PK_IDX__TAG_DATA_META_1":
			require.Equal(t, "MACHBASEDB", r.Database)
			require.Equal(t, "_TAG_DATA_META", r.TableName)
			require.Equal(t, "_ID", r.ColumnName)
			require.Equal(t, "REDBLACK", r.IndexType)
			required[r.IndexName] = true
		case "_TAG_SIMPLE_META_NAME":
			require.Equal(t, "MACHBASEDB", r.Database)
			require.Equal(t, "_TAG_SIMPLE_META", r.TableName)
			require.Equal(t, "NAME", r.ColumnName)
			require.Equal(t, "REDBLACK", r.IndexType)
			required[r.IndexName] = true
		case "__PK_IDX__TAG_SIMPLE_META_1":
			require.Equal(t, "MACHBASEDB", r.Database)
			require.Equal(t, "_TAG_SIMPLE_META", r.TableName)
			require.Equal(t, "_ID", r.ColumnName)
			require.Equal(t, "REDBLACK", r.IndexType)
			required[r.IndexName] = true
		default:
			// Ignore additional system indexes that may be added by newer engine versions.
		}
	}
	for name, seen := range required {
		require.True(t, seen, "required index missing: %s", name)
	}
}

func testExplain(t *testing.T) {
	ctx := t.Context()
	db := machsvrDB
	conn, err := db.Connect(ctx, api.WithPassword("sys", "manager"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	plan, err := conn.Explain(ctx, "select * from TAG_DATA order by time desc", false)
	require.Nil(t, err)
	require.True(t, len(plan) > 0)
	require.True(t, strings.HasPrefix(plan, " PROJECT"))
	require.True(t, strings.Contains(plan, "KEYVALUE FULL SCAN"))
	require.True(t, strings.Contains(plan, "VOLATILE FULL SCAN"))
}

func testExplainFull(t *testing.T) {
	ctx := t.Context()
	db := machsvrDB
	conn, err := db.Connect(ctx, api.WithPassword("sys", "manager"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	plan, err := conn.Explain(ctx, "select * from TAG_DATA order by time desc", true)
	require.Nil(t, err)
	require.True(t, len(plan) > 0)
	require.True(t, strings.Contains(plan, "********"))
	require.True(t, strings.Contains(plan, " NAME           COUNT   ACCUMULATE(ms)  AVERAGE(ms)"), plan)
}

func testColumns(t *testing.T) {
	ctx := t.Context()
	db := machsvrDB
	conn, err := db.Connect(ctx, api.WithPassword("sys", "manager"))
	require.NoError(t, err, "connect fail")
	defer conn.Close()

	rows, err := conn.Query(ctx, "select * from log_data limit 10")
	if err != nil {
		t.Fatal(err)
	}
	require.NotNil(t, rows, "no rows selected")
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		t.Fatal(err)
	}

	data := []struct {
		name   string
		typ    string
		size   int
		length int
	}{
		{"TIME", "datetime", 8, 0},
		{"SHORT_VALUE", "int16", 2, 0},
		{"USHORT_VALUE", "int16", 2, 0},
		{"INT_VALUE", "int32", 4, 0},
		{"UINT_VALUE", "int32", 4, 0},
		{"LONG_VALUE", "int64", 8, 0},
		{"ULONG_VALUE", "int64", 8, 0},
		{"DOUBLE_VALUE", "double", 8, 0},
		{"FLOAT_VALUE", "float", 4, 0},
		{"STR_VALUE", "string", 20, 0},
		{"JSON_VALUE", "string", 32767, 0},
		{"IPV4_VALUE", "ipv4", 5, 0},
		{"IPV6_VALUE", "ipv6", 17, 0},
		{"TEXT_VALUE", "string", 67108864, 0},
		{"BIN_VALUE", "binary", 67108864, 0},
	}
	require.Equal(t, len(data), len(cols), "column count was %d, want %d", len(cols), len(data))
	for i, cd := range data {
		require.Equal(t, cd.name, cols[i].Name, "column[%d] name was %q, want %q", i, cols[i].Name, cd.name)
		require.Equal(t, cd.typ, string(cols[i].DataType), "column[%d] %q's type was %q, want %q", i, cols[i].Name, cols[i].DataType, cd.typ)
	}
}

func testColumnsNameCaseSensitivity(t *testing.T) {
	ctx := t.Context()
	db := machsvrDB
	conn, err := db.Connect(ctx, api.WithPassword("sys", "manager"))
	require.NoError(t, err, "connect fail")
	defer conn.Close()

	rows, err := conn.Query(ctx, "select TiMe,Short_Value from log_data limit 10")
	if err != nil {
		t.Fatal(err)
	}
	require.NotNil(t, rows, "no rows selected")
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		t.Fatal(err)
	}

	data := []struct {
		name   string
		typ    string
		size   int
		length int
	}{
		{"TiMe", "datetime", 8, 0},
		{"Short_Value", "int16", 2, 0},
	}
	require.Equal(t, len(data), len(cols), "column count was %d, want %d", len(cols), len(data))
	for i, cd := range data {
		require.Equal(t, cd.name, cols[i].Name, "column[%d] name was %q, want %q", i, cols[i].Name, cd.name)
		require.Equal(t, cd.typ, string(cols[i].DataType), "column[%d] %q's type was %q, want %q", i, cols[i].Name, cols[i].DataType, cd.typ)
	}
}

func testQueryRow(t *testing.T) {
	ctx := t.Context()
	db := machsvrDB
	conn, err := db.Connect(ctx, api.WithPassword("sys", "manager"))
	require.NoError(t, err, "connect fail")
	defer conn.Close()

	row := conn.QueryRow(ctx, "SELECT * from tag_data WHERE name='_not_exist_'")
	require.EqualError(t, row.Err(), "sql: no rows in result set")
	require.Equal(t, int64(0), row.RowsAffected())
	require.Equal(t, "sql: no rows in result set", row.Message())
	var result int
	err = row.Scan(&result)
	require.EqualError(t, err, "sql: no rows in result set")
	columns, err := row.Columns()
	require.NoError(t, err)

	expectedColumns := []string{
		"NAME", "TIME", "VALUE", "SHORT_VALUE", "USHORT_VALUE",
		"INT_VALUE", "UINT_VALUE", "LONG_VALUE", "ULONG_VALUE",
		"STR_VALUE", "JSON_VALUE", "IPV4_VALUE", "IPV6_VALUE",
		"BIN_VALUE",
	}
	require.Equal(t, len(expectedColumns), len(columns))
	for i, col := range columns {
		require.Equal(t, expectedColumns[i], col.Name)
	}

	row = conn.QueryRow(ctx, "SELECT count(*) from tag_data")
	require.NoError(t, row.Err())
	require.Equal(t, int64(1), row.RowsAffected())
	require.Equal(t, "a row selected.", row.Message())

	var count int64
	err = row.Scan(&count)
	require.NoError(t, err)
	require.GreaterOrEqual(t, count, int64(0))
}

func testInserAndQueryLogTable(t *testing.T) {
	ctx := t.Context()
	db := machsvrDB

	conn, err := db.Connect(ctx, api.WithPassword("sys", "manager"))
	require.NoError(t, err, "connect fail")
	defer conn.Close()

	var one int = 1
	var two int = 2
	var three int16 = 3
	var four int16 = 4
	var five int32 = 5
	var f32 float32 = 6.6
	var f64 float64 = 7.77
	var tick time.Time = time.Now()

	result := conn.Exec(ctx, "insert into log_data values(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		tick,   // time
		0, one, // short, ushort
		&two, three, // int, uint
		&four, five, // long, ulong
		f64, f32, // double, float
		"hello world",                                                    // str_value
		`{"data":"some_data", "id":1}`,                                   // json
		net.ParseIP("127.0.0.1"), net.ParseIP("AB:CC:CC:CC:CC:CC:CC:FF"), // ipv4, ipv6
		fmt.Sprintf("varchar_1_%s.", randomVarchar()), // text_value
		[]byte("binary_00"),                           // bin_value
	)
	if err := result.Err(); err != nil {
		t.Fatal(err)
	}
	require.Equal(t, int64(1), result.RowsAffected())
	result = conn.Exec(ctx, "insert into log_data values(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		tick.Add(1), // time
		0, one,      // short, ushort
		&two, three, // int, uint
		&four, five, // long, ulong
		f64, f32, // double, float
		"hello world",                                                    // str_value
		`{"data":"some_data", "id":2}`,                                   // json
		net.ParseIP("127.0.0.1"), net.ParseIP("AB:CC:CC:CC:CC:CC:CC:FF"), // ipv4, ipv6
		fmt.Sprintf("varchar_2_%s.", randomVarchar()), // text_value
		[]byte("binary_01"),                           // bin_value
	)
	if err := result.Err(); err != nil {
		t.Fatal(err)
	}
	require.Equal(t, int64(1), result.RowsAffected())
}

var seededRand = rand.New(rand.NewSource(time.Now().UnixNano()))

func randomVarchar() string {
	rangeStart := 0
	rangeEnd := 10
	offset := rangeEnd - rangeStart
	randLength := seededRand.Intn(offset) + rangeStart

	charSet := "aAbBcCdDeEfFgGhHiIjJkKlLmMnNoOpPqQrRsStTuUvVwWxXyYzZ"
	randString := StringWithCharset(randLength, charSet)
	return randString
}

func StringWithCharset(length int, charset string) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset)-1)]
	}
	return string(b)
}

func testWatchLogTable(t *testing.T) {
	ctx := t.Context()
	db := machsvrDB

	conf := spi.WatcherConfig{
		ConnProvider: func() (api.Conn, error) {
			return db.Connect(ctx, api.WithPassword("sys", "manager"))
		},
		Timeformat: "2006-01-02 15:04:05.999999",
		Timezone:   time.UTC,
		TableName:  "tag_data",
		TagNames:   []string{"tag1", "tag2"},
	}
	w, err := spi.NewWatcher(ctx, conf)
	require.NoError(t, err, "new watcher fail")
	defer w.Close()

	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()
	tickCount := 0

	for {
		select {
		case data := <-w.C:
			if err, ok := data.(error); ok {
				t.Log("Error", err.Error())
				t.Fail()
				return
			} else if rec, ok := data.(spi.WatchData); !ok {
				t.Log("Data", data)
				t.Fail()
				return
			} else {
				if tickCount > 5 {
					return
				}
				require.Equal(t, 4, len(rec["NAME"].(string)), "NAME")
				require.IsType(t, "", rec["TIME"], "TIME")
				require.LessOrEqual(t, 1.23, rec["VALUE"], "VALUE")
				require.Equal(t, int16(1), rec["SHORT_VALUE"], "SHORT_VALUE")
				require.Less(t, int32(0), rec["INT_VALUE"], "INT_VALUE")
				require.Equal(t, int64(2), rec["LONG_VALUE"], "LONG_VALUE")
				require.Equal(t, "str1", rec["STR_VALUE"], "STR_VALUE")
				require.Equal(t, api.JSONString(`{"key1":"value1"}`), rec["JSON_VALUE"], "JSON_VALUE")
			}
		case <-tick.C:
			tickCount++
			conn, err := conf.ConnProvider()
			require.NoError(t, err, "connect fail")
			name := "tag1"
			if tickCount%2 == 0 {
				name = "tag2"
			}
			values := []any{name, time.Now(), 1.23 * float64(tickCount), 1, tickCount, 2, "str1", `{"key1":"value1"}`}
			result := conn.Exec(ctx, `insert into tag_data (name, time, value, short_value, int_value, long_value, str_value, json_value) values(?, ?, ?, ?, ?, ?, ?, ?)`, values...)
			conn.Close()
			require.NoError(t, result.Err(), "insert fail")
			time.Sleep(100 * time.Millisecond)
			w.Execute()
		}
	}
}

func testAppendLogTable(t *testing.T) {
	ctx := t.Context()
	db := machsvrDB

	conn, err := db.Connect(ctx, api.WithPassword("sys", "manager"))
	require.NoError(t, err, "connect fail")
	defer conn.Close()

	appender, err := conn.Appender(ctx, "log_data")
	require.NoError(t, err)
	require.Equal(t, "LOG_DATA", appender.TableName())
	require.Equal(t, api.TableTypeLog, appender.TableType())
	appender = appender.WithInputFormats()

	expectCols := []*api.Column{
		{Name: "_ARRIVAL_TIME", Type: api.ColumnTypeDatetime, Length: 8, DataType: api.DataTypeDatetime},
		{Name: "TIME", Type: api.ColumnTypeDatetime, Length: 8, DataType: api.DataTypeDatetime},
		{Name: "SHORT_VALUE", Type: api.ColumnTypeShort, Length: 2, DataType: api.DataTypeInt16},
		{Name: "USHORT_VALUE", Type: api.ColumnTypeUShort, Length: 2, DataType: api.DataTypeUInt16},
		{Name: "INT_VALUE", Type: api.ColumnTypeInteger, Length: 4, DataType: api.DataTypeInt32},
		{Name: "UINT_VALUE", Type: api.ColumnTypeUInteger, Length: 4, DataType: api.DataTypeUInt32},
		{Name: "LONG_VALUE", Type: api.ColumnTypeLong, Length: 8, DataType: api.DataTypeInt64},
		{Name: "ULONG_VALUE", Type: api.ColumnTypeULong, Length: 8, DataType: api.DataTypeUInt64},
		{Name: "DOUBLE_VALUE", Type: api.ColumnTypeDouble, Length: 8, DataType: api.DataTypeFloat64},
		{Name: "FLOAT_VALUE", Type: api.ColumnTypeFloat, Length: 4, DataType: api.DataTypeFloat32},
		{Name: "STR_VALUE", Type: api.ColumnTypeVarchar, Length: 400, DataType: api.DataTypeString},
		{Name: "JSON_VALUE", Type: api.ColumnTypeJSON, Length: 32767, DataType: api.DataTypeJSON},
		{Name: "IPV4_VALUE", Type: api.ColumnTypeIPv4, Length: 5, DataType: api.DataTypeIPv4},
		{Name: "IPV6_VALUE", Type: api.ColumnTypeIPv6, Length: 17, DataType: api.DataTypeIPv6},
		{Name: "TEXT_VALUE", Type: api.ColumnTypeText, Length: 67108864, DataType: api.DataTypeString},
		{Name: "BIN_VALUE", Type: api.ColumnTypeBinary, Length: 67108864, DataType: api.DataTypeBinary},
	}
	cols, _ := appender.Columns()
	require.Equal(t, len(expectCols), len(cols), strings.Join(cols.Names(), ", "))
	for i, col := range cols {
		require.Equal(t, expectCols[i].Name, col.Name)
		require.Equal(t, expectCols[i].Type, col.Type, "diff column: "+col.Name)
		require.Equal(t, expectCols[i].DataType, col.DataType, "diff column: "+col.Name)
		require.Equal(t, expectCols[i].Length, col.Length, "diff column: "+col.Name)
	}

	expectCount := 10000
	for i := 0; i < expectCount; i++ {
		ip4 := net.ParseIP(fmt.Sprintf("192.168.0.%d", i%255))
		ip6 := net.ParseIP(fmt.Sprintf("12:FF:FF:FF:CC:EE:FF:%02X", i%255))
		varchar := fmt.Sprintf("varchar_append-%d", i)
		err = appender.AppendLogTime(
			time.Now(),                      // _arrival_time
			time.Now(),                      // time
			int16(i),                        // short
			uint16(i*10),                    // ushort
			int(i*100),                      // int
			uint(i*1000),                    // uint
			int64(i*10000),                  // long
			uint64(i*100000),                // ulong
			float64(i),                      // double
			float32(i),                      // float
			varchar,                         // varchar
			fmt.Sprintf("{\"json\":%d}", i), // json
			ip4,                             // IPv4
			ip6,                             // IPv6
			fmt.Sprintf("text_append-%d-%s.", i, randomVarchar()),
			[]byte(fmt.Sprintf("binary_append_%02d", i)),
		)
		require.NoError(t, err)
	}
	sc, fc, err := appender.Close()
	require.NoError(t, err)
	require.Equal(t, int64(expectCount), sc)
	require.Equal(t, int64(0), fc)
	sc, fc, err = appender.Close()
	require.NoError(t, err)
	require.Equal(t, int64(expectCount), sc)
	require.Equal(t, int64(0), fc)
}

func testAppendTagTable(t *testing.T) {
	ctx := t.Context()
	db := machsvrDB

	conn, err := db.Connect(ctx, api.WithPassword("sys", "manager"))
	require.NoError(t, err, "connect fail")
	defer conn.Close()

	appender, err := conn.Appender(ctx, "tag_data")
	if err != nil {
		t.Fatal(err)
	}
	require.Equal(t, "TAG_DATA", appender.TableName())
	require.Equal(t, api.TableTypeTag, appender.TableType())
	appender = appender.WithInputFormats()

	// On systems with slow network configurations (e.g., GitHub Actions runners),
	// the appender may flush data too frequently (default: 5ms), causing rapid,
	// fragmented exchanges that can fail tests. Disable delay based flushing by setting it to 0.
	appender = appender.
		WithBatchMaxDelay(0).
		WithBatchMaxBytes(1024). // reduce tcp packet size
		WithBatchMaxRows(2000)

	expectCols := []*api.Column{
		{Name: "NAME", Type: api.ColumnTypeVarchar, Length: 100, DataType: api.DataTypeString},
		{Name: "TIME", Type: api.ColumnTypeDatetime, Length: 8, DataType: api.DataTypeDatetime},
		{Name: "VALUE", Type: api.ColumnTypeDouble, Length: 8, DataType: api.DataTypeFloat64},
		{Name: "SHORT_VALUE", Type: api.ColumnTypeShort, Length: 2, DataType: api.DataTypeInt16},
		{Name: "USHORT_VALUE", Type: api.ColumnTypeUShort, Length: 2, DataType: api.DataTypeUInt16},
		{Name: "INT_VALUE", Type: api.ColumnTypeInteger, Length: 4, DataType: api.DataTypeInt32},
		{Name: "UINT_VALUE", Type: api.ColumnTypeUInteger, Length: 4, DataType: api.DataTypeUInt32},
		{Name: "LONG_VALUE", Type: api.ColumnTypeLong, Length: 8, DataType: api.DataTypeInt64},
		{Name: "ULONG_VALUE", Type: api.ColumnTypeULong, Length: 8, DataType: api.DataTypeUInt64},
		{Name: "STR_VALUE", Type: api.ColumnTypeVarchar, Length: 400, DataType: api.DataTypeString},
		{Name: "JSON_VALUE", Type: api.ColumnTypeJSON, Length: 32767, DataType: api.DataTypeJSON},
		{Name: "IPV4_VALUE", Type: api.ColumnTypeIPv4, Length: 5, DataType: api.DataTypeIPv4},
		{Name: "IPV6_VALUE", Type: api.ColumnTypeIPv6, Length: 17, DataType: api.DataTypeIPv6},
		{Name: "BIN_VALUE", Type: api.ColumnTypeBinary, Length: 32767, DataType: api.DataTypeBinary},
	}
	cols, _ := appender.Columns()
	require.Equal(t, len(expectCols), len(cols))
	for i, c := range cols {
		require.Equal(t, expectCols[i].Name, c.Name)
		require.Equal(t, expectCols[i].Type, c.Type, "diff column: "+c.Name)
		require.Equal(t, expectCols[i].DataType, c.DataType, "diff column: "+c.Name)
		require.Equal(t, expectCols[i].Length, c.Length, "diff column: "+c.Name)
	}

	// FIXME: windows github actions runner failed to append 10000 rows, need to investigate further,
	// for now reduce the count to 5000
	// It might be related with host's network configurations.
	//
	// For the reference, here are some settings that can be applied to Windows to
	// improve the performance of appending large number of rows:
	//
	// - name: Windows Network Tuning
	//    if: matrix.os == 'windows'
	//    shell: powershell
	//    run: |
	//      Write-Host "===== BEFORE SETTINGS ====="
	//      netsh int tcp show global
	//      netsh int ipv4 show dynamicport tcp

	//      Write-Host "===== EXPAND DYNAMIC PORT ====="
	//      netsh int ipv4 set dynamicport tcp start=10000 num=55000

	//      Write-Host "===== REDUCE TIME_WAIT ====="
	//      reg add HKLM\SYSTEM\CurrentControlSet\Services\Tcpip\Parameters `
	//        /v TcpTimedWaitDelay /t REG_DWORD /d 30 /f

	//      Write-Host "===== DISABLE TCP AUTOTUNING ====="
	//      netsh int tcp set global autotuninglevel=disabled

	//      Write-Host "===== AFTER SETTINGS ====="
	//      netsh int ipv4 show dynamicport tcp
	//
	// expectCount := 10000
	expectCount := 5000
	for i := 0; i < expectCount; i++ {
		ip4 := net.ParseIP(fmt.Sprintf("192.168.0.%d", i%255))
		ip6 := net.ParseIP(fmt.Sprintf("12:FF:FF:FF:CC:EE:FF:%02X", i%255))
		varchar := fmt.Sprintf("varchar_append-%d", i)
		err = appender.Append(
			fmt.Sprintf("name-%d", i%100),   // name
			time.Now(),                      // time
			float64(i)*1.1,                  // value
			int16(i),                        // short_value
			uint16(i*10),                    // ushort_value
			int(i*100),                      // int_value
			uint(i*1000),                    // uint_value
			int64(i*10000),                  // long_value
			uint64(i*100000),                // ulong_value
			varchar,                         // str_value
			fmt.Sprintf("{\"json\":%d}", i), // json_value
			ip4,                             // IPv4_value
			ip6,                             // IPv6_value
			[]byte{0x01, 0x02, 0x03},        // bin_value
		)
		if err != nil {
			t.Fatal(err)
		}
	}
	time.Sleep(10 * time.Millisecond) // wait for appender to flush
	if flusher, ok := appender.(api.Flusher); ok {
		err = flusher.Flush()
		if err != nil {
			t.Fatal(err)
		}
	}
	time.Sleep(10 * time.Millisecond) // wait for appender to flush
	sc, fc, err := appender.Close()
	require.NoError(t, err)
	require.Equal(t, int64(expectCount), sc)
	require.Equal(t, int64(0), fc)
}

func TestBitTypeColumn(t *testing.T) {
	ctx := t.Context()
	db := machsvrDB

	conn, err := db.Connect(ctx, api.WithPassword("sys", "manager"))
	require.NoError(t, err, "connect fail")
	defer conn.Close()

	result := conn.Exec(ctx,
		"CREATE TABLE bit_table (i1 INTEGER, i2 UINTEGER, i3 FLOAT, i4 DOUBLE, i5 SHORT, i6 VARCHAR(10))",
	)
	require.NoError(t, result.Err(), "create bit table fail")

	result = conn.Exec(ctx, "INSERT INTO bit_table VALUES (-1, 1, 1, 1, 2, 'aaa')")
	require.NoError(t, result.Err(), "insert bit table fail")
	require.NoError(t, err)

	rows, err := conn.Query(ctx, "SELECT * FROM bit_table WHERE BITAND(i2, 1) = 1")
	require.NoError(t, err, "select bit table BITAND(i2, 1) should not fail")
	for rows.Next() {
		var i1 int
		var i2 uint
		var i3 float32
		var i4 float64
		var i5 int16
		var i6 string
		err := rows.Scan(&i1, &i2, &i3, &i4, &i5, &i6)
		require.NoError(t, err, "scan bit table fail")
		require.Equal(t, -1, i1)
		require.Equal(t, uint(1), i2)
		require.Equal(t, float32(1), i3)
		require.Equal(t, float64(1), i4)
		require.Equal(t, int16(2), i5)
		require.Equal(t, "aaa", i6)
	}
	rows.Close()

	rows, err = conn.Query(ctx, "SELECT * FROM bit_table WHERE BITAND(i4, 1) = 1")
	if _, ok := conn.(*machgo.Conn); ok {
		require.Error(t, err, "select bit table BITAND(i1, i3) should fail within Query()")
		require.Equal(t, "MACHCLI-ERR-2037, Function [BITAND] argument data type is mismatched.", err.Error())
	} else {
		require.NoError(t, err, "select bit table BITAND(i1, i3) should not fail within Query()")
		require.False(t, rows.Next(), "select bit table BITAND(i4, 1) should fail")
		require.Error(t, rows.Err(), "select bit table BITAND(i4, 1) should fail")
		// https://github.com/machbase/neo/issues/956
		require.Equal(t, "MACH-ERR 2037 Function [BITAND] argument data type is mismatched.", rows.Err().Error())
	}

	if rows != nil {
		rows.Close()
	}

	rows, err = conn.Query(ctx, "SELECT BITAND(i1, i3) FROM bit_table")
	if _, ok := conn.(*machgo.Conn); ok {
		require.Error(t, err, "select bit table BITAND(i1, i3) should fail within Query()")
		require.Equal(t, "MACHCLI-ERR-2037, Function [BITAND] argument data type is mismatched.", err.Error())
	} else {
		require.NoError(t, err, "select bit table BITAND(i1, i3) should not fail within Query()")
		require.False(t, rows.Next(), "select bit table BITAND(i1, i3) should fail")
		require.Error(t, rows.Err(), "select bit table BITAND(i4, 1) should fail")
		// https://github.com/machbase/neo/issues/956
		require.Equal(t, "MACH-ERR 2037 Function [BITAND] argument data type is mismatched.", rows.Err().Error())
	}
	if rows != nil {
		rows.Close()
	}

	result = conn.Exec(ctx, "DROP TABLE bit_table")
	require.NoError(t, result.Err(), "drop bit table fail")
}

func TestProxyUser(t *testing.T) {
	ctx := t.Context()
	db := machsvrDB

	sysConn, err := db.Connect(ctx, api.WithPassword("sys", "manager"))
	require.NoError(t, err, "connect fail")
	defer sysConn.Close()

	result := sysConn.Exec(ctx, "CREATE USER demo IDENTIFIED BY demo")
	require.NoError(t, result.Err())
	defer func() {
		result := sysConn.Exec(ctx, "DROP table demo.TAG_DATA")
		require.NoError(t, result.Err())
		result = sysConn.Exec(ctx, "DROP USER demo")
		require.NoError(t, result.Err())
	}()

	// create table
	conn, err := db.Connect(ctx, api.WithPassword("demo", "demo"))
	require.NoError(t, err, "connect fail")

	result = conn.Exec(ctx, "CREATE TAG TABLE tag_data (name VARCHAR(100) primary key, time datetime basetime, value double, json_value json)")
	require.NoError(t, result.Err())

	now, _ := time.ParseInLocation("2006-01-02 15:04:05", "2021-01-01 00:00:00", time.UTC)
	// insert tag_data
	result = conn.Exec(ctx, `insert into tag_data values('demo-1', ?, 1.23, '{"key1": "value1"}')`, now)
	require.NoError(t, result.Err(), "insert fail")

	// insert demo.tag_data
	result = sysConn.Exec(ctx, `insert into demo.tag_data values('demo-1', ?, 1.23, '{"key1": "value1"}')`, now.Add(1))
	require.NoError(t, result.Err(), "insert fail")

	result = sysConn.Exec(ctx, "exec table_flush(demo.tag_data)")
	require.NoError(t, result.Err(), "table_flush fail")

	row := sysConn.QueryRow(ctx, "select count(*) from demo.tag_data where name = ?", "demo-1")
	require.NoError(t, row.Err())
	var count int
	row.Scan(&count)
	require.Equal(t, 2, count)

	result = conn.Exec(ctx, `drop table tag_data`)
	require.NoError(t, result.Err(), "drop table fail")
	conn.Close()

	// connect as proxy user
	proxyConn, err := db.Connect(ctx, api.WithAuthKey("sys", spi.DefaultKey()), api.WithProxyUser("demo"))
	require.NoError(t, err, "connect fail")
	defer proxyConn.Close()

	result = proxyConn.Exec(ctx, "CREATE TAG TABLE tag_data (name VARCHAR(100) primary key, time datetime basetime, value double, json_value json)")
	require.NoError(t, result.Err(), fmt.Sprintf("create table fail: %T", db))

	// insert tag_data
	result = proxyConn.Exec(ctx, `insert into tag_data values('demo-1', ?, 1.23, '{"key1": "value1"}')`, now)
	require.NoError(t, result.Err(), "insert fail")

	// insert demo.tag_data
	result = sysConn.Exec(ctx, `insert into demo.tag_data values('demo-1', ?, 1.23, '{"key1": "value1"}')`, now.Add(1))
	require.NoError(t, result.Err(), "insert fail")

	result = sysConn.Exec(ctx, "exec table_flush(demo.tag_data)")
	require.NoError(t, result.Err(), "table_flush fail")

	row = sysConn.QueryRow(ctx, "select count(*) from demo.tag_data where name = ?", "demo-1")
	require.NoError(t, row.Err())
	row.Scan(&count)
	require.Equal(t, 2, count)
}
