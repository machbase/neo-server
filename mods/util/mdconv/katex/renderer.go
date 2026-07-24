package katex

import (
	"bytes"
	"html"
	"strings"

	"github.com/machbase/neo-server/v8/mods/util/katexdsl"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"
)

type HTMLRenderer struct {
	Options RenderOptions
}

func (r *HTMLRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(KindInline, r.renderInline)
	reg.Register(KindBlock, r.renderBlock)
}

func (r *HTMLRenderer) renderInline(w util.BufWriter, src []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}

	n := node.(*Inline)
	buf := bytes.Buffer{}
	err := katexdsl.RenderWithOption(&buf, n.Equation, katexdsl.Option{
		DisplayMode:  false,
		Output:       normalizeOutput(r.Options.Output),
		ThrowOnError: r.Options.ThrowOnError,
		Leqno:        r.Options.Leqno,
		Fleqn:        r.Options.Fleqn,
	})
	if err != nil {
		if r.Options.ThrowOnError {
			return ast.WalkStop, err
		}
		_, _ = w.WriteString(html.EscapeString("$" + string(n.Equation) + "$"))
		return ast.WalkContinue, nil
	}
	if className := strings.TrimSpace(r.Options.InlineWrapperClass); className != "" {
		_, _ = w.WriteString(`<span class="` + html.EscapeString(className) + `">`)
	}
	_, _ = w.Write(buf.Bytes())
	if className := strings.TrimSpace(r.Options.InlineWrapperClass); className != "" {
		_, _ = w.WriteString(`</span>`)
	}
	return ast.WalkContinue, nil
}

func (r *HTMLRenderer) renderBlock(w util.BufWriter, src []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}

	n := node.(*Block)
	renderOpt := katexdsl.Option{
		DisplayMode:  true,
		Output:       normalizeOutput(r.Options.Output),
		ThrowOnError: r.Options.ThrowOnError,
		Leqno:        r.Options.Leqno,
		Fleqn:        r.Options.Fleqn,
	}

	classes := mergeClasses(r.Options.BlockWrapperClass)
	styles := make([]string, 0, 3)
	align := ""
	if n.Options != nil {
		if v, ok := optionString(n.Options["class"]); ok {
			classes = mergeClasses(classes, v)
		}
		if v, ok := optionString(n.Options["align"]); ok {
			switch strings.ToLower(v) {
			case "left":
				align = "left"
			case "center":
				align = "center"
			case "right":
				align = "right"
			}
		}
		if v, ok := optionString(n.Options["width"]); ok {
			styles = append(styles, "width:"+v)
		}
		if v, ok := optionString(n.Options["style"]); ok {
			styles = append(styles, v)
		}
		if v, ok := optionBool(n.Options["throwOnError"]); ok {
			renderOpt.ThrowOnError = v
		}
		if v, ok := optionString(n.Options["output"]); ok {
			renderOpt.Output = normalizeOutput(v)
		}
		if v, ok := optionBool(n.Options["leqno"]); ok {
			renderOpt.Leqno = v
		}
		if v, ok := optionBool(n.Options["fleqn"]); ok {
			renderOpt.Fleqn = v
		}
	}
	styles = applyBlockAlign(styles, align)

	buf := bytes.Buffer{}
	err := katexdsl.RenderWithOption(&buf, n.Equation, renderOpt)
	if err != nil {
		if renderOpt.ThrowOnError {
			return ast.WalkStop, err
		}
		writeOpenDiv(w, classes, styles)
		_, _ = w.WriteString("<p>" + html.EscapeString("$$") + "</p>")
		_, _ = w.WriteString("<pre><code>" + html.EscapeString(string(n.Equation)) + "</code></pre>")
		_, _ = w.WriteString("<p>" + html.EscapeString("$$") + "</p>")
		_, _ = w.WriteString(`</div>`)
		return ast.WalkContinue, nil
	}

	writeOpenDiv(w, classes, styles)
	_, _ = w.Write(buf.Bytes())
	_, _ = w.WriteString(`</div>`)
	return ast.WalkContinue, nil
}

func applyBlockAlign(styles []string, align string) []string {
	switch align {
	case "left":
		return append(styles, "display:flex", "justify-content:flex-start")
	case "center":
		return append(styles, "display:flex", "justify-content:center")
	case "right":
		return append(styles, "display:flex", "justify-content:flex-end")
	default:
		return styles
	}
}

func normalizeOutput(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return "mathml"
	}
	return v
}

func mergeClasses(classes ...string) string {
	parts := make([]string, 0, len(classes))
	for _, c := range classes {
		trimmed := strings.TrimSpace(c)
		if trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	return strings.Join(parts, " ")
}

func writeOpenDiv(w util.BufWriter, className string, styles []string) {
	className = strings.TrimSpace(className)
	styleText := strings.TrimSpace(strings.Join(styles, ";"))
	if styleText != "" && !strings.HasSuffix(styleText, ";") {
		styleText += ";"
	}

	_, _ = w.WriteString("<div")
	if className != "" {
		_, _ = w.WriteString(` class="` + html.EscapeString(className) + `"`)
	}
	if styleText != "" {
		_, _ = w.WriteString(` style="` + html.EscapeString(styleText) + `"`)
	}
	_, _ = w.WriteString(`>`)
}
