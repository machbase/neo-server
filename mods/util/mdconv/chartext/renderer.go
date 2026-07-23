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
	Width      string
	Height     string
	Theme      string
	Renderer   string
	Loader     string
	EchartsSrc string
	CdnSrc     string
	ThemeSrcs  []string
	PluginSrcs []string
}

var builtInPluginPaths = map[string]string{
	"liquidfill": "/web/echarts/echarts-liquidfill.min.js",
	"wordcloud":  "/web/echarts/echarts-wordcloud.min.js",
	"gl":         "/web/echarts/echarts-gl.min.js",
}

var builtInThemeNames = map[string]bool{
	"white":          true,
	"dark":           true,
	"essos":          true,
	"chalk":          true,
	"purple-passion": true,
	"romantic":       true,
	"walden":         true,
	"westeros":       true,
	"wonderland":     true,
	"vintage":        true,
	"macarons":       true,
	"infographic":    true,
	"shine":          true,
	"roma":           true,
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
	theme := "white"
	if darkMode {
		theme = "dark"
	}
	themeSrcs := []string{}
	if theme == "dark" {
		themeSrcs = append(themeSrcs, "/web/echarts/themes/dark.js")
	}
	return renderConfig{
		Width:      "100%",
		Height:     "400px",
		Theme:      theme,
		Renderer:   "canvas",
		Loader:     "auto",
		EchartsSrc: "/web/echarts/echarts.min.js",
		CdnSrc:     "https://cdn.jsdelivr.net/npm/echarts@5/dist/echarts.min.js",
		ThemeSrcs:  themeSrcs,
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
		if s == "light" {
			s = "white"
		}
		themeSrcs := []string{}
		if builtInThemeNames[s] {
			if s != "white" {
				themeSrcs = append(themeSrcs, "/web/echarts/themes/"+s+".js")
			}
		} else if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") {
			themeSrcs = append(themeSrcs, s)
		} else {
			return fmt.Errorf("chart theme must be one of white,dark,essos,chalk,purple-passion,romantic,walden,westeros,wonderland,vintage,macarons,infographic,shine,roma,light or an http(s) URL")
		}
		cfg.Theme = s
		cfg.ThemeSrcs = themeSrcs
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
	if mode, ok := opts["loader"]; ok {
		s, ok := optionString(mode)
		if !ok {
			return fmt.Errorf("chart loader must be a string")
		}
		s = strings.ToLower(s)
		if s != "none" && s != "local" && s != "auto" {
			return fmt.Errorf("chart loader must be none, local or auto")
		}
		cfg.Loader = s
	}
	if src, ok := opts["echartsSrc"]; ok {
		s, ok := optionString(src)
		if !ok {
			return fmt.Errorf("chart echartsSrc must be a string")
		}
		cfg.EchartsSrc = s
	}
	if src, ok := opts["cdnSrc"]; ok {
		s, ok := optionString(src)
		if !ok {
			return fmt.Errorf("chart cdnSrc must be a string")
		}
		cfg.CdnSrc = s
	}
	if plugins, ok := opts["plugins"]; ok {
		items, ok := optionStringList(plugins)
		if !ok {
			return fmt.Errorf("chart plugins must be a string or string array")
		}
		pluginSet := map[string]struct{}{}
		resolved := make([]string, 0, len(items))
		for _, item := range items {
			key := strings.ToLower(strings.TrimSpace(item))
			if key == "" {
				continue
			}
			src := key
			if path, found := builtInPluginPaths[key]; found {
				src = path
			}
			if _, exists := pluginSet[src]; exists {
				continue
			}
			pluginSet[src] = struct{}{}
			resolved = append(resolved, src)
		}
		cfg.PluginSrcs = resolved
	}
	return nil
}

func jsArrayLiteral(values []string) string {
	quoted := make([]string, 0, len(values))
	for _, v := range values {
		quoted = append(quoted, strconv.Quote(v))
	}
	return "[" + strings.Join(quoted, ",") + "]"
}

func buildScript(id string, code string, cfg renderConfig) (string, error) {
	safeCode := scriptClosePattern.ReplaceAllString(code, `<\\/script>`)
	quotedCode := strconv.Quote(safeCode)
	quotedID := strconv.Quote(id)
	quotedTheme := strconv.Quote(cfg.Theme)
	quotedRenderer := strconv.Quote(cfg.Renderer)
	quotedLoader := strconv.Quote(cfg.Loader)
	quotedEchartsSrc := strconv.Quote(cfg.EchartsSrc)
	quotedCdnSrc := strconv.Quote(cfg.CdnSrc)
	quotedThemeSrcs := jsArrayLiteral(cfg.ThemeSrcs)
	quotedPluginSrcs := jsArrayLiteral(cfg.PluginSrcs)

	var b strings.Builder
	b.WriteString("(function(){")
	b.WriteString("var __script=document.currentScript||window.__chartextCurrentScript||null;")
	b.WriteString("var __dom=null;")
	b.WriteString("if(__script&&__script.previousElementSibling&&__script.previousElementSibling.classList&&__script.previousElementSibling.classList.contains('chartext-echarts')){__dom=__script.previousElementSibling;}")
	b.WriteString("if(!__dom){__dom=document.getElementById(")
	b.WriteString(quotedID)
	b.WriteString(");}")
	b.WriteString("if(!__dom){return;}")
	b.WriteString("var __loaderMode=")
	b.WriteString(quotedLoader)
	b.WriteString(";")
	b.WriteString("var __echartsSrc=")
	b.WriteString(quotedEchartsSrc)
	b.WriteString(";")
	b.WriteString("var __cdnSrc=")
	b.WriteString(quotedCdnSrc)
	b.WriteString(";")
	b.WriteString("var __themeSrcs=")
	b.WriteString(quotedThemeSrcs)
	b.WriteString(";")
	b.WriteString("var __pluginSrcs=")
	b.WriteString(quotedPluginSrcs)
	b.WriteString(";")
	b.WriteString("if(!window.__chartextScriptPromises){window.__chartextScriptPromises={};}")
	b.WriteString("if(!window.__chartextEchartsLoaderPromises){window.__chartextEchartsLoaderPromises={};}")
	b.WriteString("function __loadScriptOnce(src){")
	b.WriteString("if(!src){return Promise.resolve();}")
	b.WriteString("var p=window.__chartextScriptPromises[src];")
	b.WriteString("if(p){return p;}")
	b.WriteString("p=new Promise(function(resolve,reject){")
	b.WriteString("var s=document.createElement('script');")
	b.WriteString("s.src=src;")
	b.WriteString("s.async=true;")
	b.WriteString("s.onload=function(){resolve();};")
	b.WriteString("s.onerror=function(){reject(new Error('failed to load script: '+src));};")
	b.WriteString("document.head.appendChild(s);")
	b.WriteString("});")
	b.WriteString("window.__chartextScriptPromises[src]=p;")
	b.WriteString("return p;")
	b.WriteString("}")
	b.WriteString("function __ensureEcharts(){")
	b.WriteString("if(window.echarts){return Promise.resolve(window.echarts);}")
	b.WriteString("if(__loaderMode==='none'){return Promise.reject(new Error('ECharts library is not loaded.'));}")
	b.WriteString("var key=__loaderMode+'|'+(__echartsSrc||'')+'|'+(__cdnSrc||'');")
	b.WriteString("if(window.__chartextEchartsLoaderPromises[key]){return window.__chartextEchartsLoaderPromises[key];}")
	b.WriteString("function __loadLocal(){")
	b.WriteString("return __loadScriptOnce(__echartsSrc).then(function(){")
	b.WriteString("if(!window.echarts){throw new Error('ECharts loaded but window.echarts is undefined.');}")
	b.WriteString("return window.echarts;")
	b.WriteString("});")
	b.WriteString("}")
	b.WriteString("var loader; if(__loaderMode==='local'){loader=__loadLocal();}else{loader=__loadLocal().catch(function(localErr){")
	b.WriteString("if(!__cdnSrc){throw localErr;}")
	b.WriteString("return __loadScriptOnce(__cdnSrc).then(function(){")
	b.WriteString("if(!window.echarts){throw new Error('CDN ECharts loaded but window.echarts is undefined.');}")
	b.WriteString("return window.echarts;")
	b.WriteString("});")
	b.WriteString("});}")
	b.WriteString("window.__chartextEchartsLoaderPromises[key]=loader;")
	b.WriteString("return loader;")
	b.WriteString("}")
	b.WriteString("function __ensurePlugins(){")
	b.WriteString("if(!__pluginSrcs||!__pluginSrcs.length){return Promise.resolve();}")
	b.WriteString("var tasks=[];")
	b.WriteString("for(var i=0;i<__pluginSrcs.length;i++){tasks.push(__loadScriptOnce(__pluginSrcs[i]));}")
	b.WriteString("return Promise.all(tasks).then(function(){return;});")
	b.WriteString("}")
	b.WriteString("function __ensureThemeAssets(){")
	b.WriteString("if(!__themeSrcs||!__themeSrcs.length){return Promise.resolve();}")
	b.WriteString("var tasks=[];")
	b.WriteString("for(var i=0;i<__themeSrcs.length;i++){tasks.push(__loadScriptOnce(__themeSrcs[i]));}")
	b.WriteString("return Promise.all(tasks).then(function(){return;});")
	b.WriteString("}")
	b.WriteString("__ensureEcharts().then(function(){return __ensureThemeAssets();}).then(function(){return __ensurePlugins();}).then(function(){")
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
	b.WriteString("}).catch(function(e){__dom.innerText='ECharts load error: '+(e&&e.message?e.message:String(e));});")
	b.WriteString("})();")
	return b.String(), nil
}

func nextChartID() string {
	id := chartIDSeq.Add(1)
	return fmt.Sprintf("chart-%d", id)
}
