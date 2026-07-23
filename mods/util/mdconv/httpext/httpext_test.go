package httpext

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"
)

func startRawServer(t *testing.T, response []byte) (string, func()) {
	t.Helper()
	lsnr, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := lsnr.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
		buf := make([]byte, 4096)
		_, _ = conn.Read(buf)
		_, _ = conn.Write(response)
	}()
	return lsnr.Addr().String(), func() {
		_ = lsnr.Close()
		<-done
	}
}

func readRawHTTPRequest(conn net.Conn) ([]byte, error) {
	br := bufio.NewReader(conn)
	raw := &bytes.Buffer{}
	contentLength := 0

	for {
		line, err := br.ReadString('\n')
		if err != nil {
			return nil, err
		}
		raw.WriteString(line)
		trimmed := strings.TrimRight(line, "\r\n")
		if trimmed == "" {
			break
		}
		if strings.HasPrefix(strings.ToLower(trimmed), "content-length:") {
			v := strings.TrimSpace(strings.SplitN(trimmed, ":", 2)[1])
			n, err := strconv.Atoi(v)
			if err == nil && n > 0 {
				contentLength = n
			}
		}
	}

	if contentLength > 0 {
		body := make([]byte, contentLength)
		if _, err := io.ReadFull(br, body); err != nil {
			return nil, err
		}
		raw.Write(body)
	}
	return raw.Bytes(), nil
}

func splitRawHTTPHeaderBody(raw []byte) ([]byte, []byte, bool) {
	if i := bytes.Index(raw, []byte("\r\n\r\n")); i >= 0 {
		return raw[:i+4], raw[i+4:], true
	}
	if i := bytes.Index(raw, []byte("\n\n")); i >= 0 {
		return raw[:i+2], raw[i+2:], true
	}
	return nil, nil, false
}

func TestExecuteRawHTTPClientCapturesRawHeaders(t *testing.T) {
	response := []byte("HTTP/1.1 200 Weird\r\n" +
		"x-Zeta: 1\r\n" +
		"X-alpha: 2\r\n" +
		"Content-Length: 5\r\n" +
		"\r\n" +
		"hello")
	addr, cleanup := startRawServer(t, response)
	defer cleanup()

	content := fmt.Sprintf("GET http://%s/abc HTTP/1.1\nX-Beta: one\nx-alpha: two\n\n", addr)
	rawReq, rawRsp, err := executeRawHTTPClient(content)
	require.NoError(t, err)

	require.Contains(t, rawReq, "GET /abc HTTP/1.1\r\n")
	require.Contains(t, rawReq, "X-Beta: one\r\n")
	require.Contains(t, rawReq, "x-alpha: two\r\n")
	require.Less(t, strings.Index(rawReq, "X-Beta: one"), strings.Index(rawReq, "x-alpha: two"))

	require.Contains(t, rawRsp, "HTTP/1.1 200 Weird\r\n")
	require.Contains(t, rawRsp, "x-Zeta: 1\r\nX-alpha: 2\r\nContent-Length: 5\r\n\r\nhello")
}

func TestExecuteRawHTTPClientCompressesRequestBodyWhenGzipEncoding(t *testing.T) {
	response := []byte("HTTP/1.1 200 OK\r\nContent-Length: 2\r\n\r\nok")
	requestCh := make(chan []byte, 1)

	lsnr, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := lsnr.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
		reqRaw, err := readRawHTTPRequest(conn)
		if err == nil {
			requestCh <- reqRaw
		}
		_, _ = conn.Write(response)
	}()
	defer func() {
		_ = lsnr.Close()
		<-done
	}()

	body := `{"name":"neo","count":3}`
	content := fmt.Sprintf("POST http://%s/zip HTTP/1.1\nContent-Type: application/json\nContent-Encoding: gzip\nContent-Length: 1\n\n%s", lsnr.Addr().String(), body)

	rawReq, _, err := executeRawHTTPClient(content)
	require.NoError(t, err)

	var capturedReq []byte
	select {
	case capturedReq = <-requestCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for captured request")
	}

	require.Contains(t, rawReq, "Content-Encoding: gzip\r\n")
	headerRaw, bodyRaw, ok := splitRawHTTPHeaderBody(capturedReq)
	require.True(t, ok)
	require.Contains(t, string(headerRaw), "POST /zip HTTP/1.1\r\n")
	require.Contains(t, string(headerRaw), fmt.Sprintf("Content-Length: %d\r\n", len(bodyRaw)))

	gz, err := gzip.NewReader(bytes.NewReader(bodyRaw))
	require.NoError(t, err)
	defer gz.Close()
	decoded, err := io.ReadAll(gz)
	require.NoError(t, err)
	require.Equal(t, body, string(decoded))
}

func TestExtenderRendersRequestAndResponseAsCodeBlocks(t *testing.T) {
	response := []byte("HTTP/1.1 200 OK\r\nContent-Length: 2\r\n\r\nok")
	addr, cleanup := startRawServer(t, response)
	defer cleanup()

	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			&Extender{},
		),
		goldmark.WithRendererOptions(
			html.WithXHTML(),
		),
	)

	src := fmt.Sprintf("## HTTP\n\n```http\nGET http://%s/ping\nX-Test: v\n```\n", addr)
	out := &bytes.Buffer{}
	err := md.Convert([]byte(src), out)
	require.NoError(t, err)

	htmlOut := out.String()
	require.Contains(t, htmlOut, `class="httpext-pre"`)
	require.Equal(t, 1, strings.Count(htmlOut, `class="httpext-pre"`))
	require.NotContains(t, htmlOut, `class="httpext-table"`)
	require.NotContains(t, htmlOut, `class="httpext-lno"`)
	require.Contains(t, htmlOut, `class="httpext-divider"`)
	require.Contains(t, htmlOut, `class="httpext-method">GET</span>`)
	require.Contains(t, htmlOut, `class="httpext-path">/ping</span>`)
	require.Contains(t, htmlOut, `class="httpext-status-code">200</span>`)
	require.NotContains(t, htmlOut, "language-http")
}

func TestExtenderHideRequestOption(t *testing.T) {
	response := []byte("HTTP/1.1 200 OK\r\nContent-Length: 2\r\n\r\nok")
	addr, cleanup := startRawServer(t, response)
	defer cleanup()

	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			&Extender{},
		),
		goldmark.WithRendererOptions(
			html.WithXHTML(),
		),
	)

	src := fmt.Sprintf("## HTTP\n\n```http {hide-request=true}\nGET http://%s/ping\nX-Test: v\n```\n", addr)
	out := &bytes.Buffer{}
	err := md.Convert([]byte(src), out)
	require.NoError(t, err)

	htmlOut := out.String()
	require.NotContains(t, htmlOut, "GET /ping HTTP/1.1")
	require.Contains(t, htmlOut, `class="httpext-status-code">200</span>`)
	require.NotContains(t, htmlOut, `class="httpext-lno"`)
	require.NotContains(t, htmlOut, `class="httpext-divider"`)
}

func TestExtenderTokenClassesAndStyleOverride(t *testing.T) {
	response := []byte("HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: 36\r\n\r\n{\"name\":\"neo\",\"count\":3,\"ok\":true}")
	addr, cleanup := startRawServer(t, response)
	defer cleanup()

	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			&Extender{},
		),
		goldmark.WithRendererOptions(
			html.WithXHTML(),
		),
	)

	src := fmt.Sprintf("## HTTP\n\n```http {style-method=\"color:#ff0000\", style-json-key=\"font-weight:700\"}\nGET http://%s/ping?name=neo&count=3\nX-Test: v\n```\n", addr)
	out := &bytes.Buffer{}
	err := md.Convert([]byte(src), out)
	require.NoError(t, err)

	htmlOut := out.String()
	require.Contains(t, htmlOut, `class="httpext-method"`)
	require.Contains(t, htmlOut, `class="httpext-path"`)
	require.Contains(t, htmlOut, `class="httpext-param-name"`)
	require.Contains(t, htmlOut, `class="httpext-param-value"`)
	require.Contains(t, htmlOut, `class="httpext-header-key"`)
	require.Contains(t, htmlOut, `class="httpext-header-value"`)
	require.Contains(t, htmlOut, `class="httpext-response-protocol"`)
	require.Contains(t, htmlOut, `class="httpext-status-code"`)
	require.Contains(t, htmlOut, `class="httpext-status-message"`)
	require.Contains(t, htmlOut, `class="httpext-json-key"`)
	require.Contains(t, htmlOut, `class="httpext-json-string"`)
	require.Contains(t, htmlOut, `class="httpext-json-number"`)
	require.Contains(t, htmlOut, `class="httpext-json-boolean"`)
	require.Contains(t, htmlOut, `class="httpext-method" style="color:#ff0000"`)
	require.Contains(t, htmlOut, `class="httpext-json-key" style="font-weight:700"`)
}

func TestExtenderUnknownStyleKeyWarning(t *testing.T) {
	response := []byte("HTTP/1.1 200 OK\r\nContent-Length: 2\r\n\r\nok")
	addr, cleanup := startRawServer(t, response)
	defer cleanup()

	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			&Extender{},
		),
		goldmark.WithRendererOptions(
			html.WithXHTML(),
		),
	)

	src := fmt.Sprintf("## HTTP\n\n```http {style-not-allowed=\"color:red\", style-method=\"color:#00f\"}\nGET http://%s/ping\n```\n", addr)
	out := &bytes.Buffer{}
	err := md.Convert([]byte(src), out)
	require.NoError(t, err)

	htmlOut := out.String()
	require.Contains(t, htmlOut, `class="httpext-warning"`)
	require.Contains(t, htmlOut, `unknown style key`)
	require.Contains(t, htmlOut, `style-not-allowed`)
	require.Contains(t, htmlOut, `class="httpext-method" style="color:#00f"`)
}

func TestExtenderLineNumbersOptOut(t *testing.T) {
	response := []byte("HTTP/1.1 200 OK\r\nContent-Length: 2\r\n\r\nok")
	addr, cleanup := startRawServer(t, response)
	defer cleanup()

	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			&Extender{},
		),
		goldmark.WithRendererOptions(
			html.WithXHTML(),
		),
	)

	src := fmt.Sprintf("## HTTP\n\n```http {line-numbers=true}\nGET http://%s/ping\n```\n", addr)
	out := &bytes.Buffer{}
	err := md.Convert([]byte(src), out)
	require.NoError(t, err)

	htmlOut := out.String()
	require.Contains(t, htmlOut, `class="httpext-table"`)
	require.Contains(t, htmlOut, `class="httpext-lno">1</td>`)
	require.Contains(t, htmlOut, `class="httpext-divider-row"`)
	require.Contains(t, htmlOut, `class="httpext-status-code">200</span>`)
}

func TestExtenderDecompressesGzipPrintableBody(t *testing.T) {
	compressed := &bytes.Buffer{}
	gz := gzip.NewWriter(compressed)
	_, err := gz.Write([]byte(`{"success":true,"reason":"ok"}`))
	require.NoError(t, err)
	require.NoError(t, gz.Close())

	response := append([]byte(fmt.Sprintf("HTTP/1.1 200 OK\r\nContent-Encoding: gzip\r\nContent-Type: application/json\r\nContent-Length: %d\r\n\r\n", compressed.Len())), compressed.Bytes()...)
	addr, cleanup := startRawServer(t, response)
	defer cleanup()

	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			&Extender{},
		),
		goldmark.WithRendererOptions(
			html.WithXHTML(),
		),
	)

	src := fmt.Sprintf("## HTTP\n\n```http\nGET http://%s/ping\n```\n", addr)
	out := &bytes.Buffer{}
	err = md.Convert([]byte(src), out)
	require.NoError(t, err)

	htmlOut := out.String()
	require.Contains(t, htmlOut, `class="httpext-json-key"`)
	require.Contains(t, htmlOut, `class="httpext-json-boolean"`)
	require.Contains(t, htmlOut, "\n  ")
	require.Contains(t, htmlOut, `success`)
	require.Contains(t, htmlOut, `reason`)
	require.Contains(t, htmlOut, `ok`)
}

func TestExtenderIndentOptOut(t *testing.T) {
	body := `{"success":true,"reason":"ok"}`
	response := []byte(fmt.Sprintf("HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: %d\r\n\r\n%s", len(body), body))
	addr, cleanup := startRawServer(t, response)
	defer cleanup()

	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			&Extender{},
		),
		goldmark.WithRendererOptions(
			html.WithXHTML(),
		),
	)

	src := fmt.Sprintf("## HTTP\n\n```http {indent=false}\nGET http://%s/ping\n```\n", addr)
	out := &bytes.Buffer{}
	err := md.Convert([]byte(src), out)
	require.NoError(t, err)

	htmlOut := out.String()
	require.Contains(t, htmlOut, `class="httpext-json-punct">{</span><span class="httpext-json-key">&#34;success&#34;</span><span class="httpext-json-punct">:</span><span class="httpext-json-boolean">true</span><span class="httpext-json-punct">,</span><span class="httpext-json-key">&#34;reason&#34;</span><span class="httpext-json-punct">:</span><span class="httpext-json-string">&#34;ok&#34;</span><span class="httpext-json-punct">}</span>`)
	require.Contains(t, htmlOut, `class="httpext-json-key"`)
	require.Contains(t, htmlOut, `class="httpext-json-boolean"`)
}

func TestExtenderCSVRainbowColumns(t *testing.T) {
	response := []byte("HTTP/1.1 200 OK\r\nContent-Type: text/csv\r\nContent-Length: 43\r\n\r\nid,name,score\n1,alice,97\n2,\"kim,neo\",88")
	addr, cleanup := startRawServer(t, response)
	defer cleanup()

	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			&Extender{},
		),
		goldmark.WithRendererOptions(
			html.WithXHTML(),
		),
	)

	src := fmt.Sprintf("## HTTP\n\n```http\nGET http://%s/csv\n```\n", addr)
	out := &bytes.Buffer{}
	err := md.Convert([]byte(src), out)
	require.NoError(t, err)

	htmlOut := out.String()
	require.Contains(t, htmlOut, `class="httpext-csv-col-0 httpext-csv-col-p0">id</span><span class="httpext-csv-delim">,</span><span class="httpext-csv-col-1 httpext-csv-col-p1">name</span><span class="httpext-csv-delim">,</span><span class="httpext-csv-col-2 httpext-csv-col-p2">score</span>`)
	require.Contains(t, htmlOut, `class="httpext-csv-col-1 httpext-csv-col-p1">&#34;kim,neo&#34;</span>`)
}

func TestExtenderCSVColumnStyleOverride(t *testing.T) {
	response := []byte("HTTP/1.1 200 OK\r\nContent-Type: text/csv\r\nContent-Length: 11\r\n\r\na,b,c\n1,2,3")
	addr, cleanup := startRawServer(t, response)
	defer cleanup()

	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			&Extender{},
		),
		goldmark.WithRendererOptions(
			html.WithXHTML(),
		),
	)

	src := fmt.Sprintf("## HTTP\n\n```http {style-csv-col-1=\"font-weight:700\", style-csv-delim=\"opacity:0.5\"}\nGET http://%s/csv\n```\n", addr)
	out := &bytes.Buffer{}
	err := md.Convert([]byte(src), out)
	require.NoError(t, err)

	htmlOut := out.String()
	require.Contains(t, htmlOut, `class="httpext-csv-col-1 httpext-csv-col-p1" style="font-weight:700">b</span>`)
	require.Contains(t, htmlOut, `class="httpext-csv-delim" style="opacity:0.5">,</span>`)
}

func TestExecuteRawHTTPClientChunkedBodyDecoded(t *testing.T) {
	chunk1 := "{\"CODE\":\"ZAVX\",\"PRICE\":1}\n"
	chunk2 := "{\"CODE\":\"ZAVX\",\"PRICE\":2}\n"
	response := []byte("HTTP/1.1 200 OK\r\n" +
		"Content-Type: application/x-ndjson\r\n" +
		"Transfer-Encoding: chunked\r\n" +
		"\r\n" +
		fmt.Sprintf("%x\r\n%s\r\n", len(chunk1), chunk1) +
		fmt.Sprintf("%x\r\n%s\r\n", len(chunk2), chunk2) +
		"0\r\n\r\n")
	addr, cleanup := startRawServer(t, response)
	defer cleanup()

	content := fmt.Sprintf("GET http://%s/ndjson HTTP/1.1\n\n", addr)
	rawReq, rawRsp, err := executeRawHTTPClient(content)
	require.NoError(t, err)

	require.Contains(t, rawReq, "GET /ndjson HTTP/1.1\r\n")
	require.Contains(t, rawRsp, "Transfer-Encoding: chunked\r\n")
	require.Contains(t, rawRsp, "\r\n\r\n{\"CODE\":\"ZAVX\",\"PRICE\":1}\n{\"CODE\":\"ZAVX\",\"PRICE\":2}\n")
	require.NotContains(t, rawRsp, "\r\n1e\r\n")
	require.NotContains(t, rawRsp, "\r\n0\r\n\r\n")
}

func TestExtenderCSVRainbowColumnsWithPipeDelimiter(t *testing.T) {
	response := []byte("HTTP/1.1 200 OK\r\nContent-Type: text/csv\r\nContent-Length: 43\r\n\r\nid|name|score\n1|alice|97\n2|\"kim|neo\"|88")
	addr, cleanup := startRawServer(t, response)
	defer cleanup()

	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			&Extender{},
		),
		goldmark.WithRendererOptions(
			html.WithXHTML(),
		),
	)

	src := fmt.Sprintf("## HTTP\n\n```http\nGET http://%s/csv\n```\n", addr)
	out := &bytes.Buffer{}
	err := md.Convert([]byte(src), out)
	require.NoError(t, err)

	htmlOut := out.String()
	require.Contains(t, htmlOut, `class="httpext-csv-col-0 httpext-csv-col-p0">id</span><span class="httpext-csv-delim">|</span><span class="httpext-csv-col-1 httpext-csv-col-p1">name</span><span class="httpext-csv-delim">|</span><span class="httpext-csv-col-2 httpext-csv-col-p2">score</span>`)
	require.Contains(t, htmlOut, `class="httpext-csv-col-1 httpext-csv-col-p1">&#34;kim|neo&#34;</span>`)
}
