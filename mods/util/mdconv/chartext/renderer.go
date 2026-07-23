package chartext

import (
	"bytes"
	"fmt"
	"html"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"
)

var chartIDSeq atomic.Uint64

var sizePattern = regexp.MustCompile(`^[0-9]+(?:\.[0-9]+)?(?:px|%|vh|vw|rem|em)?$`)
var scriptClosePattern = regexp.MustCompile(`(?i)</script>`)

type HTMLRenderer struct {
	DarkMode bool
}

type renderConfig struct {
	Width    string
	Height   string
	Theme    string
	Renderer string
}

func (r *HTMLRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(KindBlock, r.Render)
}

func (r *HTMLRenderer) Render(w util.BufWriter, src []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		_, _ = w.WriteString("</div>")
		return ast.WalkContinue, nil
	}

	n := node.(*Block)
	_, _ = w.WriteString(`<div class="chartext">`)

	b := bytes.Buffer{}
	lines := n.Lines()
	for i := 0; i < lines.Len(); i++ {
		line := lines.At(i)
		b.Write(line.Value(src))
	}

	if b.Len() == 0 {
		_, _ = w.WriteString(`<div class="chartext-error">Chart code is empty.</div>`)
		return ast.WalkContinue, nil
	}

	cfg := defaultRenderConfig(r.DarkMode)
	if err := applyFenceOptions(&cfg, n.Options); err != nil {
		_, _ = w.WriteString(`<div class="chartext-error">` + html.EscapeString(err.Error()) + `</div>`)
		return ast.WalkContinue, err
	}

	id := nextChartID()
	style := `width:` + cfg.Width + `;height:` + cfg.Height
	_, _ = w.WriteString(`<div class="chartext-echarts" id="` + html.EscapeString(id) + `" style="` + html.EscapeString(style) + `"></div>`)

	script, err := buildScript(id, b.String(), cfg)
	if err != nil {
		_, _ = w.WriteString(`<div class="chartext-error">` + html.EscapeString(err.Error()) + `</div>`)
		return ast.WalkContinue, err
	}
	_, _ = w.WriteString(`<script type="text/javascript">`)
	_, _ = w.WriteString(script)
	_, _ = w.WriteString(`</script>`)
	return ast.WalkContinue, nil
}

func defaultRenderConfig(darkMode bool) renderConfig {
	theme := "light"
	if darkMode {
		theme = "dark"
	}
	return renderConfig{
		Width:    "100%",
		Height:   "400px",
		Theme:    theme,
		Renderer: "canvas",
	}
}

func applyFenceOptions(cfg *renderConfig, opts map[string]any) error {
	if len(opts) == 0 {
		return nil
	}
	if width, ok := opts["width"]; ok {
		s, ok := optionString(width)
		if !ok {
			return fmt.Errorf("chart width must be a string")
		}
		if !sizePattern.MatchString(s) {
			return fmt.Errorf("invalid chart width value: %q", s)
		}
		cfg.Width = s
	}
	if height, ok := opts["height"]; ok {
		s, ok := optionString(height)
		if !ok {
			return fmt.Errorf("chart height must be a string")
		}
		if !sizePattern.MatchString(s) {
			return fmt.Errorf("invalid chart height value: %q", s)
		}
		cfg.Height = s
	}
	if theme, ok := opts["theme"]; ok {
		s, ok := optionString(theme)
		if !ok {
			return fmt.Errorf("chart theme must be a string")
		}
		s = strings.ToLower(s)
		if s != "light" && s != "dark" {
			return fmt.Errorf("chart theme must be light or dark")
		}
		cfg.Theme = s
	}
	if mode, ok := opts["renderer"]; ok {
		s, ok := optionString(mode)
		if !ok {
			return fmt.Errorf("chart renderer must be a string")
		}
		s = strings.ToLower(s)
		if s != "canvas" && s != "svg" {
			return fmt.Errorf("chart renderer must be canvas or svg")
		}
		cfg.Renderer = s
	}
	return nil
}

func buildScript(id string, code string, cfg renderConfig) (string, error) {
	safeCode := scriptClosePattern.ReplaceAllString(code, `<\\/script>`)
	quotedCode := strconv.Quote(safeCode)
	quotedID := strconv.Quote(id)
	quotedTheme := strconv.Quote(cfg.Theme)
	quotedRenderer := strconv.Quote(cfg.Renderer)

	var b strings.Builder
	b.WriteString("(function(){")
	b.WriteString("var __script=document.currentScript||window.__chartextCurrentScript||null;")
	b.WriteString("var __dom=null;")
	b.WriteString("if(__script&&__script.previousElementSibling&&__script.previousElementSibling.classList&&__script.previousElementSibling.classList.contains('chartext-echarts')){__dom=__script.previousElementSibling;}")
	b.WriteString("if(!__dom){__dom=document.getElementById(")
	b.WriteString(quotedID)
	b.WriteString(");}")
	b.WriteString("if(!__dom){return;}")
	b.WriteString("if(!window.echarts){__dom.innerText='ECharts library is not loaded.';return;}")
	b.WriteString("if(__dom.__chartextResizeHandler){window.removeEventListener('resize',__dom.__chartextResizeHandler);__dom.__chartextResizeHandler=null;}")
	b.WriteString("var __prevChart=echarts.getInstanceByDom(__dom);")
	b.WriteString("if(__prevChart){__prevChart.dispose();}")
	b.WriteString("var __option;")
	b.WriteString("try{")
	b.WriteString("var __chartCode=")
	b.WriteString(quotedCode)
	b.WriteString(";")
	b.WriteString("var __factory=new Function('__ctx',\"var option;\\n(function(){\\n\"+__chartCode+\"\\n}).call(__ctx);\\nif(typeof option!==\\\"undefined\\\"){return option;}\\nif(__ctx&&typeof __ctx.option!==\\\"undefined\\\"){return __ctx.option;}\\nreturn null;\");")
	b.WriteString("__option=__factory({});")
	b.WriteString("}catch(e){__dom.innerText='Chart code error: '+(e&&e.message?e.message:String(e));return;}")
	b.WriteString("if(!__option||typeof __option!=='object'){__dom.innerText='Chart option is not defined.';return;}")
	b.WriteString("try{")
	b.WriteString("var __chart=echarts.init(__dom,")
	b.WriteString(quotedTheme)
	b.WriteString(",{renderer:")
	b.WriteString(quotedRenderer)
	b.WriteString("});")
	b.WriteString("__chart.setOption(__option);")
	b.WriteString("var __resizeHandler=function(){__chart.resize();};")
	b.WriteString("__dom.__chartextResizeHandler=__resizeHandler;")
	b.WriteString("window.addEventListener('resize',__resizeHandler);")
	b.WriteString("}catch(e){__dom.innerText='Chart render error: '+(e&&e.message?e.message:String(e));}")
	b.WriteString("})();")
	return b.String(), nil
}

func nextChartID() string {
	id := chartIDSeq.Add(1)
	return fmt.Sprintf("chart-%d", id)
}
