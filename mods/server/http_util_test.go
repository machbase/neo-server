package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/machbase/neo-server/v8/mods/logging"
	"github.com/stretchr/testify/require"
)

func TestHttpLoggerWithFileWritesAccessLog(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logPath := filepath.Join(t.TempDir(), "access.log")

	router := gin.New()
	router.Use(HttpLoggerWithFile("http-util-file", logPath))
	router.GET("/logging/file", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/logging/file?x=1", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	body, err := osReadFileWithRetry(logPath)
	require.NoError(t, err)
	require.Contains(t, string(body), "/logging/file?x=1")
}

func TestHttpLoggerWithFileConfWritesAccessLog(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logPath := filepath.Join(t.TempDir(), "access-conf.log")

	router := gin.New()
	router.Use(HttpLoggerWithFileConf("http-util-file-conf", logging.LogFileConf{
		Filename:             logPath,
		Level:                "DEBUG",
		MaxSize:              1,
		MaxBackups:           1,
		MaxAge:               1,
		Append:               true,
		PrefixWidth:          12,
		EnableSourceLocation: false,
	}))
	router.POST("/logging/file-conf", func(c *gin.Context) {
		_, _ = io.WriteString(c.Writer, "created")
	})

	req := httptest.NewRequest(http.MethodPost, "/logging/file-conf", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	body, err := osReadFileWithRetry(logPath)
	require.NoError(t, err)
	require.Contains(t, string(body), "/logging/file-conf")
}

func TestHttpLoggerWithFilterAndFileConfFallsBackWithoutFile(t *testing.T) {
	gin.SetMode(gin.TestMode)
	filterCalled := false

	router := gin.New()
	router.Use(HttpLoggerWithFilterAndFileConf("http-util-filter", func(req *http.Request, statusCode int, latency time.Duration) bool {
		filterCalled = true
		return false
	}, logging.LogFileConf{}))
	router.GET("/logging/filter", func(c *gin.Context) {
		c.String(http.StatusNoContent, "")
	})

	req := httptest.NewRequest(http.MethodGet, "/logging/filter", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNoContent, rec.Code)
	require.True(t, filterCalled)
}

func TestWithHttpWebDirSetsWrappedAssets(t *testing.T) {
	tempDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "index.html"), []byte("index page"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "app.js"), []byte("console.log('ok')"), 0o644))

	svr := &httpd{}
	WithHttpWebDir(tempDir)(svr)

	require.NotNil(t, svr.uiContentFs)

	assetFile, err := svr.uiContentFs.Open("app.js")
	require.NoError(t, err)
	assetBody, err := io.ReadAll(assetFile)
	require.NoError(t, err)
	require.NoError(t, assetFile.Close())
	require.Equal(t, "console.log('ok')", string(assetBody))

	fallbackFile, err := svr.uiContentFs.Open("missing/route")
	require.NoError(t, err)
	fallbackBody, err := io.ReadAll(fallbackFile)
	require.NoError(t, err)
	require.NoError(t, fallbackFile.Close())
	require.Equal(t, "index page", string(fallbackBody))
}

func osReadFileWithRetry(path string) ([]byte, error) {
	var lastErr error
	for range 10 {
		body, err := os.ReadFile(path)
		if err == nil {
			return body, nil
		}
		lastErr = err
		time.Sleep(10 * time.Millisecond)
	}
	return nil, lastErr
}
