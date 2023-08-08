package util

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// /////////////////
// utilites
func SplitFields(line string, stripQuote bool) []string {
	lastQuote := rune(0)
	f := func(c rune) bool {
		switch {
		case c == lastQuote:
			lastQuote = rune(0)
			return false
		case lastQuote != rune(0):
			return false
		case unicode.In(c, unicode.Quotation_Mark):
			lastQuote = c
			return false
		default:
			return unicode.IsSpace(c)
		}
	}
	fields := strings.FieldsFunc(line, f)

	if stripQuote {
		for i, f := range fields {
			c := []rune(f)[0]
			if unicode.In(c, unicode.Quotation_Mark) {
				fields[i] = strings.Trim(f, string(c))
			}
		}
	}
	return fields
}

func StripQuote(str string) string {
	if len(str) == 0 {
		return str
	}
	c := []rune(str)[0]
	if unicode.In(c, unicode.Quotation_Mark) {
		return strings.Trim(str, string(c))
	}
	return str
}

func StringFields(values []any, timeformat string, timeLocation *time.Location, precision int) []string {
	cols := make([]string, len(values))
	for i, r := range values {
		if r == nil {
			cols[i] = "NULL"
			continue
		}
		switch v := r.(type) {
		case *string:
			cols[i] = *v
		case string:
			cols[i] = v
		case *time.Time:
			switch timeformat {
			case "", "ns":
				cols[i] = strconv.FormatInt(v.UnixNano(), 10)
			case "ms":
				cols[i] = strconv.FormatInt(v.UnixMilli(), 10)
			case "us":
				cols[i] = strconv.FormatInt(v.UnixMicro(), 10)
			case "s":
				cols[i] = strconv.FormatInt(v.Unix(), 10)
			default:
				if timeLocation == nil {
					timeLocation = time.UTC
				}
				cols[i] = v.In(timeLocation).Format(timeformat)
			}
		case *float64:
			if precision < 0 {
				cols[i] = fmt.Sprintf("%f", *v)
			} else {
				cols[i] = fmt.Sprintf("%.*f", precision, *v)
			}
		case *int:
			cols[i] = strconv.FormatInt(int64(*v), 10)
		case int:
			cols[i] = strconv.FormatInt(int64(v), 10)
		case *int8:
			cols[i] = strconv.FormatInt(int64(*v), 10)
		case int8:
			cols[i] = strconv.FormatInt(int64(v), 10)
		case *int16:
			cols[i] = strconv.FormatInt(int64(*v), 10)
		case int16:
			cols[i] = strconv.FormatInt(int64(v), 10)
		case *int32:
			cols[i] = strconv.FormatInt(int64(*v), 10)
		case int32:
			cols[i] = strconv.FormatInt(int64(v), 10)
		case *int64:
			cols[i] = strconv.FormatInt(*v, 10)
		case int64:
			cols[i] = strconv.FormatInt(v, 10)
		case *net.IP:
			cols[i] = v.String()
		case net.IP:
			cols[i] = v.String()
		default:
			cols[i] = fmt.Sprintf("%T", r)
		}
	}
	return cols
}
