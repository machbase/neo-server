package tql

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"regexp"
	"runtime/debug"
	"strings"
	"sync"
	"time"
	"unicode"

	client "github.com/machbase/neo-client/v2"
	"github.com/machbase/neo-client/v2/api"
	"github.com/machbase/neo-server/v8/mods/codec/facility"
	"github.com/machbase/neo-server/v8/mods/eventbus"
	"github.com/machbase/neo-server/v8/mods/logging"
	"github.com/machbase/neo-server/v8/mods/tql/expression"
	"github.com/machbase/neo-server/v8/mods/util"
)

const (
	PRAGMA_LOG_LEVEL = "log-level"
)

type Task struct {
	ctx          context.Context
	ctxCancel    context.CancelFunc
	params       map[string][]string
	inputReader  io.Reader
	outputWriter io.Writer
	toJsonOutput bool
	logWriter    io.Writer
	consoleUser  string
	consoleId    string
	consoleTopic string
	consoleOtp   string

	// log level for io.Writer
	// default is ERROR
	// use `#pragma log-level: INFO` in the tql script to change the log level
	logLevel        Level
	consoleLogLevel Level

	argValues []any

	httpClientFactory func() *http.Client

	volatileAssetsProvider VolatileAssetsProvider

	// compiled result
	sourcePath string
	sourceHash string
	compiled   bool
	compileErr error
	output     *output
	nodes      []*Node

	// preemptive cache update
	_preemptiveCacheUpdateStarted bool
	_preemptiveCacheUpdateTimeout time.Duration

	_shouldStop          bool
	_shouldStopListeners []func()

	_resultColumns client.Columns
	_stateLock     sync.RWMutex
	_created       time.Time
}

var (
	_ facility.Logger             = &Task{}
	_ facility.VolatileFileWriter = &Task{}
)

func NewTask() *Task {
	return NewTaskContext(context.Background())
}

func NewTaskContext(ctx context.Context) *Task {
	ret := &Task{
		logLevel: ERROR,
		_created: time.Now(),
	}
	ret.volatileAssetsProvider = instance.vap
	ret.ctx, ret.ctxCancel = context.WithCancel(ctx)
	context.AfterFunc(ret.ctx, func() {
		ret.fireCircuitBreak(nil)
	})
	return ret
}

func (x *Task) NewHttpClient() *http.Client {
	if x.httpClientFactory != nil {
		return x.httpClientFactory()
	}
	return &http.Client{}
}

func (x *Task) SetHttpClientFactory(factory func() *http.Client) {
	x.httpClientFactory = factory
}

func (x *Task) SetInputReader(r io.Reader) {
	x.inputReader = r
}

func (x *Task) InputReader() io.Reader {
	return x.inputReader
}

func (x *Task) SetOutputWriter(w io.Writer) {
	if w == nil {
		x.outputWriter = &util.NopCloseWriter{Writer: os.Stdout}
	} else {
		x.outputWriter = w
	}
}

func (x *Task) SetOutputWriterJson(w io.Writer, json bool) {
	x.SetOutputWriter(w)
	x.toJsonOutput = json
}

func (x *Task) OutputWriter() io.Writer {
	if x.outputWriter == nil {
		x.outputWriter = &util.NopCloseWriter{Writer: os.Stdout}
	}
	return x.outputWriter
}

func (x *Task) SetLogWriter(w io.Writer) {
	x.logWriter = w
}

func (x *Task) SetConsole(user string, id string, otp string) {
	x.consoleUser = user
	x.consoleId = id
	if user != "" && id != "" {
		x.consoleTopic = fmt.Sprintf("console:%s:%s", user, id)
	}
	x.consoleOtp = otp
}

func (x *Task) SetConsoleLogLevel(level Level) {
	x.consoleLogLevel = level
}

func (x *Task) SetLogLevel(level Level) {
	x.logLevel = level
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
	x.volatileAssetsProvider = sc.vap
	x.sourcePath = sc.path0
	return x.Compile(bytes.NewBuffer(sc.content))
}

func (x *Task) CompileString(code string) error {
	return x.Compile(bytes.NewBufferString(code))
}

func (x *Task) Compile(codeReader io.Reader) error {
	code, err := io.ReadAll(codeReader)
	if err != nil {
		return err
	}
	h := sha1.New()
	h.Write(code)
	x.sourceHash = fmt.Sprintf("%x", h.Sum(nil))

	err = x.compile(bytes.NewBuffer(code))
	if err != nil {
		x.LogError("Compile", err.Error())
	} else {
		nodeNames := []string{}
		for _, n := range x.nodes {
			nodeNames = append(nodeNames, n.Name())
		}
		if x.output != nil {
			nodeNames = append(nodeNames, x.output.Name())
		}
		x.LogDebug("Task compiled", strings.Join(nodeNames, " → "))
	}
	return err
}

func (x *Task) compile(codeReader io.Reader) error {
	script, err := ParseScriptReader(codeReader, functions)
	if err != nil {
		x.compileErr = err
		return err
	}
	if len(script.Statements) == 0 {
		x.compileErr = errors.New("empty expressions")
		return x.compileErr
	}
	if err := ValidateScriptStructure(script); err != nil {
		x.compileErr = err
		return err
	}

	nodeIdx := 0
	var pragmas map[string]string
	var codes []*Statement
	for _, stmt := range script.Statements {
		if stmt.IsCode() {
			codes = append(codes, stmt)
		}
	}
	tailStmt := codes[len(codes)-1]

	for _, stmt := range script.Statements {
		curLine := stmt.toLine()
		if stmt.IsPragma {
			kvs := util.ParseNameValuePairs(stmt.Text)
			for _, kv := range kvs {
				switch kv.Name {
				case PRAGMA_LOG_LEVEL:
					x.SetLogLevel(ParseLogLevel(kv.Value))
					continue
				default:
					if pragmas == nil {
						pragmas = map[string]string{}
					}
					pragmas[kv.Name] = kv.Value
				}
			}
			continue
		}
		if stmt.IsComment {
			continue
		}
		if stmt == tailStmt {
			// sink
			x.output, err = NewNode(x).compileSink(curLine)
			if err != nil {
				x.compileErr = wrapCompileStatementError("sink_compile_error", stmt, curLine, err)
				return x.compileErr
			}
			x.output.pragma = pragmas
			if nodeIdx > 0 {
				x.nodes[nodeIdx-1].next = x.output
			}
		} else {
			// src and map
			node := NewNode(x)
			if err := node.compile(stmt.Text); err != nil {
				kind := "map_compile_error"
				if nodeIdx == 0 {
					kind = "source_compile_error"
				}
				x.compileErr = wrapCompileStatementError(kind, stmt, curLine, err)
				return x.compileErr
			}
			node.pragma = pragmas
			node.tqlLine = curLine
			x.nodes = append(x.nodes, node)
			if nodeIdx > 0 {
				x.nodes[nodeIdx-1].next = x.nodes[nodeIdx]
			}
			nodeIdx++
		}
		pragmas = nil
	}

	if x.output == nil {
		x.compileErr = errors.New("no sink exists")
		return x.compileErr
	}
	x.compiled = true
	return nil
}

func wrapCompileStatementError(kind string, stmt *Statement, line *Line, cause error) error {
	if cause == nil {
		return nil
	}
	lineNo := 0
	columnNo := 0
	if line != nil {
		lineNo = line.line
	}
	if stmt != nil {
		if lineNo == 0 {
			lineNo = stmt.Line
		}
		columnNo = stmt.Span.Start.Column
	}
	_ = lineNo
	_ = columnNo
	return newScriptError(kind, stmt, cause.Error(), cause)
}

type Result struct {
	Err      error
	Message  string
	IsDbSink bool
	_created time.Time
}

type ResultModel struct {
	Success bool             `json:"success"`
	Reason  string           `json:"reason"`
	Elapse  string           `json:"elapse"`
	Data    *ResultDataModel `json:"data,omitempty"`
}

type ResultDataModel struct {
	Message string `json:"message,omitempty"`
}

func (rs *Result) MarshalJSON() ([]byte, error) {
	m := &ResultModel{
		Success: rs.Err == nil,
		Reason:  "undefined",
		Elapse:  time.Since(rs._created).String(),
	}
	if rs.Err != nil {
		m.Reason = rs.Err.Error()
	} else {
		m.Reason = "success"
	}
	if rs.Message != "" {
		m.Data = &ResultDataModel{
			Message: rs.Message,
		}
	}
	return json.Marshal(&m)
}

func (x *Task) Execute() *Result {
	result := x.execute()
	if result.Err != nil {
		x.LogError("Task", result.Err.Error())
	} else {
		x.LogDebug("Task elapsed", time.Since(x._created).String())
	}
	return result
}

func (x *Task) execute() *Result {
	if !x.compiled {
		return &Result{Err: errors.New("not compiled task"), _created: x._created}
	}
	defer func() {
		if r := recover(); r != nil {
			w := &bytes.Buffer{}
			w.Write(debug.Stack())
			x.LogErrorf("panic %v\n%s", r, w.String())
		}
	}()

	if x.output.cachedData != nil {
		// send cached data to client first
		x.outputWriter.Write(x.output.cachedData)

		// Do preemptive update in background
		// if the cachedData and cacheWriter are set => preemptive update
		if x.output.cacheWriter != nil {
			var cancel context.CancelFunc
			x._preemptiveCacheUpdateTimeout = 10 * time.Second
			x.ctx, cancel = context.WithTimeout(context.Background(), x._preemptiveCacheUpdateTimeout)
			context.AfterFunc(x.ctx, func() {
				x.fireCircuitBreak(nil)
			})
			x._preemptiveCacheUpdateStarted = true
			go func() {
				defer cancel()
				x.executeOutput()
			}()
		}
		return &Result{
			Err:      nil,
			Message:  "cached",
			IsDbSink: x.output.dbSink != nil,
			_created: x._created,
		}
	}

	x.executeOutput()

	if x.output != nil {
		return &Result{
			Err:      x.output.lastError,
			Message:  x.output.lastMessage,
			IsDbSink: x.output.dbSink != nil,
			_created: x._created,
		}
	}
	return &Result{
		Err:      errors.New("no sink exists"),
		_created: x._created,
	}
}

func (x *Task) executeOutput() {
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
}

func (x *Task) Cancel() {
	// do not cancel if the task is in preemptive cache update mode
	if !x._preemptiveCacheUpdateStarted {
		x.fireCircuitBreak(nil)
	}
}

func (x *Task) AddShouldStopListener(fn func()) {
	if fn == nil {
		return
	}

	x._stateLock.Lock()
	alreadyStopped := x._shouldStop
	if !alreadyStopped {
		x._shouldStopListeners = append(x._shouldStopListeners, fn)
	}
	x._stateLock.Unlock()

	if alreadyStopped {
		fn()
	}
}

func (x *Task) fireCircuitBreak(_ *Node) {
	x._stateLock.Lock()
	if x._shouldStop {
		x._stateLock.Unlock()
		return
	}
	x._shouldStop = true
	listeners := append([]func(){}, x._shouldStopListeners...)
	x._stateLock.Unlock()

	for _, fn := range listeners {
		fn()
	}
}

func (x *Task) shouldStop() bool {
	x._stateLock.RLock()
	ret := x._shouldStop
	x._stateLock.RUnlock()
	return ret
}

func (x *Task) SetResultColumns(cols client.Columns) {
	x._stateLock.Lock()
	ts := make([]*client.Column, len(cols))
	for i, c := range cols {
		x := *c
		switch x.DataType {
		case "sql.RawBytes":
			x.DataType = api.DataTypeBinary
		case "sql.NullBool":
			x.DataType = api.DataTypeBoolean
		case "sql.NullByte":
			x.DataType = api.DataTypeByte
		case "sql.NullFloat64":
			x.DataType = api.DataTypeFloat64
		case "sql.NullInt16":
			x.DataType = api.DataTypeInt16
		case "sql.NullInt32":
			x.DataType = api.DataTypeInt32
		case "sql.NullInt64":
			x.DataType = api.DataTypeInt64
		case "sql.NullString":
			x.DataType = api.DataTypeString
		case "sql.NullTime":
			x.DataType = api.DataTypeDatetime
		}
		ts[i] = &x
	}
	x._resultColumns = ts
	x._stateLock.Unlock()
}

func (x *Task) ResultColumns() client.Columns {
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

func (x *Task) OutputHttpHeaders() map[string][]string {
	if x.output != nil {

		return x.output.HttpHeaders()
	}
	return nil
}

func (x *Task) OutputChartType() string {
	if x.output != nil {
		if x.output.IsChart() {
			return "echarts"
		} else if x.output.IsGeoMap() {
			return "geomap"
		}
	}
	return ""
}

var asNodeNameRegex = regexp.MustCompile(`([a-zA-Z][a-zA-Z0-9_]+).+`)

func asNodeName(expr *expression.Expression) string {
	if toks := expr.Tokens(); len(toks) > 0 && toks[0].Kind == expression.FUNCTION {
		subs := asNodeNameRegex.FindStringSubmatch(expr.String())
		if len(subs) >= 2 {
			return subs[1] + "()"
		}
	}
	return expr.String()
}

func (task *Task) SetVolatileAssetsProvider(p VolatileAssetsProvider) {
	task.volatileAssetsProvider = p
}

func (task *Task) VolatileFilePrefix() string {
	return task.volatileAssetsProvider.VolatileFilePrefix()
}

func (task *Task) VolatileFileWrite(name string, data []byte, deadline time.Time) {
	if task.volatileAssetsProvider == nil {
		return
	}
	task.volatileAssetsProvider.VolatileFileWrite(name, data, deadline)
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

func (x *Task) _log(level Level, args ...any) {
	if x.logWriter != nil && level >= x.logLevel {
		if lw, ok := x.logWriter.(logging.Log); ok {
			if lvl := level.LoggingLevel(); lvl >= lw.Level() {
				lw.Log(lvl, strings.TrimRightFunc(fmt.Sprintln(args...), unicode.IsSpace))
			}
		} else {
			line := fmt.Sprintln(append([]any{"[" + Levels[level] + "]"}, args...)...)
			x.logWriter.Write([]byte(line))
		}
	}
	if x.consoleTopic != "" && level >= x.consoleLogLevel {
		toks := []string{}
		for _, arg := range args {
			toks = append(toks, fmt.Sprintf("%v", arg))
		}
		eventbus.PublishLogTask(x.consoleTopic, Levels[level], fmt.Sprintf("%p", x), strings.Join(toks, " "))
	}
}

func (x *Task) _logf(level Level, format string, args ...any) {
	if x.logWriter != nil && level >= x.logLevel {
		if lw, ok := x.logWriter.(logging.Log); ok {
			if lvl := level.LoggingLevel(); lvl >= lw.Level() {
				lw.Logf(lvl, format, args...)
			}
		} else {
			line := fmt.Sprintf("[%s] "+format+"\n", append([]any{Levels[level]}, args...)...)
			x.logWriter.Write([]byte(line))
		}
	}
	if x.consoleTopic != "" && level >= x.consoleLogLevel {
		eventbus.PublishLogTask(x.consoleTopic, Levels[level], fmt.Sprintf("%p", x), fmt.Sprintf(format, args...))
	}
}

var _ io.Writer = (*Task)(nil)

func (x *Task) Write(p []byte) (n int, err error) {
	x._log(INFO, string(p))
	return len(p), nil
}

var Levels = []string{"TRACE", "DEBUG", "INFO", "WARN", "ERROR"}

type Level int

const (
	TRACE Level = iota
	DEBUG
	INFO
	WARN
	ERROR
)

func (l Level) LoggingLevel() logging.Level {
	switch l {
	default:
		return logging.LevelInfo
	case TRACE:
		return logging.LevelTrace
	case DEBUG:
		return logging.LevelDebug
	case WARN:
		return logging.LevelWarn
	case ERROR:
		return logging.LevelError
	}
}

func ParseLogLevel(str string) Level {
	s := strings.ToUpper(str)
	for i := range Levels {
		if s == Levels[i] {
			return Level(i)
		}
	}
	return ERROR
}

type Closer interface {
	Close() error
}

type Node struct {
	task *Task
	name string
	next Receiver

	src  chan *Record
	expr *expression.Expression
	nrow int

	functions map[string]expression.Function
	values    map[string]any
	debug     bool

	closeWg sync.WaitGroup
	closers []Closer
	mutex   sync.Mutex

	_inflight *Record

	eofCallback func(*Node)

	pragma  map[string]string
	tqlLine *Line
}

var _ expression.Parameters = (*Node)(nil)

func (node *Node) compile(code string) error {
	expr, err := node.Parse(code)
	if err != nil {
		return fmt.Errorf("%s at %s", err.Error(), code)
	}
	if expr == nil {
		return fmt.Errorf("compile error at %s", code)
	}
	node.name = asNodeName(expr)
	node.expr = expr
	node.src = make(chan *Record)
	return nil
}

func (node *Node) Parse(text string) (*expression.Expression, error) {
	return expression.NewWithFunctions(text, node.functions)
}

func (node *Node) SetInflight(rec *Record) {
	node._inflight = rec
}

func (node *Node) Function(name string) expression.Function {
	return node.functions[name]
}

func (node *Node) Name() string {
	return node.name
}

func (node *Node) Inflight() *Record {
	return node._inflight
}

func (node *Node) Rownum() int {
	return node.nrow
}

func (node *Node) Receive(rec *Record) {
	select {
	case node.src <- rec:
	case <-node.task.ctx.Done():
		node.task.Cancel()
	}
}

func (node *Node) SetEOF(f func(*Node)) {
	node.eofCallback = f
}

func (node *Node) Pragma(name string) (string, bool) {
	if node.pragma != nil {
		if v, ok := node.pragma[name]; ok {
			return v, true
		}
	}
	return "", false
}

func (node *Node) PragmaBool(name string) bool {
	if node.pragma != nil {
		if v, ok := node.pragma[name]; ok {
			if v == "" || v == "1" || strings.ToLower(v) == "true" {
				return true
			}
		}
	}
	return false
}

// Get implements expression.Parameters
func (node *Node) Get(name string) (any, error) {
	switch name {
	case "PI":
		return math.Pi, nil
	case "nil", "NULL":
		return expression.NullValue, nil
	default:
		inflight := node.Inflight()
		if inflight == nil {
			return nil, nil
		}
		if node.Name() == "SET()" && !strings.HasPrefix(name, "$") {
			return func(v any) {
				inflight.SetVariable(name, v)
			}, nil
		} else {
			return inflight.GetVariable(name)
		}
	}
}

func (node *Node) fmSET(left any, right any) (any, error) {
	if left == nil {
		return node.Inflight(), nil
	}
	if fn, ok := left.(func(any)); ok {
		fn(right)
	} else {
		return nil, fmt.Errorf("%q left operand is not valid", "LET")
	}
	return node.Inflight(), nil
}

func (node *Node) GetValue(name string) (any, bool) {
	if node.values == nil {
		return nil, false
	}
	ret, ok := node.values[name]
	return ret, ok
}

func (node *Node) SetValue(name string, value any) {
	if node.values == nil {
		node.values = make(map[string]any)
	}
	node.values[name] = value
}

func (node *Node) DeleteValue(name string) {
	if node.values != nil {
		delete(node.values, name)
	}
}

func (node *Node) yield(key any, values []any) {
	var yieldRec *Record
	if len(values) == 0 {
		yieldRec = NewRecord(key, []any{})
	} else if len(values) == 1 {
		yieldRec = NewRecord(key, values[0])
	} else {
		yieldRec = NewRecord(key, values)
	}
	if node.debug {
		node.task.LogDebug("++", node.name, "-->", node.next.Name(), yieldRec.String(), " ")
	}
	yieldRec.Tell(node.next)
}

func (node *Node) start() {
	node.closeWg.Add(1)
	go func() {
		defer func() {
			node.closeWg.Done()
			if o := recover(); o != nil {
				w := &bytes.Buffer{}
				w.Write(debug.Stack())
				node.task.Log("panic", node.name, o, w.String())
				node.task.LogErrorf("panic %s %v\n%s", node.name, o, w.String())
			}
		}()
		var lastWill *Record
	loop:
		for {
			select {
			case <-node.task.ctx.Done():
				// task has benn cancelled.
				break loop
			case rec := <-node.src:
				if rec == nil {
					// when chan is closed:
					// while record.Tell() is called the ctx is done
					break loop
				} else if rec.IsEOF() || rec.IsCircuitBreak() {
					lastWill = rec
					break loop
				} else if rec.IsError() {
					rec.Tell(node.next)
					continue
				} else { // else if !node.task.shouldStop() <- do not use shouldStop() : https://github.com/machbase/neo/issues/309
					node.nrow++
					node.SetInflight(rec)
					if node.debug {
						node.task.LogDebug("->", node.Name(), "RECV", fmt.Sprintf("%v", rec.key), rec.StringValueTypes(), " ")
					}
					ret, err := node.expr.Eval(node)
					if err != nil {
						ErrorRecord(err).Tell(node.next)
						continue
					}
					if ret == nil {
						continue
					}

					to_next := func(rec *Record) bool {
						if rec == nil {
							return true
						}
						if rec.IsEOF() {
							rec.Tell(node.next)
							return false
						} else if rec.IsCircuitBreak() {
							node.task.fireCircuitBreak(node)
							return false
						} else {
							rec.Tell(node.next)
							return true
						}
					}
					switch rs := ret.(type) {
					case *Record:
						to_next(rs)
					case []*Record:
						for _, rec := range rs {
							if alive := to_next(rec); !alive {
								break
							}
						}
					default:
						errRec := ErrorRecord(fmt.Errorf("func '%s' returns invalid type: %T", node.Name(), ret))
						errRec.Tell(node.next)
					}
				}
			}
		}
		if lastWill != nil {
			if node.eofCallback != nil {
				node.eofCallback(node)
			}
			lastWill.Tell(node.next)
		}
	}()
}

func (node *Node) wait() {
	node.closeWg.Wait()
}

func (node *Node) stop() {
	if node.src != nil {
		close(node.src)
	}
	node.wait()
	for i := len(node.closers) - 1; i >= 0; i-- {
		c := node.closers[i]
		if err := c.Close(); err != nil {
			node.task.LogError(node.name, "context closer", err.Error())
		}
	}
}

func (node *Node) AddCloser(c Closer) {
	node.mutex.Lock()
	node.closers = append(node.closers, c)
	node.mutex.Unlock()
}

func (node *Node) CancelCloser(c Closer) {
	node.mutex.Lock()
	idx := -1
	for i, cl := range node.closers {
		if c == cl {
			idx = i
			break
		}
	}
	if idx >= 0 {
		node.closers = append(node.closers[:idx], node.closers[idx+1:]...)
	}
	node.mutex.Unlock()
}

type Receiver interface {
	Name() string
	Receive(*Record)
}

const kEOF = "f0ec1dea-03e8-4121-8c98-0b78704e009d"
const kBREAK = "5bd2e423-4536-4a8d-a80d-c11567fc296f"
const kBYTES = "a6cd7131-63cc-4f83-9cbb-709a3d317780"
const kIMAGE = "f2f79e86-44dc-4721-95e0-ba42ebe1fe88"
const kERR = "0fd184f8-0f4a-4d05-bf0f-77bd31642eae"
const kARR = "057f1cb0-df9f-41d3-b003-ba7c1ef8f497"

var EofRecord = &Record{key: kEOF}
var BreakRecord = &Record{key: kBREAK}

func ErrorRecord(err error) *Record     { return &Record{key: kERR, value: err} }
func ArrayRecord(arr []*Record) *Record { return &Record{key: kARR, value: arr} }

type Record struct {
	key         any
	value       any
	contentType string
	vars        map[string]any
}

func NewRecord(k, v any) *Record {
	return &Record{key: k, value: v}
}

func NewRecordVars(k, v any, vars map[string]any) *Record {
	return &Record{key: k, value: v, vars: vars}
}

func NewBytesRecord(raw []byte) *Record {
	return &Record{key: kBYTES, value: raw}
}

func NewImageRecord(raw []byte, contentType string) *Record {
	return &Record{key: kIMAGE, value: raw, contentType: contentType}
}

func (r *Record) ReplaceValue(v any) *Record {
	r.value = v
	return r
}

func (r *Record) ReplaceKey(k any) *Record {
	r.key = k
	return r
}

func (r *Record) ReplaceKeyValue(k, v any) *Record {
	r.key = k
	r.value = v
	return r
}

func (r *Record) IsEOF() bool {
	return r.key == kEOF
}

func (r *Record) IsCircuitBreak() bool {
	return r.key == kBREAK
}

func (r *Record) IsError() bool {
	return r.key == kERR
}

func (r *Record) IsBytes() bool {
	return r.key == kBYTES
}

func (r *Record) IsImage() bool {
	return r.key == kIMAGE
}

func (r *Record) Error() error {
	if r.key == kERR {
		return r.value.(error)
	} else {
		return nil
	}
}

func (r *Record) IsArray() bool {
	return r.key == kARR
}

func (r *Record) IsTuple() bool {
	switch r.key {
	case kEOF, kBREAK, kBYTES, kIMAGE, kERR, kARR:
		return false
	default:
		return true
	}
}

func (r *Record) Array() []*Record {
	if r.key == kARR {
		return r.value.([]*Record)
	} else {
		return nil
	}
}

func (r *Record) Key() any {
	return r.key
}

func (r *Record) Value() any {
	return r.value
}

func (r *Record) SetVariable(name string, value any) {
	if r.vars == nil {
		r.vars = map[string]any{}
	}
	r.vars[name] = value
}

func (r *Record) GetVariable(name string) (any, error) {
	if r.vars != nil && strings.HasPrefix(name, "$") {
		if v, ok := r.vars[strings.TrimPrefix(name, "$")]; ok {
			return v, nil
		}
		return nil, nil
	} else {
		return nil, fmt.Errorf("undefined variable '%s'", name)
	}
}

func (r *Record) Flatten() []any {
	k := r.Key()
	v := r.Value()
	switch vv := v.(type) {
	case []any:
		return append([]any{k}, vv...)
	case any:
		return []any{k, vv}
	default:
		if vv == nil {
			return []any{k}
		}
		return []any{k, fmt.Sprintf("Record: unsupported value type(%T)", vv)}
	}
}

func (r *Record) Tell(receiver Receiver) {
	if receiver == nil {
		return
	}
	receiver.Receive(r)
}

func (r *Record) String() string {
	if r == nil {
		return "<nil>"
	}
	if r.key == kEOF {
		return "EOF"
	} else if r.key == kBREAK {
		return "CIRCUITBREAK"
	} else if r.key == kBYTES {
		return "BYTES"
	} else if r.key == kIMAGE {
		return "IMAGE"
	} else if r.key == kERR {
		return fmt.Sprintf("ERROR %s", r.value)
	} else if r.key == kARR {
		return "ARRAY"
	} else {
		return fmt.Sprintf("K:%T(%v) V:%s", r.key, r.key, r.StringValueTypes())
	}
}

func (r *Record) Fields() []any {
	var ret []any
	if value := r.Value(); value == nil {
		// if the value of the record is nil, yield key only
		ret = []any{r.Key()}
	} else {
		switch v := value.(type) {
		case [][]any:
			ret = []any{r.Key()}
			for n := range v {
				ret = append(ret, v[n]...)
			}
		case []any:
			ret = append([]any{r.Key()}, v...)
		case any:
			ret = []any{r.Key(), v}
		}
	}
	return ret
}

func (p *Record) StringValueTypes() string {
	if arr, ok := p.value.([]any); ok {
		return p.stringTypesOfArray(arr, 3)
	} else if arr, ok := p.value.([][]any); ok {
		subTypes := []string{}
		for i, subarr := range arr {
			if i == 3 && len(arr) > i {
				subTypes = append(subTypes, fmt.Sprintf("[%d]{%s}, ...", i, p.stringTypesOfArray(subarr, 3)))
				break
			} else {
				subTypes = append(subTypes, fmt.Sprintf("[%d]{%s}", i, p.stringTypesOfArray(subarr, 3)))
			}
		}

		return fmt.Sprintf("(len=%d) [][]any{%s}", len(arr), strings.Join(subTypes, ","))
	} else {
		return fmt.Sprintf("%T", p.value)
	}
}

func (p *Record) stringTypesOfArray(arr []any, limit int) string {
	s := []string{}
	for i, a := range arr {
		aType := fmt.Sprintf("%T", a)
		if subarr, ok := a.([]any); ok {
			s2 := []string{}
			for n, subelm := range subarr {
				if n == limit && len(subarr) > n {
					s2 = append(s2, fmt.Sprintf("%T,... (len=%d)", subelm, len(subarr)))
					break
				} else {
					s2 = append(s2, fmt.Sprintf("%T", subelm))
				}
			}
			aType = "[]any{" + strings.Join(s2, ",") + "}"
		}

		if i == limit && len(arr) > i {
			t := fmt.Sprintf("%s, ... (len=%d)", aType, len(arr))
			s = append(s, t)
			break
		} else {
			s = append(s, aType)
		}
	}
	return strings.Join(s, ", ")
}

func (p *Record) EqualKey(other *Record) bool {
	if other == nil {
		return false
	}
	switch lv := p.key.(type) {
	case time.Time:
		if rv, ok := other.key.(time.Time); !ok {
			return false
		} else {
			return lv.Nanosecond() == rv.Nanosecond()
		}
	case []int:
		if rv, ok := other.key.([]int); !ok {
			return false
		} else {
			if len(lv) != len(rv) {
				return false
			}
			for i := range lv {
				if lv[i] != rv[i] {
					return false
				}
			}
			return true
		}
	}
	return p.key == other.key
}

func (p *Record) EqualValue(other *Record) bool {
	if other == nil {
		return false
	}
	lv := fmt.Sprintf("%#v", p.value)
	rv := fmt.Sprintf("%#v", other.value)
	return lv == rv
}
