package bridge_test

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/machbase/neo-server/v8/mods/bridge"
	"github.com/machbase/neo-server/v8/mods/model"
	"github.com/stretchr/testify/require"
)

type bridgeProviderStub struct {
	defs        map[string]*model.BridgeDefinition
	loadAllErr  error
	loadErr     error
	saveErr     error
	removeErr   error
	lastSaved   *model.BridgeDefinition
	lastRemoved string
}

func newBridgeProviderStub(defs ...*model.BridgeDefinition) *bridgeProviderStub {
	ret := &bridgeProviderStub{defs: map[string]*model.BridgeDefinition{}}
	for _, def := range defs {
		cloned := *def
		ret.defs[def.Name] = &cloned
	}
	return ret
}

func (p *bridgeProviderStub) LoadAllBridges() ([]*model.BridgeDefinition, error) {
	if p.loadAllErr != nil {
		return nil, p.loadAllErr
	}
	ret := make([]*model.BridgeDefinition, 0, len(p.defs))
	for _, def := range p.defs {
		cloned := *def
		ret = append(ret, &cloned)
	}
	return ret, nil
}

func (p *bridgeProviderStub) LoadBridge(name string) (*model.BridgeDefinition, error) {
	if p.loadErr != nil {
		return nil, p.loadErr
	}
	def, ok := p.defs[name]
	if !ok {
		return nil, fmt.Errorf("bridge '%s' not found", name)
	}
	cloned := *def
	return &cloned, nil
}

func (p *bridgeProviderStub) SaveBridge(def *model.BridgeDefinition) error {
	if p.saveErr != nil {
		return p.saveErr
	}
	cloned := *def
	p.lastSaved = &cloned
	p.defs[def.Name] = &cloned
	return nil
}

func (p *bridgeProviderStub) RemoveBridge(name string) error {
	if p.removeErr != nil {
		return p.removeErr
	}
	p.lastRemoved = name
	delete(p.defs, name)
	return nil
}

func sqliteBridgePath(t *testing.T) string {
	t.Helper()
	return "file:" + filepath.Join(t.TempDir(), "bridge.db") + "?cache=shared"
}

func TestServiceStartStop(t *testing.T) {
	bridge.UnregisterAll()
	t.Cleanup(bridge.UnregisterAll)

	provider := newBridgeProviderStub(&model.BridgeDefinition{
		Name: "bridge_start_stop",
		Type: model.BRIDGE_SQLITE,
		Path: sqliteBridgePath(t),
	})
	svc := bridge.NewService(bridge.WithProvider(provider))

	require.NoError(t, svc.Start())
	registered, err := bridge.GetBridge("bridge_start_stop")
	require.NoError(t, err)
	require.Equal(t, "bridge_start_stop", registered.Name())

	svc.Stop()
	_, err = bridge.GetBridge("bridge_start_stop")
	require.EqualError(t, err, "undefined bridge name 'bridge_start_stop'")
}

func TestServiceSqliteLifecycle(t *testing.T) {
	bridge.UnregisterAll()
	t.Cleanup(bridge.UnregisterAll)

	provider := newBridgeProviderStub()
	svc := bridge.NewService(bridge.WithProvider(provider))
	ctx := context.Background()
	name := "bridge_service_sqlite"

	addRsp, err := svc.AddBridge(ctx, &bridge.AddBridgeRequest{
		Name: name,
		Type: "sqlite",
		Path: sqliteBridgePath(t),
	})
	require.NoError(t, err)
	require.True(t, addRsp.Success)
	require.Equal(t, "success", addRsp.Reason)
	require.NotEmpty(t, addRsp.Elapse)
	require.NotNil(t, provider.lastSaved)
	require.Equal(t, name, provider.lastSaved.Name)

	listRsp, err := svc.ListBridge(ctx)
	require.NoError(t, err)
	require.True(t, listRsp.Success)
	require.Equal(t, "success", listRsp.Reason)
	require.Len(t, listRsp.Bridges, 1)
	require.Equal(t, name, listRsp.Bridges[0].Name)
	require.Equal(t, "sqlite", listRsp.Bridges[0].Type)

	getRsp, err := svc.GetBridge(ctx, &bridge.GetBridgeRequest{Name: name})
	require.NoError(t, err)
	require.True(t, getRsp.Success)
	require.Equal(t, name, getRsp.Bridge.Name)

	testRsp, err := svc.TestBridge(ctx, &bridge.TestBridgeRequest{Name: name})
	require.NoError(t, err)
	require.True(t, testRsp.Success)
	require.Equal(t, "success", testRsp.Reason)

	createRsp, err := svc.Exec(ctx, &bridge.ExecRequest{
		Name: name,
		Command: bridge.ExecCommand{
			SqlExec: &bridge.SqlRequest{SqlText: `CREATE TABLE example(id INTEGER PRIMARY KEY, name TEXT)`},
		},
	})
	require.NoError(t, err)
	require.True(t, createRsp.Success)
	require.EqualValues(t, 0, createRsp.Result.SqlExecResult.RowsAffected)
	require.EqualValues(t, 0, createRsp.Result.SqlExecResult.LastInsertedId)

	insertRsp, err := svc.Exec(ctx, &bridge.ExecRequest{
		Name: name,
		Command: bridge.ExecCommand{
			SqlExec: &bridge.SqlRequest{SqlText: `INSERT INTO example(id, name) VALUES(?, ?)`, Params: []any{1, "alpha"}},
		},
	})
	require.NoError(t, err)
	require.True(t, insertRsp.Success)
	require.EqualValues(t, 1, insertRsp.Result.SqlExecResult.RowsAffected)
	require.EqualValues(t, 1, insertRsp.Result.SqlExecResult.LastInsertedId)

	queryRsp, err := svc.Exec(ctx, &bridge.ExecRequest{
		Name: name,
		Command: bridge.ExecCommand{
			SqlQuery: &bridge.SqlRequest{SqlText: `SELECT id, name FROM example ORDER BY id`},
		},
	})
	require.NoError(t, err)
	require.True(t, queryRsp.Success)
	require.Equal(t, "success", queryRsp.Reason)
	require.Len(t, queryRsp.Result.SqlQueryResult.Fields, 2)
	require.NotEmpty(t, queryRsp.Result.SqlQueryResult.Handle)

	fetchRsp, err := svc.SqlQueryResultFetch(ctx, queryRsp.Result.SqlQueryResult)
	require.NoError(t, err)
	require.True(t, fetchRsp.Success)
	require.False(t, fetchRsp.HasNoRows)
	require.Equal(t, []any{int64(1), "alpha"}, fetchRsp.Values)

	fetchEndRsp, err := svc.SqlQueryResultFetch(ctx, queryRsp.Result.SqlQueryResult)
	require.NoError(t, err)
	require.True(t, fetchEndRsp.Success)
	require.True(t, fetchEndRsp.HasNoRows)
	require.Empty(t, fetchEndRsp.Values)

	closeRsp, err := svc.SqlQueryResultClose(ctx, queryRsp.Result.SqlQueryResult)
	require.NoError(t, err)
	require.True(t, closeRsp.Success)
	require.Equal(t, "success", closeRsp.Reason)

	missingHandleRsp, err := svc.SqlQueryResultFetch(ctx, queryRsp.Result.SqlQueryResult)
	require.NoError(t, err)
	require.False(t, missingHandleRsp.Success)
	require.Equal(t, fmt.Sprintf("SqlBridge: handle '%s' not found", queryRsp.Result.SqlQueryResult.Handle), missingHandleRsp.Reason)

	statsRsp, err := svc.StatsBridge(ctx, &bridge.StatsBridgeRequest{Name: name})
	require.NoError(t, err)
	require.False(t, statsRsp.Success)
	require.Equal(t, fmt.Sprintf("bridge '%s' does not support stats", name), statsRsp.Reason)

	delRsp, err := svc.DelBridge(ctx, &bridge.DelBridgeRequest{Name: name})
	require.NoError(t, err)
	require.True(t, delRsp.Success)
	require.Equal(t, name, provider.lastRemoved)

	_, err = bridge.GetBridge(name)
	require.EqualError(t, err, fmt.Sprintf("undefined bridge name '%s'", name))
}

func TestServiceErrorPaths(t *testing.T) {
	bridge.UnregisterAll()
	t.Cleanup(bridge.UnregisterAll)

	t.Run("list and get failures", func(t *testing.T) {
		provider := newBridgeProviderStub()
		provider.loadAllErr = fmt.Errorf("load all failed")
		provider.loadErr = fmt.Errorf("load failed")
		svc := bridge.NewService(bridge.WithProvider(provider))

		listRsp, err := svc.ListBridge(context.Background())
		require.NoError(t, err)
		require.False(t, listRsp.Success)
		require.Equal(t, "load all failed", listRsp.Reason)

		getRsp, err := svc.GetBridge(context.Background(), &bridge.GetBridgeRequest{Name: "missing"})
		require.NoError(t, err)
		require.False(t, getRsp.Success)
		require.Equal(t, "load failed", getRsp.Reason)
	})

	t.Run("add validations and persistence failure", func(t *testing.T) {
		provider := newBridgeProviderStub()
		svc := bridge.NewService(bridge.WithProvider(provider))
		ctx := context.Background()

		tooLongRsp, err := svc.AddBridge(ctx, &bridge.AddBridgeRequest{Name: "01234567890123456789012345678901234567890", Type: "sqlite", Path: sqliteBridgePath(t)})
		require.NoError(t, err)
		require.False(t, tooLongRsp.Success)
		require.Equal(t, "name is too long, should be shorter than 40 characters", tooLongRsp.Reason)

		invalidTypeRsp, err := svc.AddBridge(ctx, &bridge.AddBridgeRequest{Name: "bad_type", Type: "invalid", Path: sqliteBridgePath(t)})
		require.NoError(t, err)
		require.False(t, invalidTypeRsp.Success)
		require.Equal(t, "unsupported bridge type: invalid", invalidTypeRsp.Reason)

		emptyPathRsp, err := svc.AddBridge(ctx, &bridge.AddBridgeRequest{Name: "empty_path", Type: "sqlite"})
		require.NoError(t, err)
		require.False(t, emptyPathRsp.Success)
		require.Equal(t, "path is empty, it should be specified", emptyPathRsp.Reason)

		provider.saveErr = fmt.Errorf("save failed")
		saveFailRsp, err := svc.AddBridge(ctx, &bridge.AddBridgeRequest{Name: "save_fail", Type: "sqlite", Path: sqliteBridgePath(t)})
		require.NoError(t, err)
		require.False(t, saveFailRsp.Success)
		require.Equal(t, "save failed", saveFailRsp.Reason)

		bridge.Unregister("save_fail")
	})

	t.Run("exec fetch close and test missing bridge", func(t *testing.T) {
		svc := bridge.NewService(bridge.WithProvider(newBridgeProviderStub()))
		ctx := context.Background()

		execRsp, err := svc.Exec(ctx, &bridge.ExecRequest{Name: "missing"})
		require.NoError(t, err)
		require.False(t, execRsp.Success)
		require.Equal(t, "undefined bridge name 'missing'", execRsp.Reason)

		fetchRsp, err := svc.SqlQueryResultFetch(ctx, &bridge.SqlQueryResult{Handle: "missing"})
		require.NoError(t, err)
		require.False(t, fetchRsp.Success)
		require.Equal(t, "SqlBridge: handle 'missing' not found", fetchRsp.Reason)

		closeRsp, err := svc.SqlQueryResultClose(ctx, &bridge.SqlQueryResult{Handle: "missing"})
		require.NoError(t, err)
		require.False(t, closeRsp.Success)
		require.Equal(t, "handle 'missing' not found", closeRsp.Reason)

		testRsp, err := svc.TestBridge(ctx, &bridge.TestBridgeRequest{Name: "missing"})
		require.NoError(t, err)
		require.False(t, testRsp.Success)
		require.Equal(t, "undefined bridge name 'missing'", testRsp.Reason)
	})

	t.Run("remove failure", func(t *testing.T) {
		provider := newBridgeProviderStub()
		provider.removeErr = fmt.Errorf("remove failed")
		svc := bridge.NewService(bridge.WithProvider(provider))

		delRsp, err := svc.DelBridge(context.Background(), &bridge.DelBridgeRequest{Name: "missing"})
		require.NoError(t, err)
		require.False(t, delRsp.Success)
		require.Equal(t, "remove failed", delRsp.Reason)
	})
}
