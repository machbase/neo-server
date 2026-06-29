package bridge

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"
)

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

func setSingleBridgeForTest(t *testing.T, name string, br Bridge) {
	t.Helper()
	registryLock.Lock()
	prev := registry
	registry = map[string]Bridge{name: br}
	registryLock.Unlock()
	t.Cleanup(func() {
		registryLock.Lock()
		registry = prev
		registryLock.Unlock()
	})
}

func TestServiceTestBridgeBranches(t *testing.T) {
	svc := &Service{}
	ctx := context.Background()

	registryLock.Lock()
	prev := registry
	registry = map[string]Bridge{}
	registryLock.Unlock()
	t.Cleanup(func() {
		registryLock.Lock()
		registry = prev
		registryLock.Unlock()
	})

	missingRsp, err := svc.TestBridge(ctx, &TestBridgeRequest{Name: "missing"})
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
	rsp, err := svc.TestBridge(ctx, &TestBridgeRequest{Name: "sql_good"})
	require.NoError(t, err)
	require.True(t, rsp.Success)
	require.Equal(t, "success", rsp.Reason)

	sqlFailConnect := &sqlBridgeStub{bridgeBaseStub: bridgeBaseStub{name: "sql_fail_connect"}, connErr: fmt.Errorf("connect failed")}
	setSingleBridgeForTest(t, "sql_fail_connect", sqlFailConnect)
	rsp, err = svc.TestBridge(ctx, &TestBridgeRequest{Name: "sql_fail_connect"})
	require.NoError(t, err)
	require.False(t, rsp.Success)
	require.Equal(t, "connect failed", rsp.Reason)

	badConn, err := db.Conn(ctx)
	require.NoError(t, err)
	require.NoError(t, badConn.Close())
	sqlPingFail := &sqlBridgeStub{bridgeBaseStub: bridgeBaseStub{name: "sql_ping_fail"}, conn: badConn}
	setSingleBridgeForTest(t, "sql_ping_fail", sqlPingFail)
	rsp, err = svc.TestBridge(ctx, &TestBridgeRequest{Name: "sql_ping_fail"})
	require.NoError(t, err)
	require.False(t, rsp.Success)
	require.NotEmpty(t, rsp.Reason)

	ct := &connectionTestBridgeStub{bridgeBaseStub: bridgeBaseStub{name: "ct"}, ok: true, reason: "ok"}
	setSingleBridgeForTest(t, "ct", ct)
	rsp, err = svc.TestBridge(ctx, &TestBridgeRequest{Name: "ct"})
	require.NoError(t, err)
	require.True(t, rsp.Success)
	require.Equal(t, "ok", rsp.Reason)

	plain := &bridgeBaseStub{name: "plain"}
	setSingleBridgeForTest(t, "plain", plain)
	rsp, err = svc.TestBridge(ctx, &TestBridgeRequest{Name: "plain"})
	require.NoError(t, err)
	require.False(t, rsp.Success)
	require.Equal(t, "bridge 'plain' does not support testing", rsp.Reason)
}

func TestServiceStatsBridgeBranches(t *testing.T) {
	svc := &Service{}
	ctx := context.Background()

	st := &statsBridgeStub{
		bridgeBaseStub: bridgeBaseStub{name: "stats"},
		s:              BridgeTrafficStats{InMsgs: 1, InBytes: 2, OutMsgs: 3, OutBytes: 4, Inserted: 5, Appended: 6},
	}
	setSingleBridgeForTest(t, "stats", st)
	rsp, err := svc.StatsBridge(ctx, &StatsBridgeRequest{Name: "stats"})
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
	rsp, err = svc.StatsBridge(ctx, &StatsBridgeRequest{Name: "plain"})
	require.NoError(t, err)
	require.False(t, rsp.Success)
	require.Equal(t, "bridge 'plain' does not support stats", rsp.Reason)
}
