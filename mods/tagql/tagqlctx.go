package tagql

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/machbase/neo-server/mods/expression"
	"github.com/pkg/errors"
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
			return nil, errors.Wrapf(err, "at %s", strExpr)
		}
		exprs = append(exprs, expr)
	}

	nodes := make([]*ExecutionContext, len(exprs))
	for n, expr := range exprs {
		nodes[n] = &ExecutionContext{
			Name:    expr.String(),
			Context: ctxCtx,
			Expr:    expr,
			src:     make(chan *ExecutionParam),
			sink:    ret.r,
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
			ec.head.src <- &ExecutionParam{Ctx: ec.head, K: values[0], V: values[1:]}
		} else {
			ec.head.src <- ExecutionEOF
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
		case *ExecutionParam:
			if castV == ExecutionEOF {
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
		case []*ExecutionParam:
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

type ExecutionContext struct {
	context.Context
	Name string
	Expr *expression.Expression
	src  chan *ExecutionParam
	sink chan<- any
	Next *ExecutionContext

	values map[string]any
	buffer map[any][]any
	Debug  bool

	closeWg sync.WaitGroup
}

func (ctx *ExecutionContext) Get(name string) (any, bool) {
	if ctx.values == nil {
		return nil, false
	}
	ret, ok := ctx.values[name]
	return ret, ok
}

func (ctx *ExecutionContext) Set(name string, value any) {
	if ctx.values == nil {
		ctx.values = make(map[string]any)
	}
	ctx.values[name] = value
}

func (ctx *ExecutionContext) Buffer(key any, value any) {
	if ctx.buffer == nil {
		ctx.buffer = map[any][]any{}
	}
	if values, ok := ctx.buffer[key]; ok {
		ctx.buffer[key] = append(values, value)
	} else {
		ctx.buffer[key] = []any{value}
	}
}

func (ctx *ExecutionContext) YieldBuffer(key any) {
	values, ok := ctx.buffer[key]
	if !ok {
		return
	}
	ctx.yield(key, values)
	delete(ctx.buffer, key)
}

func (ctx *ExecutionContext) yield(key any, values []any) {
	if len(values) == 0 {
		return
	}
	var yieldValue *ExecutionParam
	if len(values) == 1 {
		yieldValue = &ExecutionParam{Ctx: ctx.Next, K: key, V: values[0]}
	} else {
		yieldValue = &ExecutionParam{Ctx: ctx.Next, K: key, V: values}
	}
	if ctx.Next != nil {
		if ctx.Debug {
			fmt.Println("++", ctx.Name, "-->", ctx.Next.Name, yieldValue.String())
		}
		ctx.Next.src <- yieldValue
	} else {
		if ctx.Debug {
			fmt.Println("++", ctx.Name, "==> SINK", yieldValue.String())
		}
		ctx.sink <- yieldValue
	}
}

func (ctx *ExecutionContext) Start() {
	ctx.closeWg.Add(1)
	go func() {
		defer func() {
			if ctx.Next != nil {
				ctx.Next.src <- ExecutionEOF
			}
			ctx.sink <- ExecutionEOF
			ctx.closeWg.Done()
			if o := recover(); o != nil {
				fmt.Println("panic", ctx.Name, o)
			}
		}()

		drop := func(p *ExecutionParam) {
			if ctx.Debug {
				fmt.Println("--", ctx.Name, "DROP", p.K, p.StringValueTypes())
			}
		}

		for p := range ctx.src {
			if p == ExecutionEOF {
				break
			}
			if ret, err := ctx.Expr.Eval(p); err != nil {
				ctx.sink <- err
			} else if ret != nil {
				var resultset []*ExecutionParam
				switch rs := ret.(type) {
				case *ExecutionParam:
					resultset = []*ExecutionParam{rs}
				case []*ExecutionParam:
					resultset = rs
				default:
					ctx.sink <- fmt.Errorf("func returns invalid type: %T (%s)", ret, ctx.Name)
				}

				for _, param := range resultset {
					ctx.yield(param.K, []any{param.V})
				}
			} else {
				drop(p)
			}
		}

		for k, v := range ctx.buffer {
			ctx.yield(k, v)
		}
	}()
}

func (ctx *ExecutionContext) Stop() {
	if ctx.src != nil {
		ctx.closeWg.Wait()
		close(ctx.src)
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

func (p *ExecutionParam) String() string {
	return fmt.Sprintf("K:%T(%v) V:%s", p.K, p.K, p.StringValueTypes())
}

func (p *ExecutionParam) StringValueTypes() string {
	if arr, ok := p.V.([]any); ok {
		return p.stringTypesOfArray(arr, 3)
	} else if arr, ok := p.V.([][]any); ok {
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
		return fmt.Sprintf("%T", p.V)
	}
}

func (p *ExecutionParam) stringTypesOfArray(arr []any, limit int) string {
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
	case []int:
		if rv, ok := other.K.([]int); !ok {
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
