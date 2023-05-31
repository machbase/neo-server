package fcom

import (
	"fmt"
	"time"
)

func to_len(args ...any) (any, error) {
	if arr, ok := args[0].([]any); ok {
		return float64(len(arr)), nil
	} else if str, ok := args[0].(string); ok {
		return float64(len(str)), nil
	} else {
		return float64(len(args)), nil
	}
}

func element(args ...any) (any, error) {
	if len(args) < 3 {
		return nil, fmt.Errorf("f(element) invalud number of args (n:%d)", len(args))
	}
	var idx int
	if n, ok := args[len(args)-1].(float64); ok {
		idx = int(n)
	} else {
		return nil, fmt.Errorf("f(element) 2nd arg should be int")
	}
	if len(args)-1 <= idx {
		return nil, fmt.Errorf("f(element) out of index %d / %d", idx, len(args)-1)
	}
	switch v := args[idx].(type) {
	case int:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case float64:
		return v, nil
	case string:
		return v, nil
	case bool:
		return v, nil
	case time.Time:
		return float64(v.UnixNano()) / float64(time.Second), nil
	default:
		return nil, fmt.Errorf("f(element) unsupported type %T", v)
	}
}
