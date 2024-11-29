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
	"github.com/stretchr/testify/require"
)

func TestHttpQuery(t *testing.T) {
	tests := []struct {
		name        string
		sqlText     string
		params      map[string]string
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
			params: map[string]string{
				"transpose": "true",
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
			params: map[string]string{
				"rowsFlatten": "true",
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
			params: map[string]string{
				"rowsArray": "true",
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
			var params = "q=" + url.QueryEscape(tc.sqlText)
			for k, v := range tc.params {
				params = params + "&" + k + "=" + url.QueryEscape(v)
			}
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
					params[k] = v == "true"
				default:
					params[k] = v
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
