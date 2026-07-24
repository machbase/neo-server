package d2ext

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"sync"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"
	"oss.terrastruct.com/d2/d2graph"
	"oss.terrastruct.com/d2/d2layouts/d2dagrelayout"
	"oss.terrastruct.com/d2/d2layouts/d2elklayout"
	"oss.terrastruct.com/d2/d2lib"
	"oss.terrastruct.com/d2/d2plugin"
	"oss.terrastruct.com/d2/d2renderers/d2svg"
	"oss.terrastruct.com/d2/d2themes/d2themescatalog"
	d2log "oss.terrastruct.com/d2/lib/log"
	"oss.terrastruct.com/d2/lib/textmeasure"
)

var (
	d2PluginListOnce sync.Once
	d2PluginList     []d2plugin.Plugin
	d2PluginListErr  error
)

var supportedElkAlgorithms = map[string]struct{}{
	"layered": {},
	"mrtree":  {},
	//	"radial":      {},
	//	"rectpacking": {},
	//	"spore":       {},
	//	"box":    {},
	//	"fixed":  {},
	"random": {},
}

func ptr[T any](v T) *T {
	return &v
}

type HTMLRenderer struct {
	Layout          d2graph.LayoutGraph
	ThemeID         *int64
	Sketch          bool
	OptionApplierFn FenceOptionApplier
}

type FenceOptionApplier func(opts map[string]any, compileOpts *d2lib.CompileOptions, renderOpts *d2svg.RenderOpts) error

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

	compileOpts := &d2lib.CompileOptions{
		Ruler:          ruler,
		LayoutResolver: r.defaultLayoutResolver,
	}

	renderOpts := &d2svg.RenderOpts{
		Pad:    ptr(int64(d2svg.DEFAULT_PADDING)),
		Sketch: &r.Sketch,
	}

	if r.ThemeID != nil {
		renderOpts.ThemeID = ptr(int64(*r.ThemeID))
	} else {
		renderOpts.ThemeID = ptr(int64(d2themescatalog.CoolClassics.ID))
	}

	if len(n.Options) > 0 {
		applier := r.OptionApplierFn
		if applier == nil {
			applier = applyDefaultFenceOptionsWithCompile
		}
		if err := applier(n.Options, compileOpts, renderOpts); err != nil {
			_, _ = w.Write(b.Bytes())
			return ast.WalkContinue, err
		}
	}

	ctx := d2log.WithDefault(context.Background())

	diagram, _, err := d2lib.Compile(ctx, b.String(), compileOpts, renderOpts)
	if err != nil {
		_, err = w.Write(b.Bytes())
		return ast.WalkContinue, err
	}

	if renderOpts.Scale == nil {
		renderOpts.Scale = Pointer(1.0)
	}

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

func applyDefaultFenceOptions(opts map[string]any, _ *d2lib.CompileOptions, renderOpts *d2svg.RenderOpts) error {
	return applyDefaultFenceOptionsWithCompile(opts, nil, renderOpts)
}

func applyDefaultFenceOptionsWithCompile(opts map[string]any, compileOpts *d2lib.CompileOptions, renderOpts *d2svg.RenderOpts) error {
	layoutName := ""
	if v, ok := opts["layout"]; ok {
		layoutValue, ok := optionString(v)
		if !ok {
			return fmt.Errorf("layout must be a string")
		}
		layoutName = strings.ToLower(layoutValue)
		if layoutName == "" {
			return fmt.Errorf("layout must not be empty")
		}
		if compileOpts != nil {
			compileOpts.Layout = Pointer(layoutName)
		}
	}

	if v, ok := opts["sketch"]; ok {
		if b, parsed := optionBool(v); parsed {
			renderOpts.Sketch = Pointer(b)
		}
	}

	if v, ok := opts["theme"]; ok {
		themeName, ok := optionString(v)
		if !ok {
			return fmt.Errorf("theme must be a string")
		}
		themeID, found := resolveThemeIDByName(themeName)
		if !found {
			return fmt.Errorf("unknown d2 theme name: %q", themeName)
		}
		renderOpts.ThemeID = Pointer(themeID)
	}

	if v, ok := opts["themeID"]; ok {
		if themeID, parsed := optionInt64(v); parsed {
			renderOpts.ThemeID = Pointer(themeID)
		}
	}

	if v, ok := opts["pad"]; ok {
		if pad, parsed := optionInt64(v); parsed {
			renderOpts.Pad = Pointer(pad)
		}
	}

	if v, ok := opts["scale"]; ok {
		if scale, parsed := optionFloat64(v); parsed {
			renderOpts.Scale = Pointer(scale)
		}
	}

	if compileOpts != nil {
		requestedLayout := "dagre"
		if compileOpts.Layout != nil && strings.TrimSpace(*compileOpts.Layout) != "" {
			requestedLayout = strings.ToLower(strings.TrimSpace(*compileOpts.Layout))
		}

		if v, ok := opts["algorithm"]; ok {
			if requestedLayout != "elk" {
				return fmt.Errorf("algorithm is only supported when layout is elk")
			}
			algorithm, ok := optionString(v)
			if !ok {
				return fmt.Errorf("algorithm must be a string")
			}
			algorithm = strings.ToLower(algorithm)
			if algorithm == "" {
				return fmt.Errorf("algorithm must not be empty")
			}
			if _, supported := supportedElkAlgorithms[algorithm]; !supported {
				return fmt.Errorf("unsupported elk algorithm: %s (supported: %s)", algorithm, strings.Join(supportedElkAlgorithmNames(), ", "))
			}

			baseResolver := compileOpts.LayoutResolver
			compileOpts.LayoutResolver = func(engine string) (d2graph.LayoutGraph, error) {
				normalized := strings.ToLower(strings.TrimSpace(engine))
				if normalized == "" || normalized == "elk" {
					return func(ctx context.Context, g *d2graph.Graph) error {
						return d2elklayout.Layout(ctx, g, &d2elklayout.ConfigurableOpts{
							Algorithm:       algorithm,
							NodeSpacing:     d2elklayout.DefaultOpts.NodeSpacing,
							Padding:         d2elklayout.DefaultOpts.Padding,
							EdgeNodeSpacing: d2elklayout.DefaultOpts.EdgeNodeSpacing,
							SelfLoopSpacing: d2elklayout.DefaultOpts.SelfLoopSpacing,
						})
					}, nil
				}
				if baseResolver != nil {
					return baseResolver(engine)
				}
				return nil, fmt.Errorf("unsupported layout engine: %s", engine)
			}
		}

		nodeSep, hasNodeSep := parseIntOption(opts, "nodesep")
		edgeSep, hasEdgeSep := parseIntOption(opts, "edgesep")
		if hasNodeSep || hasEdgeSep {
			if requestedLayout != "dagre" {
				return fmt.Errorf("nodesep/edgesep are only supported when layout is dagre")
			}

			if !hasNodeSep {
				nodeSep = d2dagrelayout.DefaultOpts.NodeSep
			}
			if !hasEdgeSep {
				edgeSep = d2dagrelayout.DefaultOpts.EdgeSep
			}

			baseResolver := compileOpts.LayoutResolver
			compileOpts.LayoutResolver = func(engine string) (d2graph.LayoutGraph, error) {
				normalized := strings.ToLower(strings.TrimSpace(engine))
				if normalized == "" || normalized == "dagre" {
					return func(ctx context.Context, g *d2graph.Graph) error {
						return d2dagrelayout.Layout(ctx, g, &d2dagrelayout.ConfigurableOpts{
							NodeSep: nodeSep,
							EdgeSep: edgeSep,
						})
					}, nil
				}
				if baseResolver != nil {
					return baseResolver(engine)
				}
				return nil, fmt.Errorf("unsupported layout engine: %s", engine)
			}
		}
	}

	return nil
}

func (r *HTMLRenderer) defaultLayoutResolver(engine string) (d2graph.LayoutGraph, error) {
	normalized := strings.ToLower(strings.TrimSpace(engine))
	if normalized == "" || normalized == "dagre" {
		if r.Layout != nil {
			return r.Layout, nil
		}
		return d2dagrelayout.DefaultLayout, nil
	}

	plugins, err := listD2Plugins()
	if err != nil {
		return nil, err
	}

	plugin, err := d2plugin.FindPlugin(context.Background(), plugins, normalized)
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return nil, fmt.Errorf("unknown layout engine: %s (available: %s)", normalized, strings.Join(availableLayoutNames(plugins), ", "))
		}
		return nil, err
	}

	return plugin.Layout, nil
}

func listD2Plugins() ([]d2plugin.Plugin, error) {
	d2PluginListOnce.Do(func() {
		d2PluginList, d2PluginListErr = d2plugin.ListPlugins(context.Background())
	})
	if d2PluginListErr != nil {
		return nil, d2PluginListErr
	}
	return d2PluginList, nil
}

func availableLayoutNames(plugins []d2plugin.Plugin) []string {
	names := make([]string, 0, len(plugins))
	for _, p := range plugins {
		info, err := p.Info(context.Background())
		if err != nil {
			continue
		}
		if info == nil || strings.TrimSpace(info.Name) == "" {
			continue
		}
		names = append(names, strings.ToLower(strings.TrimSpace(info.Name)))
	}

	if len(names) == 0 {
		return []string{"dagre"}
	}

	sort.Strings(names)
	unique := names[:0]
	for i, name := range names {
		if i == 0 || name != names[i-1] {
			unique = append(unique, name)
		}
	}
	return unique
}

func parseIntOption(opts map[string]any, key string) (int, bool) {
	v, ok := opts[key]
	if !ok {
		return 0, false
	}
	i, parsed := optionInt64(v)
	if !parsed {
		return 0, false
	}
	return int(i), true
}

func supportedElkAlgorithmNames() []string {
	names := make([]string, 0, len(supportedElkAlgorithms))
	for name := range supportedElkAlgorithms {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func resolveThemeIDByName(name string) (int64, bool) {
	query := strings.TrimSpace(name)
	if query == "" {
		return 0, false
	}

	for _, t := range d2themescatalog.LightCatalog {
		if strings.EqualFold(t.Name, query) {
			return t.ID, true
		}
	}
	for _, t := range d2themescatalog.DarkCatalog {
		if strings.EqualFold(t.Name, query) {
			return t.ID, true
		}
	}

	return 0, false
}
