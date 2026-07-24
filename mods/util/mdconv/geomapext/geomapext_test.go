package geomapext

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

func TestTransformerReplacesOnlyGeomapBlocks(t *testing.T) {
	source := []byte("geomap\n{\"type\":\"FeatureCollection\",\"features\":[]}\ngo\nfmt.Println(\"x\")\n")
	segmentOf := func(content string) text.Segment {
		start := bytes.Index(source, []byte(content))
		require.NotEqual(t, -1, start)
		return text.NewSegment(start, start+len(content))
	}

	reader := text.NewReader(source)
	doc := ast.NewDocument()

	geomapBlock := ast.NewFencedCodeBlock(ast.NewTextSegment(segmentOf("geomap")))
	geomapLines := text.NewSegments()
	geomapLines.Append(segmentOf(`{"type":"FeatureCollection","features":[]}`))
	geomapBlock.SetLines(geomapLines)
	doc.AppendChild(doc, geomapBlock)

	goBlock := ast.NewFencedCodeBlock(ast.NewTextSegment(segmentOf("go")))
	goLines := text.NewSegments()
	goLines.Append(segmentOf(`fmt.Println("x")`))
	goBlock.SetLines(goLines)
	doc.AppendChild(doc, goBlock)

	(&Transformer{}).Transform(doc, reader, nil)

	var geomapBlocks int
	var fencedBlocks int
	ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch n := node.(type) {
		case *Block:
			geomapBlocks++
			require.Equal(t, KindBlock, n.Kind())
		case *ast.FencedCodeBlock:
			fencedBlocks++
			require.Equal(t, []byte("go"), n.Language(source))
		}
		return ast.WalkContinue, nil
	})

	require.Equal(t, 1, geomapBlocks)
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
	r := &HTMLRenderer{}
	r.RegisterFuncs(reg)
	require.Contains(t, reg.kinds, KindBlock)

	source := []byte(`[{"type":"marker","coordinates":[37.5,127.0],"properties":{"popup":{"content":"hello"}}}]` + "\n")
	block := &Block{Options: map[string]any{"width": "600px", "height": "320px", "tile": "default", "fit": "auto"}}
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
	require.Contains(t, html, `class="geomapext"`)
	require.Contains(t, html, `class="geomapext-map"`)
	require.Contains(t, html, `width:600px;height:320px`)
	require.Contains(t, html, `/web/geomap/leaflet.js`)
	require.Contains(t, html, `/web/geomap/leaflet.css`)
	require.Contains(t, html, `window.__geomapextLeafletLoaderPromises`)
	require.Contains(t, html, `L.map(`)
	require.Contains(t, html, `L.tileLayer`)
	require.Contains(t, html, `L.marker`)
}

func TestBlockHelpers(t *testing.T) {
	srcBlank := []byte(" \n\t")
	b := &Block{}
	b.AppendChild(b, ast.NewTextSegment(text.NewSegment(0, 1)))
	b.AppendChild(b, ast.NewTextSegment(text.NewSegment(1, 3)))
	require.True(t, b.IsBlank(srcBlank))

	srcNonBlank := []byte("x")
	b2 := &Block{}
	b2.AppendChild(b2, ast.NewTextSegment(text.NewSegment(0, 1)))
	require.False(t, b2.IsBlank(srcNonBlank))

	// Ensure dump path is exercised.
	require.NotPanics(t, func() {
		b2.Dump(srcNonBlank, 0)
	})
}
func TestRenderBranches(t *testing.T) {
	r := &HTMLRenderer{}

	t.Run("empty payload", func(t *testing.T) {
		b := &Block{Options: map[string]any{}}
		lines := text.NewSegments()
		b.SetLines(lines)

		var out bytes.Buffer
		writer := bufio.NewWriter(&out)

		status, err := r.Render(writer, nil, b, true)
		require.NoError(t, err)
		require.Equal(t, ast.WalkContinue, status)
		require.NoError(t, writer.Flush())
		require.Contains(t, out.String(), "Geomap JSON is empty")
	})

	t.Run("invalid fence options", func(t *testing.T) {
		src := []byte("[]")
		b := &Block{Options: map[string]any{"fit": "invalid"}}
		lines := text.NewSegments()
		lines.Append(text.NewSegment(0, len(src)))
		b.SetLines(lines)

		var out bytes.Buffer
		writer := bufio.NewWriter(&out)

		status, err := r.Render(writer, src, b, true)
		require.Error(t, err)
		require.Equal(t, ast.WalkContinue, status)
		require.NoError(t, writer.Flush())
		require.Contains(t, out.String(), "geomap fit must be auto, bounds or center")
	})
}

func TestBuildScriptAndSanitize(t *testing.T) {
	cfg := defaultRenderConfig(false)
	cfg.TileOption = `{"maxZoom":18}`

	script, err := buildScript("map-1", `{"x":"</script>"}`, cfg)
	require.NoError(t, err)
	require.NotContains(t, script, `{"x":"</script>"}`)
	require.Contains(t, script, `__payloadText =`)
	require.Contains(t, script, `__tileOptionRaw =`)
	require.Contains(t, script, `maxZoom`)

	cfg.TileOption = ""
	script, err = buildScript("map-2", `{}`, cfg)
	require.NoError(t, err)
	require.Contains(t, script, "var __tileOptionRaw = null;")
}

func TestParseInlineAndSplitErrors(t *testing.T) {
	opts, err := parseInlineOptions(`width=600px,center=[37.5,127.0],title="a,b"`)
	require.NoError(t, err)
	require.Equal(t, "600px", opts["width"])
	require.Equal(t, []any{37.5, 127.0}, opts["center"])
	require.Equal(t, "a,b", opts["title"])

	_, err = parseInlineOptions(`width`)
	require.Error(t, err)

	_, err = parseInlineOptions(` =x`)
	require.Error(t, err)

	_, err = splitTopLevel("a,b}", ',')
	require.Error(t, err)
	_, err = splitTopLevel("a,b]", ',')
	require.Error(t, err)
	_, err = splitTopLevel(`a,"b`, ',')
	require.Error(t, err)
	_, err = splitTopLevel("a,[b", ',')
	require.Error(t, err)
}

func TestParseOptionValueAndConverters(t *testing.T) {
	v, err := parseOptionValue(`"quoted"`)
	require.NoError(t, err)
	require.Equal(t, "quoted", v)

	v, err = parseOptionValue("[1,2,3]")
	require.NoError(t, err)
	require.Equal(t, []any{int64(1), int64(2), int64(3)}, v)

	v, err = parseOptionValue("true")
	require.NoError(t, err)
	require.Equal(t, true, v)

	v, err = parseOptionValue("12")
	require.NoError(t, err)
	require.Equal(t, int64(12), v)

	v, err = parseOptionValue("1.5")
	require.NoError(t, err)
	require.Equal(t, 1.5, v)

	v, err = parseOptionValue("raw")
	require.NoError(t, err)
	require.Equal(t, "raw", v)

	_, err = parseOptionValue("")
	require.Error(t, err)
	_, err = parseOptionValue(`"bad`)
	require.Error(t, err)
	_, err = parseOptionValue("[1,2")
	require.Error(t, err)

	_, ok := optionString(" x ")
	require.True(t, ok)
	_, ok = optionString(1)
	require.False(t, ok)

	f, ok := optionFloat64(int64(3))
	require.True(t, ok)
	require.Equal(t, 3.0, f)
	f, ok = optionFloat64("3.5")
	require.True(t, ok)
	require.Equal(t, 3.5, f)
	_, ok = optionFloat64(struct{}{})
	require.False(t, ok)

	i, ok := optionInt(float64(9.8))
	require.True(t, ok)
	require.Equal(t, 9, i)
	i, ok = optionInt("42")
	require.True(t, ok)
	require.Equal(t, 42, i)
	_, ok = optionInt(struct{}{})
	require.False(t, ok)

	pair, ok := optionFloat64Pair([]float64{1, 2})
	require.True(t, ok)
	require.Equal(t, [2]float64{1, 2}, pair)
	_, ok = optionFloat64Pair("x")
	require.False(t, ok)
}

func TestParseFenceOptionsAndTileTemplate(t *testing.T) {
	opts, err := parseFenceOptions("geomap {}")
	require.NoError(t, err)
	require.NotNil(t, opts)
	require.Len(t, opts, 0)

	opts, err = parseFenceOptions("geomap {bad}")
	require.Error(t, err)
	require.Nil(t, opts)

	require.True(t, isTileTemplateURL("https://tile.openstreetmap.org/{z}/{x}/{y}.png"))
	require.False(t, isTileTemplateURL("ftp://tile.openstreetmap.org/{z}/{x}/{y}.png"))
	require.False(t, isTileTemplateURL("https://tile.openstreetmap.org/{z}/{x}.png"))
}
