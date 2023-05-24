package tagql

import (
	"fmt"
	"io"
	"time"

	"github.com/machbase/neo-server/mods/expression"
)

func newMapFunctions() map[string]expression.Function {
	return map[string]expression.Function{
		"GROUP_TIME": (&mapfGroupTime{}).GROUP_TIME,
		"FFT":        mapf_FFT,
	}
}

type mapfGroupTime struct {
	lastKey time.Time
	data    []MapData
}

// group by 1 second
// `map=GROUP_TIME(K, V, 1000*1000*1000)`
func (mf *mapfGroupTime) GROUP_TIME(args ...any) (any, error) {
	if args[0] == io.EOF {
		if len(mf.data) == 0 {
			return nil, nil
		}
		ret := []any{mf.lastKey, mf.data}
		mf.data = []MapData{}
		return ret, nil
	}
	if len(args) < 3 {
		return nil, fmt.Errorf("invalid number of args")
	}
	// K : time
	key, ok := args[0].(*time.Time)
	if !ok {
		return nil, fmt.Errorf("arg k should be time, but %T", args[0])
	}
	// V : value
	val, ok := args[1].(MapData)
	if !ok {
		return nil, fmt.Errorf("arg v should be []any, but %T", args[1])
	}
	// duration
	argMod, ok := args[2].(float64)
	if !ok {
		return nil, fmt.Errorf("3rd argument should be duration, but %T", args[2])
	}
	mod := int64(argMod)

	newKey := time.Unix(0, (key.UnixNano()/mod)*mod)
	newVal := append([]any{key}, val...)

	if mf.lastKey.IsZero() {
		mf.lastKey = newKey
		// add to buffer
		mf.data = append(mf.data, val)
		return nil, nil
	} else if mf.lastKey == newKey {
		// add to buffer
		mf.data = append(mf.data, val)
		return nil, nil
	}

	ret := []any{mf.lastKey, mf.data}
	// 1. flush buffer
	mf.data = []MapData{}
	// 2. update lastKey, add to buffer
	mf.lastKey = newKey
	mf.data = append(mf.data, newVal)

	return ret, nil
}

func mapf_FFT(args ...any) (any, error) {
	return nil, nil
}
