package tql

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math"
	"os"
	"runtime/debug"
	"strings"
	"sync"

	"github.com/machbase/neo-server/mods/expression"
	"github.com/machbase/neo-server/mods/stream"
	"github.com/machbase/neo-server/mods/stream/spec"
	spi "github.com/machbase/neo-spi"
	"github.com/pkg/errors"
)

type Task struct {
	ctx          context.Context
	ctxCancel    context.CancelFunc
	params       map[string][]string
	db           spi.Database
	inputReader  io.Reader
	outputWriter spec.OutputStream
	toJsonOutput bool
	logWriter    io.Writer

	// comments start with plus(+) symbold and sperated by comma.
	// ex) => `// +brief, markdown`
	pragma []string

	// compiled result
	compiled   bool
	compileErr error
	output     *output
	nodes      []*Node

	_shouldStop    bool
	_resultColumns spi.Columns
	_stateLock     sync.RWMutex
}

var (
	_ expression.Parameters = &Task{}
)

func NewTask() *Task {
	return NewTaskContext(context.Background())
}

func NewTaskContext(ctx context.Context) *Task {
	ret := &Task{}
	ret.ctx, ret.ctxCancel = context.WithCancel(ctx)
	return ret
}

func (x *Task) SetDatabase(db spi.Database) {
	x.db = db
}

func (x *Task) SetInputReader(r io.Reader) {
	x.inputReader = r
}

func (x *Task) InputReader() io.Reader {
	return x.inputReader
}

func (x *Task) SetOutputWriter(w io.Writer) error {
	var err error
	if w == nil {
		x.outputWriter, err = stream.NewOutputStream("-")
		if err != nil {
			return err
		}
	} else if o, ok := w.(spec.OutputStream); ok {
		x.outputWriter = o
	} else {
		x.outputWriter = &stream.WriterOutputStream{Writer: w}
	}
	return nil
}

func (x *Task) SetOutputWriterJson(w io.Writer, json bool) {
	x.SetOutputWriter(w)
	x.toJsonOutput = json
}

func (x *Task) OutputWriter() spec.OutputStream {
	if x.outputWriter == nil {
		x.outputWriter, _ = stream.NewOutputStream("-")
	}
	return x.outputWriter
}

func (x *Task) SetLogWriter(w io.Writer) {
	x.logWriter = w
}

func (x *Task) SetParams(p map[string][]string) {
	if x.params == nil {
		x.params = map[string][]string{}
	}
	for k, v := range p {
		x.params[k] = v
	}
}

func (x *Task) Params() map[string][]string {
	return x.params
}

// Get implements expression.Parameters
func (x *Task) Get(name string) (any, error) {
	if strings.HasPrefix(name, "$") {
		if p, ok := x.params[strings.TrimPrefix(name, "$")]; ok {
			if len(p) > 0 {
				return p[len(p)-1], nil
			}
		}
		return nil, nil
	} else {
		switch name {
		default:
			return nil, fmt.Errorf("undefined variable '%s'", name)
		case "CTX":
			return x, nil
		case "PI":
			return math.Pi, nil
		case "outputstream":
			return x.outputWriter, nil
		case "nil":
			return nil, nil
		}
	}
}

func (x *Task) CompileScript(sc *Script) error {
	file, err := os.Open(sc.path)
	if err != nil {
		return err
	}
	defer file.Close()
	return x.Compile(file)
}

func (x *Task) CompileString(code string) error {
	return x.Compile(bytes.NewBufferString(code))
}

func (x *Task) Compile(codeReader io.Reader) error {
	err := x.compile(codeReader)
	if err != nil {
		x.LogError("Compile", err.Error())
	}
	return err
}

func (x *Task) compile(codeReader io.Reader) error {
	lines, err := readLines(x, codeReader)
	if err != nil {
		x.compileErr = err
		return err
	}
	if len(lines) == 0 {
		x.compileErr = errors.New("empty expressions")
		return x.compileErr
	}

	var exprs []*Line
	for _, line := range lines {
		if line.isComment {
			// //+pragma
			if strings.HasPrefix(line.text, "+") {
				toks := strings.Split(line.text[1:], ",")
				for _, t := range toks {
					x.pragma = append(x.pragma, strings.TrimSpace(t))
				}
			}
		} else {
			exprs = append(exprs, line)
		}
	}

	lastIdx := -1
	if len(exprs) > 1 {
		lastIdx = len(exprs) - 1
	}
	for n, mapLine := range exprs {
		if n != lastIdx {
			// src and map
			node := NewNode(x)
			if err := node.compile(mapLine.text); err != nil {
				return err
			}
			x.nodes = append(x.nodes, node)
			if n > 0 {
				x.nodes[n-1].next = x.nodes[n]
			}
		} else {
			// sink
			x.output, err = NewNode(x).compileSink(exprs[len(exprs)-1].text)
			if err != nil {
				x.compileErr = errors.Wrapf(err, "line %d", exprs[len(exprs)-1].line)
				return x.compileErr
			}
			x.nodes[len(exprs)-2].next = x.output
		}
	}

	x.compiled = true
	return nil
}

func (x *Task) Execute() error {
	err := x.execute()
	if err != nil {
		x.LogError("execute error", err.Error())
	}
	return err
}

func (x *Task) execute() (err error) {
	if !x.compiled {
		return errors.New("not compiled task")
	}
	defer func() {
		if r := recover(); r != nil {
			w := &bytes.Buffer{}
			w.Write(debug.Stack())
			x.LogErrorf("panic %v\n%s", r, w.String())
		}
	}()

	// start output
	if x.output != nil {
		x.output.start()
	}
	// start nodes
	for _, child := range x.nodes {
		child.start()
	}
	NewRecord("", nil).Tell(x.nodes[0])
	EofRecord.Tell(x.nodes[0])

	// wait all nodes are finished
	for _, child := range x.nodes {
		child.stop()
	}
	if x.output != nil {
		x.output.stop()
	}

	if err == nil && x.output != nil {
		err = x.output.lastError
	}
	return
}

func (x *Task) onCircuitBreak(fromNode *Node) {
	x._stateLock.Lock()
	x._shouldStop = true
	x._stateLock.Unlock()
}

func (x *Task) shouldStop() bool {
	x._stateLock.RLock()
	ret := x._shouldStop
	x._stateLock.RUnlock()
	return ret
}

func (x *Task) SetResultColumns(cols spi.Columns) {
	x._stateLock.Lock()
	x._resultColumns = cols
	x._stateLock.Unlock()
}

func (x *Task) ResultColumns() spi.Columns {
	x._stateLock.RLock()
	ret := x._resultColumns
	x._stateLock.RUnlock()
	return ret
}

func (x *Task) OutputContentType() string {
	if x.output != nil {
		ret := x.output.ContentType()
		return ret
	}
	return "application/octet-stream"
}

func (x *Task) OutputContentEncoding() string {
	if x.output != nil {
		if contentEncoding := x.output.ContentEncoding(); len(contentEncoding) > 0 {
			return contentEncoding
		}
	}
	return "identity"
}

func (x *Task) OutputChartType() string {
	if x.output != nil && x.output.IsChart() {
		return "echarts"
	}
	return ""
}

type TaskLog interface {
	Logf(format string, args ...any)
	Log(args ...any)
	LogDebugf(format string, args ...any)
	LogDebug(args ...any)
	LogWarnf(format string, args ...any)
	LogWarn(args ...any)
	LogErrorf(format string, args ...any)
	LogError(args ...any)
}

func (x *Task) Logf(format string, args ...any)      { x._logf("INFO", format, args...) }
func (x *Task) LogDebugf(format string, args ...any) { x._logf("DEBUG", format, args...) }
func (x *Task) LogWarnf(format string, args ...any)  { x._logf("WARN", format, args...) }
func (x *Task) LogErrorf(format string, args ...any) { x._logf("ERROR", format, args...) }

func (x *Task) Log(args ...any)      { x._log("INFO", args...) }
func (x *Task) LogDebug(args ...any) { x._log("DEBUG", args...) }
func (x *Task) LogWarn(args ...any)  { x._log("WARN", args...) }
func (x *Task) LogError(args ...any) { x._log("ERROR", args...) }

func (x *Task) LogFatalf(format string, args ...any) {
	stack := string(debug.Stack())
	x._logf("FATAL", format+"\n%s", append(args, stack))
}

func (x *Task) LogFatal(args ...any) {
	stack := string(debug.Stack())
	x._log("FATAL", append(args, "\n", stack))
}

func (x *Task) _log(prefix string, args ...any) {
	if x.logWriter == nil {
		fmt.Println(append([]any{prefix}, args...)...)
	} else {
		line := fmt.Sprintln(append([]any{prefix}, args...)...) + "\n"
		x.logWriter.Write([]byte(line))
	}
}

func (x *Task) _logf(prefix string, format string, args ...any) {
	if x.logWriter == nil {
		fmt.Printf("[%s] "+format+"\n", append([]any{prefix}, args...)...)
	} else {
		line := fmt.Sprintln(append([]any{prefix}, args...)...) + "\n"
		x.logWriter.Write([]byte(line))
	}
}
