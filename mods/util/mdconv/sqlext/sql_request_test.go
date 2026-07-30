package sqlext

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/machbase/neo-server/v8/mods/util"
	"github.com/machbase/neo-server/v8/spi"
	"github.com/stretchr/testify/require"
	"github.com/yuin/goldmark/ast"
)

func TestDecodeQueryRequestJSONNormalizesParams(t *testing.T) {
	req := &QueryRequest{}
	err := req.DecodeJSON(strings.NewReader(`{"q":"select * from t where a = ?","p":[1,1.5,true,"neo"],"binaryformat":"base64"}`))
	require.NoError(t, err)
	require.Equal(t, "select * from t where a = ?", req.SqlText)
	require.Equal(t, []any{1, 1.5, true, "neo"}, req.Params)
	require.Equal(t, "base64", req.BinaryFormat)
}

func TestDecodeQueryRequestJSONRejectsCompositeParam(t *testing.T) {
	req := &QueryRequest{}
	err := req.DecodeJSON(strings.NewReader(`{"q":"select * from t","p":[{"nested":1}]}`))
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid p")
	require.Contains(t, err.Error(), "scalar")
}

func TestParseQueryParams(t *testing.T) {
	params, err := ParseQueryParams("   ")
	require.NoError(t, err)
	require.Nil(t, params)

	params, err = ParseQueryParams(`[1,2.5,false,"x"]`)
	require.NoError(t, err)
	require.Equal(t, []any{1, 2.5, false, "x"}, params)

	_, err = ParseQueryParams(`{"not":"an array"}`)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid p")
}

type stubRows struct {
	columns     []string
	columnTypes []*sql.ColumnType
	values      []any
	multiValues [][]any
	index       int
}

func (r *stubRows) Columns() ([]string, error) {
	return r.columns, nil
}

func (r *stubRows) ColumnTypes() ([]*sql.ColumnType, error) {
	return r.columnTypes, nil
}

func (r *stubRows) Close() error { return nil }

func (r *stubRows) Next() bool {
	if len(r.multiValues) > 0 {
		if r.index >= len(r.multiValues) {
			return false
		}
		r.index++
		return true
	}
	if r.index >= 1 {
		return false
	}
	r.index++
	return true
}

func (r *stubRows) Scan(dest ...any) error {
	if len(r.multiValues) > 0 {
		rowValues := r.multiValues[r.index-1]
		for i, v := range rowValues {
			if i >= len(dest) {
				break
			}
			if p, ok := dest[i].(*any); ok {
				*p = v
				continue
			}
			if p, ok := dest[i].(*string); ok {
				*p = v.(string)
				continue
			}
			dest[i] = v
		}
		return nil
	}
	if len(r.values) == 0 {
		return nil
	}
	for i, v := range r.values {
		if i >= len(dest) {
			break
		}
		if p, ok := dest[i].(*any); ok {
			*p = v
			continue
		}
		if p, ok := dest[i].(*string); ok {
			*p = v.(string)
			continue
		}
		dest[i] = v
	}
	return nil
}

func TestExecuteUsesTQLAPIWhenConnUnset(t *testing.T) {
	var gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/db/tql", r.URL.Path)
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		gotBody = string(body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"ok":true}`)
	}))
	defer server.Close()

	prevBaseURL := tqlBaseURL
	tqlBaseURL = server.URL
	t.Cleanup(func() { tqlBaseURL = prevBaseURL })

	req := &QueryRequest{SqlText: "select 1", Output: "json", Preview: 3}
	var out bytes.Buffer
	err := req.Execute(context.Background(), &out, nil)
	require.NoError(t, err)
	require.Contains(t, gotBody, "SQL(")
	require.Contains(t, out.String(), `"ok":true`)
}

func TestExecuteBuildsTQLRequestBody(t *testing.T) {
	prevDoRequest := tqlDoRequest
	t.Cleanup(func() { tqlDoRequest = prevDoRequest })

	tqlDoRequest = func(req *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(req.Body)
		require.NoError(t, err)
		got := string(body)
		require.Contains(t, got, "SQL(")
		require.Contains(t, got, "TAKE(3)")
		require.Contains(t, got, "JSON(")
		require.Contains(t, got, "timeformat('2006-01-02 15:04:05')")
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"ok":true}`)), Header: make(http.Header)}, nil
	}

	req := &QueryRequest{SqlText: "select 1", Output: "json", Preview: 3, TimeFormat: "2006-01-02 15:04:05", Timezone: "Asia/Seoul"}
	var out bytes.Buffer
	err := req.Execute(context.Background(), &out, nil)
	require.NoError(t, err)
	require.Contains(t, out.String(), `"ok":true`)
}

func TestExecuteUsesTQLAPIAndPreservesQueryText(t *testing.T) {
	prevDoRequest := tqlDoRequest
	t.Cleanup(func() { tqlDoRequest = prevDoRequest })

	var gotBody string
	tqlDoRequest = func(req *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(req.Body)
		require.NoError(t, err)
		gotBody = string(body)
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"ok":true}`)), Header: make(http.Header)}, nil
	}

	req := &QueryRequest{SqlText: "select 1", Output: "json", Preview: 2}
	var out bytes.Buffer
	err := req.Execute(context.Background(), &out, nil)
	require.NoError(t, err)
	require.Contains(t, gotBody, "{<<END_OF_SQL")
	require.Contains(t, gotBody, "select 1")
	require.Contains(t, gotBody, "TAKE(2)")
}

func TestNormalizeQueryParamValue(t *testing.T) {
	tests := []struct {
		name    string
		input   any
		want    any
		wantErr string
	}{
		{name: "nil", input: nil, want: nil},
		{name: "string", input: "neo", want: "neo"},
		{name: "bool", input: true, want: true},
		{name: "int", input: 3, want: 3},
		{name: "float", input: 1.25, want: 1.25},
		{name: "json integer", input: json.Number("42"), want: int64(42)},
		{name: "json float", input: json.Number("3.14"), want: 3.14},
		{name: "invalid json number", input: json.Number("nope"), wantErr: "invalid syntax"},
		{name: "slice", input: []any{"x"}, wantErr: "scalar"},
		{name: "map", input: map[string]any{"x": 1}, wantErr: "scalar"},
		{name: "struct", input: struct{ Value string }{Value: "x"}, wantErr: "unsupported bind parameter type"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NormalizeQueryParamValue(tc.input)
			if tc.wantErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestDecodeQueryParsesURLValuesAndPostForm(t *testing.T) {
	req := &QueryRequest{}
	values := url.Values{
		"q":                  {"select 1"},
		"preview":            {"7"},
		"timeformat":         {"2006-01-02 15:04:05"},
		"tz":                 {"Asia/Seoul"},
		"binaryformat":       {"base64"},
		"rowsFlattern":       {"true"},
		"rowsArray":          {"true"},
		"transpose":          {"true"},
		"precision":          {"3"},
		"rownum":             {"true"},
		"heading":            {"false"},
		"header":             {"false"},
		"delimiter":          {"|"},
		"boxStyle":           {"rounded"},
		"boxSeparateColumns": {"true"},
		"boxDrawBorder":      {"false"},
		"output":             {"json"},
		"format":             {"csv"},
		"replyTo":            {"neo"},
		"database":           {"sys"},
		"user":               {"u"},
		"password":           {"p"},
		"authKey":            {"k"},
		"noCache":            {"true"},
		"noMeta":             {"true"},
		"noFormat":           {"true"},
		"cache":              {"true"},
		"truncate":           {"9"},
		"p":                  {"[1,2]"},
	}

	require.NoError(t, req.DecodeQuery(values))
	require.Equal(t, "select 1", req.SqlText)
	require.Equal(t, 7, req.Preview)
	require.Equal(t, "Asia/Seoul", req.Timezone)
	require.True(t, req.RowsFlattern)
	require.True(t, req.RowsArray)
	require.True(t, req.Transpose)
	require.Equal(t, 3, req.Precision)
	require.True(t, req.RowNum)
	require.False(t, req.Heading)
	require.False(t, req.Header)
	require.Equal(t, "|", req.Delimiter)
	require.True(t, req.BoxSeparateCols)
	require.False(t, req.BoxDrawBorder)
	require.Equal(t, "json", req.Output)
	require.Equal(t, "csv", req.Format)
	require.True(t, req.NoCache)
	require.True(t, req.NoMeta)
	require.True(t, req.NoFormat)
	require.True(t, req.Cache)
	require.Equal(t, 9, req.Truncate)
	require.Equal(t, []any{1, 2}, req.Params)

	postReq := &QueryRequest{}
	require.NoError(t, postReq.DecodePostForm(values))
	require.Equal(t, "select 1", postReq.SqlText)
}

func TestQueryRequestSettersCoverAccessorPaths(t *testing.T) {
	req := NewQueryRequest()
	req.SetQueryText("select 1")
	req.SetParams([]any{1, "neo"})
	req.SetOutput("json")
	req.SetFormat("csv")
	req.SetPreview(7)
	req.SetTimeFormat("2006-01-02 15:04:05")
	req.SetTimezone("Asia/Seoul")
	req.SetBinaryFormat("base64")
	req.SetRowsFlattern(true)
	req.SetRowsArray(true)
	req.SetTranspose(true)
	req.SetPrecision(3)
	req.SetRowNum(true)
	req.SetHeading(false)
	req.SetHeader(false)
	req.SetDelimiter("|")
	req.SetBoxStyle("rounded")
	req.SetBoxSeparateCols(true)
	req.SetBoxDrawBorder(false)
	req.SetReplyTo("neo")
	req.SetDatabase("sys")
	req.SetUser("u")
	req.SetPassword("p")
	req.SetAuthKey("k")
	req.SetNoCache(true)
	req.SetNoMeta(true)

	require.Equal(t, "select 1", req.GetQueryText())
	require.Equal(t, []any{1, "neo"}, req.GetParams())
	require.Equal(t, "json", req.Output)
	require.Equal(t, "csv", req.Format)
	require.Equal(t, 7, req.Preview)
	require.Equal(t, "2006-01-02 15:04:05", req.TimeFormat)
	require.Equal(t, "Asia/Seoul", req.Timezone)
	require.Equal(t, "base64", req.BinaryFormat)
	require.True(t, req.RowsFlattern)
	require.True(t, req.RowsArray)
	require.True(t, req.Transpose)
	require.Equal(t, 3, req.Precision)
	require.True(t, req.RowNum)
	require.False(t, req.Heading)
	require.False(t, req.Header)
	require.Equal(t, "|", req.Delimiter)
	require.Equal(t, "rounded", req.BoxStyle)
	require.True(t, req.BoxSeparateCols)
	require.False(t, req.BoxDrawBorder)
	require.Equal(t, "neo", req.ReplyTo)
	require.Equal(t, "sys", req.Database)
	require.Equal(t, "u", req.User)
	require.Equal(t, "p", req.Password)
	require.Equal(t, "k", req.AuthKey)
	require.True(t, req.NoCache)
	require.True(t, req.NoMeta)

	hook := &QueryHook{SetUserMessage: func(string) {}}
	req.AddHook(hook)
	require.NotNil(t, req.Hook.SetUserMessage)
}

func TestOutputFormattingHelpersAndRenderRowsByFormat(t *testing.T) {
	require.Equal(t, "box", normalizeRequestedOutputFormat("", ""))
	require.Equal(t, "json", normalizeRequestedOutputFormat(" JSON ", ""))
	require.Equal(t, "csv", normalizeRequestedOutputFormat("", "CSV"))
	require.Equal(t, "application/json", contentTypeForFormat("json"))
	require.Equal(t, "application/x-ndjson", contentTypeForFormat("ndjson"))
	require.Equal(t, "text/csv; charset=utf-8", contentTypeForFormat("csv"))
	require.Equal(t, "text/markdown", contentTypeForFormat("markdown"))
	require.Equal(t, "text/html", contentTypeForFormat("html"))
	require.Equal(t, "text/plain", contentTypeForFormat("text"))
	require.Equal(t, "text/plain", contentTypeForFormat("unknown"))
	require.False(t, shouldAppendTimezoneToHeader("ns"))
	require.True(t, shouldAppendTimezoneToHeader("2006-01-02 15:04:05"))
	require.Equal(t, "ts", formatTableHeaderLabel("ts", "string", "ns", ""))
	require.Equal(t, "ts(Asia/Seoul)", formatTableHeaderLabel("ts", "datetime", "2006-01-02 15:04:05", "Asia/Seoul"))
	require.Equal(t, "ts", formatTableHeaderLabel("ts", "datetime", "ns", "Asia/Seoul"))

	ts := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	payload := []byte{0x01, 0x02}
	rows := &stubRows{columns: []string{"ts", "payload"}, values: []any{ts, payload}}

	var jsonOut bytes.Buffer
	_, err := renderRowsByFormat(&jsonOut, "json", true, rows, []string{"ts", "payload"}, nil, "2006-01-02 15:04:05", "UTC", util.NewTimeFormatter(util.Timeformat("2006-01-02 15:04:05"), util.TimeLocation(time.UTC)), util.NewBinaryFormatter("hex"), spi.SQLStatementTypeSelect, 10)
	require.NoError(t, err)
	require.Contains(t, jsonOut.String(), "2024-01-02 03:04:05")

	rows.index = 0
	var ndjsonOut bytes.Buffer
	_, err = renderRowsByFormat(&ndjsonOut, "ndjson", true, rows, []string{"ts", "payload"}, nil, "2006-01-02 15:04:05", "UTC", util.NewTimeFormatter(util.Timeformat("2006-01-02 15:04:05"), util.TimeLocation(time.UTC)), util.NewBinaryFormatter("hex"), spi.SQLStatementTypeSelect, 10)
	require.NoError(t, err)
	require.Contains(t, ndjsonOut.String(), "0x0102")

	rows.index = 0
	var csvOut bytes.Buffer
	_, err = renderRowsByFormat(&csvOut, "csv", true, rows, []string{"ts", "payload"}, nil, "2006-01-02 15:04:05", "UTC", util.NewTimeFormatter(util.Timeformat("2006-01-02 15:04:05"), util.TimeLocation(time.UTC)), util.NewBinaryFormatter("hex"), spi.SQLStatementTypeSelect, 10)
	require.NoError(t, err)
	require.Contains(t, csvOut.String(), "2024-01-02 03:04:05")

	rows.index = 0
	var tableOut bytes.Buffer
	_, err = renderRowsByFormat(&tableOut, "table", true, rows, []string{"ts", "payload"}, nil, "2006-01-02 15:04:05", "UTC", util.NewTimeFormatter(util.Timeformat("2006-01-02 15:04:05"), util.TimeLocation(time.UTC)), util.NewBinaryFormatter("hex"), spi.SQLStatementTypeSelect, 10)
	require.NoError(t, err)
	require.Contains(t, tableOut.String(), "TS")
	require.Contains(t, tableOut.String(), "0x0102")

	var noneOut bytes.Buffer
	_, err = renderRowsByFormat(&noneOut, "none", true, rows, nil, nil, "", "", nil, nil, spi.SQLStatementTypeSelect, 10)
	require.NoError(t, err)
	require.Empty(t, noneOut.String())
}

func TestPreviewSourceRunSQLAndRendererHelpers(t *testing.T) {
	require.Equal(t, "", previewSource("select 1;\nselect 2;", Options{Source: "hide"}))
	require.Equal(t, "select 1;\nselect 2;", previewSource("select 1;\nselect 2;", Options{Source: "all"}))
	require.Equal(t, "select 2;", previewSource("select 1;\nselect 2;", Options{Source: "2"}))
	require.Equal(t, "select 2;", previewSource("select 1;\nselect 2;", Options{Source: "2-2"}))

	prevDoRequest := tqlDoRequest
	t.Cleanup(func() { tqlDoRequest = prevDoRequest })
	tqlDoRequest = func(req *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(req.Body)
		require.NoError(t, err)
		require.Contains(t, string(body), "SQL(")
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"data":{"columns":["value"],"types":["string"],"rows":[["neo"]]},"success":true,"reason":"a row selected.","elapse":"0s"}`)),
			Header:     make(http.Header),
		}, nil
	}
	var out strings.Builder
	require.NoError(t, runSQL("select 1", Options{Format: "json", Timeout: time.Millisecond}, &out))
	require.Contains(t, out.String(), "neo")

	body := "a,b\n1,2\n"
	require.Equal(t, byte(','), detectCSVDelimiter(body))
	require.Equal(t, []string{"a", "b\n1", "2\n"}, mustSplitCSVFields(t, body, ','))
	require.Equal(t, []string{"1", "2"}, mustSplitCSVFields(t, "1,2", ','))
	var buf testBufWriter
	renderer := &HTMLRenderer{}
	status, err := renderer.Render(&buf, nil, &Block{Source: "select 1", Options: Options{Execute: true, Format: "csv"}, Output: body}, true)
	require.NoError(t, err)
	require.Equal(t, ast.WalkContinue, status)
	require.Contains(t, buf.String(), "sqlext-result")
}

type testBufWriter struct {
	strings.Builder
}

func (w *testBufWriter) Write(p []byte) (int, error) {
	return w.Builder.Write(p)
}

func (w *testBufWriter) WriteString(s string) (int, error) {
	return w.Builder.WriteString(s)
}

func (w *testBufWriter) Available() int {
	return 1024 * 1024
}

func (w *testBufWriter) Buffered() int {
	return w.Len()
}

func (w *testBufWriter) Reset() {
	w.Builder.Reset()
}

func (w *testBufWriter) Flush() error {
	return nil
}

func mustSplitCSVFields(t *testing.T, line string, delim byte) []string {
	t.Helper()
	fields, ok := splitCSVFields(line, delim)
	require.True(t, ok)
	return fields
}
