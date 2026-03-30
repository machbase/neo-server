package server

import (
	"net/http"
	"net/http/httptest"
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
