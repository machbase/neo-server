package sqlext

import "github.com/yuin/goldmark/ast"

type NodeKind int

const (
	KindBlock ast.NodeKind = ast.NodeKind(2000)
)

type Block struct {
	ast.BaseBlock
	Source   string
	Options  Options
	Output   string
	Error    string
	TimedOut bool
}

func (b *Block) Kind() ast.NodeKind { return ast.NodeKind(KindBlock) }

func (b *Block) Dump(source []byte, level int) {
	ast.DumpHelper(b, source, level, nil, nil)
}
