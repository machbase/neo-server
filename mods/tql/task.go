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
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	client "github.com/machbase/neo-client/v2"
	"github.com/machbase/neo-client/v2/api"
	"github.com/machbase/neo-server/v8/mods/codec"
	"github.com/machbase/neo-server/v8/mods/codec/facility"
	"github.com/machbase/neo-server/v8/mods/codec/opts"
	"github.com/machbase/neo-server/v8/mods/eventbus"
	"github.com/machbase/neo-server/v8/mods/logging"
	"github.com/machbase/neo-server/v8/mods/tql/expression"
	"github.com/machbase/neo-server/v8/mods/util"
)

const (
	PRAGMA_LOG_LEVEL            = "log-level"
	PRAGMA_PIPELINE_BUFFER      = "pipeline-buffer"
	DEFAULT_PLAN_CACHE_CAPACITY = 200
)

type Task struct {
	ctx          context.Context
	ctxCancel    context.CancelFunc
	execution    *execution
	executionMu  sync.RWMutex
	runtime      *executionRuntime
	runtimeMu    sync.RWMutex
	lastStats    []StageStats
	lastOutput   OutputMetadata
	statsMu      sync.RWMutex
	executeMu    sync.Mutex
	executing    bool
	executed     bool
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
	plan       *compiledPlan
	output     *output
	nodes      []*Node

	cacheWriterOverride *bytes.Buffer

	_resultColumns client.Columns
	_stateLock     sync.RWMutex
	_created       time.Time
}

type compiledNode struct {
	name   string
	code   string
	line   Line
	pragma map[string]string
	kind   StatementKind
	role   StageRole
	buffer int
}

type compiledPlan struct {
	nodes    []compiledNode
	sink     compiledNode
	logLevel *Level
}

type compiledPlanCacheStore struct {
	mu       sync.Mutex
	capacity int
	plans    map[string]*compiledPlan
	order    []string
}

func newCompiledPlanCache(capacity int) *compiledPlanCacheStore {
	return &compiledPlanCacheStore{
		capacity: capacity,
		plans:    make(map[string]*compiledPlan),
	}
}

func (x *compiledPlanCacheStore) Load(key string) (*compiledPlan, bool) {
	x.mu.Lock()
	plan, ok := x.plans[key]
	x.mu.Unlock()
	return plan, ok
}

func (x *compiledPlanCacheStore) Store(key string, plan *compiledPlan) {
	x.mu.Lock()
	if _, exists := x.plans[key]; exists {
		x.plans[key] = plan
		x.mu.Unlock()
		return
	}
	if len(x.order) == x.capacity {
		oldest := x.order[0]
		delete(x.plans, oldest)
		x.order = x.order[1:]
	}
	x.plans[key] = plan
	x.order = append(x.order, key)
	x.mu.Unlock()
}

func (x *compiledPlanCacheStore) Len() int {
	x.mu.Lock()
	length := len(x.plans)
	x.mu.Unlock()
	return length
}

var compiledPlanCache = newCompiledPlanCache(DEFAULT_PLAN_CACHE_CAPACITY)

type PlanStage struct {
	Role   StageRole
	Code   string
	Line   int
	Pragma map[string]string
	Buffer int
}

type StageRole int

const (
	StageSource StageRole = iota
	StageMap
	StageSink
)

type planInstantiationError struct {
	nodeIndex int
	sink      bool
	err       error
}

func (x *planInstantiationError) Error() string { return x.err.Error() }
func (x *planInstantiationError) Unwrap() error { return x.err }

type execution struct {
	ctx        context.Context
	cancel     context.CancelCauseFunc
	preemptive bool

	stateLock      sync.RWMutex
	shouldStop     bool
	stopListeners  []func()
	stopAt         *Node
	stopOriginIdx  int
	stopOriginName string
	stopCh         chan struct{}
	err            error
	statsMu        sync.RWMutex
	stats          map[int]*stageStats
}

type stageStats struct {
	Index               int
	Name                string
	Role                StageRole
	InputCount          int64
	OutputCount         int64
	QueueCapacity       int
	QueueHighWatermark  int
	BlockedSendDuration time.Duration
	ErrorCount          int64
	LastError           string
	StopOrigin          bool
	StartedAt           time.Time
	FinishedAt          time.Time
}

type StageStats struct {
	Index               int
	Name                string
	Role                StageRole
	InputCount          int64
	OutputCount         int64
	QueueCapacity       int
	QueueHighWatermark  int
	BlockedSendDuration time.Duration
	ErrorCount          int64
	LastError           string
	StopOrigin          bool
	Elapsed             time.Duration
}

type OutputMetadata struct {
	ContentType     string
	ContentEncoding string
	ChartType       string
	HttpHeaders     map[string][]string
}

type executionRuntime struct {
	task      *Task
	execution *execution
	config    executionConfig
	columnsMu sync.RWMutex
	columns   client.Columns
	schema    SchemaState
}

type SchemaState int

const (
	SchemaUnknown SchemaState = iota
	SchemaKnown
	SchemaFinal
)

type executionConfig struct {
	params       map[string][]string
	inputReader  io.Reader
	outputWriter io.Writer
	toJSONOutput bool
	argValues    []any

	logWriter         io.Writer
	logLevel          Level
	consoleLogLevel   Level
	consoleUser       string
	consoleID         string
	consoleTopic      string
	consoleOTP        string
	httpClientFactory func() *http.Client
	volatileProvider  VolatileAssetsProvider
}

func newExecutionRuntime(task *Task, execution *execution) *executionRuntime {
	config := executionConfig{
		inputReader:       task.inputReader,
		outputWriter:      task.outputWriter,
		toJSONOutput:      task.toJsonOutput,
		argValues:         append([]any(nil), task.argValues...),
		logWriter:         task.logWriter,
		logLevel:          task.logLevel,
		consoleLogLevel:   task.consoleLogLevel,
		consoleUser:       task.consoleUser,
		consoleID:         task.consoleId,
		consoleTopic:      task.consoleTopic,
		consoleOTP:        task.consoleOtp,
		httpClientFactory: task.httpClientFactory,
		volatileProvider:  task.volatileAssetsProvider,
	}
	if len(task.params) > 0 {
		config.params = make(map[string][]string, len(task.params))
		for key, values := range task.params {
			config.params[key] = append([]string(nil), values...)
		}
	}
	return &executionRuntime{
		task:      task,
		execution: execution,
		config:    config,
	}
}

func (x *execution) startStage(index int, name string, role StageRole, queueCapacity int) {
	x.statsMu.Lock()
	if x.stats == nil {
		x.stats = make(map[int]*stageStats)
	}
	x.stats[index] = &stageStats{Index: index, Name: name, Role: role, QueueCapacity: queueCapacity, StartedAt: time.Now()}
	x.statsMu.Unlock()
}

func (x *execution) finishStage(index int) {
	x.statsMu.Lock()
	if stat := x.stats[index]; stat != nil {
		stat.FinishedAt = time.Now()
	}
	x.statsMu.Unlock()
}

func (x *execution) addInput(index int) {
	x.statsMu.Lock()
	if stat := x.stats[index]; stat != nil {
		stat.InputCount++
	}
	x.statsMu.Unlock()
}

func (x *execution) addOutput(index int, queueDepth int, blocked time.Duration) {
	x.statsMu.Lock()
	if stat := x.stats[index]; stat != nil {
		stat.OutputCount++
		if queueDepth > stat.QueueHighWatermark {
			stat.QueueHighWatermark = queueDepth
		}
		stat.BlockedSendDuration += blocked
	}
	x.statsMu.Unlock()
}

func (x *execution) addStageError(index int, err error) {
	if err == nil {
		return
	}
	x.statsMu.Lock()
	if stat := x.stats[index]; stat != nil {
		stat.ErrorCount++
		stat.LastError = err.Error()
	}
	x.statsMu.Unlock()
}

func (x *execution) StageStats() []StageStats {
	x.statsMu.RLock()
	stats := make([]StageStats, 0, len(x.stats))
	for _, stat := range x.stats {
		finishedAt := stat.FinishedAt
		if finishedAt.IsZero() {
			finishedAt = time.Now()
		}
		stats = append(stats, StageStats{
			Index:               stat.Index,
			Name:                stat.Name,
			Role:                stat.Role,
			InputCount:          stat.InputCount,
			OutputCount:         stat.OutputCount,
			QueueCapacity:       stat.QueueCapacity,
			QueueHighWatermark:  stat.QueueHighWatermark,
			BlockedSendDuration: stat.BlockedSendDuration,
			ErrorCount:          stat.ErrorCount,
			LastError:           stat.LastError,
			StopOrigin:          stat.StopOrigin,
			Elapsed:             finishedAt.Sub(stat.StartedAt),
		})
	}
	x.statsMu.RUnlock()
	slices.SortFunc(stats, func(left, right StageStats) int {
		return left.Index - right.Index
	})
	return stats
}

func (x *executionRuntime) Context() context.Context {
	return x.execution.ctx
}

func (x *executionRuntime) StartStage(index int, name string, role StageRole, queueCapacity int) {
	x.execution.startStage(index, name, role, queueCapacity)
}

func (x *executionRuntime) FinishStage(index int) {
	x.execution.finishStage(index)
}

func (x *executionRuntime) AddInput(index int) {
	x.execution.addInput(index)
}

func (x *executionRuntime) AddOutput(index int, queueDepth int, blocked time.Duration) {
	x.execution.addOutput(index, queueDepth, blocked)
}

func (x *executionRuntime) EmitError(index int, err error) *Record {
	x.execution.addStageError(index, err)
	return ErrorRecord(err)
}

func (x *executionRuntime) ReportError(index int, err error) {
	x.execution.addStageError(index, err)
	x.execution.reportError(err)
}

func (x *executionRuntime) Fail(err error) {
	x.fail(err)
}

func (x *executionRuntime) Warn(args ...any) {
	x.LogWarn(args...)
}

func (x *executionRuntime) InputReader() io.Reader { return x.config.inputReader }

func (x *executionRuntime) Params() map[string][]string { return x.config.params }

func (x *executionRuntime) ArgValues() []any { return x.config.argValues }

func (x *executionRuntime) OutputWriter() io.Writer {
	if x.config.outputWriter == nil {
		return &util.NopCloseWriter{Writer: os.Stdout}
	}
	return x.config.outputWriter
}

func (x *executionRuntime) ToJSONOutput() bool { return x.config.toJSONOutput }

func (x *executionRuntime) ConsoleUser() string { return x.config.consoleUser }

func (x *executionRuntime) ConsoleOTP() string { return x.config.consoleOTP }

func (x *executionRuntime) LogWriter() io.Writer { return x.config.logWriter }

func (x *executionRuntime) NewChildTask() *Task {
	child := NewTaskContext(x.Context())
	child.SetParams(x.Params())
	child.SetConsoleLogLevel(x.config.consoleLogLevel)
	child.SetConsole(x.config.consoleUser, x.config.consoleID, x.config.consoleOTP)
	child.SetLogWriter(x.config.logWriter)
	child.SetLogLevel(x.config.logLevel)
	child.SetHttpClientFactory(x.config.httpClientFactory)
	child.SetVolatileAssetsProvider(x.config.volatileProvider)
	return child
}

func (x *executionRuntime) NewHTTPClient() *http.Client {
	if x.config.httpClientFactory != nil {
		return x.config.httpClientFactory()
	}
	return &http.Client{}
}

func (x *executionRuntime) VolatileFilePrefix() string {
	if x.config.volatileProvider == nil {
		return ""
	}
	return x.config.volatileProvider.VolatileFilePrefix()
}

func (x *executionRuntime) VolatileFileWrite(name string, data []byte, deadline time.Time) {
	if x.config.volatileProvider != nil {
		x.config.volatileProvider.VolatileFileWrite(name, data, deadline)
	}
}

func (x *executionRuntime) SetResultColumns(cols client.Columns) {
	x.columnsMu.RLock()
	final := x.schema == SchemaFinal
	x.columnsMu.RUnlock()
	if final {
		x.Fail(errors.New("result columns mutation after schema finalization"))
		return
	}
	x.setResultColumns(normalizeResultColumns(cols))
}

func (x *executionRuntime) setResultColumns(cols client.Columns) {
	x.columnsMu.Lock()
	x.columns = normalizeResultColumns(cols)
	if len(x.columns) > 0 {
		x.schema = SchemaKnown
	}
	x.columnsMu.Unlock()
}

func (x *executionRuntime) ResultColumns() client.Columns {
	x.columnsMu.RLock()
	cols := x.columns
	x.columnsMu.RUnlock()
	return normalizeResultColumns(cols)
}

func (x *executionRuntime) SchemaState() SchemaState {
	x.columnsMu.RLock()
	state := x.schema
	x.columnsMu.RUnlock()
	return state
}

func (x *executionRuntime) FinalizeSchema() {
	x.columnsMu.Lock()
	x.schema = SchemaFinal
	x.columnsMu.Unlock()
}

func (x *executionRuntime) AddShouldStopListener(fn func()) {
	if fn == nil {
		return
	}
	x.execution.stateLock.Lock()
	alreadyStopped := x.execution.shouldStop
	if !alreadyStopped {
		x.execution.stopListeners = append(x.execution.stopListeners, fn)
	}
	x.execution.stateLock.Unlock()
	if alreadyStopped {
		fn()
	}
}

func (x *executionRuntime) Logf(format string, args ...any)      { x.logf(INFO, format, args...) }
func (x *executionRuntime) LogInfof(format string, args ...any)  { x.logf(INFO, format, args...) }
func (x *executionRuntime) LogTracef(format string, args ...any) { x.logf(TRACE, format, args...) }
func (x *executionRuntime) LogDebugf(format string, args ...any) { x.logf(DEBUG, format, args...) }
func (x *executionRuntime) LogWarnf(format string, args ...any)  { x.logf(WARN, format, args...) }
func (x *executionRuntime) LogErrorf(format string, args ...any) { x.logf(ERROR, format, args...) }
func (x *executionRuntime) Log(args ...any)                      { x.log(INFO, args...) }
func (x *executionRuntime) LogInfo(args ...any)                  { x.log(INFO, args...) }
func (x *executionRuntime) LogTrace(args ...any)                 { x.log(TRACE, args...) }
func (x *executionRuntime) LogDebug(args ...any)                 { x.log(DEBUG, args...) }
func (x *executionRuntime) LogWarn(args ...any)                  { x.log(WARN, args...) }
func (x *executionRuntime) LogError(args ...any)                 { x.log(ERROR, args...) }

func (x *executionRuntime) log(level Level, args ...any) {
	if x.config.logWriter != nil && level >= x.config.logLevel {
		if logger, ok := x.config.logWriter.(logging.Log); ok {
			if logLevel := level.LoggingLevel(); logLevel >= logger.Level() {
				logger.Log(logLevel, strings.TrimRightFunc(fmt.Sprintln(args...), unicode.IsSpace))
			}
		} else {
			line := fmt.Sprintln(append([]any{"[" + Levels[level] + "]"}, args...)...)
			x.config.logWriter.Write([]byte(line))
		}
	}
	if x.config.consoleTopic != "" && level >= x.config.consoleLogLevel {
		parts := make([]string, 0, len(args))
		for _, arg := range args {
			parts = append(parts, fmt.Sprintf("%v", arg))
		}
		eventbus.PublishLogTask(x.config.consoleTopic, Levels[level], fmt.Sprintf("%p", x.task), strings.Join(parts, " "))
	}
}

func (x *executionRuntime) logf(level Level, format string, args ...any) {
	if x.config.logWriter != nil && level >= x.config.logLevel {
		if logger, ok := x.config.logWriter.(logging.Log); ok {
			if logLevel := level.LoggingLevel(); logLevel >= logger.Level() {
				logger.Logf(logLevel, format, args...)
			}
		} else {
			line := fmt.Sprintf("[%s] "+format+"\n", append([]any{Levels[level]}, args...)...)
			x.config.logWriter.Write([]byte(line))
		}
	}
	if x.config.consoleTopic != "" && level >= x.config.consoleLogLevel {
		eventbus.PublishLogTask(x.config.consoleTopic, Levels[level], fmt.Sprintf("%p", x.task), fmt.Sprintf(format, args...))
	}
}

func (x *executionRuntime) Cancel() {
	if !x.execution.preemptive {
		x.task.ctxCancel()
		x.execution.fireCircuitBreak(nil)
	}
}

func (x *executionRuntime) stopSignal(node *Node) <-chan struct{} {
	x.execution.stateLock.RLock()
	defer x.execution.stateLock.RUnlock()
	if !x.execution.shouldStop || x.execution.stopAt == nil || node.index <= x.execution.stopAt.index {
		return x.execution.stopCh
	}
	return nil
}

func (x *executionRuntime) shouldStopNode(node *Node) bool {
	x.execution.stateLock.RLock()
	defer x.execution.stateLock.RUnlock()
	return x.execution.shouldStop && (x.execution.stopAt == nil || node.index <= x.execution.stopAt.index)
}

func (x *executionRuntime) ShouldStop() bool {
	x.execution.stateLock.RLock()
	ret := x.execution.shouldStop
	x.execution.stateLock.RUnlock()
	return ret
}

func (x *executionRuntime) shouldFinalizeOnStop() bool {
	x.execution.stateLock.RLock()
	defer x.execution.stateLock.RUnlock()
	return x.execution.shouldStop && x.execution.stopAt == nil
}

func (x *executionRuntime) fireCircuitBreak(stopAt *Node) {
	x.execution.fireCircuitBreak(stopAt)
}

func (x *executionRuntime) fail(err error) {
	if err == nil {
		return
	}
	x.execution.reportError(err)
	x.execution.cancel(err)
	x.execution.fireCircuitBreak(nil)
}

func newExecution(ctx context.Context, preemptive bool) *execution {
	execCtx, cancel := context.WithCancelCause(ctx)
	ret := &execution{
		ctx:           execCtx,
		cancel:        cancel,
		preemptive:    preemptive,
		stopOriginIdx: -1,
		stopCh:        make(chan struct{}),
	}
	context.AfterFunc(execCtx, func() {
		ret.fireCircuitBreak(nil)
	})
	return ret
}

func (x *execution) fireCircuitBreak(stopAt *Node) {
	x.stateLock.Lock()
	if x.shouldStop {
		x.stateLock.Unlock()
		return
	}
	x.shouldStop = true
	x.stopAt = stopAt
	x.stopOriginIdx = -1
	if stopAt != nil {
		x.stopOriginIdx = stopAt.index
		x.stopOriginName = stopAt.Name()
	}
	close(x.stopCh)
	listeners := append([]func(){}, x.stopListeners...)
	x.stateLock.Unlock()

	if stopAt != nil {
		x.statsMu.Lock()
		if stat := x.stats[stopAt.index]; stat != nil {
			stat.StopOrigin = true
		}
		x.statsMu.Unlock()
	}

	for _, fn := range listeners {
		fn()
	}
}

func (x *execution) reportError(err error) {
	if err == nil {
		return
	}
	x.stateLock.Lock()
	if x.err == nil {
		x.err = err
	}
	x.stateLock.Unlock()
}

func (x *execution) Error() error {
	x.stateLock.RLock()
	err := x.err
	x.stateLock.RUnlock()
	return err
}

var (
	_ facility.Logger             = &Task{}
	_ facility.VolatileFileWriter = &Task{}
	_ facility.Logger             = &executionRuntime{}
	_ facility.VolatileFileWriter = &executionRuntime{}
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
	ret.execution = newExecution(ret.ctx, false)
	return ret
}

func (x *Task) beginExecution(ctx context.Context, preemptive bool) *execution {
	ret := newExecution(ctx, preemptive)
	x.executionMu.Lock()
	x.execution = ret
	x.executionMu.Unlock()
	return ret
}

func (x *Task) currentExecution() *execution {
	x.executionMu.RLock()
	ret := x.execution
	x.executionMu.RUnlock()
	return ret
}

func (x *Task) Context() context.Context {
	return x.currentExecution().ctx
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

func (x *Task) SinkName() string {
	if x.plan == nil {
		return ""
	}
	return x.plan.sink.name
}

func (x *Task) ExplainPlan() []PlanStage {
	if x.plan == nil {
		return nil
	}
	stages := make([]PlanStage, 0, len(x.plan.nodes)+1)
	for _, node := range x.plan.nodes {
		stages = append(stages, PlanStage{
			Role:   node.role,
			Code:   node.code,
			Line:   node.line.line,
			Pragma: clonePragma(node.pragma),
			Buffer: node.buffer,
		})
	}
	stages = append(stages, PlanStage{
		Role:   x.plan.sink.role,
		Code:   x.plan.sink.code,
		Line:   x.plan.sink.line.line,
		Pragma: clonePragma(x.plan.sink.pragma),
		Buffer: x.plan.sink.buffer,
	})
	return stages
}

func (x *Task) Compile(codeReader io.Reader) error {
	code, err := io.ReadAll(codeReader)
	if err != nil {
		return err
	}
	h := sha1.New()
	h.Write(code)
	x.sourceHash = fmt.Sprintf("%x", h.Sum(nil))
	if plan, ok := compiledPlanCache.Load(x.sourceHash); ok {
		if plan.logLevel != nil {
			x.SetLogLevel(*plan.logLevel)
		}
		x.plan = plan
		x.nodes = nil
		x.output = nil
		x.compiled = true
		x.executeMu.Lock()
		x.executed = false
		x.executeMu.Unlock()
		return nil
	}

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
	if err == nil {
		x.executeMu.Lock()
		x.executed = false
		x.executeMu.Unlock()
		x.nodes = nil
		x.output = nil
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

	plan := &compiledPlan{}
	var nodeStatements []*Statement
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
					level := ParseLogLevel(kv.Value)
					x.SetLogLevel(level)
					plan.logLevel = &level
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
			buffer, err := parsePipelineBuffer(pragmas)
			if err != nil {
				x.compileErr = wrapCompileStatementError("sink_compile_error", stmt, curLine, err)
				return x.compileErr
			}
			plan.sink = compiledNode{
				code:   curLine.text,
				line:   *curLine,
				pragma: clonePragma(pragmas),
				role:   StageSink,
				buffer: buffer,
			}
		} else {
			buffer, err := parsePipelineBuffer(pragmas)
			if err != nil {
				kind := "map_compile_error"
				if len(plan.nodes) == 0 {
					kind = "source_compile_error"
				}
				x.compileErr = wrapCompileStatementError(kind, stmt, curLine, err)
				return x.compileErr
			}
			role := StageMap
			if len(plan.nodes) == 0 {
				role = StageSource
			}
			plan.nodes = append(plan.nodes, compiledNode{
				code:   stmt.Text,
				line:   *curLine,
				pragma: clonePragma(pragmas),
				kind:   stmt.Kind,
				role:   role,
				buffer: buffer,
			})
			nodeStatements = append(nodeStatements, stmt)
		}
		pragmas = nil
	}

	if err := x.validatePlan(plan); err != nil {
		var instantiateErr *planInstantiationError
		if errors.As(err, &instantiateErr) {
			if instantiateErr.sink {
				x.compileErr = wrapCompileStatementError("sink_compile_error", tailStmt, tailStmt.toLine(), err)
			} else if instantiateErr.nodeIndex < len(nodeStatements) {
				kind := "map_compile_error"
				if instantiateErr.nodeIndex == 0 {
					kind = "source_compile_error"
				}
				stmt := nodeStatements[instantiateErr.nodeIndex]
				x.compileErr = wrapCompileStatementError(kind, stmt, stmt.toLine(), err)
			} else {
				x.compileErr = err
			}
		} else {
			x.compileErr = err
		}
		return x.compileErr
	}
	x.plan = plan
	x.compiled = true
	compiledPlanCache.Store(x.sourceHash, plan)
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
	Err            error
	Message        string
	IsDbSink       bool
	OutputMetadata OutputMetadata
	_created       time.Time
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
	x.executeMu.Lock()
	if x.executing {
		x.executeMu.Unlock()
		return &Result{Err: errors.New("task is already executing"), _created: x._created}
	}
	if x.executed {
		x.executeMu.Unlock()
		return &Result{Err: errors.New("task has already been executed"), _created: x._created}
	}
	x.executing = true
	x.executed = true
	x.executeMu.Unlock()
	defer func() {
		x.executeMu.Lock()
		x.executing = false
		x.executeMu.Unlock()
	}()

	result := x.execute()
	if result.Err != nil {
		x.LogError("Task", result.Err.Error())
	} else {
		x.LogDebug("Task elapsed", time.Since(x._created).String())
	}
	return result
}

func (x *Task) cloneForExecution(ctx context.Context, writer io.Writer) (*Task, error) {
	if x.plan == nil {
		return nil, errors.New("task plan is unavailable")
	}
	if x.inputReader != nil {
		return nil, errors.New("preemptive cache update does not support an input reader")
	}

	clone := NewTaskContext(ctx)
	clone.params = make(map[string][]string, len(x.params))
	for key, values := range x.params {
		clone.params[key] = append([]string(nil), values...)
	}
	clone.outputWriter = writer
	clone.toJsonOutput = x.toJsonOutput
	clone.logWriter = x.logWriter
	clone.consoleUser = x.consoleUser
	clone.consoleId = x.consoleId
	clone.consoleTopic = x.consoleTopic
	clone.consoleOtp = x.consoleOtp
	clone.logLevel = x.logLevel
	clone.consoleLogLevel = x.consoleLogLevel
	clone.argValues = append([]any(nil), x.argValues...)
	clone.httpClientFactory = x.httpClientFactory
	clone.volatileAssetsProvider = x.volatileAssetsProvider
	clone.sourcePath = x.sourcePath
	if cacheWriter, ok := writer.(*bytes.Buffer); ok {
		clone.cacheWriterOverride = cacheWriter
	}
	clone.plan = x.plan
	clone.compiled = true
	return clone, nil
}

func clonePragma(pragmas map[string]string) map[string]string {
	if pragmas == nil {
		return nil
	}
	ret := make(map[string]string, len(pragmas))
	for key, value := range pragmas {
		ret[key] = value
	}
	return ret
}

func parsePipelineBuffer(pragmas map[string]string) (int, error) {
	if pragmas == nil || pragmas[PRAGMA_PIPELINE_BUFFER] == "" {
		return 0, nil
	}
	buffer, err := strconv.Atoi(pragmas[PRAGMA_PIPELINE_BUFFER])
	if err != nil || buffer < 0 {
		return 0, fmt.Errorf("invalid %s %q", PRAGMA_PIPELINE_BUFFER, pragmas[PRAGMA_PIPELINE_BUFFER])
	}
	return buffer, nil
}

func (x *Task) validatePlan(plan *compiledPlan) error {
	for index, spec := range plan.nodes {
		node := NewNode(x)
		if err := node.compile(spec.code); err != nil {
			return &planInstantiationError{nodeIndex: index, err: err}
		}
	}

	sinkNode := NewNode(x)
	sinkNode.role = plan.sink.role
	if err := sinkNode.compile(plan.sink.code); err != nil {
		return &planInstantiationError{sink: true, err: err}
	}
	plan.sink.name = sinkNode.Name()
	return nil
}

func (x *Task) instantiatePlan(plan *compiledPlan) error {
	x.nodes = nil
	x.output = nil
	for index, spec := range plan.nodes {
		node := NewNode(x)
		if err := node.compile(spec.code); err != nil {
			return &planInstantiationError{nodeIndex: index, err: err}
		}
		node.pragma = clonePragma(spec.pragma)
		line := spec.line
		node.tqlLine = &line
		node.index = index
		node.kind = spec.kind
		node.role = spec.role
		node.output = make(chan *Record, spec.buffer)
		x.nodes = append(x.nodes, node)
	}
	if len(x.nodes) == 0 {
		return errors.New("no source exists")
	}

	sinkLine := plan.sink.line
	x.output = &output{
		index:   len(x.nodes),
		task:    x,
		name:    plan.sink.name,
		pragma:  clonePragma(plan.sink.pragma),
		tqlLine: &sinkLine,
	}
	x.compiled = true
	return nil
}

func (x *Task) runPreemptiveCacheUpdate(cacheWriter *bytes.Buffer) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	clone, err := x.cloneForExecution(ctx, cacheWriter)
	if err != nil {
		x.LogError("Cache preemptive update", err.Error())
		return
	}
	clone.Execute()
}

func (x *Task) execute() (result *Result) {
	if !x.compiled {
		return &Result{Err: errors.New("not compiled task"), _created: x._created}
	}
	if x.plan == nil {
		return &Result{Err: errors.New("task execution panic: compiled plan is unavailable"), _created: x._created}
	}
	if err := x.instantiatePlan(x.plan); err != nil {
		return &Result{Err: err, _created: x._created}
	}
	exec := x.beginExecution(x.ctx, false)
	runtime := newExecutionRuntime(x, exec)
	if err := x.output.prepare(runtime); err != nil {
		if x.output != nil {
			err = x.output.wrapSinkError("sink_compile_error", err)
		}
		return &Result{Err: err, _created: x._created}
	}
	defer func() {
		if r := recover(); r != nil {
			w := &bytes.Buffer{}
			w.Write(debug.Stack())
			x.LogErrorf("panic %v\n%s", r, w.String())
			result = &Result{
				Err:      fmt.Errorf("task execution panic: %v", r),
				_created: x._created,
			}
		}
	}()

	if x.output.cachedData != nil {
		// send cached data to client first
		x.outputWriter.Write(x.output.cachedData)
		metadata := x.output.Metadata()
		x.publishOutputMetadata(metadata)

		// Do preemptive update in background
		// if the cachedData and cacheWriter are set => preemptive update
		if x.output.cacheWriter != nil {
			go x.runPreemptiveCacheUpdate(x.output.cacheWriter)
		}
		return &Result{
			Err:            nil,
			Message:        "cached",
			IsDbSink:       x.output.dbSink != nil,
			OutputMetadata: metadata,
			_created:       x._created,
		}
	}

	runtime = x.executeOutput(runtime)
	x.publishResultColumns(runtime.ResultColumns())
	x.statsMu.Lock()
	x.lastStats = runtime.execution.StageStats()
	x.statsMu.Unlock()

	if x.output != nil {
		metadata := x.output.Metadata()
		x.publishOutputMetadata(metadata)
		err := exec.Error()
		if err == nil {
			err = x.output.lastError
		}
		return &Result{
			Err:            err,
			Message:        x.output.lastMessage,
			IsDbSink:       x.output.dbSink != nil,
			OutputMetadata: metadata,
			_created:       x._created,
		}
	}
	return &Result{
		Err:      errors.New("no sink exists"),
		_created: x._created,
	}
}

func (x *Task) LastStageStats() []StageStats {
	x.statsMu.RLock()
	stats := append([]StageStats(nil), x.lastStats...)
	x.statsMu.RUnlock()
	return stats
}

func (x *Task) publishOutputMetadata(metadata OutputMetadata) {
	x.statsMu.Lock()
	x.lastOutput = cloneOutputMetadata(metadata)
	x.statsMu.Unlock()
}

func (x *Task) LastOutputMetadata() OutputMetadata {
	x.statsMu.RLock()
	metadata := x.lastOutput
	x.statsMu.RUnlock()
	return cloneOutputMetadata(metadata)
}

func cloneOutputMetadata(metadata OutputMetadata) OutputMetadata {
	metadata.HttpHeaders = cloneHeaders(metadata.HttpHeaders)
	return metadata
}

func cloneHeaders(headers map[string][]string) map[string][]string {
	if headers == nil {
		return nil
	}
	ret := make(map[string][]string, len(headers))
	for key, values := range headers {
		ret[key] = append([]string(nil), values...)
	}
	return ret
}

func (x *Task) executeOutput(runtime *executionRuntime) *executionRuntime {
	x.runtimeMu.Lock()
	x.runtime = runtime
	x.runtimeMu.Unlock()
	defer func() {
		x.runtimeMu.Lock()
		if x.runtime == runtime {
			x.runtime = nil
		}
		x.runtimeMu.Unlock()
	}()
	for _, node := range x.nodes {
		node.runtime = runtime
	}
	x.output.runtime = runtime
	first := x.nodes[0]
	stream := first.RunSource()
	for _, child := range x.nodes[1:] {
		stream = child.Run(stream)
	}
	done := x.output.Consume(stream)

	<-done
	for _, child := range x.nodes {
		<-child.done
	}
	return runtime
}

func (x *Task) Cancel() {
	if exec := x.currentExecution(); !exec.preemptive {
		x.ctxCancel()
		exec.fireCircuitBreak(nil)
	}
}

func (x *Task) AddShouldStopListener(fn func()) {
	if fn == nil {
		return
	}

	exec := x.currentExecution()
	exec.stateLock.Lock()
	alreadyStopped := exec.shouldStop
	if !alreadyStopped {
		exec.stopListeners = append(exec.stopListeners, fn)
	}
	exec.stateLock.Unlock()

	if alreadyStopped {
		fn()
	}
}

func (x *Task) fireCircuitBreak(stopAt *Node) {
	x.currentExecution().fireCircuitBreak(stopAt)
}

func (x *Task) shouldStopNode(node *Node) bool {
	exec := x.currentExecution()
	exec.stateLock.RLock()
	defer exec.stateLock.RUnlock()
	return exec.shouldStop && (exec.stopAt == nil || node.index <= exec.stopAt.index)
}

func (x *Task) stopSignal(node *Node) <-chan struct{} {
	exec := x.currentExecution()
	exec.stateLock.RLock()
	defer exec.stateLock.RUnlock()
	if !exec.shouldStop || exec.stopAt == nil || node.index <= exec.stopAt.index {
		return exec.stopCh
	}
	return nil
}

func (x *Task) publishResultColumns(cols client.Columns) {
	ts := normalizeResultColumns(cols)
	x._stateLock.Lock()
	x._resultColumns = ts
	x._stateLock.Unlock()
}

func (x *Task) ResultColumns() client.Columns {
	x._stateLock.RLock()
	ret := x._resultColumns
	x._stateLock.RUnlock()
	return normalizeResultColumns(ret)
}

func normalizeResultColumns(cols client.Columns) client.Columns {
	ret := make(client.Columns, len(cols))
	for index, column := range cols {
		if column == nil {
			continue
		}
		copy := *column
		switch copy.DataType {
		case "sql.RawBytes":
			copy.DataType = api.DataTypeBinary
		case "sql.NullBool":
			copy.DataType = api.DataTypeBoolean
		case "sql.NullByte":
			copy.DataType = api.DataTypeByte
		case "sql.NullFloat64":
			copy.DataType = api.DataTypeFloat64
		case "sql.NullInt16":
			copy.DataType = api.DataTypeInt16
		case "sql.NullInt32":
			copy.DataType = api.DataTypeInt32
		case "sql.NullInt64":
			copy.DataType = api.DataTypeInt64
		case "sql.NullString":
			copy.DataType = api.DataTypeString
		case "sql.NullTime":
			copy.DataType = api.DataTypeDatetime
		}
		ret[index] = &copy
	}
	return ret
}

func (x *Task) OutputContentType() string {
	if x.output != nil {
		ret := x.output.ContentType()
		return ret
	}
	if metadata := x.LastOutputMetadata(); metadata.ContentType != "" {
		return metadata.ContentType
	}
	return "application/octet-stream"
}

func (x *Task) OutputContentEncoding() string {
	if x.output != nil {
		if contentEncoding := x.output.ContentEncoding(); len(contentEncoding) > 0 {
			return contentEncoding
		}
	}
	if metadata := x.LastOutputMetadata(); metadata.ContentEncoding != "" {
		return metadata.ContentEncoding
	}
	return "identity"
}

func (x *Task) OutputHttpHeaders() map[string][]string {
	if x.output != nil {

		return x.output.HttpHeaders()
	}
	return cloneHeaders(x.LastOutputMetadata().HttpHeaders)
}

func (x *Task) OutputChartType() string {
	if x.output != nil {
		if x.output.IsChart() {
			return "echarts"
		} else if x.output.IsGeoMap() {
			return "geomap"
		}
	}
	return x.LastOutputMetadata().ChartType
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
	task    *Task
	runtime *executionRuntime
	name    string
	index   int
	kind    StatementKind
	role    StageRole

	input  <-chan *Record
	output chan *Record
	done   chan struct{}
	expr   *expression.Expression
	nrow   int

	functions map[string]expression.Function
	values    map[string]any
	debug     bool

	closers []Closer
	closed  bool
	mutex   sync.Mutex

	_inflight *Record

	finalizeCallback func(*Node)

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
	node.output = make(chan *Record)
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

func (node *Node) ensureRuntime() *executionRuntime {
	if node.runtime == nil {
		node.runtime = newExecutionRuntime(node.task, node.task.currentExecution())
	}
	return node.runtime
}

func (node *Node) emit(rec *Record) {
	if rec == nil {
		return
	}
	node.ensureRuntime()
	startedAt := time.Now()
	for {
		select {
		case node.output <- rec:
			node.runtime.AddOutput(node.index, len(node.output), time.Since(startedAt))
			return
		case <-node.runtime.Context().Done():
			node.runtime.Cancel()
			return
		case <-node.runtime.stopSignal(node):
			if node.runtime.shouldStopNode(node) {
				return
			}
		}
	}
}

func (node *Node) SetFinalize(f func(*Node)) {
	node.finalizeCallback = f
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
		node.ensureRuntime().LogDebug("++", node.name, "-->", "output", yieldRec.String(), " ")
	}
	node.emit(yieldRec)
}

func (node *Node) RunSource() <-chan *Record {
	node.ensureRuntime()
	node.done = make(chan struct{})
	go func() {
		node.runtime.StartStage(node.index, node.Name(), StageSource, cap(node.output))
		defer func() {
			node.closeResources()
			close(node.output)
			close(node.done)
			node.runtime.FinishStage(node.index)
			if o := recover(); o != nil {
				w := &bytes.Buffer{}
				w.Write(debug.Stack())
				node.runtime.Log("panic", node.name, o, w.String())
				node.runtime.LogErrorf("panic %s %v\n%s", node.name, o, w.String())
				node.runtime.Fail(fmt.Errorf("source %s panic: %v", node.name, o))
			}
		}()

		select {
		case <-node.runtime.Context().Done():
			if node.finalizeCallback != nil {
				node.finalizeCallback(node)
			}
			return
		default:
		}

		node.SetInflight(nil)
		ret, err := node.expr.Eval(node)
		if err != nil {
			node.emit(node.runtime.EmitError(node.index, err))
		} else {
			switch records := ret.(type) {
			case nil:
			case *Record:
				node.emit(records)
			case []*Record:
				for _, record := range records {
					node.emit(record)
				}
			default:
				node.emit(node.runtime.EmitError(node.index, fmt.Errorf("func '%s' returns invalid type: %T", node.Name(), ret)))
			}
		}
		if node.finalizeCallback != nil {
			node.finalizeCallback(node)
		}
	}()
	return node.output
}

func (node *Node) Run(input <-chan *Record) <-chan *Record {
	node.ensureRuntime()
	node.input = input
	node.done = make(chan struct{})
	go func() {
		node.runtime.StartStage(node.index, node.Name(), StageMap, cap(node.output))
		defer func() {
			node.closeResources()
			close(node.output)
			close(node.done)
			node.runtime.FinishStage(node.index)
			if o := recover(); o != nil {
				w := &bytes.Buffer{}
				w.Write(debug.Stack())
				node.runtime.Log("panic", node.name, o, w.String())
				node.runtime.LogErrorf("panic %s %v\n%s", node.name, o, w.String())
				node.runtime.Fail(fmt.Errorf("node %s panic: %v", node.name, o))
			}
		}()
		completed := false
	loop:
		for {
			select {
			case <-node.runtime.Context().Done():
				// task has benn cancelled.
				completed = true
				break loop
			case <-node.runtime.stopSignal(node):
				if node.runtime.shouldStopNode(node) {
					completed = node.runtime.shouldFinalizeOnStop()
					break loop
				}
			case rec, ok := <-node.input:
				if !ok || rec == nil {
					completed = true
					break loop
				} else if rec.IsError() {
					node.runtime.AddInput(node.index)
					node.emit(rec)
					continue
				} else { // else if !node.task.shouldStop() <- do not use shouldStop() : https://github.com/machbase/neo/issues/309
					node.runtime.AddInput(node.index)
					node.nrow++
					node.SetInflight(rec)
					if node.debug {
						node.runtime.LogDebug("->", node.Name(), "RECV", fmt.Sprintf("%v", rec.key), rec.StringValueTypes(), " ")
					}
					ret, err := node.expr.Eval(node)
					if err != nil {
						node.emit(node.runtime.EmitError(node.index, err))
						continue
					}
					if ret == nil {
						continue
					}

					to_next := func(rec *Record) {
						if rec == nil {
							return
						}
						node.emit(rec)
					}
					switch rs := ret.(type) {
					case *Record:
						to_next(rs)
					case []*Record:
						for _, rec := range rs {
							to_next(rec)
						}
					default:
						errRec := node.runtime.EmitError(node.index, fmt.Errorf("func '%s' returns invalid type: %T", node.Name(), ret))
						node.emit(errRec)
					}
				}
			}
		}
		if completed && node.finalizeCallback != nil {
			node.finalizeCallback(node)
		}
	}()
	return node.output
}

func (node *Node) closeResources() {
	node.mutex.Lock()
	if node.closed {
		node.mutex.Unlock()
		return
	}
	node.closed = true
	closers := node.closers
	node.closers = nil
	node.mutex.Unlock()

	for i := len(closers) - 1; i >= 0; i-- {
		c := closers[i]
		if err := c.Close(); err != nil {
			node.ensureRuntime().LogError(node.name, "context closer", err.Error())
		}
	}
}

func (node *Node) AddCloser(c Closer) {
	if c == nil {
		return
	}
	node.mutex.Lock()
	if !node.closed {
		node.closers = append(node.closers, c)
		node.mutex.Unlock()
		return
	}
	node.mutex.Unlock()
	if err := c.Close(); err != nil {
		node.ensureRuntime().LogError(node.name, "context closer", err.Error())
	}
}

func (node *Node) CancelCloser(c Closer) {
	node.mutex.Lock()
	if node.closed {
		node.mutex.Unlock()
		return
	}
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

const kBYTES = "a6cd7131-63cc-4f83-9cbb-709a3d317780"
const kIMAGE = "f2f79e86-44dc-4721-95e0-ba42ebe1fe88"
const kERR = "0fd184f8-0f4a-4d05-bf0f-77bd31642eae"
const kARR = "057f1cb0-df9f-41d3-b003-ba7c1ef8f497"

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
	case kBYTES, kIMAGE, kERR, kARR:
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

func (r *Record) String() string {
	if r == nil {
		return "<nil>"
	}
	switch r.key {
	case kBYTES:
		return "BYTES"
	case kIMAGE:
		return "IMAGE"
	case kERR:
		return fmt.Sprintf("ERROR %s", r.value)
	case kARR:
		return "ARRAY"
	default:
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

type DatabaseSink interface {
	Open(runtime *executionRuntime) error
	Close() (string, error)
	AddRow([]any) error
}

var (
	_ DatabaseSink = &insert{}
	_ DatabaseSink = &appender{}
)

type Encoder struct {
	format      string
	opts        []opts.Option
	cacheOption *CacheParam
}

func (e *Encoder) RowEncoder(args ...opts.Option) codec.RowsEncoder {
	e.opts = append(args, e.opts...)
	ret := codec.NewEncoder(e.format, e.opts...)
	return ret
}

type output struct {
	index   int
	task    *Task
	name    string
	runtime *executionRuntime
	ready   bool

	input <-chan *Record

	encoder  codec.RowsEncoder
	dbSink   DatabaseSink
	isChart  bool
	isGeoMap bool

	lastError   error
	lastMessage string

	cacheOption *CacheParam
	cacheWriter *bytes.Buffer
	cachedData  []byte

	pragma  map[string]string
	tqlLine *Line
}

func (out *output) prepare(runtime *executionRuntime) error {
	if out == nil || out.tqlLine == nil {
		return errors.New("no sink exists")
	}
	if out.ready {
		return nil
	}
	sinkNode := NewNode(out.task)
	sinkNode.role = StageSink
	sinkNode.runtime = runtime
	sinkOutput, err := sinkNode.compileSink(out.tqlLine)
	if err != nil {
		return err
	}
	out.name = sinkOutput.name
	out.encoder = sinkOutput.encoder
	out.dbSink = sinkOutput.dbSink
	out.isChart = sinkOutput.isChart
	out.isGeoMap = sinkOutput.isGeoMap
	out.cacheOption = sinkOutput.cacheOption
	out.cacheWriter = sinkOutput.cacheWriter
	out.cachedData = sinkOutput.cachedData
	out.runtime = runtime
	out.ready = true
	return nil
}

func (out *output) wrapSinkError(kind string, err error) error {
	if err == nil || out == nil || out.tqlLine == nil {
		return err
	}
	return &ScriptError{
		Kind:    kind,
		Message: err.Error(),
		Span: expression.SourceSpan{
			Start: expression.SourcePosition{Line: out.tqlLine.line, Column: 1},
		},
		StatementText: out.tqlLine.text,
		Cause:         err,
	}
}

func (node *Node) compileSink(code *Line) (ret *output, err error) {
	if node.runtime == nil {
		node.runtime = newExecutionRuntime(node.task, node.task.currentExecution())
	}
	defer func() {
		// panic case: if the 'code' is not applicable as SINK
		if x := recover(); x != nil {
			if e, ok := x.(error); ok {
				err = fmt.Errorf("unable to apply to SINK: %s %s", code.text, e.Error())
				debug.PrintStack()
			} else {
				err = fmt.Errorf("unable to apply to SINK: %s", code.text)
			}
		}
	}()
	expr, err := node.Parse(code.text)
	if err != nil {
		return nil, err
	}
	node.name = asNodeName(expr)
	sink, err := expr.Eval(node)
	if err != nil {
		return nil, err
	}
	if sink == nil {
		if code.text == "" {
			return nil, errors.New("NULL is not applicable for SINK")
		} else {
			return nil, fmt.Errorf("%q is not applicable for SINK", code.text)
		}
	}
	ret = &output{}
	switch val := sink.(type) {
	case *Encoder:
		if val == nil {
			return nil, errors.New("no encoder found")
		}
		var writer io.Writer = node.runtime.OutputWriter()
		// check cache option
		if node.task.cacheWriterOverride != nil && val.cacheOption != nil {
			ret.cacheOption = val.cacheOption
			ret.cacheWriter = node.task.cacheWriterOverride
			writer = io.MultiWriter(ret.cacheWriter)
		} else if cache := tqlResultCache.Load(); val.cacheOption != nil && cache != nil {
			ret.cacheOption = val.cacheOption
			if item := cache.Get(val.cacheOption.key); item != nil {
				// get cached data
				ret.cachedData = item.Data
				// check preemptive update is set and valid
				if preemptiveUpdateRatio := val.cacheOption.preemptiveUpdate; preemptiveUpdateRatio > 0 && preemptiveUpdateRatio < 1 {
					// check if the cache is required to be updated in advance
					preemptiveTTL := time.Duration(float64(item.TTL) * (1 - preemptiveUpdateRatio))
					preemptiveUpdateAt := item.ExpiresAt.Add(-1 * preemptiveTTL)
					if preemptiveUpdateAt.Before(time.Now()) {
						if u := item.updates.Add(1); u == 1 {
							// update cache
							ret.cacheWriter = &bytes.Buffer{}
							writer = io.MultiWriter(ret.cacheWriter)
						}
					}
				}
			} else {
				ret.cacheWriter = &bytes.Buffer{}
				writer = io.MultiWriter(ret.cacheWriter, node.runtime.OutputWriter())
			}
		}

		options := []opts.Option{
			opts.Logger(node.runtime),
			opts.OutputStream(writer),
			opts.ChartJson(node.runtime.ToJSONOutput()),
			opts.GeoMapJson(node.runtime.ToJSONOutput()),
			opts.VolatileFileWriter(node.runtime),
		}
		ret.encoder = val.RowEncoder(options...)
		if _, ok := ret.encoder.(opts.CanSetChartJson); ok {
			ret.isChart = true
		} else if _, ok := ret.encoder.(opts.CanSetGeoMapJson); ok {
			ret.isGeoMap = true
		}
		if enc, ok := ret.encoder.(interface {
			ExportParams(params map[string][]string)
		}); ok {
			enc.ExportParams(node.runtime.Params())
		}
	case DatabaseSink:
		ret.dbSink = val
	default:
		return nil, fmt.Errorf("type (%T) is not applicable for SINK", val)
	}
	ret.name = asNodeName(expr)
	ret.task = node.task
	ret.tqlLine = code
	return ret, nil
}

func (out *output) Consume(input <-chan *Record) <-chan struct{} {
	if out.runtime == nil {
		out.runtime = newExecutionRuntime(out.task, out.task.currentExecution())
	}
	out.input = input
	done := make(chan struct{})
	go func() {
		out.runtime.StartStage(out.index, out.name, StageSink, 0)
		defer func() {
			close(done)
			out.runtime.FinishStage(out.index)
			if r := recover(); r != nil {
				w := &bytes.Buffer{}
				w.Write(debug.Stack())
				out.runtime.LogErrorf("panic %s %v\n%s", out.name, r, w.String())
				out.runtime.Fail(fmt.Errorf("sink %s panic: %v", out.name, r))
			}
		}()

		shouldClose := false
		saneEncoder := true
	loop:
		for {
			select {
			case <-out.runtime.Context().Done():
				out.runtime.fireCircuitBreak(nil)
				// task has been cancelled.
				break loop
			case rec, ok := <-out.input:
				if !ok || rec == nil {
					break loop
				} else if rec.IsError() {
					out.runtime.AddInput(out.index)
					out.runtime.execution.addStageError(out.index, rec.Error())
					out.lastError = rec.Error()
					continue
				}
				out.runtime.AddInput(out.index)
				if !shouldClose && saneEncoder {
					resultColumns := out.runtime.ResultColumns()
					if len(resultColumns) == 0 {
						arr := rec.Flatten()
						for i, v := range arr {
							resultColumns = append(resultColumns,
								&client.Column{
									Name:     fmt.Sprintf("column%d", i-1),
									DataType: api.DataTypeOf(v),
								})
						}
					}
					out.runtime.SetResultColumns(resultColumns)
					out.setHeader(resultColumns[1:])
					out.runtime.FinalizeSchema()
					if err := out.openEncoder(); err == nil {
						// success to open sink encoder
						shouldClose = true
						saneEncoder = true
					} else {
						// fail to open sink encoder
						out.lastError = err
						out.runtime.LogError(err.Error())
						out.runtime.execution.addStageError(out.index, err)
						out.runtime.Fail(err)
						saneEncoder = false
					}
				}
				if !saneEncoder {
					continue
				}
				if rec.IsArray() {
					for _, v := range rec.Array() {
						if err := out.addRow(v); err != nil {
							out.runtime.LogError(err.Error())
							out.runtime.ReportError(out.index, err)
						}
					}
				} else if rec.IsTuple() {
					if err := out.addRow(rec); err != nil {
						out.runtime.LogError(err.Error())
						out.runtime.ReportError(out.index, err)
					}
				} else if rec.IsImage() {
					if err := out.addRow(rec); err != nil {
						out.runtime.LogError(err.Error())
						out.runtime.ReportError(out.index, err)
					}
				}
			}
		}
		if saneEncoder {
			if shouldClose {
				out.closeEncoder()
			} else {
				// encoder has not been opened, which means no records are produced
				if resultColumns := out.runtime.ResultColumns(); len(resultColumns) > 0 {
					out.runtime.SetResultColumns(resultColumns)
					out.setHeader(resultColumns[1:])
					out.runtime.FinalizeSchema()
					if err := out.openEncoder(); err == nil {
						out.closeEncoder()
					} else {
						out.runtime.LogError(err.Error())
					}
				}
			}
		}

		if cache := tqlResultCache.Load(); out.cacheOption != nil && out.cacheOption.key != "" && out.cacheWriter != nil && cache != nil {
			if data := out.cacheWriter.Bytes(); len(data) > 0 {
				cache.Set(out.cacheOption.key, data, out.cacheOption.ttl)
			}
		}
	}()
	return done
}

func (out *output) Name() string {
	return out.name
}

func (out *output) Receive(rec *Record) {
	panic("output cannot receive records directly")
}

func (out *output) setHeader(cols client.Columns) {
	if out.encoder != nil {
		codec.SetEncoderColumns(out.encoder, cols)
	}
}

func (out *output) ContentType() string {
	if out.encoder != nil {
		return out.encoder.ContentType()
	} else if out.dbSink != nil {
		return "application/json"
	}
	return "application/octet-stream"
}

func (out *output) Metadata() OutputMetadata {
	contentEncoding := out.ContentEncoding()
	if contentEncoding == "" {
		contentEncoding = "identity"
	}
	metadata := OutputMetadata{
		ContentType:     out.ContentType(),
		ContentEncoding: contentEncoding,
		HttpHeaders:     out.HttpHeaders(),
	}
	if out.IsChart() {
		metadata.ChartType = "echarts"
	} else if out.IsGeoMap() {
		metadata.ChartType = "geomap"
	}
	return cloneOutputMetadata(metadata)
}

func (out *output) HttpHeaders() map[string][]string {
	if out.encoder != nil {
		return out.encoder.HttpHeaders()
	} else if out.dbSink != nil {
		return nil
	}
	return nil
}

func (out *output) IsChart() bool {
	return out.isChart
}

func (out *output) IsGeoMap() bool {
	return out.isGeoMap
}

func (out *output) ContentEncoding() string {
	//ex: return "gzip" for  Content-Encoding: gzip
	return ""
}

func (out *output) openEncoder() error {
	if out.encoder != nil {
		return out.encoder.Open()
	} else if out.dbSink != nil {
		return out.dbSink.Open(out.runtime)
	} else {
		return errors.New("no output encoder")
	}
}

func (out *output) closeEncoder() {
	if out.encoder != nil {
		out.encoder.Close()
	} else if out.dbSink != nil {
		resultMessage, err := out.dbSink.Close()
		if out.lastError == nil && err != nil {
			out.lastError = err
		}
		out.lastMessage = resultMessage
	}
}

func (out *output) addRow(rec *Record) error {
	var addFunc func([]any) error
	if out.encoder != nil {
		addFunc = out.encoder.AddRow
	} else if out.dbSink != nil {
		addFunc = out.dbSink.AddRow
	} else {
		return fmt.Errorf("%s has no destination", out.name)
	}

	if rec.IsArray() {
		for _, r := range rec.Array() {
			out.addRow(r)
		}
		return nil
	} else if rec.IsImage() && rec.Value() != nil {
		value := rec.Value()
		if raw, ok := value.([]byte); ok {
			return addFunc([]any{rec.contentType, raw})
		} else {
			return fmt.Errorf("%s can not write invalid image data (%T)", out.name, value)
		}
	} else if !rec.IsTuple() {
		return fmt.Errorf("%s can not write %v", out.name, rec)
	}

	if value := rec.Value(); value != nil {
		switch v := value.(type) {
		case [][]any:
			var err error
			for n := range v {
				err = addFunc(v[n])
				if err != nil {
					break
				}
			}
			return err
		case []any:
			return addFunc(v)
		case any:
			return addFunc([]any{v})
		}
	}
	return nil
}
