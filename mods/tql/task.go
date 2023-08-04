package tql

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"runtime/debug"
	"strings"

	"github.com/machbase/neo-server/mods/expression"
	"github.com/machbase/neo-server/mods/stream"
	"github.com/machbase/neo-server/mods/stream/spec"
	spi "github.com/machbase/neo-spi"
	"github.com/pkg/errors"
)

type Task struct {
	ctx          context.Context
	params       map[string][]string
	inputReader  io.Reader
	outputWriter spec.OutputStream
	toJsonOutput bool

	// comments start with plus(+) symbold and sperated by comma.
	// ex) => `// +brief, markdown`
	pragma []string

	// compiled result
	compiled   bool
	compileErr error
	input      *input
	output     *output
	nodes      []*Node
}

var (
	_ expression.Parameters = &Task{}
)

func NewTask() *Task {
	return &Task{}
}

func NewTaskContext(ctx context.Context) *Task {
	ret := &Task{}
	ret.ctx = ctx
	return ret
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
		x.LogError("Compile %s", err.Error())
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

	// src
	if len(exprs) >= 1 {
		x.input, err = NewNode(x).compileSource(exprs[0].text)
		if err != nil {
			x.compileErr = errors.Wrapf(err, "at line %d", exprs[0].line)
			return x.compileErr
		}
	}

	// sink
	if len(exprs) >= 2 {
		x.output, err = NewNode(x).compileSink(exprs[len(exprs)-1].text)
		if err != nil {
			x.compileErr = errors.Wrapf(err, "at line %d", exprs[len(exprs)-1].line)
			return x.compileErr
		}
		x.output.log = x
	}

	// map
	if len(exprs) >= 3 {
		exprs = exprs[1 : len(exprs)-1]
		for n, mapLine := range exprs {
			node := NewNode(x)
			if err := node.compile(mapLine.text); err != nil {
				return err
			}
			node.log = x
			x.nodes = append(x.nodes, node)
			if n > 0 {
				x.nodes[n-1].next = x.nodes[n]
			}
			x.nodes[n].next = x.output
		}
	}
	if len(x.nodes) > 0 {
		x.input.next = x.nodes[0]
	} else {
		x.input.next = x.output
	}
	x.compiled = true
	return nil
}

func (x *Task) ExecuteHandler(db spi.Database, w http.ResponseWriter) error {
	w.Header().Set("Content-Type", x.output.ContentType())
	if contentEncoding := x.output.ContentEncoding(); len(contentEncoding) > 0 {
		w.Header().Set("Content-Encoding", contentEncoding)
	}
	if x.output.IsChart() {
		w.Header().Set("X-Chart-Type", "echarts")
	}
	return x.Execute(db)
}

func (x *Task) Execute(db spi.Database) error {
	err := x.execute(db)
	if err != nil {
		x.LogError("Execute %s", err.Error())
	}
	return err
}

func (x *Task) execute(db spi.Database) (err error) {
	if !x.compiled {
		return errors.New("not compiled task")
	}
	if x.input == nil || x.output == nil {
		return errors.New("task has no input or output")
	}

	x.input.db = db
	x.output.db = db
	// start output
	x.output.start()
	// start nodes
	for _, child := range x.nodes {
		child.start()
	}
	// run input
	err = x.input.start()

	// wait all nodes are finished
	for _, child := range x.nodes {
		child.stop()
	}
	x.output.stop()

	if err == nil {
		err = x.output.lastError
	}
	return
}

// DumpSQL returns the generated SQL statement if the input source is a database source
func (x *Task) DumpSQL() string {
	if x.input == nil || x.input.dbSrc == nil {
		return ""
	}
	return x.input.dbSrc.ToSQL()
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

func (x *Task) Logf(format string, args ...any) {
	fmt.Printf("[INFO] "+format+"\n", args...)
}

func (x *Task) Log(args ...any) {
	fmt.Println(append([]any{"[INFO]"}, args...)...)
}

func (x *Task) LogDebugf(format string, args ...any) {
	fmt.Printf("[DEBUG] "+format+"\n", args...)
}

func (x *Task) LogDebug(args ...any) {
	fmt.Println(append([]any{"[DEBUG]"}, args...)...)
}

func (x *Task) LogWarnf(format string, args ...any) {
	fmt.Printf("[WARN] "+format+"\n", args...)
}

func (x *Task) LogWarn(args ...any) {
	fmt.Println(append([]any{"[WARN]"}, args...)...)
}

func (x *Task) LogErrorf(format string, args ...any) {
	fmt.Printf("[ERROR] "+format+"\n", args...)
}

func (x *Task) LogError(args ...any) {
	fmt.Println(append([]any{"[ERROR]"}, args...)...)
}

func (x *Task) LogFatalf(format string, args ...any) {
	fmt.Printf("[FATAL] "+format+"\n", args...)
	debug.PrintStack()
}

func (x *Task) LogFatal(args ...any) {
	fmt.Println(append([]any{"[FATAL]"}, args...)...)
	debug.PrintStack()
}
