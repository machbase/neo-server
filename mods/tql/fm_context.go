package tql

import (
	"fmt"
	"net/url"
	"strconv"
	"time"
)

type NodeContext struct {
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
	switch v := inflight.key.(type) {
	case *int:
		return *v
	case *int8:
		return *v
	case *int16:
		return *v
	case *int32:
		return *v
	case *int64:
		return *v
	case *float32:
		return *v
	case *float64:
		return *v
	case *string:
		return *v
	case *time.Time:
		return *v
	case *bool:
		return *v
	default:
		return v
	}
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
		switch v := val[idx].(type) {
		case *int:
			return *v, nil
		case *int8:
			return *v, nil
		case *int16:
			return *v, nil
		case *int32:
			return *v, nil
		case *int64:
			return *v, nil
		case *float32:
			return *v, nil
		case *float64:
			return *v, nil
		case *string:
			return *v, nil
		case *time.Time:
			return *v, nil
		case *bool:
			return *v, nil
		default:
			return val[idx], nil
		}
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

// tql function: escapeParam()
func (node *Node) EscapeParam(str string) any {
	return url.QueryEscape(str)
}
