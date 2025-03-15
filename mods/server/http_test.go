package server

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gofrs/uuid/v5"
	"github.com/golang-jwt/jwt/v4"
	"github.com/gorilla/websocket"
	"github.com/machbase/neo-server/v8/api"
	"github.com/machbase/neo-server/v8/mods/eventbus"
	"github.com/machbase/neo-server/v8/mods/util"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestStatz(t *testing.T) {
	at, _, err := jwtLogin("sys", "manager")
	require.NoError(t, err)

	req, _ := http.NewRequest(http.MethodGet, httpServerAddress+"/debug/statz", nil)
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
		return "", "", errors.New(rsp.Status)
	}
	loginRsp := &LoginRsp{}
	err = json.NewDecoder(rsp.Body).Decode(loginRsp)
	if err != nil {
		return "", "", err
	}
	rsp.Body.Close()
	return loginRsp.AccessToken, loginRsp.RefreshToken, nil
}

func TestLicense(t *testing.T) {
	at, _, err := jwtLogin("sys", "manager")
	require.NoError(t, err)
	require.NotEmpty(t, at)

	tests := []struct {
		name   string
		method string
		path   string
		expect func(t *testing.T, rsp *http.Response)
	}{
		{
			name:   "get-check-eula-required",
			method: http.MethodGet,
			path:   "/web/api/check",
			expect: func(t *testing.T, rsp *http.Response) {
				require.Equal(t, http.StatusOK, rsp.StatusCode)
				body, err := io.ReadAll(rsp.Body)
				require.NoError(t, err)
				require.Equal(t, true, gjson.GetBytes(body, "success").Bool())
				require.Equal(t, "success", gjson.GetBytes(body, "reason").String())
				require.Equal(t, true, gjson.GetBytes(body, "eulaRequired").Bool())
				require.Equal(t, "Valid", gjson.GetBytes(body, "licenseStatus").String())
			},
		},
		{
			name:   "get-eula",
			method: http.MethodGet,
			path:   "/web/api/license/eula",
			expect: func(t *testing.T, rsp *http.Response) {
				require.Equal(t, http.StatusOK, rsp.StatusCode)
				require.Equal(t, "text/plain; charset=utf-8", rsp.Header.Get("Content-Type"))
				body, err := io.ReadAll(rsp.Body)
				require.NoError(t, err)
				require.Equal(t, eulaTxt, string(body))
			},
		},
		{
			name:   "post-eula",
			method: http.MethodPost,
			path:   "/web/api/license/eula",
			expect: func(t *testing.T, rsp *http.Response) {
				body, err := io.ReadAll(rsp.Body)
				require.NoError(t, err)
				require.Equal(t, http.StatusOK, rsp.StatusCode, string(body))
				require.Equal(t, "application/json; charset=utf-8", rsp.Header.Get("Content-Type"))
				require.Equal(t, true, gjson.GetBytes(body, "success").Bool())
				require.Equal(t, "success", gjson.GetBytes(body, "reason").String())
			},
		},
		{
			name:   "get-check-eula",
			method: http.MethodGet,
			path:   "/web/api/check",
			expect: func(t *testing.T, rsp *http.Response) {
				require.Equal(t, http.StatusOK, rsp.StatusCode)
				body, err := io.ReadAll(rsp.Body)
				require.NoError(t, err)
				require.Equal(t, true, gjson.GetBytes(body, "success").Bool())
				require.Equal(t, "success", gjson.GetBytes(body, "reason").String())
				require.Equal(t, false, gjson.GetBytes(body, "eulaRequired").Bool(), string(body))
				require.Equal(t, "Valid", gjson.GetBytes(body, "licenseStatus").String())
			},
		},
		{
			name:   "delete-eula",
			method: http.MethodDelete,
			path:   "/web/api/license/eula",
			expect: func(t *testing.T, rsp *http.Response) {
				body, err := io.ReadAll(rsp.Body)
				require.NoError(t, err)
				require.Equal(t, http.StatusOK, rsp.StatusCode, string(body))
				require.Equal(t, "application/json; charset=utf-8", rsp.Header.Get("Content-Type"))
				require.Equal(t, true, gjson.GetBytes(body, "success").Bool())
				require.Equal(t, "success", gjson.GetBytes(body, "reason").String())
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req, _ := http.NewRequest(tc.method, httpServerAddress+tc.path, nil)
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", at))
			rsp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			tc.expect(t, rsp)
			rsp.Body.Close()
		})
	}
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

func TestHttpWrite(t *testing.T) {
	tests := []struct {
		name             string
		queryParams      string
		payloadType      string
		payloadReq       any
		selectSql        string
		selectQueryParam string
		selectExpect     []string
	}{
		{
			name:        "json",
			queryParams: "?timeformat=s",
			payloadType: "application/json",
			payloadReq: map[string]any{
				"data": map[string]any{
					"columns": []string{"name", "time", "value", "jsondata", "ival", "sval"},
					"rows": [][]any{
						{"test_1", testTimeTick.Unix(), 1.12, nil, 101, 102},
						{"test_1", testTimeTick.Unix() + 1, 2.23, nil, 201, 202},
					},
				},
			},
			selectSql:        `select * from test_w where name = 'test_1'`,
			selectQueryParam: `&timeformat=s&format=csv`,
			selectExpect: []string{
				`NAME,TIME,VALUE,JSONDATA,IVAL,SVAL`,
				`test_1,1705291859,1.12,NULL,101,102`,
				`test_1,1705291860,2.23,NULL,201,202`,
				"\n"},
		},
		{
			name:        "ndjson",
			queryParams: "?timeformat=s&method=insert",
			payloadType: "application/x-ndjson",
			payloadReq: []any{
				map[string]any{"name": "test_2", "time": testTimeTick.Unix(), "value": 1.12, "jsondata": nil, "ival": 101, "sval": 102},
				map[string]any{"name": "test_2", "time": testTimeTick.Unix() + 1, "value": 2.23, "jsondata": nil, "ival": 201, "sval": 202},
			},
			selectSql:        `select * from test_w where name = 'test_2'`,
			selectQueryParam: `&timeformat=s&format=csv`,
			selectExpect: []string{
				`NAME,TIME,VALUE,JSONDATA,IVAL,SVAL`,
				`test_2,1705291859,1.12,NULL,101,102`,
				`test_2,1705291860,2.23,NULL,201,202`,
				"\n"},
		},
		{
			name:        "ndjson-append",
			queryParams: "?timeformat=s&method=append",
			payloadType: "application/x-ndjson",
			payloadReq: []any{
				map[string]any{"name": "test_3", "time": testTimeTick.Unix(), "value": 1.12, "jsondata": nil, "ival": 101, "sval": 102},
				map[string]any{"name": "test_3", "time": testTimeTick.Unix() + 1, "value": 2.23, "jsondata": nil, "ival": 201, "sval": 202},
			},
			selectSql:        `select * from test_w where name = 'test_3'`,
			selectQueryParam: `&timeformat=s&format=csv`,
			selectExpect: []string{
				`NAME,TIME,VALUE,JSONDATA,IVAL,SVAL`,
				`test_3,1705291859,1.12,NULL,101,102`,
				`test_3,1705291860,2.23,NULL,201,202`,
				"\n"},
		},
		{
			name:        "csv",
			queryParams: "?timeformat=s&method=insert&header=columns",
			payloadType: "text/csv",
			payloadReq: []any{
				`name,TIME,Value,JSONDATA,ival,SVAL`, // case insensitive
				`csv_1,` + fmt.Sprintf("%d", testTimeTick.Unix()) + `,1.12,,101,102`,
				`csv_1,` + fmt.Sprintf("%d", testTimeTick.Unix()+1) + `,2.23,,201,202`,
			},
			selectSql:        `select * from test_w where name = 'csv_1'`,
			selectQueryParam: `&timeformat=s&format=csv`,
			selectExpect: []string{
				`NAME,TIME,VALUE,JSONDATA,IVAL,SVAL`,
				`csv_1,1705291859,1.12,NULL,101,102`,
				`csv_1,1705291860,2.23,NULL,201,202`,
				"\n"},
		},
		{
			name:        "csv-append-partial",
			queryParams: "?timeformat=s&method=append&header=columns",
			payloadType: "text/csv",
			payloadReq: []any{
				`name,TIME,Value`, // case insensitive
				`csv_partial_1,` + fmt.Sprintf("%d", testTimeTick.Unix()) + `,1.12`,
				`csv_partial_1,` + fmt.Sprintf("%d", testTimeTick.Unix()+1) + `,2.23`,
			},
			selectSql:        `select * from test_w where name = 'csv_partial_1'`,
			selectQueryParam: `&timeformat=s&format=csv`,
			selectExpect: []string{
				`NAME,TIME,VALUE,JSONDATA,IVAL,SVAL`,
				`csv_partial_1,1705291859,1.12,NULL,NULL,NULL`,
				`csv_partial_1,1705291860,2.23,NULL,NULL,NULL`,
				"\n"},
		},
		{
			name:        "csv-append-partial2",
			queryParams: "?timeformat=s&method=append&header=columns",
			payloadType: "text/csv",
			payloadReq: []any{
				`name,TIME,Value,sval`, // case insensitive
				`csv_partial_2,` + fmt.Sprintf("%d", testTimeTick.Unix()) + `,1.12,102`,
				`csv_partial_2,` + fmt.Sprintf("%d", testTimeTick.Unix()+1) + `,2.23,202`,
			},
			selectSql:        `select * from test_w where name = 'csv_partial_2'`,
			selectQueryParam: `&timeformat=s&format=csv`,
			selectExpect: []string{
				`NAME,TIME,VALUE,JSONDATA,IVAL,SVAL`,
				`csv_partial_2,1705291859,1.12,NULL,NULL,102`,
				`csv_partial_2,1705291860,2.23,NULL,NULL,202`,
				"\n"},
		},
		{
			name:        "csv",
			queryParams: "?timeformat=s&method=insert&header=columns&compress=gzip",
			payloadType: "text/csv",
			payloadReq: []any{
				`name,TIME,Value,JSONDATA,ival,SVAL`, // case insensitive
				`csv_gzip,` + fmt.Sprintf("%d", testTimeTick.Unix()) + `,1.12,,101,102`,
				`csv_gzip,` + fmt.Sprintf("%d", testTimeTick.Unix()+1) + `,2.23,,201,202`,
			},
			selectSql:        `select * from test_w where name = 'csv_gzip'`,
			selectQueryParam: `&timeformat=s&format=csv`,
			selectExpect: []string{
				`NAME,TIME,VALUE,JSONDATA,IVAL,SVAL`,
				`csv_gzip,1705291859,1.12,NULL,101,102`,
				`csv_gzip,1705291860,2.23,NULL,201,202`,
				"\n"},
		},
		{
			name:        "csv-append-partial-gzip",
			queryParams: "?timeformat=s&method=append&header=columns&compress=gzip",
			payloadType: "text/csv",
			payloadReq: []any{
				`name,TIME,Value`, // case insensitive
				`csv_partial_gzip,` + fmt.Sprintf("%d", testTimeTick.Unix()) + `,1.12`,
				`csv_partial_gzip,` + fmt.Sprintf("%d", testTimeTick.Unix()+1) + `,2.23`,
			},
			selectSql:        `select * from test_w where name = 'csv_partial_gzip'`,
			selectQueryParam: `&timeformat=s&format=csv`,
			selectExpect: []string{
				`NAME,TIME,VALUE,JSONDATA,IVAL,SVAL`,
				`csv_partial_gzip,1705291859,1.12,NULL,NULL,NULL`,
				`csv_partial_gzip,1705291860,2.23,NULL,NULL,NULL`,
				"\n"},
		},
	}

	at, _, err := jwtLogin("sys", "manager")
	require.NoError(t, err)

	creTable := `create tag table test_w (
		name varchar(200) primary key,
		time datetime basetime,
		value double summarized,
		jsondata json,
		ival int,
		sval short)`
	req, _ := http.NewRequest(http.MethodGet, httpServerAddress+"/db/query?q="+url.QueryEscape(creTable), nil)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", at))
	rsp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rsp.StatusCode)
	rsp.Body.Close()

	t.Cleanup(func() {
		dropTable := `drop table test_w`
		req, _ := http.NewRequest(http.MethodGet, httpServerAddress+"/db/query?q="+url.QueryEscape(dropTable), nil)
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", at))
		rsp, _ := http.DefaultClient.Do(req)
		require.Equal(t, http.StatusOK, rsp.StatusCode)
		rsp.Body.Close()
	})

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var payload io.Reader
			var compressed bool
			if tc.payloadType == "application/json" {
				b, _ := json.Marshal(tc.payloadReq)
				payload = bytes.NewBuffer(b)
			} else if tc.payloadType == "application/x-ndjson" {
				b := &bytes.Buffer{}
				enc := json.NewEncoder(b)
				for _, row := range tc.payloadReq.([]any) {
					enc.Encode(row)
				}
				payload = b
			} else if tc.payloadType == "text/csv" {
				var w io.Writer
				b := &bytes.Buffer{}
				if strings.Contains(tc.queryParams, "compress=gzip") {
					compressed = true
					w = gzip.NewWriter(b)
				} else {
					w = b
				}
				for _, row := range tc.payloadReq.([]any) {
					w.Write([]byte(row.(string) + "\n"))
				}
				if g, ok := w.(*gzip.Writer); ok {
					g.Close()
				}
				payload = b
			}
			req, _ := http.NewRequest(http.MethodPost, httpServerAddress+"/db/write/test_w"+tc.queryParams, payload)
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", at))
			req.Header.Set("Content-Type", tc.payloadType)
			if compressed {
				req.Header.Set("Content-Encoding", "gzip")
			}
			rsp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			rspBody, _ := io.ReadAll(rsp.Body)
			rsp.Body.Close()
			require.Equal(t, http.StatusOK, rsp.StatusCode, string(rspBody))

			api.FlushAppendWorkers()
			conn, _ := httpServer.db.Connect(context.Background(), api.WithTrustUser("sys"))
			conn.Exec(context.Background(), `EXEC table_flush(test_w)`)
			conn.Close()

			if tc.selectSql != "" {
				req, _ = http.NewRequest(http.MethodGet,
					httpServerAddress+"/db/query?q="+url.QueryEscape(tc.selectSql)+tc.selectQueryParam, nil)
				req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", at))
				rsp, err = http.DefaultClient.Do(req)
				require.NoError(t, err)
				require.Equal(t, http.StatusOK, rsp.StatusCode, string(rspBody))
				rspBody, _ = io.ReadAll(rsp.Body)
				rsp.Body.Close()
				require.Equal(t, strings.Join(tc.selectExpect, "\n"), string(rspBody))
			}
		})
	}
}

func TestImageFileUploadAndWatch(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skip test on windows")
	}

	at, _, err := jwtLogin("sys", "manager")
	require.NoError(t, err)

	creTable := `create tag table test (
		NAME varchar(200) primary key,
		TIME datetime basetime,
		VALUE double summarized,
		EXT_DATA json)`
	req, _ := http.NewRequest(http.MethodGet, httpServerAddress+"/db/query?q="+url.QueryEscape(creTable), nil)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", at))
	rsp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rsp.StatusCode)
	rsp.Body.Close()

	t.Cleanup(func() {
		dropTable := `drop table test`
		req, _ := http.NewRequest(http.MethodGet, httpServerAddress+"/db/query?q="+url.QueryEscape(dropTable), nil)
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", at))
		rsp, _ := http.DefaultClient.Do(req)
		require.Equal(t, http.StatusOK, rsp.StatusCode)
		rsp.Body.Close()
	})

	t.Run("watcher", func(t *testing.T) {
		// call watch api
		params := url.Values{}
		params.Add("tag", "test")
		params.Add("period", "2s")
		params.Add("keep-alive", "10s")
		params.Add("max-rows", "3")
		params.Add("parallelism", "1")
		params.Add("timeformat", "Default")
		params.Add("tz", "local")

		req, _ := http.NewRequest(http.MethodGet, httpServerAddress+"/db/watch/test?"+params.Encode(), nil)
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", at))
		rsp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		if rsp.StatusCode != http.StatusOK {
			rspBody, _ := io.ReadAll(rsp.Body)
			rsp.Body.Close()
			require.Equal(t, http.StatusOK, rsp.StatusCode, string(rspBody))
		}
		t.Parallel()

		var fileId string
		reader := bufio.NewReader(rsp.Body)
	waiting_loop:
		for {
			line, err := reader.ReadString('\n')
			require.NoError(t, err)
			line = strings.TrimSpace(line)
			switch line {
			case ": keep-alive", "":
				continue
			default:
				msg := strings.TrimPrefix(line, "data: ")
				require.Equal(t, "test", gjson.Get(msg, "NAME").String(), msg)
				require.Equal(t, testTimeTick.Format(util.GetTimeformat("Default")), gjson.Get(msg, "TIME").String(), msg)
				extData := gjson.Get(msg, "EXT_DATA").String()
				require.Equal(t, "image.png", gjson.Get(extData, "FN").String(), extData)
				require.Equal(t, int64(12692), gjson.Get(extData, "SZ").Int(), extData)
				require.Equal(t, "image/png", gjson.Get(extData, "CT").String(), extData)
				require.Equal(t, "./testsuite_tmp/store", gjson.Get(extData, "SD").String(), extData)
				fileId = gjson.Get(extData, "ID").String()
				break waiting_loop
			}
		}
		rsp.Body.Close()

		// get image file
		req, _ = http.NewRequest(http.MethodGet, httpServerAddress+"/db/query/file/TEST/EXT_DATA/"+fileId, nil)
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", at))
		rsp, err = http.DefaultClient.Do(req)
		require.NoError(t, err)
		rspBody, _ := io.ReadAll(rsp.Body)
		rsp.Body.Close()
		require.Equal(t, http.StatusOK, rsp.StatusCode, "%s %q", string(rspBody), fileId)
		require.Equal(t, "image/png", rsp.Header.Get("Content-Type"))
		require.Equal(t, "12692", rsp.Header.Get("Content-Length"))
		require.Equal(t, "attachment; filename=image.png", rsp.Header.Get("Content-Disposition"))
		rsp.Body.Close()

		imgBody, _ := os.ReadFile("test/image.png")
		require.Equal(t, imgBody, rspBody)
	})

	t.Run("uploader", func(t *testing.T) {
		t.Parallel()

		fd, _ := os.Open("test/image.png")
		req, err = buildMultipartFormDataRequest(httpServerAddress+"/db/write/TEST",
			[]string{"NAME", "TIME", "VALUE", "EXT_DATA"},
			[]any{"test", testTimeTick, 3.14, fd})
		if err != nil {
			t.Fatal(err)
			return
		}
		rsp, err = http.DefaultClient.Do(req)
		require.NoError(t, err)
		rspBody, _ := io.ReadAll(rsp.Body)
		rsp.Body.Close()
		require.Equal(t, http.StatusOK, rsp.StatusCode, string(rspBody))

		result := map[string]any{}
		if err := json.Unmarshal(rspBody, &result); err != nil {
			t.Fatal(err)
		}
		ext_id := result["data"].(map[string]any)["files"].(map[string]any)["EXT_DATA"].(map[string]any)["ID"].(string)
		elapsed := result["elapse"].(string)
		require.JSONEq(t, `
			{
				"success":true,
				"reason":"success, 1 record(s) inserted",
				"elapse":"`+elapsed+`",
				"data": {
					"files": {
						"EXT_DATA": { 
							"CT":"image/png",
							"FN":"image.png",
							"ID":"`+ext_id+`",
							"SD":"./testsuite_tmp/store",
							"SZ":12692
						}
					}
				}
			}`, string(rspBody))

		var id uuid.UUID
		err = id.Parse(ext_id)
		require.NoError(t, err, rspBody)
		require.Equal(t, uint8(6), id.Version(), rspBody)
		timestamp, err := uuid.TimestampFromV6(id)
		require.NoError(t, err, rspBody)
		ts, err := timestamp.Time()
		require.NoError(t, err, rspBody)
		require.LessOrEqual(t, ts.UnixNano(), testTimeTick.UnixNano(), rspBody)
		require.GreaterOrEqual(t, ts.UnixNano(), testTimeTick.Add(-5*time.Second).UnixNano(), rspBody)
	})
}

func escapeQuotes(s string) string {
	var quoteEscaper = strings.NewReplacer("\\", "\\\\", `"`, "\\\"")
	return quoteEscaper.Replace(s)
}

func buildMultipartFormDataRequest(url string, names []string, values []any) (*http.Request, error) {
	var ret *http.Request
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	for i := range names {
		key := names[i]
		r := values[i]
		h := make(textproto.MIMEHeader)
		var src io.Reader

		if fd, ok := r.(*os.File); ok {
			filename := filepath.Base(fd.Name())
			h.Set("Content-Disposition",
				fmt.Sprintf(`form-data; name="%s"; filename="%s"`,
					escapeQuotes(key), escapeQuotes(filename)))
			h.Set("X-Store-Dir", "${data}/store")
			if contentType := mime.TypeByExtension(filepath.Ext(filename)); contentType != "" {
				h.Set("Content-Type", contentType)
			} else {
				h.Set("Content-Type", "application/octet-stream")
			}
			defer fd.Close()
			src = fd
		} else {
			h.Set("Content-Disposition",
				fmt.Sprintf(`form-data; name="%s"`, escapeQuotes(key)))
			switch val := r.(type) {
			case string:
				src = bytes.NewBuffer([]byte(val))
			case time.Time:
				src = bytes.NewBuffer([]byte(fmt.Sprintf("%d", val.UnixNano())))
			case float64:
				src = bytes.NewBuffer([]byte(fmt.Sprintf("%f", val)))
			default:
				return nil, fmt.Errorf("unsupported type %T", val)
			}
		}
		if dst, err := w.CreatePart(h); err != nil {
			return nil, err
		} else {
			if _, err := io.Copy(dst, src); err != nil {
				return nil, err
			}
		}
	}
	// Don't forget to close the multipart writer.
	// If you don't close it, your request will be missing the terminating boundary.
	w.Close()

	if req, err := http.NewRequest("POST", url, &b); err != nil {
		return nil, err
	} else {
		ret = req
	}
	// Don't forget to set the content type, this will contain the boundary.
	ret.Header.Set("Content-Type", w.FormDataContentType())
	return ret, nil
}

func TestLineProtocol(t *testing.T) {
	tests := []struct {
		name string
		data string
	}{
		{
			name: "ilp-success",
			data: `cpu,cpu=cpu-total,host=desktop usage_irq=0,usage_softirq=0.004171359446037821,usage_guest=0,usage_user=0.3253660367906774,usage_system=0.0792558294748905,usage_idle=99.59120677410203,usage_guest_nice=0,usage_nice=0,usage_iowait=0,usage_steal=0 1670975120000000000
mem,host=desktop committed_as=8780218368i,dirty=327680i,huge_pages_free=0i,shared=67067904i,sreclaimable=414224384i,total=67377881088i,buffered=810778624i,vmalloc_total=35184372087808i,active=3356581888i,available_percent=95.04513097460023,free=56726638592i,slab=617472000i,available=64039395328i,vmalloc_used=54685696i,cached=7298387968i,inactive=6323064832i,low_total=0i,page_tables=32129024i,high_free=0i,commit_limit=35836420096i,high_total=0i,swap_total=2147479552i,write_back_tmp=0i,write_back=0i,used=2542075904i,swap_cached=0i,vmalloc_chunk=0i,mapped=652132352i,huge_page_size=2097152i,huge_pages_total=0i,low_free=0i,sunreclaim=203247616i,swap_free=2147479552i,used_percent=3.7728641253646424 1670975120000000000
disk,device=nvme0n1p3,fstype=ext4,host=desktop,mode=rw,path=/ total=1967315451904i,free=1823398948864i,used=43906785280i,used_percent=2.3513442109214915,inodes_total=122068992i,inodes_free=121125115i,inodes_used=943877i 1670975120000000000
system,host=desktop n_users=2i,load1=0.08,load5=0.1,load15=0.09,n_cpus=24i 1670975120000000000
system,host=desktop uptime=513536i 1670975120000000000
system,host=desktop uptime_format="5 days, 22:38" 1670975120000000000
processes,host=desktop zombies=0i,unknown=0i,dead=0i,paging=0i,total_threads=1084i,blocked=0i,stopped=0i,running=0i,sleeping=282i,total=426i,idle=144i 1670975120000000000`,
		},
	}

	at, _, err := jwtLogin("sys", "manager")
	require.NoError(t, err)

	creTable := `create tag table test (
		NAME varchar(200) primary key,
		TIME datetime basetime,
		VALUE double summarized,
		EXT_DATA json)`
	req, _ := http.NewRequest(http.MethodGet, httpServerAddress+"/db/query?q="+url.QueryEscape(creTable), nil)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", at))
	rsp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rsp.StatusCode)
	rsp.Body.Close()

	t.Cleanup(func() {
		dropTable := `drop table test`
		req, _ := http.NewRequest(http.MethodGet, httpServerAddress+"/db/query?q="+url.QueryEscape(dropTable), nil)
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", at))
		rsp, _ := http.DefaultClient.Do(req)
		require.Equal(t, http.StatusOK, rsp.StatusCode)
		rsp.Body.Close()
	})

	for _, tc := range tests {
		for _, compress := range []bool{false, true} {
			buf := &bytes.Buffer{}
			if compress {
				w := gzip.NewWriter(buf)
				w.Write([]byte(tc.data))
				w.Close()
			} else {
				buf.WriteString(tc.data)
			}
			testName := tc.name
			if compress {
				testName += "-gzip"
			}
			t.Run(testName, func(t *testing.T) {
				// success case - line protocol
				req, _ := http.NewRequest("POST", httpServerAddress+"/metrics/write?db=test", buf)
				req.Header.Set("Content-Type", "application/octet-stream")
				if compress {
					req.Header.Set("Content-Encoding", "gzip")
				}
				rsp, err := http.DefaultClient.Do(req)
				require.NoError(t, err)
				rspBody, _ := io.ReadAll(rsp.Body)
				rsp.Body.Close()
				require.Equal(t, http.StatusNoContent, rsp.StatusCode, string(rspBody))
			})
		}
	}
}

func TestHttpQuery(t *testing.T) {
	tests := []struct {
		name        string
		sqlText     string
		params      url.Values
		contentType string
		expectObj   map[string]any
	}{
		{
			name:        "select_v$example",
			sqlText:     `select (min(min_time)),(max(max_time)) from v$EXAMPLE_stat where name = 'temp'`,
			contentType: "application/json",
			expectObj: map[string]any{
				"success": true, "reason": "success",
				"data": map[string]any{
					"columns": []any{"(MIN(MIN_TIME))", "(MAX(MAX_TIME))"},
					"types":   []any{"datetime", "datetime"},
					"rows": []any{
						[]any{float64(testTimeTick.UnixNano()), float64(testTimeTick.UnixNano())},
					},
				},
			},
		},
		{
			name:    "select_v$example_transpose",
			sqlText: `select (min(min_time)),(max(max_time)) from v$EXAMPLE_stat where name = 'temp'`,
			params: url.Values{
				"transpose": []string{"true"},
			},
			contentType: "application/json",
			expectObj: map[string]any{
				"success": true, "reason": "success",
				"data": map[string]any{
					"columns": []any{"(MIN(MIN_TIME))", "(MAX(MAX_TIME))"},
					"types":   []any{"datetime", "datetime"},
					"cols": []any{
						[]any{float64(testTimeTick.UnixNano())},
						[]any{float64(testTimeTick.UnixNano())},
					},
				},
			},
		},
		{
			name:    "select_v$example_rowsFlatten",
			sqlText: `select (min(min_time)),(max(max_time)) from v$EXAMPLE_stat where name = 'temp'`,
			params: url.Values{
				"rowsFlatten": []string{"true"},
			},
			contentType: "application/json",
			expectObj: map[string]any{
				"success": true, "reason": "success",
				"data": map[string]any{
					"columns": []any{"(MIN(MIN_TIME))", "(MAX(MAX_TIME))"},
					"types":   []any{"datetime", "datetime"},
					"rows": []any{
						float64(testTimeTick.UnixNano()), float64(testTimeTick.UnixNano()),
					},
				},
			},
		},
		{
			name:    "select_v$example_rowsArray",
			sqlText: `select (min(min_time)),(max(max_time)) from v$EXAMPLE_stat where name = 'temp'`,
			params: url.Values{
				"rowsArray": []string{"true"},
			},
			contentType: "application/json",
			expectObj: map[string]any{
				"success": true, "reason": "success",
				"data": map[string]any{
					"columns": []any{"(MIN(MIN_TIME))", "(MAX(MAX_TIME))"},
					"types":   []any{"datetime", "datetime"},
					"rows": []any{
						map[string]any{"(MIN(MIN_TIME))": float64(testTimeTick.UnixNano()), "(MAX(MAX_TIME))": float64(testTimeTick.UnixNano())},
					},
				},
			},
		},
		{
			name: "select_between_sub_query",
			sqlText: `SELECT
						to_timestamp((mTime)) AS TIME,
						SUM(SUMMVAL) / SUM(CNTMVAL) AS VALUE
					FROM (
						SELECT
							TIME / (1000 * 1000 * 1000) * (1000 * 1000 * 1000) as mtime, 
							sum(VALUE) as SUMMVAL,
							count(VALUE) as CNTMVAL
						FROM
							EXAMPLE
						WHERE
							NAME = 'test.query'
						AND TIME BETWEEN 1705291858000000000 and 1705291958000000000
						GROUP BY mTime
					)
					GROUP BY TIME
					ORDER by TIME LIMIT 400`,
			contentType: "application/json",
			expectObj: map[string]any{
				"success": true, "reason": "success",
				"data": map[string]any{
					"columns": []any{"TIME", "VALUE"},
					"types":   []any{"int64", "double"},
					"rows": []any{
						[]any{float64(testTimeTick.Add(time.Second).UnixNano()), 1.5 * 1.0},
						[]any{float64(testTimeTick.Add(2 * time.Second).UnixNano()), 1.5 * 2.0},
						[]any{float64(testTimeTick.Add(3 * time.Second).UnixNano()), 1.5 * 3.0},
						[]any{float64(testTimeTick.Add(4 * time.Second).UnixNano()), 1.5 * 4.0},
						[]any{float64(testTimeTick.Add(5 * time.Second).UnixNano()), 1.5 * 5.0},
						[]any{float64(testTimeTick.Add(6 * time.Second).UnixNano()), 1.5 * 6.0},
						[]any{float64(testTimeTick.Add(7 * time.Second).UnixNano()), 1.5 * 7.0},
						[]any{float64(testTimeTick.Add(8 * time.Second).UnixNano()), 1.5 * 8.0},
						[]any{float64(testTimeTick.Add(9 * time.Second).UnixNano()), 1.5 * 9.0},
						[]any{float64(testTimeTick.Add(10 * time.Second).UnixNano()), 1.5 * 10.0},
					},
				},
			},
		},
	}

	at, _, err := jwtLogin("sys", "manager")
	require.NoError(t, err)

	for _, tc := range tests {
		t.Run("GET_"+tc.name, func(t *testing.T) {
			var params = "q=" + url.QueryEscape(tc.sqlText) + "&" + tc.params.Encode()
			req, _ := http.NewRequest(http.MethodGet, httpServerAddress+"/db/query?"+params, nil)
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", at))
			rsp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, rsp.StatusCode)
			require.Equal(t, tc.contentType, rsp.Header.Get("Content-Type"))
			result, _ := io.ReadAll(rsp.Body)
			rsp.Body.Close()

			resultObj := map[string]any{}
			err = json.Unmarshal(result, &resultObj)
			require.NoError(t, err)
			delete(resultObj, "elapse")
			require.EqualValues(t, tc.expectObj, resultObj)
		})
		t.Run("POST_"+tc.name, func(t *testing.T) {
			params := map[string]any{"q": tc.sqlText}
			for k, v := range tc.params {
				switch k {
				case "transpose", "rowsFlatten", "rowsArray":
					params[k] = v[0] == "true"
				default:
					params[k] = v[0]
				}
			}
			payload, _ := json.Marshal(params)
			req, _ := http.NewRequest(http.MethodPost, httpServerAddress+"/db/query", bytes.NewBuffer(payload))
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", at))
			req.Header.Set("Content-Type", "application/json")
			rsp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			result, _ := io.ReadAll(rsp.Body)
			rsp.Body.Close()
			require.Equal(t, http.StatusOK, rsp.StatusCode, string(result))
			require.Equal(t, tc.contentType, rsp.Header.Get("Content-Type"))

			resultObj := map[string]any{}
			err = json.Unmarshal(result, &resultObj)
			require.NoError(t, err)
			delete(resultObj, "elapse")
			require.EqualValues(t, tc.expectObj, resultObj)
		})

	}
}

func TestHttpTables(t *testing.T) {
	tests := []struct {
		name       string
		queryParam string
		expectObj  map[string]any
	}{
		{
			name: "tables",
			expectObj: map[string]any{
				"success": true, "reason": "success",
				"data": map[string]any{
					"columns": []any{"ROWNUM", "DB", "USER", "NAME", "TYPE"},
					"types":   []any{"int32", "string", "string", "string", "string"},
					"rows": []any{
						[]any{float64(1), "MACHBASEDB", "SYS", "EXAMPLE", "Tag Table"},
						[]any{float64(2), "MACHBASEDB", "SYS", "LOG_DATA", "Log Table"},
						[]any{float64(3), "MACHBASEDB", "SYS", "TAG_DATA", "Tag Table"},
						[]any{float64(4), "MACHBASEDB", "SYS", "TAG_SIMPLE", "Tag Table"},
					},
				},
			},
		},
		{
			name:       "tables_name_filter",
			queryParam: "?showall=true&name=*DATA*",
			expectObj: map[string]any{
				"success": true, "reason": "success",
				"data": map[string]any{
					"columns": []any{"ROWNUM", "DB", "USER", "NAME", "TYPE"},
					"types":   []any{"int32", "string", "string", "string", "string"},
					"rows": []any{
						[]any{float64(1), "MACHBASEDB", "SYS", "LOG_DATA", "Log Table"},
						[]any{float64(2), "MACHBASEDB", "SYS", "TAG_DATA", "Tag Table"},
						[]any{float64(3), "MACHBASEDB", "SYS", "_EXAMPLE_DATA_0", "KeyValue Table (data)"},
						[]any{float64(4), "MACHBASEDB", "SYS", "_TAG_DATA_DATA_0", "KeyValue Table (data)"},
						[]any{float64(5), "MACHBASEDB", "SYS", "_TAG_DATA_META", "Lookup Table (meta)"},
						[]any{float64(6), "MACHBASEDB", "SYS", "_TAG_SIMPLE_DATA_0", "KeyValue Table (data)"},
					},
				},
			},
		},
	}

	at, _, err := jwtLogin("sys", "manager")
	require.NoError(t, err)

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req, _ := http.NewRequest(http.MethodGet, httpServerAddress+"/web/api/tables"+tc.queryParam, nil)
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", at))
			rsp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, rsp.StatusCode)
			require.Equal(t, "application/json; charset=utf-8", rsp.Header.Get("Content-Type"))
			result, _ := io.ReadAll(rsp.Body)
			rsp.Body.Close()

			resultObj := map[string]any{}
			err = json.Unmarshal(result, &resultObj)
			require.NoError(t, err)
			delete(resultObj, "elapse")
			require.EqualValues(t, tc.expectObj, resultObj)
		})
	}
}

func TestHttpTags(t *testing.T) {
	tests := []struct {
		name      string
		table     string
		expectObj map[string]any
	}{
		{
			name:  "tags",
			table: "example",
			expectObj: map[string]any{
				"success": true, "reason": "success",
				"data": map[string]any{
					"columns": []any{"ROWNUM", "NAME"},
					"types":   []any{"int32", "string"},
					"rows": []any{
						[]any{float64(1), "temp"},
						[]any{float64(2), "test.query"},
					},
				},
			},
		},
	}

	at, _, err := jwtLogin("sys", "manager")
	require.NoError(t, err)

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req, _ := http.NewRequest(http.MethodGet, httpServerAddress+"/web/api/tables/"+tc.table+"/tags", nil)
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", at))
			rsp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, rsp.StatusCode)
			require.Equal(t, "application/json; charset=utf-8", rsp.Header.Get("Content-Type"))
			result, _ := io.ReadAll(rsp.Body)
			rsp.Body.Close()

			resultObj := map[string]any{}
			err = json.Unmarshal(result, &resultObj)
			require.NoError(t, err)
			delete(resultObj, "elapse")
			require.EqualValues(t, tc.expectObj, resultObj)
		})
	}
}

func TestHttpTagStat(t *testing.T) {
	tests := []struct {
		name      string
		table     string
		tag       string
		expectObj map[string]any
	}{
		{
			name:  "tag_stat",
			table: "example",
			tag:   "temp",
			expectObj: map[string]any{
				"success": true, "reason": "success",
				"data": map[string]any{
					"columns": []any{
						"ROWNUM", "NAME", "ROW_COUNT", "MIN_TIME", "MAX_TIME",
						"MIN_VALUE", "MIN_VALUE_TIME", "MAX_VALUE", "MAX_VALUE_TIME", "RECENT_ROW_TIME"},
					"types": []any{"int32", "string", "int64", "datetime", "datetime", "double", "datetime", "double", "datetime", "datetime"},
					"rows": []any{
						[]any{
							float64(1), "temp", float64(1), float64(testTimeTick.UnixNano()), float64(testTimeTick.UnixNano()),
							3.14, float64(testTimeTick.UnixNano()), 3.14, float64(testTimeTick.UnixNano()), float64(testTimeTick.UnixNano())},
					},
				},
			},
		},
	}

	at, _, err := jwtLogin("sys", "manager")
	require.NoError(t, err)

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req, _ := http.NewRequest(http.MethodGet, httpServerAddress+"/web/api/tables/"+tc.table+"/tags/"+tc.tag+"/stat", nil)
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", at))
			rsp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, rsp.StatusCode)
			require.Equal(t, "application/json; charset=utf-8", rsp.Header.Get("Content-Type"))
			result, _ := io.ReadAll(rsp.Body)
			rsp.Body.Close()

			resultObj := map[string]any{}
			err = json.Unmarshal(result, &resultObj)
			require.NoError(t, err)
			delete(resultObj, "elapse")
			require.EqualValues(t, tc.expectObj, resultObj)
		})
	}
}

func TestTQL(t *testing.T) {
	tests := []struct {
		name        string
		codes       string
		contentType string
		expect      []string
		expectObj   map[string]any
	}{
		{
			name: "csv_output",
			codes: `FAKE(linspace(0,1,2))
					CSV()`,
			contentType: "text/csv; charset=utf-8",
			expect: []string{
				"0", "1", "\n",
			},
		},
		{
			name: "json_output",
			codes: `FAKE(linspace(0,1,2))
					JSON()`,
			contentType: "application/json",
			expectObj: map[string]any{"success": true, "reason": "success", "data": map[string]any{
				"columns": []any{"x"}, "types": []any{"double"},
				"rows": []any{[]any{0.0}, []any{1.0}},
			}},
		},
	}
	at, _, err := jwtLogin("sys", "manager")
	require.NoError(t, err)

	for _, tc := range tests {
		for _, method := range []string{http.MethodGet, http.MethodPost} {
			t.Run(method+"_"+tc.name, func(t *testing.T) {
				var req *http.Request
				if method == http.MethodGet {
					req, _ = http.NewRequest(method, httpServerAddress+"/web/api/tql?$="+url.QueryEscape(tc.codes), nil)
				} else {
					reader := bytes.NewBufferString(tc.codes)
					req, _ = http.NewRequest(method, httpServerAddress+"/web/api/tql", reader)
				}
				req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", at))
				rsp, err := http.DefaultClient.Do(req)
				require.NoError(t, err)
				require.Equal(t, http.StatusOK, rsp.StatusCode)
				require.Equal(t, tc.contentType, rsp.Header.Get("Content-Type"))
				result, _ := io.ReadAll(rsp.Body)
				rsp.Body.Close()
				if tc.expectObj != nil {
					resultObj := map[string]any{}
					err := json.Unmarshal(result, &resultObj)
					require.NoError(t, err)
					delete(resultObj, "elapse")
					require.EqualValues(t, tc.expectObj, resultObj)
				} else {
					require.Equal(t, strings.Join(tc.expect, "\n"), string(result))
				}
			})
		}
	}
}

func TestTQL_Payload(t *testing.T) {
	tests := []struct {
		name        string
		codes       string
		payload     []byte
		payloadType string
		contentType string
		expect      []string
		expectObj   map[string]any
	}{
		{
			name: "csv_from_payload",
			codes: `CSV(payload())
					CSV()`,
			payload:     []byte("a,1\nb,2\n"),
			payloadType: "text/csv",
			contentType: "text/csv; charset=utf-8",
			expect:      []string{"a,1", "b,2", "\n"},
		},
		{
			name:        "csv_map.tql",
			codes:       "@csv_map.tql",
			payload:     []byte("a,1\nb,2\n"),
			payloadType: "text/csv",
			contentType: "text/csv; charset=utf-8",
			expect:      []string{"a,10", "b,20", "\n"},
		},
	}
	at, _, err := jwtLogin("sys", "manager")
	require.NoError(t, err)

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var req *http.Request
			payload := bytes.NewBuffer(tc.payload)
			if strings.HasPrefix(tc.codes, "@") {
				req, _ = http.NewRequest(http.MethodPost, httpServerAddress+"/web/api/tql/"+tc.codes[1:], payload)
			} else {
				req, _ = http.NewRequest(http.MethodPost, httpServerAddress+"/web/api/tql?$="+url.QueryEscape(tc.codes), payload)
			}
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", at))
			req.Header.Set("Content-Type", tc.payloadType)

			rsp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, rsp.StatusCode)
			require.Equal(t, tc.contentType, rsp.Header.Get("Content-Type"))
			result, _ := io.ReadAll(rsp.Body)
			rsp.Body.Close()
			if tc.expectObj != nil {
				resultObj := map[string]any{}
				err := json.Unmarshal(result, &resultObj)
				require.NoError(t, err)
				delete(resultObj, "elapse")
				require.EqualValues(t, tc.expectObj, resultObj)
			} else {
				require.Equal(t, strings.Join(tc.expect, "\n"), string(result))
			}
		})
	}
}

func TestTQL_SyntaxErrors(t *testing.T) {
	tests := []struct {
		name      string
		codes     string
		expectObj map[string]any
	}{
		{
			name: "mapkey_wrong_argument",
			codes: `FAKE(linspace(0,1,2))
					MAPKEY(-1,-1) // intended syntax error
					//APPEND(table('example'))
					JSON()`,
			expectObj: map[string]any{
				"success": true,
				"reason":  "success",
				"data": map[string]any{
					"columns": []any{"x"},
					"types":   []any{"double"},
					"rows":    []any{},
				},
			},
		},
	}
	at, _, err := jwtLogin("sys", "manager")
	require.NoError(t, err)

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			reader := bytes.NewBufferString(tc.codes)
			req, _ := http.NewRequest(http.MethodPost, httpServerAddress+"/web/api/tql", reader)
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", at))
			rsp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, rsp.StatusCode)
			result, _ := io.ReadAll(rsp.Body)
			rsp.Body.Close()

			resultObj := map[string]any{}
			err = json.Unmarshal(result, &resultObj)
			require.NoError(t, err)
			delete(resultObj, "elapse")
			require.EqualValues(t, tc.expectObj, resultObj)
		})
	}
}

type SplitSqlResult struct {
	Success bool   `json:"success"`
	Reason  string `json:"reason"`
	Elapse  string `json:"elapse"`
	Data    struct {
		Statements []*util.SqlStatement `json:"statements"`
	} `json:"data,omitempty"`
}

func TestSplitSQL(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		expects []*util.SqlStatement
	}{
		{
			name:  "select_first",
			input: `select * from first;`,
			expects: []*util.SqlStatement{
				{BeginLine: 1, EndLine: 1, IsComment: false, Text: "select * from first;", Env: &util.SqlStatementEnv{}},
			},
		},
		{
			name:  "select_second",
			input: "\nselect * from second;  ",
			expects: []*util.SqlStatement{
				{BeginLine: 2, EndLine: 2, IsComment: false, Text: "select * from second;", Env: &util.SqlStatementEnv{}},
			},
		},
	}

	at, _, err := jwtLogin("sys", "manager")
	require.NoError(t, err)

	for _, tc := range tests {
		req, _ := http.NewRequest(http.MethodPost, httpServerAddress+"/web/api/splitter/sql", strings.NewReader(tc.input))
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", at))
		rsp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, rsp.StatusCode)
		result, _ := io.ReadAll(rsp.Body)
		rsp.Body.Close()

		resultObj := SplitSqlResult{}
		err = json.Unmarshal(result, &resultObj)
		require.NoError(t, err)

		require.EqualValues(t, tc.expects, resultObj.Data.Statements)
	}
}
