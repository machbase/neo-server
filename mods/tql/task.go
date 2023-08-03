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
	functions    map[string]expression.Function
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
	mapExprs   []string

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

func NewTaskContext(ctx context.Context) *Task {
	ret := NewTask()
	ret.ctx = ctx
	return ret
}

func (x *Task) Function(name string) expression.Function {
	return x.functions[name]
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
	x.compiled = true
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
		srcLine := exprs[0]
		src, err := x.compileSource(srcLine.text)
		if err != nil {
			x.compileErr = errors.Wrapf(err, "at line %d", srcLine.line)
			return x.compileErr
		}
		x.input = src
	}

	// sink
	if len(exprs) >= 2 {
		sinkLine := exprs[len(exprs)-1]
		// validates the syntax
		sink, err := x.compileSink(sinkLine.text)
		if err != nil {
			x.compileErr = errors.Wrapf(err, "at line %d", sinkLine.line)
			return x.compileErr
		}
		x.output = sink
	}

	// map
	if len(exprs) >= 3 {
		exprs = exprs[1 : len(exprs)-1]
		for _, mapLine := range exprs {
			// validates the syntax
			_, err := x.Parse(mapLine.text)
			if err != nil {
				x.compileErr = errors.Wrapf(err, "at line %d", mapLine.line)
				return x.compileErr
			}
			x.mapExprs = append(x.mapExprs, mapLine.text)
		}
	}
	return nil
}

var mapFunctionsMacro = [][2]string{
	{"SCRIPT(", "SCRIPT(CTX,"},
	{"TAKE(", "TAKE(CTX,"},
	{"DROP(", "DROP(CTX,"},
	{"PUSHKEY(", "PUSHKEY(CTX,"},
	{"POPKEY(", "POPKEY(CTX,"},
	{"GROUPBYKEY(", "GROUPBYKEY(CTX,"},
	{"FLATTEN(", "FLATTEN(CTX,"},
	{"FILTER(", "FILTER(CTX,"},
	{"FFT(", "FFT(CTX,"},
}

func (x *Task) Parse(text string) (*expression.Expression, error) {
	for _, f := range mapFunctionsMacro {
		text = strings.ReplaceAll(text, f[0], f[1])
	}
	text = strings.ReplaceAll(text, "(CTX,)", "(CTX)")
	return expression.NewWithFunctions(text, x.functions)
}

func (x *Task) parseSource(text string) (*expression.Expression, error) {
	return expression.NewWithFunctions(text, x.functions)
}

func (x *Task) parseSink(text string) (*expression.Expression, error) {
	return expression.NewWithFunctions(text, x.functions)
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
	exprs := []*expression.Expression{}
	for _, str := range x.mapExprs {
		expr, err := x.Parse(str)
		if err != nil {
			return errors.Wrapf(err, "at %s", str)
		}
		if expr == nil {
			return fmt.Errorf("compile error at %s", str)
		}
		exprs = append(exprs, expr)
	}

	x.resultCh = make(chan any)
	x.encoderCh = make(chan []any)
	x.db = db

	x.nodes = make([]*Node, len(exprs))
	for n, expr := range exprs {
		x.nodes[n] = &Node{
			id:   n,
			Name: expr.String(),
			Expr: expr,
			Src:  make(chan *Record),
			Sink: x.resultCh,
			Next: nil,
			task: x,
		}
		if n > 0 {
			x.nodes[n-1].Next = x.nodes[n]
		}
	}
	if len(x.nodes) > 0 {
		x.headNode = x.nodes[0]
	}
	x.start()
	x.wait()
	x.stop()
	return x.lastError
}

func (x *Task) AddNode(node *Node) {
	node.id = len(x.nodes)
	x.nodes = append(x.nodes, node)
}

func (x *Task) compileSource(code string) (*input, error) {
	expr, err := x.parseSource(code)
	if err != nil {
		return nil, err
	}
	src, err := expr.Eval(x)
	if err != nil {
		return nil, err
	}
	var ret *input
	switch src := src.(type) {
	case DatabaseSource:
		ret = &input{dbSrc: src}
	case ChannelSource:
		ret = &input{chSrc: src}
	default:
		return nil, fmt.Errorf("%T is not applicable for INPUT", src)
	}
	return ret, nil
}

func (x *Task) start() {
	////////////////////////////////
	// encoder
	x.encoderChWg.Add(1)
	var cols spi.Columns
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
				if len(cols) == 0 {
					for i, v := range arr {
						cols = append(cols, &spi.Column{
							Name: fmt.Sprintf("C%02d", i),
							Type: x.columnTypeName(v)})
					}
				}
				x.output.SetHeader(cols)
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
		child.Start()
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
	deligate := &InputDelegateWrapper{
		DatabaseFunc: func() spi.Database {
			return x.db
		},
		ShouldStopFunc: func() bool {
			return x.circuitBreaker || x.lastError != nil
		},
		FeedHeaderFunc: func(c spi.Columns) {
			cols = c
		},
		FeedFunc: func(values []any) {
			if x.headNode != nil {
				if values != nil {
					x.headNode.Src <- x.headNode.NewRecord(values[0], values[1:])
				} else {
					x.headNode.Src <- EofRecord
				}
			} else {
				// there is no chain, just forward input data to sink directly
				x.sendToEncoder(values)
			}
		},
	}
	if err := x.input.run(deligate); err != nil {
		x.lastError = err
	}
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
