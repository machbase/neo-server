package tagql

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/machbase/neo-server/mods/expression"
)

type ExecutionChain struct {
	nodes     []*ExecutionContext
	head      *ExecutionContext
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
		strExpr := normalizeMapFuncExpr(str)
		expr, err := expression.NewWithFunctions(strExpr, mapFunctions)
		if err != nil {
			return nil, err
		}
		exprs = append(exprs, expr)
	}

	nodes := make([]*ExecutionContext, len(exprs))
	for n, expr := range exprs {
		nodes[n] = &ExecutionContext{
			Name:    expr.String(),
			Context: ctxCtx,
			Expr:    expr,
			C:       make(chan *ExecutionParam),
			R:       ret.r,
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
			ec.head.C <- &ExecutionParam{Ctx: ec.head, K: values[0], V: values[1:]}
		} else {
			ec.head.C <- ExecutionEOF
		}
	} else {
		if values != nil {
			ec.sink <- values
		}
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

	for ret := range ec.r {
		switch castV := ret.(type) {
		case *ExecutionParam:
			if castV == ExecutionEOF {
				ec.waitWg.Done()
			} else {
				switch tV := castV.V.(type) {
				case []any:
					if subarr, ok := tV[0].([][]any); ok {
						for _, subitm := range subarr {
							fields := append([]any{castV.K}, subitm...)
							ec.sink <- fields
						}
					} else {
						fields := append([]any{castV.K}, tV...)
						ec.sink <- fields
					}
				case [][]any:
					for _, row := range tV {
						fields := append([]any{castV.K}, row...)
						ec.sink <- fields
					}
				}
			}
		case []*ExecutionParam:
			for _, v := range castV {
				switch tV := v.V.(type) {
				case []any:
					fields := append([]any{v.K}, tV...)
					ec.sink <- fields
				case [][]any:
					for _, row := range tV {
						fields := append([]any{v.K}, row...)
						ec.sink <- fields
					}
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

type ExecutionContext struct {
	context.Context
	Name string
	Expr *expression.Expression
	C    chan *ExecutionParam
	R    chan<- any
	Next *ExecutionContext

	closeWg sync.WaitGroup
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
