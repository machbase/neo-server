package backup

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/machbase/neo-client/v2/api"
	"github.com/machbase/neo-server/v8/spi"
	"github.com/machbase/neo-server/v8/spi/machsvr"
	"github.com/stretchr/testify/require"
)

var (
	testServer  *machsvr.TestServer
	testHomeDir string
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)

	if isBackupHelperProcess() {
		os.Exit(m.Run())
	}

	if err := setupDefaultSPI(); err != nil {
		panic(err)
	}
	code := m.Run()
	teardownDefaultSPI()
	os.Exit(code)
}

func isBackupHelperProcess() bool {
	return os.Getenv("GO_WANT_BACKUP_RESTORE_HELPER") == "1" ||
		os.Getenv("GO_WANT_BACKUP_VERIFY_HELPER") == "1" ||
		os.Getenv("GO_WANT_BACKUP_LIFECYCLE_HELPER") == "1"
}

func setupDefaultSPI() error {
	testServer := &machsvr.TestServer{}
	testHomeDir = mustAbsPath(filepath.Join("tmp", "machbase_default"))
	if err := os.RemoveAll(testHomeDir); err != nil {
		return err
	}
	testServer.StartServer(filepath.Join("tmp", "machbase_default"))

	db, err := sql.Open("machbase", fmt.Sprintf("host=127.0.0.1; port=%d; user=sys; password=manager", testServer.MachPort()))
	if err != nil {
		return err
	}
	defer db.Close()
	conn, err := db.Conn(context.Background())
	if err != nil {
		return err
	}
	defer conn.Close()

	keyPEM, err := testServer.MachKeyPEM()
	if err != nil {
		return err
	}
	spi.SetDefaultDSN(map[string]string{
		"host":         "127.0.0.1",
		"port":         fmt.Sprintf("%d", testServer.MachPort()),
		"user":         "sys",
		"auth_key_pem": keyPEM,
	})
	return nil
}

func teardownDefaultSPI() {
	if testServer != nil {
		testServer.StopServer()
	}
	if testHomeDir != "" {
		_ = os.RemoveAll(testHomeDir)
	}
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

func TestBackupdStopAndRouter(t *testing.T) {
	s := NewBackupd(WithBackupdBaseDir(t.TempDir()))
	// ensure Stop path is executed at least once
	s.Stop()

	r := gin.New()
	g := r.Group("/api/backup")
	s.HttpRouter(g)

	routes := r.Routes()
	paths := map[string]bool{}
	for _, rt := range routes {
		paths[rt.Method+" "+rt.Path] = true
	}
	require.True(t, paths["GET /api/backup/archives"])
	require.True(t, paths["POST /api/backup/archive"])
	require.True(t, paths["GET /api/backup/archive/status"])
	require.True(t, paths["GET /api/backup/mounts"])
	require.True(t, paths["POST /api/backup/mounts/:name"])
	require.True(t, paths["DELETE /api/backup/mounts/:name"])
}

func TestBackupdHandleArchiveConnectPaths(t *testing.T) {
	t.Run("full backup path reaches connect and returns success", func(t *testing.T) {
		baseDir := newBackupWorkDir(t)
		s := NewBackupd(WithBackupdBaseDir(baseDir))
		payload := `{"type":"database","duration":{"type":"full"},"path":"nested/backup1"}`
		w := performBackupRequest(t, s.handleArchive, http.MethodPost, "/api/backup/archive", payload)
		require.Equal(t, http.StatusOK, w.Code)
		waitBackupSettled(t, s, 5*time.Second)
		_, err := os.Stat(filepath.Join(baseDir, "nested"))
		require.NoError(t, err)
	})

	t.Run("incremental backup path reaches connect and returns success", func(t *testing.T) {
		s := NewBackupd(WithBackupdBaseDir(newBackupWorkDir(t)))
		payload := `{"type":"database","duration":{"type":"incremental","after":"prev/full"},"path":"nested/backup2"}`
		w := performBackupRequest(t, s.handleArchive, http.MethodPost, "/api/backup/archive", payload)
		require.Equal(t, http.StatusOK, w.Code)
		waitBackupSettled(t, s, 5*time.Second)
	})

	t.Run("time backup path reaches connect and returns success", func(t *testing.T) {
		s := NewBackupd(WithBackupdBaseDir(newBackupWorkDir(t)))
		payload := `{"type":"database","duration":{"type":"time","from":"1700000000","to":"1700000600"},"path":"nested/backup3"}`
		w := performBackupRequest(t, s.handleArchive, http.MethodPost, "/api/backup/archive", payload)
		require.Equal(t, http.StatusOK, w.Code)
		waitBackupSettled(t, s, 5*time.Second)
	})

	t.Run("table backup with table name reaches connect and returns success", func(t *testing.T) {
		s := NewBackupd(WithBackupdBaseDir(newBackupWorkDir(t)))
		payload := `{"type":"table","tableName":"demo_table","duration":{"type":"full"},"path":"nested/backup4"}`
		w := performBackupRequest(t, s.handleArchive, http.MethodPost, "/api/backup/archive", payload)
		require.Equal(t, http.StatusOK, w.Code)
		waitBackupSettled(t, s, 5*time.Second)
	})
}

func TestBackupdHandleArchivesAndMountConnectPaths(t *testing.T) {
	origConnector := connectDefault
	t.Cleanup(func() { connectDefault = origConnector })

	t.Run("archives with existing base dir returns data", func(t *testing.T) {
		baseDir := newBackupWorkDir(t)
		require.NoError(t, os.MkdirAll(filepath.Join(baseDir, "archive1"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(baseDir, "archive1", "backup.dat"), []byte("x"), 0o644))
		connectDefault = func(context.Context) (*sql.Conn, error) {
			return newMockSQLConn(t, sqlMockBehavior{
				columns: []string{"PATH", "MOUNTDB"},
			}), nil
		}

		s := NewBackupd(WithBackupdBaseDir(baseDir))
		w := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(w)
		ctx.Request = httptest.NewRequest(http.MethodGet, "/api/backup/archives", nil)
		s.handleArchives(ctx)
		require.Equal(t, http.StatusOK, w.Code)
		body := decodeJSONBody(t, w)
		require.Equal(t, true, body["success"])
		data := body["data"].([]any)
		elm1 := data[0].(map[string]any)
		require.Equal(t, "archive1", elm1["path"])
		require.Equal(t, false, elm1["isMount"], body)
	})

	t.Run("mount with absolute path succeeds", func(t *testing.T) {
		baseDir := newBackupWorkDir(t)
		absPath := filepath.Join(baseDir, "missing")
		capturedSQL := ""
		connectDefault = func(context.Context) (*sql.Conn, error) {
			return newMockSQLConn(t, sqlMockBehavior{
				onExec: func(sqlText string) { capturedSQL = sqlText },
			}), nil
		}
		s := NewBackupd(WithBackupdBaseDir(baseDir))
		w := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(w)
		payload, err := json.Marshal(map[string]string{"path": absPath})
		require.NoError(t, err)
		ctx.Params = gin.Params{{Key: "name", Value: "mount_abs"}}
		ctx.Request = httptest.NewRequest(http.MethodPost, "/api/backup/mounts/mount_abs", strings.NewReader(string(payload)))
		ctx.Request.Header.Set("Content-Type", "application/json")
		s.handleMount(ctx)
		require.Equal(t, http.StatusOK, w.Code, "response: %s", w.Body.String())
		body := decodeJSONBody(t, w)
		require.Equal(t, true, body["success"])
		require.Contains(t, capturedSQL, "MOUNT DATABASE")
		require.Contains(t, capturedSQL, sqlPath(absPath))
	})

	t.Run("mount with relative path succeeds", func(t *testing.T) {
		baseDir := newBackupWorkDir(t)
		capturedSQL := ""
		connectDefault = func(context.Context) (*sql.Conn, error) {
			return newMockSQLConn(t, sqlMockBehavior{
				onExec: func(sqlText string) { capturedSQL = sqlText },
			}), nil
		}
		s := NewBackupd(WithBackupdBaseDir(baseDir))
		w := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(w)
		ctx.Params = gin.Params{{Key: "name", Value: "mount_rel"}}
		ctx.Request = httptest.NewRequest(http.MethodPost, "/api/backup/mounts/mount_rel", strings.NewReader(`{"path":"rel/path"}`))
		ctx.Request.Header.Set("Content-Type", "application/json")
		s.handleMount(ctx)
		require.Equal(t, http.StatusOK, w.Code)
		body := decodeJSONBody(t, w)
		require.Equal(t, true, body["success"])
		require.Contains(t, capturedSQL, "MOUNT DATABASE")
		require.Contains(t, capturedSQL, sqlPath(filepath.Join(baseDir, "rel", "path")))
	})

	t.Run("unmount with unknown mount succeeds", func(t *testing.T) {
		capturedSQL := ""
		connectDefault = func(context.Context) (*sql.Conn, error) {
			return newMockSQLConn(t, sqlMockBehavior{
				onExec: func(sqlText string) { capturedSQL = sqlText },
			}), nil
		}
		s := NewBackupd(WithBackupdBaseDir(newBackupWorkDir(t)))
		w := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(w)
		ctx.Params = gin.Params{{Key: "name", Value: "mount_a"}}
		ctx.Request = httptest.NewRequest(http.MethodDelete, "/api/backup/mounts/mount_a", nil)
		s.handleUnmount(ctx)
		require.Equal(t, http.StatusOK, w.Code)
		body := decodeJSONBody(t, w)
		require.Equal(t, true, body["success"])
		require.Equal(t, "UNMOUNT DATABASE 'mount_a'", capturedSQL)
	})

	t.Run("mounts list succeeds", func(t *testing.T) {
		connectDefault = func(context.Context) (*sql.Conn, error) {
			return newMockSQLConn(t, sqlMockBehavior{
				columns: []string{"NAME", "PATH", "BACKUPTBSID", "BACKUPSCN", "MOUNTDB", "DBBEGINTIME", "DBENDTIME", "BACKUPBEGINTIME", "BACKUPENDTIME", "FLAG"},
			}), nil
		}
		s := NewBackupd(WithBackupdBaseDir(newBackupWorkDir(t)))
		w := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(w)
		ctx.Request = httptest.NewRequest(http.MethodGet, "/api/backup/mounts", nil)
		s.handleMounts(ctx)
		require.Equal(t, http.StatusOK, w.Code)
	})
}

func TestBackupdHandleArchivesAndMountFailurePathsWithInjectedConnector(t *testing.T) {
	origConnector := connectDefault
	t.Cleanup(func() { connectDefault = origConnector })

	t.Run("archives returns bad request on connect error", func(t *testing.T) {
		connectDefault = func(context.Context) (*sql.Conn, error) {
			return nil, assertErr("connect fail")
		}

		s := NewBackupd(WithBackupdBaseDir(newBackupWorkDir(t)))
		w := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(w)
		ctx.Request = httptest.NewRequest(http.MethodGet, "/api/backup/archives", nil)
		s.handleArchives(ctx)

		require.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("archives returns internal error on query failure", func(t *testing.T) {
		connectDefault = func(context.Context) (*sql.Conn, error) {
			return newMockSQLConn(t, sqlMockBehavior{queryErr: assertErr("query fail")}), nil
		}

		s := NewBackupd(WithBackupdBaseDir(newBackupWorkDir(t)))
		require.NoError(t, os.MkdirAll(filepath.Join(s.baseDir, "archive1"), 0o755))
		w := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(w)
		ctx.Request = httptest.NewRequest(http.MethodGet, "/api/backup/archives", nil)
		s.handleArchives(ctx)

		require.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("archives returns internal error on scan failure", func(t *testing.T) {
		connectDefault = func(context.Context) (*sql.Conn, error) {
			return newMockSQLConn(t, sqlMockBehavior{
				columns: []string{"PATH", "MOUNTDB"},
				rows:    [][]driver.Value{{struct{}{}, "mount_name"}},
			}), nil
		}

		s := NewBackupd(WithBackupdBaseDir(newBackupWorkDir(t)))
		require.NoError(t, os.MkdirAll(filepath.Join(s.baseDir, "archive1"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(s.baseDir, "archive1", "backup.dat"), []byte("x"), 0o644))
		w := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(w)
		ctx.Request = httptest.NewRequest(http.MethodGet, "/api/backup/archives", nil)
		s.handleArchives(ctx)

		require.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("mounts returns internal error on query failure", func(t *testing.T) {
		connectDefault = func(context.Context) (*sql.Conn, error) {
			return newMockSQLConn(t, sqlMockBehavior{queryErr: assertErr("query fail")}), nil
		}

		s := NewBackupd(WithBackupdBaseDir(newBackupWorkDir(t)))
		w := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(w)
		ctx.Request = httptest.NewRequest(http.MethodGet, "/api/backup/mounts", nil)
		s.handleMounts(ctx)

		require.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("mounts returns internal error on scan failure", func(t *testing.T) {
		connectDefault = func(context.Context) (*sql.Conn, error) {
			return newMockSQLConn(t, sqlMockBehavior{
				columns: []string{"NAME", "PATH", "BACKUPTBSID", "BACKUPSCN", "MOUNTDB", "DBBEGINTIME", "DBENDTIME", "BACKUPBEGINTIME", "BACKUPENDTIME", "FLAG"},
				rows:    [][]driver.Value{{struct{}{}, "", int64(0), int64(0), "", "", "", "", "", 0}},
			}), nil
		}

		s := NewBackupd(WithBackupdBaseDir(newBackupWorkDir(t)))
		s.cutset = "/"
		w := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(w)
		ctx.Request = httptest.NewRequest(http.MethodGet, "/api/backup/mounts", nil)
		s.handleMounts(ctx)

		require.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("mount returns internal error on exec failure", func(t *testing.T) {
		connectDefault = func(context.Context) (*sql.Conn, error) {
			return newMockSQLConn(t, sqlMockBehavior{execErr: assertErr("exec fail")}), nil
		}

		s := NewBackupd(WithBackupdBaseDir(newBackupWorkDir(t)))
		w := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(w)
		ctx.Params = gin.Params{{Key: "name", Value: "mount_fail"}}
		ctx.Request = httptest.NewRequest(http.MethodPost, "/api/backup/mounts/mount_fail", strings.NewReader(`{"path":"rel/path"}`))
		ctx.Request.Header.Set("Content-Type", "application/json")
		s.handleMount(ctx)

		require.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("unmount returns internal error on exec failure", func(t *testing.T) {
		connectDefault = func(context.Context) (*sql.Conn, error) {
			return newMockSQLConn(t, sqlMockBehavior{execErr: assertErr("exec fail")}), nil
		}

		s := NewBackupd(WithBackupdBaseDir(newBackupWorkDir(t)))
		w := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(w)
		ctx.Params = gin.Params{{Key: "name", Value: "mount_fail"}}
		ctx.Request = httptest.NewRequest(http.MethodDelete, "/api/backup/mounts/mount_fail", nil)
		s.handleUnmount(ctx)

		require.Equal(t, http.StatusInternalServerError, w.Code)
	})
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

func TestBackupLifecycleScenario(t *testing.T) {
	cmd := exec.Command(os.Args[0], "-test.run=^TestBackupLifecycleScenarioHelper$", "--")
	cmd.Env = append(os.Environ(), "GO_WANT_BACKUP_LIFECYCLE_HELPER=1")
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, string(output))
}

func TestBackupLifecycleScenarioHelper(t *testing.T) {
	if os.Getenv("GO_WANT_BACKUP_LIFECYCLE_HELPER") != "1" {
		return
	}

	ctx := context.Background()
	homeDir := mustAbs(t, filepath.Join("tmp", "machbase"))
	backupDir := mustAbs(t, filepath.Join("tmp", "backup", "scenario_db"))

	// Keep the backup test deterministic by resetting the fixed test home path.
	require.NoError(t, os.RemoveAll(homeDir))
	require.NoError(t, os.RemoveAll(filepath.Dir(backupDir)))
	require.NoError(t, os.MkdirAll(filepath.Join(homeDir, "conf"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(homeDir, "trc"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(homeDir, "dbs"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Dir(backupDir), 0o755))

	confSrc := filepath.Join("..", "..", "spi", "machsvr", "test", "testsuite.conf")
	confBytes, err := os.ReadFile(confSrc)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(homeDir, "conf", "machbase.conf"), confBytes, 0o644))

	port := freeTCPPort(t)
	require.NoError(t, machsvr.Initialize(homeDir, port, machsvr.OPT_SIGHANDLER_OFF))
	t.Cleanup(func() {
		machsvr.Finalize()
		_ = os.RemoveAll(homeDir)
		_ = os.RemoveAll(filepath.Dir(backupDir))
	})

	// 1) create database
	require.False(t, machsvr.ExistsDatabase())
	require.NoError(t, machsvr.CreateDatabase())

	db, err := machsvr.NewDatabase(machsvr.DatabaseOption{MaxOpenConn: -1, MaxOpenQuery: -1})
	require.NoError(t, err)
	require.NoError(t, db.Startup())
	t.Cleanup(func() {
		_ = db.Shutdown()
	})

	conn, err := db.ConnectTrust(ctx, "sys")
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = conn.Close()
	})

	const tableName = "backup_e2e_tbl"
	const mountName = "backup_mount"

	var result api.Result

	// 2) create table
	result = conn.Exec(ctx, fmt.Sprintf("CREATE TABLE %s (id INTEGER, name VARCHAR(40))", tableName))
	require.NoError(t, result.Err(), result.Message())

	// 3) insert test data
	result = conn.Exec(ctx, fmt.Sprintf("INSERT INTO %s VALUES(1, 'alpha')", tableName))
	require.NoError(t, result.Err(), result.Message())
	result = conn.Exec(ctx, fmt.Sprintf("INSERT INTO %s VALUES(2, 'beta')", tableName))
	require.NoError(t, result.Err(), result.Message())
	result = conn.Exec(ctx, fmt.Sprintf("EXEC table_flush(%s)", tableName))
	require.NoError(t, result.Err(), result.Message())

	// 4) backup
	result = conn.Exec(ctx, fmt.Sprintf("BACKUP DATABASE INTO DISK = '%s'", sqlPath(backupDir)))
	require.NoError(t, result.Err(), result.Message())
	require.NoError(t, waitForFile(filepath.Join(backupDir, "backup.dat"), 5*time.Second))

	// 5) drop table
	result = conn.Exec(ctx, fmt.Sprintf("DROP TABLE %s", tableName))
	require.NoError(t, result.Err(), result.Message())

	// 6) mount backup
	result = conn.Exec(ctx, fmt.Sprintf("MOUNT DATABASE '%s' TO '%s'", sqlPath(backupDir), mountName))
	require.NoError(t, result.Err(), result.Message())

	// 7) select mounted table
	var mountedDB string
	err = conn.QueryRow(ctx, "SELECT MOUNTDB FROM V$STORAGE_MOUNT_DATABASES").Scan(&mountedDB)
	require.NoError(t, err)
	require.NotEmpty(t, mountedDB)

	var mountedCount int
	candidates := []string{
		fmt.Sprintf("SELECT count(*) FROM %s.sys.%s", mountedDB, tableName),
		fmt.Sprintf("SELECT count(*) FROM %s.sys.%s", mountName, tableName),
		fmt.Sprintf("SELECT count(*) FROM %s.%s", mountedDB, tableName),
		fmt.Sprintf("SELECT count(*) FROM %s..%s", mountedDB, tableName),
		fmt.Sprintf("SELECT count(*) FROM %s@%s", tableName, mountedDB),
		fmt.Sprintf(`SELECT count(*) FROM "%s"."%s"`, mountedDB, tableName),
	}

	queryOK := false
	for _, q := range candidates {
		err = conn.QueryRow(ctx, q).Scan(&mountedCount)
		if err == nil {
			queryOK = true
			break
		}
	}
	require.True(t, queryOK, "mounted table query failed with all known candidates, mountdb=%s", mountedDB)
	require.Equal(t, 2, mountedCount)

	// 8) shutdown
	require.NoError(t, conn.Close())
	require.NoError(t, db.Shutdown())
	machsvr.Finalize()

	// 9) restore database
	runRestoreHelper(t, homeDir, backupDir)
	runVerifyHelper(t, homeDir, tableName, 2)
}

func TestBackupRestoreHelper(t *testing.T) {
	if os.Getenv("GO_WANT_BACKUP_RESTORE_HELPER") != "1" {
		return
	}
	homeDir := os.Getenv("BACKUP_RESTORE_HOME_DIR")
	backupDir := os.Getenv("BACKUP_RESTORE_DIR")
	if homeDir == "" || backupDir == "" {
		os.Exit(2)
	}

	if err := machsvr.Initialize(homeDir, 0, machsvr.OPT_SIGHANDLER_OFF); err != nil {
		fmt.Fprintln(os.Stderr, "initialize:", err)
		os.Exit(3)
	}
	defer machsvr.Finalize()
	if machsvr.ExistsDatabase() {
		if err := machsvr.DestroyDatabase(); err != nil {
			fmt.Fprintln(os.Stderr, "destroy:", err)
			os.Exit(5)
		}
	}

	if err := machsvr.RestoreDatabase(backupDir); err != nil {
		fmt.Fprintln(os.Stderr, "restore:", err)
		os.Exit(4)
	}
	os.Exit(0)
}

func runRestoreHelper(t *testing.T, homeDir, backupDir string) {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestBackupRestoreHelper$", "--")
	cmd.Env = append(os.Environ(),
		"GO_WANT_BACKUP_RESTORE_HELPER=1",
		"BACKUP_RESTORE_HOME_DIR="+homeDir,
		"BACKUP_RESTORE_DIR="+backupDir,
	)
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, string(output))
}

func TestBackupVerifyHelper(t *testing.T) {
	if os.Getenv("GO_WANT_BACKUP_VERIFY_HELPER") != "1" {
		return
	}
	homeDir := os.Getenv("BACKUP_VERIFY_HOME_DIR")
	tableName := os.Getenv("BACKUP_VERIFY_TABLE")
	if homeDir == "" || tableName == "" {
		os.Exit(2)
	}

	if err := machsvr.Initialize(homeDir, 0, machsvr.OPT_SIGHANDLER_OFF); err != nil {
		fmt.Fprintln(os.Stderr, "verify initialize:", err)
		os.Exit(3)
	}
	defer machsvr.Finalize()

	db, err := machsvr.NewDatabase(machsvr.DatabaseOption{MaxOpenConn: -1, MaxOpenQuery: -1})
	if err != nil {
		fmt.Fprintln(os.Stderr, "verify newdb:", err)
		os.Exit(4)
	}
	defer db.Shutdown()
	if err := db.Startup(); err != nil {
		fmt.Fprintln(os.Stderr, "verify startup:", err)
		os.Exit(5)
	}

	conn, err := db.ConnectTrust(context.Background(), "sys")
	if err != nil {
		fmt.Fprintln(os.Stderr, "verify connect:", err)
		os.Exit(6)
	}
	defer conn.Close()

	var count int
	if err := conn.QueryRow(context.Background(), fmt.Sprintf("SELECT count(*) FROM %s", tableName)).Scan(&count); err != nil {
		fmt.Fprintln(os.Stderr, "verify query:", err)
		os.Exit(7)
	}
	if count != 2 {
		fmt.Fprintln(os.Stderr, "verify count:", count)
		os.Exit(8)
	}
	os.Exit(0)
}

func runVerifyHelper(t *testing.T, homeDir, tableName string, _ int) {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestBackupVerifyHelper$", "--")
	cmd.Env = append(os.Environ(),
		"GO_WANT_BACKUP_VERIFY_HELPER=1",
		"BACKUP_VERIFY_HOME_DIR="+homeDir,
		"BACKUP_VERIFY_TABLE="+tableName,
	)
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, string(output))
}

func mustAbs(t *testing.T, p string) string {
	t.Helper()
	abs, err := filepath.Abs(p)
	require.NoError(t, err)
	return abs
}

func mustAbsPath(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		panic(err)
	}
	return abs
}

func freeTCPPort(t *testing.T) int {
	t.Helper()
	lsnr, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer lsnr.Close()
	return lsnr.Addr().(*net.TCPAddr).Port
}

func sqlPath(path string) string {
	if runtime.GOOS == "windows" {
		return strings.ReplaceAll(path, "\\", "\\\\")
	}
	return path
}

func waitForFile(path string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		if st, err := os.Stat(path); err == nil && st.Size() >= 0 {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timeout waiting for file %s", path)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func waitBackupSettled(t *testing.T, s *Backupd, timeout time.Duration) {
	t.Helper()
	start := time.Now()
	deadline := start.Add(timeout)
	sawRunning := false

	for {
		running := s.backup.IsRunning
		if running {
			sawRunning = true
		}

		if sawRunning && !running {
			// Double-check once after a short delay to avoid transient flips.
			time.Sleep(25 * time.Millisecond)
			if !s.backup.IsRunning {
				return
			}
		}

		if !sawRunning && time.Since(start) > 300*time.Millisecond {
			// Backup may have started and completed before polling observed IsRunning=true.
			return
		}

		if time.Now().After(deadline) {
			require.FailNow(t, "backup did not settle before timeout")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func newBackupWorkDir(t *testing.T) string {
	t.Helper()
	name := strings.ReplaceAll(t.Name(), "/", "_")
	baseDir := mustAbs(t, filepath.Join("tmp", "backup-tests", name))
	require.NoError(t, os.RemoveAll(baseDir))
	require.NoError(t, os.MkdirAll(baseDir, 0o755))
	t.Cleanup(func() {
		require.NoError(t, removeAllWithRetry(baseDir, 5*time.Second))
	})
	return baseDir
}

func removeAllWithRetry(path string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		err := os.RemoveAll(path)
		if err == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return err
		}
		time.Sleep(50 * time.Millisecond)
	}
}

var mockSQLDriverSeq atomic.Uint64

type sqlMockBehavior struct {
	queryErr error
	execErr  error
	onExec   func(sqlText string)
	columns  []string
	rows     [][]driver.Value
}

func newMockSQLConn(t *testing.T, behavior sqlMockBehavior) *sql.Conn {
	t.Helper()

	name := fmt.Sprintf("backup-sql-mock-%d", mockSQLDriverSeq.Add(1))
	sql.Register(name, &sqlMockDriver{behavior: behavior})

	db, err := sql.Open(name, "")
	require.NoError(t, err)

	conn, err := db.Conn(context.Background())
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = conn.Close()
		_ = db.Close()
	})

	return conn
}

type sqlMockDriver struct {
	behavior sqlMockBehavior
}

func (d *sqlMockDriver) Open(string) (driver.Conn, error) {
	return &sqlMockConn{behavior: d.behavior}, nil
}

type sqlMockConn struct {
	behavior sqlMockBehavior
}

func (c *sqlMockConn) Prepare(string) (driver.Stmt, error) {
	return nil, assertErr("not implemented")
}

func (c *sqlMockConn) Close() error { return nil }

func (c *sqlMockConn) Begin() (driver.Tx, error) { return nil, assertErr("not implemented") }

func (c *sqlMockConn) ExecContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Result, error) {
	if c.behavior.onExec != nil {
		c.behavior.onExec(query)
	}
	if c.behavior.execErr != nil {
		return nil, c.behavior.execErr
	}
	return driver.RowsAffected(0), nil
}

func (c *sqlMockConn) QueryContext(_ context.Context, _ string, _ []driver.NamedValue) (driver.Rows, error) {
	if c.behavior.queryErr != nil {
		return nil, c.behavior.queryErr
	}
	rows := make([][]driver.Value, len(c.behavior.rows))
	copy(rows, c.behavior.rows)
	return &sqlMockRows{columns: c.behavior.columns, rows: rows}, nil
}

type sqlMockRows struct {
	columns []string
	rows    [][]driver.Value
	index   int
}

func (r *sqlMockRows) Columns() []string {
	return r.columns
}

func (r *sqlMockRows) Close() error { return nil }

func (r *sqlMockRows) Next(dest []driver.Value) error {
	if r.index >= len(r.rows) {
		return io.EOF
	}
	row := r.rows[r.index]
	r.index++
	for i := range dest {
		dest[i] = nil
	}
	for i := 0; i < len(dest) && i < len(row); i++ {
		dest[i] = row[i]
	}
	return nil
}
