package jshext

import (
	"html"
	"strings"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"
)

type HTMLRenderer struct{}

func (r *HTMLRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(KindBlock, r.Render)
}

func (r *HTMLRenderer) Render(w util.BufWriter, _ []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	bn := node.(*Block)
	var b strings.Builder
	b.WriteString(`<div class="jshext">`)
	if bn.Source != "" {
		b.WriteString(`<pre class="chroma"><code class="language-javascript">`)
		b.WriteString(html.EscapeString(bn.Source))
		b.WriteString(`</code></pre>`)
	}
	if bn.Options.Execute {
		if bn.Options.Result != "none" {
			b.WriteString(`<div class="jshext-result">`)
			b.WriteString(`<pre>`)
			b.WriteString(html.EscapeString(bn.Output))
			b.WriteString(`</pre>`)
			b.WriteString(`</div>`)
		}
	}
	b.WriteString(`</div>`)
	_, _ = w.WriteString(b.String())
	return ast.WalkContinue, nil
}
