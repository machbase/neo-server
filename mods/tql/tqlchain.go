package tql

import (
	"fmt"
	"runtime/debug"
	"sync"
	"time"

	"github.com/machbase/neo-server/mods/expression"
	spi "github.com/machbase/neo-spi"
)

type ExecutionChain struct {
	input  *input
	output *output
	db     spi.Database

	encoderNeedToClose bool

	nodes    []*SubContext
	headNode *SubContext
	nodesWg  sync.WaitGroup

	resultCh    chan any
	encoderCh   chan []any
	encoderChWg sync.WaitGroup

	closeOnce      sync.Once
	lastError      error
	circuitBreaker bool
}

func newExecutionChain(task *Task, db spi.Database, input *input, output *output, exprs []*expression.Expression) (*ExecutionChain, error) {
	ret := &ExecutionChain{}
	ret.resultCh = make(chan any)
	ret.encoderCh = make(chan []any)

	nodes := make([]*SubContext, len(exprs))
	for n, expr := range exprs {
		nodes[n] = &SubContext{
			Name:    expr.String(),
			Context: task.Context(),
			Expr:    expr,
			Src:     make(chan *Record),
			Sink:    ret.resultCh,
			Next:    nil,
			Params:  task.Params(),
		}
		if n > 0 {
			nodes[n-1].Next = nodes[n]
		}
	}
	ret.nodes = nodes
	ret.input = input
	ret.output = output
	ret.db = db

	if len(ret.nodes) > 0 {
		ret.headNode = ret.nodes[0]
	}
	return ret, nil
}

func (ec *ExecutionChain) Error() error {
	return ec.lastError
}

func (ec *ExecutionChain) sendToEncoder(values []any) {
	if len(values) > 0 {
		if t, ok := values[0].(*time.Time); ok {
			values[0] = *t
		}
		ec.encoderCh <- values
	}
}

func (ec *ExecutionChain) Run() error {
	ec.start()
	ec.wait()
	ec.stop()
	return ec.Error()
}

func (ec *ExecutionChain) columnTypeName(v any) string {
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

func (ec *ExecutionChain) start() {
	////////////////////////////////
	// encoder
	ec.encoderChWg.Add(1)
	var cols spi.Columns
	go func() {
		defer func() {
			if r := recover(); r != nil {
				if err, ok := r.(error); ok {
					fmt.Println("ERR", err.Error())
					debug.PrintStack()
				}
			}
			ec.encoderChWg.Done()
		}()
		for arr := range ec.encoderCh {
			if !ec.encoderNeedToClose {
				if len(cols) == 0 {
					for i, v := range arr {
						cols = append(cols, &spi.Column{
							Name: fmt.Sprintf("C%02d", i),
							Type: ec.columnTypeName(v)})
					}
				}
				ec.output.SetHeader(cols)
				ec.output.Open(ec.db)
				ec.encoderNeedToClose = true
			}
			if len(arr) == 0 {
				continue
			}
			if rec, ok := arr[0].(*Record); ok && rec.IsEOF() {
				continue
			}
			if err := ec.output.AddRow(arr); err != nil {
				fmt.Println("ERR", err.Error())
			}
		}
	}()

	////////////////////////////////
	// nodes
	for _, child := range ec.nodes {
		ec.nodesWg.Add(1)
		child.Start()
	}

	sink0 := func(k any, v any) {
		ec.sendToEncoder([]any{k, v})
	}

	sink1 := func(k any, v []any) {
		ec.sendToEncoder(append([]any{k}, v...))
	}

	sink2 := func(k any, v [][]any) {
		for _, row := range v {
			sink1(k, row)
		}
	}

	go func() {
		for ret := range ec.resultCh {
			switch castV := ret.(type) {
			case *Record:
				if castV.IsEOF() {
					ec.nodesWg.Done()
				} else if castV.IsCircuitBreak() {
					ec.circuitBreaker = true
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
				ec.lastError = castV
			}
		}
	}()

	////////////////////////////////
	// input source
	deligate := &InputDelegateWrapper{
		DatabaseFunc: func() spi.Database {
			return ec.db
		},
		ShouldStopFunc: func() bool {
			return ec.circuitBreaker || ec.lastError != nil
		},
		FeedHeaderFunc: func(c spi.Columns) {
			cols = c
		},
		FeedFunc: func(values []any) {
			if ec.headNode != nil {
				if values != nil {
					ec.headNode.Src <- ec.headNode.NewRecord(values[0], values[1:])
				} else {
					ec.headNode.Src <- ec.headNode.NewEOF()
				}
			} else {
				// there is no chain, just forward input data to sink directly
				ec.sendToEncoder(values)
			}
		},
	}
	if err := ec.input.Run(deligate); err != nil {
		ec.lastError = err
	}
}

func (ec *ExecutionChain) wait() {
	ec.nodesWg.Wait()
}

func (ec *ExecutionChain) stop() {
	ec.closeOnce.Do(func() {
		for _, ctx := range ec.nodes {
			ctx.Stop()
		}
		close(ec.resultCh)
		close(ec.encoderCh)
		ec.encoderChWg.Wait()
		if ec.output != nil && ec.encoderNeedToClose {
			ec.output.Close()
		}
	})
}
