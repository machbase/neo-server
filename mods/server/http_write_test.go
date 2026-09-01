package server

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/machbase/neo-server/v8/spi"
	"github.com/stretchr/testify/require"
)

type multipartTestFile struct {
	fieldName   string
	fileName    string
	contentType string
	content     []byte
	headers     map[string]string
}

func buildMultipartTestRequest(target string, fields map[string]string, files ...multipartTestFile) (*http.Request, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	for key, value := range fields {
		if err := writer.WriteField(key, value); err != nil {
			return nil, err
		}
	}

	for _, file := range files {
		h := make(textproto.MIMEHeader)
		h.Set("Content-Disposition", fmt.Sprintf(`form-data; name=%q; filename=%q`, file.fieldName, file.fileName))
		if file.contentType != "" {
			h.Set("Content-Type", file.contentType)
		}
		for key, value := range file.headers {
			h.Set(key, value)
		}
		part, err := writer.CreatePart(h)
		if err != nil {
			return nil, err
		}
		if _, err := part.Write(file.content); err != nil {
			return nil, err
		}
	}

	if err := writer.Close(); err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, target, &body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req, nil
}

func TestResolveExecUser(t *testing.T) {
	svr := newTestHTTPServer(t)

	t.Run("api_token_authenticated_with_user", func(t *testing.T) {
		ctx, _ := newTestHTTPContext(http.MethodPost, "/db/write/t", nil)
		ctx.Set("api-token-authenticated", true)
		ctx.Set("api-token-user", "alice")
		execUser, errReason := svr.resolveExecUser(ctx)
		require.Empty(t, errReason)
		require.Equal(t, "alice", execUser)
	})

	t.Run("api_token_authenticated_without_user", func(t *testing.T) {
		ctx, _ := newTestHTTPContext(http.MethodPost, "/db/write/t", nil)
		ctx.Set("api-token-authenticated", true)
		execUser, errReason := svr.resolveExecUser(ctx)
		require.Empty(t, execUser)
		require.Equal(t, "authorization user is missing", errReason)
	})

	t.Run("not_authenticated", func(t *testing.T) {
		ctx, _ := newTestHTTPContext(http.MethodPost, "/db/write/t", nil)
		execUser, errReason := svr.resolveExecUser(ctx)
		require.Empty(t, errReason)
		require.Empty(t, execUser)
	})
}

// TestHttpWriteWithDatabaseParam verifies /db/write "db" parameter support for
// multiple-database writes (machbase/neo#1484): insert and append both accept a
// request-scoped "db" and write to the correct database without leaking state.
func TestHttpWriteWithDatabaseParam(t *testing.T) {
	at, _, err := jwtLogin("sys", "manager")
	require.NoError(t, err)

	dbName := fmt.Sprintf("HTTPWRITEDB%d", time.Now().UnixNano()%1000000)
	tableName := fmt.Sprintf("HTTP_WRITE_DB_T_%d", time.Now().UnixNano())

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

	doWrite := func(t *testing.T, method string, db string, payload string) (*http.Response, []byte) {
		t.Helper()
		target := httpServerAddress + "/db/write/" + tableName + "?method=" + method
		if db != "" {
			target += "&db=" + url.QueryEscape(db)
		}
		req, _ := http.NewRequest(http.MethodPost, target, strings.NewReader(payload))
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", at))
		req.Header.Set("Content-Type", "application/json")
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

	creTable := fmt.Sprintf(`CREATE TAG TABLE %s (name varchar(40) primary key, time datetime basetime, value double)`, tableName)
	rsp, body = doQuery(t, creTable, "")
	require.Equal(t, http.StatusOK, rsp.StatusCode, string(body))
	t.Cleanup(func() {
		rsp, body := doQuery(t, "DROP TABLE "+tableName, "")
		require.Equal(t, http.StatusOK, rsp.StatusCode, string(body))
	})

	rsp, body = doQuery(t, creTable, dbName)
	require.Equal(t, http.StatusOK, rsp.StatusCode, string(body))

	t.Run("insert_isolates_by_database", func(t *testing.T) {
		payload := `{"data":{"columns":["name","time","value"],"rows":[["default-db",` +
			fmt.Sprintf("%d", testTimeTick.UnixNano()) + `,1.5]]}}`
		rsp, body := doWrite(t, "insert", "", payload)
		require.Equal(t, http.StatusOK, rsp.StatusCode, string(body))

		payload = `{"data":{"columns":["name","time","value"],"rows":[["other-db",` +
			fmt.Sprintf("%d", testTimeTick.UnixNano()+1) + `,2.5]]}}`
		rsp, body = doWrite(t, "insert", dbName, payload)
		require.Equal(t, http.StatusOK, rsp.StatusCode, string(body))

		rsp, body = doQuery(t, fmt.Sprintf(`SELECT NAME FROM %s`, tableName), "")
		require.Equal(t, http.StatusOK, rsp.StatusCode, string(body))
		require.Contains(t, string(body), "default-db")
		require.NotContains(t, string(body), "other-db")

		rsp, body = doQuery(t, fmt.Sprintf(`SELECT NAME FROM %s`, tableName), dbName)
		require.Equal(t, http.StatusOK, rsp.StatusCode, string(body))
		require.Contains(t, string(body), "other-db")
		require.NotContains(t, string(body), "default-db")
	})

	t.Run("append_isolates_by_database", func(t *testing.T) {
		payload := `{"data":{"columns":["name","time","value"],"rows":[["append-default",` +
			fmt.Sprintf("%d", testTimeTick.UnixNano()+2) + `,3.5]]}}`
		rsp, body := doWrite(t, "append", "", payload)
		require.Equal(t, http.StatusOK, rsp.StatusCode, string(body))

		payload = `{"data":{"columns":["name","time","value"],"rows":[["append-other",` +
			fmt.Sprintf("%d", testTimeTick.UnixNano()+3) + `,4.5]]}}`
		rsp, body = doWrite(t, "append", dbName, payload)
		require.Equal(t, http.StatusOK, rsp.StatusCode, string(body))

		spi.FlushAppendWorkers("", "", tableName)
		spi.FlushAppendWorkers(dbName, "", tableName)

		// TAG table appends are not visible to query until table_flush runs.
		rsp, body = doQuery(t, fmt.Sprintf(`EXEC table_flush(%s)`, tableName), "")
		require.Equal(t, http.StatusOK, rsp.StatusCode, string(body))
		rsp, body = doQuery(t, fmt.Sprintf(`EXEC table_flush(%s)`, tableName), dbName)
		require.Equal(t, http.StatusOK, rsp.StatusCode, string(body))

		rsp, body = doQuery(t, fmt.Sprintf(`SELECT NAME FROM %s WHERE NAME = 'append-default'`, tableName), "")
		require.Equal(t, http.StatusOK, rsp.StatusCode, string(body))
		require.Contains(t, string(body), "append-default")

		rsp, body = doQuery(t, fmt.Sprintf(`SELECT NAME FROM %s WHERE NAME = 'append-other'`, tableName), dbName)
		require.Equal(t, http.StatusOK, rsp.StatusCode, string(body))
		require.Contains(t, string(body), "append-other")
	})

	t.Run("invalid_db_name_returns_400", func(t *testing.T) {
		rsp, body := doWrite(t, "insert", "bad db;name", `{"data":{"columns":["name","time","value"],"rows":[]}}`)
		require.Equal(t, http.StatusBadRequest, rsp.StatusCode, string(body))
	})

	t.Run("nonexistent_db_returns_error", func(t *testing.T) {
		rsp, body := doWrite(t, "insert", "no_such_database_xyz", `{"data":{"columns":["name","time","value"],"rows":[]}}`)
		require.NotEqual(t, http.StatusOK, rsp.StatusCode, string(body))
	})
}

// TestHttpWriteQualifiedTableNamePrecedence verifies that a "db.user.table"/
// "user.table" qualifier embedded in the URL path takes precedence over the "db"
// query parameter, for both insert and append, keeping /db/write consistent with
// how /db/query resolves the same qualifier.
func TestHttpWriteQualifiedTableNamePrecedence(t *testing.T) {
	at, _, err := jwtLogin("sys", "manager")
	require.NoError(t, err)

	db1 := fmt.Sprintf("HTTPWRITEQDB1%d", time.Now().UnixNano()%1000000)
	db2 := fmt.Sprintf("HTTPWRITEQDB2%d", time.Now().UnixNano()%1000000)
	tableName := fmt.Sprintf("HTTP_WRITE_Q_T_%d", time.Now().UnixNano())

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

	doWrite := func(t *testing.T, path string, method string, db string, payload string) (*http.Response, []byte) {
		t.Helper()
		target := httpServerAddress + "/db/write/" + path + "?method=" + method
		if db != "" {
			target += "&db=" + url.QueryEscape(db)
		}
		req, _ := http.NewRequest(http.MethodPost, target, strings.NewReader(payload))
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", at))
		req.Header.Set("Content-Type", "application/json")
		rsp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		body, _ := io.ReadAll(rsp.Body)
		rsp.Body.Close()
		return rsp, body
	}

	for _, db := range []string{db1, db2} {
		rsp, body := doQuery(t, "CREATE DATABASE IF NOT EXISTS "+db, "")
		require.Equal(t, http.StatusOK, rsp.StatusCode, string(body))
		db := db // capture
		t.Cleanup(func() {
			rsp, body := doQuery(t, "DROP DATABASE "+db+" CASCADE", "")
			require.Equal(t, http.StatusOK, rsp.StatusCode, string(body))
		})
	}

	creTable := fmt.Sprintf(`CREATE TAG TABLE %s (name varchar(40) primary key, time datetime basetime, value double)`, tableName)
	for _, db := range []string{"", db1, db2} {
		rsp, body := doQuery(t, creTable, db)
		require.Equal(t, http.StatusOK, rsp.StatusCode, string(body))
	}
	t.Cleanup(func() {
		rsp, body := doQuery(t, "DROP TABLE "+tableName, "")
		require.Equal(t, http.StatusOK, rsp.StatusCode, string(body))
	})

	payload := func(name string, offset int64) string {
		return fmt.Sprintf(`{"data":{"columns":["name","time","value"],"rows":[["%s",%d,1.5]]}}`,
			name, testTimeTick.UnixNano()+offset)
	}

	t.Run("insert_user_dot_table_with_db_param", func(t *testing.T) {
		// /db/write/SYS.<table>?db=<db1>
		rsp, body := doWrite(t, "SYS."+tableName, "insert", db1, payload("qp-user-table", 10))
		require.Equal(t, http.StatusOK, rsp.StatusCode, string(body))

		rsp, body = doQuery(t, fmt.Sprintf(`SELECT NAME FROM %s`, tableName), db1)
		require.Equal(t, http.StatusOK, rsp.StatusCode, string(body))
		require.Contains(t, string(body), "qp-user-table")

		rsp, body = doQuery(t, fmt.Sprintf(`SELECT NAME FROM %s`, tableName), "")
		require.Equal(t, http.StatusOK, rsp.StatusCode, string(body))
		require.NotContains(t, string(body), "qp-user-table")
	})

	t.Run("insert_db_dot_user_dot_table_without_param", func(t *testing.T) {
		// /db/write/<db1>.SYS.<table>, no "db" query param
		rsp, body := doWrite(t, db1+".SYS."+tableName, "insert", "", payload("path-qualified", 20))
		require.Equal(t, http.StatusOK, rsp.StatusCode, string(body))

		rsp, body = doQuery(t, fmt.Sprintf(`SELECT NAME FROM %s`, tableName), db1)
		require.Equal(t, http.StatusOK, rsp.StatusCode, string(body))
		require.Contains(t, string(body), "path-qualified")

		rsp, body = doQuery(t, fmt.Sprintf(`SELECT NAME FROM %s`, tableName), "")
		require.Equal(t, http.StatusOK, rsp.StatusCode, string(body))
		require.NotContains(t, string(body), "path-qualified")
	})

	t.Run("path_db_wins_over_query_param", func(t *testing.T) {
		// /db/write/<db1>.SYS.<table>?db=<db2> -> db1 (path) wins, db2 (query) is ignored
		rsp, body := doWrite(t, db1+".SYS."+tableName, "insert", db2, payload("path-wins", 30))
		require.Equal(t, http.StatusOK, rsp.StatusCode, string(body))

		rsp, body = doQuery(t, fmt.Sprintf(`SELECT NAME FROM %s`, tableName), db1)
		require.Equal(t, http.StatusOK, rsp.StatusCode, string(body))
		require.Contains(t, string(body), "path-wins")

		rsp, body = doQuery(t, fmt.Sprintf(`SELECT NAME FROM %s`, tableName), db2)
		require.Equal(t, http.StatusOK, rsp.StatusCode, string(body))
		require.NotContains(t, string(body), "path-wins")
	})

	t.Run("append_user_dot_table_with_db_param", func(t *testing.T) {
		// /db/write/SYS.<table>?method=append&db=<db1>
		rsp, body := doWrite(t, "SYS."+tableName, "append", db1, payload("append-qp-user-table", 40))
		require.Equal(t, http.StatusOK, rsp.StatusCode, string(body))
		spi.FlushAppendWorkers(db1, "SYS", tableName)

		rsp, body = doQuery(t, fmt.Sprintf(`EXEC table_flush(%s)`, tableName), db1)
		require.Equal(t, http.StatusOK, rsp.StatusCode, string(body))
		rsp, body = doQuery(t, fmt.Sprintf(`SELECT NAME FROM %s`, tableName), db1)
		require.Equal(t, http.StatusOK, rsp.StatusCode, string(body))
		require.Contains(t, string(body), "append-qp-user-table")
	})

	t.Run("append_path_db_wins_over_query_param", func(t *testing.T) {
		// /db/write/<db1>.SYS.<table>?method=append&db=<db2> -> db1 (path) wins
		rsp, body := doWrite(t, db1+".SYS."+tableName, "append", db2, payload("append-path-wins", 50))
		require.Equal(t, http.StatusOK, rsp.StatusCode, string(body))
		spi.FlushAppendWorkers(db1, "SYS", tableName)

		rsp, body = doQuery(t, fmt.Sprintf(`EXEC table_flush(%s)`, tableName), db1)
		require.Equal(t, http.StatusOK, rsp.StatusCode, string(body))
		rsp, body = doQuery(t, fmt.Sprintf(`SELECT NAME FROM %s`, tableName), db1)
		require.Equal(t, http.StatusOK, rsp.StatusCode, string(body))
		require.Contains(t, string(body), "append-path-wins")

		rsp, body = doQuery(t, fmt.Sprintf(`SELECT NAME FROM %s`, tableName), db2)
		require.Equal(t, http.StatusOK, rsp.StatusCode, string(body))
		require.NotContains(t, string(body), "append-path-wins")
	})
}

func TestHandleFileWriteRejectsInvalidContentType(t *testing.T) {
	svr := newTestHTTPServer(t)
	ctx, writer := newTestHTTPContext(http.MethodPost, "/db/write/TEST", []byte("not multipart"))
	ctx.Request.Header.Set("Content-Type", "application/json")

	svr.handleFileWrite(ctx)

	require.Equal(t, http.StatusInternalServerError, writer.Code)
	require.Contains(t, writer.Body.String(), "content-type must be 'multipart/form-data'")
}

func TestHandleFileWriteErrors(t *testing.T) {
	jwt := HttpTestLogin(t, "sys", "manager")
	tableName := fmt.Sprintf("P2_FILE_%d", testTimeTick.Unix())
	failingTableName := fmt.Sprintf("P2_FILE_FAIL_%d", testTimeTick.Unix())

	createTable := fmt.Sprintf(`create tag table %s (
		NAME varchar(200) primary key,
		TIME datetime basetime,
		VALUE double summarized,
		EXT_DATA json)`, tableName)
	failingCreateTable := fmt.Sprintf(`create tag table %s (
		NAME varchar(200) primary key,
		TIME datetime basetime,
		VALUE double summarized,
		EXT_DATA int)`, failingTableName)
	req, err := http.NewRequest(http.MethodGet, httpServerAddress+"/db/query?q="+url.QueryEscape(createTable), nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", jwt.AccessToken))
	rsp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rsp.StatusCode)
	rsp.Body.Close()

	req, err = http.NewRequest(http.MethodGet, httpServerAddress+"/db/query?q="+url.QueryEscape(failingCreateTable), nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", jwt.AccessToken))
	rsp, err = http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rsp.StatusCode)
	rsp.Body.Close()

	t.Cleanup(func() {
		dropTable := fmt.Sprintf("drop table %s", tableName)
		req, _ := http.NewRequest(http.MethodGet, httpServerAddress+"/db/query?q="+url.QueryEscape(dropTable), nil)
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", jwt.AccessToken))
		rsp, _ := http.DefaultClient.Do(req)
		if rsp != nil {
			rsp.Body.Close()
		}

		dropFailingTable := fmt.Sprintf("drop table %s", failingTableName)
		req, _ = http.NewRequest(http.MethodGet, httpServerAddress+"/db/query?q="+url.QueryEscape(dropFailingTable), nil)
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", jwt.AccessToken))
		rsp, _ = http.DefaultClient.Do(req)
		if rsp != nil {
			rsp.Body.Close()
		}
	})

	doMultipart := func(t *testing.T, targetTable string, fields map[string]string, files ...multipartTestFile) (int, string) {
		t.Helper()
		req, err := buildMultipartTestRequest(httpServerAddress+"/db/write/"+targetTable, fields, files...)
		require.NoError(t, err)
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", jwt.AccessToken))
		rsp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer rsp.Body.Close()
		body, err := io.ReadAll(rsp.Body)
		require.NoError(t, err)
		return rsp.StatusCode, string(body)
	}

	t.Run("missing store dir rejects file upload", func(t *testing.T) {
		status, body := doMultipart(t,
			tableName,
			map[string]string{
				"NAME":  "missing-store-dir",
				"TIME":  fmt.Sprintf("%d", testTimeTick.UnixNano()),
				"VALUE": "3.14",
			},
			multipartTestFile{
				fieldName:   "EXT_DATA",
				fileName:    "sample.txt",
				contentType: "text/plain",
				content:     []byte("hello"),
			},
		)
		result := WriteResponse{}
		require.NoError(t, json.Unmarshal([]byte(body), &result))

		require.Equal(t, http.StatusBadRequest, status)
		require.False(t, result.Success)
		require.Equal(t, `file "EXT_DATA" requires X-Store-Dir header`, result.Reason)
	})

	t.Run("unknown column rejects multipart value", func(t *testing.T) {
		status, body := doMultipart(t,
			tableName,
			map[string]string{
				"NAME":       "unknown-column",
				"TIME":       fmt.Sprintf("%d", testTimeTick.UnixNano()),
				"VALUE":      "3.14",
				"BAD_COLUMN": "oops",
			},
		)
		result := WriteResponse{}
		require.NoError(t, json.Unmarshal([]byte(body), &result))

		require.Equal(t, http.StatusBadRequest, status)
		require.False(t, result.Success)
		require.Contains(t, result.Reason, `column "BAD_COLUMN" not found`)
	})

	t.Run("request level store dir participates in path map replacement", func(t *testing.T) {
		status, body := doMultipart(t,
			tableName,
			map[string]string{
				"NAME":  "pathmap-ok",
				"TIME":  fmt.Sprintf("%d", testTimeTick.UnixNano()),
				"VALUE": "3.14",
			},
			multipartTestFile{
				fieldName:   "EXT_DATA",
				fileName:    "sample.txt",
				contentType: "text/plain",
				content:     []byte("hello"),
				headers: map[string]string{
					"X-Store-Dir": "${data}/store-p2",
				},
			},
		)

		require.Equal(t, http.StatusOK, status, body)
		require.Contains(t, body, `"success":true`)
		require.Contains(t, body, `store-p2`)
	})

	t.Run("insert failure removes just-written file", func(t *testing.T) {
		storeDir := t.TempDir()
		status, body := doMultipart(t,
			failingTableName,
			map[string]string{
				"NAME":  "type-mismatch-row",
				"TIME":  fmt.Sprintf("%d", testTimeTick.UnixNano()),
				"VALUE": "6.28",
			},
			multipartTestFile{
				fieldName:   "EXT_DATA",
				fileName:    "mismatch.txt",
				contentType: "text/plain",
				content:     []byte("second"),
				headers: map[string]string{
					"X-Store-Dir": storeDir,
				},
			},
		)
		result := WriteResponse{}
		require.NoError(t, json.Unmarshal([]byte(body), &result))

		require.Equal(t, http.StatusInternalServerError, status)
		require.False(t, result.Success)

		entries, err := os.ReadDir(storeDir)
		require.NoError(t, err)
		require.Len(t, entries, 0)
	})
}

func TestHandleLineWrite(t *testing.T) {
	jwt := HttpTestLogin(t, "sys", "manager")
	tableName := fmt.Sprintf("P2_LINE_%d", testTimeTick.Unix())

	createTable := fmt.Sprintf(`create tag table %s (
		NAME varchar(200) primary key,
		TIME datetime basetime,
		VALUE double summarized,
		EXT_DATA json)`, tableName)
	req, err := http.NewRequest(http.MethodGet, httpServerAddress+"/db/query?q="+url.QueryEscape(createTable), nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", jwt.AccessToken))
	rsp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rsp.StatusCode)
	rsp.Body.Close()

	t.Cleanup(func() {
		dropTable := fmt.Sprintf("drop table %s", tableName)
		req, _ := http.NewRequest(http.MethodGet, httpServerAddress+"/db/query?q="+url.QueryEscape(dropTable), nil)
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", jwt.AccessToken))
		rsp, _ := http.DefaultClient.Do(req)
		if rsp != nil {
			rsp.Body.Close()
		}
	})

	doLineWrite := func(t *testing.T, body []byte, headers map[string]string, query string) (int, string) {
		t.Helper()
		req, err := http.NewRequest(http.MethodPost, httpServerAddress+"/metrics/write?db="+tableName+query, bytes.NewReader(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/octet-stream")
		for key, value := range headers {
			req.Header.Set(key, value)
		}
		rsp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer rsp.Body.Close()
		payload, err := io.ReadAll(rsp.Body)
		require.NoError(t, err)
		return rsp.StatusCode, string(payload)
	}

	t.Run("invalid gzip returns bad request", func(t *testing.T) {
		status, body := doLineWrite(t, []byte("not-gzip"), map[string]string{"Content-Encoding": "gzip"}, "")
		require.Equal(t, http.StatusBadRequest, status)
		require.Contains(t, body, "invalid gzip compression")
	})

	t.Run("missing timestamp returns bad request", func(t *testing.T) {
		status, body := doLineWrite(t, []byte("cpu,host=desktop usage_irq=1"), nil, "")
		require.Equal(t, http.StatusBadRequest, status)
		require.Contains(t, body, `"error":"no timestamp"`)
	})

	t.Run("measurement parse error returns internal server error", func(t *testing.T) {
		status, body := doLineWrite(t, []byte(",host=desktop usage_irq=1 1670975120000000000"), nil, "")
		require.Equal(t, http.StatusInternalServerError, status)
		require.Contains(t, body, "measurement error")
	})

	t.Run("tag parse error returns internal server error", func(t *testing.T) {
		status, body := doLineWrite(t, []byte("cpu,host usage_irq=1 1670975120000000000"), nil, "")
		require.Equal(t, http.StatusInternalServerError, status)
		require.Contains(t, body, "tag error")
	})

	t.Run("precision ms accepts millisecond timestamp", func(t *testing.T) {
		line := []byte("cpu,host=desktop usage_irq=1 1670975120000")
		status, body := doLineWrite(t, line, nil, "&precision=ms")
		require.Equal(t, http.StatusNoContent, status, body)
	})

	t.Run("precision us accepts microsecond timestamp", func(t *testing.T) {
		line := []byte("cpu,host=desktop usage_irq=1 1670975120000000")
		status, body := doLineWrite(t, line, nil, "&precision=us")
		require.Equal(t, http.StatusNoContent, status, body)
	})

	t.Run("invalid field syntax returns error", func(t *testing.T) {
		status, body := doLineWrite(t, []byte("cpu,host=desktop usage_irq 1670975120000000000"), nil, "")
		require.Equal(t, http.StatusInternalServerError, status)
		require.True(t,
			strings.Contains(body, "field error") || strings.Contains(body, "measurement error"),
			body,
		)
	})

	t.Run("gzip compressed valid payload succeeds", func(t *testing.T) {
		var buf bytes.Buffer
		gw := gzip.NewWriter(&buf)
		_, err := gw.Write([]byte("cpu,host=desktop usage_irq=1 1670975120000000000"))
		require.NoError(t, err)
		require.NoError(t, gw.Close())

		status, body := doLineWrite(t, buf.Bytes(), map[string]string{"Content-Encoding": "gzip"}, "")
		require.Equal(t, http.StatusNoContent, status, body)
	})
}

func TestWriteBinaryFormat(t *testing.T) {
	// create table
	sql := "CREATE TAG TABLE IF NOT EXISTS wbin (name varchar(40) primary key, time datetime base time, value binary)"
	req, _ := http.NewRequest(http.MethodGet, httpServerAddress+"/db/query?q="+url.QueryEscape(sql), nil)
	rsp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rsp.StatusCode)
	rsp.Body.Close()

	// drop table
	defer func() {
		done := spi.StopAppendWorker("", "", "wbin")
		<-done

		sql := "DROP TABLE wbin"
		req, _ := http.NewRequest(http.MethodGet, httpServerAddress+"/db/query?q="+url.QueryEscape(sql), nil)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		resp.Body.Close()
	}()

	tests := []struct {
		name        string
		contentType string
		binformat   string
		input       string
		expectQuery string
		expect      string
	}{
		{
			name:        "json_base64",
			contentType: "application/json",
			binformat:   "base64",
			input:       `{"data":{"columns":["NAME", "TIME", "VALUE"],"rows":[["json_base64", 1691800174123456789, "AQKgsMDQ4PA="]]}}`,
			expectQuery: `select name, value from wbin where name='json_base64'`,
			expect:      `["json_base64","0x0102a0b0c0d0e0f0"]`,
		},
		{
			name:        "json_hex",
			contentType: "application/json",
			binformat:   "hex",
			input:       `{"data":{"columns":["NAME", "TIME", "VALUE"],"rows":[["json_hex", 1691800174123456789, "0x0102a0b0c0d0e0f0"]]}}`,
			expectQuery: `select name, value from wbin where name='json_hex'`,
			expect:      `["json_hex","0x0102a0b0c0d0e0f0"]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// call write API with then given content type and binary format
			path := httpServerAddress + "/db/write/wbin?method=append"
			// For testing method=append, the server side appender should be closed.
			// The alive 'append' connection prevents the cleaning up (drop table) after test and cause other test failure,
			// it cause "resource busy" error when drop table.

			if tt.binformat != "" {
				path += "&binaryformat=" + url.QueryEscape(tt.binformat)
			}
			req, _ = http.NewRequest(http.MethodPost, path, strings.NewReader(tt.input))
			req.Header.Set("Content-Type", tt.contentType)
			rsp, err = http.DefaultClient.Do(req)
			require.NoError(t, err)
			response, _ := io.ReadAll(rsp.Body)
			require.Equal(t, http.StatusOK, rsp.StatusCode, string(response))
			rsp.Body.Close()

			// flush appender to make sure data is visible to query,
			// since the test is using method=append,
			// the data is not immediately visible to query until the appender is flushed or closed.
			spi.FlushAppendWorkers("", "", "wbin")

			// flush to make sure data is visible to query
			flush := httpServerAddress + "/db/query?q=" + url.QueryEscape("exec table_flush(wbin)")
			req, _ = http.NewRequest(http.MethodGet, flush, nil)
			rsp, err = http.DefaultClient.Do(req)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, rsp.StatusCode)
			rsp.Body.Close()

			// query the table to verify the value is stored and returned as expected
			query := httpServerAddress + "/db/query?q=" + url.QueryEscape(tt.expectQuery)
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
