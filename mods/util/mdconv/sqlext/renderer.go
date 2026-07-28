package sqlext

import (
	"fmt"
	"html"
	"strconv"
	"strings"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"
)

type HTMLRenderer struct{}

func (r *HTMLRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(KindBlock, r.Render)
}

func renderCSVContent(body string) string {
	const defaultDelimiter = byte(',')
	delim := detectCSVDelimiter(body)
	lines := strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n")
	var b strings.Builder
	for i, line := range lines {
		if i > 0 {
			b.WriteString("\n")
		}
		fields, _ := splitCSVFields(line, delim)
		for idx, field := range fields {
			className := "sqlext-csv-col-" + strconv.Itoa(idx)
			paletteClass := "sqlext-csv-col-p" + strconv.Itoa(idx%12)
			b.WriteString(fmt.Sprintf(`<span class="%s %s">%s</span>`, className, paletteClass, html.EscapeString(field)))
			if idx < len(fields)-1 {
				b.WriteString(`<span class="sqlext-csv-delim">`)
				b.WriteString(html.EscapeString(string(delim)))
				b.WriteString(`</span>`)
			}
		}
	}
	return b.String()
}

func detectCSVDelimiter(body string) byte {
	const defaultDelimiter = byte(',')
	lines := strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n")
	var candidates []byte = []byte{',', '|', ';', '\t'}
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

func scoreDelimiter(lines []string, delim byte) (int, int) {
	var total int
	var cols int
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Count(line, string(delim)) + 1
		if fields > cols {
			cols = fields
		}
		total += fields
	}
	return total, cols
}

func splitCSVFields(line string, delim byte) ([]string, bool) {
	if delim == 0 {
		return []string{line}, false
	}
	var fields []string
	var buf strings.Builder
	inQuote := false
	for i := 0; i < len(line); i++ {
		ch := line[i]
		if ch == '"' {
			if inQuote && i+1 < len(line) && line[i+1] == '"' {
				buf.WriteByte(ch)
				i++
			} else {
				inQuote = !inQuote
			}
			continue
		}
		if ch == delim && !inQuote {
			fields = append(fields, buf.String())
			buf.Reset()
			continue
		}
		buf.WriteByte(ch)
	}
	fields = append(fields, buf.String())
	return fields, true
}

func (r *HTMLRenderer) Render(w util.BufWriter, _ []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	bn := node.(*Block)
	var b strings.Builder
	b.WriteString(`<div class="sqlext">`)
	if bn.Options.Execute && bn.Source != "" {
		b.WriteString(`<div class="sqlext-source">`)
		b.WriteString(`<pre class="chroma"><code class="language-sql">`)
		b.WriteString(html.EscapeString(bn.Source))
		b.WriteString(`</code></pre>`)
		b.WriteString(`</div>`)
	}
	if bn.Options.Execute && bn.Options.Format != "none" {
		b.WriteString(`<div class="sqlext-result">`)
		b.WriteString(`<pre>`)
		if strings.EqualFold(bn.Options.Format, "csv") {
			b.WriteString(renderCSVContent(bn.Output))
		} else {
			b.WriteString(html.EscapeString(bn.Output))
		}
		b.WriteString(`</pre>`)
		b.WriteString(`</div>`)
	}
	b.WriteString(`</div>`)
	_, _ = w.WriteString(b.String())
	return ast.WalkContinue, nil
}
