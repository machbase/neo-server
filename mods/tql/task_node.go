package tql

import (
	"fmt"
	"sync"

	"github.com/machbase/neo-server/mods/expression"
	"github.com/pkg/errors"
)

type Node struct {
	log TaskLog

	name string
	expr *expression.Expression
	src  chan *Record
	next Receiver
	nrow int

	task      *Task
	functions map[string]expression.Function
	values    map[string]any
	buffer    map[any][]any
	Debug     bool

	closeWg sync.WaitGroup
	closers []Closer
	mutex   sync.Mutex

	inflight *Record
}

var _ expression.Parameters = &Node{}

type Closer interface {
	Close() error
}

func (node *Node) compile(code string) error {
	expr, err := node.Parse(code)
	if err != nil {
		return errors.Wrapf(err, "at %s", code)
	}
	if expr == nil {
		return fmt.Errorf("compile error at %s", code)
	}
	node.name = expr.String()
	node.expr = expr
	node.src = make(chan *Record)
	return nil
}

func (node *Node) SetInflight(rec *Record) {
	node.inflight = rec
}

func (node *Node) Task() *Task {
	return node.task
}

func (node *Node) Function(name string) expression.Function {
	return node.functions[name]
}

func (node *Node) Name() string {
	return node.name
}

func (node *Node) Inflight() *Record {
	return node.inflight
}

func (node *Node) Receive(rec *Record) {
	node.src <- rec
}

func (node *Node) Parse(text string) (*expression.Expression, error) {
	return expression.NewWithFunctions(text, node.functions)
}

// Get implements expression.Parameters
func (node *Node) Get(name string) (any, error) {
	switch name {
	case "K":
		if node.inflight != nil {
			return node.inflight.key, nil
		}
	case "V":
		if node.inflight != nil {
			return node.inflight.value, nil
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
	var yieldValue *Record
	if len(values) == 1 {
		yieldValue = NewRecord(key, values[0])
	} else {
		yieldValue = NewRecord(key, values)
	}
	if node.Debug {
		node.log.LogDebug("++", node.Name(), "-->", node.next.Name(), yieldValue.String(), " ")
	}
	yieldValue.Tell(node.next)
}

func (node *Node) start() {
	node.closeWg.Add(1)
	go func() {
		defer func() {
			EofRecord.Tell(node.next)
			node.closeWg.Done()
			if o := recover(); o != nil {
				node.task.LogError("panic %s %v", node.Name, o)
			}
		}()

		drop := func(p *Record) {
			if node.Debug {
				node.log.LogDebug("--", node.Name(), "DROP", fmt.Sprintf("%v", p.key), p.StringValueTypes(), " ")
			}
		}

		for rec := range node.src {
			if rec.IsEOF() {
				break
			}
			node.nrow++
			node.SetInflight(rec)
			if ret, err := node.expr.Eval(node); err != nil {
				ErrorRecord(err).Tell(node.next)
			} else {
				if ret == nil {
					drop(rec)
					continue
				}
				switch rs := ret.(type) {
				case *Record:
					if rs.IsEOF() || rs.IsCircuitBreak() {
						rs.Tell(node.next)
						break
					} else if rs.IsError() {
						rs.Tell(node.next)
					} else {
						rs.Tell(node.next)
					}
				case []*Record:
					ArrayRecord(rs).Tell(node.next)
				default:
					errRec := ErrorRecord(fmt.Errorf("func returns invalid type: %T (%s)", ret, node.Name()))
					errRec.Tell(node.next)
				}
			}
		}

		for k, v := range node.buffer {
			node.yield(k, v)
		}
	}()
}

func (node *Node) stop() {
	if node.src != nil {
		node.closeWg.Wait()
		close(node.src)
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
