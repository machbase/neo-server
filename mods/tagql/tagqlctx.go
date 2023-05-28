package tagql

import (
	"context"
	"sync"
	"time"

	"github.com/machbase/neo-server/mods/expression"
	"github.com/machbase/neo-server/mods/tagql/ctx"
	"github.com/machbase/neo-server/mods/tagql/fmap"
	"github.com/pkg/errors"
)

type ExecutionChain struct {
	nodes     []*ctx.Context
	head      *ctx.Context
	r         chan any
	sink      chan []any
	closeOnce sync.Once
	waitWg    sync.WaitGroup
	lastError error
}

func NewExecutionChain(ctxCtx context.Context, exprstrs []string) (*ExecutionChain, error) {
	ret := &ExecutionChain{}
	ret.r = make(chan any)
	ret.sink = make(chan []any)

	exprs := []*expression.Expression{}
	for _, str := range exprstrs {
		expr, err := fmap.Parse(str)
		if err != nil {
			return nil, errors.Wrapf(err, "at %s", str)
		}
		exprs = append(exprs, expr)
	}

	nodes := make([]*ctx.Context, len(exprs))
	for n, expr := range exprs {
		nodes[n] = &ctx.Context{
			Name:    expr.String(),
			Context: ctxCtx,
			Expr:    expr,
			Src:     make(chan *ctx.Param),
			Sink:    ret.r,
			Next:    nil,
		}
		if n > 0 {
			nodes[n-1].Next = nodes[n]
		}
	}
	ret.nodes = nodes

	if len(ret.nodes) > 0 {
		ret.head = ret.nodes[0]
	}
	return ret, nil
}

func (ec *ExecutionChain) Error() error {
	return ec.lastError
}

func (ec *ExecutionChain) Source(values []any) {
	if ec.head != nil {
		if values != nil {
			ec.head.Src <- &ctx.Param{Ctx: ec.head, K: values[0], V: values[1:]}
		} else {
			ec.head.Src <- ctx.ExecutionEOF
		}
	} else {
		// there is no chain, just forward input data to sink directly
		ec.sendToSink(values)
	}
}

func (ec *ExecutionChain) sendToSink(values []any) {
	if len(values) > 0 {
		if t, ok := values[0].(*time.Time); ok {
			values[0] = *t
		}
		ec.sink <- values
	}
}

func (ec *ExecutionChain) Sink() <-chan []any {
	return ec.sink
}

func (ec *ExecutionChain) Start() {
	for _, child := range ec.nodes {
		ec.waitWg.Add(1)
		child.Start()
	}

	sink0 := func(k any, v any) {
		ec.sendToSink([]any{k, v})
	}

	sink1 := func(k any, v []any) {
		ec.sendToSink(append([]any{k}, v...))
	}

	sink2 := func(k any, v [][]any) {
		for _, row := range v {
			sink1(k, row)
		}
	}

	for ret := range ec.r {
		switch castV := ret.(type) {
		case *ctx.Param:
			if castV == ctx.ExecutionEOF {
				ec.waitWg.Done()
			} else {
				switch tV := castV.V.(type) {
				case []any:
					if subarr, ok := tV[0].([][]any); ok {
						sink2(castV.K, subarr)
					} else {
						sink1(castV.K, tV)
					}
				case [][]any:
					sink2(castV.K, tV)
				default:
					sink0(castV.K, castV.V)
				}
			}
		case []*ctx.Param:
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
}

func (ec *ExecutionChain) Wait() {
	ec.waitWg.Wait()
}

func (ec *ExecutionChain) Stop() {
	for _, ctx := range ec.nodes {
		ctx.Stop()
	}
	ec.closeOnce.Do(func() {
		close(ec.r)
		close(ec.sink)
	})
}
