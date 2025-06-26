package d2ext

import (
	"bytes"
	"context"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"
	"oss.terrastruct.com/d2/d2graph"
	"oss.terrastruct.com/d2/d2layouts/d2dagrelayout"
	"oss.terrastruct.com/d2/d2layouts/d2elklayout"
	"oss.terrastruct.com/d2/d2layouts/d2grid"
	"oss.terrastruct.com/d2/d2lib"
	"oss.terrastruct.com/d2/d2renderers/d2svg"
	"oss.terrastruct.com/d2/d2themes/d2themescatalog"
	d2log "oss.terrastruct.com/d2/lib/log"
	"oss.terrastruct.com/d2/lib/textmeasure"
)

var (
	defaultLayout  = d2dagrelayout.DefaultLayout
	defaultThemeID = d2themescatalog.CoolClassics.ID
)

type HTMLRenderer struct {
}

func (r *HTMLRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(KindBlock, r.Render)
}

func (r *HTMLRenderer) Render(w util.BufWriter, src []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n := node.(*Block)
	if !entering {
		w.WriteString("</div>")
		return ast.WalkContinue, nil
	}
	w.WriteString(`<div class="d2">`)

	b := bytes.Buffer{}
	lines := n.Lines()
	for i := 0; i < lines.Len(); i++ {
		line := lines.At(i)
		b.Write(line.Value(src))
	}

	if b.Len() == 0 {
		return ast.WalkContinue, nil
	}

	ruler, err := textmeasure.NewRuler()
	if err != nil {
		return ast.WalkStop, err
	}

	layoutResolver := func(engine string) (d2graph.LayoutGraph, error) {
		switch engine {
		case "elk":
			return d2elklayout.DefaultLayout, nil
		case "dagre":
			return d2dagrelayout.DefaultLayout, nil
		case "grid":
			return d2grid.Layout, nil
		default:
			return d2dagrelayout.DefaultLayout, nil
		}
	}

	compileOpts := &d2lib.CompileOptions{
		LayoutResolver: layoutResolver,
		Ruler:          ruler,
	}

	renderOpts := &d2svg.RenderOpts{}

	ctx := d2log.WithDefault(context.Background())

	diagram, _, err := d2lib.Compile(ctx, b.String(), compileOpts, renderOpts)
	if err != nil {
		_, err = w.Write(b.Bytes())
		return ast.WalkContinue, err
	}

	if renderOpts.Scale == nil {
		renderOpts.Scale = Pointer(1.0)
	}

	// renderOpts.ThemeID = go2.Pointer(d2themescatalog.NeutralDefault.ID)
	// renderOpts.ThemeID = go2.Pointer(d2themescatalog.DarkMauve.ID)

	out, err := d2svg.Render(diagram, renderOpts)
	if err != nil {
		_, err = w.Write(b.Bytes())
		return ast.WalkContinue, err
	}

	_, err = w.Write(out)
	return ast.WalkContinue, err
}

func Pointer[T any](v T) *T {
	return &v
}
