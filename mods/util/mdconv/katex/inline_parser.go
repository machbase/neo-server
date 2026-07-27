package katex

import (
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

type InlineParser struct{}

func (p *InlineParser) Trigger() []byte {
	return []byte{'$'}
}

func (p *InlineParser) Parse(parent ast.Node, block text.Reader, pc parser.Context) ast.Node {
	line, _ := block.PeekLine()
	if len(line) < 4 || line[0] != '$' || line[1] != '`' {
		return nil
	}

	for i := 2; i < len(line)-1; i++ {
		if line[i] == '`' && line[i+1] == '$' {
			if i == 2 {
				return nil
			}
			equation := append([]byte(nil), line[2:i]...)
			block.Advance(i + 2)
			return &Inline{Equation: equation}
		}
	}

	return nil
}
