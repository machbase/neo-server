package context

import (
	gocontext "context"
	"fmt"
	"sync"

	"github.com/machbase/neo-server/mods/expression"
)

type Context struct {
	gocontext.Context
	Name   string
	Expr   *expression.Expression
	Src    chan *Param
	Sink   chan<- any
	Next   *Context
	Params map[string][]string
	Nrow   int

	values map[string]any
	buffer map[any][]any
	Debug  bool

	closeWg sync.WaitGroup
	closers []Closer
	mutex   sync.Mutex
}

type Closer interface {
	Close() error
}

func (ctx *Context) Get(name string) (any, bool) {
	if ctx.values == nil {
		return nil, false
	}
	ret, ok := ctx.values[name]
	return ret, ok
}

func (ctx *Context) Set(name string, value any) {
	if ctx.values == nil {
		ctx.values = make(map[string]any)
	}
	ctx.values[name] = value
}

func (ctx *Context) Buffer(key any, value any) {
	if ctx.buffer == nil {
		ctx.buffer = map[any][]any{}
	}
	if values, ok := ctx.buffer[key]; ok {
		ctx.buffer[key] = append(values, value)
	} else {
		ctx.buffer[key] = []any{value}
	}
}

func (ctx *Context) YieldBuffer(key any) {
	values, ok := ctx.buffer[key]
	if !ok {
		return
	}
	ctx.yield(key, values)
	delete(ctx.buffer, key)
}

func (ctx *Context) yield(key any, values []any) {
	if len(values) == 0 {
		return
	}
	var yieldValue *Param
	if len(values) == 1 {
		yieldValue = &Param{Ctx: ctx.Next, K: key, V: values[0]}
	} else {
		yieldValue = &Param{Ctx: ctx.Next, K: key, V: values}
	}
	if ctx.Next != nil {
		if ctx.Debug {
			fmt.Println("++", ctx.Name, "-->", ctx.Next.Name, yieldValue.String())
		}
		ctx.Next.Src <- yieldValue
	} else {
		if ctx.Debug {
			fmt.Println("++", ctx.Name, "==> SINK", yieldValue.String())
		}
		ctx.Sink <- yieldValue
	}
}

func (ctx *Context) Start() {
	ctx.closeWg.Add(1)
	go func() {
		defer func() {
			if ctx.Next != nil {
				ctx.Next.Src <- ExecutionEOF
			}
			ctx.Sink <- ExecutionEOF
			ctx.closeWg.Done()
			if o := recover(); o != nil {
				fmt.Println("panic", ctx.Name, o)
			}
		}()

		drop := func(p *Param) {
			if ctx.Debug {
				fmt.Println("--", ctx.Name, "DROP", p.K, p.StringValueTypes())
			}
		}

		for p := range ctx.Src {
			if p == ExecutionEOF {
				break
			}
			ctx.Nrow++
			if ret, err := ctx.Expr.Eval(p); err != nil {
				ctx.Sink <- err
			} else if ret != nil {
				var resultset []*Param
				switch rs := ret.(type) {
				case *Param:
					if rs == ExecutionEOF {
						break
					} else if rs == ExecutionCircuitBreak {
						ctx.Sink <- ExecutionCircuitBreak
					} else {
						resultset = []*Param{rs}
					}
				case []*Param:
					resultset = rs
				default:
					ctx.Sink <- fmt.Errorf("func returns invalid type: %T (%s)", ret, ctx.Name)
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

func (ctx *Context) Stop() {
	if ctx.Src != nil {
		ctx.closeWg.Wait()
		close(ctx.Src)
	}
	for i := len(ctx.closers) - 1; i >= 0; i-- {
		c := ctx.closers[i]
		if err := c.Close(); err != nil {
			fmt.Println("ERR context closer", err.Error())
		}
	}
}

func (ctx *Context) LazyClose(c Closer) {
	ctx.mutex.Lock()
	ctx.closers = append(ctx.closers, c)
	ctx.mutex.Unlock()
}

func (ctx *Context) CancelClose(c Closer) {
	ctx.mutex.Lock()
	idx := -1
	for i, cl := range ctx.closers {
		if c == cl {
			idx = i
			break
		}
	}
	if idx >= 0 {
		ctx.closers = append(ctx.closers[:idx], ctx.closers[idx+1:]...)
	}
	ctx.mutex.Unlock()
}
