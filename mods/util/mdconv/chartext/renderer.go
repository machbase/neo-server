package chartext

import (
	"bytes"
	_ "embed"
	"fmt"
	"html"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"text/template"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"
)

var chartIDSeq atomic.Uint64

var sizePattern = regexp.MustCompile(`^[0-9]+(?:\.[0-9]+)?(?:px|%|vh|vw|rem|em)?$`)
var scriptClosePattern = regexp.MustCompile(`(?i)</script>`)

//go:embed renderer_js.tmpl
var scriptTemplateText string

var scriptTemplate = template.Must(template.New("chartext-script").Option("missingkey=error").Parse(scriptTemplateText))

type scriptTemplateData struct {
	QuotedID         string
	QuotedLoader     string
	QuotedEchartsSrc string
	QuotedCdnSrc     string
	QuotedThemeSrcs  string
	QuotedPluginSrcs string
	QuotedCode       string
	QuotedTheme      string
	QuotedRenderer   string
}

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
	data := scriptTemplateData{
		QuotedID:         strconv.Quote(id),
		QuotedLoader:     strconv.Quote(cfg.Loader),
		QuotedEchartsSrc: strconv.Quote(cfg.EchartsSrc),
		QuotedCdnSrc:     strconv.Quote(cfg.CdnSrc),
		QuotedThemeSrcs:  jsArrayLiteral(cfg.ThemeSrcs),
		QuotedPluginSrcs: jsArrayLiteral(cfg.PluginSrcs),
		QuotedCode:       strconv.Quote(safeCode),
		QuotedTheme:      strconv.Quote(cfg.Theme),
		QuotedRenderer:   strconv.Quote(cfg.Renderer),
	}

	var out bytes.Buffer
	if err := scriptTemplate.Execute(&out, data); err != nil {
		return "", err
	}
	return out.String(), nil
}

func nextChartID() string {
	id := chartIDSeq.Add(1)
	return fmt.Sprintf("chart-%d", id)
}
