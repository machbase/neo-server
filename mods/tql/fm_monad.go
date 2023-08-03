package tql

import (
	"fmt"
)

type lazyOption struct {
	flag bool
}

func (x *Task) fmLazy(flag bool) *lazyOption {
	return &lazyOption{flag: flag}
}

func (x *Task) fmTake(node *Node, key any, value any, limit int) *Record {
	if node.Nrow > limit {
		return BreakRecord
	}
	return node.NewRecord(key, value)
}

func (x *Task) fmDrop(node *Node, key any, value any, limit int) *Record {
	if node.Nrow <= limit {
		return nil
	}
	return node.NewRecord(key, value)
}

func (x *Task) fmFilter(node *Node, key any, value any, flag bool) *Record {
	if !flag {
		return nil // drop this vector
	}
	return node.NewRecord(key, value)
}

func (x *Task) fmFlatten(node *Node, key any, value any) any {
	if arr, ok := value.([]any); ok {
		ret := []*Record{}
		for _, elm := range arr {
			if subarr, ok := elm.([]any); ok {
				for _, subelm := range subarr {
					ret = append(ret, node.NewRecord(key, subelm))
				}
			} else if subarr, ok := elm.([][]any); ok {
				for _, subelm := range subarr {
					ret = append(ret, node.NewRecord(key, subelm))
				}
			} else {
				ret = append(ret, node.NewRecord(key, elm))
			}
		}
		return ret
	} else {
		return node.NewRecord(key, value)
	}
}

func (x *Task) fmGroupByKey(node *Node, key any, value any, args ...any) any {
	lazy := false
	if len(args) > 0 {
		for _, arg := range args {
			switch v := arg.(type) {
			case *lazyOption:
				lazy = v.flag
			}
		}
	}
	if lazy {
		node.Buffer(key, value)
		return nil
	}

	var curKey any
	curKey, _ = node.GetValue("curKey")
	defer func() {
		node.SetValue("curKey", curKey)
	}()
	if curKey == nil {
		curKey = key
	}
	node.Buffer(key, value)

	if curKey != key {
		node.YieldBuffer(curKey)
		curKey = key
	}
	return nil
}

// Drop Key, then make the first element of value to promote as a key,
// decrease dimension of vector as result if the input is not multiple dimension vector.
// `map=POPKEY(V, 0)` produces
// 1 dimension : `K: [V1, V2, V3...]` ==> `V1 : [V2, V3, .... ]`
// 2 dimension : `K: [[V11, V12, V13...],[V21, V22, V23...], ...] ==> `V11: [V12, V13...]` and `V21: [V22, V23...]` ...
func (x *Task) fmPopKey(node *Node, key any, value any, args ...int) (any, error) {
	var nth = 0
	if len(args) > 0 {
		nth = args[0]
	}

	// V : value
	switch val := value.(type) {
	default:
		return nil, fmt.Errorf("f(POPKEY) V should be []any or [][]any, but %T", val)
	case []any:
		if nth < 0 || nth >= len(val) {
			return nil, fmt.Errorf("f(POPKEY) 1st arg should be between 0 and %d, but %d", len(val)-1, nth)
		}
		newKey := val[nth]
		newVal := append(val[0:nth], val[nth+1:]...)
		ret := node.NewRecord(newKey, newVal)
		return ret, nil
	case [][]any:
		ret := make([]*Record, len(val))
		for i, v := range val {
			if len(v) < 2 {
				return nil, fmt.Errorf("f(POPKEY) arg elements should be larger than 2, but %d", len(v))
			}
			if len(v) == 2 {
				ret[i] = node.NewRecord(v[0], v[1])
			} else {
				ret[i] = node.NewRecord(v[0], v[1:])
			}
		}
		return ret, nil
	}
}

// Merge all incoming values into a single key,
// incresing dimension of vector as result.
// `map=PUSHKEY(NewKEY)` produces `NewKEY: [K, V...]`
func (x *Task) fmPushKey(node *Node, key any, value any, newKey any) (any, error) {
	var newVal []any
	if val, ok := value.([]any); ok {
		newVal = append([]any{key}, val...)
	} else {
		return nil, fmt.Errorf("f(PUSHKEY) V should be []any, but %T", value)
	}
	return node.NewRecord(newKey, newVal), nil
}
