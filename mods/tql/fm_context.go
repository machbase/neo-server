package tql

import (
	"fmt"
	"strconv"
)

type NodeContext struct {
	// Key  any
	node *Node
}

// tql function: context()
func (node *Node) GetContext() *NodeContext {
	return &NodeContext{
		node: node,
	}
}

// tql function: key()
func (node *Node) GetRecordKey() any {
	inflight := node.Inflight()
	if inflight == nil {
		return nil
	}
	return inflight.key
}

// tql function: value()
func (node *Node) GetRecordValue(args ...any) (any, error) {
	inflight := node.Inflight()
	if inflight == nil || inflight.value == nil {
		return nil, nil
	}
	if len(args) == 0 {
		return inflight.value, nil
	}
	idx := 0
	switch v := args[0].(type) {
	case int:
		idx = v
	case float32:
		idx = int(v)
	case float64:
		idx = int(v)
	case string:
		if parsed, err := strconv.ParseInt(v, 10, 32); err != nil {
			return nil, err
		} else {
			idx = int(parsed)
		}
	default:
		return nil, ErrWrongTypeOfArgs("value", 0, "index of value tuple", v)
	}
	switch val := inflight.value.(type) {
	case []any:
		if idx >= len(val) {
			return nil, ErrArgs("value", 0, fmt.Sprintf("%d is out of range of the value(len:%d)", idx, len(val)))
		}
		return val[idx], nil
	default:
		return nil, ErrArgs("value", 0, "out of index value tuple")
	}
}

// tql function: payload()
func (node *Node) GetRequestPayload() any {
	return node.task.inputReader
}

// tql function: param()
func (node *Node) GetRequestParam(name string) any {
	vals := node.task.params[name]
	if len(vals) == 1 {
		return vals[0]
	} else if len(vals) > 0 {
		return vals
	}
	return nil
}
