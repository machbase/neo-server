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
	return unboxValue(inflight.key)
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
		return unboxValue(val[idx]), nil
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

// tql function: ARGS()
func (node *Node) fmArgs() (any, error) {
	data, err := node.fmArgsParam()
	if err != nil {
		return nil, err
	}
	genRawData(node, &rawdata{data: data})
	return nil, nil
}

// tql function: args()
func (node *Node) fmArgsParam(args ...any) (any, error) {
	argValues := node.task.argValues
	if len(argValues) == 0 {
		return nil, nil
	}
	var ret any

	if len(args) == 0 {
		ret = argValues
	} else {
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
			return nil, ErrWrongTypeOfArgs("arg", 0, "index of value tuple", v)
		}
		if idx >= len(argValues) {
			return nil, ErrArgs("arg", 0, fmt.Sprintf("%d is out of range of the arg(len:%d)", idx, len(argValues)))
		}
		ret = unboxValue(argValues[idx])
	}

	if node.Name() == "FAKE()" {
		return &rawdata{data: ret}, nil
	} else {
		return ret, nil
	}
}

// tql function: escapeParam()
func (node *Node) EscapeParam(str string) any {
	return url.QueryEscape(str)
}

func unboxValue(val any) any {
	switch v := val.(type) {
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
		return val
	}
}
