package tagql

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/machbase/neo-server/mods/codec"
	"github.com/machbase/neo-server/mods/expression"
	spi "github.com/machbase/neo-spi"
)

type ExecutionChain struct {
	R <-chan any

	encoder   codec.RowsEncoder
	chain     []*ExecutionContext
	head      *ExecutionContext
	r         chan any
	closeOnce sync.Once
	waitWg    sync.WaitGroup
}

func NewExecutionChain(ctxCtx context.Context, exprs []*expression.Expression, encoder codec.RowsEncoder) *ExecutionChain {
	ret := &ExecutionChain{}
	ret.r = make(chan any)
	ret.R = ret.r
	ret.encoder = encoder
	ret.chain = NewContextChain(ctxCtx, exprs, ret.r)
	if len(ret.chain) > 0 {
		ret.head = ret.chain[0]
	}
	ret.waitWg.Add(len(ret.chain))
	return ret
}

func (ec *ExecutionChain) Done() {
	ec.waitWg.Done()
}

func (ec *ExecutionChain) Wait() {
	ec.waitWg.Wait()
}

func (ec *ExecutionChain) Stop() {
	for _, ctx := range ec.chain {
		ctx.Stop()
	}
}

func (ec *ExecutionChain) Open(cols spi.Columns) {
	if ec.encoder != nil {
		ec.encoder.Open(cols)
	}
}

func (ec *ExecutionChain) Close() {
	ec.closeOnce.Do(func() {
		close(ec.r)
	})
	if ec.encoder != nil {
		ec.encoder.Close()
	}
}

func (ec *ExecutionChain) Source(values []any) {
	if ec.head != nil {
		if values != nil {
			ec.head.C <- &ExecutionParam{Ctx: ec.head, K: values[0], V: values[1:]}
		} else {
			ec.head.C <- ExecutionEOF
		}
	} else {
		if values != nil {
			ec.encoder.AddRow(values)
		} else {
			ec.encoder.Close()
		}
	}
}

type ExecutionContext struct {
	context.Context
	Name string
	Expr *expression.Expression
	C    chan *ExecutionParam
	R    chan<- any
	Next *ExecutionContext

	closeWg sync.WaitGroup
}

func NewContextChain(ctxCtx context.Context, exprs []*expression.Expression, r chan<- any) []*ExecutionContext {
	ctxArr := make([]*ExecutionContext, len(exprs))
	for n, expr := range exprs {
		ctxArr[n] = &ExecutionContext{
			Name:    expr.String(),
			Context: ctxCtx,
			Expr:    expr,
			C:       make(chan *ExecutionParam),
			R:       r,
			Next:    nil,
		}
		if n > 0 {
			ctxArr[n-1].Next = ctxArr[n]
		}
		ctxArr[n].Start()
	}
	return ctxArr
}

func (ctx *ExecutionContext) Start() {
	ctx.closeWg.Add(1)
	go func() {
		var curKey any
		var curVal []any

		for p := range ctx.C {
			if p == ExecutionEOF {
				break
			}
			if ret, err := ctx.Expr.Eval(p); err != nil {
				ctx.R <- err
			} else if ret != nil {
				var resultset []*ExecutionParam
				switch rs := ret.(type) {
				case *ExecutionParam:
					resultset = []*ExecutionParam{rs}
				case []*ExecutionParam:
					resultset = rs
				default:
					ctx.R <- fmt.Errorf("func returns invalid type: %T (%s)", ret, ctx.Name)
				}

				for _, param := range resultset {
					if curKey == nil {
						curKey = param.K
						curVal = []any{}
					}
					if curKey == param.K {
						// aggregate
						curVal = append(curVal, param.V)
					} else {
						yieldValue := &ExecutionParam{Ctx: ctx, K: curKey, V: curVal}
						if ctx.Next != nil {
							ctx.Next.C <- yieldValue
						} else {
							ctx.R <- yieldValue
						}
						curKey = param.K
						curVal = []any{param.V}
					}
				}
			}
		}
		if curKey != nil && len(curVal) > 0 {
			if ctx.Next != nil {
				ctx.Next.C <- &ExecutionParam{Ctx: ctx, K: curKey, V: curVal}
			} else {
				ctx.R <- &ExecutionParam{Ctx: ctx, K: curKey, V: curVal}
			}
		}

		if ctx.Next != nil {
			ctx.Next.C <- ExecutionEOF
		}
		ctx.R <- ExecutionEOF

		ctx.closeWg.Done()
	}()
}

func (ctx *ExecutionContext) Stop() {
	if ctx.C != nil {
		ctx.closeWg.Wait()
		close(ctx.C)
	}
}

// ////////////////////////////
// PARAM
var ExecutionEOF = &ExecutionParam{}

type ExecutionParam struct {
	Ctx *ExecutionContext
	K   any
	V   any
}

func (p *ExecutionParam) Get(name string) (any, error) {
	if name == "K" || name == "k" {
		switch k := p.K.(type) {
		case *time.Time:
			return *k, nil
		default:
			return p.K, nil
		}
	} else if name == "V" || name == "v" {
		return p.V, nil
	} else if name == "P" || name == "p" {
		return p, nil
	} else if strings.ToLower(name) == "ctx" {
		return p.Ctx, nil
	} else {
		return nil, fmt.Errorf("parameter '%s' is not defined", name)
	}
}

func (p *ExecutionParam) EqualKey(other *ExecutionParam) bool {
	if other == nil {
		return false
	}
	switch lv := p.K.(type) {
	case time.Time:
		if rv, ok := other.K.(time.Time); !ok {
			return false
		} else {
			return lv.Nanosecond() == rv.Nanosecond()
		}
	}
	return p.K == other.K
}

func (p *ExecutionParam) EqualValue(other *ExecutionParam) bool {
	if other == nil {
		return false
	}
	lv := fmt.Sprintf("%#v", p.V)
	rv := fmt.Sprintf("%#v", other.V)
	// fmt.Println("lv", lv)
	// fmt.Println("rv", rv)
	return lv == rv
}
