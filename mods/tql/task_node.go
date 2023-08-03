package tql

import (
	"fmt"
	"sync"

	"github.com/machbase/neo-server/mods/expression"
)

type Node struct {
	Name string
	Expr *expression.Expression
	Src  chan *Record
	Sink chan<- any
	Next *Node
	Nrow int

	task      *Task
	functions map[string]expression.Function
	values    map[string]any
	buffer    map[any][]any
	Debug     bool

	closeWg sync.WaitGroup
	closers []Closer
	mutex   sync.Mutex

	currentRecord *Record
}

var _ expression.Parameters = &Node{}

type Closer interface {
	Close() error
}

func (node *Node) SetRecord(rec *Record) {
	node.currentRecord = rec
}

func (node *Node) Task() *Task {
	return node.task
}

func (node *Node) Function(name string) expression.Function {
	return node.functions[name]
}

func (node *Node) Record() *Record {
	return node.currentRecord
}

func (node *Node) Logf(format string, args ...any) {
	if node.task != nil {
		node.task.LogInfo("[INFO] "+format, args...)
	} else {
		fmt.Printf("[INFO] "+format+"\n", args...)
	}
}

func (node *Node) LogDebug(args ...string) {
	if !node.Debug {
		return
	}
	node.task.LogDebugString(args...)
}

func (node *Node) LogWarnf(format string, args ...any) {
	if node.task != nil {
		node.task.LogInfo("[WARN] "+format, args...)
	} else {
		fmt.Printf("[WARN] "+format+"\n", args...)
	}
}

func (node *Node) LogErrorf(format string, args ...any) {
	if node.task != nil {
		node.task.LogError(format, args...)
	} else {
		fmt.Printf("[ERROR] "+format+"\n", args...)
	}
}

func (node *Node) Parse(text string) (*expression.Expression, error) {
	return expression.NewWithFunctions(text, node.functions)
}

// Get implements expression.Parameters
func (node *Node) Get(name string) (any, error) {
	switch name {
	case "K":
		if node.currentRecord != nil {
			return node.currentRecord.key, nil
		}
	case "V":
		if node.currentRecord != nil {
			return node.currentRecord.value, nil
		}
	case "CTX":
		return node, nil
	default:
		if node.task != nil {
			return node.task.Get(name)
		}
	}
	return nil, nil
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

func (node *Node) Buffer(key any, value any) {
	if node.buffer == nil {
		node.buffer = map[any][]any{}
	}
	if values, ok := node.buffer[key]; ok {
		node.buffer[key] = append(values, value)
	} else {
		node.buffer[key] = []any{value}
	}
}

func (node *Node) YieldBuffer(key any) {
	values, ok := node.buffer[key]
	if !ok {
		return
	}
	node.yield(key, values)
	delete(node.buffer, key)
}

func (node *Node) yield(key any, values []any) {
	if len(values) == 0 {
		return
	}
	if node.Next != nil {
		var yieldValue *Record
		if len(values) == 1 {
			yieldValue = NewRecord(key, values[0])
		} else {
			yieldValue = NewRecord(key, values)
		}
		if node.Debug {
			node.task.LogDebugString("++", node.Name, "-->", node.Next.Name, yieldValue.String(), " ")
		}
		node.Next.Src <- yieldValue
	} else {
		var yieldValue *Record
		if len(values) == 1 {
			yieldValue = NewRecord(key, values[0])
		} else {
			yieldValue = NewRecord(key, values)
		}
		if node.Debug {
			node.task.LogDebugString("++", node.Name, "==> SINK", yieldValue.String())
		}
		node.Sink <- yieldValue
	}
}

func (node *Node) start() {
	node.closeWg.Add(1)
	go func() {
		defer func() {
			if node.Next != nil {
				node.Next.Src <- EofRecord
			}
			node.Sink <- EofRecord
			node.closeWg.Done()
			if o := recover(); o != nil {
				node.task.LogError("panic %s %v", node.Name, o)
			}
		}()

		drop := func(p *Record) {
			node.LogDebug("--", node.Name, "DROP", fmt.Sprintf("%v", p.key), p.StringValueTypes(), " ")
		}

		for rec := range node.Src {
			if rec.IsEOF() {
				break
			}
			node.Nrow++
			node.SetRecord(rec)
			if ret, err := node.Expr.Eval(node); err != nil {
				node.Sink <- err
			} else if ret != nil {
				var resultset []*Record
				switch rs := ret.(type) {
				case *Record:
					if rs.IsEOF() {
						break
					} else if rs.IsCircuitBreak() {
						node.Sink <- rs
					} else {
						if rs != nil {
							resultset = []*Record{rs}
						}
					}
				case []*Record:
					resultset = rs
				default:
					node.Sink <- fmt.Errorf("func returns invalid type: %T (%s)", ret, node.Name)
				}

				for _, record := range resultset {
					node.yield(record.key, []any{record.value})
				}
			} else {
				drop(rec)
			}
		}

		for k, v := range node.buffer {
			node.yield(k, v)
		}
	}()
}

func (node *Node) Stop() {
	if node.Src != nil {
		node.closeWg.Wait()
		close(node.Src)
	}
	for i := len(node.closers) - 1; i >= 0; i-- {
		c := node.closers[i]
		if err := c.Close(); err != nil {
			node.task.LogError("context closer %s", err.Error())
		}
	}
}

func (node *Node) LazyClose(c Closer) {
	node.mutex.Lock()
	node.closers = append(node.closers, c)
	node.mutex.Unlock()
}

func (node *Node) CancelClose(c Closer) {
	node.mutex.Lock()
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
