package httpext

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

type headerLine struct {
	Name  string
	Value string
}

type parsedRequest struct {
	Method  string
	URL     *url.URL
	Version string
	Headers []headerLine
	Body    []byte
}

func executeRawHTTPClient(content string) (string, string, error) {
	req, err := parseHTTPClientFence(content)
	if err != nil {
		return "", "", err
	}

	rawReqBytes := buildRawRequest(req)
	rawReq := string(rawReqBytes)

	respBytes, err := executeRawRequest(req.URL, rawReqBytes)
	if err != nil {
		return rawReq, string(respBytes), err
	}
	return rawReq, string(respBytes), nil
}

func parseHTTPClientFence(content string) (*parsedRequest, error) {
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

	body := []byte(strings.Join(lines[idx:], "\n"))
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

func buildRawRequest(req *parsedRequest) []byte {
	target := req.URL.RequestURI()
	if target == "" {
		target = "/"
	}

	w := &bytes.Buffer{}
	_, _ = fmt.Fprintf(w, "%s %s %s\r\n", req.Method, target, req.Version)

	hasHost := false
	hasContentLength := false
	hasConnection := false
	for _, h := range req.Headers {
		nameLower := strings.ToLower(h.Name)
		if nameLower == "host" {
			hasHost = true
		}
		if nameLower == "content-length" {
			hasContentLength = true
		}
		if nameLower == "connection" {
			hasConnection = true
		}
		_, _ = fmt.Fprintf(w, "%s: %s\r\n", h.Name, h.Value)
	}
	if !hasHost {
		_, _ = fmt.Fprintf(w, "Host: %s\r\n", req.URL.Host)
	}
	if len(req.Body) > 0 && !hasContentLength {
		_, _ = fmt.Fprintf(w, "Content-Length: %d\r\n", len(req.Body))
	}
	if !hasConnection {
		_, _ = io.WriteString(w, "Connection: close\r\n")
	}
	_, _ = io.WriteString(w, "\r\n")
	if len(req.Body) > 0 {
		_, _ = w.Write(req.Body)
	}
	return w.Bytes()
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
