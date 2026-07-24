package katex

import (
	"bytes"
	"strings"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

type Transformer struct{}

func (t *Transformer) Transform(doc *ast.Document, reader text.Reader, _ parser.Context) {
	source := reader.Source()
	for node := doc.FirstChild(); node != nil; {
		next := node.NextSibling()

		para, ok := node.(*ast.Paragraph)
		if !ok {
			node = next
			continue
		}

		raw := bytes.TrimSpace(para.Text(source))
		if len(raw) < 2 || !bytes.HasPrefix(raw, []byte("$$")) {
			node = next
			continue
		}

		inlineOpts, firstPart := parseBlockOpening(raw)

		parent := para.Parent()
		if parent == nil {
			node = next
			continue
		}

		equation := &bytes.Buffer{}
		appendPart := func(part []byte) {
			part = bytes.TrimSpace(part)
			if len(part) == 0 {
				return
			}
			if equation.Len() > 0 {
				equation.WriteString("\n\n")
			}
			equation.Write(part)
		}

		removeNodes := []ast.Node{para}
		foundClose := false

		if body, closed := stripClosingDelimiter(firstPart); closed {
			appendPart(body)
			foundClose = true
		} else {
			appendPart(firstPart)
			cursor := next
			for cursor != nil {
				mid, ok := cursor.(*ast.Paragraph)
				if !ok {
					break
				}

				midRaw := bytes.TrimSpace(mid.Text(source))
				removeNodes = append(removeNodes, mid)
				if body, closed := stripClosingDelimiter(midRaw); closed {
					appendPart(body)
					foundClose = true
					break
				}
				appendPart(midRaw)
				cursor = mid.NextSibling()
			}
		}

		if !foundClose {
			node = next
			continue
		}

		block := &Block{Equation: append([]byte(nil), equation.Bytes()...), Options: inlineOpts}
		parent.ReplaceChild(parent, para, block)
		for _, rem := range removeNodes[1:] {
			parent.RemoveChild(parent, rem)
		}

		node = block
	}
}

func parseBlockOpening(raw []byte) (map[string]any, []byte) {
	afterPrefix := raw[2:]
	trimmed := bytes.TrimSpace(afterPrefix)
	if len(trimmed) == 0 {
		return nil, nil
	}

	if !(len(afterPrefix) > 0 && isSpace(afterPrefix[0]) && trimmed[0] == '{') {
		return nil, trimmed
	}

	optRaw, remainder, ok := splitLeadingBraceBlock(trimmed)
	if !ok {
		return nil, trimmed
	}

	opts, err := parseInlineOptions(string(optRaw))
	if err != nil {
		return nil, trimmed
	}

	return opts, bytes.TrimSpace(remainder)
}

func stripClosingDelimiter(raw []byte) ([]byte, bool) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) < 2 || !bytes.HasSuffix(trimmed, []byte("$$")) {
		return trimmed, false
	}
	return bytes.TrimSpace(trimmed[:len(trimmed)-2]), true
}

func splitLeadingBraceBlock(raw []byte) ([]byte, []byte, bool) {
	s := string(raw)
	if !strings.HasPrefix(s, "{") {
		return nil, nil, false
	}

	depth := 0
	inQuote := false
	escaped := false
	for idx, r := range s {
		if inQuote {
			if escaped {
				escaped = false
				continue
			}
			if r == '\\' {
				escaped = true
				continue
			}
			if r == '"' {
				inQuote = false
			}
			continue
		}

		switch r {
		case '"':
			inQuote = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				inside := []byte(s[1:idx])
				remainder := []byte(s[idx+1:])
				return inside, remainder, true
			}
			if depth < 0 {
				return nil, nil, false
			}
		}
	}

	return nil, nil, false
}

func isSpace(ch byte) bool {
	return ch == ' ' || ch == '\t' || ch == '\r' || ch == '\n'
}
