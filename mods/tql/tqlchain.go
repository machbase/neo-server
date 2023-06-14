package tql

import (
	gocontext "context"
	"fmt"
	"sync"
	"time"

	"github.com/machbase/neo-server/mods/expression"
	"github.com/machbase/neo-server/mods/tql/context"
	"github.com/machbase/neo-server/mods/tql/fsink"
	"github.com/machbase/neo-server/mods/tql/fsrc"
	spi "github.com/machbase/neo-spi"
)

type ExecutionChain struct {
	input  fsrc.Input
	output fsink.Output
	db     spi.Database

	encoderNeedToClose bool

	nodes    []*context.Context
	headNode *context.Context
	nodesWg  sync.WaitGroup

	resultCh    chan any
	encoderCh   chan []any
	encoderChWg sync.WaitGroup

	closeOnce      sync.Once
	lastError      error
	circuitBreaker bool
}

func newExecutionChain(ctxCtx gocontext.Context, db spi.Database, input fsrc.Input, output fsink.Output, exprs []*expression.Expression, params map[string][]string) (*ExecutionChain, error) {
	ret := &ExecutionChain{}
	ret.resultCh = make(chan any)
	ret.encoderCh = make(chan []any)

	nodes := make([]*context.Context, len(exprs))
	for n, expr := range exprs {
		nodes[n] = &context.Context{
			Name:    expr.String(),
			Context: ctxCtx,
			Expr:    expr,
			Src:     make(chan *context.Param),
			Sink:    ret.resultCh,
			Next:    nil,
			Params:  params,
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
			if err := ec.output.AddRow(arr); err != nil {
				fmt.Println("ERR", err.Error())
			}
		}
		ec.encoderChWg.Done()
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
			case *context.Param:
				if castV == context.ExecutionEOF {
					ec.nodesWg.Done()
				} else if castV == context.ExecutionCircuitBreak {
					ec.circuitBreaker = true
				} else {
					switch tV := castV.V.(type) {
					case []any:
						if len(tV) == 0 {
							sink1(castV.K, tV)
						} else {
							if subarr, ok := tV[0].([][]any); ok {
								sink2(castV.K, subarr)
							} else {
								sink1(castV.K, tV)
							}
						}
					case [][]any:
						sink2(castV.K, tV)
					default:
						sink0(castV.K, castV.V)
					}
				}
			case []*context.Param:
				for _, v := range castV {
					switch tV := v.V.(type) {
					case []any:
						sink1(v.K, tV)
					case [][]any:
						sink2(v.K, tV)
					default:
						sink0(v.K, tV)
					}
				}
			case error:
				ec.lastError = castV
			}
		}
	}()

	////////////////////////////////
	// input source
	deligate := &fsrc.InputDelegateWrapper{
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
					ec.headNode.Src <- &context.Param{Ctx: ec.headNode, K: values[0], V: values[1:]}
				} else {
					ec.headNode.Src <- context.ExecutionEOF
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
