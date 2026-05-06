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
	accessLog := logging.NewLogFile("http-util-file", logging.LogFileConf{
		Filename:             logPath,
		Level:                "DEBUG",
		MaxSize:              10,
		MaxBackups:           2,
		MaxAge:               7,
		Compress:             false,
		Append:               true,
		RotateSchedule:       "@midnight",
		Console:              false,
		PrefixWidth:          20,
		EnableSourceLocation: false,
	})
	closeLogOnCleanup(t, accessLog)
	router.Use(logger(accessLog, nil))
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
	fileConf := logging.LogFileConf{
		Filename:             logPath,
		Level:                "DEBUG",
		MaxSize:              1,
		MaxBackups:           1,
		MaxAge:               1,
		Append:               true,
		PrefixWidth:          12,
		EnableSourceLocation: false,
	}

	router := gin.New()
	accessLog := logging.NewLogFile("http-util-file-conf", fileConf)
	closeLogOnCleanup(t, accessLog)
	router.Use(logger(accessLog, nil))
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

func closeLogOnCleanup(t *testing.T, log logging.Log) {
	t.Helper()
	closer, ok := log.(interface{ Close() error })
	if !ok {
		return
	}
	t.Cleanup(func() {
		require.NoError(t, closer.Close())
	})
}
