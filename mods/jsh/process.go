package jsh

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	js "github.com/dop251/goja"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/machbase/neo-server/v8/mods/eventbus"
	"github.com/machbase/neo-server/v8/mods/logging"
	"github.com/machbase/neo-server/v8/mods/util"
	"github.com/machbase/neo-server/v8/mods/util/ssfs"
)

func (j *Jsh) moduleProcess(r *js.Runtime, module *js.Object) {
	// m = require("@jsh/process")
	o := module.Get("exports").(*js.Object)
	// m.pid()
	o.Set("pid", j.process_pid)
	// m.ppid()
	o.Set("ppid", j.process_ppid)
	// m.isDaemon()
	o.Set("isDaemon", j.process_isDaemon)
	// m.isOrphan()
	o.Set("isOrphan", j.process_isOrphan)
	// m.args()
	o.Set("args", j.process_args)
	// m.cwd()
	o.Set("cwd", j.process_cwd)
	// m.cd("/path/to/dir")
	o.Set("cd", j.process_cd)
	// m.readDir("/path/to/dir", (entry) => {})
	o.Set("readDir", j.process_readDir)
	// m.exists("/path/to/file"): {path: String, isDir: Boolean, readOnly: Boolean}
	o.Set("exists", j.process_exists)
	// m.readLine()
	o.Set("readLine", j.process_readLine)
	// m.print("hello", "world")
	o.Set("print", j.process_print)
	o.Set("println", j.process_println)
	// m.table
	o.Set("Table", j.process_Table)
	// m.exec(args)
	o.Set("exec", j.process_exec)
	// m.sleep(ms)
	o.Set("sleep", j.process_sleep)
	// m.kill(pid)
	o.Set("kill", j.process_kill)
	// m.ps()
	o.Set("ps", j.process_ps)
	// m.openEditor("/path/to/file")
	o.Set("openEditor", j.process_openEditor)
	// m.process_parseCommandLine("cmd arg1 arg2 | cmd2 > redirect")
	o.Set("parseCommandLine", j.process_parseCommandLine)
	// tok = m.addCleanup(()=>{})
	o.Set("addCleanup", j.process_addCleanup)
	// m.removeCleanup(tok)
	o.Set("removeCleanup", j.process_removeCleanup)

	// m.daemonize()
	o.Set("daemonize", j.process_daemonize)
	// m.schedule("@every 2s", (currentTime) => { println("hello") })
	o.Set("schedule", j.schedule)

	// m.serviceStatus()
	o.Set("serviceStatus", j.process_serviceStatus)
	// m.serviceStart(svc)
	o.Set("serviceStart", j.process_serviceStart)
	// m.serviceStop(svc)
	o.Set("serviceStop", j.process_serviceStop)
	// m.serviceReread()
	o.Set("serviceReread", j.process_serviceReread)
	// m.serviceUpdate()
	o.Set("serviceUpdate", j.process_serviceUpdate)
}

// jsh.pid()
func (j *Jsh) process_pid() js.Value {
	return j.vm.ToValue(j.pid)
}

// jsh.ppid()
func (j *Jsh) process_ppid() js.Value {
	return j.vm.ToValue(j.ppid)
}

// jsh.isDaemon()
func (j *Jsh) process_isDaemon() js.Value {
	return j.vm.ToValue(j.ppid == 1)
}

func (j *Jsh) process_isOrphan() js.Value {
	return j.vm.ToValue(j.ppid == 0 || j.ppid == 0xFFFFFFFF)
}

// jsh.args()
func (j *Jsh) process_args() js.Value {
	return j.vm.ToValue(j.args)
}

// jsh.cwd()
func (j *Jsh) process_cwd() js.Value {
	return j.vm.ToValue(j.cwd)
}

// jsh.openEditor(call js.FunctionCall) js.Value {
func (j *Jsh) process_openEditor(call js.FunctionCall) js.Value {
	fmt.Println("openEditor", j.consoleId, j.userName)
	if j.consoleId == "" || j.userName == "" {
		panic(j.vm.ToValue("openEditor: no console bind"))
	}
	if len(call.Arguments) == 0 {
		panic(j.vm.ToValue("openEditor: missing argument"))
	}
	var path string
	j.vm.ExportTo(call.Arguments[0], &path)
	fmt.Println("openEditor", path)
	eventbus.PublishOpenFile(fmt.Sprintf("console:%s:%s", j.userName, j.consoleId), path)
	return js.Undefined()
}

// jsh.cd("/path/to/dir")
func (j *Jsh) process_cd(call js.FunctionCall) js.Value {
	if len(call.Arguments) == 0 {
		panic(j.vm.ToValue(fmt.Sprintf("cd: invalid argument %s", call.Arguments[0].ExportType())))
	}
	path, ok := call.Arguments[0].Export().(string)
	if !ok {
		panic(j.vm.ToValue(fmt.Sprintf("cd: invalid argument %s", call.Arguments[0].ExportType())))
	}

	if !filepath.IsAbs(path) {
		path = filepath.Clean(filepath.Join(j.cwd, path))
	}
	ent, err := ssfs.Default().Get(path)
	if err != nil {
		panic(j.vm.NewGoError(err))
	}
	if !ent.IsDir {
		panic(j.vm.ToValue(fmt.Sprintf("%q is not a directory", path)))
	}
	j.cwd = filepath.ToSlash(path)
	return js.Undefined()
}

// jsh.readDir("/path/to/dir", (dir) => {})
func (j *Jsh) process_readDir(call js.FunctionCall) js.Value {
	if len(call.Arguments) != 2 {
		panic(j.vm.ToValue(fmt.Sprintf("readdir: missing argument")))
	}
	path, ok := call.Arguments[0].Export().(string)
	if !ok {
		panic(j.vm.ToValue(fmt.Sprintf("readdir: invalid argument %s", call.Arguments[0].ExportType())))
	}
	fn, ok := call.Arguments[1].Export().(func(js.FunctionCall) js.Value)
	if !ok {
		panic(j.vm.ToValue(fmt.Sprintf("readdir: invalid argument %s", call.Arguments[1].ExportType())))
	}

	callback := func(m *js.Object) bool {
		r := fn(js.FunctionCall{
			This:      js.Undefined(),
			Arguments: []js.Value{m},
		})
		if r.Export().(bool) {
			return true
		}
		return false
	}

	if filepath.IsAbs(path) {
		path = filepath.Clean(path)
	} else {
		path = filepath.Clean(filepath.Join(j.cwd, path))
	}
	root_fs := ssfs.Default()
	ent, err := root_fs.Get(path)
	if err != nil {
		panic(j.vm.ToValue(fmt.Sprintf("readdir: %s", err.Error())))
	}
	if !ent.IsDir {
		panic(j.vm.ToValue(fmt.Sprintf("%s is not a directory", path)))
	}

	for _, d := range ent.Children {
		m := j.vm.NewObject()
		m.Set("name", d.Name)
		m.Set("isDir", d.IsDir)
		m.Set("readOnly", d.ReadOnly)
		m.Set("type", d.Type)
		m.Set("size", d.Size)
		m.Set("virtual", d.Virtual)
		if !callback(m) {
			break
		}
	}
	return js.Undefined()
}

func (j *Jsh) process_exists(call js.FunctionCall) js.Value {
	if len(call.Arguments) == 0 {
		panic(j.vm.ToValue("exists: missing argument"))
	}

	path, ok := call.Arguments[0].Export().(string)
	if !ok {
		panic(j.vm.ToValue(fmt.Sprintf("exists: invalid argument %s", call.Arguments[0].ExportType())))
	}

	if filepath.IsAbs(path) {
		path = filepath.Clean(path)
	} else {
		path = filepath.Clean(filepath.Join(j.cwd, path))
	}
	root_fs := ssfs.Default()
	ent, err := root_fs.Get(path)
	if err != nil {
		panic(j.vm.ToValue(fmt.Sprintf("exists: %s", err.Error())))
	}

	m := j.vm.NewObject()
	m.Set("path", path)
	m.Set("isDir", ent.IsDir)
	m.Set("readOnly", ent.ReadOnly)

	return j.vm.ToValue(m)
}

// jsh.print("hello", "world")
func (j *Jsh) process_print(call js.FunctionCall) js.Value {
	args := make([]any, len(call.Arguments))
	for i := 0; i < len(call.Arguments); i++ {
		args[i] = call.Arguments[i].Export()
	}
	if err := j.print0(logging.LevelInfo, false, args...); err != nil {
		panic(j.vm.NewGoError(err))
	}
	return js.Undefined()
}

func (j *Jsh) process_println(call js.FunctionCall) js.Value {
	args := make([]any, len(call.Arguments))
	for i := 0; i < len(call.Arguments); i++ {
		args[i] = call.Arguments[i].Export()
	}
	if err := j.print0(logging.LevelInfo, true, args...); err != nil {
		panic(j.vm.NewGoError(err))
	}
	return js.Undefined()
}

func (j *Jsh) process_Table(call js.ConstructorCall) *js.Object {
	ret := j.vm.NewObject()
	tb := table.NewWriter()
	tb.SetStyle(table.StyleLight)

	ret.Set("appendRow", func(call js.FunctionCall) js.Value {
		row := make([]any, len(call.Arguments))
		for i := range call.Arguments {
			if err := j.vm.ExportTo(call.Arguments[i], &row[i]); err != nil {
				panic(j.vm.ToValue(fmt.Sprintf("Table.appendRow: invalid argument %s", err.Error())))
			}
		}
		tb.AppendRow(row)
		return js.Undefined()
	})
	ret.Set("resetRows", func(call js.FunctionCall) js.Value {
		tb.ResetRows()
		return js.Undefined()
	})
	ret.Set("appendHeader", func(call js.FunctionCall) js.Value {
		row := make([]any, len(call.Arguments))
		for i := range call.Arguments {
			if err := j.vm.ExportTo(call.Arguments[i], &row[i]); err != nil {
				panic(j.vm.ToValue(fmt.Sprintf("Table.appendRow: invalid argument %s", err.Error())))
			}
		}
		tb.AppendHeader(row)
		return js.Undefined()
	})
	ret.Set("resetHeaders", func(call js.FunctionCall) js.Value {
		tb.ResetHeaders()
		return js.Undefined()
	})
	ret.Set("render", func(call js.FunctionCall) js.Value {
		for _, ln := range strings.Split(tb.Render(), "\n") {
			j.Print(ln, "\n")
		}
		return js.Undefined()
	})
	return ret
}

// jsh.exec(["cmd", "arg1", "arg2"])
func (j *Jsh) process_exec(call js.FunctionCall) js.Value {
	if len(call.Arguments) == 0 {
		panic(j.vm.ToValue("exec: missing argument"))
	}
	args := make([]string, 0, len(call.Arguments))
	for i := 0; i < len(call.Arguments); i++ {
		if v, ok := call.Arguments[i].Export().(string); ok {
			args = append(args, v)
		} else {
			panic(j.vm.ToValue(fmt.Sprintf("exec: invalid argument %s", call.Arguments[i].ExportType())))
		}
	}
	err := j.NewChild().Exec(args)
	if err != nil {
		return j.vm.NewGoError(err)
	}
	return js.Undefined()
}

func (j *Jsh) realpath(path string) (string, error) {
	if realPath, err := ssfs.Default().FindRealPath(path); err != nil {
		return "", err
	} else {
		return realPath.AbsPath, nil
	}
}

func watcher(watch <-chan *Jsh, watchPath string, pReload *bool) {
	var pJsh *Jsh
	var lastModified time.Time
	for {
		select {
		case njsh := <-watch:
			if njsh == nil {
				return
			}
			pJsh = njsh
		case <-time.After(1 * time.Second):
			info, err := os.Stat(watchPath)
			if err != nil {
				return
			}
			modTime := info.ModTime()
			if lastModified.IsZero() {
				lastModified = modTime
				continue
			}
			if modTime.Equal(lastModified) {
				continue
			}
			lastModified = modTime
			*pReload = true
			if pJsh != nil {
				pJsh.Kill("reload")
			}
		}
	}
}

// jsh.daemonize()
func (j *Jsh) process_daemonize(call js.FunctionCall) js.Value {
	opts := struct {
		Reload bool `json:"reload"`
	}{
		Reload: false,
	}
	if len(call.Arguments) > 0 {
		if err := j.vm.ExportTo(call.Arguments[0], &opts); err != nil {
			panic(j.vm.ToValue(fmt.Sprintf("daemonize: invalid argument %s", err.Error())))
		}
	}

	//var lastModified time.Time
	var watchPath string
	if opts.Reload {
		if path, err := j.realpath(j.sourceName); err != nil {
			// daemonize: reload watcher failed
			opts.Reload = false
		} else {
			watchPath = path
		}
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				logging.GetLog("jsh").Errorf("daemonize: %v", r)
			}
		}()

		var chJsh chan *Jsh
		var doReload = false
		if opts.Reload {
			chJsh = make(chan *Jsh)
			defer close(chJsh)
			go watcher(chJsh, watchPath, &doReload)
		}

		logName := j.sourceName
		if logName == "" {
			logName = "jsh"
		}
		w := logging.GetLog(logName)
	reload:
		nJsh := NewJsh(
			&JshDaemonContext{Context: context.Background()}, // daemon
			WithParent(nil), // daemon
			WithWriter(w),   // log writer
			WithReader(bytes.NewBuffer(nil)),
			WithNativeModules(j.modules...),
			WithWorkingDir("/"),
			WithEcho(false),
			WithUserName(j.userName),
		)
		nJsh.program = j.program
		if opts.Reload && chJsh != nil {
			// notify current process
			chJsh <- nJsh
		}
		nJsh.Run(j.sourceName, "", j.args)
		if doReload {
			// if process terminated by reload-watcher
			doReload = false
			// reload source code
			_, j.sourceCode = j.searchPath(j.sourceName)
			if j.sourceCode == "" {
				panic(j.vm.ToValue(fmt.Sprintf("daemonize: %s not found", j.sourceName)))
			}
			if program, err := js.Compile(j.sourceName, j.sourceCode, false); err != nil {
				panic(j.vm.ToValue(fmt.Sprintf("daemonize: %s", err.Error())))
			} else {
				j.program = program
			}
			goto reload
		}
	}()

	return js.Undefined()
}

// jsh.schedule("@every 2s", (currentTime, token) => { println("hello"); token.stop(); })
func (j *Jsh) schedule(call js.FunctionCall) js.Value {
	defaultCron := util.DefaultCron()
	if defaultCron == nil {
		panic(j.vm.ToValue("schedule: scheduler does not exist"))
	}
	if len(call.Arguments) < 2 {
		panic(j.vm.ToValue("schedule: missing argument"))
	}
	spec := call.Arguments[0].String()
	var callback js.Callable
	if fn, ok := js.AssertFunction(call.Arguments[1]); !ok {
		panic(j.vm.ToValue("schedule: invalid callback"))
	} else {
		callback = fn
	}

	done := make(chan struct{})
	token := j.vm.NewObject()
	token.Set("stop", func(call js.FunctionCall) js.Value {
		j.Log(logging.LevelWarn, "schedule: stopped")
		close(done)
		return js.Undefined()
	})
	entryId, err := defaultCron.AddFunc(spec, func() {
		_, err := callback(js.Undefined(), j.vm.ToValue(time.Now().Unix()*1000), token)
		if err != nil {
			if _, ok := err.(*js.InterruptedError); !ok {
				// ignore interrupted error
				j.Log(logging.LevelWarn, "schedule: callback", err)
			}
			close(done)
		}
	})
	if err != nil {
		j.Log(logging.LevelWarn, "schedule:", err.Error())
		return js.Undefined()
	}
	select {
	case <-done:
	case <-j.Context.Done():
		j.Log(logging.LevelWarn, "schedule: canceled")
	case <-j.chKill:
		j.Log(logging.LevelWarn, "schedule: interrupted")
	}
	defaultCron.Remove(entryId)
	return js.Undefined()
}

// jsh.sleep()
func (j *Jsh) process_sleep(call js.FunctionCall) js.Value {
	if len(call.Arguments) == 0 {
		panic(j.vm.ToValue("sleep: missing argument"))
	}
	dur := time.Duration(0)
	switch v := call.Arguments[0].Export().(type) {
	case time.Duration:
		dur = v
	case int:
		dur = time.Duration(v) * time.Millisecond
	case int32:
		dur = time.Duration(v) * time.Millisecond
	case int64:
		dur = time.Duration(v) * time.Millisecond
	case float32:
		dur = time.Duration(v) * time.Millisecond
	case float64:
		dur = time.Duration(v) * time.Millisecond
	case string:
		if d, err := time.ParseDuration(v); err == nil {
			dur = d
		} else {
			panic(j.vm.ToValue(fmt.Sprintf("sleep: invalid argument %s", v)))
		}
	}
	select {
	case <-j.Context.Done():
		j.Log(logging.LevelWarn, "sleep: canceled")
	case <-j.chKill:
		j.Log(logging.LevelWarn, "sleep: interrupted")
	case <-time.After(dur):
	}
	return js.Undefined()
}

// jsh.kill(pid)
func (j *Jsh) process_kill(call js.FunctionCall) js.Value {
	if len(call.Arguments) == 0 {
		return js.Undefined()
	}
	if pid, ok := call.Arguments[0].Export().(int64); ok {
		if p, exists := jshProcesses[JshPID(pid)]; exists && p != nil {
			p.Kill("killed")
		} else {
			panic(j.vm.ToValue(fmt.Sprintf("kill: pid %d not found", pid)))
		}
	} else {
		panic(j.vm.ToValue(fmt.Sprintf("kill: invalid argument %s", call.Arguments[0].ExportType())))
	}
	return js.Undefined()
}

// jsh.ps()
func (j *Jsh) process_ps(call js.FunctionCall) js.Value {
	jshMutex.RLock()
	defer jshMutex.RUnlock()

	var toObj = func(p *Jsh) *js.Object {
		obj := j.vm.NewObject()
		obj.Set("pid", uint32(p.pid))
		obj.Set("ppid", uint32(p.ppid))
		obj.Set("user", p.userName)
		obj.Set("name", p.sourceName)
		obj.Set("startAt", p.startAt)
		obj.Set("uptime", time.Since(p.startAt).Round(time.Second).String())
		return obj
	}
	if len(call.Arguments) > 0 {
		var pid uint32
		if err := j.vm.ExportTo(call.Arguments[0], &pid); err != nil {
			panic(j.vm.ToValue(fmt.Sprintf("invalid argument %s", call.Arguments[0].ExportType())))
		}
		if p, exists := jshProcesses[JshPID(pid)]; exists && p != nil {
			return j.vm.ToValue(toObj(p))
		} else {
			panic(j.vm.ToValue(fmt.Sprintf("ps: pid %d not found", pid)))
		}
	}

	ret := make([]*js.Object, 0, len(jshProcesses))
	for _, p := range jshProcesses {
		if p == nil {
			continue
		}
		obj := toObj(p)
		ret = append(ret, obj)
	}
	return j.vm.ToValue(ret)
}

// tok = jsh.addCleanup(() => {})
func (j *Jsh) process_addCleanup(call js.FunctionCall) js.Value {
	if len(call.Arguments) == 0 {
		panic(j.vm.ToValue("missing argument"))
	}
	cb, ok := call.Arguments[0].Export().(func(js.FunctionCall) js.Value)
	if !ok {
		panic(j.vm.ToValue(fmt.Sprintf("invalid cleanup type: %s", call.Arguments[0].ExportType())))
	}
	fn := func(io.Writer) {
		defer func() {
			if e := recover(); e != nil {
				if err, ok := e.(error); ok {
					j.resultErr = append(j.resultErr, err)
				}
			}
		}()
		cb(js.FunctionCall{This: js.Undefined(), Arguments: []js.Value{}})
	}
	id := j.AddCleanup(fn)
	return j.vm.ToValue(id)
}

func (j *Jsh) process_removeCleanup(call js.FunctionCall) js.Value {
	if len(call.Arguments) == 0 {
		panic(j.vm.ToValue("missing argument"))
	}
	id, ok := call.Arguments[0].Export().(int64)
	if !ok {
		panic(j.vm.ToValue(fmt.Sprintf("invalid cleanup type: %s", call.Arguments[0].ExportType())))
	}
	j.RemoveCleanup(id)
	return js.Undefined()
}

func (j *Jsh) process_parseCommandLine(call js.FunctionCall) js.Value {
	if len(call.Arguments) == 0 {
		panic(j.vm.ToValue("missing argument"))
	}
	line, ok := call.Arguments[0].Export().(string)
	if !ok {
		panic(j.vm.ToValue(fmt.Sprintf("invalid argument %s", call.Arguments[0].ExportType())))
	}
	parts := ParseCommandLine(line)
	return j.vm.ToValue(parts)
}

type CommandPart struct {
	Args     []string `json:"args"`
	Pipe     bool     `json:"pipe"`
	Redirect string   `json:"redirect"` // ">" or ">>"
	Target   string   `json:"target"`
}

// parse line into args, pipe('|') and redirect('>') and redirect('>>')
func ParseCommandLine(line string) []CommandPart {
	var parts []CommandPart
	var current CommandPart
	var buffer string
	var inSingleQuotes, inDoubleQuotes bool

	for i := 0; i < len(line); i++ {
		char := line[i]

		switch char {
		case ' ':
			if inSingleQuotes || inDoubleQuotes {
				buffer += string(char)
			} else if buffer != "" {
				current.Args = append(current.Args, buffer)
				buffer = ""
			}
		case '"':
			if inSingleQuotes {
				buffer += string(char)
			} else {
				inDoubleQuotes = !inDoubleQuotes
			}
		case '\'':
			if inDoubleQuotes {
				buffer += string(char)
			} else {
				inSingleQuotes = !inSingleQuotes
			}
		case '|':
			if inSingleQuotes || inDoubleQuotes {
				buffer += string(char)
			} else {
				if buffer != "" {
					current.Args = append(current.Args, buffer)
					buffer = ""
				}
				current.Pipe = true
				parts = append(parts, current)
				current = CommandPart{}
			}
		case '>':
			if inSingleQuotes || inDoubleQuotes {
				buffer += string(char)
			} else {
				if buffer != "" {
					current.Args = append(current.Args, buffer)
					buffer = ""
				}
				if i+1 < len(line) && line[i+1] == '>' {
					current.Redirect = ">>"
					i++
				} else {
					current.Redirect = ">"
				}
				// Capture the target file
				for i+1 < len(line) && line[i+1] == ' ' {
					i++
				}
				start := i + 1
				for i+1 < len(line) && line[i+1] != ' ' && line[i+1] != '|' {
					i++
				}
				current.Target = line[start : i+1]
				parts = append(parts, current)
				current = CommandPart{}
			}
		default:
			buffer += string(char)
		}
	}

	if buffer != "" {
		current.Args = append(current.Args, buffer)
	}
	if len(current.Args) > 0 || current.Pipe || current.Redirect != "" {
		parts = append(parts, current)
	}
	return parts
}

// jsh.serviceStatus()
func (j *Jsh) process_serviceStatus(call js.FunctionCall) js.Value {
	serviceName := ""
	if len(call.Arguments) > 0 {
		err := j.vm.ExportTo(call.Arguments[0], &serviceName)
		if err != nil {
			panic(j.vm.ToValue(fmt.Sprintf("invalid argument %s", call.Arguments[0].ExportType())))
		}
	}

	jshServicesLock.RLock()
	defer jshServicesLock.RUnlock()
	ret := make([]map[string]any, 0)
	if serviceName != "" {
		s, ok := jshServices[serviceName]
		if ok {
			obj := map[string]any{}
			obj["name"] = s.Config.Name
			obj["enable"] = s.Config.Enable
			if s.pid > 0 {
				obj["pid"] = s.pid
				obj["status"] = "Running"
			} else {
				obj["pid"] = 0
				obj["status"] = "Stopped"
			}
			ret = append(ret, obj)
		}
	} else {
		for name, s := range jshServices {
			obj := map[string]any{}
			obj["name"] = name
			obj["enable"] = s.Config.Enable
			if s.pid > 0 {
				obj["pid"] = s.pid
				obj["status"] = "Running"
			} else {
				obj["pid"] = 0
				obj["status"] = "Stopped"
			}
			ret = append(ret, obj)
		}
	}
	return j.vm.ToValue(ret)
}

// jsh.serviceStart(svc)
func (j *Jsh) process_serviceStart(call js.FunctionCall) js.Value {
	if len(call.Arguments) == 0 {
		panic(j.vm.ToValue("serviceStart: missing argument"))
	}
	svc, ok := call.Arguments[0].Export().(string)
	if !ok {
		panic(j.vm.ToValue(fmt.Sprintf("serviceStart: invalid argument %s", call.Arguments[0].ExportType())))
	}
	s, ok := jshServices[svc]
	if !ok {
		panic(j.vm.ToValue(fmt.Sprintf("serviceStart: service %s not found", svc)))
	}
	if s.pid > 0 {
		panic(j.vm.ToValue(fmt.Sprintf("serviceStart: service %s already running", svc)))
	}
	s.Config.Start()
	return js.Undefined()
}

// jsh.serviceStop(svc)
func (j *Jsh) process_serviceStop(call js.FunctionCall) js.Value {
	if len(call.Arguments) == 0 {
		panic(j.vm.ToValue("serviceStop: missing argument"))
	}
	svc, ok := call.Arguments[0].Export().(string)
	if !ok {
		panic(j.vm.ToValue(fmt.Sprintf("serviceStop: invalid argument %s", call.Arguments[0].ExportType())))
	}
	s, ok := jshServices[svc]
	if !ok {
		panic(j.vm.ToValue(fmt.Sprintf("serviceStop: service %s not found", svc)))
	}
	if s.pid == 0 {
		panic(j.vm.ToValue(fmt.Sprintf("serviceStop: service %s not running", svc)))
	}
	s.Config.Stop()
	return js.Undefined()
}

// jsh.serviceReread()
func (j *Jsh) process_serviceReread(call js.FunctionCall) js.Value {
	if len(call.Arguments) > 0 {
		panic(j.vm.ToValue("serviceReread: no argument"))
	}
	list, err := ReadServices()
	if err != nil {
		if os.IsNotExist(err) {
			return js.Null()
		} else {
			panic(j.vm.ToValue(fmt.Sprintf("serviceReread: %s", err.Error())))
		}
	}
	return j.vm.ToValue(list)
}

// jsh.serviceUpdate()
func (j *Jsh) process_serviceUpdate(call js.FunctionCall) js.Value {
	if len(call.Arguments) > 0 {
		panic(j.vm.ToValue("serviceUpdate: no argument"))
	}
	list, err := ReadServices()
	if err != nil {
		panic(j.vm.ToValue(fmt.Sprintf("serviceUpdate: %s", err.Error())))
	}

	result := make([]map[string]any, 0)
	list.Update(func(sc *ServiceConfig, s string, err error) {
		if err != nil {
			result = append(result, map[string]any{
				"name":   sc.Name,
				"status": "error",
				"error":  err.Error(),
			})
		} else {
			result = append(result, map[string]any{
				"name":   sc.Name,
				"status": s,
			})
		}
	})
	return j.vm.ToValue(result)
}
