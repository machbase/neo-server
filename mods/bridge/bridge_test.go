package bridge

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"reflect"
	"sync"
	"testing"

	"github.com/machbase/neo-server/v8/mods/model"
	"github.com/stretchr/testify/require"
)

func TestSqlite3(t *testing.T) {
	BRIDGE_NAME := "sqlite"

	define := model.BridgeDefinition{
		Id:   1,
		Type: model.BRIDGE_SQLITE,
		Name: BRIDGE_NAME,
		Path: "file:../../tmp/connector_sqlite3.db?cache=shared",
	}

	RegisterByID(&define)
	defer UnregisterByID(define.Id)

	prev := defProvider
	SetBridgeProvider(&testDefProviderStub{defs: map[string]int64{BRIDGE_NAME: define.Id}})
	t.Cleanup(func() { SetBridgeProvider(prev) })

	ctx := t.Context()
	scope := model.UserScope{User: "sys"}

	br, err := GetBridge(ctx, scope, BRIDGE_NAME)
	require.Nil(t, err)
	require.NotNil(t, br)
	_, ok := br.(SqlBridge)
	require.True(t, ok)
	require.Equal(t, BRIDGE_NAME, br.Name())

	sqlBr := br.(SqlBridge)
	conn, err := sqlBr.Connect(ctx)
	require.Nil(t, err)
	require.NotNil(t, conn)
	defer conn.Close()

	_, err = conn.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS example(id INTEGER NOT NULL PRIMARY KEY, name TEXT, age TEXT, address TEXT, UNIQUE(name))`)
	require.Nil(t, err)

	result := conn.QueryRowContext(ctx, `SELECT count(*) from example`)
	require.NotNil(t, result)
	var count int
	result.Scan(&count)

	_, err = conn.ExecContext(ctx, fmt.Sprintf(`INSERT INTO example VALUES(%d, 'hong-%d', '12', 'address for %d')`, count+1, count+1, count+1))
	require.Nil(t, err)

	rows, err := conn.QueryContext(t.Context(), `SELECT * FROM example`)
	require.Nil(t, err)
	defer rows.Close()
	colTypes, err := rows.ColumnTypes()
	require.Nil(t, err)
	require.Equal(t, "id", colTypes[0].Name())
	require.Equal(t, "name", colTypes[1].Name())
	require.Equal(t, "age", colTypes[2].Name())
	require.Equal(t, "address", colTypes[3].Name())
	require.Equal(t, "INTEGER", colTypes[0].DatabaseTypeName())
	require.Equal(t, "TEXT", colTypes[1].DatabaseTypeName())
	require.Equal(t, "TEXT", colTypes[2].DatabaseTypeName())
	require.Equal(t, "TEXT", colTypes[3].DatabaseTypeName())
	require.Equal(t, reflect.TypeOf(sql.NullInt64{}), colTypes[0].ScanType())
	require.Equal(t, reflect.TypeOf(sql.NullString{}), colTypes[1].ScanType())
	require.Equal(t, reflect.TypeOf(sql.NullString{}), colTypes[2].ScanType())
	require.Equal(t, reflect.TypeOf(sql.NullString{}), colTypes[3].ScanType())
	for rows.Next() {
		var id int
		var name, age, address string
		rows.Scan(&id, &name, &age, &address)
		fmt.Println(id, name, age, address)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	require.Nil(t, err)
}

type bridgeBaseStub struct{ name string }

func (b *bridgeBaseStub) Name() string           { return b.name }
func (b *bridgeBaseStub) String() string         { return fmt.Sprintf("bridge '%s' (stub)", b.name) }
func (b *bridgeBaseStub) BeforeRegister() error  { return nil }
func (b *bridgeBaseStub) AfterUnregister() error { return nil }

type sqlBridgeStub struct {
	bridgeBaseStub
	conn    *sql.Conn
	connErr error
}

func (s *sqlBridgeStub) Type() string                               { return "sqlite" }
func (s *sqlBridgeStub) DB() *sql.DB                                { return nil }
func (s *sqlBridgeStub) Connect(context.Context) (*sql.Conn, error) { return s.conn, s.connErr }
func (s *sqlBridgeStub) NewScanType(string, string) any             { return nil }
func (s *sqlBridgeStub) NormalizeType(v []any) []any                { return v }
func (s *sqlBridgeStub) ParameterMarker(int) string                 { return "?" }
func (s *sqlBridgeStub) SupportLastInsertId() bool                  { return false }

type connectionTestBridgeStub struct {
	bridgeBaseStub
	ok     bool
	reason string
}

func (c *connectionTestBridgeStub) TestConnection() (bool, string) { return c.ok, c.reason }

type statsBridgeStub struct {
	bridgeBaseStub
	s BridgeTrafficStats
}

func (s *statsBridgeStub) StatsSnapshot() BridgeTrafficStats { return s.s }

// testDefProviderStub is a minimal BridgeDefProvider that maps a bridge name
// directly to a registry ID, without any owner/scope enforcement. It is used
// by tests that only care about the runtime registry behavior.
type testDefProviderStub struct {
	mu   sync.Mutex
	defs map[string]int64
}

func (p *testDefProviderStub) LoadBridge(ctx context.Context, scope model.UserScope, name string) (*model.BridgeDefinition, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	id, ok := p.defs[name]
	if !ok {
		return nil, fmt.Errorf("undefined bridge name '%s'", name)
	}
	return &model.BridgeDefinition{Id: id, Name: name}, nil
}

func (p *testDefProviderStub) LoadAllBridgesForBootstrap(ctx context.Context) ([]*model.BridgeDefinition, error) {
	return []*model.BridgeDefinition{}, nil
}
func (p *testDefProviderStub) LoadAllBridges(ctx context.Context, scope model.UserScope) ([]*model.BridgeDefinition, error) {
	return []*model.BridgeDefinition{}, nil
}
func (p *testDefProviderStub) SaveBridge(ctx context.Context, scope model.UserScope, def *model.BridgeDefinition) error {
	return nil
}
func (p *testDefProviderStub) RemoveBridge(ctx context.Context, scope model.UserScope, name string) error {
	return nil
}

var testBridgeIDSeq int64

func nextTestBridgeID() int64 {
	testBridgeIDSeq++
	return testBridgeIDSeq
}

func setSingleBridgeForTest(t *testing.T, name string, br Bridge) {
	t.Helper()
	id := nextTestBridgeID()

	registryLock.Lock()
	prevRegistry := registry
	registry = map[int64]Bridge{id: br}
	registryLock.Unlock()

	prevProvider := defProvider
	SetBridgeProvider(&testDefProviderStub{defs: map[string]int64{name: id}})

	t.Cleanup(func() {
		registryLock.Lock()
		registry = prevRegistry
		registryLock.Unlock()
		SetBridgeProvider(prevProvider)
	})
}

func clearRegistryForTest(t *testing.T) {
	t.Helper()
	registryLock.Lock()
	prevRegistry := registry
	registry = map[int64]Bridge{}
	registryLock.Unlock()

	prevProvider := defProvider
	SetBridgeProvider(&testDefProviderStub{defs: map[string]int64{}})

	t.Cleanup(func() {
		registryLock.Lock()
		registry = prevRegistry
		registryLock.Unlock()
		SetBridgeProvider(prevProvider)
	})
}

func TestServiceTestBridgeBranches(t *testing.T) {
	svc := &Service{}
	ctx := context.Background()
	scope := model.UserScope{User: "sys"}

	clearRegistryForTest(t)

	missingRsp, err := svc.TestBridge(ctx, scope, &TestBridgeRequest{Name: "missing"})
	require.NoError(t, err)
	require.False(t, missingRsp.Success)
	require.Equal(t, "undefined bridge name 'missing'", missingRsp.Reason)

	dir := t.TempDir()
	db, err := sql.Open("sqlite3", "file:"+filepath.Join(dir, "mgmt.db")+"?cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	goodConn, err := db.Conn(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _ = goodConn.Close() })

	sqlGood := &sqlBridgeStub{bridgeBaseStub: bridgeBaseStub{name: "sql_good"}, conn: goodConn}
	setSingleBridgeForTest(t, "sql_good", sqlGood)
	rsp, err := svc.TestBridge(ctx, scope, &TestBridgeRequest{Name: "sql_good"})
	require.NoError(t, err)
	require.True(t, rsp.Success)
	require.Equal(t, "success", rsp.Reason)

	sqlFailConnect := &sqlBridgeStub{bridgeBaseStub: bridgeBaseStub{name: "sql_fail_connect"}, connErr: fmt.Errorf("connect failed")}
	setSingleBridgeForTest(t, "sql_fail_connect", sqlFailConnect)
	rsp, err = svc.TestBridge(ctx, scope, &TestBridgeRequest{Name: "sql_fail_connect"})
	require.NoError(t, err)
	require.False(t, rsp.Success)
	require.Equal(t, "connect failed", rsp.Reason)

	badConn, err := db.Conn(ctx)
	require.NoError(t, err)
	require.NoError(t, badConn.Close())
	sqlPingFail := &sqlBridgeStub{bridgeBaseStub: bridgeBaseStub{name: "sql_ping_fail"}, conn: badConn}
	setSingleBridgeForTest(t, "sql_ping_fail", sqlPingFail)
	rsp, err = svc.TestBridge(ctx, scope, &TestBridgeRequest{Name: "sql_ping_fail"})
	require.NoError(t, err)
	require.False(t, rsp.Success)
	require.NotEmpty(t, rsp.Reason)

	ct := &connectionTestBridgeStub{bridgeBaseStub: bridgeBaseStub{name: "ct"}, ok: true, reason: "ok"}
	setSingleBridgeForTest(t, "ct", ct)
	rsp, err = svc.TestBridge(ctx, scope, &TestBridgeRequest{Name: "ct"})
	require.NoError(t, err)
	require.True(t, rsp.Success)
	require.Equal(t, "ok", rsp.Reason)

	plain := &bridgeBaseStub{name: "plain"}
	setSingleBridgeForTest(t, "plain", plain)
	rsp, err = svc.TestBridge(ctx, scope, &TestBridgeRequest{Name: "plain"})
	require.NoError(t, err)
	require.False(t, rsp.Success)
	require.Equal(t, "bridge 'plain' does not support testing", rsp.Reason)
}

func TestServiceStatsBridgeBranches(t *testing.T) {
	svc := &Service{}
	ctx := context.Background()
	scope := model.UserScope{User: "sys"}

	clearRegistryForTest(t)

	st := &statsBridgeStub{
		bridgeBaseStub: bridgeBaseStub{name: "stats"},
		s:              BridgeTrafficStats{InMsgs: 1, InBytes: 2, OutMsgs: 3, OutBytes: 4, Inserted: 5, Appended: 6},
	}
	setSingleBridgeForTest(t, "stats", st)
	rsp, err := svc.StatsBridge(ctx, scope, &StatsBridgeRequest{Name: "stats"})
	require.NoError(t, err)
	require.True(t, rsp.Success)
	require.Equal(t, uint64(1), rsp.InMsgs)
	require.Equal(t, uint64(2), rsp.InBytes)
	require.Equal(t, uint64(3), rsp.OutMsgs)
	require.Equal(t, uint64(4), rsp.OutBytes)
	require.Equal(t, uint64(5), rsp.Inserted)
	require.Equal(t, uint64(6), rsp.Appended)

	plain := &bridgeBaseStub{name: "plain"}
	setSingleBridgeForTest(t, "plain", plain)
	rsp, err = svc.StatsBridge(ctx, scope, &StatsBridgeRequest{Name: "plain"})
	require.NoError(t, err)
	require.False(t, rsp.Success)
	require.Equal(t, "bridge 'plain' does not support stats", rsp.Reason)
}

// bridgeProviderStub is a fake BridgeProvider (persistence layer) used to
// drive the Service methods end-to-end without a real database.
type bridgeProviderStub struct {
	defs        map[string]*model.BridgeDefinition
	nextId      int64
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
		if cloned.Id == 0 {
			ret.nextId++
			cloned.Id = ret.nextId
		}
		ret.defs[def.Name] = &cloned
	}
	return ret
}

func (p *bridgeProviderStub) LoadAllBridgesForBootstrap(ctx context.Context) ([]*model.BridgeDefinition, error) {
	return p.LoadAllBridges(ctx, model.UserScope{})
}

func (p *bridgeProviderStub) LoadAllBridges(ctx context.Context, scope model.UserScope) ([]*model.BridgeDefinition, error) {
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

func (p *bridgeProviderStub) LoadBridge(ctx context.Context, scope model.UserScope, name string) (*model.BridgeDefinition, error) {
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

func (p *bridgeProviderStub) SaveBridge(ctx context.Context, scope model.UserScope, def *model.BridgeDefinition) error {
	if p.saveErr != nil {
		return p.saveErr
	}
	if def.Id == 0 {
		p.nextId++
		def.Id = p.nextId
	}
	cloned := *def
	p.lastSaved = &cloned
	p.defs[def.Name] = &cloned
	return nil
}

func (p *bridgeProviderStub) RemoveBridge(ctx context.Context, scope model.UserScope, name string) error {
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

func withDefProvider(t *testing.T, p BridgeProvider) {
	t.Helper()
	prev := defProvider
	SetBridgeProvider(p)
	t.Cleanup(func() { SetBridgeProvider(prev) })
}

func TestServiceStartStop(t *testing.T) {
	UnregisterAll()
	sqlitePath := sqliteBridgePath(t) // register TempDir cleanup before UnregisterAll (LIFO fix)
	t.Cleanup(UnregisterAll)

	provider := newBridgeProviderStub(&model.BridgeDefinition{
		Name: "bridge_start_stop",
		Type: model.BRIDGE_SQLITE,
		Path: sqlitePath,
	})
	withDefProvider(t, provider)
	svc := NewService()
	scope := model.UserScope{User: "sys"}

	require.NoError(t, svc.Start())
	registered, err := GetBridge(context.Background(), scope, "bridge_start_stop")
	require.NoError(t, err)
	require.Equal(t, "bridge_start_stop", registered.Name())

	svc.Stop()
	_, err = GetBridge(context.Background(), scope, "bridge_start_stop")
	require.EqualError(t, err, "undefined bridge id '1'")
}

func TestServiceSqliteLifecycle(t *testing.T) {
	UnregisterAll()
	sqlitePath := sqliteBridgePath(t) // register TempDir cleanup before UnregisterAll (LIFO fix)
	t.Cleanup(UnregisterAll)

	provider := newBridgeProviderStub()
	withDefProvider(t, provider)
	svc := NewService()
	ctx := context.Background()
	scope := model.UserScope{User: "sys"}
	name := "bridge_service_sqlite"

	addRsp, err := svc.AddBridge(ctx, scope, &AddBridgeRequest{
		Name: name,
		Type: "sqlite",
		Path: sqlitePath,
	})
	require.NoError(t, err)
	require.True(t, addRsp.Success)
	require.Equal(t, "success", addRsp.Reason)
	require.NotEmpty(t, addRsp.Elapse)
	require.NotNil(t, provider.lastSaved)
	require.Equal(t, name, provider.lastSaved.Name)

	listRsp, err := svc.ListBridge(ctx, scope)
	require.NoError(t, err)
	require.True(t, listRsp.Success)
	require.Equal(t, "success", listRsp.Reason)
	require.Len(t, listRsp.Bridges, 1)
	require.Equal(t, name, listRsp.Bridges[0].Name)
	require.Equal(t, "sqlite", listRsp.Bridges[0].Type)

	getRsp, err := svc.GetBridge(ctx, scope, &GetBridgeRequest{Name: name})
	require.NoError(t, err)
	require.True(t, getRsp.Success)
	require.Equal(t, name, getRsp.Bridge.Name)

	testRsp, err := svc.TestBridge(ctx, scope, &TestBridgeRequest{Name: name})
	require.NoError(t, err)
	require.True(t, testRsp.Success)
	require.Equal(t, "success", testRsp.Reason)

	createRsp, err := svc.Exec(ctx, scope, &ExecRequest{
		Name: name,
		Command: ExecCommand{
			SqlExec: &SqlRequest{SqlText: `CREATE TABLE example(id INTEGER PRIMARY KEY, name TEXT)`},
		},
	})
	require.NoError(t, err)
	require.True(t, createRsp.Success)
	require.EqualValues(t, 0, createRsp.Result.SqlExecResult.RowsAffected)
	require.EqualValues(t, 0, createRsp.Result.SqlExecResult.LastInsertedId)

	insertRsp, err := svc.Exec(ctx, scope, &ExecRequest{
		Name: name,
		Command: ExecCommand{
			SqlExec: &SqlRequest{SqlText: `INSERT INTO example(id, name) VALUES(?, ?)`, Params: []any{1, "alpha"}},
		},
	})
	require.NoError(t, err)
	require.True(t, insertRsp.Success)
	require.EqualValues(t, 1, insertRsp.Result.SqlExecResult.RowsAffected)
	require.EqualValues(t, 1, insertRsp.Result.SqlExecResult.LastInsertedId)

	queryRsp, err := svc.Exec(ctx, scope, &ExecRequest{
		Name: name,
		Command: ExecCommand{
			SqlQuery: &SqlRequest{SqlText: `SELECT id, name FROM example ORDER BY id`},
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

	statsRsp, err := svc.StatsBridge(ctx, scope, &StatsBridgeRequest{Name: name})
	require.NoError(t, err)
	require.False(t, statsRsp.Success)
	require.Equal(t, fmt.Sprintf("bridge '%s' does not support stats", name), statsRsp.Reason)

	delRsp, err := svc.DelBridge(ctx, scope, &DelBridgeRequest{Name: name})
	require.NoError(t, err)
	require.True(t, delRsp.Success)
	require.Equal(t, name, provider.lastRemoved)

	_, err = GetBridge(ctx, scope, name)
	require.Equal(t, fmt.Sprintf("bridge '%s' not found", name), err.Error())
}

func TestServiceErrorPaths(t *testing.T) {
	UnregisterAll()
	t.Cleanup(UnregisterAll)
	scope := model.UserScope{User: "sys"}

	t.Run("list and get failures", func(t *testing.T) {
		provider := newBridgeProviderStub()
		provider.loadAllErr = fmt.Errorf("load all failed")
		provider.loadErr = fmt.Errorf("load failed")
		withDefProvider(t, provider)
		svc := NewService()

		listRsp, err := svc.ListBridge(context.Background(), scope)
		require.NoError(t, err)
		require.False(t, listRsp.Success)
		require.Equal(t, "load all failed", listRsp.Reason)

		getRsp, err := svc.GetBridge(context.Background(), scope, &GetBridgeRequest{Name: "missing"})
		require.NoError(t, err)
		require.False(t, getRsp.Success)
		require.Equal(t, "load failed", getRsp.Reason)
	})

	t.Run("add validations and persistence failure", func(t *testing.T) {
		provider := newBridgeProviderStub()
		withDefProvider(t, provider)
		svc := NewService()
		ctx := context.Background()

		tooLongRsp, err := svc.AddBridge(ctx, scope, &AddBridgeRequest{Name: "01234567890123456789012345678901234567890", Type: "sqlite", Path: sqliteBridgePath(t)})
		require.NoError(t, err)
		require.False(t, tooLongRsp.Success)
		require.Equal(t, "name is too long, should be shorter than 40 characters", tooLongRsp.Reason)

		invalidTypeRsp, err := svc.AddBridge(ctx, scope, &AddBridgeRequest{Name: "bad_type", Type: "invalid", Path: sqliteBridgePath(t)})
		require.NoError(t, err)
		require.False(t, invalidTypeRsp.Success)
		require.Equal(t, "unsupported bridge type: invalid", invalidTypeRsp.Reason)

		emptyPathRsp, err := svc.AddBridge(ctx, scope, &AddBridgeRequest{Name: "empty_path", Type: "sqlite"})
		require.NoError(t, err)
		require.False(t, emptyPathRsp.Success)
		require.Equal(t, "path is empty, it should be specified", emptyPathRsp.Reason)

		provider.saveErr = fmt.Errorf("save failed")
		saveFailRsp, err := svc.AddBridge(ctx, scope, &AddBridgeRequest{Name: "save_fail", Type: "sqlite", Path: sqliteBridgePath(t)})
		require.NoError(t, err)
		require.False(t, saveFailRsp.Success)
		require.Equal(t, "save failed", saveFailRsp.Reason)
	})

	t.Run("exec fetch close and test missing bridge", func(t *testing.T) {
		provider := newBridgeProviderStub()
		withDefProvider(t, provider)
		svc := NewService()
		ctx := context.Background()

		execRsp, err := svc.Exec(ctx, scope, &ExecRequest{Name: "missing"})
		require.NoError(t, err)
		require.False(t, execRsp.Success)
		require.Equal(t, "bridge 'missing' not found", execRsp.Reason)

		fetchRsp, err := svc.SqlQueryResultFetch(ctx, &SqlQueryResult{Handle: "missing"})
		require.NoError(t, err)
		require.False(t, fetchRsp.Success)
		require.Equal(t, "SqlBridge: handle 'missing' not found", fetchRsp.Reason)

		closeRsp, err := svc.SqlQueryResultClose(ctx, &SqlQueryResult{Handle: "missing"})
		require.NoError(t, err)
		require.False(t, closeRsp.Success)
		require.Equal(t, "handle 'missing' not found", closeRsp.Reason)

		testRsp, err := svc.TestBridge(ctx, scope, &TestBridgeRequest{Name: "missing"})
		require.NoError(t, err)
		require.False(t, testRsp.Success)
		require.Equal(t, "bridge 'missing' not found", testRsp.Reason)
	})

	t.Run("remove failure", func(t *testing.T) {
		provider := newBridgeProviderStub(&model.BridgeDefinition{Id: 1, Name: "missing", Type: model.BRIDGE_SQLITE, Path: sqliteBridgePath(t)})
		provider.removeErr = fmt.Errorf("remove failed")
		withDefProvider(t, provider)
		svc := NewService()

		delRsp, err := svc.DelBridge(context.Background(), scope, &DelBridgeRequest{Name: "missing"})
		require.NoError(t, err)
		require.False(t, delRsp.Success)
		require.Equal(t, "remove failed", delRsp.Reason)
	})
}
