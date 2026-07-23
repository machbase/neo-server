package httpdsl

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/machbase/neo-server/v8/mods/util/ssfs"
)

type Exchange struct {
	RequestRaw  string
	ResponseRaw string
}

type headerLine struct {
	Name  string
	Value string
}

type parsedRequest struct {
	Method  string
	URL     *url.URL
	Version string
	Headers []headerLine
	Body    []string
}

type fileDirective struct {
	Path      string
	FromOS    bool
	Directive bool
}

type fileBodyReader struct {
	reader io.Reader
	closer io.Closer
}

func (r *fileBodyReader) Read(p []byte) (int, error) {
	return r.reader.Read(p)
}

func (r *fileBodyReader) Close() error {
	if r.closer == nil {
		return nil
	}
	return r.closer.Close()
}

func Execute(content string) (Exchange, error) {
	req, err := parseHTTPClient(content)
	if err != nil {
		return Exchange{}, err
	}

	rawReqBytes, err := buildRawRequest(req)
	if err != nil {
		return Exchange{}, err
	}
	rawReq := string(rawReqBytes)

	respBytes, err := executeRawRequest(req.URL, rawReqBytes)
	if err != nil {
		return Exchange{RequestRaw: rawReq, ResponseRaw: string(respBytes)}, err
	}
	return Exchange{RequestRaw: rawReq, ResponseRaw: string(respBytes)}, nil
}

func parseHTTPClient(content string) (*parsedRequest, error) {
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	idx := 0
	for idx < len(lines) && strings.TrimSpace(lines[idx]) == "" {
		idx++
	}
	if idx >= len(lines) {
		return nil, fmt.Errorf("http: empty request")
	}

	method, rawURL, version := parseRequestLine(strings.TrimSpace(lines[idx]))
	if method == "" || rawURL == "" {
		return nil, fmt.Errorf("http: invalid request line")
	}
	idx++

	headers := []headerLine{}
	for idx < len(lines) {
		trimmed := strings.TrimSpace(lines[idx])
		if trimmed == "" {
			idx++
			break
		}
		if strings.HasPrefix(trimmed, "?") {
			rawURL += trimmed
			idx++
			continue
		}
		if strings.HasPrefix(trimmed, "&") {
			rawURL += trimmed
			idx++
			continue
		}
		if strings.HasPrefix(trimmed, "HTTP/") && version == "" {
			version = trimmed
			idx++
			continue
		}
		split := strings.SplitN(trimmed, ":", 2)
		if len(split) != 2 {
			return nil, fmt.Errorf("http: invalid header line %q", trimmed)
		}
		headers = append(headers, headerLine{Name: strings.TrimSpace(split[0]), Value: strings.TrimSpace(split[1])})
		idx++
	}

	body := lines[idx:]
	rawURL = normalizeRawURLQuery(rawURL)
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("http: invalid URL: %w", err)
	}
	if !u.IsAbs() || u.Host == "" {
		return nil, fmt.Errorf("http: absolute URL is required")
	}
	if version == "" {
		version = "HTTP/1.1"
	}
	return &parsedRequest{Method: method, URL: u, Version: version, Headers: headers, Body: body}, nil
}

func normalizeRawURLQuery(rawURL string) string {
	if !strings.Contains(rawURL, "?") {
		return rawURL
	}
	parts := strings.SplitN(rawURL, "?", 2)
	if len(parts) < 2 {
		return rawURL
	}
	params := url.Values{}
	for _, part := range strings.Split(parts[1], "&") {
		if strings.TrimSpace(part) == "" {
			continue
		}
		kv := strings.SplitN(part, "=", 2)
		key := strings.TrimSpace(kv[0])
		if len(kv) == 2 {
			params.Add(key, strings.TrimSpace(kv[1]))
		} else {
			params.Add(key, "")
		}
	}
	encoded := params.Encode()
	if encoded == "" {
		return parts[0]
	}
	return parts[0] + "?" + encoded
}

var requestVersionRegexp = regexp.MustCompile(`^(.*?)(?:\s+(HTTP/(?:\d|\d\.\d)))?$`)

func parseRequestLine(line string) (method, rawURL, version string) {
	var params string
	if strings.Contains(line, "?") {
		parts := strings.SplitN(line, "?", 2)
		if len(parts) > 1 {
			toks := requestVersionRegexp.FindStringSubmatch(parts[1])
			if len(toks) > 1 {
				params = toks[1]
				if len(toks) > 2 {
					version = toks[2]
				}
			} else {
				params = parts[1]
			}
		}
		line = parts[0]
	}
	parts := strings.Fields(line)
	if len(parts) < 2 {
		return "", "", ""
	}
	method = parts[0]
	rawURL = parts[1]
	if len(parts) > 2 {
		version = parts[2]
	}
	if params != "" {
		rawURL += "?" + params
	}
	return method, rawURL, version
}

func buildRawRequest(req *parsedRequest) ([]byte, error) {
	target := req.URL.RequestURI()
	if target == "" {
		target = "/"
	}
	body, err := resolveRequestBody(req.Method, req.Headers, req.Body)
	if err != nil {
		return nil, err
	}
	if len(body) > 0 && requestBodyShouldBeGzipped(req.Headers) {
		compressed, err := gzipCompress(body)
		if err != nil {
			return nil, fmt.Errorf("http: gzip request body failed: %w", err)
		}
		body = compressed
	}

	w := &bytes.Buffer{}
	_, _ = fmt.Fprintf(w, "%s %s %s\r\n", req.Method, target, req.Version)

	hasHost := false
	hasConnection := false
	for _, h := range req.Headers {
		nameLower := strings.ToLower(h.Name)
		if nameLower == "host" {
			hasHost = true
		}
		if nameLower == "content-length" {
			if len(body) > 0 {
				continue
			}
		}
		if nameLower == "connection" {
			hasConnection = true
		}
		_, _ = fmt.Fprintf(w, "%s: %s\r\n", h.Name, h.Value)
	}
	if !hasHost {
		_, _ = fmt.Fprintf(w, "Host: %s\r\n", req.URL.Host)
	}
	if len(body) > 0 {
		_, _ = fmt.Fprintf(w, "Content-Length: %d\r\n", len(body))
	}
	if !hasConnection {
		_, _ = io.WriteString(w, "Connection: close\r\n")
	}
	_, _ = io.WriteString(w, "\r\n")
	if len(body) > 0 {
		_, _ = w.Write(body)
	}
	return w.Bytes(), nil
}

func resolveRequestBody(method string, headers []headerLine, lines []string) ([]byte, error) {
	if len(lines) == 0 {
		return nil, nil
	}
	contentType := strings.ToLower(headerValue(headers, "Content-Type"))

	if contentType == "application/x-www-form-urlencoded" {
		b := &strings.Builder{}
		for i, line := range lines {
			b.WriteString(line)
			if i != 0 && !strings.HasPrefix(line, "&") {
				b.WriteString("\n")
			}
		}
		return []byte(b.String()), nil
	}

	if strings.Contains(contentType, "multipart/form-data") {
		out := &bytes.Buffer{}
		for _, line := range lines {
			r, err := parseFileLine(method, line)
			if err != nil {
				_, _ = out.WriteString(fmt.Sprintf("Error opening file: %v", err))
				continue
			}
			if r == nil {
				_, _ = out.WriteString(line + "\n")
				continue
			}
			if _, err := io.Copy(out, r); err != nil {
				return nil, err
			}
			if closer, ok := r.(io.Closer); ok {
				_ = closer.Close()
			}
		}
		return out.Bytes(), nil
	}

	r, err := parseFileLine(method, lines[0])
	if err != nil {
		return []byte(fmt.Sprintf("Error opening file: %v", err)), nil
	}
	if r != nil {
		out := &bytes.Buffer{}
		if _, err := io.Copy(out, r); err != nil {
			return nil, err
		}
		if closer, ok := r.(io.Closer); ok {
			_ = closer.Close()
		}
		for _, line := range lines[1:] {
			r, err := parseFileLine(method, line)
			if err != nil {
				_, _ = out.WriteString(fmt.Sprintf("Error opening file: %v", err))
				continue
			}
			if r == nil {
				_, _ = out.WriteString(line + "\n")
				continue
			}
			if _, err := io.Copy(out, r); err != nil {
				return nil, err
			}
			if closer, ok := r.(io.Closer); ok {
				_ = closer.Close()
			}
		}
		return out.Bytes(), nil
	}

	return []byte(strings.Join(lines, "\n")), nil
}

func parseFileLine(method string, line string) (io.Reader, error) {
	directive := parseFileDirective(line)
	if strings.EqualFold(method, http.MethodPost) || strings.EqualFold(method, http.MethodPut) || strings.EqualFold(method, http.MethodPatch) {
		if !directive.Directive {
			return nil, nil
		}
	} else if !directive.Directive {
		return strings.NewReader(line + "\n"), nil
	}

	if !directive.Directive {
		return strings.NewReader(line + "\n"), nil
	}

	if directive.FromOS {
		in, err := os.Open(directive.Path)
		if err != nil {
			return nil, err
		}
		return &fileBodyReader{reader: io.MultiReader(in, strings.NewReader("\n")), closer: in}, nil
	}

	def := ssfs.Default()
	if def == nil {
		return nil, fmt.Errorf("server side file system is not initialized")
	}
	ent, err := def.Get(directive.Path)
	if err != nil {
		return nil, err
	}
	return io.MultiReader(bytes.NewReader(ent.Content), strings.NewReader("\n")), nil
}

func parseFileDirective(line string) fileDirective {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || !strings.HasPrefix(trimmed, "<") {
		return fileDirective{}
	}

	// File directive syntax/semantics:
	// - < path            -> SSFS file by default
	// - < @path           -> OS file (path first char '@')
	// - <@charset path    -> SSFS file (charset token parsed but ignored)
	// - <@charset @path   -> OS file
	// - <@ path           -> SSFS file (no charset token)
	if strings.HasPrefix(trimmed, "< ") {
		path := strings.TrimSpace(strings.TrimPrefix(trimmed, "<"))
		if path == "" {
			return fileDirective{}
		}
		fromOS := strings.HasPrefix(path, "@")
		if fromOS {
			path = strings.TrimSpace(strings.TrimPrefix(path, "@"))
			if path == "" {
				return fileDirective{}
			}
		}
		return fileDirective{Path: path, FromOS: fromOS, Directive: true}
	}

	if strings.HasPrefix(trimmed, "<@") {
		rest := strings.TrimSpace(strings.TrimPrefix(trimmed, "<@"))
		if rest == "" {
			return fileDirective{}
		}
		parts := strings.Fields(rest)
		if len(parts) == 0 {
			return fileDirective{}
		}

		var path string
		if len(parts) == 1 {
			path = parts[0]
		} else {
			path = parts[1]
		}
		fromOS := strings.HasPrefix(path, "@")
		if fromOS {
			path = strings.TrimSpace(strings.TrimPrefix(path, "@"))
			if path == "" {
				return fileDirective{}
			}
		}
		return fileDirective{Path: path, FromOS: fromOS, Directive: true}
	}

	return fileDirective{}
}

func headerValue(headers []headerLine, name string) string {
	for _, h := range headers {
		if strings.EqualFold(h.Name, name) {
			return strings.TrimSpace(h.Value)
		}
	}
	return ""
}

func requestBodyShouldBeGzipped(headers []headerLine) bool {
	for _, h := range headers {
		if !strings.EqualFold(h.Name, "Content-Encoding") {
			continue
		}
		for _, token := range strings.Split(h.Value, ",") {
			if strings.EqualFold(strings.TrimSpace(token), "gzip") {
				return true
			}
		}
	}
	return false
}

func gzipCompress(body []byte) ([]byte, error) {
	buf := &bytes.Buffer{}
	gz := gzip.NewWriter(buf)
	if _, err := gz.Write(body); err != nil {
		_ = gz.Close()
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

type captureConn struct {
	net.Conn
	buf bytes.Buffer
}

func (c *captureConn) Read(p []byte) (int, error) {
	n, err := c.Conn.Read(p)
	if n > 0 {
		_, _ = c.buf.Write(p[:n])
	}
	return n, err
}

func executeRawRequest(u *url.URL, rawReq []byte) ([]byte, error) {
	host := u.Hostname()
	port := u.Port()
	if port == "" {
		if strings.EqualFold(u.Scheme, "https") {
			port = "443"
		} else {
			port = "80"
		}
	}
	addr := net.JoinHostPort(host, port)

	dialer := &net.Dialer{Timeout: 10 * time.Second}
	var baseConn net.Conn
	var err error
	if strings.EqualFold(u.Scheme, "https") {
		baseConn, err = tls.DialWithDialer(dialer, "tcp", addr, &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12})
	} else {
		baseConn, err = dialer.Dial("tcp", addr)
	}
	if err != nil {
		return nil, fmt.Errorf("http: dial failed: %w", err)
	}
	defer baseConn.Close()

	_ = baseConn.SetDeadline(time.Now().Add(30 * time.Second))
	if _, err := baseConn.Write(rawReq); err != nil {
		return nil, fmt.Errorf("http: write request failed: %w", err)
	}

	capConn := &captureConn{Conn: baseConn}
	br := bufio.NewReader(capConn)
	resp, err := http.ReadResponse(br, nil)
	if err != nil {
		return capConn.buf.Bytes(), fmt.Errorf("http: read response failed: %w", err)
	}
	defer resp.Body.Close()
	bodyDecoded, err := io.ReadAll(resp.Body)
	if err != nil {
		return capConn.buf.Bytes(), fmt.Errorf("http: read response body failed: %w", err)
	}

	headerRaw, ok := extractRawHTTPHeader(capConn.buf.Bytes())
	if !ok {
		return capConn.buf.Bytes(), nil
	}
	return append(headerRaw, bodyDecoded...), nil
}

func extractRawHTTPHeader(raw []byte) ([]byte, bool) {
	if i := bytes.Index(raw, []byte("\r\n\r\n")); i >= 0 {
		return raw[:i+4], true
	}
	if i := bytes.Index(raw, []byte("\n\n")); i >= 0 {
		return raw[:i+2], true
	}
	return nil, false
}
