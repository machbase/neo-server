package d2ext

import (
	"bufio"
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
)

func TestExtenderExtend(t *testing.T) {
	md := goldmark.New()

	require.NotPanics(t, func() {
		(&Extender{}).Extend(md)
	})
}

func TestTransformerReplacesOnlyD2Blocks(t *testing.T) {
	source := []byte("d2\na -> b\ngo\nfmt.Println(\"x\")\n")
	segmentOf := func(content string) text.Segment {
		start := bytes.Index(source, []byte(content))
		require.NotEqual(t, -1, start)
		return text.NewSegment(start, start+len(content))
	}

	reader := text.NewReader(source)
	doc := ast.NewDocument()

	d2Block := ast.NewFencedCodeBlock(ast.NewTextSegment(segmentOf("d2")))
	d2Lines := text.NewSegments()
	d2Lines.Append(segmentOf("a -> b"))
	d2Block.SetLines(d2Lines)
	doc.AppendChild(doc, d2Block)

	goBlock := ast.NewFencedCodeBlock(ast.NewTextSegment(segmentOf("go")))
	goLines := text.NewSegments()
	goLines.Append(segmentOf(`fmt.Println("x")`))
	goBlock.SetLines(goLines)
	doc.AppendChild(doc, goBlock)

	(&Transformer{}).Transform(doc, reader, nil)

	var d2Blocks int
	var fencedBlocks int
	ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch n := node.(type) {
		case *Block:
			d2Blocks++
			require.Equal(t, KindBlock, n.Kind())
		case *ast.FencedCodeBlock:
			fencedBlocks++
			require.Equal(t, []byte("go"), n.Language(source))
		}
		return ast.WalkContinue, nil
	})

	require.Equal(t, 1, d2Blocks)
	require.Equal(t, 1, fencedBlocks)
}

func TestBlockHelpersAndRendererWithEmptyBlock(t *testing.T) {
	blankSource := []byte(" \n")
	blankBlock := &Block{}
	blankLines := text.NewSegments()
	blankLines.Append(text.NewSegment(0, 1))
	blankBlock.SetLines(blankLines)
	blankBlock.AppendChild(blankBlock, ast.NewTextSegment(text.NewSegment(0, 1)))
	require.True(t, blankBlock.IsBlank(blankSource))
	require.NotPanics(t, func() { blankBlock.Dump(blankSource, 0) })

	nonBlankSource := []byte("x\n")
	nonBlankBlock := &Block{}
	nonBlankLines := text.NewSegments()
	nonBlankLines.Append(text.NewSegment(0, 1))
	nonBlankBlock.SetLines(nonBlankLines)
	nonBlankBlock.AppendChild(nonBlankBlock, ast.NewTextSegment(text.NewSegment(0, 1)))
	require.False(t, nonBlankBlock.IsBlank(nonBlankSource))

	backing := &bytes.Buffer{}
	buf := bufio.NewWriter(backing)
	emptyBlock := &Block{}
	emptyBlock.SetLines(text.NewSegments())
	renderer := &HTMLRenderer{}

	status, err := renderer.Render(buf, nil, emptyBlock, true)
	require.NoError(t, err)
	require.Equal(t, ast.WalkContinue, status)

	status, err = renderer.Render(buf, nil, emptyBlock, false)
	require.NoError(t, err)
	require.Equal(t, ast.WalkContinue, status)
	require.NoError(t, buf.Flush())
	require.Equal(t, `<div class="d2"></div>`, backing.String())

	ptr := Pointer(42)
	require.NotNil(t, ptr)
	require.Equal(t, 42, *ptr)
}

type nodeRendererRegistererStub struct {
	kinds []ast.NodeKind
}

func (s *nodeRendererRegistererStub) Register(kind ast.NodeKind, _ renderer.NodeRendererFunc) {
	s.kinds = append(s.kinds, kind)
}

func TestHTMLRendererRegisterAndRender(t *testing.T) {
	reg := &nodeRendererRegistererStub{}
	r := &HTMLRenderer{}
	r.RegisterFuncs(reg)
	require.Contains(t, reg.kinds, KindBlock)

	source := []byte("a -> b\n")
	block := &Block{}
	lines := text.NewSegments()
	lines.Append(text.NewSegment(0, len(source)-1))
	block.SetLines(lines)

	backing := &bytes.Buffer{}
	buf := bufio.NewWriter(backing)
	status, err := r.Render(buf, source, block, true)
	require.NoError(t, err)
	require.Equal(t, ast.WalkContinue, status)
	status, err = r.Render(buf, source, block, false)
	require.NoError(t, err)
	require.Equal(t, ast.WalkContinue, status)
	require.NoError(t, buf.Flush())
	require.Contains(t, backing.String(), `<div class="d2">`)
	require.Contains(t, backing.String(), `<svg`)
}
