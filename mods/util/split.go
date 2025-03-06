package util

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
)

type SqlStatementEnv struct {
	Error  string `json:"error,omitempty"`
	Bridge string `json:"bridge,omitempty"`
}

func (sse *SqlStatementEnv) Reset() {
	sse.Error = ""
	sse.Bridge = ""
}

type SqlStatement struct {
	Text      string           `json:"text"`
	BeginLine int              `json:"beginLine"`
	EndLine   int              `json:"endLine"`
	IsComment bool             `json:"isComment"`
	Env       *SqlStatementEnv `json:"env,omitempty"`
}

// SplitSqlStatements splits multiple SQL statements from the reader.
func SplitSqlStatements(reader io.Reader) ([]*SqlStatement, error) {
	var env = &SqlStatementEnv{}
	var statements []*SqlStatement
	var buffer bytes.Buffer
	var commentBuffer bytes.Buffer
	scanner := bufio.NewScanner(reader)
	scanner.Split(bufio.ScanRunes)

	inString := false
	inSingleLineComment := false
	inSingleDash := false
	inSingleSlash := false
	lineNumber := 1
	statementStartLine := 1

	for scanner.Scan() {
		char := scanner.Text()

		if inSingleLineComment {
			if char == "\n" {
				inSingleLineComment = false
				commentText := commentBuffer.String()
				if newEnv, err := ParseStatementEnv(env, commentText); err != nil {
					return nil, fmt.Errorf("line %d: %w", lineNumber, err)
				} else {
					env = newEnv
				}
				statements = append(statements, &SqlStatement{
					Text:      commentText,
					BeginLine: statementStartLine,
					EndLine:   lineNumber,
					IsComment: true,
					Env:       env,
				})
				lineNumber++
				if strings.TrimSpace(buffer.String()) == "" {
					statementStartLine = lineNumber
				}
			}
			if char != "\r" {
				commentBuffer.WriteString(char)
			}
			continue
		}

		switch char {
		case "'":
			inString = !inString
		case "-":
			if !inString {
				if inSingleDash {
					commentBuffer.Reset()
					inSingleLineComment = true
					commentBuffer.WriteString("--")
				}
				inSingleDash = !inSingleDash
				continue
			}
		case "/":
			if !inString {
				if inSingleSlash {
					commentBuffer.Reset()
					inSingleLineComment = true
					commentBuffer.WriteString("//")
				}
				inSingleSlash = !inSingleSlash
				continue
			}
		case ";":
			if !inString {
				statements = append(statements, &SqlStatement{
					Text:      buffer.String() + ";",
					BeginLine: statementStartLine,
					EndLine:   lineNumber,
					IsComment: false,
					Env:       env,
				})
				buffer.Reset()
				statementStartLine = lineNumber
				continue
			}
		case "\r":
		case "\n":
			lineNumber++
		}

		if strings.TrimSpace(buffer.String()) == "" && strings.ContainsAny(char, " \t\r\n") {
			statementStartLine = lineNumber
		} else {
			if inSingleDash {
				buffer.WriteString("-")
				inSingleDash = false
			}
			if inSingleSlash {
				buffer.WriteString("/")
				inSingleSlash = false
			}
			buffer.WriteString(char)
		}
	}

	if len(strings.TrimSpace(buffer.String())) > 0 {
		statements = append(statements, &SqlStatement{
			Text:      buffer.String(),
			BeginLine: statementStartLine,
			EndLine:   lineNumber,
			IsComment: false,
			Env:       env,
		})
	}

	return statements, scanner.Err()
}

func ParseStatementEnv(prev *SqlStatementEnv, text string) (*SqlStatementEnv, error) {
	text = strings.TrimSpace(strings.TrimPrefix(text, "--"))
	if !strings.HasPrefix(text, "env:") {
		return prev, nil
	}
	// -- env: bridge=sqlite
	// -- env: reset
	text = strings.TrimSpace(strings.TrimPrefix(text, "env:"))
	pairs := ParseNameValuePairs(text)
	if len(pairs) == 0 {
		return prev, nil
	}
	env := &SqlStatementEnv{}
	*env = *prev
	for _, pair := range pairs {
		switch pair.Name {
		case "bridge":
			env.Bridge = pair.Value
		case "reset":
			env.Reset()
		default:
			env.Error = fmt.Sprintf("unknown env: %s", pair.Name)
		}
	}
	return env, nil
}

type NameValuePair struct {
	Name  string
	Value string
}

func (v *NameValuePair) String() string {
	if strings.ContainsAny(v.Value, " \r\n\t\"") {
		return fmt.Sprintf(`%s="%s"`, v.Name, strings.ReplaceAll(v.Value, `"`, `\"`))
	} else {
		return fmt.Sprintf("%s=%s", v.Name, v.Value)
	}
}

var parseNameValuePairsRegexp = regexp.MustCompile(`(\w+)(?:=("([^"\\]*(\\.[^"\\]*)*)"|[^ ]+))?`)

// ParseNameValuePairs parses multiple name=value pairs
// where values can contain whitespace within double quotation marks.
//
//	func main() {
//	    input := `name1=value1 name2="value \"with\" spaces" name3=value3 name4 `
//	    result := tokenize(input)
//	    for k, v := range result {
//	        fmt.Printf("%s=%s\n", k, v)
//	    }
//	}
func ParseNameValuePairs(input string) []NameValuePair {
	pairs := make([]NameValuePair, 0)
	matches := parseNameValuePairsRegexp.FindAllStringSubmatch(input, -1)

	for _, match := range matches {
		key := match[1]
		value := match[2]
		if value == "" {
			value = ""
		} else if value[0] == '"' && value[len(value)-1] == '"' {
			value = value[1 : len(value)-1]
			value = strings.ReplaceAll(value, `\"`, `"`)
		}
		pairs = append(pairs, NameValuePair{key, value})
	}

	return pairs
}

// /////////////////
// utilities
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
		case time.Time:
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
		case float64:
			if precision < 0 {
				cols[i] = fmt.Sprintf("%f", v)
			} else {
				cols[i] = fmt.Sprintf("%.*f", precision, v)
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
			if o, ok := r.(Stringify); ok {
				cols[i] = o.String()
			} else {
				cols[i] = fmt.Sprintf("%T", r)
			}
		}
	}
	return cols
}

type Stringify interface {
	String() string
}
