package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestBackupdStartSetsCutset(t *testing.T) {
	s := NewBackupd(WithBackupdBaseDir(t.TempDir()))
	require.NoError(t, s.Start())

	expected := "/"
	if runtime.GOOS == "windows" {
		expected = "\\"
	}
	require.Equal(t, expected, s.cutset)
}

func TestBackupdHttpHandler(t *testing.T) {
	s := NewBackupd()
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/backup/ping", nil)

	s.HttpHandler(ctx)

	require.Equal(t, http.StatusOK, w.Code)
	body := decodeJSONBody(t, w)
	require.Equal(t, "pong", body["message"])
}

func TestBackupdHandleArchiveStatus(t *testing.T) {
	t.Run("success when idle", func(t *testing.T) {
		s := NewBackupd()
		s.backup = backupState{IsRunning: false, Info: BackupArchive{Type: "database"}}

		w := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(w)
		ctx.Request = httptest.NewRequest(http.MethodGet, "/api/backup/archive/status", nil)

		s.handleArchiveStatus(ctx)

		require.Equal(t, http.StatusOK, w.Code)
		body := decodeJSONBody(t, w)
		require.Equal(t, true, body["success"])
		require.Equal(t, "success", body["reason"])
	})

	t.Run("internal server error when backup failed", func(t *testing.T) {
		s := NewBackupd()
		s.backup = backupState{IsRunning: false, Message: "backup failed", err: assertErr("failed")}

		w := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(w)
		ctx.Request = httptest.NewRequest(http.MethodGet, "/api/backup/archive/status", nil)

		s.handleArchiveStatus(ctx)

		require.Equal(t, http.StatusInternalServerError, w.Code)
		body := decodeJSONBody(t, w)
		require.Equal(t, false, body["success"])
		require.Equal(t, "backup failed", body["reason"])
	})
}

func TestBackupdHandleArchiveValidation(t *testing.T) {
	t.Run("reject malformed body", func(t *testing.T) {
		s := NewBackupd(WithBackupdBaseDir(t.TempDir()))
		w := performBackupRequest(t, s.handleArchive, http.MethodPost, "/api/backup/archive", "{}")

		require.Equal(t, http.StatusBadRequest, w.Code)
		body := decodeJSONBody(t, w)
		require.Equal(t, false, body["success"])
	})

	t.Run("reject when backup already running", func(t *testing.T) {
		s := NewBackupd(WithBackupdBaseDir(t.TempDir()))
		s.backup.IsRunning = true
		payload := `{"type":"database","duration":{"type":"full"},"path":"backup/a"}`

		w := performBackupRequest(t, s.handleArchive, http.MethodPost, "/api/backup/archive", payload)

		require.Equal(t, http.StatusInternalServerError, w.Code)
		body := decodeJSONBody(t, w)
		require.Equal(t, "backup is running.", body["reason"])
	})

	t.Run("reject table backup without table name", func(t *testing.T) {
		s := NewBackupd(WithBackupdBaseDir(t.TempDir()))
		payload := `{"type":"table","duration":{"type":"full"},"path":"backup/a"}`

		w := performBackupRequest(t, s.handleArchive, http.MethodPost, "/api/backup/archive", payload)

		require.Equal(t, http.StatusBadRequest, w.Code)
		body := decodeJSONBody(t, w)
		require.Equal(t, "table name is empty", body["reason"])
	})

	t.Run("reject invalid backup target type", func(t *testing.T) {
		s := NewBackupd(WithBackupdBaseDir(t.TempDir()))
		payload := `{"type":"invalid","duration":{"type":"full"},"path":"backup/a"}`

		w := performBackupRequest(t, s.handleArchive, http.MethodPost, "/api/backup/archive", payload)

		require.Equal(t, http.StatusBadRequest, w.Code)
		body := decodeJSONBody(t, w)
		require.True(t, strings.Contains(body["reason"].(string), "invalid backup"))
	})

	t.Run("reject invalid duration type", func(t *testing.T) {
		s := NewBackupd(WithBackupdBaseDir(t.TempDir()))
		payload := `{"type":"database","duration":{"type":"unknown"},"path":"backup/a"}`

		w := performBackupRequest(t, s.handleArchive, http.MethodPost, "/api/backup/archive", payload)

		require.Equal(t, http.StatusBadRequest, w.Code)
		body := decodeJSONBody(t, w)
		require.True(t, strings.Contains(body["reason"].(string), "invalid backup type"))
	})
}

func TestBackupdHandleArchivesReturnsEmptyWhenBaseDirMissing(t *testing.T) {
	missingBase := filepath.Join(t.TempDir(), "missing-backup-dir")
	s := NewBackupd(WithBackupdBaseDir(missingBase))

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/backup/archives", nil)

	s.handleArchives(ctx)

	require.Equal(t, http.StatusOK, w.Code)
	body := decodeJSONBody(t, w)
	require.Equal(t, true, body["success"])
	require.Equal(t, "success", body["reason"])
	require.Equal(t, []any{}, body["data"])
}

func TestBackupdHandleMountValidation(t *testing.T) {
	t.Run("reject empty mount name", func(t *testing.T) {
		s := NewBackupd(WithBackupdBaseDir(t.TempDir()))

		w := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(w)
		ctx.Request = httptest.NewRequest(http.MethodPost, "/api/backup/mounts/", strings.NewReader(`{"path":"a"}`))
		ctx.Request.Header.Set("Content-Type", "application/json")

		s.handleMount(ctx)

		require.Equal(t, http.StatusBadRequest, w.Code)
		body := decodeJSONBody(t, w)
		require.Equal(t, "invalid mount name", body["reason"])
	})

	t.Run("reject missing path in body", func(t *testing.T) {
		s := NewBackupd(WithBackupdBaseDir(t.TempDir()))

		w := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(w)
		ctx.Params = gin.Params{{Key: "name", Value: "test_mount"}}
		ctx.Request = httptest.NewRequest(http.MethodPost, "/api/backup/mounts/test_mount", strings.NewReader(`{}`))
		ctx.Request.Header.Set("Content-Type", "application/json")

		s.handleMount(ctx)

		require.Equal(t, http.StatusBadRequest, w.Code)
		body := decodeJSONBody(t, w)
		require.Equal(t, false, body["success"])
	})
}

func TestBackupdHandleUnmountRejectsEmptyName(t *testing.T) {
	s := NewBackupd(WithBackupdBaseDir(t.TempDir()))

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest(http.MethodDelete, "/api/backup/mounts/", nil)

	s.handleUnmount(ctx)

	require.Equal(t, http.StatusBadRequest, w.Code)
	body := decodeJSONBody(t, w)
	require.Equal(t, "invalid mount name", body["reason"])
}

func performBackupRequest(t *testing.T, handler gin.HandlerFunc, method, path, payload string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest(method, path, strings.NewReader(payload))
	ctx.Request.Header.Set("Content-Type", "application/json")
	handler(ctx)
	return w
}

func decodeJSONBody(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	body := map[string]any{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	return body
}

type backupTestErr string

func (e backupTestErr) Error() string { return string(e) }

func assertErr(msg string) error {
	return backupTestErr(msg)
}
