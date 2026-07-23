package httpext

import (
	"bytes"
	"compress/gzip"
	_ "embed"
	"encoding/json"
	"html"
	"io"
	"net/url"
	"strconv"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"
)

type HTMLRenderer struct{}

func (r *HTMLRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(KindBlock, r.Render)
}

func (r *HTMLRenderer) Render(w util.BufWriter, _ []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	n := node.(*Block)
	styles := mergeClassStyles(n.ClassStyles)
	_, _ = w.WriteString(`<style>` + defaultCSS + `</style>`)
	for _, warn := range n.Warnings {
		_, _ = w.WriteString(`<div class="httpext-warning">` + html.EscapeString(warn) + `</div>`)
	}
	requestHTML := ""
	if n.ShowRequest {
		requestHTML = renderHTTPMessage(n.Request, true, styles, n.IndentJSON)
	}
	responseHTML := renderHTTPMessage(n.Response, false, styles, n.IndentJSON)
	_, _ = w.WriteString(renderCombinedHTTPMessages(requestHTML, responseHTML, n.ShowRequest, n.ShowLineNumbers))
	return ast.WalkContinue, nil
}

//go:embed renderer.css
var defaultCSS string

func mergeClassStyles(overrides map[string]string) map[string]string {
	ret := map[string]string{}
	for _, className := range styleKeyToClassName {
		ret[className] = ""
	}
	for k, v := range overrides {
		ret[k] = v
	}
	return ret
}

func span(className, value string, styles map[string]string) string {
	return spanWithClasses([]string{className}, value, styles)
}

func spanWithClasses(classNames []string, value string, styles map[string]string) string {
	if len(classNames) == 0 {
		return html.EscapeString(value)
	}
	styleAttr := ""
	if styles != nil {
		for _, className := range classNames {
			if css, ok := styles[className]; ok && strings.TrimSpace(css) != "" {
				styleAttr = ` style="` + html.EscapeString(css) + `"`
				break
			}
		}
	}
	return `<span class="` + html.EscapeString(strings.Join(classNames, " ")) + `"` + styleAttr + `>` + html.EscapeString(value) + `</span>`
}

func renderHTTPMessage(raw string, isRequest bool, styles map[string]string, indentJSON bool) string {
	headerPart, bodyPart := splitHTTPMessage(raw)
	headers := splitLinesKeepLF(headerPart)
	b := &strings.Builder{}
	if len(headers) > 0 {
		first := strings.TrimRight(headers[0], "\r\n")
		if isRequest {
			b.WriteString(renderRequestLine(first, styles))
		} else {
			b.WriteString(renderResponseLine(first, styles))
		}
		b.WriteString("\n")
	}
	contentType := ""
	contentEncoding := ""
	for i := 1; i < len(headers); i++ {
		line := strings.TrimRight(headers[i], "\r\n")
		if line == "" {
			continue
		}
		name, val, ok := parseHeaderLine(line)
		if !ok {
			b.WriteString(html.EscapeString(line))
			b.WriteString("\n")
			continue
		}
		if strings.EqualFold(name, "Content-Type") {
			contentType = strings.ToLower(strings.TrimSpace(strings.SplitN(val, ";", 2)[0]))
		}
		if strings.EqualFold(name, "Content-Encoding") {
			contentEncoding = strings.ToLower(strings.TrimSpace(val))
		}
		b.WriteString(span("httpext-header-key", name, styles))
		b.WriteString(": ")
		b.WriteString(span("httpext-header-value", val, styles))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	if bodyPart != "" {
		displayBody := decodeResponseBody(bodyPart, contentType, contentEncoding)
		if isJSONContentType(contentType) {
			if indentJSON && strings.ToLower(contentType) != "application/x-ndjson" {
				displayBody = prettyJSON(displayBody)
			}
			b.WriteString(renderJSONBody(displayBody, styles))
		} else if isCSVContentType(contentType) {
			b.WriteString(renderCSVBody(displayBody, styles))
		} else {
			b.WriteString(span("httpext-body", displayBody, styles))
		}
	}
	return b.String()
}

func renderCombinedHTTPMessages(requestHTML string, responseHTML string, showRequest bool, showLineNumbers bool) string {
	if showLineNumbers {
		return renderWithLineNumbers(requestHTML, responseHTML, showRequest)
	}
	b := &strings.Builder{}
	b.WriteString(`<div class="httpext-pre">`)
	if showRequest {
		b.WriteString(`<div class="httpext-line">`)
		b.WriteString(requestHTML)
		b.WriteString(`</div><div class="httpext-divider"></div>`)
	}
	b.WriteString(`<div class="httpext-line">`)
	b.WriteString(responseHTML)
	b.WriteString(`</div></div>`)
	return b.String()
}

func renderWithLineNumbers(requestHTML string, responseHTML string, showRequest bool) string {
	b := &strings.Builder{}
	b.WriteString(`<div class="httpext-pre"><table class="httpext-table"><tbody>`)
	lineNo := 1
	if showRequest {
		lineNo = writeNumberedLines(b, requestHTML, lineNo)
		b.WriteString(`<tr class="httpext-divider-row"><td class="httpext-lno">&nbsp;</td><td class="httpext-line"><div class="httpext-divider"></div></td></tr>`)
	}
	_ = writeNumberedLines(b, responseHTML, lineNo)
	b.WriteString(`</tbody></table></div>`)
	return b.String()
}

func writeNumberedLines(b *strings.Builder, contentHTML string, startLine int) int {
	lines := strings.Split(strings.ReplaceAll(contentHTML, "\r\n", "\n"), "\n")
	if len(lines) == 0 {
		lines = []string{""}
	}
	lineNo := startLine
	for _, line := range lines {
		cell := line
		if cell == "" {
			cell = "&nbsp;"
		}
		b.WriteString(`<tr><td class="httpext-lno">`)
		b.WriteString(strconv.Itoa(lineNo))
		b.WriteString(`</td><td class="httpext-line">`)
		b.WriteString(cell)
		b.WriteString(`</td></tr>`)
		lineNo++
	}
	return lineNo
}

func splitHTTPMessage(raw string) (string, string) {
	if parts := strings.SplitN(raw, "\r\n\r\n", 2); len(parts) == 2 {
		return parts[0], parts[1]
	}
	if parts := strings.SplitN(raw, "\n\n", 2); len(parts) == 2 {
		return parts[0], parts[1]
	}
	return raw, ""
}

func splitLinesKeepLF(s string) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

func renderRequestLine(line string, styles map[string]string) string {
	parts := strings.Fields(line)
	if len(parts) < 2 {
		return html.EscapeString(line)
	}
	b := &strings.Builder{}
	b.WriteString(span("httpext-method", parts[0], styles))
	b.WriteString(" ")
	b.WriteString(renderRequestTarget(parts[1], styles))
	if len(parts) > 2 {
		b.WriteString(" ")
		b.WriteString(span("httpext-request-protocol", parts[2], styles))
	}
	return b.String()
}

func renderRequestTarget(target string, styles map[string]string) string {
	idx := strings.Index(target, "?")
	if idx < 0 {
		return span("httpext-path", target, styles)
	}
	pathPart := target[:idx]
	queryPart := target[idx+1:]
	b := &strings.Builder{}
	b.WriteString(span("httpext-path", pathPart, styles))
	b.WriteString("?")
	pairs := strings.Split(queryPart, "&")
	for i, p := range pairs {
		if i > 0 {
			b.WriteString("&")
		}
		kv := strings.SplitN(p, "=", 2)
		name, _ := urlQueryUnescape(kv[0])
		b.WriteString(span("httpext-param-name", name, styles))
		if len(kv) > 1 {
			val, _ := urlQueryUnescape(kv[1])
			b.WriteString("=")
			b.WriteString(span("httpext-param-value", val, styles))
		}
	}
	return b.String()
}

func renderResponseLine(line string, styles map[string]string) string {
	parts := strings.Fields(line)
	if len(parts) < 2 {
		return html.EscapeString(line)
	}
	b := &strings.Builder{}
	b.WriteString(span("httpext-response-protocol", parts[0], styles))
	b.WriteString(" ")
	b.WriteString(span("httpext-status-code", parts[1], styles))
	if len(parts) > 2 {
		b.WriteString(" ")
		b.WriteString(span("httpext-status-message", strings.Join(parts[2:], " "), styles))
	}
	return b.String()
}

func parseHeaderLine(line string) (string, string, bool) {
	idx := strings.Index(line, ":")
	if idx <= 0 {
		return "", "", false
	}
	return strings.TrimSpace(line[:idx]), strings.TrimSpace(line[idx+1:]), true
}

func isJSONContentType(contentType string) bool {
	if contentType == "application/json" || contentType == "application/x-ndjson" {
		return true
	}
	return strings.Contains(contentType, "+json")
}

func isPrintableContentType(contentType string) bool {
	if contentType == "" {
		return false
	}
	if strings.HasPrefix(contentType, "text/") {
		return true
	}
	switch contentType {
	case "application/json",
		"application/javascript",
		"application/x-ndjson",
		"application/xml",
		"application/xhtml+xml",
		"application/x-www-form-urlencoded",
		"application/atom+xml",
		"application/rss+xml",
		"application/geo+json",
		"application/hal+json",
		"application/hal+xml",
		"application/ld+json",
		"application/vnd.api+json",
		"application/vnd.collection+json",
		"application/vnd.geo+json":
		return true
	}
	return strings.Contains(contentType, "+json") || strings.Contains(contentType, "+xml")
}

func isCSVContentType(contentType string) bool {
	if contentType == "text/csv" || contentType == "application/csv" {
		return true
	}
	return strings.Contains(contentType, "+csv") || strings.Contains(contentType, "csv")
}

func urlQueryUnescape(v string) (string, error) {
	return url.QueryUnescape(v)
}

func decodeResponseBody(bodyPart string, contentType string, contentEncoding string) string {
	if contentEncoding != "gzip" || !isPrintableContentType(contentType) {
		return bodyPart
	}
	zr, err := gzip.NewReader(bytes.NewReader([]byte(bodyPart)))
	if err != nil {
		return bodyPart
	}
	defer zr.Close()
	decoded, err := io.ReadAll(zr)
	if err != nil {
		return bodyPart
	}
	return string(decoded)
}

func prettyJSON(input string) string {
	var out bytes.Buffer
	if err := json.Indent(&out, []byte(input), "", "  "); err != nil {
		return input
	}
	return out.String()
}

func renderJSONBody(body string, styles map[string]string) string {
	lexer := lexers.Get("json")
	if lexer == nil {
		return span("httpext-body", body, styles)
	}
	it, err := lexer.Tokenise(nil, body)
	if err != nil {
		return span("httpext-body", body, styles)
	}

	tokens := []chroma.Token{}
	for token := it(); token != chroma.EOF; token = it() {
		tokens = append(tokens, token)
	}

	b := &strings.Builder{}
	for i, tok := range tokens {
		if tok.Value == "" {
			continue
		}
		className := classifyJSONToken(tokens, i)
		if className == "" {
			b.WriteString(html.EscapeString(tok.Value))
			continue
		}
		b.WriteString(span(className, tok.Value, styles))
	}
	return b.String()
}

func renderCSVBody(body string, styles map[string]string) string {
	delim := detectCSVDelimiter(body)
	lines := strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n")
	b := &strings.Builder{}
	for i, line := range lines {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(renderCSVLine(line, delim, styles))
	}
	return b.String()
}

func renderCSVLine(line string, delim byte, styles map[string]string) string {
	fields, _ := splitCSVFields(line, delim)
	b := &strings.Builder{}
	for i, field := range fields {
		className := "httpext-csv-col-" + strconv.Itoa(i)
		paletteClass := "httpext-csv-col-p" + strconv.Itoa(i%12)
		b.WriteString(spanWithClasses([]string{className, paletteClass}, field, styles))
		if i < len(fields)-1 {
			b.WriteString(span("httpext-csv-delim", string(delim), styles))
		}
	}
	return b.String()
}

func detectCSVDelimiter(body string) byte {
	const defaultDelimiter = byte(',')
	lines := splitLinesForDelimiterDetection(body)
	if len(lines) == 0 {
		return defaultDelimiter
	}
	candidates := []byte{',', '|', ';', '\t'}
	best := defaultDelimiter
	bestScore := -1

	for _, delim := range candidates {
		score, modeCols := scoreDelimiter(lines, delim)
		if modeCols <= 1 {
			continue
		}
		if score > bestScore {
			bestScore = score
			best = delim
		}
	}
	return best
}

func splitLinesForDelimiterDetection(body string) []string {
	normalized := strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n")
	lines := make([]string, 0, len(normalized))
	for _, line := range normalized {
		if strings.TrimSpace(line) == "" {
			continue
		}
		lines = append(lines, line)
		if len(lines) >= 64 {
			break
		}
	}
	return lines
}

func scoreDelimiter(lines []string, delim byte) (int, int) {
	freq := map[int]int{}
	totalCols := 0
	validLines := 0
	badLines := 0

	for _, line := range lines {
		fields, ok := splitCSVFields(line, delim)
		if !ok {
			badLines++
			continue
		}
		cols := len(fields)
		validLines++
		totalCols += cols
		freq[cols]++
	}
	if validLines == 0 {
		return -1, 0
	}
	modeCols := 1
	modeFreq := 0
	for cols, count := range freq {
		if count > modeFreq || (count == modeFreq && cols > modeCols) {
			modeCols = cols
			modeFreq = count
		}
	}
	score := modeFreq*100 + modeCols*10 + (totalCols / validLines) - badLines*50
	if delim == ',' {
		score++
	}
	return score, modeCols
}

func splitCSVFields(line string, delim byte) ([]string, bool) {
	if line == "" {
		return []string{""}, true
	}
	fields := []string{}
	start := 0
	inQuotes := false
	for i := 0; i < len(line); i++ {
		ch := line[i]
		if ch == '"' {
			if inQuotes && i+1 < len(line) && line[i+1] == '"' {
				i++
				continue
			}
			inQuotes = !inQuotes
			continue
		}
		if ch == delim && !inQuotes {
			fields = append(fields, line[start:i])
			start = i + 1
		}
	}
	if inQuotes {
		return []string{line}, false
	}
	fields = append(fields, line[start:])
	return fields, true
}

func classifyJSONToken(tokens []chroma.Token, idx int) string {
	tok := tokens[idx]
	tt := tok.Type
	val := strings.TrimSpace(tok.Value)

	if isQuotedJSONLiteral(tok.Value) {
		if jsonStringIsKey(tokens, idx) {
			return "httpext-json-key"
		}
		return "httpext-json-string"
	}

	if tt == chroma.Punctuation {
		return "httpext-json-punct"
	}
	if tt.InSubCategory(chroma.LiteralNumber) {
		return "httpext-json-number"
	}
	if tt.InSubCategory(chroma.LiteralString) {
		return "httpext-json-string"
	}
	if strings.EqualFold(val, "true") || strings.EqualFold(val, "false") {
		return "httpext-json-boolean"
	}
	if strings.EqualFold(val, "null") {
		return "httpext-json-null"
	}
	if strings.TrimSpace(tok.Value) == "" {
		return ""
	}
	return "httpext-body"
}

func jsonStringIsKey(tokens []chroma.Token, idx int) bool {
	for i := idx + 1; i < len(tokens); i++ {
		next := tokens[i]
		if strings.TrimSpace(next.Value) == "" {
			continue
		}
		return strings.TrimSpace(next.Value) == ":"
	}
	return false
}

func isQuotedJSONLiteral(s string) bool {
	t := strings.TrimSpace(s)
	if len(t) < 2 {
		return false
	}
	return t[0] == '"' && t[len(t)-1] == '"'
}
