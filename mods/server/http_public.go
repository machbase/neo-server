package server

/**
# CGI handling in http_public.go

`handlePublic()` executes `.../cgi-bin/*.js` through the JSH engine and interprets the script output as a parsed CGI response.

## Supported CGI response forms

The script output must contain headers first, followed by a blank line, then the optional body.

- Header terminator: `\r\n\r\n` or `\n\n`
- Header syntax: `Name: value`
- Standard CGI headers: `Content-Type`, `Status`, `Location`

The server supports these parsed CGI response forms from RFC 3875:

1. Document response

   ```text
   Content-Type: text/plain
   Status: 200 OK

   hello
   ```

   `Content-Type` is required.
   `Status` is optional and defaults to `200`.

2. Local redirect response

   ```text
   Location: /public/app/index.html

   ```

   No other headers or body are allowed.
   The server rewrites the request path and re-enters the Gin router internally.

3. Client redirect response

   ```text
   Location: https://example.com/next

   ```

   No body is allowed.
   The server sends `302 Found` to the client.

4. Client redirect response with document

   ```text
   Location: https://example.com/next
   Status: 302 Found
   Content-Type: text/html

   <html>...</html>
   ```

   `Status` must be a `3xx` code.
   `Content-Type` is required when a body is present.

## Additional behavior

- `HEAD` requests discard the CGI message body but still apply response headers.
- Duplicate `Status`, `Content-Type`, and `Location` headers are rejected.
- Malformed CGI output returns HTTP 500 from `handlePublic()`.

## Compatibility extension

For compatibility with existing scripts, the first non-empty response line may also be written as an HTTP-style status line instead of a CGI `Status:` header.

Example:

```text
HTTP/1.1 201 Created
Content-Type: application/json

{"ok":true}
```

This is an implementation-defined extension and is not part of standard parsed CGI/1.1 response syntax.
*/

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/machbase/neo-server/v8/jsh/engine"
	"github.com/machbase/neo-server/v8/jsh/lib"
	"github.com/machbase/neo-server/v8/jsh/root"
)

const cgiDiagnosticMaxBytes = 4096

func (svr *httpd) handlePublic(ctx *gin.Context) {
	tick := time.Now()
	path := ctx.Param("path")
	// Remove leading slash and prevent directory traversal higher than /public/
	path = strings.TrimPrefix(path, "/")
	if strings.Contains(path, "..") {
		handleError(ctx, http.StatusBadRequest, "invalid path", tick)
		return
	}
	path = "/public/" + path
	if strings.Contains(path, "/cgi-bin/") {
		if !strings.HasSuffix(path, ".js") {
			path = path + ".js"
		}
		if ent, err := svr.serverFs.Get(path); err != nil || ent.IsDir {
			handleError(ctx, http.StatusNotFound, "not found", tick)
			return
		}
		toks := strings.SplitN(path, "/cgi-bin/", 2)
		appPath := toks[0]
		appRealPath, err := svr.serverFs.FindRealPath(appPath)
		mountPoint := "/work" + appPath
		cgiPath := "/cgi-bin/" + toks[1]
		if err != nil {
			handleError(ctx, http.StatusInternalServerError, "app path error: "+err.Error(), tick)
			return
		}
		code := strings.Join([]string{
			"const process = require('process');",
			"try {",
			fmt.Sprintf("const result = process.exec('%s%s');", mountPoint, cgiPath),
			"result && console.println(JSON.stringify(result, null, 2));",
			"} catch (err) {",
			"console.println(JSON.stringify({ error: err.message }, null, 2));",
			"}",
		}, "\n")
		fsTabs := []engine.FSTab{
			root.RootFSTab(),
			lib.LibFSTab(),
			// Mount the app directory to /work in the script's virtual filesystem
			{MountPoint: mountPoint, Source: appRealPath.AbsPath},
		}

		env := contextToCGIEnv(ctx, path)
		// ServiceController
		env[engine.ControllerSharedMountEnv] = svr.authServer.serviceController.SharedMountPoint()
		env[engine.ControllerAddressEnv] = svr.authServer.serviceController.Address()
		// Common env
		env["HOME"] = "/work"
		env["PWD"] = mountPoint + filepath.Dir(cgiPath)
		env["QUERY"] = ctx.Request.URL.Query()
		cgiWriter := &CgiBinWriter{ctx: ctx, svr: svr}
		stdoutCapture := newLimitedCaptureWriter(cgiDiagnosticMaxBytes)
		stderrCapture := newLimitedCaptureWriter(cgiDiagnosticMaxBytes)
		conf := engine.Config{
			Name:   path,
			Code:   code,
			FSTabs: fsTabs,
			Env:    env,
			Reader: ctx.Request.Body,
			Writer: io.MultiWriter(cgiWriter, stdoutCapture),
			ErrorWriter: io.MultiWriter(
				stderrCapture,
				os.Stderr,
			),
			ExecBuilder: func(code string, args []string, env map[string]any) (*exec.Cmd, error) {
				self, err := os.Executable()
				if err != nil {
					return nil, err
				}
				conf := engine.Config{
					Code:   code,
					Args:   args,
					FSTabs: fsTabs,
					Env:    env,
				}
				secretBox, err := engine.NewSecretBox(conf)
				if err != nil {
					return nil, err
				}
				execCmd := exec.Command(self, "jsh", "-S", secretBox.FilePath(), args[0])
				return execCmd, nil
			},
		}
		jr, err := engine.New(conf)
		if err != nil {
			handleError(ctx, http.StatusInternalServerError, "engine error: "+err.Error(), tick)
			return
		}
		lib.Enable(jr)
		if err := jr.Run(); err != nil {
			msg := "engine run error: " + err.Error()
			msg = appendCgiDiagnostic(msg, stdoutCapture.String(), stderrCapture.String())
			handleError(ctx, http.StatusInternalServerError, msg, tick)
			return
		}
		if err := cgiWriter.Finalize(); err != nil {
			msg := "invalid cgi response: " + err.Error()
			msg = appendCgiDiagnostic(msg, stdoutCapture.String(), stderrCapture.String())
			handleError(ctx, http.StatusInternalServerError, msg, tick)
		}
		return
	} else if ctx.Request.Method == http.MethodGet {
		ent, err := svr.serverFs.Get(path)
		if err != nil {
			handleError(ctx, http.StatusNotFound, "not found", tick)
			return
		}
		if ent.IsDir {
			path, _ = url.JoinPath(path, "index.html")
			ent, err = svr.serverFs.Get(path)
			if err != nil || ent.IsDir {
				handleError(ctx, http.StatusNotFound, "not found", tick)
				return
			}
		} else {
			// Redirect to path without "index.html" suffix if it exists
			// e.g. redirect "/public/foo/index.html" to "/public/foo/"
			if strings.HasSuffix(path, "/index.html") {
				ctx.Redirect(http.StatusFound, strings.TrimSuffix(path, "index.html"))
				return
			}
		}
		// Serve the file content with correct Content-Type
		contentType := contentTypeOfFile(ent.Name)
		if ent, err := svr.serverFs.Get(path); err == nil && !ent.IsDir {
			ctx.Header("Content-Type", contentType)
			ctx.Writer.Write(ent.Content)
			return
		}
	}
	handleError(ctx, http.StatusNotFound, "not found", tick)
}

func contextToCGIEnv(ctx *gin.Context, scriptName string) map[string]any {
	m := map[string]any{
		// CGI standard env
		"AUTH_TYPE":                "",
		"CONTENT_ENCODING":         ctx.Request.Header.Get("Content-Encoding"),
		"CONTENT_LENGTH":           ctx.Request.Header.Get("Content-Length"),
		"CONTENT_TYPE":             ctx.Request.Header.Get("Content-Type"),
		"GATEWAY_INTERFACE":        "CGI/1.1",
		"HTTP_ACCEPT":              ctx.Request.Header.Get("Accept"),
		"HTTP_ACCEPT_CHARSET":      ctx.Request.Header.Get("Accept-Charset"),
		"HTTP_ACCEPT_ENCODING":     ctx.Request.Header.Get("Accept-Encoding"),
		"HTTP_ACCEPT_LANGUAGE":     ctx.Request.Header.Get("Accept-Language"),
		"HTTP_COOKIE":              ctx.Request.Header.Get("Cookie"),
		"HTTP_FORWARDED":           ctx.Request.Header.Get("Forwarded"),
		"HTTP_HOST":                ctx.Request.Header.Get("Host"),
		"HTTP_PROXY_AUTHORIZATION": ctx.Request.Header.Get("Proxy-Authorization"),
		"HTTP_USER_AGENT":          ctx.Request.Header.Get("User-Agent"),
		"PATH_INFO":                ctx.Request.URL.Path,
		"PATH_TRANSLATED":          ctx.Request.URL.Path,
		"QUERY_STRING":             ctx.Request.URL.RawQuery,
		"REMOTE_ADDR":              ctx.ClientIP(),
		"REMOTE_HOST":              ctx.ClientIP(),
		"REMOTE_USER":              "",
		"REQUEST_METHOD":           ctx.Request.Method,
		"SCRIPT_NAME":              scriptName,
		"SERVER_PROTOCOL":          ctx.Request.Proto,
		"SERVER_SOFTWARE":          "machbase-neo",
	}
	return m
}

type CgiBinWriter struct {
	ctx            *gin.Context
	svr            *httpd
	router         *gin.Engine
	headerBuf      []byte
	headerParsed   bool
	headersApplied bool
	bodySeen       bool
	sawOutput      bool
	response       *cgiResponseMeta
}

// Log implements jsh/log.LogKind to keep console.log output scoped to this response writer.
// For CGI, write plain stdout lines without level prefixes.
func (w *CgiBinWriter) Log(_ slog.Level, args ...any) {
	_, _ = fmt.Fprintln(w, args...)
}

// Print implements jsh/log.PrintKind for request-local console output.
func (w *CgiBinWriter) Print(args ...any) {
	_, _ = fmt.Fprint(w, args...)
}

// Println implements jsh/log.PrintKind for request-local console output.
func (w *CgiBinWriter) Println(args ...any) {
	_, _ = fmt.Fprintln(w, args...)
}

// Printf implements jsh/log.PrintKind for request-local console output.
func (w *CgiBinWriter) Printf(format string, args ...any) {
	_, _ = fmt.Fprintf(w, format, args...)
}

func (w *CgiBinWriter) Write(p []byte) (n int, err error) {
	if len(p) == 0 {
		return 0, nil
	}
	w.sawOutput = true

	if w.headerParsed {
		if err := w.writeBody(p); err != nil {
			return 0, err
		}
		return len(p), nil
	}

	w.headerBuf = append(w.headerBuf, p...)
	headerEnd, separatorLen := findCgiHeaderEnd(w.headerBuf)
	if headerEnd < 0 {
		return len(p), nil
	}

	buffered := w.headerBuf
	bodyStart := headerEnd + separatorLen
	response, err := parseCgiResponseHeader(buffered[:headerEnd])
	if err != nil {
		return 0, err
	}

	w.headerBuf = nil
	w.headerParsed = true
	w.response = response

	if bodyStart == len(buffered) {
		return len(p), nil
	}
	if err := w.writeBody(buffered[bodyStart:]); err != nil {
		return 0, err
	}
	return len(p), nil
}

func (w *CgiBinWriter) Finalize() error {
	if !w.sawOutput {
		return errors.New("empty response")
	}
	if !w.headerParsed {
		return errors.New("missing header separator")
	}
	if w.response == nil {
		return errors.New("missing response headers")
	}
	if w.bodySeen {
		return nil
	}

	responseType, err := w.response.classify(false)
	if err != nil {
		return err
	}
	if responseType == cgiResponseLocalRedirect {
		return w.processLocalRedirect()
	}
	if err := w.applyResponseHeaders(responseType); err != nil {
		return err
	}
	w.ctx.Writer.WriteHeaderNow()
	return nil
}

func (w *CgiBinWriter) writeBody(p []byte) error {
	responseType, err := w.response.classify(true)
	if err != nil {
		return err
	}
	if responseType == cgiResponseLocalRedirect || responseType == cgiResponseClientRedirect {
		return errors.New("redirect response must not include a message body")
	}
	if !w.headersApplied {
		if err := w.applyResponseHeaders(responseType); err != nil {
			return err
		}
	}
	w.bodySeen = true
	if w.ctx.Request.Method == http.MethodHead {
		return nil
	}
	for len(p) > 0 {
		n, writeErr := w.ctx.Writer.Write(p)
		if writeErr != nil {
			return writeErr
		}
		if n <= 0 {
			return io.ErrShortWrite
		}
		p = p[n:]
	}
	return nil
}

func findCgiHeaderEnd(p []byte) (int, int) {
	crlfIndex := bytes.Index(p, []byte("\r\n\r\n"))
	lfIndex := bytes.Index(p, []byte("\n\n"))
	if crlfIndex >= 0 && (lfIndex < 0 || crlfIndex < lfIndex) {
		return crlfIndex, 4
	}
	if lfIndex >= 0 {
		return lfIndex, 2
	}
	return -1, 0
}

func splitCgiHeaderLine(line string) (string, string, bool) {
	colonIndex := strings.Index(line, ":")
	if colonIndex < 0 {
		return "", "", false
	}

	key := strings.TrimSpace(line[:colonIndex])
	value := strings.TrimSpace(line[colonIndex+1:])
	if key == "" {
		return "", "", false
	}
	return key, value, true
}

type cgiResponseType int

const (
	cgiResponseDocument cgiResponseType = iota + 1
	cgiResponseLocalRedirect
	cgiResponseClientRedirect
	cgiResponseClientRedirectWithDocument
)

type cgiResponseMeta struct {
	statusCode  int
	hasStatus   bool
	contentType string
	location    string
	headers     http.Header
}

func parseCgiResponseHeader(headerBlock []byte) (*cgiResponseMeta, error) {
	meta := &cgiResponseMeta{headers: http.Header{}}
	normalized := strings.ReplaceAll(string(headerBlock), "\r\n", "\n")
	firstLine := true
	for _, rawLine := range strings.Split(normalized, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}
		if firstLine {
			firstLine = false
			if statusCode, ok := parseCgiStatusLine(line); ok {
				meta.statusCode = statusCode
				meta.hasStatus = true
				continue
			}
		}

		key, value, ok := splitCgiHeaderLine(line)
		if !ok {
			return nil, fmt.Errorf("malformed header line %q", line)
		}
		switch strings.ToLower(key) {
		case "status":
			if meta.hasStatus {
				return nil, errors.New("duplicate Status header")
			}
			statusCode, err := parseCgiStatusHeaderValue(value)
			if err != nil {
				return nil, err
			}
			meta.statusCode = statusCode
			meta.hasStatus = true
		case "content-type":
			if meta.contentType != "" {
				return nil, errors.New("duplicate Content-Type header")
			}
			meta.contentType = value
		case "location":
			if meta.location != "" {
				return nil, errors.New("duplicate Location header")
			}
			meta.location = value
		default:
			meta.headers.Add(key, value)
		}
	}
	return meta, nil
}

func (meta *cgiResponseMeta) classify(hasBody bool) (cgiResponseType, error) {
	if meta.location != "" {
		if isLocalRedirectTarget(meta.location) {
			if hasBody {
				return 0, errors.New("local redirect must not include a message body")
			}
			if meta.hasStatus || meta.contentType != "" || len(meta.headers) > 0 {
				return 0, errors.New("local redirect must not include headers other than Location")
			}
			return cgiResponseLocalRedirect, nil
		}
		if hasBody {
			if !meta.hasStatus {
				return 0, errors.New("client redirect with document requires Status")
			}
			if meta.contentType == "" {
				return 0, errors.New("client redirect with document requires Content-Type")
			}
			if meta.statusCode < 300 || meta.statusCode >= 400 {
				return 0, errors.New("client redirect with document requires a 3xx status")
			}
			return cgiResponseClientRedirectWithDocument, nil
		}
		if meta.hasStatus {
			return 0, errors.New("client redirect must not include Status unless a document is returned")
		}
		if meta.contentType != "" {
			return 0, errors.New("client redirect must not include Content-Type without a document")
		}
		if !onlyCgiExtensionHeaders(meta.headers) {
			return 0, errors.New("client redirect must not include protocol headers")
		}
		return cgiResponseClientRedirect, nil
	}
	if meta.contentType == "" {
		return 0, errors.New("document response requires Content-Type")
	}
	return cgiResponseDocument, nil
}

func (w *CgiBinWriter) applyResponseHeaders(responseType cgiResponseType) error {
	if w.headersApplied {
		return nil
	}
	for key, values := range w.response.headers {
		if strings.HasPrefix(key, "X-Cgi-") && responseType == cgiResponseClientRedirect {
			continue
		}
		for _, value := range values {
			w.ctx.Writer.Header().Add(key, value)
		}
	}
	switch responseType {
	case cgiResponseDocument:
		w.ctx.Header("Content-Type", w.response.contentType)
		if w.response.hasStatus {
			w.ctx.Status(w.response.statusCode)
		}
	case cgiResponseClientRedirect:
		w.ctx.Header("Location", w.response.location)
		w.ctx.Status(http.StatusFound)
	case cgiResponseClientRedirectWithDocument:
		w.ctx.Header("Location", w.response.location)
		w.ctx.Header("Content-Type", w.response.contentType)
		w.ctx.Status(w.response.statusCode)
	default:
		return fmt.Errorf("unsupported response type %d", responseType)
	}
	w.headersApplied = true
	return nil
}

func (w *CgiBinWriter) processLocalRedirect() error {
	if w.response == nil {
		return errors.New("missing response metadata")
	}
	redirectURL, err := url.Parse(w.response.location)
	if err != nil {
		return err
	}
	engine := w.router
	if engine == nil {
		if w.svr == nil {
			return errors.New("router is unavailable for local redirect")
		}
		engine = w.svr.Router()
	}
	w.ctx.Request.URL.Path = redirectURL.Path
	w.ctx.Request.URL.RawPath = redirectURL.RawPath
	w.ctx.Request.URL.RawQuery = redirectURL.RawQuery
	w.ctx.Request.RequestURI = redirectURL.RequestURI()
	engine.HandleContext(w.ctx)
	return nil
}

func parseCgiStatusHeaderValue(value string) (int, error) {
	fields := strings.Fields(value)
	if len(fields) == 0 {
		return 0, errors.New("empty Status header")
	}
	statusCode, err := strconv.Atoi(fields[0])
	if err != nil {
		return 0, fmt.Errorf("invalid Status header: %w", err)
	}
	return statusCode, nil
}

func parseCgiStatusLine(line string) (int, bool) {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return 0, false
	}
	if !strings.HasPrefix(fields[0], "HTTP") {
		return 0, false
	}
	statusCode, err := strconv.Atoi(fields[1])
	if err != nil {
		return 0, false
	}
	return statusCode, true
}

func isLocalRedirectTarget(location string) bool {
	return strings.HasPrefix(location, "/")
}

func onlyCgiExtensionHeaders(headers http.Header) bool {
	for key := range headers {
		if !strings.HasPrefix(key, "X-Cgi-") {
			return false
		}
	}
	return true
}

type limitedCaptureWriter struct {
	max       int
	buf       bytes.Buffer
	truncated bool
}

func newLimitedCaptureWriter(max int) *limitedCaptureWriter {
	if max <= 0 {
		max = 1024
	}
	return &limitedCaptureWriter{max: max}
}

func (w *limitedCaptureWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	remaining := w.max - w.buf.Len()
	if remaining > 0 {
		toWrite := p
		if len(toWrite) > remaining {
			toWrite = toWrite[:remaining]
		}
		_, _ = w.buf.Write(toWrite)
	}
	if w.buf.Len() >= w.max && len(p) > remaining {
		w.truncated = true
	}
	return len(p), nil
}

func (w *limitedCaptureWriter) String() string {
	if w == nil {
		return ""
	}
	if !w.truncated {
		return w.buf.String()
	}
	return w.buf.String() + "\n...<truncated>"
}

func appendCgiDiagnostic(base string, stdout string, stderr string) string {
	stdout = strings.TrimSpace(stdout)
	stderr = strings.TrimSpace(stderr)
	if stdout == "" && stderr == "" {
		return base
	}
	b := strings.Builder{}
	b.WriteString(base)
	if stdout != "" {
		b.WriteString("; cgi_stdout=")
		b.WriteString(strconv.Quote(stdout))
	}
	if stderr != "" {
		b.WriteString("; cgi_stderr=")
		b.WriteString(strconv.Quote(stderr))
	}
	return b.String()
}
