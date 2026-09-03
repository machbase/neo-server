package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
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
	"testing"
	"time"

	"github.com/gofrs/uuid/v5"
	"github.com/machbase/neo-server/v8/mods/logging"
	"github.com/machbase/neo-server/v8/mods/util"
	"github.com/machbase/neo-server/v8/mods/util/ssfs"
	"github.com/machbase/neo-server/v8/spi"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestHttpQueryUsesJWTCurrentUser(t *testing.T) {
	username := fmt.Sprintf("query_user_%d", time.Now().UnixNano())
	password := "query_password"
	conn, err := spi.Connect(t.Context(), "sys")
	require.NoError(t, err)
	_, err = conn.ExecContext(t.Context(), fmt.Sprintf("CREATE USER %s IDENTIFIED BY '%s'", username, password))
	require.NoError(t, err)
	conn.Close()
	t.Cleanup(func() {
		cleanupConn, connectErr := spi.Connect(context.Background(), "sys")
		if connectErr != nil {
			return
		}
		defer cleanupConn.Close()
		_, _ = cleanupConn.ExecContext(context.Background(), "DROP USER "+username)
	})

	accessToken, _, err := jwtLogin(username, password)
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodGet, httpServerAddress+"/web/api/query?q="+url.QueryEscape("SELECT current_user()"), nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	rsp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer rsp.Body.Close()
	body, err := io.ReadAll(rsp.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rsp.StatusCode, string(body))
	require.Equal(t, strings.ToUpper(username), gjson.GetBytes(body, "data.rows.0.0").String())
}

// TestHandleWatchQueryRejectsMissingExecUser guards against handleWatchQuery
// (machbase/neo#1468) silently falling back to connecting as "sys" for
// API-token-authenticated requests whose token carries no user: it must
// reject with resolveExecUser's error instead of proceeding to spi.NewWatcher.
func TestHandleWatchQueryRejectsMissingExecUser(t *testing.T) {
	svr := newTestHTTPServer(t)

	ctx, writer := newTestHTTPContext(http.MethodGet, "/db/watch/any_table", nil)
	ctx.Set("api-token-authenticated", true)
	svr.handleWatchQuery(ctx)

	require.Equal(t, http.StatusUnauthorized, writer.Code, writer.Body.String())
	require.Equal(t, "authorization user is missing", gjson.GetBytes(writer.Body.Bytes(), "reason").String())
}

// TestHttpWatchQueryExecUserScope verifies /db/watch/:table (machbase/neo#1468)
// connects using the authenticated request's own user scope rather than a fixed
// "sys" connection.
//
// /db/watch, like /db/query, is an API-token endpoint: neo-web never calls it
// (it has no watch feature at all), and its own editor sends a JWT bearer only
// to /web/api/query, never to /db/*. External API clients are the intended
// caller here (see neo-web/src/components/securityKey/{apiToken,createToken}.tsx
// for the documented "Authorization: Bearer $TOKEN" + /db/query curl example),
// so this test authenticates with a generated API token, not a JWT — a JWT
// bearer on /db/* isn't parsed by any middleware and would only prove the
// (uninteresting) fact that the "sys" fallback can watch any table.
func TestHttpWatchQueryExecUserScope(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skip test on windows")
	}

	server := coverageRunningServer(t)
	username := fmt.Sprintf("watch_scope_user_%d", time.Now().UnixNano())
	table := username + ".OWN_TBL"

	sysConn, err := spi.Connect(t.Context(), "sys")
	require.NoError(t, err)
	_, err = sysConn.ExecContext(t.Context(), fmt.Sprintf("CREATE USER %s IDENTIFIED BY 'password'", username))
	require.NoError(t, err)
	sysConn.Close()
	t.Cleanup(func() {
		cleanupConn, connectErr := spi.Connect(context.Background(), "sys")
		if connectErr != nil {
			return
		}
		defer cleanupConn.Close()
		_, _ = cleanupConn.ExecContext(context.Background(), "DROP TABLE "+table)
		_, _ = cleanupConn.ExecContext(context.Background(), "DROP USER "+username)
	})

	ownConn, err := spi.Connect(t.Context(), username)
	require.NoError(t, err)
	_, err = ownConn.ExecContext(t.Context(),
		"CREATE TAG TABLE OWN_TBL (NAME VARCHAR(40) PRIMARY KEY, TIME DATETIME BASETIME, VALUE DOUBLE SUMMARIZED)")
	require.NoError(t, err)
	ownConn.Close()

	generated, err := server.generateApiToken(contextWithModelUser(context.Background(), username), "watch-scope-test", 0)
	require.NoError(t, err)
	require.Equal(t, strings.ToUpper(username), generated.User)
	t.Cleanup(func() {
		_ = server.deleteApiToken(contextWithModelUser(context.Background(), username), generated.Id)
	})

	watch := func(reqCtx context.Context, targetTable string) *http.Response {
		params := url.Values{"tag": {"any"}, "period": {"1s"}, "keep-alive": {"1s"}}
		req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, httpServerAddress+"/db/watch/"+targetTable+"?"+params.Encode(), nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+generated.Token)
		rsp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		return rsp
	}

	t.Run("own_table_succeeds", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()
		rsp := watch(ctx, table)
		defer rsp.Body.Close()
		if rsp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(rsp.Body)
			require.Equal(t, http.StatusOK, rsp.StatusCode, string(body))
		}
		// a keep-alive line proves the SSE stream (and thus spi.NewWatcher, connected
		// as the token's own user) started successfully.
		reader := bufio.NewReader(rsp.Body)
		line, err := reader.ReadString('\n')
		require.NoError(t, err)
		require.Equal(t, ": keep-alive", strings.TrimSpace(line))
	})

	t.Run("other_users_table_rejected", func(t *testing.T) {
		// proves the request truly runs as the token owner rather than "sys":
		// "sys" could watch _NEO_API_TOKEN, but the token owner has no such grant.
		rsp := watch(t.Context(), "_NEO_API_TOKEN")
		defer rsp.Body.Close()
		require.NotEqual(t, http.StatusOK, rsp.StatusCode)
	})
}

// chain with a generated API token, complementing the JWT/MQTT current_user()
// coverage in TestHttpQueryUsesJWTCurrentUser and TestMqttQueryUsesMappedCurrentUser.
func TestHttpQueryApiTokenExecUser(t *testing.T) {
	server := coverageRunningServer(t)

	username := fmt.Sprintf("token_query_user_%d", time.Now().UnixNano())
	ownTable := username + "_own_table"
	sysConn, err := spi.Connect(t.Context(), "sys")
	require.NoError(t, err)
	_, err = sysConn.ExecContext(t.Context(), fmt.Sprintf("CREATE USER %s IDENTIFIED BY 'password'", username))
	require.NoError(t, err)
	sysConn.Close()
	t.Cleanup(func() {
		cleanupConn, connectErr := spi.Connect(context.Background(), "sys")
		if connectErr != nil {
			return
		}
		defer cleanupConn.Close()
		_, _ = cleanupConn.ExecContext(context.Background(), "DROP TABLE "+ownTable)
		_, _ = cleanupConn.ExecContext(context.Background(), "DROP USER "+username)
	})

	ownConn, err := spi.Connect(t.Context(), username)
	require.NoError(t, err)
	_, err = ownConn.ExecContext(t.Context(), fmt.Sprintf("CREATE TAG TABLE %s (name varchar(100) primary key, time datetime basetime, value double)", ownTable))
	require.NoError(t, err)
	_, err = ownConn.ExecContext(t.Context(), fmt.Sprintf("INSERT INTO %s VALUES ('temp', now, 1.0)", ownTable))
	require.NoError(t, err)
	ownConn.Close()

	generated, err := server.generateApiToken(contextWithModelUser(context.Background(), username), "e2e-test", 0)
	require.NoError(t, err)
	require.Equal(t, strings.ToUpper(username), generated.User)
	t.Cleanup(func() {
		_ = server.deleteApiToken(contextWithModelUser(context.Background(), username), generated.Id)
	})

	testHttpd := &httpd{log: logging.GetLog("http-token-e2e-test"), authServer: server}
	testHttpd.enableTokenAuth.Store(true)
	doQuery := func(sqlText string) []byte {
		ctx, writer := newTestHTTPContext(http.MethodGet, "/db/query?q="+url.QueryEscape(sqlText), nil)
		ctx.Request.Header.Set("Authorization", "Bearer "+generated.Token)
		testHttpd.handleAuthToken(ctx)
		require.False(t, ctx.IsAborted(), writer.Body.String())
		testHttpd.handleQuery(ctx)
		return writer.Body.Bytes()
	}

	t.Run("current_user_matches_token_owner", func(t *testing.T) {
		body := doQuery("SELECT current_user()")
		require.True(t, gjson.GetBytes(body, "success").Bool(), string(body))
		require.Equal(t, strings.ToUpper(username), gjson.GetBytes(body, "data.rows.0.0").String())
	})

	t.Run("sys_owned_table_rejected", func(t *testing.T) {
		body := doQuery("SELECT * FROM _NEO_API_TOKEN")
		require.False(t, gjson.GetBytes(body, "success").Bool(), string(body))
		require.NotEmpty(t, gjson.GetBytes(body, "reason").String())
	})

	t.Run("own_table_succeeds", func(t *testing.T) {
		body := doQuery("SELECT name FROM " + ownTable)
		require.True(t, gjson.GetBytes(body, "success").Bool(), string(body))
		require.Equal(t, "temp", gjson.GetBytes(body, "data.rows.0.0").String())
	})

	t.Run("optional_mode_still_honors_a_valid_bearer_token", func(t *testing.T) {
		// Http.EnableTokenAuth=false: a request without any token still runs as
		// "sys", but a request carrying a valid API token must use its scope.
		testHttpd.enableTokenAuth.Store(false)
		defer testHttpd.enableTokenAuth.Store(true)

		body := doQuery("SELECT current_user()")
		require.True(t, gjson.GetBytes(body, "success").Bool(), string(body))
		require.Equal(t, strings.ToUpper(username), gjson.GetBytes(body, "data.rows.0.0").String())
	})
}

func TestHttpQuery(t *testing.T) {
	tests := []struct {
		name        string
		sqlText     string
		params      url.Values
		contentType string
		expectObj   map[string]any
		expect      []string
	}{
		{
			name:    "select_aliveness",
			sqlText: `select 123 as VaLue`,
			params: url.Values{
				"format": []string{"box"},
			},
			contentType: "text/plain",
			expect: []string{
				"+-------+",
				"| VALUE |",
				"+-------+",
				"| 123   |",
				"+-------+",
			},
		},
		{
			name:        "select_v$example",
			sqlText:     `select (MIN(MIN_TIME)),(MAX(MAX_TIME)) from v$EXAMPLE_stat where name = 'temp'`,
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
			name:    "select_v$example_bind_params",
			sqlText: `select (MIN(MIN_TIME)),(MAX(MAX_TIME)) from v$EXAMPLE_stat where name = ?`,
			params: url.Values{
				"p": []string{`["temp"]`},
			},
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
			name:    "select_v$example_bind_params_csv",
			sqlText: `select (min(min_time)),(max(max_time)) from v$EXAMPLE_stat where name = ?`,
			params: url.Values{
				"p":      []string{`["temp"]`},
				"format": []string{"csv"},
			},
			contentType: "text/csv; charset=utf-8",
			expect: []string{
				"(min(min_time)),(max(max_time))",
				"1705291859000000000,1705291859000000000",
			},
		},
		{
			name:    "select_v$example_bind_params_csv_header_skip",
			sqlText: `select (min(min_time)),(max(max_time)) from v$EXAMPLE_stat where name = ?`,
			params: url.Values{
				"p":          []string{`["temp"]`},
				"format":     []string{"csv"},
				"header":     []string{"skip"},
				"timeformat": []string{"s"},
			},
			contentType: "text/csv; charset=utf-8",
			expect: []string{
				"1705291859,1705291859",
			},
		},
		{
			name:    "select_v$example_named_bind_params",
			sqlText: `select (MIN(MIN_TIME)),(MAX(MAX_TIME)) from v$EXAMPLE_stat where name = :name`,
			params: url.Values{
				"p": []string{`{"name":"temp"}`},
			},
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
					"columns": []any{"(min(min_time))", "(max(max_time))"},
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
					"columns": []any{"(min(min_time))", "(max(max_time))"},
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
					"columns": []any{"(min(min_time))", "(max(max_time))"},
					"types":   []any{"datetime", "datetime"},
					"rows": []any{
						map[string]any{"(min(min_time))": float64(testTimeTick.UnixNano()), "(max(max_time))": float64(testTimeTick.UnixNano())},
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

			if tc.expectObj != nil {
				resultObj := map[string]any{}
				err = json.Unmarshal(result, &resultObj)
				require.NoError(t, err)
				delete(resultObj, "elapse")
				require.EqualValues(t, tc.expectObj, resultObj)
			} else {
				lines := strings.Split(strings.TrimSpace(string(result)), "\n")
				require.Equal(t, tc.expect, lines)
			}
		})
		t.Run("POST_"+tc.name, func(t *testing.T) {
			params := map[string]any{"q": tc.sqlText}
			for k, v := range tc.params {
				params[k] = v[0]
			}
			for k, v := range tc.params {
				switch k {
				case "p":
					var bindParams any
					err := json.Unmarshal([]byte(v[0]), &bindParams)
					require.NoError(t, err)
					params[k] = bindParams
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

			if tc.expectObj != nil {
				resultObj := map[string]any{}
				err = json.Unmarshal(result, &resultObj)
				require.NoError(t, err)
				delete(resultObj, "elapse")
				require.EqualValues(t, tc.expectObj, resultObj)
			} else {
				lines := strings.Split(strings.TrimSpace(string(result)), "\n")
				require.Equal(t, tc.expect, lines)
			}
		})
		t.Run("POST_FORM_"+tc.name, func(t *testing.T) {
			params := map[string]any{"q": tc.sqlText}
			for k, v := range tc.params {
				switch k {
				case "p":
					var bindParams any
					err := json.Unmarshal([]byte(v[0]), &bindParams)
					require.NoError(t, err)
					params[k] = bindParams
				case "transpose", "rowsFlatten", "rowsArray":
					params[k] = v[0] == "true"
				default:
					params[k] = v[0]
				}
			}
			payload := url.Values{}
			for k, v := range params {
				switch val := v.(type) {
				case []any, map[string]any:
					jsonVal, _ := json.Marshal(val)
					payload.Set(k, string(jsonVal))
				case bool:
					payload.Set(k, fmt.Sprintf("%t", val))
				default:
					payload.Set(k, fmt.Sprintf("%v", val))
				}
			}
			req, _ := http.NewRequest(http.MethodPost, httpServerAddress+"/db/query", strings.NewReader(payload.Encode()))
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", at))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			rsp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			result, _ := io.ReadAll(rsp.Body)
			rsp.Body.Close()
			require.Equal(t, http.StatusOK, rsp.StatusCode, string(result))
			require.Equal(t, tc.contentType, rsp.Header.Get("Content-Type"))

			if tc.expectObj != nil {
				resultObj := map[string]any{}
				err = json.Unmarshal(result, &resultObj)
				require.NoError(t, err)
				delete(resultObj, "elapse")
				require.EqualValues(t, tc.expectObj, resultObj)
			} else {
				lines := strings.Split(strings.TrimSpace(string(result)), "\n")
				require.Equal(t, tc.expect, lines)
			}
		})
	}
}

// TestHttpQueryWithDatabaseParam verifies /db/query "db" parameter support for
// multiple-database queries (machbase/neo#1483): a request-scoped `USE <db>`
// is executed on the checked-out connection before the query, and pooled
// connections must not leak database state across unrelated requests.
func TestHttpQueryWithDatabaseParam(t *testing.T) {
	at, _, err := jwtLogin("sys", "manager")
	require.NoError(t, err)

	dbName := fmt.Sprintf("HTTPQUERYDB%d", time.Now().UnixNano()%1000000)
	tableName := fmt.Sprintf("HTTP_QUERY_DB_T_%d", time.Now().UnixNano())

	doQuery := func(t *testing.T, sqlText string, db string) (*http.Response, []byte) {
		t.Helper()
		params := url.Values{"q": []string{sqlText}}
		if db != "" {
			params.Set("db", db)
		}
		req, _ := http.NewRequest(http.MethodGet, httpServerAddress+"/db/query?"+params.Encode(), nil)
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", at))
		rsp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		body, _ := io.ReadAll(rsp.Body)
		rsp.Body.Close()
		return rsp, body
	}

	rsp, body := doQuery(t, "CREATE DATABASE IF NOT EXISTS "+dbName, "")
	require.Equal(t, http.StatusOK, rsp.StatusCode, string(body))
	t.Cleanup(func() {
		rsp, body := doQuery(t, "DROP DATABASE "+dbName+" CASCADE", "")
		require.Equal(t, http.StatusOK, rsp.StatusCode, string(body))
	})

	rsp, body = doQuery(t, fmt.Sprintf(`CREATE TABLE %s (id LONG PRIMARY KEY, name VARCHAR(40))`, tableName), "")
	require.Equal(t, http.StatusOK, rsp.StatusCode, string(body))
	t.Cleanup(func() {
		rsp, body := doQuery(t, "DROP TABLE "+tableName, "")
		require.Equal(t, http.StatusOK, rsp.StatusCode, string(body))
	})

	rsp, body = doQuery(t, fmt.Sprintf(`CREATE TABLE %s (id LONG PRIMARY KEY, name VARCHAR(40))`, tableName), dbName)
	require.Equal(t, http.StatusOK, rsp.StatusCode, string(body))

	rsp, body = doQuery(t, fmt.Sprintf(`INSERT INTO %s VALUES(1, 'default-db')`, tableName), "")
	require.Equal(t, http.StatusOK, rsp.StatusCode, string(body))

	rsp, body = doQuery(t, fmt.Sprintf(`INSERT INTO %s VALUES(1, 'other-db')`, tableName), dbName)
	require.Equal(t, http.StatusOK, rsp.StatusCode, string(body))

	t.Run("select_from_default_database", func(t *testing.T) {
		rsp, body := doQuery(t, fmt.Sprintf(`SELECT NAME FROM %s`, tableName), "")
		require.Equal(t, http.StatusOK, rsp.StatusCode, string(body))
		require.Contains(t, string(body), "default-db")
		require.NotContains(t, string(body), "other-db")
	})

	t.Run("select_from_other_database", func(t *testing.T) {
		rsp, body := doQuery(t, fmt.Sprintf(`SELECT NAME FROM %s`, tableName), dbName)
		require.Equal(t, http.StatusOK, rsp.StatusCode, string(body))
		require.Contains(t, string(body), "other-db")
		require.NotContains(t, string(body), "default-db")
	})

	t.Run("no_database_leak_across_reused_connections", func(t *testing.T) {
		// Repeatedly alternate the "db" param over the pooled connection to make sure the
		// driver resets the database back to default when the connection is reused.
		for i := 0; i < 5; i++ {
			rsp, body := doQuery(t, fmt.Sprintf(`SELECT NAME FROM %s`, tableName), dbName)
			require.Equal(t, http.StatusOK, rsp.StatusCode, string(body))
			require.Contains(t, string(body), "other-db")

			rsp, body = doQuery(t, fmt.Sprintf(`SELECT NAME FROM %s`, tableName), "")
			require.Equal(t, http.StatusOK, rsp.StatusCode, string(body))
			require.Contains(t, string(body), "default-db")
		}
	})

	t.Run("empty_db_param_uses_default_database", func(t *testing.T) {
		rsp, body := doQuery(t, "SELECT 1", "")
		require.Equal(t, http.StatusOK, rsp.StatusCode, string(body))
	})

	t.Run("invalid_db_name_returns_400", func(t *testing.T) {
		rsp, body := doQuery(t, "SELECT 1", "bad db;name")
		require.Equal(t, http.StatusBadRequest, rsp.StatusCode, string(body))
	})

	t.Run("nonexistent_db_returns_500", func(t *testing.T) {
		rsp, body := doQuery(t, "SELECT 1", "no_such_database_xyz")
		require.Equal(t, http.StatusInternalServerError, rsp.StatusCode, string(body))
	})
}

func TestHttpQueryMutation(t *testing.T) {
	mutationTable := fmt.Sprintf("HTTP_QUERY_MUT_%d", time.Now().UnixNano())
	baseTime := testTimeTick.UnixNano() + 123456789

	execMutation := func(t *testing.T, name string, sqlText string, expectedReason string) {
		t.Helper()

		params := map[string]any{"q": sqlText}
		payload, _ := json.Marshal(params)
		req, _ := http.NewRequest(http.MethodPost, httpServerAddress+"/db/query", bytes.NewBuffer(payload))
		req.Header.Set("Content-Type", "application/json")

		rsp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		result, _ := io.ReadAll(rsp.Body)
		rsp.Body.Close()
		require.Equal(t, http.StatusOK, rsp.StatusCode, "%s: %s", name, string(result))
		require.Equal(t, "application/json", rsp.Header.Get("Content-Type"), "%s", name)

		resultObj := map[string]any{}
		err = json.Unmarshal(result, &resultObj)
		require.NoError(t, err, "%s", name)

		require.Len(t, resultObj, 3, "%s", name)
		require.Contains(t, resultObj, "success", "%s", name)
		require.Contains(t, resultObj, "reason", "%s", name)
		require.Contains(t, resultObj, "elapse", "%s", name)
		require.NotContains(t, resultObj, "data", "%s", name)
		require.EqualValues(t, map[string]any{
			"success": true,
			"reason":  expectedReason,
		}, map[string]any{
			"success": resultObj["success"],
			"reason":  resultObj["reason"],
		}, "%s", name)
	}

	t.Run("mutation_create_table", func(t *testing.T) {
		execMutation(t,
			"mutation_create_table",
			fmt.Sprintf(`CREATE TAG TABLE IF NOT EXISTS %s (name varchar(40) primary key, time datetime basetime, value double summarized)`, mutationTable),
			"table created.")
	})

	t.Run("mutation_insert_data_1", func(t *testing.T) {
		execMutation(t,
			"mutation_insert_data_1",
			fmt.Sprintf(`INSERT INTO %s VALUES('http-query-mutation', %d, 3.14)`, mutationTable, baseTime),
			"a row inserted.")
	})

	t.Run("mutation_insert_data_2", func(t *testing.T) {
		execMutation(t,
			"mutation_insert_data_2",
			fmt.Sprintf(`INSERT INTO %s VALUES('http-query-mutation', %d, 6.28)`, mutationTable, baseTime+1),
			"a row inserted.")
	})

	t.Run("mutation_insert_data_3", func(t *testing.T) {
		execMutation(t,
			"mutation_insert_data_3",
			fmt.Sprintf(`INSERT INTO %s VALUES('http-query-mutation', %d, 9.42)`, mutationTable, baseTime+2),
			"a row inserted.")
	})

	t.Run("mutation_delete_data", func(t *testing.T) {
		execMutation(t,
			"mutation_delete_data",
			fmt.Sprintf(`DELETE FROM %s WHERE name='http-query-mutation'`, mutationTable),
			"3 rows deleted.")
	})

	t.Run("mutation_drop_table", func(t *testing.T) {
		execMutation(t,
			"mutation_drop_table",
			fmt.Sprintf(`DROP TABLE %s`, mutationTable),
			"table dropped.")
	})
}

func TestHttpQueryEmptySqlErrors(t *testing.T) {
	var params = "q=" + "&" + url.Values{"format": []string{"box"}}.Encode()
	req, _ := http.NewRequest(http.MethodGet, httpServerAddress+"/db/query?"+params, nil)
	rsp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusBadRequest, rsp.StatusCode)
	require.Equal(t, "application/json; charset=utf-8", rsp.Header.Get("Content-Type"))
	result, _ := io.ReadAll(rsp.Body)
	rsp.Body.Close()

	resultObj := map[string]any{}
	err = json.Unmarshal(result, &resultObj)
	require.NoError(t, err)
	delete(resultObj, "elapse")
	require.EqualValues(t, map[string]any{
		"success": false, "reason": "sql text is empty",
	}, resultObj)
}

func TestHttpQueryBindParamErrors(t *testing.T) {
	payload := url.Values{}
	payload.Set("q", `select (min(min_time)),(max(max_time)) from v$EXAMPLE_stat where name = ?`)
	payload.Set("p", `["temp"]`)

	req, _ := http.NewRequest(http.MethodPost, httpServerAddress+"/db/query", strings.NewReader(payload.Encode()))
	//req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", at))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rsp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	result, _ := io.ReadAll(rsp.Body)
	rsp.Body.Close()
	require.Equal(t, http.StatusOK, rsp.StatusCode, string(result))
	require.Equal(t, "application/json", rsp.Header.Get("Content-Type"))

	resultObj := map[string]any{}
	err = json.Unmarshal(result, &resultObj)
	require.NoError(t, err)
	delete(resultObj, "elapse")
	require.EqualValues(t, map[string]any{
		"success": true, "reason": "success",
		"data": map[string]any{
			"columns": []any{"(min(min_time))", "(max(max_time))"},
			"types":   []any{"datetime", "datetime"},
			"rows": []any{
				[]any{float64(testTimeTick.UnixNano()), float64(testTimeTick.UnixNano())},
			},
		},
	}, resultObj)
}

func TestHttpQueryBindParamInvalid(t *testing.T) {
	params := url.Values{}
	params.Set("q", `select (min(min_time)),(max(max_time)) from v$EXAMPLE_stat where name = ?`)
	params.Set("p", `[["temp"]]`)

	req, _ := http.NewRequest(http.MethodGet, httpServerAddress+"/db/query?"+params.Encode(), nil)
	// req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", at))
	rsp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	result, _ := io.ReadAll(rsp.Body)
	rsp.Body.Close()
	require.Equal(t, http.StatusBadRequest, rsp.StatusCode, string(result))

	resultObj := map[string]any{}
	err = json.Unmarshal(result, &resultObj)
	require.NoError(t, err)
	require.Equal(t, false, resultObj["success"])
	require.Contains(t, resultObj["reason"], "bind parameter must be scalar")
}

func TestHttpQueryUnsupportedContentType(t *testing.T) {
	payload := []byte(`{"q":"select (min(min_time)),(max(max_time)) from v$EXAMPLE_stat where name = ?","p":[{"name":"temp"}]}`)

	req, _ := http.NewRequest(http.MethodPost, httpServerAddress+"/db/query", bytes.NewBuffer(payload))
	// req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", at))
	req.Header.Set("Content-Type", "application/json")
	rsp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	result, _ := io.ReadAll(rsp.Body)
	rsp.Body.Close()
	require.Equal(t, http.StatusBadRequest, rsp.StatusCode, string(result))

	resultObj := map[string]any{}
	err = json.Unmarshal(result, &resultObj)
	require.NoError(t, err)
	require.Equal(t, false, resultObj["success"])
	require.Contains(t, resultObj["reason"], "bind parameter must be scalar")
}

func TestHttpQueryUnsupportedContentTypeForm(t *testing.T) {
	payload := []byte(`{"q":"select (min(min_time)),(max(max_time)) from v$EXAMPLE_stat where name = 'temp'"}`)

	req, _ := http.NewRequest(http.MethodPost, httpServerAddress+"/db/query", bytes.NewBuffer(payload))
	// req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", at))
	req.Header.Set("Content-Type", "text/plain")
	rsp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	result, _ := io.ReadAll(rsp.Body)
	rsp.Body.Close()
	require.Equal(t, http.StatusUnsupportedMediaType, rsp.StatusCode, string(result))

	resultObj := map[string]any{}
	err = json.Unmarshal(result, &resultObj)
	require.NoError(t, err)
	require.Equal(t, false, resultObj["success"])
	require.Contains(t, resultObj["reason"], "unsupported content-type: text/plain")
}

func TestHttpQueryUnsupportedTimeLocation(t *testing.T) {
	payload := url.Values{}
	payload.Set("q", `select (min(min_time)),(max(max_time)) from v$EXAMPLE_stat where name = 'temp'`)
	payload.Set("tz", "Invalid/Location")

	req, _ := http.NewRequest(http.MethodGet, httpServerAddress+"/db/query?"+payload.Encode(), nil)
	rsp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	result, _ := io.ReadAll(rsp.Body)
	rsp.Body.Close()
	require.Equal(t, http.StatusBadRequest, rsp.StatusCode, string(result))

	resultObj := map[string]any{}
	err = json.Unmarshal(result, &resultObj)
	require.NoError(t, err)
	require.Equal(t, false, resultObj["success"])
	require.Contains(t, resultObj["reason"], "unknown time zone Invalid/Location")
}

func TestHttpQueryCompressedResponse(t *testing.T) {
	payload := url.Values{}
	payload.Set("q", `select * from EXAMPLE where name = 'temp' limit 10`)
	payload.Set("format", "csv")
	payload.Set("compress", "gzip")

	req, _ := http.NewRequest(http.MethodGet, httpServerAddress+"/db/query?"+payload.Encode(), nil)
	rsp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rsp.StatusCode)
	require.Equal(t, "text/csv; charset=utf-8", rsp.Header.Get("Content-Type"))
	require.True(t, rsp.Uncompressed)
	decompressedData, _ := io.ReadAll(rsp.Body)
	rsp.Body.Close()
	require.Equal(t, strings.Join([]string{
		"NAME,TIME,VALUE",
		"temp,1705291859000000000,3.14",
		"",
		"",
	}, "\n"), string(decompressedData))
}

func TestHttpQueryEncrypted(t *testing.T) {
	sql := `SELECT count(*) from example`
	encrypted := "ENC:" + util.MustEncryptString(sql, "AES", "1234567890abcdef")

	req, _ := http.NewRequest(http.MethodGet, httpServerAddress+"/db/query?q="+url.QueryEscape(encrypted)+"&format=box", nil)
	rsp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	result, _ := io.ReadAll(rsp.Body)
	rsp.Body.Close()
	require.Equal(t, http.StatusOK, rsp.StatusCode, string(result))
	require.Equal(t, "text/plain", rsp.Header.Get("Content-Type"))
	require.Equal(t, strings.Join([]string{
		"+----------+",
		"| COUNT(*) |",
		"+----------+",
		"| 11       |",
		"+----------+",
		"",
	}, "\n"), string(result))

	encrypted = "ENC:" + util.MustEncryptString(sql, "AES", "wrong_7890abcdef")
	req, _ = http.NewRequest(http.MethodGet, httpServerAddress+"/db/query?q="+url.QueryEscape(encrypted)+"&format=box", nil)
	rsp, err = http.DefaultClient.Do(req)
	require.NoError(t, err)
	result, _ = io.ReadAll(rsp.Body)
	rsp.Body.Close()

	require.Equal(t, http.StatusBadRequest, rsp.StatusCode, string(result))
	require.Equal(t, "application/json; charset=utf-8", rsp.Header.Get("Content-Type"))
	resultObj := map[string]any{}
	err = json.Unmarshal(result, &resultObj)
	delete(resultObj, "elapse")
	require.NoError(t, err)
	require.EqualValues(t, map[string]any{
		"success": false, "reason": "decrypt sql fail, invalid padding",
	}, resultObj)
}

func TestHttpQueryImageFileUploadAndWatch(t *testing.T) {
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
				require.Equal(t, filepath.Join(projRootDir, "tmp", "test", "machbase_home", "store"), gjson.Get(extData, "SD").String(), extData)
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
							"SD":"`+filepath.Join(projRootDir, "tmp", "test", "machbase_home", "store")+`",
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

func TestHandleTqlQueryExec(t *testing.T) {
	at, _, err := jwtLogin("sys", "manager")
	require.NoError(t, err)

	t.Run("token query param authorizes request and delegates to tql handler", func(t *testing.T) {
		query := url.Values{}
		query.Set(TQL_SCRIPT_PARAM, "FAKE(linspace(0,1,2))\nCSV()")
		query.Set(TQL_TOKEN_PARAM, at)

		req, err := http.NewRequest(http.MethodGet, httpServerAddress+"/web/api/tql-exec?"+query.Encode(), nil)
		require.NoError(t, err)

		rsp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer rsp.Body.Close()

		body, err := io.ReadAll(rsp.Body)
		require.NoError(t, err)

		require.Equal(t, http.StatusOK, rsp.StatusCode)
		require.Equal(t, "text/csv; charset=utf-8", rsp.Header.Get("Content-Type"))
		require.Equal(t, strings.Join([]string{"0", "1", "\n"}, "\n"), string(body))
	})

	t.Run("invalid token aborts before tql execution", func(t *testing.T) {
		query := url.Values{}
		query.Set(TQL_SCRIPT_PARAM, "FAKE(linspace(0,1,2))\nCSV()")
		query.Set(TQL_TOKEN_PARAM, "not-a-valid-token")

		req, err := http.NewRequest(http.MethodGet, httpServerAddress+"/web/api/tql-exec?"+query.Encode(), nil)
		require.NoError(t, err)

		rsp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer rsp.Body.Close()

		body, err := io.ReadAll(rsp.Body)
		require.NoError(t, err)

		require.Equal(t, http.StatusUnauthorized, rsp.StatusCode)
		require.Contains(t, rsp.Header.Get("Content-Type"), "application/json")
		require.Contains(t, string(body), `"success":false`)
		require.Contains(t, string(body), "reason")
		require.NotContains(t, string(body), "columns")
	})
}

func TestHandleTqlQuery(t *testing.T) {
	svr := newTestHTTPServer(t)

	t.Run("get request without script returns bad request", func(t *testing.T) {
		ctx, writer := newTestHTTPContext(http.MethodGet, "/web/api/tql", nil)

		svr.handleTqlQuery(ctx)

		require.Equal(t, http.StatusBadRequest, writer.Code)
		require.Contains(t, writer.Body.String(), `"success":false`)
		require.Contains(t, writer.Body.String(), "script not found")
	})

	t.Run("post body script executes successfully", func(t *testing.T) {
		ctx, writer := newTestHTTPContext(http.MethodPost, "/web/api/tql", []byte("FAKE(linspace(0,1,2))\nCSV()"))

		svr.handleTqlQuery(ctx)

		require.Equal(t, http.StatusOK, writer.Code)
		require.Equal(t, "text/csv; charset=utf-8", writer.Header().Get("Content-Type"))
		require.Equal(t, strings.Join([]string{"0", "1", "\n"}, "\n"), writer.Body.String())
	})

	t.Run("post query script accepts payload", func(t *testing.T) {
		script := "CSV(payload())\nCSV()"
		target := "/web/api/tql?$=" + url.QueryEscape(script)
		ctx, writer := newTestHTTPContext(http.MethodPost, target, []byte("a,1\nb,2\n"))
		ctx.Request.Header.Set("Content-Type", "text/csv")

		svr.handleTqlQuery(ctx)

		require.Equal(t, http.StatusOK, writer.Code)
		require.Equal(t, "text/csv; charset=utf-8", writer.Header().Get("Content-Type"))
		require.Equal(t, strings.Join([]string{"a,1", "b,2", "\n"}, "\n"), writer.Body.String())
	})

	t.Run("unsupported method returns method not allowed", func(t *testing.T) {
		ctx, writer := newTestHTTPContext(http.MethodPut, "/web/api/tql?$="+url.QueryEscape("FAKE(linspace(0,1,2))\nCSV()"), nil)

		svr.handleTqlQuery(ctx)

		require.Equal(t, http.StatusMethodNotAllowed, writer.Code)
		require.Contains(t, writer.Body.String(), `"success":false`)
		require.Contains(t, writer.Body.String(), "unsupported method")
	})

	t.Run("compile error returns bad request", func(t *testing.T) {
		ctx, writer := newTestHTTPContext(http.MethodGet, "/web/api/tql?$="+url.QueryEscape("FAKE("), nil)

		svr.handleTqlQuery(ctx)

		require.Equal(t, http.StatusBadRequest, writer.Code)
		require.Contains(t, writer.Body.String(), `"success":false`)
		require.Contains(t, writer.Body.String(), "reason")
	})
}

func TestHandleTqlFile(t *testing.T) {
	oldDefault := ssfs.Default()
	ssfs.SetDefault(httpServer.serverFs)
	t.Cleanup(func() {
		ssfs.SetDefault(oldDefault)
	})

	writeServerFile := func(t *testing.T, path string, content []byte) {
		t.Helper()
		require.NoError(t, httpServer.serverFs.Set(path, content))
		t.Cleanup(func() {
			_ = httpServer.serverFs.Remove(path)
		})
	}

	doRequest := func(t *testing.T, method, target string, body []byte, headers map[string]string) *http.Response {
		t.Helper()
		req, err := http.NewRequest(method, httpServerAddress+target, bytes.NewReader(body))
		require.NoError(t, err)
		for key, value := range headers {
			req.Header.Set(key, value)
		}
		rsp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		return rsp
	}

	t.Run("non tql public path redirects", func(t *testing.T) {
		noRedirectClient := &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}

		req, err := http.NewRequest(http.MethodGet, httpServerAddress+"/db/tql/public/redirect-policy.txt", nil)
		require.NoError(t, err)

		rsp, err := noRedirectClient.Do(req)
		require.NoError(t, err)
		defer rsp.Body.Close()

		require.Equal(t, http.StatusFound, rsp.StatusCode)
		require.Equal(t, "/public/redirect-policy.txt", rsp.Header.Get("Location"))
	})

	t.Run("non tql static file returns content", func(t *testing.T) {
		const staticPath = "/query_test_static.txt"
		writeServerFile(t, staticPath, []byte("hello from static file"))

		rsp := doRequest(t, http.MethodGet, "/db/tql"+staticPath, nil, nil)
		defer rsp.Body.Close()

		body, err := io.ReadAll(rsp.Body)
		require.NoError(t, err)

		require.Equal(t, http.StatusOK, rsp.StatusCode)
		require.Equal(t, "text/plain", rsp.Header.Get("Content-Type"))
		require.Equal(t, "hello from static file", string(body))
	})

	t.Run("missing tql file returns not found", func(t *testing.T) {
		rsp := doRequest(t, http.MethodGet, "/db/tql/query_test_missing.tql", nil, nil)
		defer rsp.Body.Close()

		body, err := io.ReadAll(rsp.Body)
		require.NoError(t, err)

		require.Equal(t, http.StatusNotFound, rsp.StatusCode)
		require.Contains(t, string(body), `"success":false`)
		require.Contains(t, string(body), "not found")
	})

	t.Run("compile failure returns internal server error", func(t *testing.T) {
		const brokenPath = "/query_test_broken.tql"
		writeServerFile(t, brokenPath, []byte("FAKE("))

		rsp := doRequest(t, http.MethodGet, "/db/tql"+brokenPath, nil, nil)
		defer rsp.Body.Close()

		body, err := io.ReadAll(rsp.Body)
		require.NoError(t, err)

		require.Equal(t, http.StatusInternalServerError, rsp.StatusCode)
		require.Contains(t, string(body), `"success":false`)
		require.Contains(t, string(body), "reason")
	})

	t.Run("json output header changes response format", func(t *testing.T) {
		const scriptPath = "/query_test_output.tql"
		writeServerFile(t, scriptPath, []byte("FAKE(linspace(0,360,5))\nMAPVALUE(1, sin((value(0)/180)*PI))\nCHART()"))

		rsp := doRequest(t, http.MethodGet, "/db/tql"+scriptPath, nil, map[string]string{TqlHeaderChartOutput: "json"})
		defer rsp.Body.Close()

		body, err := io.ReadAll(rsp.Body)
		require.NoError(t, err)

		require.Equal(t, http.StatusOK, rsp.StatusCode)
		require.NotEmpty(t, rsp.Header.Get("Content-Type"))
		require.Equal(t, "echarts", rsp.Header.Get(TqlHeaderChartType))
		require.Contains(t, string(body), `"chartID"`)
		require.Contains(t, string(body), `"jsAssets"`)
		require.Contains(t, string(body), `"jsCodeAssets"`)
	})
}

// TestHandleTqlFileExecUserScope guards handleTqlFile (machbase/neo#1468): a
// stored .tql file executed via /web/api/tql/*path must run as the JWT's own
// user (task.SetConsole), not always "sys".
func TestHandleTqlFileExecUserScope(t *testing.T) {
	oldDefault := ssfs.Default()
	ssfs.SetDefault(httpServer.serverFs)
	t.Cleanup(func() { ssfs.SetDefault(oldDefault) })

	username := fmt.Sprintf("tqlfile_scope_user_%d", time.Now().UnixNano())
	table := "OWN_TQLFILE_TBL"

	sysConn, err := spi.Connect(t.Context(), "sys")
	require.NoError(t, err)
	_, err = sysConn.ExecContext(t.Context(), fmt.Sprintf("CREATE USER %s IDENTIFIED BY 'password'", username))
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

	ownConn, err := spi.Connect(t.Context(), username)
	require.NoError(t, err)
	_, err = ownConn.ExecContext(t.Context(), fmt.Sprintf(
		"CREATE TAG TABLE %s (NAME VARCHAR(40) PRIMARY KEY, TIME DATETIME BASETIME, VALUE DOUBLE SUMMARIZED)", table))
	require.NoError(t, err)
	ownConn.Close()

	const scriptPath = "/query_test_scope.tql"
	require.NoError(t, httpServer.serverFs.Set(scriptPath, []byte(fmt.Sprintf(
		"FAKE(json({[1]}))\nSQL(\"insert into %s(NAME,TIME,VALUE) values(current_user(), now, 1)\")\n", table))))
	t.Cleanup(func() { _ = httpServer.serverFs.Remove(scriptPath) })

	accessToken, _, err := jwtLogin(username, "password")
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodGet, httpServerAddress+"/web/api/tql"+scriptPath, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	rsp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer rsp.Body.Close()
	body, err := io.ReadAll(rsp.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rsp.StatusCode, string(body))

	checkConn, err := spi.Connect(t.Context(), "sys")
	require.NoError(t, err)
	defer checkConn.Close()
	var name string
	require.NoError(t, checkConn.QueryRowContext(t.Context(),
		fmt.Sprintf("select name from %s.%s", username, table)).Scan(&name))
	require.Equal(t, strings.ToUpper(username), name)
}

func TestQueryBinaryFormat(t *testing.T) {
	// create table
	sql := "CREATE TAG TABLE IF NOT EXISTS test_bin (name varchar(40) primary key, time datetime base time, value binary)"
	req, _ := http.NewRequest(http.MethodGet, httpServerAddress+"/db/query?q="+url.QueryEscape(sql), nil)
	rsp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rsp.StatusCode)
	rsp.Body.Close()

	// insert data
	sql = "INSERT INTO test_bin VALUES('name', now, '0x0102A0B0')"
	req, _ = http.NewRequest(http.MethodGet, httpServerAddress+"/db/query?q="+url.QueryEscape(sql), nil)
	rsp, err = http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rsp.StatusCode)
	rsp.Body.Close()

	// drop table
	defer func() {
		sql := "DROP TABLE test_bin"
		req, _ := http.NewRequest(http.MethodGet, httpServerAddress+"/db/query?q="+url.QueryEscape(sql), nil)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		resp.Body.Close()
	}()

	tests := []struct {
		name      string
		format    string
		binformat string
		expect    string
	}{
		{
			name:   "json_default",
			format: "json",
			expect: `["name","0x0102a0b0"]`,
		},
		{
			name:      "json_base64",
			format:    "json",
			binformat: "base64",
			expect:    `["name","AQKgsA=="]`,
		},
		{
			name:   "csv_default",
			format: "csv",
			expect: "name,0x0102a0b0\n",
		},
		{
			name:      "csv_base64",
			format:    "csv",
			binformat: "base64",
			expect:    "name,AQKgsA==\n",
		},
		{
			name:   "ndjson_defaul",
			format: "ndjson",
			expect: `{"NAME":"name","VALUE":"0x0102a0b0"}` + "\n",
		},
		{
			name:      "ndjson_base64",
			format:    "ndjson",
			binformat: "base64",
			expect:    `{"NAME":"name","VALUE":"AQKgsA=="}` + "\n",
		},
		{
			name:   "box_default",
			format: "box",
			expect: "| name | 0x0102a0b0 |\n",
		},
		{
			name:      "box_base64",
			format:    "box",
			binformat: "base64",
			expect:    "| name | AQKgsA== |\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql = "SELECT NAME, VALUE FROM test_bin"
			query := httpServerAddress + "/db/query?q=" + url.QueryEscape(sql)
			if tt.format != "" {
				query += "&format=" + url.QueryEscape(tt.format)
			}
			if tt.binformat != "" {
				query += "&binaryformat=" + url.QueryEscape(tt.binformat)
			}
			req, _ = http.NewRequest(http.MethodGet, query, nil)
			rsp, err = http.DefaultClient.Do(req)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, rsp.StatusCode)
			body, err := io.ReadAll(rsp.Body)
			require.NoError(t, err)
			require.Contains(t, string(body), tt.expect)
			rsp.Body.Close()
		})
	}
}
