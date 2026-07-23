package httpdsl

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/machbase/neo-server/v8/mods/util/ssfs"
	"github.com/stretchr/testify/require"
)

func splitRawHTTPMessage(raw string) (string, string) {
	if i := strings.Index(raw, "\r\n\r\n"); i >= 0 {
		return raw[:i+4], raw[i+4:]
	}
	if i := strings.Index(raw, "\n\n"); i >= 0 {
		return raw[:i+2], raw[i+2:]
	}
	return raw, ""
}

func newHTTPDSLTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/echo":
			if r.Method == http.MethodGet {
				_ = json.NewEncoder(w).Encode(map[string]string{
					"q":      r.URL.Query().Get("q"),
					"format": r.URL.Query().Get("format"),
				})
				return
			}
			raw, _ := io.ReadAll(r.Body)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(raw)
		case "/api/form":
			require.NoError(t, r.ParseForm())
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{
				"q":      r.FormValue("q"),
				"format": r.FormValue("format"),
			})
		case "/api/upload":
			require.NoError(t, r.ParseMultipartForm(4<<20))
			name := r.FormValue("name")
			img, imgHeader, err := r.FormFile("image")
			require.NoError(t, err)
			defer img.Close()
			imgRaw, err := io.ReadAll(img)
			require.NoError(t, err)
			doc, docHeader, err := r.FormFile("doc")
			require.NoError(t, err)
			defer doc.Close()
			docRaw, err := io.ReadAll(doc)
			require.NoError(t, err)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"name":      name,
				"image":     imgHeader.Filename,
				"imageBody": string(imgRaw),
				"doc":       docHeader.Filename,
				"docBody":   string(docRaw),
			})
		default:
			http.NotFound(w, r)
		}
	}))
}

func TestExecuteGetWithQueryExtension(t *testing.T) {
	ts := newHTTPDSLTestServer(t)
	defer ts.Close()

	script := fmt.Sprintf("GET %s/api/echo\n?q=select * from tag_simple\n&format=json\n", ts.URL)
	ex, err := Execute(script)
	require.NoError(t, err)
	require.Contains(t, ex.RequestRaw, "GET /api/echo?")
	require.Contains(t, ex.RequestRaw, "q=select+%2A+from+tag_simple")
	require.Contains(t, ex.RequestRaw, "format=json")
	require.Contains(t, ex.ResponseRaw, "HTTP/1.1 200 OK\r\n")
	_, body := splitRawHTTPMessage(ex.ResponseRaw)
	require.JSONEq(t, `{"q":"select * from tag_simple","format":"json"}`, body)
}

func TestExecutePostBodyFromOSFileDirective(t *testing.T) {
	ts := newHTTPDSLTestServer(t)
	defer ts.Close()

	dir := t.TempDir()
	jsonPath := filepath.Join(dir, "payload-한글.json")
	require.NoError(t, os.WriteFile(jsonPath, []byte(`{"name":"John","doc":"ok"}`), 0o600))

	script := fmt.Sprintf("POST %s/api/echo\nContent-Type: application/json\n\n< @%s", ts.URL, jsonPath)
	ex, err := Execute(script)
	require.NoError(t, err)
	require.Contains(t, ex.RequestRaw, "Content-Type: application/json\r\n")
	_, body := splitRawHTTPMessage(ex.ResponseRaw)
	require.JSONEq(t, `{"name":"John","doc":"ok"}`, body)
}

func TestExecutePostMultipartWithFileDirectives(t *testing.T) {
	ts := newHTTPDSLTestServer(t)
	defer ts.Close()

	dir := t.TempDir()
	imagePath := filepath.Join(dir, "1.png")
	require.NoError(t, os.WriteFile(imagePath, []byte("PNGDATA"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "한글.xml"), []byte("<doc/>"), 0o600))

	prev := ssfs.Default()
	fsys, err := ssfs.NewServerSideFileSystem([]string{"/=" + dir})
	require.NoError(t, err)
	ssfs.SetDefault(fsys)
	t.Cleanup(func() { ssfs.SetDefault(prev) })

	boundary := "----WebKitFormBoundary7MA4YWxkTrZu0gW"
	script := strings.Join([]string{
		"POST " + ts.URL + "/api/upload",
		"Content-Type: multipart/form-data; boundary=" + boundary,
		"",
		"--" + boundary,
		"Content-Disposition: form-data; name=\"name\"",
		"",
		"John",
		"--" + boundary,
		"Content-Disposition: form-data; name=\"image\"; filename=\"1.png\"",
		"Content-Type: image/png",
		"",
		"< @" + imagePath,
		"--" + boundary,
		"Content-Disposition: form-data; name=\"doc\"; filename=\"한글.xml\"",
		"Content-Type: text/xml",
		"",
		"< /한글.xml",
		"--" + boundary + "--",
	}, "\n")

	ex, err := Execute(script)
	require.NoError(t, err)
	_, body := splitRawHTTPMessage(ex.ResponseRaw)
	require.JSONEq(t, `{"name":"John","image":"1.png","imageBody":"PNGDATA","doc":"한글.xml","docBody":"<doc/>"}`, body)
}

func TestExecuteFormURLEncodedBody(t *testing.T) {
	ts := newHTTPDSLTestServer(t)
	defer ts.Close()

	script := fmt.Sprintf("POST %s/api/form\nContent-Type: application/x-www-form-urlencoded\n\nq=select * from t\n&format=json", ts.URL)
	ex, err := Execute(script)
	require.NoError(t, err)
	_, body := splitRawHTTPMessage(ex.ResponseRaw)
	require.JSONEq(t, `{"q":"select * from t","format":"json"}`, body)
}

func TestBuildRawRequestGzipContentLength(t *testing.T) {
	req := &parsedRequest{
		Method:  http.MethodPost,
		URL:     mustURL(t, "http://127.0.0.1:5654/upload"),
		Version: "HTTP/1.1",
		Headers: []headerLine{{Name: "Content-Encoding", Value: "gzip"}, {Name: "Content-Length", Value: "1"}},
		Body:    []string{"hello world"},
	}
	raw, err := buildRawRequest(req)
	require.NoError(t, err)
	head, body := splitRawHTTPMessage(string(raw))
	require.Contains(t, head, "Content-Encoding: gzip\r\n")
	require.Contains(t, head, fmt.Sprintf("Content-Length: %d\r\n", len(body)))
	gz, err := gzip.NewReader(bytes.NewReader([]byte(body)))
	require.NoError(t, err)
	defer gz.Close()
	decoded, err := io.ReadAll(gz)
	require.NoError(t, err)
	require.Equal(t, "hello world", string(decoded))
}

func TestParseAndHelperBranches(t *testing.T) {
	_, err := parseHTTPClient("\n\n")
	require.EqualError(t, err, "http: empty request")
	_, err = parseHTTPClient("GET")
	require.EqualError(t, err, "http: invalid request line")
	_, err = parseHTTPClient("GET ://bad")
	require.ErrorContains(t, err, "http: invalid URL")

	require.Equal(t, "http://a/b?x=1&y=2", normalizeRawURLQuery("http://a/b?x=1&y=2"))
	require.Equal(t, "http://a/b", normalizeRawURLQuery("http://a/b"))

	h, ok := extractRawHTTPHeader([]byte("HTTP/1.1 200 OK\nX: y\n\nbody"))
	require.True(t, ok)
	require.Equal(t, "HTTP/1.1 200 OK\nX: y\n\n", string(h))
	_, ok = extractRawHTTPHeader([]byte("HTTP/1.1 200 OK\nX: y\nbody"))
	require.False(t, ok)

	require.True(t, requestBodyShouldBeGzipped([]headerLine{{Name: "Content-Encoding", Value: "br, gzip"}}))
	require.False(t, requestBodyShouldBeGzipped([]headerLine{{Name: "Content-Encoding", Value: "br"}}))
}

func TestResolveBodyAndHelperBranches(t *testing.T) {
	body, err := resolveRequestBody(http.MethodPost, nil, nil)
	require.NoError(t, err)
	require.Nil(t, body)

	body, err = resolveRequestBody(http.MethodPost, []headerLine{{Name: "Content-Type", Value: "application/json"}}, []string{"a", "b"})
	require.NoError(t, err)
	require.Equal(t, "a\nb", string(body))

	body, err = resolveRequestBody(http.MethodPost, []headerLine{{Name: "Content-Type", Value: "application/json"}}, []string{"< @/no/such/file"})
	require.NoError(t, err)
	require.Contains(t, string(body), "Error opening file")

	body, err = resolveRequestBody(http.MethodPost, []headerLine{{Name: "Content-Type", Value: "multipart/form-data; boundary=foo"}}, []string{"plain-line"})
	require.NoError(t, err)
	require.Equal(t, "plain-line\n", string(body))

	r, err := parseFileLine(http.MethodGet, "plain")
	require.NoError(t, err)
	raw, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, "plain\n", string(raw))

	prev := ssfs.Default()
	ssfs.SetDefault(nil)
	t.Cleanup(func() { ssfs.SetDefault(prev) })
	_, err = parseFileLine(http.MethodPost, "< /a.txt")
	require.ErrorContains(t, err, "server side file system is not initialized")

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "legacy.txt"), []byte("legacy"), 0o600))
	fsys, err := ssfs.NewServerSideFileSystem([]string{"/=" + dir})
	require.NoError(t, err)
	ssfs.SetDefault(fsys)

	r, err = parseFileLine(http.MethodPost, "<@utf-8 /legacy.txt")
	require.NoError(t, err)
	raw, err = io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, "legacy\n", string(raw))

	h, ok := extractRawHTTPHeader([]byte("HTTP/1.1 200 OK\r\nA: b\r\n\r\nbody"))
	require.True(t, ok)
	require.Equal(t, "HTTP/1.1 200 OK\r\nA: b\r\n\r\n", string(h))

	m, u, v := parseRequestLine("POST http://a/b?x=1 HTTP/1.1")
	require.Equal(t, "POST", m)
	require.Equal(t, "http://a/b?x=1", u)
	require.Equal(t, "HTTP/1.1", v)

	fd := parseFileDirective("< @/tmp/file.txt")
	require.True(t, fd.Directive)
	require.True(t, fd.FromOS)
	require.Equal(t, "/tmp/file.txt", fd.Path)

	fd = parseFileDirective("<@ @/tmp/file2.txt")
	require.True(t, fd.Directive)
	require.True(t, fd.FromOS)
	require.Equal(t, "/tmp/file2.txt", fd.Path)

	fd = parseFileDirective("<@ /ssfs/path/file.txt")
	require.True(t, fd.Directive)
	require.False(t, fd.FromOS)
	require.Equal(t, "/ssfs/path/file.txt", fd.Path)
}

func TestExecuteDialErrorReturnsRequestRaw(t *testing.T) {
	ex, err := Execute("GET http://127.0.0.1:1/ping")
	require.Error(t, err)
	require.Contains(t, ex.RequestRaw, "GET /ping HTTP/1.1\r\n")
}

func mustURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	require.NoError(t, err)
	return u
}
