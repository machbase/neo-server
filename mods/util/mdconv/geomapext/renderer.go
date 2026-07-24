package geomapext

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

	"github.com/machbase/neo-server/v8/mods/util/geomapjs"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"
)

var mapIDSeq atomic.Uint64

var scriptClosePattern = regexp.MustCompile(`(?i)</script>`)

//go:embed renderer_js.tmpl
var scriptTemplateText string

var scriptTemplate = template.Must(template.New("geomapext-script").Option("missingkey=error").Parse(scriptTemplateText))

type scriptTemplateData struct {
	QuotedID         string
	QuotedLoader     string
	QuotedLeafletSrc string
	QuotedLeafletCSS string
	QuotedCDNSrc     string
	QuotedCDNCSS     string
	QuotedTile       string
	QuotedTileOption string
	QuotedFit        string
	QuotedCenter     string
	QuotedZoom       string
	QuotedGray       string
	QuotedPayload    string
	GeoJSONOptions   string
}

type HTMLRenderer struct {
	DarkMode bool
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
	_, _ = w.WriteString(`<div class="geomapext">`)

	b := bytes.Buffer{}
	lines := n.Lines()
	for i := 0; i < lines.Len(); i++ {
		line := lines.At(i)
		b.Write(line.Value(src))
	}

	if b.Len() == 0 {
		_, _ = w.WriteString(`<div class="geomapext-error">Geomap JSON is empty.</div>`)
		return ast.WalkContinue, nil
	}

	cfg := defaultRenderConfig(r.DarkMode)
	if err := applyFenceOptions(&cfg, n.Options); err != nil {
		_, _ = w.WriteString(`<div class="geomapext-error">` + html.EscapeString(err.Error()) + `</div>`)
		return ast.WalkContinue, err
	}

	id := nextMapID()
	style := `width:` + cfg.Width + `;height:` + cfg.Height
	_, _ = w.WriteString(`<div class="geomapext-map" id="` + html.EscapeString(id) + `" style="` + html.EscapeString(style) + `"></div>`)

	script, err := buildScript(id, b.String(), cfg)
	if err != nil {
		_, _ = w.WriteString(`<div class="geomapext-error">` + html.EscapeString(err.Error()) + `</div>`)
		return ast.WalkContinue, err
	}
	_, _ = w.WriteString(`<script type="text/javascript">`)
	_, _ = w.WriteString(script)
	_, _ = w.WriteString(`</script>`)
	return ast.WalkContinue, nil
}

func buildScript(id string, payload string, cfg renderConfig) (string, error) {
	safePayload := scriptClosePattern.ReplaceAllString(payload, `<\\/script>`)
	quotedTileOption := "null"
	if strings.TrimSpace(cfg.TileOption) != "" {
		quotedTileOption = strconv.Quote(cfg.TileOption)
	}

	data := scriptTemplateData{
		QuotedID:         strconv.Quote(id),
		QuotedLoader:     strconv.Quote(cfg.Loader),
		QuotedLeafletSrc: strconv.Quote(cfg.LeafletSrc),
		QuotedLeafletCSS: strconv.Quote(cfg.LeafletCSS),
		QuotedCDNSrc:     strconv.Quote(cfg.CDNSrc),
		QuotedCDNCSS:     strconv.Quote(cfg.CDNCSS),
		QuotedTile:       strconv.Quote(cfg.Tile),
		QuotedTileOption: quotedTileOption,
		QuotedFit:        strconv.Quote(cfg.Fit),
		QuotedCenter: fmt.Sprintf(
			"[%s,%s]",
			strconv.FormatFloat(cfg.Center[0], 'f', -1, 64),
			strconv.FormatFloat(cfg.Center[1], 'f', -1, 64),
		),
		QuotedZoom:     strconv.Itoa(cfg.Zoom),
		QuotedGray:     strconv.FormatFloat(cfg.Grayscale, 'f', -1, 64),
		QuotedPayload:  strconv.Quote(safePayload),
		GeoJSONOptions: geomapjs.GeoJSONOptionsObjectLiteral(true),
	}

	var out bytes.Buffer
	if err := scriptTemplate.Execute(&out, data); err != nil {
		return "", err
	}
	return out.String(), nil
}

func nextMapID() string {
	id := mapIDSeq.Add(1)
	return fmt.Sprintf("geomap-%d", id)
}
