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
	"sync"
	"time"

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
	outputWriter io.Writer
	outputStream spec.OutputStream
	toJsonOutput bool

	// comments start with plus(+) symbold and sperated by comma.
	// ex) => `// +brief, markdown`
	pragma []string

	// compiled result
	compiled   bool
	compileErr error
	input      *input
	output     *output

	// result data
	resultColumns spi.Columns

	// runtime
	db       spi.Database
	nodes    []*Node
	headNode *Node
	nodesWg  sync.WaitGroup

	resultCh           chan any
	encoderCh          chan []any
	encoderChWg        sync.WaitGroup
	encoderNeedToClose bool

	closeOnce      sync.Once
	lastError      error
	circuitBreaker bool
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
	x.outputWriter = w
	if w == nil {
		x.outputStream, err = stream.NewOutputStream("-")
		if err != nil {
			return err
		}
	} else {
		x.outputStream = &stream.WriterOutputStream{Writer: w}
	}
	return nil
}

func (x *Task) OutputWriter() io.Writer {
	return x.outputWriter
}

func (x *Task) SetOutputStream(o spec.OutputStream) {
	x.outputStream = o
	x.outputWriter = o
}

func (x *Task) OutputStream() spec.OutputStream {
	return x.outputStream
}

func (x *Task) SetJsonOutput(flag bool) {
	x.toJsonOutput = flag
}

func (x *Task) ShouldJsonOutput() bool {
	return x.toJsonOutput
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
			return x.outputStream, nil
		case "nil":
			return nil, nil
		}
	}
}

func (x *Task) AddPragma(p string) {
	x.pragma = append(x.pragma, p)
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
					x.AddPragma(strings.TrimSpace(t))
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
	}

	x.resultCh = make(chan any)
	x.encoderCh = make(chan []any)

	// map
	if len(exprs) >= 3 {
		exprs = exprs[1 : len(exprs)-1]
		for n, mapLine := range exprs {
			node := NewNode(x)
			expr, err := node.Parse(mapLine.text)
			if err != nil {
				return errors.Wrapf(err, "at %s", mapLine.text)
			}
			if expr == nil {
				return fmt.Errorf("compile error at %s", mapLine.text)
			}
			x.nodes = append(x.nodes, node)
			node.id = n
			node.Name = expr.String()
			node.Expr = expr
			node.Src = make(chan *Record)
			node.Sink = x.resultCh
			if n > 0 {
				x.nodes[n-1].Next = x.nodes[n]
			}
		}
	}
	if len(x.nodes) > 0 {
		x.headNode = x.nodes[0]
		x.input.next = x.nodes[0]
	} else {
		if x.input != nil && x.output != nil {
			x.input.next = x.output.selfNode
		}
	}
	x.compiled = true
	return nil
}

// DumpSQL returns the generated SQL statement if the input source database source
func (x *Task) DumpSQL() string {
	if x.input == nil || x.input.dbSrc == nil {
		return ""
	}
	return x.input.dbSrc.ToSQL()
}

func (x *Task) LogDebug(msg string, args ...any) {
	if len(args) > 0 {
		fmt.Printf("[DEBUG] "+msg+"\n", args...)
	} else {
		fmt.Println("[DEBUG]", msg)
	}
}

func (x *Task) LogDebugString(args ...string) {
	fmt.Println("[DEBUG]", strings.Join(args, " "))
}

func (x *Task) LogInfo(msg string, args ...any) {
	if len(args) > 0 {
		fmt.Printf("[INFO] "+msg+"\n", args...)
	} else {
		fmt.Println("[INFO]", msg)
	}
}

func (x *Task) LogError(msg string, args ...any) {
	if len(args) > 0 {
		fmt.Printf("[ERROR] "+msg+"\n", args...)
	} else {
		fmt.Println("[ERROR]", msg)
	}
	debug.PrintStack()
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
	x.db = db

	x.start()
	x.wait()
	x.stop()
	return x.lastError
}

func (x *Task) AddNode(node *Node) {
	node.id = len(x.nodes)
	x.nodes = append(x.nodes, node)
}

func (x *Task) start() {
	////////////////////////////////
	// encoder
	x.encoderChWg.Add(1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				if err, ok := r.(error); ok {
					x.LogError(err.Error())
				}
			}
			x.encoderChWg.Done()
		}()
		for arr := range x.encoderCh {
			if !x.encoderNeedToClose {
				if len(x.resultColumns) == 0 {
					for i, v := range arr {
						x.resultColumns = append(x.resultColumns, &spi.Column{
							Name: fmt.Sprintf("C%02d", i),
							Type: x.columnTypeName(v)})
					}
				}
				x.output.SetHeader(x.resultColumns)
				x.output.Open(x.db)
				x.encoderNeedToClose = true
			}
			if len(arr) == 0 {
				continue
			}
			if rec, ok := arr[0].(*Record); ok && rec.IsEOF() {
				continue
			}
			if err := x.output.AddRow(arr); err != nil {
				x.LogError(err.Error())
			}
		}
	}()

	////////////////////////////////
	// nodes
	for _, child := range x.nodes {
		x.nodesWg.Add(1)
		child.start()
	}

	sink0 := func(k any, v any) {
		x.sendToEncoder([]any{k, v})
	}

	sink1 := func(k any, v []any) {
		x.sendToEncoder(append([]any{k}, v...))
	}

	sink2 := func(k any, v [][]any) {
		for _, row := range v {
			sink1(k, row)
		}
	}

	go func() {
		for ret := range x.resultCh {
			switch castV := ret.(type) {
			case *Record:
				if castV.IsEOF() {
					x.nodesWg.Done()
				} else if castV.IsCircuitBreak() {
					x.circuitBreaker = true
				} else {
					switch tV := castV.value.(type) {
					case []any:
						if len(tV) == 0 {
							sink1(castV.key, tV)
						} else {
							if subarr, ok := tV[0].([][]any); ok {
								sink2(castV.key, subarr)
							} else {
								sink1(castV.key, tV)
							}
						}
					case [][]any:
						sink2(castV.key, tV)
					default:
						sink0(castV.key, castV.value)
					}
				}
			case []*Record:
				for _, v := range castV {
					switch tV := v.value.(type) {
					case []any:
						sink1(v.key, tV)
					case [][]any:
						sink2(v.key, tV)
					default:
						sink0(v.key, tV)
					}
				}
			case error:
				x.lastError = castV
			}
		}
	}()

	////////////////////////////////
	// input source
	if err := x.input.run(); err != nil {
		x.lastError = err
	}
}

func (x *Task) feedNodes(values []any) {
	if x.headNode != nil {
		if values != nil {
			x.headNode.Src <- NewRecord(values[0], values[1:])
		} else {
			x.headNode.Src <- EofRecord
		}
	} else {
		// there is no chain, just forward input data to sink directly
		x.sendToEncoder(values)
	}
}

func (x *Task) shouldStopNodes() bool {
	return x.circuitBreaker || x.lastError != nil
}

func (x *Task) stop() {
	x.nodesWg.Wait()
}

func (x *Task) wait() {
	x.closeOnce.Do(func() {
		for _, ctx := range x.nodes {
			ctx.Stop()
		}
		close(x.resultCh)
		close(x.encoderCh)
		x.encoderChWg.Wait()
		if x.output != nil && x.encoderNeedToClose {
			x.output.Close()
		}
	})
}

func (x *Task) sendToEncoder(values []any) {
	if len(values) > 0 {
		if t, ok := values[0].(*time.Time); ok {
			values[0] = *t
		}
		x.encoderCh <- values
	}
}

func (x *Task) columnTypeName(v any) string {
	switch v.(type) {
	default:
		return fmt.Sprintf("%T", v)
	case string:
		return "string"
	case *time.Time:
		return "datetime"
	case time.Time:
		return "datetime"
	case *float32:
		return "float"
	case float32:
		return "float"
	case *float64:
		return "double"
	case float64:
		return "double"
	}
}
