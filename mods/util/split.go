package util

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

type SqlStatementEnv struct {
	Error  string            `json:"error,omitempty"`
	Bridge string            `json:"bridge,omitempty"`
	Use    string            `json:"use,omitempty"`
	Named  map[string]string `json:"named,omitempty"`
}

func (sse *SqlStatementEnv) Reset() {
	sse.Error = ""
	sse.Bridge = ""
	sse.Use = ""
	sse.Named = nil
}

type SqlStatement struct {
	Text      string           `json:"text"`
	BeginLine int              `json:"beginLine"`
	EndLine   int              `json:"endLine"`
	IsComment bool             `json:"isComment"`
	StmtType  string           `json:"stmtType,omitempty"`
	Env       *SqlStatementEnv `json:"env,omitempty"`
}

var doubleDashAsFlags = [][]string{{"explain"}, {"desc"}, {"show", "tables"}, {"show", "table"}}

func treatDoubleDashAsFlag(statement string) bool {
	tokens := SplitFields(strings.TrimSpace(statement), true)
	if len(tokens) == 0 {
		return false
	}

	for _, prefix := range doubleDashAsFlags {
		if len(tokens) < len(prefix) {
			continue
		}
		matched := true
		for i, token := range prefix {
			if !strings.EqualFold(tokens[i], token) {
				matched = false
				break
			}
		}
		if !matched {
			continue
		}
		for _, token := range tokens[len(prefix):] {
			if !strings.HasPrefix(token, "--") {
				matched = false
				break
			}
		}
		if matched {
			return true
		}
	}
	return false
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
					if treatDoubleDashAsFlag(buffer.String()) {
						buffer.WriteString("--")
						inSingleDash = false
						continue
					}
					commentBuffer.Reset()
					inSingleLineComment = true
					commentBuffer.WriteString("--")
					inSingleDash = false
					continue
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
				statementText := buffer.String() + ";"
				statements = append(statements, newSqlStatement(statementText, statementStartLine, lineNumber, env))
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
		statementText := buffer.String()
		statements = append(statements, newSqlStatement(statementText, statementStartLine, lineNumber, env))
	}

	return statements, scanner.Err()
}

func newSqlStatement(text string, beginLine, endLine int, env *SqlStatementEnv) *SqlStatement {
	return &SqlStatement{
		Text:      text,
		BeginLine: beginLine,
		EndLine:   endLine,
		IsComment: false,
		StmtType:  detectSqlStatementType(text),
		Env:       envForSqlStatement(env, namedMarkers(text)),
	}
}

func envForSqlStatement(env *SqlStatementEnv, markers []string) *SqlStatementEnv {
	if env == nil {
		env = &SqlStatementEnv{}
	}
	ret := &SqlStatementEnv{
		Error:  env.Error,
		Bridge: env.Bridge,
		Use:    env.Use,
	}
	if len(markers) == 0 {
		return ret
	}

	values := make(map[string]string, len(env.Named))
	for name, value := range env.Named {
		values[strings.ToLower(name)] = value
	}
	missing := make([]string, 0)
	for _, marker := range markers {
		value, ok := values[strings.ToLower(marker)]
		if !ok {
			missing = append(missing, marker)
			continue
		}
		if ret.Named == nil {
			ret.Named = make(map[string]string, len(markers))
		}
		ret.Named[marker] = value
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		message := namedMarkerError(missing)
		if ret.Error == "" {
			ret.Error = message
		} else {
			ret.Error += "; " + message
		}
	}
	return ret
}

func namedMarkerError(markers []string) string {
	if len(markers) == 1 {
		return fmt.Sprintf("named parameter %q is not defined in the environment", markers[0])
	}
	quoted := make([]string, len(markers))
	for i, marker := range markers {
		quoted[i] = fmt.Sprintf("%q", marker)
	}
	return fmt.Sprintf("named parameters %s are not defined in the environment", strings.Join(quoted, ", "))
}

func namedMarkers(statement string) []string {
	var markers []string
	seen := make(map[string]struct{})
	for idx := 0; idx < len(statement); {
		switch statement[idx] {
		case '\'', '"':
			idx = skipSqlQuotedString(statement, idx, statement[idx])
		case '-':
			if idx+1 < len(statement) && statement[idx+1] == '-' {
				idx = skipSqlLineComment(statement, idx+2)
			} else {
				idx++
			}
		case '/':
			if idx+1 < len(statement) && statement[idx+1] == '/' {
				idx = skipSqlLineComment(statement, idx+2)
			} else if idx+1 < len(statement) && statement[idx+1] == '*' {
				idx = skipSqlBlockComment(statement, idx+2)
			} else {
				idx++
			}
		case ':':
			if idx > 0 && statement[idx-1] == ':' || idx+1 >= len(statement) || !isNamedMarkerStart(statement[idx+1]) {
				idx++
				continue
			}
			end := idx + 2
			for end < len(statement) && isNamedMarkerPart(statement[end]) {
				end++
			}
			if end < len(statement) && statement[end] == '-' {
				idx = end
				continue
			}
			marker := statement[idx+1 : end]
			key := strings.ToLower(marker)
			if _, ok := seen[key]; !ok {
				seen[key] = struct{}{}
				markers = append(markers, marker)
			}
			idx = end
		default:
			idx++
		}
	}
	return markers
}

func isNamedMarkerStart(char byte) bool {
	return char == '_' || char >= 'A' && char <= 'Z' || char >= 'a' && char <= 'z'
}

func isNamedMarkerPart(char byte) bool {
	return isNamedMarkerStart(char) || char >= '0' && char <= '9'
}

func skipSqlQuotedString(text string, start int, quote byte) int {
	for idx := start + 1; idx < len(text); idx++ {
		if text[idx] == '\\' && idx+1 < len(text) {
			idx++
			continue
		}
		if text[idx] != quote {
			continue
		}
		if idx+1 < len(text) && text[idx+1] == quote {
			idx++
			continue
		}
		return idx + 1
	}
	return len(text)
}

func skipSqlLineComment(text string, start int) int {
	for idx := start; idx < len(text); idx++ {
		if text[idx] == '\n' {
			return idx + 1
		}
	}
	return len(text)
}

func skipSqlBlockComment(text string, start int) int {
	for idx := start; idx+1 < len(text); idx++ {
		if text[idx] == '*' && text[idx+1] == '/' {
			return idx + 2
		}
	}
	return len(text)
}

func detectSqlStatementType(statement string) string {
	tokens := SplitFields(strings.TrimSpace(statement), true)
	if len(tokens) == 0 {
		return ""
	}

	primary := normalizeSqlKeyword(tokens[0])
	if primary == "" {
		return ""
	}

	if primary == "WITH" {
		for _, token := range tokens[1:] {
			kw := normalizeSqlKeyword(token)
			switch kw {
			case "SELECT", "INSERT", "UPDATE", "DELETE",
				"MERGE", "CREATE", "ALTER", "DROP", "TRUNCATE",
				"EXPLAIN", "SHOW", "DESC", "DESCRIBE",
				"CALL", "EXEC", "EXECUTE", "GRANT", "REVOKE":
				return strings.ToLower(kw)
			}
		}
	}

	return strings.ToLower(primary)
}

func normalizeSqlKeyword(token string) string {
	if token == "" {
		return ""
	}

	start := 0
	for start < len(token) {
		r, size := utf8.DecodeRuneInString(token[start:])
		if unicode.IsLetter(r) {
			break
		}
		start += size
	}

	end := len(token)
	for end > start {
		r, size := utf8.DecodeLastRuneInString(token[:end])
		if unicode.IsLetter(r) {
			break
		}
		end -= size
	}

	if start >= end {
		return ""
	}
	return strings.ToUpper(token[start:end])
}

func ParseStatementEnv(prev *SqlStatementEnv, text string) (*SqlStatementEnv, error) {
	text = strings.TrimSpace(strings.TrimPrefix(text, "--"))
	if !strings.HasPrefix(text, "env:") {
		return prev, nil
	}
	// -- env: bridge=sqlite
	// -- env: reset
	// -- env: use=mydb
	// -- env: named.name=Alice named.from="2024-01-01" named.to="2024-01-08"
	text = strings.TrimSpace(strings.TrimPrefix(text, "env:"))
	pairs := ParseNameValuePairs(text)
	if len(pairs) == 0 {
		return prev, nil
	}
	env := &SqlStatementEnv{}
	*env = *prev
	namedCloned := false
	for _, pair := range pairs {
		switch {
		case pair.Name == "bridge":
			env.Bridge = pair.Value
		case pair.Name == "use":
			env.Use = pair.Value
		case pair.Name == "reset":
			env.Reset()
			namedCloned = true
		case strings.HasPrefix(pair.Name, "named."):
			bindName := strings.TrimPrefix(pair.Name, "named.")
			if bindName == "" {
				env.Error = fmt.Sprintf("unknown env: %s", pair.Name)
				continue
			}
			if !namedCloned {
				// copy-on-write: keep prev.Named untouched until the first mutation
				named := make(map[string]string, len(prev.Named)+1)
				for k, v := range prev.Named {
					named[k] = v
				}
				env.Named = named
				namedCloned = true
			}
			env.Named[bindName] = pair.Value
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

var parseNameValuePairsRegexp = regexp.MustCompile(`([\w-_.]+)(?:=("([^"\\]*(\\.[^"\\]*)*)"|'([^'\\]*(\\.[^'\\]*)*)'|[^ ]+))?`)

// ParseNameValuePairs parses multiple name=value pairs
// where values can contain whitespace within single or double quotation marks.
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
		} else if len(value) >= 2 && (value[0] == '"' || value[0] == '\'') && value[len(value)-1] == value[0] {
			value = value[1 : len(value)-1]
			if match[2][0] == '"' {
				value = strings.ReplaceAll(value, `\"`, `"`)
			} else {
				value = strings.ReplaceAll(value, `\'`, `'`)
			}
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
				cols[i] = fmt.Sprintf("%#v", r)
			}
		}
	}
	return cols
}

type Stringify interface {
	String() string
}

type HttpStatement struct {
	Text      string `json:"text"`
	BeginLine int    `json:"beginLine"`
	EndLine   int    `json:"endLine"`
}

// SplitHttpStatements parses multiple HTTP statements from the reader.
// Each HTTP statement is separated by at least three continuous '#' characters.
func SplitHttpStatements(reader io.Reader) ([]*HttpStatement, error) {
	var statements []*HttpStatement
	var buffer bytes.Buffer
	scanner := bufio.NewScanner(reader)
	scanner.Split(bufio.ScanLines)

	lineNumber := 0
	statementStartLine := 1

	for scanner.Scan() {
		line := scanner.Text()
		lineNumber++

		if strings.HasPrefix(strings.TrimSpace(line), "###") {
			if buffer.Len() > 0 {
				text := buffer.String()
				if strings.TrimSpace(text) != "" {
					statements = append(statements, &HttpStatement{
						Text:      buffer.String(),
						BeginLine: statementStartLine,
						EndLine:   lineNumber - 1,
					})
				}
				buffer.Reset()
			}
			statementStartLine = lineNumber + 1
			continue
		}

		buffer.WriteString(line)
		buffer.WriteString("\n")
	}

	if buffer.Len() > 0 {
		statements = append(statements, &HttpStatement{
			Text:      buffer.String(),
			BeginLine: statementStartLine,
			EndLine:   lineNumber,
		})
	}

	return statements, scanner.Err()
}
