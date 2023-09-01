package httpd

//go:generate moq -out ./httpd_mock_test.go -pkg httpd ../../../../neo-spi Database DatabaseServer DatabaseClient DatabaseAuth Result Rows Row Appender

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"

	"github.com/gin-gonic/gin"
	"github.com/machbase/neo-server/mods/logging"
	"github.com/machbase/neo-server/mods/service/security"
	spi "github.com/machbase/neo-spi"
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

func NewMockServer(w *httptest.ResponseRecorder) (*mockServer, *gin.Context, *gin.Engine) {
	ret := &mockServer{}
	svr := &httpd{
		log:             logging.GetLog("httpd-fake"),
		db:              ret,
		neoShellAccount: map[string]string{},
		jwtCache:        security.NewJwtCache(),
	}
	ctx, engine := gin.CreateTestContext(w)
	engine.POST("/web/api/login", svr.handleLogin)
	engine.GET("/web/api/console/:console_id/data", svr.handleConsoleData)
	engine.Use(svr.handleJwtToken)
	engine.POST("/web/api/tql", svr.handlePostTagQL)

	ret.w = w
	ret.ctx = ctx
	ret.engine = engine

	ret.svr = httptest.NewServer(engine)
	return ret, ctx, engine
}

func (fds *mockServer) Shutdown() {
	fds.svr.Close()
}