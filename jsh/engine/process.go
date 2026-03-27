package engine

import (
	"bufio"
	_ "embed"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/eventloop"
)

func (jr *JSRuntime) Process(vm *goja.Runtime, module *goja.Object) {
	executable, _ := os.Executable()
	exports := module.Get("exports").(*goja.Object)

	// Basic properties
	exports.Set("env", jr.Env)
	exports.Set("argv", append([]string{executable, jr.Name}, jr.Args...))
	exports.Set("execPath", executable)
	exports.Set("pid", os.Getpid())
	exports.Set("ppid", os.Getppid())
	exports.Set("platform", runtime.GOOS)
	exports.Set("arch", runtime.GOARCH)
	exports.Set("version", "jsh-1.0.0") // JSH version

	// Version information
	versions := vm.NewObject()
	versions.Set("jsh", "1.0.0")
	versions.Set("go", runtime.Version())
	exports.Set("versions", versions)

	// Process title (can be set)
	exports.Set("title", jr.Name)

	// Streams
	exports.Set("stdin", jr.createStdin(vm))
	exports.Set("stdout", jr.createStdout(vm))
	exports.Set("stderr", jr.createStderr(vm))

	// Functions
	exports.Set("addShutdownHook", jr.AddShutdownHook)
	exports.Set("exit", doExit(vm))
	exports.Set("which", jr.Env.Which)
	exports.Set("alias", jr.Env.Alias)
	exports.Set("expand", jr.Env.Expand)
	exports.Set("exec", jr.Exec)
	exports.Set("execString", jr.ExecString)
	exports.Set("dispatchEvent", dispatchEvent(jr.EventLoop()))
	exports.Set("now", jr.Now)
	exports.Set("chdir", jr.Chdir)
	exports.Set("cwd", jr.Cwd)
	exports.Set("nextTick", doNextTick(jr.EventLoop()))

	// Resource monitoring (placeholder implementations)
	exports.Set("memoryUsage", doMemoryUsage(vm))
	exports.Set("cpuUsage", doCpuUsage(vm))
	exports.Set("uptime", doUptime(vm))
	exports.Set("hrtime", doHrtime(vm))

	// Signal handling support
	exports.Set("kill", doKill(vm))
	exports.Set("watchSignal", watchSignal(vm, jr.EventLoop(), jr.AddShutdownHook))

	// debug
	exports.Set("dumpStack", func(depth int) {
		var buf = make([]goja.StackFrame, depth)
		frames := vm.CaptureCallStack(depth, buf)
		for n, frame := range frames {
			fmt.Printf("[%d] %s: %s %s\n", n, frame.SrcName(), frame.FuncName(), frame.Position())
		}
	})
}

func (jr *JSRuntime) createStdin(vm *goja.Runtime) *goja.Object {
	stdin := vm.NewObject()
	reader := jr.Env.Reader()

	// read() - read all available data
	stdin.Set("read", func(call goja.FunctionCall) goja.Value {
		data, err := io.ReadAll(reader)
		if err != nil {
			return vm.NewGoError(fmt.Errorf("stdin read error: %w", err))
		}
		return vm.ToValue(string(data))
	})

	// readLine() - read a single line
	stdin.Set("readLine", func(call goja.FunctionCall) goja.Value {
		scanner := bufio.NewScanner(reader)
		if scanner.Scan() {
			return vm.ToValue(scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			return vm.NewGoError(fmt.Errorf("stdin readLine error: %w", err))
		}
		return goja.Null()
	})

	// readLines() - read all lines as an array
	stdin.Set("readLines", func(call goja.FunctionCall) goja.Value {
		lines := []string{}
		scanner := bufio.NewScanner(reader)
		for scanner.Scan() {
			lines = append(lines, scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			return vm.NewGoError(fmt.Errorf("stdin readLines error: %w", err))
		}
		return vm.ToValue(lines)
	})

	// readBytes(n) - read n bytes
	stdin.Set("readBytes", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return vm.NewGoError(fmt.Errorf("readBytes requires a number argument"))
		}
		n := int(call.Argument(0).ToInteger())
		if n <= 0 {
			return vm.NewGoError(fmt.Errorf("readBytes requires a positive number"))
		}
		buf := make([]byte, n)
		bytesRead, err := reader.Read(buf)
		if err != nil && err != io.EOF {
			return vm.NewGoError(fmt.Errorf("stdin readBytes error: %w", err))
		}
		return vm.ToValue(string(buf[:bytesRead]))
	})

	// readBuffer(n) - read n bytes as ArrayBuffer for binary-safe consumers
	stdin.Set("readBuffer", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return vm.NewGoError(fmt.Errorf("readBuffer requires a number argument"))
		}
		n := int(call.Argument(0).ToInteger())
		if n <= 0 {
			return vm.NewGoError(fmt.Errorf("readBuffer requires a positive number"))
		}
		buf := make([]byte, n)
		bytesRead, err := reader.Read(buf)
		if err != nil && err != io.EOF {
			return vm.NewGoError(fmt.Errorf("stdin readBuffer error: %w", err))
		}
		return vm.ToValue(vm.NewArrayBuffer(buf[:bytesRead]))
	})

	// isTTY - check if stdin is a terminal
	stdin.Set("isTTY", func(call goja.FunctionCall) goja.Value {
		file, ok := reader.(*os.File)
		if !ok {
			return vm.ToValue(false)
		}
		stat, err := file.Stat()
		if err != nil {
			return vm.ToValue(false)
		}
		return vm.ToValue((stat.Mode() & os.ModeCharDevice) != 0)
	})

	return stdin
}

func (jr *JSRuntime) createStdout(vm *goja.Runtime) *goja.Object {
	stdout := vm.NewObject()
	writer := jr.Env.Writer()

	// write(data) - write data to stdout
	stdout.Set("write", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return vm.ToValue(true)
		}
		data := call.Argument(0).String()
		_, err := writer.Write([]byte(data))
		if err != nil {
			return vm.ToValue(false)
		}
		return vm.ToValue(true)
	})

	// isTTY - check if stdout is a terminal
	stdout.Set("isTTY", func(call goja.FunctionCall) goja.Value {
		file, ok := writer.(*os.File)
		if !ok {
			return vm.ToValue(false)
		}
		stat, err := file.Stat()
		if err != nil {
			return vm.ToValue(false)
		}
		return vm.ToValue((stat.Mode() & os.ModeCharDevice) != 0)
	})

	return stdout
}

func (jr *JSRuntime) createStderr(vm *goja.Runtime) *goja.Object {
	stderr := vm.NewObject()

	// write(data) - write data to stderr
	stderr.Set("write", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return vm.ToValue(true)
		}
		data := call.Argument(0).String()
		_, err := os.Stderr.Write([]byte(data))
		if err != nil {
			return vm.ToValue(false)
		}
		return vm.ToValue(true)
	})

	// isTTY - check if stderr is a terminal
	stderr.Set("isTTY", func(call goja.FunctionCall) goja.Value {
		stat, err := os.Stderr.Stat()
		if err != nil {
			return vm.ToValue(false)
		}
		return vm.ToValue((stat.Mode() & os.ModeCharDevice) != 0)
	})

	return stderr
}

func (jr *JSRuntime) Now() time.Time {
	if jr.nowFunc == nil {
		return time.Now()
	} else {
		return jr.nowFunc()
	}
}

func (jr *JSRuntime) Cwd() string {
	return jr.Env.Get("PWD").(string)
}

func (jr *JSRuntime) Chdir(path string) error {
	if path == "" {
		path = "$HOME"
	}
	// Get target directory
	path = jr.Env.ResolvePath(path)

	// Handle relative paths
	if !strings.HasPrefix(path, "/") {
		pwd := jr.Cwd()
		path = pwd + "/" + path
	}

	// Check if directory exists
	fs := jr.Env.Filesystem()
	fd, err := fs.Open(path)
	if err != nil {
		return fmt.Errorf("chdir: no such file or directory: %s", path)
	}
	defer fd.Close()
	info, err := fd.Stat()
	if err != nil {
		return fmt.Errorf("chdir: cannot stat directory: %s", path)
	}
	if !info.IsDir() {
		return fmt.Errorf("chdir: not a directory: %s", path)
	}
	path = filepath.ToSlash(filepath.Clean(path))
	jr.Env.Set("PWD", path)
	return nil
}

type Exit struct {
	Code int `json:"code"`
}

func doExit(vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		exit := Exit{Code: 0}
		if len(call.Arguments) > 0 {
			exit.Code = int(call.Argument(0).ToInteger())
		}
		vm.Interrupt(exit)
		return goja.Undefined()
	}
}

func doNextTick(eventLoop *eventloop.EventLoop) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return goja.Undefined()
		}

		callback, ok := goja.AssertFunction(call.Argument(0))
		if !ok {
			return goja.Undefined()
		}

		args := make([]goja.Value, 0, len(call.Arguments)-1)
		for i := 1; i < len(call.Arguments); i++ {
			args = append(args, call.Arguments[i])
		}

		eventLoop.RunOnLoop(func(vm *goja.Runtime) {
			callback(goja.Undefined(), args...)
		})

		return goja.Undefined()
	}
}

func doMemoryUsage(vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		// TODO: Implement actual memory usage tracking
		result := vm.NewObject()
		result.Set("rss", 0)       // Resident Set Size
		result.Set("heapTotal", 0) // Total heap size
		result.Set("heapUsed", 0)  // Used heap size
		result.Set("external", 0)  // External memory
		result.Set("arrayBuffers", 0)
		return result
	}
}

func doCpuUsage(vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		// TODO: Implement actual CPU usage tracking
		result := vm.NewObject()
		result.Set("user", 0)   // User CPU time in microseconds
		result.Set("system", 0) // System CPU time in microseconds
		return result
	}
}

func doUptime(vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	// TODO: Track actual process start time
	startTime := time.Now()
	return func(call goja.FunctionCall) goja.Value {
		uptime := time.Since(startTime).Seconds()
		return vm.ToValue(uptime)
	}
}

func doHrtime(vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		// TODO: Implement high-resolution time
		// Returns [seconds, nanoseconds] tuple
		now := time.Now()

		if len(call.Arguments) > 0 {
			// Calculate difference from previous hrtime call
			prevArray := call.Argument(0).Export()
			if arr, ok := prevArray.([]interface{}); ok && len(arr) == 2 {
				var prevSec, prevNano int64

				// Handle both int64 and float64 types
				switch v := arr[0].(type) {
				case int64:
					prevSec = v
				case float64:
					prevSec = int64(v)
				case int:
					prevSec = int64(v)
				default:
					// Invalid type, return current time
					result := vm.NewArray()
					result.Set("0", now.Unix())
					result.Set("1", now.Nanosecond())
					return result
				}

				switch v := arr[1].(type) {
				case int64:
					prevNano = v
				case float64:
					prevNano = int64(v)
				case int:
					prevNano = int64(v)
				default:
					// Invalid type, return current time
					result := vm.NewArray()
					result.Set("0", now.Unix())
					result.Set("1", now.Nanosecond())
					return result
				}

				prevTime := time.Unix(prevSec, prevNano)
				diff := now.Sub(prevTime)

				result := vm.NewArray()
				result.Set("0", diff.Nanoseconds()/1e9)
				result.Set("1", diff.Nanoseconds()%1e9)
				return result
			}
		}

		// Return current time
		result := vm.NewArray()
		result.Set("0", now.Unix())
		result.Set("1", now.Nanosecond())
		return result
	}
}

func watchSignal(vm *goja.Runtime, loop *eventloop.EventLoop, addShutdownHook func(func())) func(call goja.FunctionCall) goja.Value {
	dispatch := dispatchEvent(loop)
	var mu sync.Mutex
	type signalRegistration struct {
		ch   chan os.Signal
		done chan struct{}
	}
	registrations := map[string]signalRegistration{}

	return func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 2 {
			return vm.NewGoError(fmt.Errorf("watchSignal requires a signal name and target object"))
		}

		signalName := call.Argument(0).String()
		osSignal, err := signalByName(signalName)
		if err != nil {
			return vm.NewGoError(fmt.Errorf("unsupported signal: %s", signalName))
		}

		target := call.Argument(1)
		if goja.IsUndefined(target) || goja.IsNull(target) {
			return vm.NewGoError(fmt.Errorf("watchSignal requires a target object"))
		}

		targetObject := target.ToObject(vm)

		mu.Lock()
		defer mu.Unlock()
		if _, ok := registrations[signalName]; ok {
			return vm.ToValue(true)
		}

		registration := signalRegistration{
			ch:   make(chan os.Signal, 1),
			done: make(chan struct{}),
		}
		signal.Notify(registration.ch, osSignal)

		go func(target *goja.Object, ch <-chan os.Signal, done <-chan struct{}) {
			for {
				select {
				case <-done:
					return
				case <-ch:
					dispatch(target, signalName)
				}
			}
		}(targetObject, registration.ch, registration.done)

		registrations[signalName] = registration
		addShutdownHook(func() {
			mu.Lock()
			registration, ok := registrations[signalName]
			if !ok {
				mu.Unlock()
				return
			}
			delete(registrations, signalName)
			mu.Unlock()

			signal.Stop(registration.ch)
			close(registration.done)
		})

		return vm.ToValue(true)
	}
}

func signalByName(signalName string) (os.Signal, error) {
	canonicalName, signalNumber, err := normalizeSignalName(signalName)
	if err != nil {
		return nil, err
	}
	_ = canonicalName
	return signalByNumber(signalNumber)
}

func signalByNumber(signalNumber int) (os.Signal, error) {
	switch signalNumber {
	case 0:
		return syscall.Signal(0), nil
	case 1:
		return syscall.Signal(1), nil
	case 2:
		return os.Interrupt, nil
	case 3:
		return syscall.Signal(3), nil
	case 6:
		return syscall.Signal(6), nil
	case 9:
		return syscall.Signal(9), nil
	case 10:
		return syscall.Signal(10), nil
	case 11:
		return syscall.Signal(11), nil
	case 12:
		return syscall.Signal(12), nil
	case 13:
		return syscall.Signal(13), nil
	case 14:
		return syscall.Signal(14), nil
	case 15:
		return syscall.Signal(15), nil
	default:
		return nil, fmt.Errorf("unsupported signal: %d", signalNumber)
	}
}

func normalizeSignalName(signalName string) (string, int, error) {
	normalized := strings.ToUpper(strings.TrimSpace(signalName))
	if normalized == "" {
		return "", 0, fmt.Errorf("unsupported signal: %s", signalName)
	}
	if signalNumber, err := strconv.Atoi(normalized); err == nil {
		return strconv.Itoa(signalNumber), signalNumber, nil
	}
	normalized = strings.TrimPrefix(normalized, "SIG")

	switch normalized {
	case "HUP":
		return "SIGHUP", 1, nil
	case "INT":
		return "SIGINT", 2, nil
	case "QUIT":
		return "SIGQUIT", 3, nil
	case "ABRT":
		return "SIGABRT", 6, nil
	case "KILL":
		return "SIGKILL", 9, nil
	case "USR1":
		return "SIGUSR1", 10, nil
	case "SEGV":
		return "SIGSEGV", 11, nil
	case "USR2":
		return "SIGUSR2", 12, nil
	case "PIPE":
		return "SIGPIPE", 13, nil
	case "ALRM":
		return "SIGALRM", 14, nil
	case "TERM":
		return "SIGTERM", 15, nil
	default:
		return "", 0, fmt.Errorf("unsupported signal: %s", signalName)
	}
}

func resolveKillSignal(value goja.Value) (string, int, os.Signal, error) {
	if goja.IsUndefined(value) {
		sig, err := signalByName("SIGTERM")
		return "SIGTERM", 15, sig, err
	}

	switch value.Export().(type) {
	case int64, int32, int16, int8, int, float64, float32:
		signalNumber := int(value.ToInteger())
		sig, err := signalByNumber(signalNumber)
		return strconv.Itoa(signalNumber), signalNumber, sig, err
	default:
		signalName := value.String()
		canonicalName, signalNumber, err := normalizeSignalName(signalName)
		if err != nil {
			return signalName, 0, nil, err
		}
		osSignal, err := signalByNumber(signalNumber)
		return canonicalName, signalNumber, osSignal, err
	}
}

func doKill(vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return vm.NewGoError(fmt.Errorf("kill requires a pid argument"))
		}

		pid := int(call.Argument(0).ToInteger())
		if pid <= 0 {
			return vm.NewGoError(fmt.Errorf("kill requires a positive pid"))
		}

		signalArg := goja.Undefined()
		if len(call.Arguments) > 1 {
			signalArg = call.Argument(1)
		}

		signalLabel, signalNumber, osSignal, err := resolveKillSignal(signalArg)
		if err != nil {
			return vm.NewGoError(err)
		}

		if err := killProcess(pid, signalLabel, signalNumber, osSignal); err != nil {
			return vm.NewGoError(fmt.Errorf("kill %d with %s: %w", pid, signalLabel, err))
		}

		return vm.ToValue(true)
	}
}
