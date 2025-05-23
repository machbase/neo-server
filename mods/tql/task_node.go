package tql

import (
	"bytes"
	"fmt"
	"math"
	"runtime/debug"
	"strings"
	"sync"

	"github.com/machbase/neo-server/v8/mods/tql/internal/expression"
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
	debug     bool

	closeWg sync.WaitGroup
	closers []Closer
	mutex   sync.Mutex

	_inflight *Record

	eofCallback func(*Node)

	pragma  map[string]string
	tqlLine *Line
}

var _ expression.Parameters = (*Node)(nil)

func (node *Node) compile(code string) error {
	expr, err := node.Parse(code)
	if err != nil {
		return fmt.Errorf("%s at %s", err.Error(), code)
	}
	if expr == nil {
		return fmt.Errorf("compile error at %s", code)
	}
	node.name = asNodeName(expr)
	node.expr = expr
	node.src = make(chan *Record)
	return nil
}

func (node *Node) Parse(text string) (*expression.Expression, error) {
	return expression.NewWithFunctions(text, node.functions)
}

func (node *Node) SetInflight(rec *Record) {
	node._inflight = rec
}

func (node *Node) Function(name string) expression.Function {
	return node.functions[name]
}

func (node *Node) Name() string {
	return node.name
}

func (node *Node) Inflight() *Record {
	return node._inflight
}

func (node *Node) Rownum() int {
	return node.nrow
}

func (node *Node) Receive(rec *Record) {
	select {
	case node.src <- rec:
	case <-node.task.ctx.Done():
		node.task.Cancel()
	}
}

func (node *Node) SetEOF(f func(*Node)) {
	node.eofCallback = f
}

func (node *Node) Pragma(name string) (string, bool) {
	if node.pragma != nil {
		if v, ok := node.pragma[name]; ok {
			return v, true
		}
	}
	return "", false
}

func (node *Node) PragmaBool(name string) bool {
	if node.pragma != nil {
		if v, ok := node.pragma[name]; ok {
			if v == "" || v == "1" || strings.ToLower(v) == "true" {
				return true
			}
		}
	}
	return false
}

// Get implements expression.Parameters
func (node *Node) Get(name string) (any, error) {
	switch name {
	case "PI":
		return math.Pi, nil
	case "nil", "NULL":
		return expression.NullValue, nil
	default:
		inflight := node.Inflight()
		if inflight == nil {
			return nil, nil
		}
		if node.Name() == "SET()" && !strings.HasPrefix(name, "$") {
			return func(v any) {
				inflight.SetVariable(name, v)
			}, nil
		} else {
			return inflight.GetVariable(name)
		}
	}
}

func (node *Node) fmSET(left any, right any) (any, error) {
	if left == nil {
		return node.Inflight(), nil
	}
	if fn, ok := left.(func(any)); ok {
		fn(right)
	} else {
		return nil, fmt.Errorf("%q left operand is not valid", "LET")
	}
	return node.Inflight(), nil
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

func (node *Node) DeleteValue(name string) {
	if node.values != nil {
		delete(node.values, name)
	}
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
		defer func() {
			node.closeWg.Done()
			if o := recover(); o != nil {
				w := &bytes.Buffer{}
				w.Write(debug.Stack())
				node.task.Log("panic", node.name, o, w.String())
				node.task.LogErrorf("panic %s %v\n%s", node.name, o, w.String())
			}
		}()
		var lastWill *Record
	loop:
		for {
			select {
			case <-node.task.ctx.Done():
				// task has benn cancelled.
				break loop
			case rec := <-node.src:
				if rec == nil {
					// when chan is closed:
					// while record.Tell() is called the ctx is done
					break loop
				} else if rec.IsEOF() || rec.IsCircuitBreak() {
					lastWill = rec
					break loop
				} else if rec.IsError() {
					rec.Tell(node.next)
					continue
				} else { // else if !node.task.shouldStop() <- do not use shouldStop() : https://github.com/machbase/neo/issues/309
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
							node.task.fireCircuitBreak(node)
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
		if lastWill != nil {
			if node.eofCallback != nil {
				node.eofCallback(node)
			}
			lastWill.Tell(node.next)
		}
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
