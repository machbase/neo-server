package util

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
)

func namedLiteralType(text string) (typ string, open int) {
	for open < len(text) {
		c := text[open]
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9') {
			break
		}
		open++
	}
	if open == len(text) || text[open] != '(' {
		return "", 0
	}
	name := text[:open]
	switch strings.ToLower(name) {
	case "short", "int16":
		return "short", len(name)
	case "ushort", "uint16":
		return "ushort", len(name)
	case "int", "integer", "int32":
		return "int", len(name)
	case "uinteger", "uint32":
		return "uinteger", len(name)
	case "long", "int64":
		return "long", len(name)
	case "ulong", "uint64":
		return "ulong", len(name)
	case "float", "float32":
		return "float", len(name)
	case "double", "float64":
		return "double", len(name)
	case "datetime", "string":
		return strings.ToLower(name), len(name)
	default:
		return "", 0
	}
}

func namedLiteralEnd(text string, open int) (int, error) {
	var quote byte
	for i := open + 1; i < len(text); i++ {
		c := text[i]
		if quote != 0 {
			switch c {
			case '\\':
				i++
			case quote:
				quote = 0
			}
			continue
		}
		switch c {
		case '\'', '"':
			quote = c
		case ')':
			return i + 1, nil
		case '(':
			return 0, fmt.Errorf("nested calls are not allowed")
		}
	}
	return 0, fmt.Errorf("missing closing parenthesis or quote")
}

func quoteNamedString(value string) string {
	return "string('" + strings.NewReplacer(`\`, `\\`, `'`, `\'`).Replace(value) + "')"
}

func namedLiteralArg(text string) (string, bool, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", false, fmt.Errorf("expected one argument")
	}
	quote := text[0]
	if quote != '\'' && quote != '"' {
		if strings.ContainsAny(text, `'",`) || strings.IndexFunc(text, unicode.IsSpace) >= 0 {
			return "", false, fmt.Errorf("expected one argument")
		}
		return text, false, nil
	}
	var value strings.Builder
	for i := 1; i < len(text); i++ {
		switch text[i] {
		case quote:
			if i != len(text)-1 {
				return "", true, fmt.Errorf("expected one quoted argument")
			}
			return value.String(), true, nil
		case '\\':
			i++
			if i == len(text) || (text[i] != quote && text[i] != '\\') {
				return "", true, fmt.Errorf("invalid escape in quoted argument")
			}
			value.WriteByte(text[i])
		default:
			value.WriteByte(text[i])
		}
	}
	return "", true, fmt.Errorf("missing closing quote")
}

var (
	namedDecimalFloat = regexp.MustCompile(`^[+-]?(?:[0-9]+(?:\.[0-9]*)?|\.[0-9]+)(?:[eE][+-]?[0-9]+)?$`)
	namedDatetime     = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}[T ][0-9]{2}:[0-9]{2}:[0-9]{2}(?:\.[0-9]{1,9})?(Z|[+-](?:[01][0-9]|2[0-3]):[0-5][0-9])?$`)
)

// ParseNamedLiteral converts a reserved named argument literal or returns the original string.
// The timezone is only used for datetime literals without an explicit offset.
func ParseNamedLiteral(text string, timezone string) (any, error) {
	literal := strings.TrimSpace(text)
	typ, open := namedLiteralType(literal)
	if typ == "" {
		return text, nil
	}
	value, err := parseNamedLiteral(literal, typ, open, timezone)
	if err != nil {
		return nil, fmt.Errorf("%s literal: %w", typ, err)
	}
	return value, nil
}

func parseNamedLiteral(text string, typ string, open int, timezone string) (any, error) {
	end, err := namedLiteralEnd(text, open)
	if err != nil {
		return nil, err
	}
	if end != len(text) {
		return nil, fmt.Errorf("unexpected text after closing parenthesis")
	}
	arg, quoted, err := namedLiteralArg(text[open+1 : end-1])
	if err != nil {
		return nil, err
	}
	switch typ {
	case "string":
		if !quoted {
			return nil, fmt.Errorf("expected a quoted string")
		}
		return arg, nil
	case "datetime":
		return parseNamedDatetime(arg, timezone)
	}
	if quoted {
		return nil, fmt.Errorf("expected an unquoted number")
	}
	switch typ {
	case "short":
		v, err := strconv.ParseInt(arg, 10, 16)
		if err == nil && v == math.MinInt16 {
			return nil, namedLiteralReservedNull(arg)
		}
		return int16(v), err
	case "ushort":
		v, err := strconv.ParseUint(arg, 10, 16)
		if err == nil && v == math.MaxUint16 {
			return nil, namedLiteralReservedNull(arg)
		}
		return uint16(v), err
	case "int":
		v, err := strconv.ParseInt(arg, 10, 32)
		if err == nil && v == math.MinInt32 {
			return nil, namedLiteralReservedNull(arg)
		}
		return int32(v), err
	case "uinteger":
		v, err := strconv.ParseUint(arg, 10, 32)
		if err == nil && v == math.MaxUint32 {
			return nil, namedLiteralReservedNull(arg)
		}
		return uint32(v), err
	case "long":
		v, err := strconv.ParseInt(arg, 10, 64)
		if err == nil && v == math.MinInt64 {
			return nil, namedLiteralReservedNull(arg)
		}
		return v, err
	case "ulong":
		v, err := strconv.ParseUint(arg, 10, 64)
		if err != nil {
			return nil, err
		}
		if v == math.MaxUint64 {
			return nil, namedLiteralReservedNull(arg)
		}
		return v, nil
	case "float", "double":
		if !namedDecimalFloat.MatchString(arg) {
			return nil, fmt.Errorf("expected a finite decimal number")
		}
		if typ == "float" {
			v, err := strconv.ParseFloat(arg, 32)
			if err == nil && float32(v) == math.MaxFloat32 {
				return nil, namedLiteralReservedNull(arg)
			}
			return float32(v), err
		}
		v, err := strconv.ParseFloat(arg, 64)
		if err == nil && v == math.MaxFloat64 {
			return nil, namedLiteralReservedNull(arg)
		}
		return v, err
	default:
		return nil, fmt.Errorf("unsupported type %q", typ)
	}
}

func namedLiteralReservedNull(arg string) error {
	return fmt.Errorf("value %s is reserved for Machbase NULL", arg)
}

func parseNamedDatetime(arg string, timezone string) (time.Time, error) {
	match := namedDatetime.FindStringSubmatch(arg)
	if match == nil {
		return time.Time{}, fmt.Errorf("expected YYYY-MM-DD HH:MM:SS[.fraction] or RFC 3339")
	}
	var t time.Time
	var err error
	if match[1] != "" {
		t, err = time.Parse(time.RFC3339Nano, arg)
	} else {
		loc, locErr := ParseTimeLocation(timezone, time.UTC)
		if locErr != nil {
			return time.Time{}, locErr
		}
		layout := "2006-01-02 15:04:05"
		if arg[10] == 'T' {
			layout = "2006-01-02T15:04:05"
		}
		t, err = time.ParseInLocation(layout, arg, loc)
		if err == nil && t.Format(layout) != arg[:19] {
			return time.Time{}, fmt.Errorf("local datetime does not exist in %s", loc)
		}
	}
	if err != nil {
		return time.Time{}, err
	}
	if t.Before(time.Unix(0, math.MinInt64)) || t.After(time.Unix(0, math.MaxInt64)) {
		return time.Time{}, fmt.Errorf("datetime is out of range for Unix nanoseconds")
	}
	if t.UnixNano() == -1 {
		return time.Time{}, namedLiteralReservedNull(arg)
	}
	return t, nil
}
