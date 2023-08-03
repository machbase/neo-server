package tql

import (
	"fmt"
	"strings"
	"time"
)

const kEOF = "f0ec1dea-03e8-4121-8c98-0b78704e009d"
const kBREAK = "5bd2e423-4536-4a8d-a80d-c11567fc296f"

var EofRecord = &Record{key: kEOF}
var BreakRecord = &Record{key: kBREAK}

type Record struct {
	key   any
	value any
}

func NewRecord(k, v any) *Record {
	return &Record{key: k, value: v}
}

func (r *Record) IsEOF() bool {
	return r.key == kEOF
}

func (r *Record) IsCircuitBreak() bool {
	return r.key == kBREAK
}

func (r *Record) Key() any {
	return r.key
}

func (r *Record) Value() any {
	return r.value
}

func (r *Record) String() string {
	return fmt.Sprintf("K:%T(%v) V:%s", r.key, r.key, r.StringValueTypes())
}

func (p *Record) StringValueTypes() string {
	if arr, ok := p.value.([]any); ok {
		return p.stringTypesOfArray(arr, 3)
	} else if arr, ok := p.value.([][]any); ok {
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
		return fmt.Sprintf("%T", p.value)
	}
}

func (p *Record) stringTypesOfArray(arr []any, limit int) string {
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

func (p *Record) EqualKey(other *Record) bool {
	if other == nil {
		return false
	}
	switch lv := p.key.(type) {
	case time.Time:
		if rv, ok := other.key.(time.Time); !ok {
			return false
		} else {
			return lv.Nanosecond() == rv.Nanosecond()
		}
	case []int:
		if rv, ok := other.key.([]int); !ok {
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
	return p.key == other.key
}

func (p *Record) EqualValue(other *Record) bool {
	if other == nil {
		return false
	}
	lv := fmt.Sprintf("%#v", p.value)
	rv := fmt.Sprintf("%#v", other.value)
	return lv == rv
}
