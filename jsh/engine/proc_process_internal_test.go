package engine

import (
	"bytes"
	"errors"
	"io/fs"
	"testing"
	"time"
)

type failingProcFS struct {
	*VirtualFS
	failWriteAfter int
	writeCount     int
}

func newFailingProcFS(failWriteAfter int) *failingProcFS {
	return &failingProcFS{
		VirtualFS:      NewVirtualFS(),
		failWriteAfter: failWriteAfter,
	}
}

func (f *failingProcFS) WriteFile(name string, data []byte) error {
	f.writeCount++
	if f.failWriteAfter > 0 && f.writeCount >= f.failWriteAfter {
		return errors.New("write failed")
	}
	return f.VirtualFS.WriteFile(name, data)
}

func newProcTestRuntime(t *testing.T, procFS fs.FS, env map[string]any) *JSRuntime {
	t.Helper()
	conf := Config{
		Name: "proc-internal-test",
		Code: `console.println("ok");`,
		FSTabs: []FSTab{
			{MountPoint: "/proc", FS: procFS},
		},
		Env:    env,
		Reader: &bytes.Buffer{},
		Writer: &bytes.Buffer{},
	}
	jr, err := New(conf)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return jr
}

func TestCreateCurrentProcessEntrySkipsWithoutController(t *testing.T) {
	jr := newProcTestRuntime(t, NewVirtualFS(), map[string]any{
		"PWD": "/work",
	})

	entry, err := jr.createCurrentProcessEntry("jsh", []string{"shell.js"})
	if err != nil {
		t.Fatalf("createCurrentProcessEntry() error = %v", err)
	}
	if entry != nil {
		t.Fatal("createCurrentProcessEntry() returned entry without controller")
	}
}

func TestCreateCurrentProcessEntrySkipsWithoutProcMount(t *testing.T) {
	jr, err := New(Config{
		Name:   "proc-mount-skip-test",
		Code:   `console.println("ok");`,
		FSTabs: []FSTab{{MountPoint: "/", FS: NewVirtualFS()}},
		Env: map[string]any{
			"PWD": "/work",
		},
		Reader: &bytes.Buffer{},
		Writer: &bytes.Buffer{},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	jr.Env.Set(ControllerAddressEnv, "stub://controller")

	entry, err := jr.createCurrentProcessEntry("jsh", []string{"shell.js"})
	if err != nil {
		t.Fatalf("createCurrentProcessEntry() error = %v", err)
	}
	if entry != nil {
		t.Fatal("createCurrentProcessEntry() returned entry without /proc mount")
	}
}

func TestCreateProcProcessEntrySkipsInvalidPID(t *testing.T) {
	jr := newProcTestRuntime(t, NewVirtualFS(), map[string]any{
		"PWD":                "/work",
		ControllerAddressEnv: "stub://controller",
	})

	entry, err := jr.createProcProcessEntry(procProcessMeta{})
	if err != nil {
		t.Fatalf("createProcProcessEntry() error = %v", err)
	}
	if entry != nil {
		t.Fatal("createProcProcessEntry() returned entry for invalid pid")
	}
}

func TestCreateCurrentProcessEntryFailsOnStatusWrite(t *testing.T) {
	procFS := newFailingProcFS(2)
	jr := newProcTestRuntime(t, procFS, map[string]any{
		"PWD":                "/work",
		ControllerAddressEnv: "stub://controller",
	})

	entry, err := jr.createCurrentProcessEntry("jsh", []string{"shell.js"})
	if err == nil {
		t.Fatal("createCurrentProcessEntry() error = nil, want failure")
	}
	if entry != nil {
		t.Fatal("createCurrentProcessEntry() returned entry on write failure")
	}

	if _, statErr := fs.Stat(procFS, "process"); statErr == nil {
		entries, err := fs.ReadDir(procFS, "process")
		if err != nil {
			t.Fatalf("ReadDir(process) error = %v", err)
		}
		if len(entries) != 0 {
			t.Fatalf("process dir should be empty after cleanup, got %d entries", len(entries))
		}
	}
}

func TestProcessEntryRemoveDoesNotDeleteConcurrentTmp(t *testing.T) {
	procFS := NewVirtualFS()
	jr := newProcTestRuntime(t, procFS, map[string]any{
		"PWD":                "/work",
		ControllerAddressEnv: "stub://controller",
	})
	if err := jr.filesystem.Mkdir("/proc/process"); err != nil {
		t.Fatalf("Mkdir(process) error = %v", err)
	}
	if err := jr.filesystem.Mkdir("/proc/process/1234"); err != nil {
		t.Fatalf("Mkdir(process/1234) error = %v", err)
	}

	entry := &procProcessEntry{
		jr:        jr,
		baseDir:   "/proc/process/1234",
		pid:       1234,
		startedAt: time.Unix(100, 0),
	}
	if err := entry.writeMeta(procProcessMeta{Pid: 1234, StartedAt: formatProcProcessTime(entry.startedAt)}); err != nil {
		t.Fatalf("writeMeta() error = %v", err)
	}
	if err := entry.writeStatus("running"); err != nil {
		t.Fatalf("writeStatus() error = %v", err)
	}
	if err := jr.filesystem.WriteFile("/proc/process/1234/.meta.json.tmp", []byte("{}")); err != nil {
		t.Fatalf("WriteFile(.meta.json.tmp) error = %v", err)
	}

	if err := entry.remove(); err != nil {
		t.Fatalf("remove() error = %v", err)
	}
	if _, err := jr.filesystem.Stat("/proc/process/1234/.meta.json.tmp"); err != nil {
		t.Fatalf("concurrent tmp file should remain, stat error = %v", err)
	}
	if _, err := jr.filesystem.Stat("/proc/process/1234/meta.json"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("meta.json stat error = %v, want not exist", err)
	}
	if _, err := jr.filesystem.Stat("/proc/process/1234/status.json"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("status.json stat error = %v, want not exist", err)
	}
}

func TestRunSetsExitCodeOnCompileError(t *testing.T) {
	jr, err := New(Config{
		Name:   "compile-error",
		Code:   `function {`,
		Reader: &bytes.Buffer{},
		Writer: &bytes.Buffer{},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	runErr := jr.Run()
	if runErr == nil {
		t.Fatal("Run() error = nil, want compile failure")
	}
	if jr.ExitCode() != -1 {
		t.Fatalf("ExitCode() = %d, want -1", jr.ExitCode())
	}
}
