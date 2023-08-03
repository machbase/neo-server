package tql

import (
	"fmt"
	"sync"

	"github.com/machbase/neo-server/mods/expression"
)

func NewNode(x *Task) *Node {
	return &Node{
		task: x,
	}
}

type Node struct {
	id   int
	Name string
	Expr *expression.Expression
	Src  chan *Record
	Sink chan<- any
	Next *Node
	Nrow int

	task   *Task
	values map[string]any
	buffer map[any][]any
	Debug  bool

	closeWg sync.WaitGroup
	closers []Closer
	mutex   sync.Mutex

	currentRecord *Record
}

var _ expression.Parameters = &Node{}

type Closer interface {
	Close() error
}

func (ctx *Node) NewRecord(k, v any) *Record {
	return &Record{key: k, value: v}
}

func (ctx *Node) SetRecord(rec *Record) {
	ctx.currentRecord = rec
}

func (ctx *Node) Record() *Record {
	return ctx.currentRecord
}

// Get implements expression.Parameters
func (ctx *Node) Get(name string) (any, error) {
	switch name {
	case "K":
		if ctx.currentRecord != nil {
			return ctx.currentRecord.key, nil
		}
	case "V":
		if ctx.currentRecord != nil {
			return ctx.currentRecord.value, nil
		}
	case "CTX":
		return ctx, nil
	default:
		if ctx.task != nil {
			return ctx.task.Get(name)
		}
	}
	return nil, nil
}

func (ctx *Node) GetValue(name string) (any, bool) {
	if ctx.values == nil {
		return nil, false
	}
	ret, ok := ctx.values[name]
	return ret, ok
}

func (ctx *Node) SetValue(name string, value any) {
	if ctx.values == nil {
		ctx.values = make(map[string]any)
	}
	ctx.values[name] = value
}

func (ctx *Node) Buffer(key any, value any) {
	if ctx.buffer == nil {
		ctx.buffer = map[any][]any{}
	}
	if values, ok := ctx.buffer[key]; ok {
		ctx.buffer[key] = append(values, value)
	} else {
		ctx.buffer[key] = []any{value}
	}
}

func (ctx *Node) YieldBuffer(key any) {
	values, ok := ctx.buffer[key]
	if !ok {
		return
	}
	ctx.yield(key, values)
	delete(ctx.buffer, key)
}

func (ctx *Node) yield(key any, values []any) {
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
			ctx.task.LogDebugString("++", ctx.Name, "-->", ctx.Next.Name, yieldValue.String(), " ")
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

func (ctx *Node) Start() {
	ctx.closeWg.Add(1)
	go func() {
		defer func() {
			if ctx.Next != nil {
				ctx.Next.Src <- EofRecord
			}
			ctx.Sink <- EofRecord
			ctx.closeWg.Done()
			if o := recover(); o != nil {
				ctx.task.LogError("panic %s %v", ctx.Name, o)
			}
		}()

		drop := func(p *Record) {
			if ctx.Debug {
				ctx.task.LogDebugString("--", ctx.Name, "DROP", fmt.Sprintf("%v", p.key), p.StringValueTypes(), " ")
			}
		}

		for rec := range ctx.Src {
			if rec.IsEOF() {
				break
			}
			ctx.Nrow++
			ctx.SetRecord(rec)
			if ret, err := ctx.Expr.Eval(ctx); err != nil {
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
				drop(rec)
			}
		}

		for k, v := range ctx.buffer {
			ctx.yield(k, v)
		}
	}()
}

func (ctx *Node) Stop() {
	if ctx.Src != nil {
		ctx.closeWg.Wait()
		close(ctx.Src)
	}
	for i := len(ctx.closers) - 1; i >= 0; i-- {
		c := ctx.closers[i]
		if err := c.Close(); err != nil {
			ctx.task.LogError("context closer %s", err.Error())
		}
	}
}

func (ctx *Node) LazyClose(c Closer) {
	ctx.mutex.Lock()
	ctx.closers = append(ctx.closers, c)
	ctx.mutex.Unlock()
}

func (ctx *Node) CancelClose(c Closer) {
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
