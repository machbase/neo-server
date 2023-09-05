package tql

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"regexp"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/machbase/neo-server/mods/expression"
	"github.com/machbase/neo-server/mods/service/eventbus"
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
	consoleUser  string
	consoleId    string
	consoleTopic string

	// comments start with plus(+) symbold and sperated by comma.
	// ex) => `// +brief, markdown`
	pragma []string

	logLevel        Level
	consoleLogLevel Level

	// compiled result
	compiled   bool
	compileErr error
	output     *output
	nodes      []*Node

	_shouldStop    bool
	_resultColumns spi.Columns
	_stateLock     sync.RWMutex
	_created       time.Time
}

func NewTask() *Task {
	return NewTaskContext(context.Background())
}

func NewTaskContext(ctx context.Context) *Task {
	ret := &Task{_created: time.Now()}
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

func (x *Task) SetConsole(user string, id string) {
	x.consoleUser = user
	x.consoleId = id
	if user != "" && id != "" {
		x.consoleTopic = fmt.Sprintf("console:%s:%s", user, id)
	}
}

func (x *Task) SetConsoleLogLevel(level Level) {
	x.consoleLogLevel = level
}

func (x *Task) SetLogLevel(level Level) {
	x.consoleLogLevel = level
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

func (x *Task) GetVariable(name string) (any, error) {
	if strings.HasPrefix(name, "$") {
		if p, ok := x.params[strings.TrimPrefix(name, "$")]; ok {
			x.LogWarnf("'$' expression is deprecated, use param(\"%s\") instead", name)
			if len(p) > 0 {
				return p[len(p)-1], nil
			}
		}
		return nil, nil
	} else {
		return nil, fmt.Errorf("undefined variable '%s'", name)
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
	} else {
		nodeNames := []string{}
		for _, n := range x.nodes {
			nodeNames = append(nodeNames, n.Name())
		}
		nodeNames = append(nodeNames, x.output.Name())
		x.LogTracef("%p Task compiled %s", x, strings.Join(nodeNames, " → "))
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
		x.LogErrorf("%p Task %s", x, err.Error())
	} else {
		x.LogDebugf("%p Task elapsed %s", x, time.Since(x._created).String())
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
	types := make([]*spi.Column, len(cols))
	for i, c := range cols {
		x := *c
		switch x.Type {
		case "sql.RawBytes":
			x.Type = spi.ColumnBufferTypeBinary
		case "sql.NullBool":
			x.Type = spi.ColumnBufferTypeBoolean
		case "sql.NullByte":
			x.Type = spi.ColumnBufferTypeByte
		case "sql.NullFloat64":
			x.Type = spi.ColumnBufferTypeDouble
		case "sql.NullInt16":
			x.Type = spi.ColumnBufferTypeInt16
		case "sql.NullInt32":
			x.Type = spi.ColumnBufferTypeInt32
		case "sql.NullInt64":
			x.Type = spi.ColumnBufferTypeInt64
		case "sql.NullString":
			x.Type = spi.ColumnBufferTypeString
		case "sql.NullTime":
			x.Type = spi.ColumnBufferTypeDatetime
		}
		types[i] = &x
	}
	x._resultColumns = types
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

func asNodeName(expr *expression.Expression) string {
	if toks := expr.Tokens(); len(toks) > 0 && toks[0].Kind == expression.FUNCTION {
		r := regexp.MustCompile(`([a-zA-Z][a-zA-Z0-9_]+).+`)
		subs := r.FindStringSubmatch(expr.String())
		if len(subs) >= 2 {
			return subs[1] + "()"
		}
	}
	return expr.String()
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

func (x *Task) Logf(format string, args ...any)      { x._logf(INFO, format, args...) }
func (x *Task) LogInfof(format string, args ...any)  { x._logf(INFO, format, args...) }
func (x *Task) LogTracef(format string, args ...any) { x._logf(TRACE, format, args...) }
func (x *Task) LogDebugf(format string, args ...any) { x._logf(DEBUG, format, args...) }
func (x *Task) LogWarnf(format string, args ...any)  { x._logf(WARN, format, args...) }
func (x *Task) LogErrorf(format string, args ...any) { x._logf(ERROR, format, args...) }

func (x *Task) Log(args ...any)      { x._log(INFO, args...) }
func (x *Task) LogInfo(args ...any)  { x._log(INFO, args...) }
func (x *Task) LogTrace(args ...any) { x._log(TRACE, args...) }
func (x *Task) LogDebug(args ...any) { x._log(DEBUG, args...) }
func (x *Task) LogWarn(args ...any)  { x._log(WARN, args...) }
func (x *Task) LogError(args ...any) { x._log(ERROR, args...) }

func (x *Task) LogFatalf(format string, args ...any) {
	stack := string(debug.Stack())
	x._logf(FATAL, format+"\n%s", append(args, stack))
}

func (x *Task) LogFatal(args ...any) {
	stack := string(debug.Stack())
	x._log(FATAL, append(args, "\n", stack))
}

func (x *Task) _log(level Level, args ...any) {
	if x.logWriter != nil && level >= x.logLevel {
		line := fmt.Sprintln(append([]any{"[" + Levels[level] + "]"}, args...)...)
		x.logWriter.Write([]byte(line))
	}
	if x.consoleTopic != "" && level >= x.consoleLogLevel {
		toks := []string{}
		for _, arg := range args {
			toks = append(toks, fmt.Sprintf("%v", arg))
		}
		eventbus.PublishLog(x.consoleTopic, Levels[level], strings.Join(toks, " "))
	}
}

func (x *Task) _logf(level Level, format string, args ...any) {
	if x.logWriter != nil && level >= x.logLevel {
		line := fmt.Sprintf("[%s] "+format+"\n", append([]any{Levels[level]}, args...)...)
		x.logWriter.Write([]byte(line))
	}
	if x.consoleTopic != "" && level >= x.consoleLogLevel {
		eventbus.PublishLog(x.consoleTopic, Levels[level], fmt.Sprintf(format, args...))
	}
}

var Levels = []string{"TRACE", "DEBUG", "INFO", "WARN", "ERROR", "FATAL"}

type Level int

const (
	TRACE Level = iota
	DEBUG
	INFO
	WARN
	ERROR
	FATAL
)

func ParseLogLevel(str string) Level {
	s := strings.ToUpper(str)
	for i := range Levels {
		if s == Levels[i] {
			return Level(i)
		}
	}
	return FATAL
}
