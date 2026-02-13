package engine

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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
	exports.Set("expand", jr.Env.Expand)
	exports.Set("exec", doExec(vm, jr.Exec))
	exports.Set("execString", doExecString(vm, jr.Exec))
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
	Code int
}

// doExecString executes a command line string via the exec function.
//
// syntax) execString(source: string, ...args: string): number
// return) exit code
func doExecString(vm *goja.Runtime, exec func(vm *goja.Runtime, source string, args []string) goja.Value) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return vm.NewGoError(fmt.Errorf("no source code provided"))
		}
		args := make([]string, 0, len(call.Arguments))
		for _, a := range call.Arguments {
			args = append(args, a.String())
		}
		return exec(vm, args[0], args[1:])
	}
}

// doExec executes a command by building an exec.Cmd and running it.
//
// syntax) exec(command: string, ...args: string): number
// return) exit code
func doExec(vm *goja.Runtime, exec func(vm *goja.Runtime, source string, args []string) goja.Value) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return vm.NewGoError(fmt.Errorf("no command provided"))
		}
		args := make([]string, 0, len(call.Arguments))
		for _, a := range call.Arguments {
			args = append(args, a.String())
		}
		return exec(vm, "", args)
	}
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

func doKill(vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return vm.NewGoError(fmt.Errorf("kill requires a pid argument"))
		}

		pid := int(call.Argument(0).ToInteger())
		signal := "SIGTERM"
		if len(call.Arguments) > 1 {
			signal = call.Argument(1).String()
		}

		// TODO: Implement actual signal sending
		// For now, just a placeholder
		_ = pid
		_ = signal

		return vm.ToValue(true)
	}
}
