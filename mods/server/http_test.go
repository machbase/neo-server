package server

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/gorilla/websocket"
	"github.com/machbase/neo-server/v8/mods/eventbus"
	"github.com/stretchr/testify/require"
)

func TestStatz(t *testing.T) {
	at, _, err := jwtLogin("sys", "manager")
	require.NoError(t, err)

	req, _ := http.NewRequest(http.MethodGet, httpServerAddress+"/db/statz", nil)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", at))
	rsp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rsp.StatusCode)

	result := map[string]any{}
	body, err := io.ReadAll(rsp.Body)
	require.NoError(t, err)
	rsp.Body.Close()

	err = json.Unmarshal(body, &result)
	require.NoError(t, err)

	require.GreaterOrEqual(t, len(result), 2)
}

func TestWebConsole(t *testing.T) {
	at, _, err := jwtLogin("sys", "manager")
	require.Nil(t, err)

	// Convert http://127.0.0.1 to ws://127.0.0.1
	u := "ws" + strings.TrimPrefix(httpServerAddress, "http") + "/web/api/console/1234/data?token=" + at
	ws, _, err := websocket.DefaultDialer.Dial(u, nil)
	require.NoError(t, err)
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
		"1 0",
		"2 0.25",
		"3 0.5",
		"4 0.75",
		"5 1",
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
			SCRIPT("js", {
				console.log($.key, $.values[0]);
				$.yieldKey($.key, $.values[0]);
			})
			PUSHKEY('test')
			CSV(precision(2))
		`)
		req, _ := http.NewRequest(http.MethodPost, httpServerAddress+"/web/api/tql", reader)
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", at))
		req.Header.Set("X-Console-Id", "1234 console-log-level=INFO log-level=ERROR")
		rsp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, rsp.StatusCode)
		result, _ := io.ReadAll(rsp.Body)
		require.Equal(t, strings.Join([]string{"1,0.00", "2,0.25", "3,0.50", "4,0.75", "5,1.00", "\n"}, "\n"), string(result))
		wg.Done()
	}()
	wg.Wait()
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
	at, _, err := jwtLogin("sys", "manager")
	require.NoError(t, err)

	req, _ := http.NewRequest(http.MethodGet, httpServerAddress+"/web/api/refs/", nil)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", at))
	rsp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rsp.StatusCode)
	require.Equal(t, "application/json; charset=utf-8", rsp.Header.Get("Content-Type"))

	result, err := io.ReadAll(rsp.Body)
	require.NoError(t, err)
	rsp.Body.Close()

	var obj RefsResponse
	err = json.Unmarshal(result, &obj)
	require.Nil(t, err)

	require.Equal(t, 3, len(obj.Data.Refs))
	require.Equal(t, obj.Data.Refs[0].Label, "REFERENCES")
	require.Equal(t, 5, len(obj.Data.Refs[0].Items))

	require.Equal(t, obj.Data.Refs[1].Label, "SDK")
	require.Equal(t, 5, len(obj.Data.Refs[1].Items))

	require.Equal(t, obj.Data.Refs[2].Label, "CHEAT SHEETS")
	require.Equal(t, 3, len(obj.Data.Refs[2].Items))
}

func TestLoginRoute(t *testing.T) {
	// wrong password case - login
	b := &bytes.Buffer{}
	loginReq := &LoginReq{
		LoginName: "sys",
		Password:  "wrong",
	}
	if err := json.NewEncoder(b).Encode(loginReq); err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest(http.MethodPost, httpServerAddress+"/web/api/login", b)
	req.Header.Set("Content-type", "application/json")
	rsp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, rsp.StatusCode)
	rsp.Body.Close()

	// success case - login
	b.Reset()
	loginReq = &LoginReq{
		LoginName: "sys",
		Password:  "manager",
	}
	if err := json.NewEncoder(b).Encode(loginReq); err != nil {
		t.Fatal(err)
	}
	req, _ = http.NewRequest(http.MethodPost, httpServerAddress+"/web/api/login", b)
	req.Header.Set("Content-type", "application/json")
	rsp, err = http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rsp.StatusCode)

	dec := json.NewDecoder(rsp.Body)
	loginRsp := &LoginRsp{}
	err = dec.Decode(loginRsp)
	require.NoError(t, err)
	rsp.Body.Close()

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
	b.Reset()
	reloginReq := &ReLoginReq{
		RefreshToken: loginRsp.RefreshToken,
	}
	if err := json.NewEncoder(b).Encode(reloginReq); err != nil {
		t.Fatal(err)
	}

	req, _ = http.NewRequest(http.MethodPost, httpServerAddress+"/web/api/relogin", b)
	req.Header.Set("Content-type", "application/json")
	req.Header.Set("Authorization", "Bearer "+loginRsp.AccessToken)
	rsp, err = http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rsp.StatusCode)

	dec = json.NewDecoder(rsp.Body)
	reRsp := &ReLoginRsp{}
	err = dec.Decode(reRsp)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rsp.StatusCode)
	rsp.Body.Close()

	// success case - logout
	b.Reset()
	logoutReq := &LogoutReq{
		RefreshToken: reRsp.RefreshToken,
	}
	if err := json.NewEncoder(b).Encode(logoutReq); err != nil {
		t.Fatal(err)
	}

	req, _ = http.NewRequest(http.MethodPost, httpServerAddress+"/web/api/logout", b)
	req.Header.Set("Content-type", "application/json")
	req.Header.Set("Authorization", "Bearer "+reRsp.AccessToken)
	rsp, err = http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rsp.StatusCode)

	dec = json.NewDecoder(rsp.Body)
	logoutRsp := &LogoutRsp{}
	err = dec.Decode(logoutRsp)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rsp.StatusCode)
	require.True(t, logoutRsp.Success)
	rsp.Body.Close()

	// session check
	b.Reset()
	req, _ = http.NewRequest(http.MethodGet, httpServerAddress+"/web/api/check", b)
	req.Header.Set("Authorization", "Bearer "+reRsp.AccessToken)
	rsp, err = http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rsp.StatusCode)
	dec = json.NewDecoder(rsp.Body)
	checkRsp := &LoginCheckRsp{}
	err = dec.Decode(checkRsp)
	require.NoError(t, err)
	rsp.Body.Close()
	require.Equal(t, http.StatusOK, rsp.StatusCode)
	require.True(t, checkRsp.Success)
}

func TestLogin(t *testing.T) {
	at, rt, err := jwtLogin("sys", "manager")
	require.NoError(t, err)
	require.NotEmpty(t, at)
	require.NotEmpty(t, rt)

	at, rt, err = jwtLogin("sys", "wrong")
	require.Equal(t, "404 Not Found", err.Error())
	require.Empty(t, at)
	require.Empty(t, rt)
}

func jwtLogin(username, password string) (string, string, error) {
	req, _ := http.NewRequest(
		http.MethodPost,
		httpServerAddress+"/web/api/login",
		bytes.NewBufferString(fmt.Sprintf(`{"loginName":"%s","password":"%s"}`, username, password)))
	req.Header.Set("Content-Type", "application/json")
	rsp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", err
	}
	if rsp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf(rsp.Status)
	}
	loginRsp := &LoginRsp{}
	err = json.NewDecoder(rsp.Body).Decode(loginRsp)
	if err != nil {
		return "", "", err
	}
	rsp.Body.Close()
	return loginRsp.AccessToken, loginRsp.RefreshToken, nil
}

// Test /web/api/md
func TestMarkdown(t *testing.T) {
	at, _, err := jwtLogin("sys", "manager")
	require.NoError(t, err)
	require.NotEmpty(t, at)

	test := []struct {
		name       string
		inputFile  string
		expectFile string
		referer    string
	}{
		{
			name:       "list",
			referer:    httpServerAddress + "/web/api/tql/sample/file.wrk",
			inputFile:  "./test/test_markdown_list.md",
			expectFile: "./test/test_markdown_list.txt",
		},
		{
			name:       "list-utf8",
			referer:    "http://127.0.0.1:5654/web/api/tql/语言/文檔.wrk",
			inputFile:  "./test/test_markdown_list_utf8.md",
			expectFile: "./test/test_markdown_list_utf8.txt",
		},
		{
			name:       "mermaid",
			referer:    "http://127.0.0.1:5654/web/api/tql/语言/文檔.wrk",
			inputFile:  "./test/test_markdown_mermaid.md",
			expectFile: "./test/test_markdown_mermaid.txt",
		},
	}
	for _, tc := range test {
		t.Run(tc.name, func(t *testing.T) {
			input, err := os.ReadFile(tc.inputFile)
			require.NoError(t, err)
			req, _ := http.NewRequest(
				http.MethodPost,
				httpServerAddress+"/web/api/md",
				bytes.NewBuffer(input),
			)
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", at))
			if tc.referer != "" {
				refer := base64.StdEncoding.EncodeToString([]byte(tc.referer))
				req.Header.Set("X-Referer", refer)
			}
			rsp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, rsp.StatusCode)
			require.Equal(t, "application/xhtml+xml", rsp.Header.Get("Content-Type"))

			result, err := io.ReadAll(rsp.Body)
			require.NoError(t, err)
			rsp.Body.Close()

			expect, err := os.ReadFile(tc.expectFile)
			require.NoError(t, err)
			if runtime.GOOS == "windows" {
				// replace \r\n to \n for windows
				expect = bytes.ReplaceAll(expect, []byte("\r\n"), []byte("\n"))
				result = bytes.ReplaceAll(result, []byte("\r\n"), []byte("\n"))
			}
			require.Equal(t, string(expect), string(result))
		})
	}
}
