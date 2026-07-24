package katex

import (
	"bufio"
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
)

func TestExtenderInlineAndBlockKaTeX(t *testing.T) {
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			&Extender{},
		),
		goldmark.WithRendererOptions(
			html.WithXHTML(),
		),
	)

	src := "before $`x^2+y^2`$ after\n\n$$\n\\int_0^1 x^2 dx\n$$\n"
	out := &bytes.Buffer{}
	err := md.Convert([]byte(src), out)
	require.NoError(t, err)

	htmlOut := out.String()
	require.Contains(t, htmlOut, `class="katex"`)
	require.Contains(t, htmlOut, `<math xmlns="http://www.w3.org/1998/Math/MathML">`)
	require.Contains(t, htmlOut, `display="block"`)
	require.NotContains(t, htmlOut, "$`x^2+y^2`$")
	require.NotContains(t, htmlOut, `<p>A = B</p>`)
	require.NotContains(t, htmlOut, `</span>A = B`)
}

func TestExtenderKeepsPlainDollarText(t *testing.T) {
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			&Extender{},
		),
		goldmark.WithRendererOptions(
			html.WithXHTML(),
		),
	)

	src := "price is $100 and not katex"
	out := &bytes.Buffer{}
	err := md.Convert([]byte(src), out)
	require.NoError(t, err)

	htmlOut := out.String()
	require.Contains(t, htmlOut, `price is $100 and not katex`)
	require.NotContains(t, htmlOut, `class="katex"`)
}

func TestExtenderWithWrapperClasses(t *testing.T) {
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			&Extender{Options: RenderOptions{InlineWrapperClass: "math-inline", BlockWrapperClass: "math-block"}},
		),
		goldmark.WithRendererOptions(
			html.WithXHTML(),
		),
	)

	src := "before $`x+y`$\n\n$$\nx^2\n$$\n"
	out := &bytes.Buffer{}
	err := md.Convert([]byte(src), out)
	require.NoError(t, err)

	htmlOut := out.String()
	require.Contains(t, htmlOut, `class="math-inline"`)
	require.Contains(t, htmlOut, `class="math-block"`)
	require.Contains(t, htmlOut, `<math xmlns="http://www.w3.org/1998/Math/MathML">`)
}

func TestExtenderThrowOnError(t *testing.T) {
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			&Extender{Options: RenderOptions{ThrowOnError: true}},
		),
		goldmark.WithRendererOptions(
			html.WithXHTML(),
		),
	)

	src := "bad $`\\invalidcommand`$"
	out := &bytes.Buffer{}
	err := md.Convert([]byte(src), out)
	require.Error(t, err)
}

func TestParserInlineAndBlock(t *testing.T) {
	pc := parser.NewContext()
	inlineParser := &InlineParser{}
	require.Equal(t, []byte{'$'}, inlineParser.Trigger())
	reader := text.NewReader([]byte("$`x+y`$"))
	inline := inlineParser.Parse(nil, reader, pc)
	require.NotNil(t, inline)
	require.IsType(t, &Inline{}, inline)
	require.Equal(t, []byte("x+y"), inline.(*Inline).Equation)

	blockParser := &BlockParser{}
	require.Equal(t, []byte{'$'}, blockParser.Trigger())
	reader = text.NewReader([]byte("$$\nA = B\n$$\n"))
	block, state := blockParser.Open(nil, reader, pc)
	require.NotNil(t, block)
	require.Equal(t, parser.HasChildren, state)
	block = block.(*Block)
	state = blockParser.Continue(block, reader, pc)
	require.Equal(t, parser.Continue|parser.NoChildren, state)
	state = blockParser.Continue(block, reader, pc)
	require.Equal(t, parser.Close, state)
	require.Equal(t, []byte("A = B"), block.(*Block).Equation)
}

func TestASTDumpAndRendererNonEntering(t *testing.T) {
	inline := &Inline{Equation: []byte("abc")}
	inline.Dump([]byte("abc"), 0)
	inline.Inline()
	require.True(t, inline.IsBlank([]byte("abc")))

	block := &Block{Equation: []byte("abc")}
	block.Dump(nil, 0)
	require.True(t, block.IsBlank([]byte("abc")))

	r := &HTMLRenderer{}
	b := &bytes.Buffer{}
	w := bufio.NewWriter(b)
	status, err := r.renderInline(w, []byte("abc"), inline, false)
	require.NoError(t, err)
	require.Equal(t, ast.WalkContinue, status)

	status, err = r.renderBlock(w, nil, block, false)
	require.NoError(t, err)
	require.Equal(t, ast.WalkContinue, status)
	require.NoError(t, w.Flush())
	require.Empty(t, b.String())
}

func TestASTIsBlankFalsePaths(t *testing.T) {
	src := []byte("x")

	inline := &Inline{}
	inline.AppendChild(inline, ast.NewTextSegment(text.NewSegment(0, 1)))
	require.False(t, inline.IsBlank(src))

	block := &Block{}
	block.AppendChild(block, ast.NewTextSegment(text.NewSegment(0, 1)))
	require.False(t, block.IsBlank(src))
}

func TestBlockParserHelpersAndOpenReject(t *testing.T) {
	pc := parser.NewContext()
	p := &BlockParser{}

	reader := text.NewReader([]byte("$$ not-a-block\n"))
	node, state := p.Open(nil, reader, pc)
	require.Nil(t, node)
	require.Equal(t, parser.NoChildren, state)

	require.True(t, p.CanInterruptParagraph())
	require.False(t, p.CanAcceptIndentedLine())
	require.NotPanics(t, func() {
		p.Close(&Block{}, reader, pc)
	})
}

func TestRendererBlockFallbackAndThrowOnError(t *testing.T) {
	invalid := &Block{Equation: []byte(`\frac{`)}

	t.Run("fallback html when throwOnError=false", func(t *testing.T) {
		r := &HTMLRenderer{Options: RenderOptions{ThrowOnError: false, BlockWrapperClass: " math-wrap "}}
		buf := &bytes.Buffer{}
		w := bufio.NewWriter(buf)

		status, err := r.renderBlock(w, nil, invalid, true)
		require.NoError(t, err)
		require.Equal(t, ast.WalkContinue, status)
		require.NoError(t, w.Flush())

		htmlOut := buf.String()
		require.Contains(t, htmlOut, `<div class="math-wrap">`)
		require.Contains(t, htmlOut, `class="katex-error"`)
		require.Contains(t, htmlOut, `\frac{`)
	})

	t.Run("return error when throwOnError=true", func(t *testing.T) {
		r := &HTMLRenderer{Options: RenderOptions{ThrowOnError: true}}
		buf := &bytes.Buffer{}
		w := bufio.NewWriter(buf)

		status, err := r.renderBlock(w, nil, invalid, true)
		require.Error(t, err)
		require.Equal(t, ast.WalkStop, status)
	})
}

func TestTransformerSingleLineAndUnclosed(t *testing.T) {
	t.Run("single-line display block", func(t *testing.T) {
		md := goldmark.New(
			goldmark.WithExtensions(extension.GFM, &Extender{}),
			goldmark.WithRendererOptions(html.WithXHTML()),
		)

		out := &bytes.Buffer{}
		err := md.Convert([]byte("$$x+y$$\n"), out)
		require.NoError(t, err)
		htmlOut := out.String()
		require.Contains(t, htmlOut, `display="block"`)
		require.NotContains(t, htmlOut, "$$x+y$$")
	})

	t.Run("unclosed block remains raw text", func(t *testing.T) {
		md := goldmark.New(
			goldmark.WithExtensions(extension.GFM, &Extender{}),
			goldmark.WithRendererOptions(html.WithXHTML()),
		)

		src := "$$\na+b\n\n## stop\n\n$$\n"
		out := &bytes.Buffer{}
		err := md.Convert([]byte(src), out)
		require.NoError(t, err)

		htmlOut := out.String()
		require.Contains(t, htmlOut, `<p>$$`)
		require.Contains(t, htmlOut, `<h2>stop</h2>`)
	})
}

func TestBlockOptionsApplyToWrapperAndRenderOption(t *testing.T) {
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			&Extender{Options: RenderOptions{BlockWrapperClass: "math-block", Output: "mathml"}},
		),
		goldmark.WithRendererOptions(
			html.WithXHTML(),
		),
	)

	src := "$$ {align=right,width=320px,class=\"math-note\",style=\"margin:4px\",leqno=true,fleqn=true,output=mathml}\n" +
		"x+y\n$$\n"
	out := &bytes.Buffer{}
	err := md.Convert([]byte(src), out)
	require.NoError(t, err)

	htmlOut := out.String()
	require.Contains(t, htmlOut, `class="math-block math-note"`)
	require.Contains(t, htmlOut, `style="width:320px;margin:4px;display:flex;justify-content:flex-end;"`)
	require.Contains(t, htmlOut, `display="block"`)
	require.Contains(t, htmlOut, `annotation encoding="application/x-tex">x+y</annotation>`)
}

func TestBlockOptionThrowOnErrorOverridesGlobal(t *testing.T) {
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			&Extender{Options: RenderOptions{ThrowOnError: true}},
		),
		goldmark.WithRendererOptions(
			html.WithXHTML(),
		),
	)

	src := "$$ {throwOnError=false}\n\\frac{\n$$\n"
	out := &bytes.Buffer{}
	err := md.Convert([]byte(src), out)
	require.NoError(t, err)

	htmlOut := out.String()
	require.Contains(t, htmlOut, `class="katex-error"`)
}

func TestBlockOpeningOptionParseFallbackForEquationLeadingBrace(t *testing.T) {
	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM, &Extender{}),
		goldmark.WithRendererOptions(html.WithXHTML()),
	)

	src := "$$ {x+y} $$\n"
	out := &bytes.Buffer{}
	err := md.Convert([]byte(src), out)
	require.NoError(t, err)

	htmlOut := out.String()
	require.Contains(t, htmlOut, `display="block"`)
	require.Contains(t, htmlOut, `annotation encoding="application/x-tex">{x+y}</annotation>`)
}

func TestParseInlineOptionsAndConverters(t *testing.T) {
	opts, err := parseInlineOptions(`align=left,width=70%,class="math note",leqno=true,fleqn=false,count=3,scale=1.25,arr=[1,"x",true]`)
	require.NoError(t, err)
	require.Equal(t, "left", opts["align"])
	require.Equal(t, "70%", opts["width"])
	require.Equal(t, "math note", opts["class"])
	require.Equal(t, true, opts["leqno"])
	require.Equal(t, false, opts["fleqn"])
	require.EqualValues(t, int64(3), opts["count"])
	require.Equal(t, 1.25, opts["scale"])
	require.Len(t, opts["arr"], 3)

	b, ok := optionBool("true")
	require.True(t, ok)
	require.True(t, b)
	b, ok = optionBool("no")
	require.False(t, ok)
	require.False(t, b)

	s, ok := optionString(int64(42))
	require.True(t, ok)
	require.Equal(t, "42", s)
	_, ok = optionString(struct{}{})
	require.False(t, ok)
}

func TestParseInlineOptionsErrorCases(t *testing.T) {
	_, err := parseInlineOptions(`broken`)
	require.Error(t, err)

	_, err = parseInlineOptions(`k=`)
	require.Error(t, err)

	_, err = parseInlineOptions(`k="unterminated`)
	require.Error(t, err)

	_, err = parseInlineOptions(`arr=[1,2`)
	require.Error(t, err)

	_, err = parseOptionValue("")
	require.Error(t, err)
}

func TestTransformerOptionHelperBranches(t *testing.T) {
	opt, rest := parseBlockOpening([]byte("$$ {align=left,class=\"x\"} x+y $$"))
	require.Equal(t, "left", opt["align"])
	require.Equal(t, []byte("x+y $$"), rest)

	opt, rest = parseBlockOpening([]byte("$$ {x+y} $$"))
	require.Nil(t, opt)
	require.Equal(t, []byte("{x+y} $$"), rest)

	_, _, ok := splitLeadingBraceBlock([]byte(`{"a\"b"`))
	require.False(t, ok)

	body, closed := stripClosingDelimiter([]byte("x+y $$"))
	require.True(t, closed)
	require.Equal(t, []byte("x+y"), body)

	body, closed = stripClosingDelimiter([]byte("x+y"))
	require.False(t, closed)
	require.Equal(t, []byte("x+y"), body)
}
