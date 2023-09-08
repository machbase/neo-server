package tql

import (
	"fmt"

	spi "github.com/machbase/neo-spi"
)

type lazyOption struct {
	flag bool
}

func (node *Node) fmLazy(flag bool) *lazyOption {
	return &lazyOption{flag: flag}
}

func (node *Node) fmTake(limit int) *Record {
	if node.nrow > limit {
		return BreakRecord
	}
	return node.Inflight()
}

func (node *Node) fmDrop(limit int) *Record {
	if node.nrow <= limit {
		return nil
	}
	return node.Inflight()
}

func (node *Node) fmFilter(flag bool) *Record {
	if !flag {
		return nil // drop this vector
	}
	return node.Inflight()
}

func (node *Node) fmFlatten() any {
	rec := node.Inflight()
	if rec.IsArray() {
		ret := []*Record{}
		for _, r := range rec.Array() {
			k := r.Key()
			switch value := r.Value().(type) {
			case []any:
				for _, v := range value {
					ret = append(ret, NewRecord(k, v))
				}
			case any:
				ret = append(ret, r)
			default:
				ret = append(ret, ErrorRecord(fmt.Errorf("fmtFlatten() unknown type '%T' in array record", value)))
			}
		}
		return ret
	} else if rec.IsTuple() {
		switch value := rec.Value().(type) {
		case []any:
			k := rec.Key()
			ret := []*Record{}
			for _, v := range value {
				ret = append(ret, NewRecord(k, v))
			}
			return ret
		case any:
			return rec
		default:
			return ErrorRecord(fmt.Errorf("fmtFlatten() unknown type '%T' in array record", value))
		}
	} else {
		return rec
	}
}

func (node *Node) fmGroupByKey(args ...any) any {
	key := node.Inflight().key
	value := node.Inflight().value
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
func (node *Node) fmPopKey(args ...int) (any, error) {
	var nth = 0
	if len(args) > 0 {
		nth = args[0]
	}

	// V : value
	inflight := node.Inflight()
	if inflight == nil || inflight.value == nil {
		return nil, nil
	}
	switch val := inflight.value.(type) {
	default:
		return nil, fmt.Errorf("f(POPKEY) V should be []any or [][]any, but %T", val)
	case []any:
		if nth < 0 || nth >= len(val) {
			return nil, fmt.Errorf("f(POPKEY) 1st arg should be between 0 and %d, but %d", len(val)-1, nth)
		}
		if node.Rownum() == 1 {
			columns := node.task.ResultColumns()
			cols := columns
			if len(columns) > nth+1 {
				cols = []*spi.Column{columns[nth+1]}
				cols = append(cols, columns[1:nth+1]...)
			}
			if len(cols) > nth+2 {
				cols = append(cols, columns[nth+2:]...)
			}
			node.task.SetResultColumns(cols)
		}
		newKey := val[nth]
		newVal := append(val[0:nth], val[nth+1:]...)
		ret := NewRecord(newKey, newVal)
		return ret, nil
	case [][]any:
		ret := make([]*Record, len(val))
		if node.Rownum() == 1 {
			columns := node.task.ResultColumns()
			if len(columns) > 1 {
				node.task.SetResultColumns(columns[1:])
			}
		}
		for i, v := range val {
			if len(v) < 2 {
				return nil, fmt.Errorf("f(POPKEY) arg elements should be larger than 2, but %d", len(v))
			}
			if len(v) == 2 {
				ret[i] = NewRecord(v[0], v[1])
			} else {
				ret[i] = NewRecord(v[0], v[1:])
			}
		}
		return ret, nil
	}
}

// Merge all incoming values into a single key,
// incresing dimension of vector as result.
// `map=PUSHKEY(NewKEY)` produces `NewKEY: [K, V...]`
func (node *Node) fmPushKey(newKey any) (any, error) {
	if node.Rownum() == 1 {
		node.task.SetResultColumns(append([]*spi.Column{node.AsColumnTypeOf(newKey)}, node.task.ResultColumns()...))
	}
	rec := node.Inflight()
	if rec == nil {
		return nil, nil
	}
	key, value := rec.key, rec.value
	var newVal []any
	if val, ok := value.([]any); ok {
		newVal = append([]any{key}, val...)
	} else {
		return nil, ErrArgs("PUSHKEY", 0, fmt.Sprintf("Value should be array, but %T", value))
	}
	return NewRecord(newKey, newVal), nil
}
