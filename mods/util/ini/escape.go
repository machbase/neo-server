package ini

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// remove inline comments
//
// inline comments must start with ';' or '#'
// and the char before the ';' or '#' must be a space
func removeComments(value string) string {
	n := len(value)
	i := 0
	for ; i < n; i++ {
		if value[i] == '\\' {
			i++
		} else if value[i] == ';' || value[i] == '#' {
			if i > 0 && unicode.IsSpace(rune(value[i-1])) {
				return strings.TrimSpace(value[0:i])
			}
		}
	}
	return strings.TrimSpace(value)
}

func removeContinuationSuffix(value string) (string, bool) {
	pos := strings.LastIndex(value, "\\")
	n := len(value)
	if pos == -1 || pos != n-1 {
		return "", false
	}
	for pos >= 0 {
		if value[pos] != '\\' {
			return "", false
		}
		pos--
		if pos < 0 || value[pos] != '\\' {
			return value[0 : n-1], true
		}
		pos--
	}
	return "", false
}

func toEscape(s string) string {
	result := bytes.NewBuffer(make([]byte, 0))

	n := len(s)

	for i := 0; i < n; i++ {
		switch s[i] {
		case 0:
			result.WriteString("\\0")
		case '\\':
			result.WriteString("\\\\")
		case '\a':
			result.WriteString("\\a")
		case '\b':
			result.WriteString("\\b")
		case '\t':
			result.WriteString("\\t")
		case '\r':
			result.WriteString("\\r")
		case '\n':
			result.WriteString("\\n")
		case ';':
			result.WriteString("\\;")
		case '#':
			result.WriteString("\\#")
		case '=':
			result.WriteString("\\=")
		case ':':
			result.WriteString("\\:")
		default:
			result.WriteByte(s[i])
		}
	}
	return result.String()
}

func fromEscape(value string) string {
	if !strings.Contains(value, "\\") {
		return value
	}

	r := ""
	n := len(value)
	for i := 0; i < n; i++ {
		if value[i] == '\\' {
			if i+1 < n {
				i++
				//if is it oct
				if i+2 < n && isOctChar(value[i]) && isOctChar(value[i+1]) && isOctChar(value[i+2]) {
					t, err := strconv.ParseInt(value[i:i+3], 8, 32)
					if err == nil {
						r = r + string(rune(t))
					}
					i += 2
					continue
				}
				switch value[i] {
				case '0':
					r = r + string(byte(0))
				case 'a':
					r = r + "\a"
				case 'b':
					r = r + "\b"
				case 'f':
					r = r + "\f"
				case 't':
					r = r + "\t"
				case 'r':
					r = r + "\r"
				case 'n':
					r = r + "\n"
				case 'v':
					r = r + "\v"
				case 'x':
					i++
					if i+3 < n && isHexChar(value[i]) &&
						isHexChar(value[i+1]) &&
						isHexChar(value[i+2]) &&
						isHexChar(value[i+3]) {

						t, err := strconv.ParseInt(value[i:i+4], 16, 32)
						if err == nil {
							r = r + string(rune(t))
						}
						i += 3
					}
				default:
					r = fmt.Sprintf("%s%c", r, value[i])
				}
			}
		} else {
			r = fmt.Sprintf("%s%c", r, value[i])
		}
	}
	return r
}

// check if it is a oct char,e.g. must be char '0' to '7'
func isOctChar(ch byte) bool {
	return ch >= '0' && ch <= '7'
}

// check if the char is a hex char, e.g. the char
// must be '0'..'9' or 'a'..'f' or 'A'..'F'
func isHexChar(ch byte) bool {
	return ch >= '0' && ch <= '9' ||
		ch >= 'a' && ch <= 'f' ||
		ch >= 'A' && ch <= 'F'
}
