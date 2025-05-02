package jsh

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"io"
	"path/filepath"
	"runtime/debug"
	"slices"
	"strings"
	"sync"
	"time"

	js "github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/require"
	"github.com/machbase/neo-server/v8/mods/jsh/analysis"
	"github.com/machbase/neo-server/v8/mods/jsh/builtin"
	"github.com/machbase/neo-server/v8/mods/jsh/console"
	"github.com/machbase/neo-server/v8/mods/jsh/db"
	"github.com/machbase/neo-server/v8/mods/jsh/filter"
	"github.com/machbase/neo-server/v8/mods/jsh/generator"
	"github.com/machbase/neo-server/v8/mods/jsh/http"
	"github.com/machbase/neo-server/v8/mods/jsh/mqtt"
	"github.com/machbase/neo-server/v8/mods/jsh/opcua"
	"github.com/machbase/neo-server/v8/mods/jsh/psutil"
	"github.com/machbase/neo-server/v8/mods/jsh/publisher"
	"github.com/machbase/neo-server/v8/mods/jsh/spatial"
	"github.com/machbase/neo-server/v8/mods/jsh/system"
	"github.com/machbase/neo-server/v8/mods/logging"
	"github.com/machbase/neo-server/v8/mods/util/ssfs"
)

type JshPID uint32

func (o JshPID) String() string {
	if o == PID_ORPHAN {
		return "-"
	}
	return fmt.Sprintf("%d", o)
}

var jshProcesses = make(map[JshPID]*Jsh)
var jshMutex = sync.RWMutex{}
var jshCounter JshPID = 1024

func allocJshPID(j *Jsh) {
	jshMutex.Lock()
	defer jshMutex.Unlock()
	for {
		jshCounter++
		if jshCounter >= 0xEFFF {
			jshCounter = 1024
			continue
		}
		if _, exists := jshProcesses[jshCounter]; !exists {
			j.pid = jshCounter
			jshProcesses[jshCounter] = j
			break
		}
	}
}

func releaseJshPID(j *Jsh) {
	jshMutex.Lock()
	defer jshMutex.Unlock()
	if j.pid != 0 {
		delete(jshProcesses, j.pid)
		j.pid = 0
	}
}

type JshOption func(*Jsh)

func WithReader(r io.Reader) JshOption {
	return func(j *Jsh) {
		j.reader = r
	}
}

func WithWriter(w io.Writer) JshOption {
	return func(j *Jsh) {
		j.writer = w
	}
}

func WithWorkingDir(cwd string) JshOption {
	return func(j *Jsh) {
		j.cwd = filepath.Clean(cwd)
	}
}

// WithEcho sets the echo option for the Jsh instance.
// If true, the Jsh will echo the input to the writer.
// default is true.
func WithEcho(b bool) JshOption {
	return func(j *Jsh) {
		j.echo = b
	}
}

func WithNewLineCRLF(b bool) JshOption {
	return func(j *Jsh) {
		j.newLineCRLF = b
	}
}

// WithUserName sets the user name for the Jsh instance.
func WithUserName(name string) JshOption {
	return func(j *Jsh) {
		j.userName = name
	}
}

func WithConsoleId(id string) JshOption {
	return func(j *Jsh) {
		j.consoleId = id
	}
}

// WithNativeModules sets the native modules to be loaded.
// The modules must be registered using RegisterModule before they can be used.
// If a module is not found, it will panic.
func WithNativeModules(modules ...string) JshOption {
	return func(j *Jsh) {
		for _, m := range modules {
			if _, exists := nativeModules[m]; exists {
				j.modules = append(j.modules, m)
			} else {
				panic(fmt.Sprintf("module %q not found", m))
			}
		}
	}
}

const (
	PID_DAEMON JshPID = 1
	PID_ORPHAN JshPID = 0xFFFFFFFF
)

func WithParent(parent *Jsh) JshOption {
	return func(jsh *Jsh) {
		if parent == nil {
			if _, ok := jsh.Context.(*JshDaemonContext); ok {
				jsh.ppid = PID_DAEMON
			} else {
				jsh.ppid = PID_ORPHAN
			}
		} else {
			jsh.ppid = parent.pid
		}
	}
}

type JshDaemonContext struct {
	context.Context
}

type Jsh struct {
	context.Context
	*Cleaner

	pid         JshPID
	ppid        JshPID
	cwd         string
	reader      io.Reader
	writer      io.Writer
	consoleId   string // if the process bind to a web-console (websocket)
	echo        bool
	newLineCRLF bool
	vm          *js.Runtime
	chStart     chan struct{}
	chStop      chan struct{}
	sourceName  string
	sourceCode  string
	userName    string
	args        []string
	modules     []string
	program     *js.Program
	startAt     time.Time
	resultVal   js.Value
	resultErr   []error

	onStatusChanged func(j *Jsh, status JshStatus)
}

type JshStatus string

const (
	JshStatusRunning JshStatus = "Running"
	JshStatusStopped JshStatus = "Stopped"
)

func NewJsh(ctx context.Context, opts ...JshOption) *Jsh {
	ret := &Jsh{
		Context:   ctx,
		Cleaner:   &Cleaner{},
		cwd:       "/",
		reader:    bytes.NewBuffer(nil),
		writer:    io.Discard,
		echo:      true,
		chStart:   make(chan struct{}),
		chStop:    make(chan struct{}),
		startAt:   time.Now(),
		resultVal: js.Undefined(),
		resultErr: nil,
	}
	for _, opt := range opts {
		opt(ret)
	}
	return ret
}

func (j *Jsh) NewChild() *Jsh {
	child := NewJsh(
		j.Context,
		WithNativeModules(j.modules...),
		WithParent(j),
		WithReader(j.reader),
		WithWriter(j.writer),
		WithEcho(j.echo),
		WithNewLineCRLF(j.newLineCRLF),
		WithWorkingDir(j.cwd),
		WithUserName(j.userName),
		WithConsoleId(j.consoleId),
	)
	return child
}

func (j *Jsh) print0(level logging.Level, eolLine bool, args ...any) error {
	if l, ok := j.writer.(logging.Log); ok {
		toks := make([]string, len(args))
		for i, arg := range args {
			if s, ok := arg.(string); ok {
				toks[i] = s
			} else {
				toks[i] = fmt.Sprintf("%v", arg)
			}
		}
		l.Log(level, strings.Join(toks, " "))
	} else {
		for i, arg := range args {
			if i > 0 {
				fmt.Fprint(j.writer, " ")
			}
			var line string
			if s, ok := arg.(string); ok {
				line = s
			} else {
				line = fmt.Sprintf("%v", arg)
			}
			if j.newLineCRLF {
				fmt.Fprint(j.writer, strings.ReplaceAll(line, "\n", "\r\n"))
			} else {
				fmt.Fprint(j.writer, line)
			}
		}
		if eolLine {
			if j.newLineCRLF {
				fmt.Fprint(j.writer, "\r\n")
			} else {
				fmt.Fprint(j.writer, "\n")
			}
		}
	}
	return nil
}

func (j *Jsh) Print(args ...any) error {
	return j.print0(logging.LevelInfo, false, args...)
}

func (j *Jsh) Log(level logging.Level, args ...any) error {
	return j.print0(level, true, args...)
}

func (j *Jsh) Exec(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("command not found")
	}

	sourceName, sourceCode := j.searchPath(args[0])
	if sourceCode == "" {
		return fmt.Errorf("command not found: %s", args[0])
	}
	args[0] = sourceName
	return j.Run(sourceName, sourceCode, args)
}

func (j *Jsh) ExecBackground(args []string) error {
	go func() {
		j.Exec(args)
	}()
	return nil
}

func init() {
	nativeModules = map[string]func(context.Context) require.ModuleLoader{
		"@jsh/process":   func(ctx context.Context) require.ModuleLoader { return ctx.(*Jsh).moduleProcess },
		"@jsh/system":    system.NewModuleLoader,
		"@jsh/db":        db.NewModuleLoader,
		"@jsh/publisher": publisher.NewModuleLoader,
		"@jsh/filter":    filter.NewModuleLoader,
		"@jsh/analysis":  analysis.NewModuleLoader,
		"@jsh/spatial":   spatial.NewModuleLoader,
		"@jsh/generator": generator.NewModuleLoader,
		"@jsh/mqtt":      mqtt.NewModuleLoader,
		"@jsh/http":      http.NewModuleLoader,
		"@jsh/psutil":    psutil.NewModuleLoader,
		"@jsh/opcua":     opcua.NewModuleLoader,
	}
}

func RegisterNativeModules(ctx context.Context, r *require.Registry, modules ...string) {
	for _, name := range modules {
		if loader, exists := nativeModules[name]; !exists {
			panic(fmt.Sprintf("module %q not found", name))
		} else {
			r.RegisterNativeModule(name, loader(ctx))
		}
	}
}

var nativeModules map[string]func(context.Context) require.ModuleLoader

// NativeModuleNamesExcludes returns the names of all registered native modules.
// It excludes the modules specified in the excludes slice.
func NativeModuleNamesExcludes(excludes ...string) []string {
	ret := make([]string, 0, len(nativeModules))
	for name := range nativeModules {
		if !slices.Contains(excludes, name) {
			ret = append(ret, name)
		}
	}
	return ret
}

// NativeModuleNames returns the names of all registered native modules.
func NativeModuleNames() []string {
	return NativeModuleNamesExcludes()
}

func ErrorToString(err error) string {
	if err == nil {
		return ""
	}
	if jsErr, ok := err.(*js.Exception); ok {
		return jsErr.String()
	} else if jsErr, ok := err.(*js.InterruptedError); ok {
		return jsErr.String()
	}
	return err.Error()
}

func (j *Jsh) Run(sourceName, sourceCode string, args []string) error {
	j.sourceName = sourceName
	j.sourceCode = sourceCode
	j.args = args

	go func() {
		allocJshPID(j)
		defer func() {
			if r := recover(); r != nil {
				if j.writer != nil {
					j.writer.Write(debug.Stack())
				} else {
					debug.PrintStack()
				}
			}
			if j.onStatusChanged != nil {
				j.onStatusChanged(j, JshStatusStopped)
			}
			releaseJshPID(j)
			close(j.chStop)
		}()
		close(j.chStart)

		if j.onStatusChanged != nil {
			j.onStatusChanged(j, JshStatusRunning)
		}
		if j.program == nil {
			if program, err := js.Compile(sourceName, sourceCode, false); err != nil {
				j.resultErr = append(j.resultErr, err)
				return
			} else {
				j.program = program
			}
		}

		j.vm = js.New()
		j.vm.SetFieldNameMapper(js.TagFieldNameMapper("json", false))

		registry := require.NewRegistry(require.WithLoader(j.loadSource))
		for _, name := range j.modules {
			if moduleLoaderFactory, ok := nativeModules[name]; ok {
				registry.RegisterNativeModule(name, moduleLoaderFactory(j))
			}
		}
		registry.Enable(j.vm)
		console.Enable(j.vm, j.Log)

		if result, err := j.vm.RunProgram(j.program); err != nil {
			j.resultErr = append(j.resultErr, err)
		} else {
			j.resultVal = result
		}

		j.RunCleanup(j.writer)
	}()
	<-j.chStart

	select {
	case <-j.chStop:
	case <-j.Context.Done():
		j.vm.Interrupt("interrupted")
		<-j.chStop
	}
	for _, err := range j.resultErr {
		if jsErr, ok := err.(*js.Exception); ok {
			j.Print(jsErr.String(), "\n")
		} else {
			j.Print(err.Error(), "\n")
		}
	}
	if len(j.resultErr) == 0 {
		return nil
	}
	return j.resultErr[0]
}

func (j *Jsh) Errors() []error {
	return j.resultErr
}

func (j *Jsh) Interrupt() {
	if j.vm == nil {
		return
	}
	j.vm.Interrupt("interrupted")
}

func (j *Jsh) loadSource(path string) ([]byte, error) {
	// TODO check if path is relative to cwd
	ss := ssfs.Default()
	if ss == nil {
		return nil, fmt.Errorf("loadSource: ssfs not initialized, %s", path)
	}
	ent, err := ss.Get("/" + strings.TrimPrefix(path, "/"))
	if err != nil || ent.IsDir {
		return nil, require.ModuleFileDoesNotExistError
	}
	return ent.Content, nil
}

var jsPath = []string{".", "/sbin"}

func (j *Jsh) searchPath(cmdPath string) (sourceName string, sourceCode string) {
	root_fs := ssfs.Default()
	if !strings.HasSuffix(cmdPath, ".js") {
		cmdPath += ".js"
	}
	if cmdPath == "@.js" {
		cmdPath = "jsh.js"
	}

	if code, ok := builtin.Code(cmdPath); ok {
		sourceName = strings.TrimSuffix(cmdPath, ".js")
		sourceCode = code
	} else if strings.HasPrefix(cmdPath, "/") {
		if ent, err := root_fs.Get(cmdPath); err == nil && !ent.IsDir {
			sourceName = cmdPath
			sourceCode = string(ent.Content)
		}
	} else if strings.HasPrefix(cmdPath, "./") || strings.HasPrefix(cmdPath, "../") {
		lookup := filepath.Join(j.cwd, cmdPath)
		if ent, err := root_fs.Get(lookup); err == nil && !ent.IsDir {
			sourceName = lookup
			sourceCode = string(ent.Content)
		}
	} else {
		for _, path := range jsPath {
			if path == "." {
				path = j.cwd
			}
			lookup := filepath.Join(path, cmdPath)
			if ent, err := root_fs.Get(lookup); err != nil || ent.IsDir {
				continue
			} else {
				sourceName = lookup
				sourceCode = string(ent.Content)
				break
			}
		}
	}
	return
}
