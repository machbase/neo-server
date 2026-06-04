package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/machbase/neo-server/v8/mods/util"
	"github.com/machbase/neo-server/v8/mods/util/ssfs"
	"github.com/stretchr/testify/require"
)

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
		{
			name:        "select_between_sub_query",
			sqlText:     "ENC:" + util.MustEncryptString(`SELECT count(*) from example`, "AES", "1234567890abcdef"),
			params:      url.Values{"format": []string{"box"}},
			contentType: "text/plain",
			expect: []string{
				"+----------+",
				"| COUNT(*) |",
				"+----------+",
				"| 11       |",
				"+----------+",
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
					var bindParams []any
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
					var bindParams []any
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
				case []any:
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
		"success": false, "reason": "empty sql",
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
