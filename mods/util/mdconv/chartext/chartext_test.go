package chartext

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

func TestTransformerReplacesOnlyChartBlocks(t *testing.T) {
	source := []byte("chart\noption = {xAxis:{type:'category'},yAxis:{type:'value'},series:[{type:'line',data:[1]}]}\ngo\nfmt.Println(\"x\")\n")
	segmentOf := func(content string) text.Segment {
		start := bytes.Index(source, []byte(content))
		require.NotEqual(t, -1, start)
		return text.NewSegment(start, start+len(content))
	}

	reader := text.NewReader(source)
	doc := ast.NewDocument()

	chartBlock := ast.NewFencedCodeBlock(ast.NewTextSegment(segmentOf("chart")))
	chartLines := text.NewSegments()
	chartLines.Append(segmentOf("option = {xAxis:{type:'category'},yAxis:{type:'value'},series:[{type:'line',data:[1]}]}"))
	chartBlock.SetLines(chartLines)
	doc.AppendChild(doc, chartBlock)

	goBlock := ast.NewFencedCodeBlock(ast.NewTextSegment(segmentOf("go")))
	goLines := text.NewSegments()
	goLines.Append(segmentOf(`fmt.Println("x")`))
	goBlock.SetLines(goLines)
	doc.AppendChild(doc, goBlock)

	(&Transformer{}).Transform(doc, reader, nil)

	var chartBlocks int
	var fencedBlocks int
	ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch n := node.(type) {
		case *Block:
			chartBlocks++
			require.Equal(t, KindBlock, n.Kind())
		case *ast.FencedCodeBlock:
			fencedBlocks++
			require.Equal(t, []byte("go"), n.Language(source))
		}
		return ast.WalkContinue, nil
	})

	require.Equal(t, 1, chartBlocks)
	require.Equal(t, 1, fencedBlocks)
}

type nodeRendererRegistererStub struct {
	kinds []ast.NodeKind
}

func (s *nodeRendererRegistererStub) Register(kind ast.NodeKind, _ renderer.NodeRendererFunc) {
	s.kinds = append(s.kinds, kind)
}

func TestHTMLRendererRegisterAndRender(t *testing.T) {
	reg := &nodeRendererRegistererStub{}
	r := &HTMLRenderer{DarkMode: true}
	r.RegisterFuncs(reg)
	require.Contains(t, reg.kinds, KindBlock)

	source := []byte("function x(v){return v;}\noption = {xAxis:{type:'category',data:['Mon']},yAxis:{type:'value'},series:[{type:'line',data:[1]}]};\n")
	block := &Block{Options: map[string]any{"width": "600px", "height": "400px", "theme": "dark"}}
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

	html := backing.String()
	require.Contains(t, html, `class="chartext"`)
	require.Contains(t, html, `class="chartext-echarts"`)
	require.Contains(t, html, `width:600px;height:400px`)
	require.Contains(t, html, `echarts.init`)
	require.Contains(t, html, `setOption`)
}
