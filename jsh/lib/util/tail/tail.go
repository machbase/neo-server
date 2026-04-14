package tail

import (
	"bufio"
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"io"
	"os"

	"github.com/dop251/goja"
)

type hostPathResolver interface {
	ResolveHostPath(name string) (string, error)
}

//go:embed tail.js
var tailJS []byte

//go:embed sse.js
var tailSSEJS []byte

func Files() map[string][]byte {
	return map[string][]byte{
		"util/tail.js":       tailJS,
		"util/tail/index.js": tailJS,
		"util/tail/sse.js":   tailSSEJS,
	}
}

func Module(_ context.Context, rt *goja.Runtime, module *goja.Object) {
	installModule(rt, module, nil)
}

func ModuleWithFS(resolver hostPathResolver) func(context.Context, *goja.Runtime, *goja.Object) {
	return func(_ context.Context, rt *goja.Runtime, module *goja.Object) {
		installModule(rt, module, resolver)
	}
}

func installModule(rt *goja.Runtime, module *goja.Object, resolver hostPathResolver) {
	m := module.Get("exports").(*goja.Object)
	m.Set("create", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			panic(rt.ToValue("tail.create(path, options?): path is required"))
		}
		path := call.Argument(0).String()
		hostPath := path
		if resolver != nil {
			resolved, err := resolver.ResolveHostPath(path)
			if err != nil {
				panic(rt.NewGoError(err))
			}
			hostPath = resolved
		}
		opts := tailOptions{}
		if len(call.Arguments) > 1 && !goja.IsUndefined(call.Argument(1)) && !goja.IsNull(call.Argument(1)) {
			if err := rt.ExportTo(call.Argument(1), &opts); err != nil {
				panic(rt.NewGoError(err))
			}
		}
		tailer := newTailer(hostPath, opts)
		obj := rt.NewObject()
		obj.Set("poll", func(call goja.FunctionCall) goja.Value {
			lines, err := tailer.Poll()
			if err != nil {
				panic(rt.NewGoError(err))
			}
			if len(call.Arguments) > 0 && !goja.IsUndefined(call.Argument(0)) && !goja.IsNull(call.Argument(0)) {
				if fn, ok := goja.AssertFunction(call.Argument(0)); ok {
					if _, err := fn(goja.Undefined(), rt.ToValue(lines)); err != nil {
						panic(err)
					}
				} else {
					panic(rt.ToValue("tail.poll(callback): callback must be a function"))
				}
			}
			return rt.ToValue(lines)
		})
		obj.Set("close", func() {
			_ = tailer.Close()
		})
		obj.Set("path", path)
		return obj
	})
}

type tailOptions struct {
	FromStart bool `json:"fromStart"`
}

type tailer struct {
	path      string
	fromStart bool

	initialized bool
	file        *os.File
	lastPos     int64
	lastSize    int64
	lastInode   uint64
	lineBuf     []byte
}

func newTailer(path string, opts tailOptions) *tailer {
	return &tailer{
		path:      path,
		fromStart: opts.FromStart,
	}
}

func (t *tailer) Poll() ([]string, error) {
	if err := t.ensureOpen(); err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}
	if t.file == nil {
		return []string{}, nil
	}

	rotated, err := t.detectRotation()
	if err != nil {
		return nil, err
	}
	if rotated {
		remaining, err := t.readNewLinesFromCurrent()
		if err != nil {
			return nil, err
		}
		if err := t.reopen(); err != nil {
			if os.IsNotExist(err) {
				return remaining, nil
			}
			return nil, err
		}
		fresh, err := t.readNewLinesFromCurrent()
		if err != nil {
			return nil, err
		}
		return append(remaining, fresh...), nil
	}

	stat, err := os.Stat(t.path)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}
	currentSize := stat.Size()
	if currentSize < t.lastPos || currentSize < t.lastSize {
		t.lastPos = 0
		t.lastSize = currentSize
		t.lineBuf = t.lineBuf[:0]
		if _, err := t.file.Seek(0, io.SeekStart); err != nil {
			return nil, err
		}
	}

	lines, err := t.readNewLinesFromCurrent()
	if err != nil {
		return nil, err
	}
	return lines, nil
}

func (t *tailer) Close() error {
	if t.file == nil {
		return nil
	}
	err := t.file.Close()
	t.file = nil
	t.lastPos = 0
	t.lastSize = 0
	t.lastInode = 0
	t.initialized = false
	t.lineBuf = nil
	return err
}

func (t *tailer) ensureOpen() error {
	if t.file != nil {
		return nil
	}
	file, err := openFileShared(t.path)
	if err != nil {
		return err
	}
	stat, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return err
	}
	inode, err := getFileIDFromHandle(file)
	if err != nil {
		inode = getInode(stat)
	}
	t.file = file
	t.lastInode = inode
	t.lastSize = stat.Size()
	if !t.initialized {
		if t.fromStart {
			t.lastPos = 0
		} else {
			t.lastPos = stat.Size()
			if _, err := t.file.Seek(t.lastPos, io.SeekStart); err != nil {
				_ = t.file.Close()
				t.file = nil
				return err
			}
		}
		t.initialized = true
		return nil
	}
	t.lastPos = 0
	if _, err := t.file.Seek(0, io.SeekStart); err != nil {
		_ = t.file.Close()
		t.file = nil
		return err
	}
	return nil
}

func (t *tailer) detectRotation() (bool, error) {
	if t.file == nil {
		return false, nil
	}
	file, err := openFileShared(t.path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil {
		return false, err
	}
	newInode, err := getFileIDFromHandle(file)
	if err != nil {
		newInode = getInode(stat)
	}
	if t.lastInode == 0 || newInode == 0 {
		return false, nil
	}
	return newInode != t.lastInode, nil
}

func (t *tailer) reopen() error {
	if t.file != nil {
		_ = t.file.Close()
	}
	t.file = nil
	t.lastPos = 0
	t.lastSize = 0
	t.lastInode = 0
	t.lineBuf = t.lineBuf[:0]
	return t.ensureOpen()
}

func (t *tailer) readNewLinesFromCurrent() ([]string, error) {
	if t.file == nil {
		return []string{}, nil
	}
	if _, err := t.file.Seek(t.lastPos, io.SeekStart); err != nil {
		return nil, err
	}
	reader := bufio.NewReaderSize(t.file, 4096)
	var lines []string
	for {
		chunk, err := reader.ReadBytes('\n')
		if len(chunk) > 0 {
			t.lastPos += int64(len(chunk))
			if chunk[len(chunk)-1] == '\n' {
				full := append(t.lineBuf, chunk[:len(chunk)-1]...)
				t.lineBuf = t.lineBuf[:0]
				full = bytes.TrimSuffix(full, []byte{'\r'})
				lines = append(lines, string(full))
			} else {
				t.lineBuf = append(t.lineBuf, chunk...)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("tail read failed: %w", err)
		}
	}
	if stat, err := t.file.Stat(); err == nil {
		t.lastSize = stat.Size()
	}
	return lines, nil
}
