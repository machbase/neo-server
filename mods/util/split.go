package util

import (
	"strings"
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
