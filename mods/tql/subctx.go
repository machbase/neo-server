package tql

import (
	"context"
	"fmt"
	"runtime/debug"
	"sync"

	"github.com/machbase/neo-server/mods/expression"
)

func NewSubContext(x Task) *SubContext {
	return &SubContext{
		task: x,
	}
}

type SubContext struct {
	context.Context
	Name   string
	Expr   *expression.Expression
	Src    chan *Record
	Sink   chan<- any
	Next   *SubContext
	Params map[string][]string
	Nrow   int

	task   Task
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

func (ctx *SubContext) NewRecord(k, v any) *Record {
	return &Record{ctx: ctx, key: k, value: v}
}

func (ctx *SubContext) NewEOF() *Record {
	return &Record{ctx: ctx, eof: true}
}

func NewEOF() *Record {
	return &Record{eof: true}
}

func (ctx *SubContext) NewCircuitBreak() *Record {
	return &Record{ctx: ctx, circuitBreak: true}
}

func (ctx *SubContext) Get(name string) (any, bool) {
	if ctx.values == nil {
		return nil, false
	}
	ret, ok := ctx.values[name]
	return ret, ok
}

func (ctx *SubContext) Set(name string, value any) {
	if ctx.values == nil {
		ctx.values = make(map[string]any)
	}
	ctx.values[name] = value
}

func (ctx *SubContext) Buffer(key any, value any) {
	if ctx.buffer == nil {
		ctx.buffer = map[any][]any{}
	}
	if values, ok := ctx.buffer[key]; ok {
		ctx.buffer[key] = append(values, value)
	} else {
		ctx.buffer[key] = []any{value}
	}
}

func (ctx *SubContext) YieldBuffer(key any) {
	values, ok := ctx.buffer[key]
	if !ok {
		return
	}
	ctx.yield(key, values)
	delete(ctx.buffer, key)
}

func (ctx *SubContext) yield(key any, values []any) {
	if len(values) == 0 {
		return
	}
	if ctx.Next != nil {
		var yieldValue *Record
		if len(values) == 1 {
			yieldValue = ctx.Next.NewRecord(key, values[0])
		} else {
			yieldValue = ctx.Next.NewRecord(key, values)
		}
		if ctx.Debug {
			fmt.Println("++", ctx.Name, "-->", ctx.Next.Name, yieldValue.String())
		}
		ctx.Next.Src <- yieldValue
	} else {
		var yieldValue *Record
		if len(values) == 1 {
			yieldValue = ctx.NewRecord(key, values[0])
		} else {
			yieldValue = ctx.NewRecord(key, values)
		}
		if ctx.Debug {
			fmt.Println("++", ctx.Name, "==> SINK", yieldValue.String())
		}
		ctx.Sink <- yieldValue
	}
}

func (ctx *SubContext) Start() {
	ctx.closeWg.Add(1)
	go func() {
		defer func() {
			if ctx.Next != nil {
				ctx.Next.Src <- ctx.NewEOF()
			}
			ctx.Sink <- ctx.NewEOF()
			ctx.closeWg.Done()
			if o := recover(); o != nil {
				fmt.Println("panic", ctx.Name, o)
				debug.PrintStack()
			}
		}()

		drop := func(p *Record) {
			if ctx.Debug {
				fmt.Println("--", ctx.Name, "DROP", p.key, p.StringValueTypes())
			}
		}

		for p := range ctx.Src {
			if p.IsEOF() {
				break
			}
			ctx.Nrow++
			if ret, err := ctx.Expr.Eval(p); err != nil {
				ctx.Sink <- err
			} else if ret != nil {
				var resultset []*Record
				switch rs := ret.(type) {
				case *Record:
					if rs.IsEOF() {
						break
					} else if rs.IsCircuitBreak() {
						ctx.Sink <- rs
					} else {
						if rs != nil {
							resultset = []*Record{rs}
						}
					}
				case []*Record:
					resultset = rs
				default:
					ctx.Sink <- fmt.Errorf("func returns invalid type: %T (%s)", ret, ctx.Name)
				}

				for _, record := range resultset {
					ctx.yield(record.key, []any{record.value})
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

func (ctx *SubContext) Stop() {
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

func (ctx *SubContext) LazyClose(c Closer) {
	ctx.mutex.Lock()
	ctx.closers = append(ctx.closers, c)
	ctx.mutex.Unlock()
}

func (ctx *SubContext) CancelClose(c Closer) {
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
