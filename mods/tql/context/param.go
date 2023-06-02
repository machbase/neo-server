package context

import (
	"fmt"
	"strings"
	"time"
)

// ////////////////////////////
// PARAM
var ExecutionEOF = &Param{}

type Param struct {
	Ctx *Context
	K   any
	V   any
}

func (p *Param) Get(name string) (any, error) {
	if name == "K" {
		switch k := p.K.(type) {
		case *time.Time:
			return *k, nil
		default:
			return p.K, nil
		}
	} else if name == "V" {
		return p.V, nil
	} else if name == "P" {
		return p, nil
	} else if name == "CTX" {
		return p.Ctx, nil
	} else if strings.HasPrefix(name, "$") {
		if arr, ok := p.Ctx.Params[strings.TrimPrefix(name, "$")]; ok && len(arr) > 0 {
			return arr[len(arr)-1], nil
		}
		return nil, nil
	}
	return nil, fmt.Errorf("undefined variable '%s'", name)
}

func (p *Param) String() string {
	return fmt.Sprintf("K:%T(%v) V:%s", p.K, p.K, p.StringValueTypes())
}

func (p *Param) StringValueTypes() string {
	if arr, ok := p.V.([]any); ok {
		return p.stringTypesOfArray(arr, 3)
	} else if arr, ok := p.V.([][]any); ok {
		subTypes := []string{}
		for i, subarr := range arr {
			if i == 3 && len(arr) > i {
				subTypes = append(subTypes, fmt.Sprintf("[%d]{%s}, ...", i, p.stringTypesOfArray(subarr, 3)))
				break
			} else {
				subTypes = append(subTypes, fmt.Sprintf("[%d]{%s}", i, p.stringTypesOfArray(subarr, 3)))
			}
		}

		return fmt.Sprintf("(len=%d) [][]any{%s}", len(arr), strings.Join(subTypes, ","))
	} else {
		return fmt.Sprintf("%T", p.V)
	}
}

func (p *Param) stringTypesOfArray(arr []any, limit int) string {
	s := []string{}
	for i, a := range arr {
		aType := fmt.Sprintf("%T", a)
		if subarr, ok := a.([]any); ok {
			s2 := []string{}
			for n, subelm := range subarr {
				if n == limit && len(subarr) > n {
					s2 = append(s2, fmt.Sprintf("%T,... (len=%d)", subelm, len(subarr)))
					break
				} else {
					s2 = append(s2, fmt.Sprintf("%T", subelm))
				}
			}
			aType = "[]any{" + strings.Join(s2, ",") + "}"
		}

		if i == limit && len(arr) > i {
			t := fmt.Sprintf("%s, ... (len=%d)", aType, len(arr))
			s = append(s, t)
			break
		} else {
			s = append(s, aType)
		}
	}
	return strings.Join(s, ", ")
}

func (p *Param) EqualKey(other *Param) bool {
	if other == nil {
		return false
	}
	switch lv := p.K.(type) {
	case time.Time:
		if rv, ok := other.K.(time.Time); !ok {
			return false
		} else {
			return lv.Nanosecond() == rv.Nanosecond()
		}
	case []int:
		if rv, ok := other.K.([]int); !ok {
			return false
		} else {
			if len(lv) != len(rv) {
				return false
			}
			for i := range lv {
				if lv[i] != rv[i] {
					return false
				}
			}
			return true
		}
	}
	return p.K == other.K
}

func (p *Param) EqualValue(other *Param) bool {
	if other == nil {
		return false
	}
	lv := fmt.Sprintf("%#v", p.V)
	rv := fmt.Sprintf("%#v", other.V)
	return lv == rv
}
