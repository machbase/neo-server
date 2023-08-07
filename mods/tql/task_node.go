package tql

import (
	"bytes"
	"fmt"
	"runtime/debug"
	"sync"

	"github.com/machbase/neo-server/mods/expression"
	"github.com/pkg/errors"
)

type Closer interface {
	Close() error
}

type Node struct {
	task *Task
	name string
	next Receiver

	src  chan *Record
	expr *expression.Expression
	nrow int

	functions map[string]expression.Function
	values    map[string]any
	buffer    map[any][]any
	debug     bool

	closeWg sync.WaitGroup
	closers []Closer
	mutex   sync.Mutex

	inflight *Record
}

var (
	_ expression.Parameters = &Node{}
)

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

func (node *Node) Parse(text string) (*expression.Expression, error) {
	return expression.NewWithFunctions(text, node.functions)
}

func (node *Node) SetInflight(rec *Record) {
	node.inflight = rec
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
	var yieldRec *Record
	if len(values) == 0 {
		yieldRec = NewRecord(key, []any{})
	} else if len(values) == 1 {
		yieldRec = NewRecord(key, values[0])
	} else {
		yieldRec = NewRecord(key, values)
	}
	if node.debug {
		node.task.LogDebug("++", node.name, "-->", node.next.Name(), yieldRec.String(), " ")
	}
	yieldRec.Tell(node.next)
}

func (node *Node) start() {
	node.closeWg.Add(1)
	go func() {
		var lastWill *Record
		defer func() {
			if o := recover(); o != nil {
				w := &bytes.Buffer{}
				w.Write(debug.Stack())
				node.task.LogErrorf("panic %s inflight:%s %v\n%s", node.name, node.inflight.String(), o, w.String())
			}
		}()
	loop:
		for {
			select {
			case <-node.task.ctx.Done():
				// task has benn cancelled.
				break loop
			case rec := <-node.src:
				if rec.IsEOF() || rec.IsCircuitBreak() {
					lastWill = rec
					break loop
				} else if rec.IsError() {
					rec.Tell(node.next)
					continue
				} else if !node.task.shouldStop() {
					node.nrow++
					node.SetInflight(rec)
					if node.debug {
						node.task.LogDebug("->", node.Name(), "RECV", fmt.Sprintf("%v", rec.key), rec.StringValueTypes(), " ")
					}
					ret, err := node.expr.Eval(node)
					if err != nil {
						ErrorRecord(err).Tell(node.next)
						continue
					}
					if ret == nil {
						continue
					}

					to_next := func(rec *Record) bool {
						if rec == nil {
							return true
						}
						if rec.IsEOF() {
							rec.Tell(node.next)
							return false
						} else if rec.IsCircuitBreak() {
							node.task.onCircuitBreak(node)
							return false
						} else {
							rec.Tell(node.next)
							return true
						}
					}
					switch rs := ret.(type) {
					case *Record:
						to_next(rs)
					case []*Record:
						for _, rec := range rs {
							if alive := to_next(rec); !alive {
								break
							}
						}
					default:
						errRec := ErrorRecord(fmt.Errorf("func '%s' returns invalid type: %T", node.Name(), ret))
						errRec.Tell(node.next)
					}
				}
			}
		}
		for k, v := range node.buffer {
			node.yield(k, v)
		}
		if lastWill != nil {
			lastWill.Tell(node.next)
		}
		node.closeWg.Done()
	}()
}

func (node *Node) wait() {
	node.closeWg.Wait()
}

func (node *Node) stop() {
	if node.src != nil {
		close(node.src)
	}
	node.wait()
	for i := len(node.closers) - 1; i >= 0; i-- {
		c := node.closers[i]
		if err := c.Close(); err != nil {
			node.task.LogError(node.name, "context closer", err.Error())
		}
	}
}

func (node *Node) AddCloser(c Closer) {
	node.mutex.Lock()
	node.closers = append(node.closers, c)
	node.mutex.Unlock()
}

func (node *Node) CancelCloser(c Closer) {
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
