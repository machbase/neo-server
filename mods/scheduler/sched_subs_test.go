package scheduler

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/machbase/neo-server/v8/mods/logging"
	"github.com/machbase/neo-server/v8/mods/tql"
	"github.com/machbase/neo-server/v8/mods/util"
	"github.com/machbase/neo-server/v8/mods/util/ssfs"
	"github.com/machbase/neo-server/v8/spi"
	"github.com/machbase/neo-server/v8/spi/machsvr"
	"github.com/robfig/cron/v3"
	"github.com/stretchr/testify/require"
)

var subsScopeTestServer *machsvr.TestServer

func TestMain(m *testing.M) {
	subsScopeTestServer = &machsvr.TestServer{}
	subsScopeTestServer.StartServer("./testsuite_tmp")
	tql.Init()
	code := m.Run()
	tql.Deinit()
	subsScopeTestServer.StopServer()
	os.Exit(code)
}

// TestSubscriberEntryDoInsertUsesExecUserScope guards doInsert (machbase/neo#1468):
// a subscriber must write using the connection scoped to its creator (ExecUser),
// not a hardcoded "sys" connection.
func TestSubscriberEntryDoInsertUsesExecUserScope(t *testing.T) {
	ctx := t.Context()
	username := fmt.Sprintf("sched_scope_ins_%d", time.Now().UnixNano())
	table := "SUBS_INS_TBL"

	sysConn, err := spi.Connect(ctx, "sys")
	require.NoError(t, err)
	_, err = sysConn.ExecContext(ctx, fmt.Sprintf("CREATE USER %s IDENTIFIED BY 'password'", username))
	require.NoError(t, err)
	sysConn.Close()
	t.Cleanup(func() {
		cleanupConn, connectErr := spi.Connect(context.Background(), "sys")
		if connectErr != nil {
			return
		}
		defer cleanupConn.Close()
		_, _ = cleanupConn.ExecContext(context.Background(), fmt.Sprintf("DROP TABLE %s.%s", username, table))
		_, _ = cleanupConn.ExecContext(context.Background(), "DROP USER "+username)
	})

	ownConn, err := spi.Connect(ctx, username)
	require.NoError(t, err)
	_, err = ownConn.ExecContext(ctx, fmt.Sprintf(
		"CREATE TAG TABLE %s (NAME VARCHAR(40) PRIMARY KEY, TIME DATETIME BASETIME, VALUE DOUBLE SUMMARIZED)", table))
	require.NoError(t, err)
	ownConn.Close()

	wd, err := util.NewWriteDescriptor("db/write/" + username + "." + table)
	require.NoError(t, err)

	ent := &SubscriberEntry{
		BaseEntry:    NewBaseEntry("subs_ins_scope", RUNNING, false),
		ExecUser:     username,
		ctx:          ctx,
		log:          logging.GetLog("subs-ins-scope-test"),
		wd:           wd,
		subscription: schedulerSubscriptionStub{},
	}

	payload := []byte(fmt.Sprintf(
		`{"data":{"columns":["NAME","TIME","VALUE"],"rows":[["temp",%d,1.5]]}}`, time.Now().UnixNano()))
	rsp := &Reason{Reason: "not specified"}
	ent.doInsert(payload, rsp)
	require.True(t, rsp.Success, rsp.Reason)

	require.NotNil(t, ent.conn)
	defer ent.conn.Close()
	var currentUser string
	require.NoError(t, ent.conn.QueryRowContext(ctx, "select current_user()").Scan(&currentUser))
	require.Equal(t, strings.ToUpper(username), currentUser)
}

// TestSubscriberEntryDoAppendUsesExecUserScope guards doAppend (machbase/neo#1468):
// the append DSN must use the proxy "sys as <user>" syntax for a non-sys ExecUser,
// matching spi.Connect's SQL proxy behavior, so the append actually connects (and
// writes) as the subscriber's creator instead of failing or silently using "sys".
func TestSubscriberEntryDoAppendUsesExecUserScope(t *testing.T) {
	ctx := t.Context()
	username := fmt.Sprintf("sched_scope_app_%d", time.Now().UnixNano())
	table := "SUBS_APP_TBL"

	sysConn, err := spi.Connect(ctx, "sys")
	require.NoError(t, err)
	_, err = sysConn.ExecContext(ctx, fmt.Sprintf("CREATE USER %s IDENTIFIED BY 'password'", username))
	require.NoError(t, err)
	sysConn.Close()
	t.Cleanup(func() {
		cleanupConn, connectErr := spi.Connect(context.Background(), "sys")
		if connectErr != nil {
			return
		}
		defer cleanupConn.Close()
		_, _ = cleanupConn.ExecContext(context.Background(), fmt.Sprintf("DROP TABLE %s.%s", username, table))
		_, _ = cleanupConn.ExecContext(context.Background(), "DROP USER "+username)
	})

	ownConn, err := spi.Connect(ctx, username)
	require.NoError(t, err)
	_, err = ownConn.ExecContext(ctx, fmt.Sprintf(
		"CREATE TAG TABLE %s (NAME VARCHAR(40) PRIMARY KEY, TIME DATETIME BASETIME, VALUE DOUBLE SUMMARIZED)", table))
	require.NoError(t, err)
	ownConn.Close()

	wd, err := util.NewWriteDescriptor("db/append/" + username + "." + table)
	require.NoError(t, err)

	ent := &SubscriberEntry{
		BaseEntry:    NewBaseEntry("subs_app_scope", RUNNING, false),
		ExecUser:     username,
		ctx:          ctx,
		log:          logging.GetLog("subs-app-scope-test"),
		wd:           wd,
		subscription: schedulerSubscriptionStub{},
	}

	payload := []byte(fmt.Sprintf(`{"data":{"rows":[["temp",%d,1.5]]}}`, time.Now().UnixNano()))
	rsp := &Reason{Reason: "not specified"}
	ent.doAppend(payload, rsp)
	require.True(t, rsp.Success, rsp.Reason)
	// the appender buffers rows until closed, so flush before verifying the count.
	require.NotNil(t, ent.appender)
	require.NoError(t, ent.appenderClose())

	countConn, err := spi.Connect(ctx, "sys")
	require.NoError(t, err)
	defer countConn.Close()
	var count int
	require.NoError(t, countConn.QueryRowContext(ctx,
		fmt.Sprintf("select count(*) from %s.%s", username, table)).Scan(&count))
	require.Equal(t, 1, count)
}

// withScopedTqlFile writes a single .tql script file to a temp dir, mounts it as
// the ssfs default (restoring the previous default on cleanup), and returns its
// loadable path for use as TaskTql/tqlLoader.Load.
func withScopedTqlFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	fileName := "scope_test.tql"
	require.NoError(t, os.WriteFile(filepath.Join(dir, fileName), []byte(content), 0644))

	fsys, err := ssfs.NewServerSideFileSystem([]string{"/=" + dir})
	require.NoError(t, err)
	prev := ssfs.Default()
	ssfs.SetDefault(fsys)
	t.Cleanup(func() { ssfs.SetDefault(prev) })
	return fileName
}

// TestTimerEntryDoTaskUsesExecUserScope guards TimerEntry.doTask (machbase/neo#1468,
// see plan-model-schedule.md follow-up): the TQL task must execute as ExecUser
// (via task.SetConsole) so that SQL()/bridge lookups inside the script run under
// the schedule creator's scope, not always "sys".
func TestTimerEntryDoTaskUsesExecUserScope(t *testing.T) {
	ctx := t.Context()
	username := fmt.Sprintf("sched_scope_timer_%d", time.Now().UnixNano())
	table := "SCOPE_TIMER_TBL"

	sysConn, err := spi.Connect(ctx, "sys")
	require.NoError(t, err)
	_, err = sysConn.ExecContext(ctx, fmt.Sprintf("CREATE USER %s IDENTIFIED BY 'password'", username))
	require.NoError(t, err)
	sysConn.Close()
	t.Cleanup(func() {
		cleanupConn, connectErr := spi.Connect(context.Background(), "sys")
		if connectErr != nil {
			return
		}
		defer cleanupConn.Close()
		_, _ = cleanupConn.ExecContext(context.Background(), fmt.Sprintf("DROP TABLE %s.%s", username, table))
		_, _ = cleanupConn.ExecContext(context.Background(), "DROP USER "+username)
	})

	ownConn, err := spi.Connect(ctx, username)
	require.NoError(t, err)
	_, err = ownConn.ExecContext(ctx, fmt.Sprintf(
		"CREATE TAG TABLE %s (NAME VARCHAR(40) PRIMARY KEY, TIME DATETIME BASETIME, VALUE DOUBLE SUMMARIZED)", table))
	require.NoError(t, err)
	ownConn.Close()

	scriptPath := withScopedTqlFile(t, fmt.Sprintf(
		"FAKE(json({[1]}))\nSQL(\"insert into %s(NAME,TIME,VALUE) values(current_user(), now, 1)\")\n", table))

	ent := &TimerEntry{
		BaseEntry: NewBaseEntry("timer_scope", RUNNING, false),
		TaskTql:   scriptPath,
		ExecUser:  username,
		s:         &Service{crons: cron.New(), tqlLoader: tql.NewLoader()},
		log:       logging.GetLog("timer-scope-test"),
	}
	ent.doTask()
	require.Equal(t, RUNNING, ent.Status(), "doTask should not fail: %v", ent.Error())

	checkConn, err := spi.Connect(ctx, "sys")
	require.NoError(t, err)
	defer checkConn.Close()
	var name string
	require.NoError(t, checkConn.QueryRowContext(ctx, fmt.Sprintf("select name from %s.%s", username, table)).Scan(&name))
	require.Equal(t, strings.ToUpper(username), name)
}

// TestSubscriberEntryDoTqlUsesExecUserScope guards SubscriberEntry.doTql
// (machbase/neo#1468): a TQL-destination subscriber must execute as its creator
// (ExecUser), not always "sys".
func TestSubscriberEntryDoTqlUsesExecUserScope(t *testing.T) {
	ctx := t.Context()
	username := fmt.Sprintf("sched_scope_subs_tql_%d", time.Now().UnixNano())
	table := "SCOPE_SUBS_TQL_TBL"

	sysConn, err := spi.Connect(ctx, "sys")
	require.NoError(t, err)
	_, err = sysConn.ExecContext(ctx, fmt.Sprintf("CREATE USER %s IDENTIFIED BY 'password'", username))
	require.NoError(t, err)
	sysConn.Close()
	t.Cleanup(func() {
		cleanupConn, connectErr := spi.Connect(context.Background(), "sys")
		if connectErr != nil {
			return
		}
		defer cleanupConn.Close()
		_, _ = cleanupConn.ExecContext(context.Background(), fmt.Sprintf("DROP TABLE %s.%s", username, table))
		_, _ = cleanupConn.ExecContext(context.Background(), "DROP USER "+username)
	})

	ownConn, err := spi.Connect(ctx, username)
	require.NoError(t, err)
	_, err = ownConn.ExecContext(ctx, fmt.Sprintf(
		"CREATE TAG TABLE %s (NAME VARCHAR(40) PRIMARY KEY, TIME DATETIME BASETIME, VALUE DOUBLE SUMMARIZED)", table))
	require.NoError(t, err)
	ownConn.Close()

	scriptPath := withScopedTqlFile(t, fmt.Sprintf(
		"FAKE(json({[1]}))\nSQL(\"insert into %s(NAME,TIME,VALUE) values(current_user(), now, 1)\")\n", table))

	wd, err := util.NewWriteDescriptor(scriptPath)
	require.NoError(t, err)
	require.True(t, wd.IsTqlDestination())

	ent := &SubscriberEntry{
		BaseEntry: NewBaseEntry("subs_tql_scope", RUNNING, false),
		TaskTql:   scriptPath,
		ExecUser:  username,
		ctx:       ctx,
		log:       logging.GetLog("subs-tql-scope-test"),
		wd:        wd,
		s:         &Service{tqlLoader: tql.NewLoader()},
	}

	rsp := &Reason{Reason: "not specified"}
	ent.doTql([]byte("{}"), map[string][]string{}, rsp)
	require.True(t, rsp.Success, rsp.Reason)

	checkConn, err := spi.Connect(ctx, "sys")
	require.NoError(t, err)
	defer checkConn.Close()
	var name string
	require.NoError(t, checkConn.QueryRowContext(ctx, fmt.Sprintf("select name from %s.%s", username, table)).Scan(&name))
	require.Equal(t, strings.ToUpper(username), name)
}

func TestInsertPayload(t *testing.T) {
	tt := []struct {
		name    string
		payload string
		expect  []string
	}{
		{
			name:    "array_of_array",
			payload: `{"data":{"columns":["one","two","three"], "rows":[[1,2,3],[4,5,6],[7,8,9]]}}`,
			expect:  []string{"one", "two", "three"},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			result := extractColumns([]byte(tc.payload))
			require.EqualValues(t, tc.expect, result)
		})
	}
}

// TestSubscriberEntryExecUser guards the user scope used for bridge lookups
// and db connections (doInsert/doAppend): a subscriber must connect as its
// creator (ExecUser), only falling back to "sys" for legacy definitions that
// predate ExecUser.
func TestSubscriberEntryExecUser(t *testing.T) {
	tt := []struct {
		name     string
		execUser string
		expect   string
	}{
		{"with_exec_user", "alice", "alice"},
		{"legacy_empty_exec_user", "", "sys"},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			ent := &SubscriberEntry{ExecUser: tc.execUser}
			require.Equal(t, tc.expect, ent.execUser())
		})
	}
}
