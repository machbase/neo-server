package httpext

import "github.com/yuin/goldmark/ast"

type Block struct {
	ast.BaseBlock
	Request         string
	Response        string
	ShowRequest     bool
	ShowLineNumbers bool
	IndentJSON      bool
	ClassStyles     map[string]string
	Warnings        []string
	ExecuteError    string
}

var KindBlock = ast.NewNodeKind("HttpClientBlock")

func (n *Block) Kind() ast.NodeKind {
	return KindBlock
}

func (n *Block) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, nil, nil)
}
