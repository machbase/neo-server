package server

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"github.com/gorilla/websocket"
	"github.com/machbase/neo-server/v8/api"
	"github.com/machbase/neo-server/v8/api/mgmt"
	"github.com/machbase/neo-server/v8/mods/eventbus"
	"github.com/machbase/neo-server/v8/mods/logging"
	"github.com/stretchr/testify/require"
)

type mockServer struct {
	api.Database
	svr *httptest.Server
	w   *httptest.ResponseRecorder

	accessToken  string
	refreshToken string

	ctx    *gin.Context
	engine *gin.Engine
}

func (fda *mockServer) UserAuth(ctx context.Context, user string, password string) (bool, string, error) {
	if user == "sys" && password == "manager" {
		return true, "", nil
	}
	return false, "invalid username or password", nil
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

func (fda *mockServer) Connect(ctx context.Context, options ...api.ConnectOption) (api.Conn, error) {
	return &mockConn{}, nil
}

type mockConn struct {
	api.Conn
}

func (fda *mockConn) Close() error { return nil }
func (fda *mockConn) Appender(ctx context.Context, tableName string, opts ...api.AppenderOption) (api.Appender, error) {
	ret := &AppenderMock{}
	ret.AppendFunc = func(values ...any) error { return nil }
	ret.CloseFunc = func() (int64, int64, error) { return 0, 0, nil }
	ret.TableNameFunc = func() string { return tableName }
	ret.ColumnsFunc = func() (api.Columns, error) {
		return api.Columns{
			{Name: "NAME", DataType: api.ColumnTypeVarchar.DataType()},
			{Name: "TIME", DataType: api.ColumnTypeDatetime.DataType()},
			{Name: "VALUE", DataType: api.ColumnTypeDouble.DataType()},
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
		jwtCache:        NewJwtCache(),
		memoryFs:        &MemoryFS{},
		serverInfoFunc: func() (*mgmt.ServerInfoResponse, error) {
			return &mgmt.ServerInfoResponse{
				Success: true,
				Reason:  "success",
				Version: &mgmt.Version{},
				Runtime: &mgmt.Runtime{},
			}, nil
		},
	}
	ctx, engine := gin.CreateTestContext(w)
	engine.POST("/web/api/login", svr.handleLogin)
	engine.GET("/web/api/console/:console_id/data", svr.handleConsoleData)
	engine.Use(svr.handleJwtToken)
	engine.POST("/web/api/tql", svr.handleTqlQuery)
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
	payload := w.Body.Bytes()
	err = json.Unmarshal(payload, &result)
	if err != nil {
		t.Log(string(payload))
		t.Fatal(err)
	}
}

func TestWebConsole(t *testing.T) {
	w := httptest.NewRecorder()
	s, ctx, engine := NewMockServer(w)
	defer s.Shutdown()

	err := s.Login("sys", "manager")
	require.Nil(t, err)

	// Convert http://127.0.0.1 to ws://127.0.0.1
	u := "ws" + strings.TrimPrefix(s.URL(), "http")
	u = u + "/web/api/console/1234/data?token=" + s.AccessToken()
	ws, _, err := websocket.DefaultDialer.Dial(u, nil)
	if err != nil {
		t.Logf("Status: %v", w.Code)
		t.Logf("Body: %v", w.Body.String())
		t.Fatalf("%v", err)
	}
	require.Nil(t, err)
	defer ws.Close()

	// PING
	ping := eventbus.NewPingTime(time.Now())
	ws.WriteJSON(ping)

	evt := eventbus.Event{}
	ws.ReadJSON(&evt)
	require.Equal(t, eventbus.EVT_PING, evt.Type)
	require.Equal(t, ping.Ping.Tick, evt.Ping.Tick)

	// LOG
	topic := "console:sys:1234"
	eventbus.PublishLog(topic, "INFO", "test message")

	evt = eventbus.Event{}
	ws.ReadJSON(&evt)
	require.Equal(t, eventbus.EVT_LOG, evt.Type)
	require.Equal(t, "test message", evt.Log.Message)

	// TQL Log
	expectLines := []string{
		"1 [0]",
		"2 [0.25]",
		"3 [0.5]",
		"4 [0.75]",
		"5 [1]",
	}
	expectCount := len(expectLines)
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		for i := 0; i < expectCount; i++ {
			evt := eventbus.Event{}
			err := ws.ReadJSON(&evt)
			if err != nil {
				t.Log(err.Error())
			}
			require.Nil(t, err, "read websocket failed")
			require.Equal(t, expectLines[i], evt.Log.Message)
		}
		wg.Done()
	}()
	wg.Add(1)
	go func() {
		reader := bytes.NewBufferString(`
			FAKE(linspace(0,1,5))
			SCRIPT({
				ctx := import("context")
				ctx.print(ctx.key(), ctx.value())
				ctx.yieldKey(ctx.key(), ctx.value()...)
			})
			PUSHKEY('test')
			CSV(precision(2))
		`)
		ctx.Request, err = http.NewRequest(http.MethodPost, "/web/api/tql", reader)
		if err != nil {
			t.Log(err.Error())
		}
		require.Nil(t, err)
		ctx.Request.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.AccessToken()))
		ctx.Request.Header.Set("X-Console-Id", "1234 console-log-level=INFO log-level=ERROR")
		engine.HandleContext(ctx)
		require.Equal(t, 200, w.Result().StatusCode)
		require.Equal(t, strings.Join([]string{"1,0.00", "2,0.25", "3,0.50", "4,0.75", "5,1.00", "\n"}, "\n"), w.Body.String())
		wg.Done()
	}()
	wg.Wait()
}

type TestServerMock struct {
	DatabaseMock
}

func (dbmock *TestServerMock) UserAuth(ctx context.Context, user string, password string) (bool, string, error) {
	ok := user == "sys" && password == "manager"
	reason := ""
	if !ok {
		reason = "invalid username or password"
	}
	return ok, reason, nil
}

func TestLoginRoute(t *testing.T) {

	dbMock := &TestServerMock{}

	wsvr, err := NewHttp(dbMock,
		WithHttpDebugMode(true),
	)
	if err != nil {
		t.Fatal(err)
	}
	router := wsvr.Router()

	var b *bytes.Buffer
	var loginReq *LoginReq
	var loginRsp *LoginRsp
	var w *httptest.ResponseRecorder
	var expectStatus int
	var req *http.Request

	// wrong password case - login
	b = &bytes.Buffer{}
	loginReq = &LoginReq{
		LoginName: "sys",
		Password:  "wrong",
	}
	expectStatus = http.StatusNotFound
	if err = json.NewEncoder(b).Encode(loginReq); err != nil {
		t.Fatal(err)
	}

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/web/api/login", b)
	req.Header.Set("Content-type", "application/json")
	router.ServeHTTP(w, req)
	require.Equal(t, expectStatus, w.Code, w.Body.String())

	// success case - login
	b = &bytes.Buffer{}
	loginReq = &LoginReq{
		LoginName: "sys",
		Password:  "manager",
	}
	expectStatus = http.StatusOK
	if err = json.NewEncoder(b).Encode(loginReq); err != nil {
		t.Fatal(err)
	}

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/web/api/login", b)
	req.Header.Set("Content-type", "application/json")
	router.ServeHTTP(w, req)
	require.Equal(t, expectStatus, w.Code, w.Body.String())

	dec := json.NewDecoder(w.Body)
	loginRsp = &LoginRsp{}
	err = dec.Decode(loginRsp)
	require.Nil(t, err, "login response decode")

	// Access Token default expire 5 minutes
	claim := NewClaimEmpty()
	_, err = jwt.ParseWithClaims(loginRsp.AccessToken, claim, func(t *jwt.Token) (interface{}, error) {
		return []byte("__secr3t__"), nil
	})
	require.Nil(t, err, "parse access token")
	require.True(t, claim.VerifyExpiresAt(time.Now().Add(4*time.Minute), true))
	require.False(t, claim.VerifyExpiresAt(time.Now().Add(6*time.Minute), true))

	// Access Token default expire 60 minutes
	claim = NewClaimEmpty()
	_, err = jwt.ParseWithClaims(loginRsp.RefreshToken, claim, func(t *jwt.Token) (interface{}, error) {
		return []byte("__secr3t__"), nil
	})
	require.Nil(t, err, "parse refresh token")
	require.True(t, claim.VerifyExpiresAt(time.Now().Add(59*time.Minute), true))
	require.False(t, claim.VerifyExpiresAt(time.Now().Add(61*time.Minute), true))

	// success case - re-login
	b = &bytes.Buffer{}
	reloginReq := &ReLoginReq{
		RefreshToken: loginRsp.RefreshToken,
	}
	expectStatus = http.StatusOK
	if err = json.NewEncoder(b).Encode(reloginReq); err != nil {
		t.Fatal(err)
	}

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/web/api/relogin", b)
	req.Header.Set("Content-type", "application/json")
	req.Header.Set("Authorization", "Bearer "+loginRsp.AccessToken)
	router.ServeHTTP(w, req)
	require.Equal(t, expectStatus, w.Code, w.Body.String())

	dec = json.NewDecoder(w.Body)
	reRsp := &ReLoginRsp{}
	err = dec.Decode(reRsp)
	require.Nil(t, err, w.Body.String())
	require.True(t, reRsp.Success, w.Body.String())

	// success case - logout
	b = &bytes.Buffer{}
	logoutReq := &LogoutReq{
		RefreshToken: reRsp.RefreshToken,
	}
	expectStatus = http.StatusOK
	if err = json.NewEncoder(b).Encode(logoutReq); err != nil {
		t.Fatal(err)
	}

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/web/api/logout", b)
	req.Header.Set("Content-type", "application/json")
	req.Header.Set("Authorization", "Bearer "+reRsp.AccessToken)
	router.ServeHTTP(w, req)
	require.Equal(t, expectStatus, w.Code, w.Body.String())

	dec = json.NewDecoder(w.Body)
	logoutRsp := &LogoutRsp{}
	err = dec.Decode(logoutRsp)
	require.Nil(t, err, w.Body.String())
	require.True(t, logoutRsp.Success, w.Body.String())

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/web/api/check", b)
	req.Header.Set("Authorization", "Bearer "+reRsp.AccessToken)
	router.ServeHTTP(w, req)
	require.Equal(t, expectStatus, w.Code, w.Body.String())
	dec = json.NewDecoder(w.Body)
	checkRsp := &LoginCheckRsp{}
	err = dec.Decode(checkRsp)
	require.Nil(t, err, w.Body.String())
	require.True(t, checkRsp.Success, w.Body.String())
}

func TestImageFiles(t *testing.T) {
	require.Equal(t, "image/apng", contentTypeOfFile("some/dir/file.apng"))
	require.Equal(t, "image/avif", contentTypeOfFile("some/dir/file.avif"))
	require.Equal(t, "image/gif", contentTypeOfFile("some/dir/file.gif"))
	require.Equal(t, "image/jpeg", contentTypeOfFile("some/dir/file.Jpeg"))
	require.Equal(t, "image/jpeg", contentTypeOfFile("some/dir/file.JPG"))
	require.Equal(t, "image/png", contentTypeOfFile("some/dir/file.PNG"))
	require.Equal(t, "image/svg+xml", contentTypeOfFile("some/dir/file.svg"))
	require.Equal(t, "image/webp", contentTypeOfFile("some/dir/file.webp"))
	require.Equal(t, "image/bmp", contentTypeOfFile("some/dir/file.BMP"))
	require.Equal(t, "image/x-icon", contentTypeOfFile("some/dir/file.ico"))
	require.Equal(t, "image/tiff", contentTypeOfFile("some/dir/file.tiff"))
	require.Equal(t, "text/plain", contentTypeOfFile("some/dir/file.txt"))
	require.Equal(t, "text/csv", contentTypeOfFile("some/dir/file.csv"))
	require.Equal(t, "application/json", contentTypeOfFile("some/dir/file.json"))
	require.Equal(t, "text/markdown", contentTypeOfFile("some/dir/file.md"))
	require.Equal(t, "text/markdown", contentTypeOfFile("some/dir/file.markdown"))
}

func TestRefsFiles(t *testing.T) {
	w := httptest.NewRecorder()
	s, _, engine := NewMockServer(w)
	defer s.Shutdown()

	err := s.Login("sys", "manager")
	require.Nil(t, err)

	req, err := http.NewRequest(http.MethodGet, "/web/api/refs/", nil)
	if err != nil {
		t.Log("ERR", err.Error())
	}
	require.Nil(t, err)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.AccessToken()))
	engine.ServeHTTP(w, req)
	require.Equal(t, 200, w.Result().StatusCode)
	require.Equal(t, "application/json; charset=utf-8", w.Header().Get("Content-Type"))

	var rsp RefsResponse
	err = json.Unmarshal(w.Body.Bytes(), &rsp)
	require.Nil(t, err)

	require.Equal(t, 3, len(rsp.Data.Refs))
	require.Equal(t, rsp.Data.Refs[0].Label, "REFERENCES")
	require.Equal(t, 5, len(rsp.Data.Refs[0].Items))

	require.Equal(t, rsp.Data.Refs[1].Label, "SDK")
	require.Equal(t, 5, len(rsp.Data.Refs[1].Items))

	require.Equal(t, rsp.Data.Refs[2].Label, "CHEAT SHEETS")
	require.Equal(t, 3, len(rsp.Data.Refs[2].Items))

}

func TestMarkdown(t *testing.T) {
	w := httptest.NewRecorder()
	s, ctx, engine := NewMockServer(w)
	defer s.Shutdown()

	err := s.Login("sys", "manager")
	require.Nil(t, err)

	reader := bytes.NewBufferString(`
## markdown test
- file_root {{ file_root }}
- file_path {{ file_path }}
- file_name {{ file_name }}
- file_dir {{ file_dir }}
`)
	expect := []string{
		"<div><h2>markdown test</h2>",
		"<ul>",
		"<li>file_root /web/api/tql</li>",
		"<li>file_path /web/api/tql/sample/file.wrk</li>",
		"<li>file_name file.wrk</li>",
		"<li>file_dir /web/api/tql/sample</li>",
		"</ul>",
		"</div>",
	}
	ctx.Request, _ = http.NewRequest(http.MethodPost, "/web/api/md", reader)
	ctx.Request.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.AccessToken()))
	refer := base64.StdEncoding.EncodeToString([]byte("http://127.0.0.1:5654/web/api/tql/sample/file.wrk"))
	ctx.Request.Header.Set("X-Referer", refer)
	engine.HandleContext(ctx)
	require.Equal(t, 200, w.Result().StatusCode)
	require.Equal(t, "application/xhtml+xml", w.Header().Get("Content-Type"))
	require.Equal(t, strings.Join(expect, "\n"), w.Body.String())
}

func TestMarkdown2(t *testing.T) {
	w := httptest.NewRecorder()
	s, ctx, engine := NewMockServer(w)
	defer s.Shutdown()

	err := s.Login("sys", "manager")
	require.Nil(t, err)

	reader := bytes.NewBufferString(`
## markdown test
- file_root {{ file_root }}
- file_path {{ file_path }}
- file_name {{ file_name }}
- file_dir {{ file_dir }}
`)
	expect := []string{
		"<div><h2>markdown test</h2>",
		"<ul>",
		"<li>file_root /web/api/tql</li>",
		"<li>file_path /web/api/tql/语言/文檔.wrk</li>",
		"<li>file_name 文檔.wrk</li>",
		"<li>file_dir /web/api/tql/语言</li>",
		"</ul>",
		"</div>",
	}

	ctx.Request, _ = http.NewRequest(http.MethodPost, "/web/api/md", reader)
	ctx.Request.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.AccessToken()))
	refer := base64.StdEncoding.EncodeToString([]byte("http://127.0.0.1:5654/web/api/tql/语言/文檔.wrk"))
	ctx.Request.Header.Set("X-Referer", refer)
	engine.HandleContext(ctx)
	require.Equal(t, 200, w.Result().StatusCode)
	require.Equal(t, "application/xhtml+xml", w.Header().Get("Content-Type"))
	require.Equal(t, strings.Join(expect, "\n"), w.Body.String())
}

func TestMarkdownMermaid(t *testing.T) {
	w := httptest.NewRecorder()
	s, ctx, engine := NewMockServer(w)
	defer s.Shutdown()

	err := s.Login("sys", "manager")
	require.Nil(t, err)

	buf, _ := os.ReadFile("test/test_markdown_mermaid.md")
	reader := bytes.NewBuffer(buf)

	buf, _ = os.ReadFile("test/test_markdown_mermaid.txt")
	expect := string(buf)

	ctx.Request, _ = http.NewRequest(http.MethodPost, "/web/api/md", reader)
	ctx.Request.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.AccessToken()))
	refer := base64.StdEncoding.EncodeToString([]byte("http://127.0.0.1:5654/web/api/tql/语言/文檔.wrk"))
	ctx.Request.Header.Set("X-Referer", refer)
	engine.HandleContext(ctx)
	require.Equal(t, 200, w.Result().StatusCode)
	require.Equal(t, "application/xhtml+xml", w.Header().Get("Content-Type"))
	result := w.Body.String()
	if expect != w.Body.String() {
		es := strings.Split(expect, "\n")
		rs := strings.Split(result, "\n")
		i := 0
		r := 0
		diff := 0
		for i < len(es) || r < len(rs) {
			if strings.TrimSpace(es[i]) != strings.TrimSpace(rs[r]) {
				t.Logf("Diff expect[%d] %s", i+1, es[i])
				t.Logf("Diff actual[%d] %s", r+1, rs[r])
				diff++
			}
			i++
			r++
		}
		if diff > 0 || i != r {
			t.Logf("Expect:\n%s<-%d", expect, len(expect))
			t.Logf("Actual:\n%s<-%d", result, len(result))
			t.Fail()
		}
	}
}
