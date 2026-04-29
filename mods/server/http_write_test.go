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
