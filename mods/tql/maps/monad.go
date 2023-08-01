package maps

import (
	"fmt"

	"github.com/machbase/neo-server/mods/tql/context"
)

type lazyOption struct {
	flag bool
}

func ToLazy(flag bool) *lazyOption {
	return &lazyOption{flag: flag}
}

func Take(ctx *context.Context, key any, value any, limit int) *context.Param {
	if ctx.Nrow > limit {
		return context.ExecutionCircuitBreak
	}
	return &context.Param{K: key, V: value}
}

func Drop(ctx *context.Context, key any, value any, limit int) *context.Param {
	if ctx.Nrow <= limit {
		return nil
	}
	return &context.Param{K: key, V: value}
}

func Filter(ctx *context.Context, key any, value any, flag bool) *context.Param {
	if !flag {
		return nil // drop this vector
	}
	return &context.Param{K: key, V: value}
}

func Flatten(ctx *context.Context, key any, value any) any {
	if arr, ok := value.([]any); ok {
		ret := []*context.Param{}
		for _, elm := range arr {
			if subarr, ok := elm.([]any); ok {
				for _, subelm := range subarr {
					ret = append(ret, &context.Param{K: key, V: subelm})
				}
			} else if subarr, ok := elm.([][]any); ok {
				for _, subelm := range subarr {
					ret = append(ret, &context.Param{K: key, V: subelm})
				}
			} else {
				ret = append(ret, &context.Param{K: key, V: elm})
			}
		}
		return ret
	} else {
		return &context.Param{K: key, V: value}
	}
}

func GroupByKey(ctx *context.Context, key any, value any, args ...any) any {
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
		ctx.Buffer(key, value)
		return nil
	}

	var curKey any
	curKey, _ = ctx.Get("curKey")
	defer func() {
		ctx.Set("curKey", curKey)
	}()
	if curKey == nil {
		curKey = key
	}
	ctx.Buffer(key, value)

	if curKey != key {
		ctx.YieldBuffer(curKey)
		curKey = key
	}
	return nil
}

// Drop Key, then make the first element of value to promote as a key,
// decrease dimension of vector as result if the input is not multiple dimension vector.
// `map=POPKEY(V, 0)` produces
// 1 dimension : `K: [V1, V2, V3...]` ==> `V1 : [V2, V3, .... ]`
// 2 dimension : `K: [[V11, V12, V13...],[V21, V22, V23...], ...] ==> `V11: [V12, V13...]` and `V21: [V22, V23...]` ...
func PopKey(ctx *context.Context, key any, value any, args ...int) (any, error) {
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
		ret := &context.Param{K: newKey, V: newVal}
		return ret, nil
	case [][]any:
		ret := make([]*context.Param, len(val))
		for i, v := range val {
			if len(v) < 2 {
				return nil, fmt.Errorf("f(POPKEY) arg elements should be larger than 2, but %d", len(v))
			}
			if len(v) == 2 {
				ret[i] = &context.Param{K: v[0], V: v[1]}
			} else {
				ret[i] = &context.Param{K: v[0], V: v[1:]}
			}
		}
		return ret, nil
	}
}

// Merge all incoming values into a single key,
// incresing dimension of vector as result.
// `map=PUSHKEY(NewKEY)` produces `NewKEY: [K, V...]`
func PushKey(ctx *context.Context, key any, value any, newKey any) (any, error) {
	var newVal []any
	if val, ok := value.([]any); ok {
		newVal = append([]any{key}, val...)
	} else {
		return nil, fmt.Errorf("f(PUSHKEY) V should be []any, but %T", value)
	}
	ret := &context.Param{
		K: newKey,
		V: newVal,
	}
	return ret, nil
}
