package katex

import (
	"bytes"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

type BlockParser struct{}

func (p *BlockParser) Trigger() []byte {
	return []byte{'$'}
}

func (p *BlockParser) Open(parent ast.Node, reader text.Reader, pc parser.Context) (ast.Node, parser.State) {
	line, _ := reader.PeekLine()
	if len(line) == 0 || !bytes.Equal(bytes.TrimSpace(line), []byte("$$")) {
		return nil, parser.NoChildren
	}
	reader.AdvanceLine()
	return &Block{}, parser.HasChildren
}

func (p *BlockParser) Continue(node ast.Node, reader text.Reader, pc parser.Context) parser.State {
	line, _ := reader.PeekLine()
	if line == nil {
		return parser.Close
	}

	n := node.(*Block)
	trimmed := bytes.TrimSpace(line)
	if bytes.Equal(trimmed, []byte("$$")) {
		reader.AdvanceLine()
		return parser.Close
	}

	if len(n.Equation) > 0 {
		n.Equation = append(n.Equation, '\n')
	}
	n.Equation = append(n.Equation, bytes.TrimRight(line, "\r\n")...)
	reader.AdvanceLine()
	return parser.Continue | parser.NoChildren
}

func (p *BlockParser) Close(node ast.Node, reader text.Reader, pc parser.Context) {}

func (p *BlockParser) CanInterruptParagraph() bool {
	return true
}

func (p *BlockParser) CanAcceptIndentedLine() bool {
	return false
}
