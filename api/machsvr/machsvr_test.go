package machsvr_test

import (
	"context"
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/machbase/neo-client/api"
	"github.com/machbase/neo-server/v8/api/machsvr"
	"github.com/machbase/neo-server/v8/api/testsuite"
	"github.com/stretchr/testify/require"
)

var machsvrDB api.Database

func TestMain(m *testing.M) {
	s := testsuite.NewServer("./testsuite_tmp")
	s.StartServer()
	machsvrDB = s.DatabaseSVR()

	code := m.Run()

	s.StopServer()
	os.Exit(code)
}

func TestAll(t *testing.T) {
	testsuite.CreateTestTables(machsvrDB)
	testsuite.TestAll(t, machsvrDB)
	testsuite.DropTestTables(machsvrDB)

	testsuite.CreateTestTables(machsvrDB)
	testsuite.TestAll(t, machsvrDB,
		tcSetMaxConn,
		tcSetMaxQuery,
		tcDatabaseError,
		tcWatcherRegistry,
		tcKillConnection,
		tcCancelConnection,
	)
	testsuite.DropTestTables(machsvrDB)
}

func tcSetMaxConn(t *testing.T) {
	engine := machsvrDB.(*machsvr.Database)
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

func tcSetMaxQuery(t *testing.T) {
	engine := machsvrDB.(*machsvr.Database)
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

func tcDatabaseError(t *testing.T) {
	engine := machsvrDB.(*machsvr.Database)
	_, err := machsvrDB.Connect(context.Background(), api.WithPassword("sys", "wrong-password"))
	require.Error(t, err)

	lastErr := engine.Error()
	require.Error(t, lastErr)
	require.True(t, strings.Contains(lastErr.Error(), "Invalid username/password") || strings.Contains(lastErr.Error(), "invalid username/password"))
}

func tcWatcherRegistry(t *testing.T) {
	engine := machsvrDB.(*machsvr.Database)
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
	engine.ListWatcher(func(state *machsvr.ConnState) bool {
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

func tcKillConnection(t *testing.T) {
	engine := machsvrDB.(*machsvr.Database)
	require.EqualError(t, engine.KillConnection("missing-watcher", true), "connection 'missing-watcher' not found")

	invalidKey := "invalid-watcher"
	engine.SetWatcher(invalidKey, &machsvr.ConnWatcher{})
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

func tcCancelConnection(t *testing.T) {
	engine := machsvrDB.(*machsvr.Database)

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

	machConn, ok := conn.(*machsvr.Conn)
	require.True(t, ok)

	after = watcherStates(engine)
	watcherID = newWatcherID(before, after)
	require.NotEmpty(t, watcherID)

	require.NoError(t, machConn.Cancel())
	require.EqualError(t, engine.KillConnection(watcherID, false), "connection '"+watcherID+"' not found")
	require.NoError(t, conn.Close())
}

func watcherStates(engine *machsvr.Database) []*machsvr.ConnState {
	states := []*machsvr.ConnState{}
	engine.ListWatcher(func(state *machsvr.ConnState) bool {
		states = append(states, state)
		return true
	})
	return states
}

func newWatcherID(before []*machsvr.ConnState, after []*machsvr.ConnState) string {
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
