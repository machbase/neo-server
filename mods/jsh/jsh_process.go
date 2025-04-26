package jsh

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"time"

	js "github.com/dop251/goja"
	"github.com/machbase/neo-server/v8/mods/eventbus"
	"github.com/machbase/neo-server/v8/mods/jsh/system"
	"github.com/machbase/neo-server/v8/mods/logging"
	"github.com/machbase/neo-server/v8/mods/util/ssfs"
)

func (j *Jsh) moduleProcess(r *js.Runtime, module *js.Object) {
	// m = require("@jsh/process")
	o := module.Get("exports").(*js.Object)
	o.Set("pid", j.process_pid)
	o.Set("ppid", j.process_ppid)
	// m.args
	o.Set("args", j.process_args)
	// m.cwd()
	o.Set("cwd", j.process_cwd)
	// m.chdir("/path/to/dir")
	o.Set("chdir", j.process_chdir)
	// m.readdir("/path/to/dir", (entry) => {})
	o.Set("readdir", j.process_readdir)
	// m.readline()
	o.Set("readline", j.process_readline)
	// m.stdout
	o.Set("print", j.process_print)
	// m.exec(args)
	o.Set("exec", j.process_exec)
	// m.daemonize()
	o.Set("daemonize", j.process_daemonize)
	// m.sleep(ms)
	o.Set("sleep", system.Sleep(j, r))
	// m.kill(pid)
	o.Set("kill", j.process_kill)
	// m.ps()
	o.Set("ps", j.process_ps)
	// m.openEditor("/path/to/file")
	o.Set("openEditor", j.process_openEditor)
	// tok = m.addCleanup(()=>{})
	o.Set("addCleanup", j.process_addCleanup)
	// m.removeCleanup(tok)
	o.Set("removeCleanup", j.process_removeCleanup)
}

// jsh.pid()
func (j *Jsh) process_pid() js.Value {
	return j.vm.ToValue(j.pid)
}

// jsh.ppid()
func (j *Jsh) process_ppid() js.Value {
	return j.vm.ToValue(j.ppid)
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
	if j.consoleId == "" || j.userName == "" {
		panic(j.vm.ToValue("openEditor: no console bind"))
	}
	if len(call.Arguments) == 0 {
		panic(j.vm.ToValue("openEditor: missing argument"))
	}
	var path string
	j.vm.ExportTo(call.Arguments[0], &path)
	eventbus.PublishOpenFile(fmt.Sprintf("console:%s:%s", j.userName, j.consoleId), path)
	return js.Undefined()
}

// jsh.chdir("/path/to/dir")
func (j *Jsh) process_chdir(call js.FunctionCall) js.Value {
	if len(call.Arguments) == 0 {
		panic(j.vm.ToValue(fmt.Sprintf("chdir: invalid argument %s", call.Arguments[0].ExportType())))
	}
	path, ok := call.Arguments[0].Export().(string)
	if !ok {
		panic(j.vm.ToValue(fmt.Sprintf("chdir: invalid argument %s", call.Arguments[0].ExportType())))
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
	j.cwd = path
	return js.Undefined()
}

// jsh.readdir("/path/to/dir", (dir) => {})
func (j *Jsh) process_readdir(call js.FunctionCall) js.Value {
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
		panic(j.vm.NewGoError(err))
	}
	if !ent.IsDir {
		panic(j.vm.ToValue(fmt.Errorf("%s is not a directory", path)))
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

// jsh.print("hello", "world")
func (j *Jsh) process_print(call js.FunctionCall) js.Value {
	args := make([]any, len(call.Arguments))
	for i := 0; i < len(call.Arguments); i++ {
		args[i] = call.Arguments[i].Export()
	}
	if err := j.Print(args...); err != nil {
		panic(j.vm.NewGoError(err))
	}
	return js.Undefined()
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

// jsh.daemonize()
func (j *Jsh) process_daemonize(call js.FunctionCall) js.Value {
	go func(name string, program *js.Program, args []string) {
		ctx := &JshDaemonContext{Context: context.Background()}
		logName := name
		if logName == "" {
			logName = "jsh"
		}
		w := logging.GetLog(logName)
		nJsh := NewJsh(
			ctx,             // daemon
			WithParent(nil), // daemon
			WithWriter(w),   // log writer
			WithReader(bytes.NewBuffer(nil)),
			WithNativeModules(j.modules...),
			WithWorkingDir("/"),
			WithEcho(false),
		)
		nJsh.program = j.program
		nJsh.Run(name, "", args)
	}(j.sourceName, j.program, []string{})
	return js.Undefined()
}

// jsh.kill(pid)
func (j *Jsh) process_kill(call js.FunctionCall) js.Value {
	if len(call.Arguments) == 0 {
		return js.Undefined()
	}
	if pid, ok := call.Arguments[0].Export().(int64); ok {
		if p, exists := jshProcesses[JshPID(pid)]; exists && p != nil {
			p.vm.Interrupt("killed")
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

	ret := make([]*js.Object, 0, len(jshProcesses))
	for _, p := range jshProcesses {
		if p == nil {
			continue
		}
		obj := j.vm.NewObject()
		obj.Set("pid", uint32(p.pid))
		obj.Set("ppid", uint32(p.ppid))
		obj.Set("user", p.userName)
		obj.Set("name", p.sourceName)
		obj.Set("startAt", p.startAt)
		obj.Set("uptime", time.Since(p.startAt).Round(time.Second).String())
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
