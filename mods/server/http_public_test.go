package server

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestCgiBinWriterDocumentResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/public/app/cgi-bin/test", nil)

	writer := &CgiBinWriter{ctx: ctx}
	_, err := writer.Write([]byte("Status: 201 Created\r\nContent-Type: text/plain\r\nX-Test: ok\r\n\r\nhello"))
	require.NoError(t, err)
	require.NoError(t, writer.Finalize())
	require.Equal(t, http.StatusCreated, recorder.Code)
	require.Equal(t, "hello", recorder.Body.String())
	require.Equal(t, "text/plain", recorder.Header().Get("Content-Type"))
	require.Equal(t, "ok", recorder.Header().Get("X-Test"))
}

func TestCgiBinWriterLocalRedirect(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/public/app/cgi-bin/source", nil)

	router := gin.New()
	router.GET("/public/target", func(ctx *gin.Context) {
		ctx.String(http.StatusOK, "redirected")
	})

	writer := &CgiBinWriter{ctx: ctx, router: router}
	_, err := writer.Write([]byte("Location: /public/target\r\n\r\n"))
	require.NoError(t, err)
	require.NoError(t, writer.Finalize())
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "redirected", recorder.Body.String())
}

func TestCgiBinWriterClientRedirect(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/public/app/cgi-bin/test", nil)

	writer := &CgiBinWriter{ctx: ctx}
	_, err := writer.Write([]byte("Location: https://example.com/next\r\n\r\n"))
	require.NoError(t, err)
	require.NoError(t, writer.Finalize())
	require.Equal(t, http.StatusFound, recorder.Code)
	require.Equal(t, "https://example.com/next", recorder.Header().Get("Location"))
}

func TestCgiBinWriterRejectsBodyWithoutContentType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/public/app/cgi-bin/test", nil)

	writer := &CgiBinWriter{ctx: ctx}
	_, err := writer.Write([]byte("Status: 200 OK\r\n\r\nhello"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "Content-Type")
}

func TestCgiBinWriterAcceptsStatusLineExtension(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/public/app/cgi-bin/test", nil)

	writer := &CgiBinWriter{ctx: ctx}
	_, err := writer.Write([]byte("HTTP/1.1 204 No Content\r\nContent-Type: text/plain\r\n\r\n"))
	require.NoError(t, err)
	require.NoError(t, writer.Finalize())
	require.Equal(t, http.StatusNoContent, recorder.Code)
	require.Equal(t, "text/plain", recorder.Header().Get("Content-Type"))
	require.Empty(t, recorder.Body.String())
}

func TestCgiBinWriterClientRedirectWithDocument(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/public/app/cgi-bin/test", nil)

	writer := &CgiBinWriter{ctx: ctx}
	_, err := writer.Write([]byte("Location: https://example.com/next\r\nStatus: 302 Found\r\nContent-Type: text/html\r\n\r\n<html>redirecting</html>"))
	require.NoError(t, err)
	require.NoError(t, writer.Finalize())
	require.Equal(t, http.StatusFound, recorder.Code)
	require.Equal(t, "https://example.com/next", recorder.Header().Get("Location"))
	require.Equal(t, "<html>redirecting</html>", recorder.Body.String())
}

func TestCgiBinWriterEmptyResponseError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/public/app/cgi-bin/test", nil)

	writer := &CgiBinWriter{ctx: ctx}
	// No Write called at all
	err := writer.Finalize()
	require.Error(t, err)
	require.Contains(t, err.Error(), "empty response")
}

func TestCgiBinWriterMissingHeaderSeparator(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/public/app/cgi-bin/test", nil)

	writer := &CgiBinWriter{ctx: ctx}
	// Write header without separator (no \r\n\r\n), but mark it as saw output
	_, err := writer.Write([]byte("Content-Type: text/plain"))
	require.NoError(t, err)
	err = writer.Finalize()
	require.Error(t, err)
	require.Contains(t, err.Error(), "missing header separator")
}

func TestCgiBinWriterChunkedWrites(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/public/app/cgi-bin/test", nil)

	writer := &CgiBinWriter{ctx: ctx}
	// Write header in two chunks
	_, err := writer.Write([]byte("Content-Type: text/plain\r\n"))
	require.NoError(t, err)
	_, err = writer.Write([]byte("\r\nhello world"))
	require.NoError(t, err)
	require.NoError(t, writer.Finalize())
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "hello world", recorder.Body.String())
}

func TestCgiBinWriterHeadRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodHead, "/public/app/cgi-bin/test", nil)

	writer := &CgiBinWriter{ctx: ctx}
	_, err := writer.Write([]byte("Content-Type: text/plain\r\n\r\nbody to discard"))
	require.NoError(t, err)
	require.NoError(t, writer.Finalize())
	require.Equal(t, http.StatusOK, recorder.Code)
	// HEAD requests should discard body
	require.Empty(t, recorder.Body.String())
}

func TestCgiBinWriterExtensionHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/public/app/cgi-bin/test", nil)

	writer := &CgiBinWriter{ctx: ctx}
	_, err := writer.Write([]byte("Content-Type: text/plain\r\nX-CGI-Custom: value\r\n\r\nok"))
	require.NoError(t, err)
	require.NoError(t, writer.Finalize())
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "value", recorder.Header().Get("X-CGI-Custom"))
	require.Equal(t, "ok", recorder.Body.String())
}

func TestCgiBinWriterLogWritesPlainCgiOutput(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/public/app/cgi-bin/test", nil)

	writer := &CgiBinWriter{ctx: ctx}
	writer.Log(slog.LevelInfo, "Content-Type: text/plain")
	writer.Println()
	writer.Log(slog.LevelInfo, "hello")

	require.NoError(t, writer.Finalize())
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "text/plain", recorder.Header().Get("Content-Type"))
	require.Equal(t, "hello\n", recorder.Body.String())
}

func TestCgiBinWriterPrintWritesWithoutNewline(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/public/app/cgi-bin/test", nil)

	writer := &CgiBinWriter{ctx: ctx}
	writer.Print("Content-Type: text/plain")
	writer.Print("\r\n\r\n")
	writer.Print("hello")

	require.NoError(t, writer.Finalize())
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "text/plain", recorder.Header().Get("Content-Type"))
	require.Equal(t, "hello", recorder.Body.String())
}

// TestCgiBinWriterConcurrentRequests verifies that concurrent CGI requests each
// write their full body without cross-contamination or truncation. Each goroutine
// writes a 50-line body through its own CgiBinWriter and checks completeness.
func TestCgiBinWriterConcurrentRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const goroutines = 50
	const linesPerRequest = 50

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		i := i
		go func() {
			defer wg.Done()

			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest(http.MethodGet, "/public/app/cgi-bin/test", nil)

			w := &CgiBinWriter{ctx: ctx}

			// Simulate console.log writing CGI header
			w.Log(slog.LevelInfo, "Content-Type: text/plain")
			w.Println()

			// Simulate many console.print/println calls (body lines)
			for line := 0; line < linesPerRequest; line++ {
				w.Printf("request=%d line=%d\n", i, line)
			}

			if err := w.Finalize(); err != nil {
				t.Errorf("goroutine %d: Finalize error: %v", i, err)
				return
			}

			body := recorder.Body.String()
			lines := strings.Split(strings.TrimRight(body, "\n"), "\n")
			if len(lines) != linesPerRequest {
				t.Errorf("goroutine %d: expected %d lines, got %d\nbody:\n%s",
					i, linesPerRequest, len(lines), body)
				return
			}
			for j, line := range lines {
				expected := fmt.Sprintf("request=%d line=%d", i, j)
				if line != expected {
					t.Errorf("goroutine %d line %d: got %q, want %q", i, j, line, expected)
					return
				}
			}
		}()
	}
	wg.Wait()
}

// TestCgiBinWriterLargeBody verifies that a large body (>64 KB) is delivered
// completely without truncation.
func TestCgiBinWriterLargeBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/public/app/cgi-bin/test", nil)

	w := &CgiBinWriter{ctx: ctx}

	const lineCount = 2000
	w.Log(slog.LevelInfo, "Content-Type: text/plain")
	w.Println()
	for i := 0; i < lineCount; i++ {
		w.Printf("line %05d: %s\n", i, strings.Repeat("x", 40))
	}

	require.NoError(t, w.Finalize())
	require.Equal(t, http.StatusOK, recorder.Code)

	body := recorder.Body.String()
	lines := strings.Split(strings.TrimRight(body, "\n"), "\n")
	require.Equal(t, lineCount, len(lines), "expected %d lines but got %d", lineCount, len(lines))
	for i, line := range lines {
		expected := fmt.Sprintf("line %05d: %s", i, strings.Repeat("x", 40))
		require.Equal(t, expected, line, "line %d mismatch", i)
	}
}

// TestCgiBinWriterChunkedBodyWrites verifies that a body delivered in many tiny
// writes is reassembled completely on the client side.
func TestCgiBinWriterChunkedBodyWrites(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/public/app/cgi-bin/test", nil)

	w := &CgiBinWriter{ctx: ctx}
	_, err := w.Write([]byte("Content-Type: text/plain\r\n\r\n"))
	require.NoError(t, err)

	// Write body byte-by-byte to trigger every partial-write code path
	body := "abcdefghijklmnopqrstuvwxyz0123456789"
	for _, ch := range body {
		_, err := w.Write([]byte(string(ch)))
		require.NoError(t, err)
	}

	require.NoError(t, w.Finalize())
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, body, recorder.Body.String())
}

func TestCgiBinWriterWriteBodyShortWrite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/public/app/cgi-bin/test", nil)
	ctx.Writer = &shortWriteResponseWriter{ResponseWriter: ctx.Writer, maxBytesPerWrite: 0}

	writer := &CgiBinWriter{ctx: ctx}
	_, err := writer.Write([]byte("Content-Type: text/plain\r\n\r\nhello"))
	require.ErrorIs(t, err, io.ErrShortWrite)
}

func TestCgiBinWriterWriteBodyPartialWriteSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/public/app/cgi-bin/test", nil)
	ctx.Writer = &shortWriteResponseWriter{ResponseWriter: ctx.Writer, maxBytesPerWrite: 1}

	writer := &CgiBinWriter{ctx: ctx}
	_, err := writer.Write([]byte("Content-Type: text/plain\r\n\r\nhello"))
	require.NoError(t, err)
	require.NoError(t, writer.Finalize())
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "hello", recorder.Body.String())
}

type shortWriteResponseWriter struct {
	gin.ResponseWriter
	maxBytesPerWrite int
}

func (w *shortWriteResponseWriter) Write(data []byte) (int, error) {
	if w.maxBytesPerWrite <= 0 {
		return 0, nil
	}
	if len(data) > w.maxBytesPerWrite {
		data = data[:w.maxBytesPerWrite]
	}
	return w.ResponseWriter.Write(data)
}

func TestCgiBinWriterEmptyWrite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/public/app/cgi-bin/test", nil)

	writer := &CgiBinWriter{ctx: ctx}
	// Empty write should be no-op
	n, err := writer.Write([]byte{})
	require.NoError(t, err)
	require.Equal(t, 0, n)
}
