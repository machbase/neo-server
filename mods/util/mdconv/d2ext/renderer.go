package d2ext

import (
	"bytes"
	"context"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"
	"oss.terrastruct.com/d2/d2graph"
	"oss.terrastruct.com/d2/d2layouts/d2dagrelayout"
	"oss.terrastruct.com/d2/d2lib"
	"oss.terrastruct.com/d2/d2renderers/d2svg"
	"oss.terrastruct.com/d2/d2themes/d2themescatalog"
	"oss.terrastruct.com/d2/lib/textmeasure"
)

var (
	defaultLayout  = d2dagrelayout.DefaultLayout
	defaultThemeID = d2themescatalog.CoolClassics.ID
)

type HTMLRenderer struct {
	Layout  func(context.Context, *d2graph.Graph) error
	ThemeID int64
	Sketch  bool
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
	opts := &d2lib.CompileOptions{
		Layout:  defaultLayout,
		Ruler:   ruler,
		ThemeID: defaultThemeID,
	}
	if r.Layout != nil {
		opts.Layout = r.Layout
	}
	if r.ThemeID != 0 {
		opts.ThemeID = r.ThemeID
	}
	diagram, _, err := d2lib.Compile(context.Background(), b.String(), opts)
	if err != nil {
		_, err = w.Write(b.Bytes())
		return ast.WalkContinue, err
	}
	out, err := d2svg.Render(diagram, &d2svg.RenderOpts{
		Pad:           10, // d2svg.DEFAULT_PADDING,
		Sketch:        r.Sketch,
		ThemeID:       r.ThemeID,
		SetDimensions: true,
	})
	if err != nil {
		_, err = w.Write(b.Bytes())
		return ast.WalkContinue, err
	}

	_, err = w.Write(out)
	return ast.WalkContinue, err
}
