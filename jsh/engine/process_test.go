package engine_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	pathpkg "path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/machbase/neo-server/v8/jsh/engine"
	"github.com/machbase/neo-server/v8/jsh/lib"
	"github.com/machbase/neo-server/v8/jsh/root"
	"github.com/machbase/neo-server/v8/jsh/service"
	"github.com/machbase/neo-server/v8/jsh/test_engine"
)

type watchedProcFS struct {
	*engine.VirtualFS
	created chan struct{}
	once    sync.Once
}

type alwaysFailProcWriteFS struct {
	*engine.VirtualFS
}

func newAlwaysFailProcWriteFS() *alwaysFailProcWriteFS {
	return &alwaysFailProcWriteFS{VirtualFS: engine.NewVirtualFS()}
}

func (f *alwaysFailProcWriteFS) WriteFile(name string, data []byte) error {
	return fmt.Errorf("simulated service-controller overload for %s", name)
}

func newWatchedProcFS() *watchedProcFS {
	return &watchedProcFS{
		VirtualFS: engine.NewVirtualFS(),
		created:   make(chan struct{}),
	}
}

func (w *watchedProcFS) WriteFile(name string, data []byte) error {
	if err := w.VirtualFS.WriteFile(name, data); err != nil {
		return err
	}
	return nil
}

func (w *watchedProcFS) Rename(oldName, newName string) error {
	if err := w.VirtualFS.Rename(oldName, newName); err != nil {
		return err
	}
	if strings.HasSuffix(filepath.ToSlash(newName), "status.json") {
		w.once.Do(func() {
			close(w.created)
		})
	}
	return nil
}

func TestProcess(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "process_env",
			Script: `
				const process = require("process");
				console.println("PATH:", process.env.get("PATH"));
				console.println("PWD:", process.env.get("PWD"));
				console.println("LIBRARY_PATH:", process.env.get("LIBRARY_PATH"));
			`,
			Output: []string{
				"PATH: /work:/sbin:.:/work/node_modules/.bin:./node_modules/.bin:/usr/bin",
				"PWD: /work",
				"LIBRARY_PATH: ./node_modules:/lib:/work/node_modules:/usr/lib",
			},
		},
		{
			Name: "process_expand",
			Script: `
				const process = require("process");
				const expanded1 = process.expand("$HOME/file.txt");
				const expanded2 = process.expand("$HOME/../lib/file.txt");
				console.println("expanded1:", expanded1);
				console.println("expanded2:", expanded2);
			`,
			Output: []string{
				"expanded1: /work/file.txt",
				"expanded2: /work/../lib/file.txt",
			},
		},
		{
			Name: "process_argv",
			Script: `
				const process = require("process");
				console.println("argc:", process.argv.length);
				console.println("argv[1]:", process.argv[1]);
			`,
			Output: []string{
				"argc: 2",
				"argv[1]: process_argv",
			},
		},
		{
			Name: "process_cwd",
			Script: `
				const process = require("process");
				console.println("cwd:", process.cwd());
			`,
			Output: []string{
				"cwd: /work",
			},
		},
		{
			Name: "process_chdir",
			Script: `
				const process = require("process");
				console.println("before:", process.cwd());
				process.chdir("/lib");
				console.println("after:", process.cwd());
			`,
			Output: []string{
				"before: /work",
				"after: /lib",
			},
		},
		{
			Name: "process_chdir_relative",
			Script: `
				const process = require("process");
				console.println("before:", process.cwd());
				process.chdir("../lib");
				console.println("after:", process.cwd());
			`,
			Output: []string{
				"before: /work",
				"after: /lib",
			},
		},

		{
			Name: "process_now",
			Script: `
				const process = require("process");
				const now = process.now();
				console.println("type:", typeof now);
			`,
			// preTest:  func(jr *JSRuntime) { jr.nowFunc = func() time.Time { return time.Unix(1764728536, 0) } },
			// postTest: func(jr *JSRuntime) { jr.nowFunc = time.Now },
			Output: []string{
				"type: object",
			},
		},
	}

	for _, tc := range tests {
		test_engine.RunTest(t, tc)
	}
}

func TestProcessStdin(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "stdin_readLines",
			Script: `
				const process = require("process");
				const lines = process.stdin.readLines();
				console.println("lines:", lines.length);
				lines.forEach((line, i) => {
					console.println("line", i + ":", line);
				});
			`,
			Input: []string{"first line", "second line", "third line"},
			Output: []string{
				"lines: 3",
				"line 0: first line",
				"line 1: second line",
				"line 2: third line",
			},
		},
		{
			Name: "stdin_readLine",
			Script: `
				const process = require("process");
				const line = process.stdin.readLine();
				console.println("got:", line);
			`,
			Input: []string{"hello world"},
			Output: []string{
				"got: hello world",
			},
		},
		{
			Name: "stdin_read",
			Script: `
				const process = require("process");
				const data = process.stdin.read();
				console.println("length:", data.length);
				const lines = data.split("\n").filter(l => l.length > 0);
				console.println("lines:", lines.length);
			`,
			Input: []string{"line1", "line2"},
			Output: []string{
				"length: 12",
				"lines: 2",
			},
		},
		{
			Name: "stdin_readBytes",
			Script: `
				const process = require("process");
				const data = process.stdin.readBytes(5);
				console.println("read:", data);
				console.println("length:", data.length);
			`,
			Input: []string{"hello world"},
			Output: []string{
				"read: hello",
				"length: 5",
			},
		},
		{
			Name: "stdin_readBuffer",
			Script: `
				const process = require("process");
				const data = process.stdin.readBuffer(4);
				const bytes = Array.from(new Uint8Array(data));
				console.println("byteLength:", data.byteLength);
				console.println("bytes:", bytes.join(","));
			`,
			InputBytes: []byte{0x1f, 0x8b, 0x08, 0x00},
			Output: []string{
				"byteLength: 4",
				"bytes: 31,139,8,0",
			},
		},
		{
			Name: "stdin_isTTY",
			Script: `
				const process = require("process");
				const isTTY = process.stdin.isTTY();
				console.println("isTTY:", isTTY);
			`,
			Input: []string{},
			Output: []string{
				"isTTY: false",
			},
		},
		{
			Name: "stdin_empty",
			Script: `
				const process = require("process");
				const lines = process.stdin.readLines();
				const nonEmpty = lines.filter(l => l.length > 0);
				console.println("non-empty lines:", nonEmpty.length);
			`,
			Input: []string{},
			Output: []string{
				"non-empty lines: 0",
			},
		},
		{
			Name: "stdin_process_lines",
			Script: `
				const process = require("process");
				const lines = process.stdin.readLines();
				let total = 0;
				lines.forEach(line => {
					const num = parseInt(line);
					if (!isNaN(num)) {
						total += num;
					}
				});
				console.println("sum:", total);
			`,
			Input: []string{"10", "20", "30"},
			Output: []string{
				"sum: 60",
			},
		},
		{
			Name: "stdin_filter_lines",
			Script: `
				const process = require("process");
				const lines = process.stdin.readLines();
				const filtered = lines.filter(line => line.includes("test"));
				console.println("found:", filtered.length);
				filtered.forEach(line => console.println(line));
			`,
			Input: []string{"test1", "something", "test2", "other", "testing"},
			Output: []string{
				"found: 3",
				"test1",
				"test2",
				"testing",
			},
		},
	}

	for _, tc := range tests {
		test_engine.RunTest(t, tc)
	}
}

func TestProcessExec(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "exec_basic",
			Script: `
				const process = require("process");
				const path = process.which('echo');
				const exitCode = process.exec(path, "hello from exec");
				console.println("exit code:", exitCode);
			`,
			ExecBuilder: testExecBuilder,
			Output: []string{
				"hello from exec",
				"exit code: 0",
			},
		},
		{
			Name: "execString_basic",
			Script: `
				const process = require("process");
				const exitCode = process.execString("console.println('hello from execString')");
				console.println("exit code:", exitCode);
			`,
			ExecBuilder: testExecBuilder,
			Output: []string{
				"hello from execString",
				"exit code: 0",
			},
		},
		{
			Name: "exec_with_args",
			Script: `
				const process = require("process");
				const path = process.which('echo');
				const exitCode = process.exec(path, "arg1", "arg2", "arg3");
				console.println("done");
			`,
			ExecBuilder: testExecBuilder,
			Output: []string{
				"arg1 arg2 arg3",
				"done",
			},
		},
	}

	for _, tc := range tests {
		test_engine.RunTest(t, tc)
	}
}

func TestProcessExecRegistersProcEntry(t *testing.T) {
	tmpDir := t.TempDir()
	servicesDir := filepath.Join(tmpDir, "services")
	backendDir := filepath.Join(tmpDir, "shared")
	if err := os.MkdirAll(servicesDir, 0o755); err != nil {
		t.Fatalf("mkdir services: %v", err)
	}
	if err := os.MkdirAll(backendDir, 0o755); err != nil {
		t.Fatalf("mkdir shared: %v", err)
	}

	ctl, err := service.NewController(&service.ControllerConfig{
		ConfigDir: "/work/services",
		SharedFS: service.ControllerSharedFSConfig{
			BackendDir: "/work/shared",
		},
		Mounts: []engine.FSTab{
			{MountPoint: "/work", FS: os.DirFS(tmpDir)},
		},
	})
	if err != nil {
		t.Fatalf("NewController() error = %v", err)
	}
	if err := ctl.Start(nil); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer ctl.Stop(nil)

	conf := engine.Config{
		Name: "process_proc_entry",
		FSTabs: []engine.FSTab{
			root.RootFSTab(),
			lib.LibFSTab(),
		},
		Env: map[string]any{
			"PATH":               "/work:/sbin",
			"PWD":                "/work",
			"HOME":               "/work",
			"LIBRARY_PATH":       "./node_modules:/lib",
			"SERVICE_CONTROLLER": ctl.Address(),
		},
		ExecBuilder: helperExecBuilder(jshBinPath),
		Reader:      &bytes.Buffer{},
		Writer:      &bytes.Buffer{},
	}
	jr, err := engine.New(conf)
	if err != nil {
		t.Fatalf("engine.New() error = %v", err)
	}
	lib.Enable(jr)

	resultCh := make(chan struct {
		exitCode int
		err      error
	}, 1)
	go func() {
		exitCode, execErr := jr.Exec("/sbin/sleep.js", "1")
		resultCh <- struct {
			exitCode int
			err      error
		}{exitCode: exitCode, err: execErr}
	}()

	processRoot := filepath.Join(backendDir, "process")
	var procDir string
	var meta struct {
		Pid                       int      `json:"pid"`
		Ppid                      int      `json:"ppid"`
		Pgid                      int      `json:"pgid"`
		Command                   string   `json:"command"`
		Args                      []string `json:"args"`
		Cwd                       string   `json:"cwd"`
		StartedAt                 string   `json:"started_at"`
		ServiceControllerClientID string   `json:"service_controller_client_id"`
		ExecPath                  string   `json:"exec_path"`
	}
	var status struct {
		Pid       int    `json:"pid"`
		State     string `json:"state"`
		UpdatedAt string `json:"updated_at"`
		StartedAt string `json:"started_at"`
	}

	deadline := time.Now().Add(5 * time.Second)
	for {
		select {
		case result := <-resultCh:
			t.Fatalf("jr.Exec() finished before proc entry appeared: exitCode=%d err=%v output=%q", result.exitCode, result.err, conf.Writer.(*bytes.Buffer).String())
		default:
		}

		entries, readErr := os.ReadDir(processRoot)
		if readErr == nil && len(entries) > 0 {
			procDir = filepath.Join(processRoot, entries[0].Name())
			metaBytes, metaErr := os.ReadFile(filepath.Join(procDir, "meta.json"))
			statusBytes, statusErr := os.ReadFile(filepath.Join(procDir, "status.json"))
			if metaErr == nil && statusErr == nil {
				if err := json.Unmarshal(metaBytes, &meta); err != nil {
					t.Fatalf("unmarshal meta.json: %v", err)
				}
				if err := json.Unmarshal(statusBytes, &status); err != nil {
					t.Fatalf("unmarshal status.json: %v", err)
				}
				break
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for proc entry under %s output=%q", processRoot, conf.Writer.(*bytes.Buffer).String())
		}
		time.Sleep(20 * time.Millisecond)
	}

	if meta.Pid <= 0 {
		t.Fatalf("meta pid = %d, want > 0", meta.Pid)
	}
	if meta.Ppid != os.Getpid() {
		t.Fatalf("meta ppid = %d, want %d", meta.Ppid, os.Getpid())
	}
	if meta.Pgid <= 0 {
		t.Fatalf("meta pgid = %d, want > 0", meta.Pgid)
	}
	if meta.Cwd != "/work" {
		t.Fatalf("meta cwd = %q, want /work", meta.Cwd)
	}
	if meta.Command == "" {
		t.Fatal("meta command is empty")
	}
	if !strings.Contains(strings.Join(meta.Args, " "), "/sbin/sleep.js") {
		t.Fatalf("meta args = %v, want sleep script path", meta.Args)
	}
	if status.Pid != meta.Pid {
		t.Fatalf("status pid = %d, want %d", status.Pid, meta.Pid)
	}
	if status.State != "running" {
		t.Fatalf("status state = %q, want running", status.State)
	}
	if status.StartedAt != meta.StartedAt {
		t.Fatalf("status started_at = %q, want %q", status.StartedAt, meta.StartedAt)
	}

	result := <-resultCh
	if result.err != nil {
		t.Fatalf("jr.Exec() error = %v", result.err)
	}
	if result.exitCode != 0 {
		t.Fatalf("jr.Exec() exitCode = %d, want 0", result.exitCode)
	}

	removeDeadline := time.Now().Add(15 * time.Second)
	for {
		_, metaErr := os.Stat(filepath.Join(procDir, "meta.json"))
		_, statusErr := os.Stat(filepath.Join(procDir, "status.json"))
		if os.IsNotExist(metaErr) && os.IsNotExist(statusErr) {
			break
		}
		if time.Now().After(removeDeadline) {
			entries, _ := os.ReadDir(procDir)
			names := make([]string, 0, len(entries))
			for _, entry := range entries {
				names = append(names, entry.Name())
			}
			t.Fatalf("proc entry files still exist: %s entries=%v", procDir, names)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestProcessExecDoesNotWriteProcEntryWithoutController(t *testing.T) {
	procDir := t.TempDir()
	conf := engine.Config{
		Name: "process_proc_disabled",
		FSTabs: []engine.FSTab{
			root.RootFSTab(),
			lib.LibFSTab(),
			{MountPoint: "/proc", Source: procDir},
		},
		Env: map[string]any{
			"PATH":         "/work:/sbin",
			"PWD":          "/work",
			"HOME":         "/work",
			"LIBRARY_PATH": "./node_modules:/lib",
		},
		ExecBuilder: helperExecBuilder(jshBinPath),
		Reader:      &bytes.Buffer{},
		Writer:      &bytes.Buffer{},
	}
	jr, err := engine.New(conf)
	if err != nil {
		t.Fatalf("engine.New() error = %v", err)
	}
	lib.Enable(jr)

	exitCode, err := jr.Exec("/sbin/echo.js", "hello")
	if err != nil {
		t.Fatalf("jr.Exec() error = %v", err)
	}
	if exitCode != 0 {
		t.Fatalf("jr.Exec() exitCode = %d, want 0", exitCode)
	}

	if _, statErr := os.Stat(filepath.Join(procDir, "process")); !os.IsNotExist(statErr) {
		t.Fatalf("/proc/process should not exist without controller, err=%v", statErr)
	}
}

func TestProcessExecStressWithProcEntryWriteFailures(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	const workers = 48
	var wg sync.WaitGroup
	errCh := make(chan error, workers)

	for i := 0; i < workers; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()

			procFS := newAlwaysFailProcWriteFS()
			var out bytes.Buffer
			var errOut bytes.Buffer
			conf := engine.Config{
				Name: "process_proc_stress",
				FSTabs: []engine.FSTab{
					root.RootFSTab(),
					lib.LibFSTab(),
					{MountPoint: "/proc-fail", FS: procFS},
				},
				Env: map[string]any{
					"PATH":                 "/work:/sbin",
					"PWD":                  "/work",
					"HOME":                 "/work",
					"LIBRARY_PATH":         "./node_modules:/lib",
					"SERVICE_CONTROLLER":   "stub://controller",
					"SERVICE_SHARED_MOUNT": "/proc-fail",
				},
				ExecBuilder: helperExecBuilder(jshBinPath),
				Reader:      &bytes.Buffer{},
				Writer:      &out,
				ErrorWriter: &errOut,
			}
			jr, err := engine.New(conf)
			if err != nil {
				errCh <- fmt.Errorf("worker %d engine.New() error: %w", i, err)
				return
			}
			lib.Enable(jr)

			want := fmt.Sprintf("worker-%d", i)
			exitCode, err := jr.Exec("/sbin/echo.js", want)
			if err != nil {
				errCh <- fmt.Errorf("worker %d jr.Exec() error: %w output=%q errout=%q", i, err, out.String(), errOut.String())
				return
			}
			if exitCode != 0 {
				errCh <- fmt.Errorf("worker %d exitCode=%d output=%q errout=%q", i, exitCode, out.String(), errOut.String())
				return
			}
			if got := strings.TrimSpace(out.String()); got != want {
				errCh <- fmt.Errorf("worker %d stdout=%q want=%q", i, got, want)
				return
			}
		}()
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Fatal(err)
	}
}

func TestCurrentProcessWritesProcEntryWhenEnabled(t *testing.T) {
	procFS := newWatchedProcFS()

	conf := engine.Config{
		Name: "current_process_proc_entry",
		Code: `
			const end = Date.now() + 500;
			while (Date.now() < end) {
			}
		`,
		FSTabs: []engine.FSTab{
			root.RootFSTab(),
			lib.LibFSTab(),
			{MountPoint: "/proc", FS: procFS},
		},
		Env: map[string]any{
			"PATH":               "/work:/sbin",
			"PWD":                "/work",
			"HOME":               "/work",
			"LIBRARY_PATH":       "./node_modules:/lib",
			"SERVICE_CONTROLLER": "stub://controller",
		},
		ProcRecord:  true,
		ProcCommand: jshBinPath,
		ProcArgs:    []string{"shell.js", "arg1"},
		Reader:      &bytes.Buffer{},
		Writer:      &bytes.Buffer{},
	}
	jr, err := engine.New(conf)
	if err != nil {
		t.Fatalf("engine.New() error = %v", err)
	}
	lib.Enable(jr)

	resultCh := make(chan error, 1)
	go func() {
		resultCh <- jr.Run()
	}()

	select {
	case <-procFS.created:
	case runErr := <-resultCh:
		t.Fatalf("jr.Run() finished before current proc entry appeared: err=%v output=%q", runErr, conf.Writer.(*bytes.Buffer).String())
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for current proc entry signal output=%q", conf.Writer.(*bytes.Buffer).String())
	}

	procDir := pathpkg.Join("process", strconv.Itoa(os.Getpid()))
	var meta struct {
		Pid                       int      `json:"pid"`
		Ppid                      int      `json:"ppid"`
		Pgid                      int      `json:"pgid"`
		Command                   string   `json:"command"`
		Args                      []string `json:"args"`
		Cwd                       string   `json:"cwd"`
		StartedAt                 string   `json:"started_at"`
		ServiceControllerClientID string   `json:"service_controller_client_id"`
		ExecPath                  string   `json:"exec_path"`
	}
	var status struct {
		Pid       int    `json:"pid"`
		State     string `json:"state"`
		UpdatedAt string `json:"updated_at"`
		StartedAt string `json:"started_at"`
	}

	metaBytes, metaErr := fs.ReadFile(procFS, pathpkg.Join(procDir, "meta.json"))
	statusBytes, statusErr := fs.ReadFile(procFS, pathpkg.Join(procDir, "status.json"))
	if metaErr != nil || statusErr != nil {
		t.Fatalf("proc entry files missing metaErr=%v statusErr=%v", metaErr, statusErr)
	}
	if err := json.Unmarshal(metaBytes, &meta); err != nil {
		t.Fatalf("unmarshal meta.json: %v", err)
	}
	if err := json.Unmarshal(statusBytes, &status); err != nil {
		t.Fatalf("unmarshal status.json: %v", err)
	}

	if meta.Pid != os.Getpid() {
		t.Fatalf("meta pid = %d, want %d", meta.Pid, os.Getpid())
	}
	if meta.Ppid != os.Getppid() {
		t.Fatalf("meta ppid = %d, want %d", meta.Ppid, os.Getppid())
	}
	if meta.Command != jshBinPath {
		t.Fatalf("meta command = %q, want %q", meta.Command, jshBinPath)
	}
	if strings.Join(meta.Args, " ") != "shell.js arg1" {
		t.Fatalf("meta args = %v", meta.Args)
	}
	if meta.Cwd != "/work" {
		t.Fatalf("meta cwd = %q, want /work", meta.Cwd)
	}
	if meta.ExecPath != jshBinPath {
		t.Fatalf("meta exec_path = %q, want %q", meta.ExecPath, jshBinPath)
	}
	if status.Pid != meta.Pid {
		t.Fatalf("status pid = %d, want %d", status.Pid, meta.Pid)
	}
	if status.State != "running" {
		t.Fatalf("status state = %q, want running", status.State)
	}
	if status.StartedAt != meta.StartedAt {
		t.Fatalf("status started_at = %q, want %q", status.StartedAt, meta.StartedAt)
	}

	if runErr := <-resultCh; runErr != nil {
		t.Fatalf("jr.Run() error = %v", runErr)
	}

	removeDeadline := time.Now().Add(15 * time.Second)
	for {
		_, metaErr := fs.Stat(procFS, pathpkg.Join(procDir, "meta.json"))
		_, statusErr := fs.Stat(procFS, pathpkg.Join(procDir, "status.json"))
		if os.IsNotExist(metaErr) && os.IsNotExist(statusErr) {
			break
		}
		if time.Now().After(removeDeadline) {
			entries, _ := fs.ReadDir(procFS, procDir)
			names := make([]string, 0, len(entries))
			for _, entry := range entries {
				names = append(names, entry.Name())
			}
			t.Fatalf("proc entry files still exist: /%s entries=%v", procDir, names)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestProcessShutdownHook(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "shutdown_hook_single",
			Script: `
				const process = require("process");
				process.addShutdownHook(() => {
					console.println("cleanup");
				});
				console.println("main");
			`,
			Output: []string{
				"main",
				"cleanup",
			},
		},
		{
			Name: "shutdown_hook_multiple",
			Script: `
				const process = require("process");
				process.addShutdownHook(() => {
					console.println("first hook");
				});
				process.addShutdownHook(() => {
					console.println("second hook");
				});
				console.println("main");
			`,
			Output: []string{
				"main",
				"second hook",
				"first hook",
			},
		},
		{
			Name: "shutdown_hook_process_exit",
			Script: `
				const process = require("process");
				process.addShutdownHook(() => {
					console.println("cleanup");
				});
				console.println("main");
				process.exit(3);
			`,
			Err: "exit status 3",
			Output: []string{
				"main",
				"cleanup",
			},
		},
		{
			Name: "shutdown_hook_panic_isolated",
			Script: `
				const process = require("process");
				process.addShutdownHook(() => {
					console.println("third hook");
				});
				process.addShutdownHook(() => {
					console.println("second hook");
					throw new Error("hook failed");
				});
				process.addShutdownHook(() => {
					console.println("first hook");
				});
				console.println("main");
			`,
			Output: []string{
				"main",
				"first hook",
				"second hook",
				"third hook",
			},
		},
	}

	for _, tc := range tests {
		test_engine.RunTest(t, tc)
	}
}

func TestProcessInfo(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "process_pid",
			Script: `
				const process = require("process");
				console.println("pid type:", typeof process.pid);
				console.println("pid > 0:", process.pid > 0);
			`,
			Output: []string{
				"pid type: number",
				"pid > 0: true",
			},
		},
		{
			Name: "process_platform_arch",
			Script: `
				const process = require("process");
				console.println("platform:", process.platform);
				console.println("arch:", process.arch);
			`,
			Output: []string{
				fmt.Sprintf("platform: %s", runtime.GOOS),
				fmt.Sprintf("arch: %s", runtime.GOARCH),
			},
		},
		{
			Name: "process_version",
			Script: `
				const process = require("process");
				console.println("version:", process.version);
				console.println("has versions:", typeof process.versions);
			`,
			Output: []string{
				"version: jsh-1.0.0",
				"has versions: object",
			},
		},
		{
			Name: "process_stdout",
			Script: `
				const process = require("process");
				process.stdout.write("Hello from stdout\n");
				console.println("stdout written");
			`,
			Output: []string{
				"Hello from stdout",
				"stdout written",
			},
		},
		{
			Name: "process_nextTick",
			Script: `
				const process = require("process");
				console.println("before nextTick");
				process.nextTick(() => {
					console.println("in nextTick");
				});
				console.println("after nextTick");
			`,
			Output: []string{
				"before nextTick",
				"after nextTick",
				"in nextTick",
			},
		},
	}

	for _, tc := range tests {
		test_engine.RunTest(t, tc)
	}
}

func TestProcessResources(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "process_memoryUsage",
			Script: `
				const process = require("process");
				const mem = process.memoryUsage();
				console.println("has rss:", typeof mem.rss);
				console.println("has heapTotal:", typeof mem.heapTotal);
				console.println("has heapUsed:", typeof mem.heapUsed);
			`,
			Output: []string{
				"has rss: number",
				"has heapTotal: number",
				"has heapUsed: number",
			},
		},
		{
			Name: "process_cpuUsage",
			Script: `
				const process = require("process");
				const cpu = process.cpuUsage();
				console.println("has user:", typeof cpu.user);
				console.println("has system:", typeof cpu.system);
			`,
			Output: []string{
				"has user: number",
				"has system: number",
			},
		},
		{
			Name: "process_uptime",
			Script: `
				const process = require("process");
				const uptime = process.uptime();
				console.println("uptime type:", typeof uptime);
				console.println("uptime >= 0:", uptime >= 0);
			`,
			Output: []string{
				"uptime type: number",
				"uptime >= 0: true",
			},
		},
		{
			Name: "process_hrtime",
			Script: `
				const process = require("process");
				const time = process.hrtime();
				console.println("is array:", Array.isArray(time));
				console.println("length:", time.length);
			`,
			Output: []string{
				"is array: true",
				"length: 2",
			},
		},
	}

	for _, tc := range tests {
		test_engine.RunTest(t, tc)
	}
}

func TestProcessEvents(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "process_event_emitter",
			Script: `
				const process = require("process");
				console.println("has on:", typeof process.on);
				console.println("has emit:", typeof process.emit);
				console.println("has removeListener:", typeof process.removeListener);
			`,
			Output: []string{
				"has on: function",
				"has emit: function",
				"has removeListener: function",
			},
		},
		{
			Name: "process_custom_event",
			Script: `
				const process = require("process");
				process.on('test', (msg) => {
					console.println("received:", msg);
				});
				process.emit('test', 'hello');
			`,
			Output: []string{
				"received: hello",
			},
		},
		{
			Name: "process_multiple_listeners",
			Script: `
				const process = require("process");
				process.on('test', () => console.println("listener 1"));
				process.on('test', () => console.println("listener 2"));
				process.emit('test');
			`,
			Output: []string{
				"listener 1",
				"listener 2",
			},
		},
		{
			Name: "process_signal_registration",
			Script: `
				const process = require("process");
				console.println("SIGINT:", process.on('sigint', () => {}) === process);
				console.println("SIGTERM:", process.once('SIGTERM', () => {}) === process);
				console.println("SIGQUIT:", process.once('SIGQUIT', () => {}) === process);
				console.println("watchSignal:", typeof process.watchSignal);
			`,
			Output: []string{
				"SIGINT: true",
				"SIGTERM: true",
				"SIGQUIT: true",
				"watchSignal: undefined",
			},
		},
		{
			Name: "process_signal_event_normalization",
			Script: `
				const process = require("process");
				let count = 0;
				process.on('sigterm', () => {
					count += 1;
					console.println('lowercase');
				});
				process.once('SIGTERM', () => {
					count += 1;
					console.println('canonical');
				});
				process.emit('SIGTERM');
				console.println('count:', count);
			`,
			Output: []string{
				"lowercase",
				"canonical",
				"count: 2",
			},
		},
		{
			Name: "process_custom_term_event_preserved",
			Script: `
				const process = require("process");
				process.once('term', () => console.println('custom term'));
				process.emit('term');
			`,
			Output: []string{
				"custom term",
			},
		},
	}

	for _, tc := range tests {
		test_engine.RunTest(t, tc)
	}
}

func TestProcessSignalForwarding(t *testing.T) {
	if runtime.GOOS == "windows" {
		requireWindowsSignalIntegration(t)
	}

	signals := []string{"SIGINT"}
	if runtime.GOOS != "windows" {
		signals = append(signals, "SIGTERM", "SIGQUIT")
	}

	for _, signalName := range signals {
		t.Run(signalName, func(t *testing.T) {
			lines, waitErr, stderrOutput := runProcessSignalHelper(t, signalName, true)
			if waitErr != nil {
				t.Fatalf("helper failed for %s: %v\nstdout:\n%s\nstderr:\n%s", signalName, waitErr, strings.Join(lines, "\n"), stderrOutput)
			}
			assertLinePresent(t, lines, "ready: "+signalName)
			assertLinePresent(t, lines, "caught: "+signalName)
		})
	}
}

func TestProcessSignalDefaultBehavior(t *testing.T) {
	if runtime.GOOS == "windows" {
		requireWindowsSignalIntegration(t)
	}

	lines, waitErr, stderrOutput := runProcessSignalHelper(t, "SIGINT", false)
	if waitErr == nil {
		t.Fatalf("expected helper without listener to terminate by signal\nstdout:\n%s\nstderr:\n%s", strings.Join(lines, "\n"), stderrOutput)
	}
	assertLinePresent(t, lines, "ready: SIGINT")
	assertLineAbsent(t, lines, "caught: SIGINT")

	lines, waitErr, stderrOutput = runProcessSignalHelper(t, "SIGINT", true)
	if waitErr != nil {
		t.Fatalf("expected helper with listener to exit cleanly: %v\nstdout:\n%s\nstderr:\n%s", waitErr, strings.Join(lines, "\n"), stderrOutput)
	}
	assertLinePresent(t, lines, "ready: SIGINT")
	assertLinePresent(t, lines, "caught: SIGINT")
}

func TestProcessExecSignalCleanup(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("process.exec signal cleanup integration is only covered on unix-like platforms")
	}

	const signalName = "SIGTERM"
	lines, cmd, childPID, stderr := startProcessExecSignalHelper(t, signalName)

	proc, err := os.FindProcess(childPID)
	if err != nil {
		_ = cmd.Process.Kill()
		t.Fatalf("find child process %d: %v", childPID, err)
	}
	if err := proc.Signal(testSignalByName(signalName)); err != nil {
		_ = cmd.Process.Kill()
		t.Fatalf("send %s to child %d: %v", signalName, childPID, err)
	}

	waitCh := make(chan error, 1)
	go func() {
		waitCh <- cmd.Wait()
	}()

	select {
	case err := <-waitCh:
		if err != nil {
			_ = cmd.Process.Kill()
			t.Fatalf("exec helper failed after child signal: %v\nstderr:\n%s", err, stderr.String())
		}
	case <-time.After(5 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatalf("timeout waiting for exec helper after child signal")
	}

	finalLines := collectRemainingLines(lines)
	assertLinePresent(t, finalLines, fmt.Sprintf("child-ready: %d", childPID))
	// Parent must survive SIGINT delivery while process.exec() is waiting.
	// Child behavior may vary by platform/session (caught handler or default terminate).
	assertLineAbsent(t, finalLines, "panic:")
	assertLineAbsent(t, finalLines, "Interrupted:")
	hasParentExitLine := false
	for _, line := range finalLines {
		if strings.HasPrefix(line, "parent exit code:") {
			hasParentExitLine = true
			break
		}
	}
	if !hasParentExitLine {
		t.Fatalf("missing parent exit code line in output:\n%s", strings.Join(finalLines, "\n"))
	}
}

func TestProcessExecParentSignalForwarding(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("process.exec parent signal forwarding integration is only covered on unix-like platforms")
	}
	if info, err := os.Stdin.Stat(); err != nil || (info.Mode()&os.ModeCharDevice) == 0 {
		t.Skip("parent signal forwarding test requires interactive TTY stdin")
	}

	const signalName = "SIGINT"
	lines, cmd, childPID, stderr := startProcessExecSignalHelper(t, signalName)

	if err := cmd.Process.Signal(os.Interrupt); err != nil {
		_ = cmd.Process.Kill()
		t.Fatalf("send %s to parent %d: %v", signalName, cmd.Process.Pid, err)
	}

	waitCh := make(chan error, 1)
	go func() {
		waitCh <- cmd.Wait()
	}()

	select {
	case err := <-waitCh:
		if err != nil {
			_ = cmd.Process.Kill()
			t.Skipf("parent signal forwarding is environment-dependent: %v\nstderr:\n%s", err, stderr.String())
		}
	case <-time.After(5 * time.Second):
		_ = cmd.Process.Kill()
		t.Skip("timeout waiting for exec helper after parent signal (environment-dependent)")
	}

	finalLines := collectRemainingLines(lines)
	assertLinePresent(t, finalLines, fmt.Sprintf("child-ready: %d", childPID))
	// Parent must survive SIGINT delivery while process.exec() is waiting.
	// Child behavior may vary by platform/session (caught handler or default terminate).
	assertLineAbsent(t, finalLines, "panic:")
	assertLineAbsent(t, finalLines, "Interrupted:")
	hasParentExitLine := false
	for _, line := range finalLines {
		if strings.HasPrefix(line, "parent exit code:") {
			hasParentExitLine = true
			break
		}
	}
	if !hasParentExitLine {
		t.Fatalf("missing parent exit code line in output:\n%s", strings.Join(finalLines, "\n"))
	}
}

func TestProcessSignalHelper(t *testing.T) {
	if os.Getenv("GO_WANT_PROCESS_SIGNAL_HELPER") != "1" {
		return
	}

	signalName := os.Getenv("JSH_TEST_SIGNAL")
	listenForSignal := os.Getenv("JSH_TEST_LISTEN_SIGNAL") == "1"
	script := `
		const process = require("process");
		const signalName = process.env.get("TEST_SIGNAL");
		const timer = setInterval(() => {}, 1000);
		setTimeout(() => {
			console.println("timeout:", signalName);
			clearInterval(timer);
		}, 5000);
		console.println("ready:", signalName);
	`
	if listenForSignal {
		script = `
			const process = require("process");
			const signalName = process.env.get("TEST_SIGNAL");
			const timer = setInterval(() => {}, 1000);
			const timeout = setTimeout(() => {
				console.println("timeout:", signalName);
				clearInterval(timer);
			}, 5000);
			process.once(signalName, () => {
				console.println("caught:", signalName);
				clearInterval(timer);
				clearTimeout(timeout);
			});
			console.println("ready:", signalName);
		`
	}
	conf := engine.Config{
		Name: "process_signal_helper",
		Code: script,
		Env: map[string]any{
			"PATH":         "/work:/sbin",
			"PWD":          "/work",
			"HOME":         "/work",
			"LIBRARY_PATH": "./node_modules:/lib",
			"TEST_SIGNAL":  signalName,
		},
		FSTabs: []engine.FSTab{
			root.RootFSTab(),
			lib.LibFSTab(),
		},
		Reader: bytes.NewBuffer(nil),
		Writer: os.Stdout,
	}

	jr, err := engine.New(conf)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	lib.Enable(jr)
	if err := jr.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(3)
	}
	os.Exit(jr.ExitCode())
}

func TestProcessExecSignalHelper(t *testing.T) {
	if os.Getenv("GO_WANT_PROCESS_EXEC_SIGNAL_HELPER") != "1" {
		return
	}

	signalName := os.Getenv("JSH_TEST_SIGNAL")
	execPath := os.Getenv("JSH_TEST_EXEC_BIN")
	if signalName == "" || execPath == "" {
		fmt.Fprintln(os.Stderr, "missing JSH_TEST_SIGNAL or JSH_TEST_EXEC_BIN")
		os.Exit(2)
	}

	conf := engine.Config{
		Name: "process_exec_signal_helper",
		Code: `
			const process = require("process");
			const exitCode = process.exec("/work/process-exec-signal-cleanup.js");
			if (exitCode instanceof Error) {
				throw exitCode;
			}
			console.println("parent exit code:", exitCode);
		`,
		Env: map[string]any{
			"PATH":         "/work:/sbin",
			"PWD":          "/work",
			"HOME":         "/work",
			"LIBRARY_PATH": "./node_modules:/lib",
			"TEST_SIGNAL":  signalName,
		},
		FSTabs: []engine.FSTab{
			root.RootFSTab(),
			lib.LibFSTab(),
		},
		ExecBuilder: helperExecBuilder(execPath),
		Reader:      bytes.NewBuffer(nil),
		Writer:      os.Stdout,
	}

	jr, err := engine.New(conf)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	lib.Enable(jr)
	if err := jr.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(3)
	}
	os.Exit(jr.ExitCode())
}

func runProcessSignalHelper(t *testing.T, signalName string, listenForSignal bool) ([]string, error, string) {
	t.Helper()

	cmd := exec.Command(os.Args[0], "-test.run=^TestProcessSignalHelper$", "--")
	prepareSignalHelperCommand(cmd)
	cmd.Env = append(os.Environ(),
		"GO_WANT_PROCESS_SIGNAL_HELPER=1",
		"JSH_TEST_SIGNAL="+signalName,
		"JSH_TEST_LISTEN_SIGNAL="+boolToEnv(listenForSignal),
	)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	stderr := &bytes.Buffer{}
	cmd.Stderr = stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("start helper: %v", err)
	}

	linesCh := make(chan string, 16)
	scanErrCh := make(chan error, 1)
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			linesCh <- scanner.Text()
		}
		scanErrCh <- scanner.Err()
		close(linesCh)
	}()

	var lines []string
	readyLine := "ready: " + signalName
	readyTimer := time.NewTimer(5 * time.Second)
	defer readyTimer.Stop()

	ready := false
	for !ready {
		select {
		case line, ok := <-linesCh:
			if !ok {
				t.Fatalf("helper exited before readiness for %s\nstdout:\n%s\nstderr:\n%s", signalName, strings.Join(lines, "\n"), stderr.String())
			}
			lines = append(lines, line)
			if line == readyLine {
				ready = true
			}
		case <-readyTimer.C:
			_ = cmd.Process.Kill()
			t.Fatalf("timeout waiting for readiness for %s\nstdout:\n%s\nstderr:\n%s", signalName, strings.Join(lines, "\n"), stderr.String())
		}
	}

	if err := sendTestSignal(cmd, signalName); err != nil {
		_ = cmd.Process.Kill()
		t.Fatalf("send %s: %v", signalName, err)
	}

	drainTimer := time.NewTimer(5 * time.Second)
	defer drainTimer.Stop()
	for linesCh != nil {
		select {
		case line, ok := <-linesCh:
			if !ok {
				linesCh = nil
				continue
			}
			lines = append(lines, line)
		case <-drainTimer.C:
			_ = cmd.Process.Kill()
			t.Fatalf("timeout draining helper output for %s\nstdout:\n%s\nstderr:\n%s", signalName, strings.Join(lines, "\n"), stderr.String())
		}
	}
	if err := <-scanErrCh; err != nil {
		t.Fatalf("scan helper output for %s: %v", signalName, err)
	}

	waitCh := make(chan error, 1)
	go func() {
		waitCh <- cmd.Wait()
	}()

	var waitErr error
	select {
	case err := <-waitCh:
		waitErr = err
	case <-time.After(5 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatalf("timeout waiting for helper exit for %s\nstdout:\n%s\nstderr:\n%s", signalName, strings.Join(lines, "\n"), stderr.String())
	}

	return lines, waitErr, stderr.String()
}

func startProcessExecSignalHelper(t *testing.T, signalName string) (<-chan string, *exec.Cmd, int, *bytes.Buffer) {
	t.Helper()

	cmd := exec.Command(os.Args[0], "-test.run=^TestProcessExecSignalHelper$", "--")
	prepareSignalHelperCommand(cmd)
	cmd.Env = append(os.Environ(),
		"GO_WANT_PROCESS_EXEC_SIGNAL_HELPER=1",
		"JSH_TEST_SIGNAL="+signalName,
		"JSH_TEST_EXEC_BIN="+jshBinPath,
	)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	stderr := &bytes.Buffer{}
	cmd.Stderr = stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("start exec helper: %v", err)
	}

	linesCh := make(chan string, 16)
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			linesCh <- scanner.Text()
		}
		close(linesCh)
	}()

	var lines []string
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()

	for {
		select {
		case line, ok := <-linesCh:
			if !ok {
				t.Fatalf("exec helper exited before child readiness\nstdout:\n%s\nstderr:\n%s", strings.Join(lines, "\n"), stderr.String())
			}
			lines = append(lines, line)
			if strings.HasPrefix(line, "child-ready: ") {
				childPID, err := strconv.Atoi(strings.TrimPrefix(line, "child-ready: "))
				if err != nil {
					_ = cmd.Process.Kill()
					t.Fatalf("parse child pid from %q: %v", line, err)
				}

				buffered := make(chan string, 16)
				for _, existing := range lines {
					buffered <- existing
				}
				go func() {
					for line := range linesCh {
						buffered <- line
					}
					close(buffered)
				}()
				return buffered, cmd, childPID, stderr
			}
		case <-timer.C:
			_ = cmd.Process.Kill()
			t.Fatalf("timeout waiting for exec child readiness\nstdout:\n%s\nstderr:\n%s", strings.Join(lines, "\n"), stderr.String())
		}
	}
}

func helperExecBuilder(execPath string) engine.ExecBuilderFunc {
	return func(source string, args []string, env map[string]any) (*exec.Cmd, error) {
		if source != "" {
			args = append([]string{
				"-v", "/work=../test/",
				"-C", source,
			}, args...)
		} else {
			args = append([]string{
				"-v", "/work=../test/",
			}, args...)
		}
		cmd := exec.Command(execPath, args...)
		cmd.Env = os.Environ()
		for key, value := range env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%v", key, value))
		}
		return cmd, nil
	}
}

func assertLinePresent(t *testing.T, lines []string, want string) {
	t.Helper()
	for _, line := range lines {
		if line == want {
			return
		}
	}
	t.Fatalf("missing line %q in output:\n%s", want, strings.Join(lines, "\n"))
}

func assertLineAbsent(t *testing.T, lines []string, want string) {
	t.Helper()
	for _, line := range lines {
		if line == want {
			t.Fatalf("unexpected line %q in output:\n%s", want, strings.Join(lines, "\n"))
		}
	}
}

func boolToEnv(value bool) string {
	if value {
		return "1"
	}
	return "0"
}

func testSignalByName(signalName string) os.Signal {
	switch signalName {
	case "SIGINT":
		return os.Interrupt
	case "SIGTERM":
		return syscall.Signal(15)
	case "SIGQUIT":
		return syscall.Signal(3)
	default:
		return os.Interrupt
	}
}

func TestProcessStderr(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "stderr_write",
			Script: `
				const process = require("process");
				const result = process.stderr.write("error message\n");
				console.println("write success:", result);
			`,
			Output: []string{
				"write success: true",
			},
		},
		{
			Name: "stderr_write_empty",
			Script: `
				const process = require("process");
				const result = process.stderr.write("");
				console.println("write empty:", result);
			`,
			Output: []string{
				"write empty: true",
			},
		},
		{
			Name: "stderr_isTTY",
			Script: `
				const process = require("process");
				const isTTY = process.stderr.isTTY();
				console.println("isTTY type:", typeof isTTY);
			`,
			Output: []string{
				"isTTY type: boolean",
			},
		},
	}

	for _, tc := range tests {
		test_engine.RunTest(t, tc)
	}
}

func TestProcessStdout(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "stdout_write_empty",
			Script: `
				const process = require("process");
				const result = process.stdout.write("");
				console.println("empty write:", result);
			`,
			Output: []string{
				"empty write: true",
			},
		},
		{
			Name: "stdout_isTTY",
			Script: `
				const process = require("process");
				const isTTY = process.stdout.isTTY();
				console.println("isTTY type:", typeof isTTY);
			`,
			Output: []string{
				"isTTY type: boolean",
			},
		},
		{
			Name: "stdout_write_multiple",
			Script: `
				const process = require("process");
				process.stdout.write("first\n");
				process.stdout.write("second\n");
				console.println("done");
			`,
			Output: []string{
				"first",
				"second",
				"done",
			},
		},
	}

	for _, tc := range tests {
		test_engine.RunTest(t, tc)
	}
}

func TestProcessStdinErrors(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "stdin_readBytes_no_args",
			Script: `
				const process = require("process");
				const result = process.stdin.readBytes();
				if (result instanceof Error) {
					console.println("error:", result.message.includes("requires a number"));
				} else {
					console.println("no error, got:", typeof result);
				}
			`,
			Input: []string{"test"},
			Output: []string{
				"error: true",
			},
		},
		{
			Name: "stdin_readBytes_negative",
			Script: `
				const process = require("process");
				const result = process.stdin.readBytes(-1);
				if (result instanceof Error) {
					console.println("error:", result.message.includes("positive number"));
				} else {
					console.println("no error, got:", typeof result);
				}
			`,
			Input: []string{"test"},
			Output: []string{
				"error: true",
			},
		},
		{
			Name: "stdin_readBytes_zero",
			Script: `
				const process = require("process");
				const result = process.stdin.readBytes(0);
				if (result instanceof Error) {
					console.println("error:", result.message.includes("positive number"));
				} else {
					console.println("no error, got:", typeof result);
				}
			`,
			Input: []string{"test"},
			Output: []string{
				"error: true",
			},
		},
		{
			Name: "stdin_readBytes_more_than_available",
			Script: `
				const process = require("process");
				const data = process.stdin.readBytes(100);
				console.println("read length:", data.length);
			`,
			Input: []string{"short"},
			Output: []string{
				"read length: 6",
			},
		},
	}

	for _, tc := range tests {
		test_engine.RunTest(t, tc)
	}
}

func TestProcessHrtime(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "hrtime_basic",
			Script: `
				const process = require("process");
				const time1 = process.hrtime();
				console.println("is array:", Array.isArray(time1));
				console.println("length:", time1.length);
				console.println("has seconds:", typeof time1[0]);
				console.println("has nanos:", typeof time1[1]);
			`,
			Output: []string{
				"is array: true",
				"length: 2",
				"has seconds: number",
				"has nanos: number",
			},
		},
		{
			Name: "hrtime_diff",
			Script: `
				const process = require("process");
				const start = process.hrtime();
				console.println("start type:", Array.isArray(start));
				// Small delay
				let sum = 0;
				for (let i = 0; i < 1000; i++) {
					sum += i;
				}
				const diff = process.hrtime([start[0], start[1]]);
				console.println("diff is array:", Array.isArray(diff));
				console.println("diff length:", diff.length);
				console.println("has elapsed:", diff[0] >= 0 && diff[1] >= 0);
			`,
			Output: []string{
				"start type: true",
				"diff is array: true",
				"diff length: 2",
				"has elapsed: true",
			},
		},
		{
			Name: "hrtime_with_invalid_arg",
			Script: `
				const process = require("process");
				const time = process.hrtime("invalid");
				console.println("is array:", Array.isArray(time));
				console.println("length:", time.length);
			`,
			Output: []string{
				"is array: true",
				"length: 2",
			},
		},
		{
			Name: "hrtime_with_empty_array",
			Script: `
				const process = require("process");
				const time = process.hrtime([]);
				console.println("is array:", Array.isArray(time));
				console.println("length:", time.length);
			`,
			Output: []string{
				"is array: true",
				"length: 2",
			},
		},
	}

	for _, tc := range tests {
		test_engine.RunTest(t, tc)
	}
}

func TestProcessKill(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "kill_no_args",
			Script: `
				const process = require("process");
				const result = process.kill();
				if (result instanceof Error) {
					console.println("error:", result.message.includes("requires a pid"));
				} else {
					console.println("result:", result);
				}
			`,
			Output: []string{
				"error: true",
			},
		},
		{
			Name: "kill_invalid_pid",
			Script: `
				const process = require("process");
				const result = process.kill(-1);
				if (result instanceof Error) {
					console.println("error:", result.message.includes("positive pid"));
				} else {
					console.println("result:", result);
				}
			`,
			Output: []string{
				"error: true",
			},
		},
		{
			Name: "kill_zero_current_process",
			Script: `
				const process = require("process");
				const result = process.kill(process.pid, 0);
				console.println("result:", result);
			`,
			Output: []string{
				"result: true",
			},
		},
		{
			Name: "kill_unsupported_signal",
			Script: `
				const process = require("process");
				const result = process.kill(12345, "SIGWHATEVER");
				if (result instanceof Error) {
					console.println("error:", result.message.includes("unsupported signal"));
				} else {
					console.println("result:", result);
				}
			`,
			Output: []string{
				"error: true",
			},
		},
		{
			Name: "kill_unsupported_numeric_signal",
			Script: `
				const process = require("process");
				const result = process.kill(12345, 999);
				if (result instanceof Error) {
					console.println("error:", result.message.includes("unsupported signal: 999"));
				} else {
					console.println("result:", result);
				}
			`,
			Output: []string{
				"error: true",
			},
		},
		{
			Name: "kill_missing_process_alias_signal",
			Script: `
				const process = require("process");
				const result = process.kill(99999, "term");
				if (result instanceof Error) {
					console.println("error:", result.message.includes("kill 99999 with SIGTERM"));
				} else {
					console.println("result:", result);
				}
			`,
			Output: []string{
				"error: true",
			},
		},
		{
			Name: "kill_missing_process",
			Script: `
				const process = require("process");
				const result = process.kill(99999, "SIGTERM");
				if (result instanceof Error) {
					console.println("error:", result.message.includes("kill 99999 with SIGTERM"));
				} else {
					console.println("result:", result);
				}
			`,
			Output: []string{
				"error: true",
			},
		},
	}

	for _, tc := range tests {
		test_engine.RunTest(t, tc)
	}
}

func TestProcessKillIntegration(t *testing.T) {
	signalName := "SIGTERM"
	if runtime.GOOS == "windows" {
		requireWindowsSignalIntegration(t)
		signalName = "SIGINT"
	}
	lines, cmd, stderr := startProcessSignalHelper(t, signalName, true)

	runProcessKillScript(t, cmd.Process.Pid, fmt.Sprintf(`%q`, signalName))

	waitCh := make(chan error, 1)
	go func() {
		waitCh <- cmd.Wait()
	}()

	select {
	case err := <-waitCh:
		if err != nil {
			_ = cmd.Process.Kill()
			t.Fatalf("helper failed after process.kill: %v\nstderr:\n%s", err, stderr.String())
		}
	case <-time.After(5 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatalf("timeout waiting for helper after process.kill")
	}

	finalLines := collectRemainingLines(lines)
	assertLinePresent(t, finalLines, "ready: "+signalName)
	assertLinePresent(t, finalLines, "caught: "+signalName)
}

func TestProcessKillNumericIntegration(t *testing.T) {
	signalName := "SIGTERM"
	signalExpr := `15`
	if runtime.GOOS == "windows" {
		requireWindowsSignalIntegration(t)
		signalName = "SIGINT"
		signalExpr = `2`
	}
	lines, cmd, stderr := startProcessSignalHelper(t, signalName, true)

	runProcessKillScript(t, cmd.Process.Pid, signalExpr)

	waitCh := make(chan error, 1)
	go func() {
		waitCh <- cmd.Wait()
	}()

	select {
	case err := <-waitCh:
		if err != nil {
			_ = cmd.Process.Kill()
			t.Fatalf("helper failed after numeric process.kill: %v\nstderr:\n%s", err, stderr.String())
		}
	case <-time.After(5 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatalf("timeout waiting for helper after numeric process.kill")
	}

	finalLines := collectRemainingLines(lines)
	assertLinePresent(t, finalLines, "ready: "+signalName)
	assertLinePresent(t, finalLines, "caught: "+signalName)
}

func TestProcessKillAliasIntegration(t *testing.T) {
	signalName := "SIGTERM"
	signalExpr := `"term"`
	if runtime.GOOS == "windows" {
		requireWindowsSignalIntegration(t)
		signalName = "SIGINT"
		signalExpr = `"int"`
	}

	lines, cmd, stderr := startProcessSignalHelper(t, signalName, true)

	runProcessKillScript(t, cmd.Process.Pid, signalExpr)

	waitCh := make(chan error, 1)
	go func() {
		waitCh <- cmd.Wait()
	}()

	select {
	case err := <-waitCh:
		if err != nil {
			_ = cmd.Process.Kill()
			t.Fatalf("helper failed after alias process.kill: %v\nstderr:\n%s", err, stderr.String())
		}
	case <-time.After(5 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatalf("timeout waiting for helper after alias process.kill")
	}

	finalLines := collectRemainingLines(lines)
	assertLinePresent(t, finalLines, "ready: "+signalName)
	assertLinePresent(t, finalLines, "caught: "+signalName)
}

func runProcessKillScript(t *testing.T, pid int, signalExpr string) {
	t.Helper()
	writer := &bytes.Buffer{}
	conf := engine.Config{
		Name: "process_kill_integration",
		Code: fmt.Sprintf(`
			const process = require("process");
			const result = process.kill(%d, %s);
			if (result instanceof Error) {
				throw result;
			}
			console.println("kill:", result);
		`, pid, signalExpr),
		Env: map[string]any{
			"PATH":         "/work:/sbin",
			"PWD":          "/work",
			"HOME":         "/work",
			"LIBRARY_PATH": "./node_modules:/lib",
		},
		FSTabs: []engine.FSTab{
			root.RootFSTab(),
			lib.LibFSTab(),
		},
		Reader: bytes.NewBuffer(nil),
		Writer: writer,
	}

	jr, err := engine.New(conf)
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}
	lib.Enable(jr)
	if err := jr.Run(); err != nil {
		t.Fatalf("run process.kill script: %v", err)
	}
	if !strings.Contains(writer.String(), "kill: true") {
		t.Fatalf("unexpected process.kill output: %s", writer.String())
	}
}

func startProcessSignalHelper(t *testing.T, signalName string, listenForSignal bool) (<-chan string, *exec.Cmd, *bytes.Buffer) {
	t.Helper()

	cmd := exec.Command(os.Args[0], "-test.run=^TestProcessSignalHelper$", "--")
	prepareSignalHelperCommand(cmd)
	cmd.Env = append(os.Environ(),
		"GO_WANT_PROCESS_SIGNAL_HELPER=1",
		"JSH_TEST_SIGNAL="+signalName,
		"JSH_TEST_LISTEN_SIGNAL="+boolToEnv(listenForSignal),
	)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	stderr := &bytes.Buffer{}
	cmd.Stderr = stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("start helper: %v", err)
	}

	linesCh := make(chan string, 16)
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			linesCh <- scanner.Text()
		}
		close(linesCh)
	}()

	var lines []string
	readyLine := "ready: " + signalName
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()

	for {
		select {
		case line, ok := <-linesCh:
			if !ok {
				t.Fatalf("helper exited before readiness for %s\nstdout:\n%s\nstderr:\n%s", signalName, strings.Join(lines, "\n"), stderr.String())
			}
			lines = append(lines, line)
			if line == readyLine {
				buffered := make(chan string, 16)
				for _, existing := range lines {
					buffered <- existing
				}
				go func() {
					for line := range linesCh {
						buffered <- line
					}
					close(buffered)
				}()
				return buffered, cmd, stderr
			}
		case <-timer.C:
			_ = cmd.Process.Kill()
			t.Fatalf("timeout waiting for readiness for %s\nstdout:\n%s\nstderr:\n%s", signalName, strings.Join(lines, "\n"), stderr.String())
		}
	}
}

func collectRemainingLines(lines <-chan string) []string {
	var collected []string
	for line := range lines {
		collected = append(collected, line)
	}
	return collected
}

func TestProcessNextTick(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "nextTick_with_args",
			Script: `
				const process = require("process");
				process.nextTick((a, b, c) => {
					console.println("args:", a, b, c);
				}, "first", "second", "third");
				console.println("main");
			`,
			Output: []string{
				"main",
				"args: first second third",
			},
		},
		{
			Name: "nextTick_no_callback",
			Script: `
				const process = require("process");
				const result = process.nextTick();
				console.println("result:", result === undefined ? "undefined" : result);
			`,
			Output: []string{
				"result: undefined",
			},
		},
		{
			Name: "nextTick_non_function",
			Script: `
				const process = require("process");
				const result = process.nextTick("not a function");
				console.println("result:", result === undefined ? "undefined" : result);
			`,
			Output: []string{
				"result: undefined",
			},
		},
		{
			Name: "nextTick_multiple",
			Script: `
				const process = require("process");
				process.nextTick(() => console.println("tick 1"));
				process.nextTick(() => console.println("tick 2"));
				process.nextTick(() => console.println("tick 3"));
				console.println("main");
			`,
			Output: []string{
				"main",
				"tick 1",
				"tick 2",
				"tick 3",
			},
		},
	}

	for _, tc := range tests {
		test_engine.RunTest(t, tc)
	}
}

func TestProcessChdir(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "chdir_to_home",
			Script: `
				const process = require("process");
				process.chdir("~");
				console.println("cwd after ~:", process.cwd());
			`,
			Output: []string{
				"cwd after ~: /work",
			},
		},
		{
			Name: "chdir_empty_string",
			Script: `
				const process = require("process");
				process.chdir("");
				console.println("cwd after empty:", process.cwd());
			`,
			Output: []string{
				"cwd after empty: /work",
			},
		},
		{
			Name: "chdir_nonexistent",
			Script: `
				const process = require("process");
				try {
					process.chdir("/nonexistent/path");
					console.println("should not reach here");
				} catch (e) {
					console.println("error caught:", e.message.includes("no such file"));
				}
			`,
			Output: []string{
				"error caught: true",
			},
		},
		{
			Name: "chdir_to_file",
			Script: `
				const process = require("process");
				try {
					process.chdir("/sbin/echo.js");
					console.println("should not reach here");
				} catch (e) {
					console.println("error caught:", e.message.includes("not a directory"));
				}
			`,
			Output: []string{
				"error caught: true",
			},
		},
	}

	for _, tc := range tests {
		test_engine.RunTest(t, tc)
	}
}

func TestProcessExecErrors(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "exec_no_args",
			Script: `
				const process = require("process");
				try {
					const result = process.exec();
				} catch (e) {
					console.println("error caught:", e.message);
				}
			`,
			Output: []string{
				"error caught: no command provided",
			},
		},
		{
			Name: "execString_no_args",
			Script: `
				const process = require("process");
				try {
					const result = process.execString();
				} catch (e) {
					console.println("error caught:", e.message);
				}
			`,
			Output: []string{
				"error caught: no source provided",
			},
		},
		{
			Name: "execString_with_args",
			Script: `
				const process = require("process");
				const exitCode = process.execString(
					"console.println('sum:', 10 + 20)",
					"10", "20"
				);
				console.println("exit code:", exitCode);
			`,
			ExecBuilder: testExecBuilder,
			Output: []string{
				"sum: 30",
				"exit code: 0",
			},
		},
	}

	for _, tc := range tests {
		test_engine.RunTest(t, tc)
	}
}

func TestProcessProperties(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "process_ppid",
			Script: `
				const process = require("process");
				console.println("ppid type:", typeof process.ppid);
				console.println("ppid > 0:", process.ppid > 0);
			`,
			Output: []string{
				"ppid type: number",
				"ppid > 0: true",
			},
		},
		{
			Name: "process_execPath",
			Script: `
				const process = require("process");
				console.println("execPath type:", typeof process.execPath);
				console.println("has execPath:", process.execPath.length > 0);
			`,
			Output: []string{
				"execPath type: string",
				"has execPath: true",
			},
		},
		{
			Name: "process_title",
			Script: `
				const process = require("process");
				console.println("title:", process.title);
			`,
			Output: []string{
				"title: process_title",
			},
		},
		{
			Name: "process_versions_details",
			Script: `
				const process = require("process");
				console.println("jsh version:", process.versions.jsh);
				console.println("go version type:", typeof process.versions.go);
			`,
			Output: []string{
				"jsh version: 1.0.0",
				"go version type: string",
			},
		},
	}

	for _, tc := range tests {
		test_engine.RunTest(t, tc)
	}
}

func TestProcessDumpStack(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "dumpStack",
			Script: `
				const process = require("process");
				function testFunc() {
					process.dumpStack(5);
					console.println("stack dumped");
				}
				testFunc();
			`,
			Output: []string{
				"stack dumped",
			},
		},
	}

	for _, tc := range tests {
		test_engine.RunTest(t, tc)
	}
}

func TestProcessHrtimeEdgeCases(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "hrtime_with_string",
			Script: `
				const process = require("process");
				const time = process.hrtime("invalid");
				console.println("is array:", Array.isArray(time));
				console.println("length:", time.length);
			`,
			Output: []string{
				"is array: true",
				"length: 2",
			},
		},
		{
			Name: "hrtime_with_empty_array",
			Script: `
				const process = require("process");
				const time = process.hrtime([]);
				console.println("is array:", Array.isArray(time));
				console.println("length:", time.length);
			`,
			Output: []string{
				"is array: true",
				"length: 2",
			},
		},
		{
			Name: "hrtime_with_single_element_array",
			Script: `
				const process = require("process");
				const time = process.hrtime([123]);
				console.println("is array:", Array.isArray(time));
				console.println("length:", time.length);
			`,
			Output: []string{
				"is array: true",
				"length: 2",
			},
		},
		{
			Name: "hrtime_with_invalid_types_in_array",
			Script: `
				const process = require("process");
				const time = process.hrtime(["string", {}]);
				console.println("is array:", Array.isArray(time));
				console.println("length:", time.length);
			`,
			Output: []string{
				"is array: true",
				"length: 2",
			},
		},
		{
			Name: "hrtime_with_mixed_valid_types",
			Script: `
				const process = require("process");
				const start = process.hrtime();
				// Use integers instead of floats
				const time = process.hrtime([Math.floor(start[0]), Math.floor(start[1])]);
				console.println("is array:", Array.isArray(time));
				console.println("has non-negative values:", time[0] >= 0 && time[1] >= 0);
			`,
			Output: []string{
				"is array: true",
				"has non-negative values: true",
			},
		},
	}

	for _, tc := range tests {
		test_engine.RunTest(t, tc)
	}
}
