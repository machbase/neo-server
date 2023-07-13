package booter

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/zclconf/go-cty/cty"
)

func IntFromCty(value cty.Value) (int, error) {
	switch value.Type() {
	case cty.Number:
		f := value.AsBigFloat()
		l, _ := f.Int64()
		return int(l), nil
	case cty.String:
		s := value.AsString()
		l, err := strconv.ParseInt(s[0:len(s)-1], 10, 32)
		return int(l), err
	default:
		return 0, fmt.Errorf("value is not a number, %s", value.Type())
	}
}

func Int64FromCty(value cty.Value) (int64, error) {
	switch value.Type() {
	case cty.Number:
		f := value.AsBigFloat()
		l, _ := f.Int64()
		return l, nil
	case cty.String:
		s := value.AsString()
		if strings.HasSuffix(s, "ms") {
			l, err := strconv.ParseInt(s[0:len(s)-2], 10, 64)
			return l * int64(time.Millisecond), err
		} else if strings.HasSuffix(s, "s") {
			l, err := strconv.ParseInt(s[0:len(s)-1], 10, 64)
			return l * int64(time.Second), err
		} else if strings.HasSuffix(s, "m") {
			l, err := strconv.ParseInt(s[0:len(s)-1], 10, 64)
			return l * int64(time.Minute), err
		} else if strings.HasSuffix(s, "h") {
			l, err := strconv.ParseInt(s[0:len(s)-1], 10, 64)
			return l * int64(time.Hour), err
		} else {
			s := value.AsString()
			if l, err := strconv.ParseInt(s, 10, 64); err == nil {
				return int64(l), err
			} else {
				return 0, fmt.Errorf("value is not a number-compatible, %s", value.Type())
			}
		}
	default:
		return 0, fmt.Errorf("value is not a number, %s", value.Type())
	}
}

func Uint64FromCty(value cty.Value) (uint64, error) {
	switch value.Type() {
	case cty.Number:
		f := value.AsBigFloat()
		l, _ := f.Uint64()
		return l, nil
	case cty.String:
		s := value.AsString()
		if strings.HasSuffix(s, "s") {
			l, err := strconv.ParseInt(s[0:len(s)-1], 10, 64)
			return uint64(l) * uint64(time.Second), err
		} else if strings.HasSuffix(s, "m") {
			l, err := strconv.ParseInt(s[0:len(s)-1], 10, 64)
			return uint64(l) * uint64(time.Minute), err
		} else if strings.HasSuffix(s, "h") {
			l, err := strconv.ParseInt(s[0:len(s)-1], 10, 64)
			return uint64(l) * uint64(time.Hour), err
		} else {
			s := value.AsString()
			if l, err := strconv.ParseInt(s, 10, 64); err == nil {
				return uint64(l), err
			} else {
				return 0, fmt.Errorf("value is not a number-compatible, %s", value.Type())
			}
		}
	default:
		return 0, fmt.Errorf("value is not a number, %s", value.Type())
	}
}

func PriorityFromCty(value cty.Value) int {
	switch value.Type() {
	case cty.Number:
		f := value.AsBigFloat()
		l, _ := f.Int64()
		return int(l)
	default:
		return 999
	}
}

func BoolFromCty(value cty.Value) (bool, error) {
	switch value.Type() {
	case cty.Bool:
		return value.True(), nil
	case cty.String:
		s := value.AsString()
		switch strings.ToLower(s) {
		case "true", "t", "yes", "y":
			return true, nil
		case "false", "f", "no", "n":
			return false, nil
		default:
			return false, fmt.Errorf("%s is not bool compatible", s)
		}
	default:
		return false, fmt.Errorf("value is not a bool, %s", value.Type())
	}
}

func StringFromCty(value cty.Value) string {
	return value.AsString()
}

/*
// Converts a string to CamelCase
var uppercaseAcronym = map[string]string{
	"ID": "id",
}

func toCamelCase(s string, initCase bool) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	if a, ok := uppercaseAcronym[s]; ok {
		s = a
	}

	n := strings.Builder{}
	n.Grow(len(s))
	capNext := initCase
	for i, v := range []byte(s) {
		vIsCap := v >= 'A' && v <= 'Z'
		vIsLow := v >= 'a' && v <= 'z'
		if capNext {
			if vIsLow {
				v += 'A'
				v -= 'a'
			}
		} else if i == 0 {
			if vIsCap {
				v += 'a'
				v -= 'A'
			}
		}
		if vIsCap || vIsLow {
			n.WriteByte(v)
			capNext = false
		} else if vIsNum := v >= '0' && v <= '9'; vIsNum {
			n.WriteByte(v)
			capNext = true
		} else {
			capNext = v == '_' || v == ' ' || v == '-' || v == '.'
		}
	}
	return n.String()
}

var matchFirstCap = regexp.MustCompile("(.)([A-Z][a-z]+)")
var matchAllCap = regexp.MustCompile("([a-z0-9])([A-Z])")

func toSnakeCase(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	if a, ok := uppercaseAcronym[s]; ok {
		s = a
	}

	snake := matchFirstCap.ReplaceAllString(s, "${1}-${2}")
	snake = matchAllCap.ReplaceAllString(snake, "${1}-${2}")
	return strings.ToLower(snake)
}

func toFlagName(prefix, name string) string {
	return prefix + "-" + toSnakeCase(name)
}
*/
