package tql

import (
	"fmt"
	"strings"
	"time"
)

type Receiver interface {
	Name() string
	Receive(*Record)
}

const kEOF = "f0ec1dea-03e8-4121-8c98-0b78704e009d"
const kBREAK = "5bd2e423-4536-4a8d-a80d-c11567fc296f"
const kBYTES = "a6cd7131-63cc-4f83-9cbb-709a3d317780"
const kIMAGE = "f2f79e86-44dc-4721-95e0-ba42ebe1fe88"
const kERR = "0fd184f8-0f4a-4d05-bf0f-77bd31642eae"
const kARR = "057f1cb0-df9f-41d3-b003-ba7c1ef8f497"

var EofRecord = &Record{key: kEOF}
var BreakRecord = &Record{key: kBREAK}

func ErrorRecord(err error) *Record     { return &Record{key: kERR, value: err} }
func ArrayRecord(arr []*Record) *Record { return &Record{key: kARR, value: arr} }

type Record struct {
	key         any
	value       any
	contentType string
}

func NewRecord(k, v any) *Record {
	return &Record{key: k, value: v}
}

func NewBytesRecord(raw []byte) *Record {
	return &Record{key: kBYTES, value: raw}
}

func NewImageRecord(raw []byte, contentType string) *Record {
	return &Record{key: kIMAGE, value: raw, contentType: contentType}
}

func (r *Record) IsEOF() bool {
	return r.key == kEOF
}

func (r *Record) IsCircuitBreak() bool {
	return r.key == kBREAK
}

func (r *Record) IsError() bool {
	return r.key == kERR
}

func (r *Record) IsBytes() bool {
	return r.key == kBYTES
}

func (r *Record) IsImage() bool {
	return r.key == kIMAGE
}

func (r *Record) Error() error {
	if r.key == kERR {
		return r.value.(error)
	} else {
		return nil
	}
}

func (r *Record) IsArray() bool {
	return r.key == kARR
}

func (r *Record) IsTuple() bool {
	switch r.key {
	case kEOF, kBREAK, kBYTES, kIMAGE, kERR, kARR:
		return false
	default:
		return true
	}
}

func (r *Record) Array() []*Record {
	if r.key == kARR {
		return r.value.([]*Record)
	} else {
		return nil
	}
}

func (r *Record) Key() any {
	return r.key
}

func (r *Record) Value() any {
	return r.value
}

func (r *Record) Flatten() []any {
	k := r.Key()
	v := r.Value()
	switch vv := v.(type) {
	case []any:
		return append([]any{k}, vv...)
	case any:
		return []any{k, vv}
	default:
		if vv == nil {
			return []any{k}
		}
		return []any{k, fmt.Sprintf("Record: unsupported value type(%T)", vv)}
	}
}

func (r *Record) Tell(receiver Receiver) {
	if receiver == nil {
		return
	}
	receiver.Receive(r)
}

func (r *Record) String() string {
	if r == nil {
		return "<nil>"
	}
	if r.key == kEOF {
		return "EOF"
	} else if r.key == kBREAK {
		return "CIRCUITEBREAK"
	} else if r.key == kBYTES {
		return "BYTES"
	} else if r.key == kIMAGE {
		return "IMAGE"
	} else if r.key == kERR {
		return fmt.Sprintf("ERROR %s", r.value)
	} else if r.key == kARR {
		return "ARRAY"
	} else {
		return fmt.Sprintf("K:%T(%s) V:%s", r.key, r.key, r.StringValueTypes())
	}
}

func (r *Record) Fields() []any {
	var ret []any
	if value := r.Value(); value == nil {
		// if the value of the record is nil, yield key only
		ret = []any{r.Key()}
	} else {
		switch v := value.(type) {
		case [][]any:
			for n := range v {
				ret = append([]any{r.Key()}, v[n]...)
			}
		case []any:
			ret = append([]any{r.Key()}, v...)
		case any:
			ret = []any{r.Key(), v}
		}
	}
	return ret
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
