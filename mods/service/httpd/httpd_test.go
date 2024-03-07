package httpd

//go:generate moq -out ./httpd_mock_test.go -pkg httpd ../../../../neo-spi Database DatabaseServer DatabaseClient DatabaseAuth Conn Result Rows Row Appender

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/machbase/neo-server/mods/logging"
	"github.com/machbase/neo-server/mods/service/security"
	spi "github.com/machbase/neo-spi"
	"github.com/stretchr/testify/require"
)

type mockServer struct {
	spi.Database
	svr *httptest.Server
	w   *httptest.ResponseRecorder

	accessToken  string
	refreshToken string

	ctx    *gin.Context
	engine *gin.Engine
}

func (fda *mockServer) UserAuth(user string, password string) (bool, error) {
	if user == "sys" && password == "manager" {
		return true, nil
	}
	return false, nil
}

func (fda *mockServer) Login(user string, password string) error {
	var reader io.Reader = bytes.NewBufferString(
		fmt.Sprintf(`{"loginName":"%s","password":"%s"}`, user, password))
	fda.ctx.Request, _ = http.NewRequest(http.MethodPost, "/web/api/login", reader)
	fda.ctx.Request.Header.Set("Content-Type", "application/json")
	fda.engine.HandleContext(fda.ctx)

	if fda.w.Result().StatusCode != 200 {
		return fmt.Errorf("login failure - %s", fda.w.Body.String())
	}
	loginRsp := &LoginRsp{}
	json.Unmarshal(fda.w.Body.Bytes(), loginRsp)
	fda.w.Body.Reset()

	fda.accessToken = loginRsp.AccessToken
	fda.refreshToken = loginRsp.RefreshToken
	return nil
}

func (fda *mockServer) URL() string {
	return fda.svr.URL
}

func (fda *mockServer) AccessToken() string {
	return fda.accessToken
}

func (fda *mockServer) RefreshToken() string {
	return fda.refreshToken
}

func (fda *mockServer) Connect(ctx context.Context, options ...spi.ConnectOption) (spi.Conn, error) {
	return &mockConn{}, nil
}

type mockConn struct {
	spi.Conn
}

func (fda *mockConn) Close() error { return nil }
func (fda *mockConn) Appender(ctx context.Context, tableName string, opts ...spi.AppenderOption) (spi.Appender, error) {
	ret := &AppenderMock{}
	ret.AppendFunc = func(values ...any) error { return nil }
	ret.CloseFunc = func() (int64, int64, error) { return 0, 0, nil }
	ret.TableNameFunc = func() string { return tableName }
	ret.ColumnsFunc = func() (spi.Columns, error) {
		return []*spi.Column{
			{Name: "TIME", Type: "string"},
			{Name: "TIME", Type: "datetime"},
			{Name: "VALUE", Type: "double"},
		}, nil
	}
	return ret, nil
}

var singleMockServer sync.Mutex

func NewMockServer(w *httptest.ResponseRecorder) (*mockServer, *gin.Context, *gin.Engine) {
	singleMockServer.Lock()
	ret := &mockServer{}
	svr := &httpd{
		log:             logging.GetLog("httpd-fake"),
		db:              ret,
		neoShellAccount: map[string]string{},
		jwtCache:        security.NewJwtCache(),
		memoryFs:        &MemoryFS{},
	}
	ctx, engine := gin.CreateTestContext(w)
	engine.POST("/web/api/login", svr.handleLogin)
	engine.GET("/web/api/console/:console_id/data", svr.handleConsoleData)
	engine.Use(svr.handleJwtToken)
	engine.POST("/web/api/tql", svr.handlePostTagQL)
	engine.POST("/web/api/md", svr.handleMarkdown)
	engine.GET("/web/api/refs/*path", svr.handleRefs)
	engine.GET("/db/query", svr.handleQuery)
	engine.POST("/db/query", svr.handleQuery)
	engine.GET("/db/statz", svr.handleStatz)

	ret.w = w
	ret.ctx = ctx
	ret.engine = engine

	ret.svr = httptest.NewServer(engine)
	return ret, ctx, engine
}

func (fds *mockServer) Shutdown() {
	fds.svr.Close()
	singleMockServer.Unlock()
}

func TestStatz(t *testing.T) {
	w := httptest.NewRecorder()
	s, ctx, engine := NewMockServer(w)
	err := s.Login("sys", "manager")
	require.Nil(t, err)
	defer s.Shutdown()

	ctx.Request, _ = http.NewRequest("GET", "/db/statz", nil)
	ctx.Request.RemoteAddr = "127.0.0.1:123"
	ctx.Request.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.AccessToken()))
	engine.HandleContext(ctx)

	result := map[string]any{}
	err = json.Unmarshal(w.Body.Bytes(), &result)
	if err != nil {
		t.Fatal(err)
	}

	t.Log(result)
}
