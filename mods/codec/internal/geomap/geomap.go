package geomap

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/machbase/neo-server/v8/mods/codec/facility"
	"github.com/machbase/neo-server/v8/mods/codec/internal"
	"github.com/machbase/neo-server/v8/mods/nums"
	"github.com/machbase/neo-server/v8/mods/util/snowflake"
)

type GeoMap struct {
	internal.RowsEncoderBase
	output io.Writer

	GeomapID string
	Width    string
	Height   string

	toJsonOutput bool

	logger             facility.Logger
	volatileFileWriter facility.VolatileFileWriter

	InitialLatLon *nums.LatLon
	Bound         *nums.LatLonBound

	InitialZoomLevel int

	tileGrayscale float64
	tileTemplate  string
	tileOption    string

	JSCodes      []string
	JSAssets     []string
	CSSAssets    []string
	JSCodeAssets []string
	PageTitle    string

	crs    string
	layers []*Layer
	icons  []*Icon
}

func New() *GeoMap {
	return &GeoMap{
		logger:   facility.DiscardLogger,
		GeomapID: snowflake.Generate(),
		Width:    "600px",
		Height:   "600px",
		crs:      "L.CRS.EPSG3857",
	}
}

func (gm *GeoMap) ContentType() string {
	if gm.toJsonOutput {
		return "application/json"
	}
	return "text/html"
}

func (gm *GeoMap) SetLogger(l facility.Logger) {
	gm.logger = l
}

func (gm *GeoMap) SetVolatileFileWriter(w facility.VolatileFileWriter) {
	gm.volatileFileWriter = w
}

func (gm *GeoMap) SetOutputStream(o io.Writer) {
	gm.output = o
}

func (gm *GeoMap) SetGeomapID(id string) {
	gm.GeomapID = id
}

func (gm *GeoMap) SetSize(width, height string) {
	gm.Width = width
	gm.Height = height
}

func (gm *GeoMap) SetMapAssets(args ...string) {
	for _, url := range args {
		if strings.HasSuffix(url, ".css") {
			gm.JSAssets = append(gm.CSSAssets, url)
		} else {
			gm.JSAssets = append(gm.JSAssets, url)
		}
	}
}

func (gm *GeoMap) SetInitialLocation(latlon *nums.LatLon, zoomLevel int) {
	gm.InitialLatLon = latlon
	gm.InitialZoomLevel = zoomLevel
}

func (gm *GeoMap) SetTileTemplate(url string) {
	gm.tileTemplate = url
}

func (gm *GeoMap) SetTileOption(opt string) {
	opt = strings.TrimSpace(opt)
	if !strings.HasPrefix(opt, "{") {
		opt = "{" + opt + "}"
	}
	gm.tileOption = opt
}

func (gm *GeoMap) TileTemplate() string {
	return gm.tileTemplate
}

func (gm *GeoMap) SetGeoMapJson(flag bool) {
	gm.toJsonOutput = flag
}

func (gm *GeoMap) SetTileGrayscale(grayscale float64) {
	gm.tileGrayscale = grayscale
}

func (gm *GeoMap) TileGrayscale() int {
	scale := gm.tileGrayscale
	if scale < 0 {
		scale = 0
	}
	if scale > 1 {
		scale = 1
	}
	return int(100 * scale)
}

func (gm *GeoMap) SetIcon(name string, opt string) {
	if !strings.HasPrefix(strings.TrimSpace(opt), "{") {
		opt = "{" + opt + "}"
	}
	icn := &Icon{}
	if err := json.Unmarshal([]byte(opt), icn); err != nil {
		gm.logger.LogWarnf("GEOMAP icon option", err.Error())
		return
	}
	icn.Name = name
	for _, n := range gm.icons {
		if n.Name == icn.Name { // already exists
			gm.logger.LogWarnf("GEOMAP icon %q already exists.", icn.Name)
			return
		}
	}
	gm.icons = append(gm.icons, icn)
}

func (gm *GeoMap) Flush(heading bool) {
}

func (gm *GeoMap) Open() error {
	return nil
}

func (gm *GeoMap) extendBound(lat, lon float64) {
	if gm.Bound == nil {
		gm.Bound = nums.NewLatLonBound(&nums.LatLon{Lat: lat, Lon: lon})
	} else {
		gm.Bound = gm.Bound.Extend(&nums.LatLon{Lat: lat, Lon: lon})
	}
}

func (gm *GeoMap) AddRow(values []any) error {
	for _, val := range values {
		if val == nil {
			continue
		}
		if m, ok := val.(map[string]any); ok {
			layer, err := NewLayer(m, gm.extendBound)
			if err != nil {
				return err
			}
			gm.layers = append(gm.layers, layer)
		}
	}
	return nil
}

func (gm *GeoMap) appendJSCode(lines ...string) {
	gm.JSCodes = append(gm.JSCodes, lines...)
}

func (gm *GeoMap) Close() {
	if gm.output == nil {
		return
	}
	if gm.InitialLatLon == nil {
		if gm.Bound != nil && !gm.Bound.IsEmpty() {
			gm.InitialLatLon = gm.Bound.Center()
		} else {
			gm.InitialLatLon = nums.NewLatLon(51.505, -0.09) // <- London
		}
	}
	if gm.InitialZoomLevel == 0 {
		gm.InitialZoomLevel = 13
	}
	// https://unpkg.com/leaflet@1.9.4/dist/leaflet.js
	gm.JSAssets = append([]string{"/web/geomap/leaflet.js"}, gm.JSAssets...)
	// https://unpkg.com/leaflet@1.9.4/dist/leaflet.css
	gm.CSSAssets = append([]string{"/web/geomap/leaflet.css"}, gm.CSSAssets...)
	if gm.tileTemplate == "" {
		gm.tileTemplate = `https://tile.openstreetmap.org/{z}/{x}/{y}.png`
	} else if gm.tileTemplate == "vworld" {
		gm.tileTemplate = `https://xdworld.vworld.kr/2d/Base/service/{z}/{x}/{y}.png`
	} else if gm.tileTemplate == "kakao" {
		gm.tileTemplate = `http://map{s}.daumcdn.net/map_2d_hd/2106wof/L{z}/{y}/{x}.png`
		gm.tileOption = `{"tms": true, "subdomains": "01234", "zoomReverse":true, "zoomOffset": 1, "maxZoom":13, "minZoom":0 }`
		gm.crs = "__crs"
		// https://github.com/proj4js/proj4js/releases/tag/2.9.2
		gm.JSAssets = append(gm.JSAssets, "/web/geomap/proj4.js")
		// Leaflet and proj4 must be loaded first
		// https://github.com/kartena/Proj4Leaflet/releases/tag/1.0.1
		gm.JSAssets = append(gm.JSAssets, "/web/geomap/proj4leaflet.js")
		// add crs code
		gm.appendJSCode(crsMarshalJS(nums.KakaoCRS, gm.crs))
	}

	gm.appendJSCode(`var map;`)
	gm.appendJSCode(`if (opt && opt.map) {`)
	gm.appendJSCode(`  map = opt.map;`)
	// remove all layers except the tile layer.
	gm.appendJSCode(`  opt.map.eachLayer(function (layer) {`)
	gm.appendJSCode(`    if (!(layer instanceof L.TileLayer)) {`)
	gm.appendJSCode(`      opt.map.removeLayer(layer);`)
	gm.appendJSCode(`    }`)
	gm.appendJSCode(`  });`)
	gm.appendJSCode(`} else {`)
	gm.appendJSCode(fmt.Sprintf(`  map = L.map("%s", {crs: %s, attributionControl:false});`, gm.GeomapID, gm.crs))
	if gm.tileOption != "" {
		gm.appendJSCode(fmt.Sprintf(`  L.tileLayer("%s", %s).addTo(map);`, gm.tileTemplate, gm.tileOption))
	} else {
		gm.appendJSCode(fmt.Sprintf(`  L.tileLayer("%s").addTo(map);`, gm.tileTemplate))
	}
	gm.appendJSCode(`  opt.map = map;`)
	gm.appendJSCode(`}`)

	if gm.Bound != nil && !gm.Bound.IsEmpty() && !gm.Bound.IsPoint() {
		gm.appendJSCode(fmt.Sprintf("map.fitBounds(%s);", gm.Bound.String()))
	} else {
		gm.appendJSCode(fmt.Sprintf("map.setView(%s,%d);", gm.InitialLatLon.String(), gm.InitialZoomLevel))
	}

	for _, icn := range gm.icons {
		var icnJson string
		if cnt, err := json.Marshal(icn); err != nil {
			continue
		} else {
			icnJson = string(cnt)
		}
		gm.appendJSCode(fmt.Sprintf(`var %s = L.icon(%s);`, icn.Name, icnJson))
	}

	for objIdx, layer := range gm.layers {
		var popupMap map[string]any
		if popup, ok := layer.Properties["popup"]; ok {
			if m, ok := popup.(map[string]any); ok {
				popupMap = m
				delete(layer.Properties, "popup")
			}
		}
		var tooltipMap map[string]any
		if tooltip, ok := layer.Properties["tooltip"]; ok {
			if m, ok := tooltip.(map[string]any); ok {
				tooltipMap = m
				delete(layer.Properties, "tooltip")
			}
		}
		gm.appendJSCode(fmt.Sprintf(`var obj%d = %s.addTo(map);`, objIdx, layer.LeafletJS()))
		if popupMap != nil {
			openCode := ""
			contentCode := ""
			if content, ok := popupMap["content"].(string); ok {
				contentCode = content
				delete(popupMap, "content")
			}
			if open, ok := popupMap["open"].(bool); ok {
				if open {
					openCode = `.openPopup()`
				}
				delete(popupMap, "open")
			}
			popupJS, _ := MarshalJS(popupMap)
			gm.appendJSCode(fmt.Sprintf(`var popup%d = obj%d.bindPopup(%q, %s)%s;`, objIdx, objIdx, contentCode, popupJS, openCode))
		}
		if tooltipMap != nil {
			openCode := ""
			contentCode := ""
			if content, ok := tooltipMap["content"].(string); ok {
				contentCode = content
				delete(tooltipMap, "content")
			}
			if open, ok := tooltipMap["open"].(bool); ok {
				if open {
					openCode = `.openTooltip()`
				}
				delete(tooltipMap, "open")
			}
			tooltipJS, _ := MarshalJS(tooltipMap)
			gm.appendJSCode(fmt.Sprintf(`var tooltip%d = obj%d.bindTooltip(%q, %s)%s;`, objIdx, objIdx, contentCode, tooltipJS, openCode))
		}
	}

	if gm.toJsonOutput && gm.volatileFileWriter != nil {
		prefix := strings.TrimSuffix(gm.volatileFileWriter.VolatileFilePrefix(), "/")

		path := fmt.Sprintf("%s/%s_opt.js", prefix, gm.GeomapID)
		gm.volatileFileWriter.VolatileFileWrite(path, []byte(gm.JSCodesOptionNoEscaped()), time.Now().Add(30*time.Second))
		gm.JSCodeAssets = append(gm.JSCodeAssets, path)

		path = fmt.Sprintf("%s/%s.js", prefix, gm.GeomapID)
		gm.volatileFileWriter.VolatileFileWrite(path, []byte(gm.JSCodesNoEscaped()), time.Now().Add(30*time.Second))
		gm.JSCodeAssets = append(gm.JSCodeAssets, path)
	}
	if gm.toJsonOutput {
		gm.renderJSON()
	} else {
		gm.renderHTML()
	}
}

func crsMarshalJS(c *nums.CRS, varName string) string {
	res := []string{}
	for _, r := range c.Options["resolutions"].([]float64) {
		res = append(res, fmt.Sprintf("%v", r))
	}
	org := ""
	if v, ok := c.Options["origin"].([]float64); ok {
		org = fmt.Sprintf("[%.f,%.f]", v[0], v[1])
	}
	bound := ""
	if v, ok := c.Options["bounds"].([][]float64); ok {
		bound = fmt.Sprintf("[%.f,%.f],[%.f,%.f]", v[0][0], v[0][1], v[1][0], v[1][1])
	}
	return fmt.Sprintf(`var %s = new L.Proj.CRS('%s', '%s', {
			resolutions: [%s],
			origin: %s,
			bounds: L.bounds(%s)
		});`,
		varName,
		c.Code, c.Proj, strings.Join(res, ","), org, bound)
}

func (gm *GeoMap) JSAssetsNoEscaped() template.HTML {
	lst := []string{}
	for _, itm := range gm.JSAssets {
		lst = append(lst, fmt.Sprintf("%q", itm))
	}
	return template.HTML("[" + strings.Join(lst, ",") + "]")
}

func (gm *GeoMap) CSSAssetsNoEscaped() template.HTML {
	lst := []string{}
	for _, itm := range gm.CSSAssets {
		lst = append(lst, fmt.Sprintf("%q", itm))
	}
	return template.HTML("[" + strings.Join(lst, ",") + "]")
}

func (gm *GeoMap) JSCodeAssetsNoEscaped() template.HTML {
	lst := []string{}
	for _, itm := range gm.JSCodeAssets {
		lst = append(lst, fmt.Sprintf("%q", itm))
	}
	return template.HTML("[" + strings.Join(lst, ",") + "]")
}

// The variable name is mapID.
var mapOptions = `var %s = {
    geojson: {
        pointToLayer: function (feature, latlng) {
            if (feature.properties && feature.properties.icon) {
                return L.marker(latlng, {icon: feature.properties.icon});
            }
            return L.circleMarker(latlng, {
                radius: (feature.properties && feature.properties.radius) ? feature.properties.radius : 10,
                stroke: (feature.properties && feature.properties.stroke != undefined) ? feature.properties.stroke : true,
                color:  (feature.properties && feature.properties.color) ? feature.properties.color : "#3388ff", 
                opacity: (feature.properties && feature.properties.opacity) ? feature.properties.opacity : 1.0,
                fillOpacity: (feature.properties && feature.properties.fillOpacity) ? feature.properties.fillOpacity : 0.2
            });
        },
        style: function (feature) {
            return {
                radius: (feature.properties && feature.properties.radius) ? feature.properties.radius : 4,
                stroke: (feature.properties && feature.properties.stroke != undefined) ? feature.properties.stroke : true,
                weight: (feature.properties && feature.properties.weight) ? feature.properties.weight : 3,
                color:  (feature.properties && feature.properties.color) ? feature.properties.color : "#3388ff", 
                opacity: (feature.properties && feature.properties.opacity) ? feature.properties.opacity : 1.0,
                fillOpacity: (feature.properties && feature.properties.fillOpacity) ? feature.properties.fillOpacity : 0.2
            };
        },
        onEachFeature: function (feature, layer) {
            if (feature.properties && feature.properties.popup && feature.properties.popup.content) {
                if (feature.properties.popup.open) {
                    layer.bindPopup(feature.properties.popup.content).openPopup();
                } else {
                    layer.bindPopup(feature.properties.popup.content);
                }
            }
        },
    },
};
`

func (gm *GeoMap) JSCodesOptionNoEscaped() template.JS {
	return template.JS(fmt.Sprintf(mapOptions, gm.GeomapID))
}

func (gm *GeoMap) JSCodesNoEscaped() template.JS {
	lst := []string{"((opt)=>{"}
	lst = append(lst, gm.JSCodes...)
	lst = append(lst, fmt.Sprintf("})(%s);", gm.GeomapID))
	return template.JS(strings.Join(lst, "\n"))
}

func (gm *GeoMap) renderJSON() {
	contents := []string{JsonTemplate}
	tpl := template.New("geomap")
	tpl = template.Must(tpl.Parse(contents[0]))
	if err := tpl.ExecuteTemplate(gm.output, "geomap", gm); err != nil {
		gm.output.Write([]byte(err.Error()))
	}
}

func (gm *GeoMap) renderHTML() {
	contents := []string{HeaderTemplate, BaseTemplate, HtmlTemplate}
	tpl := template.New("geomap").Funcs(template.FuncMap{
		"safeJS": func(s interface{}) template.JS {
			return template.JS(fmt.Sprint(s))
		},
	})
	tpl = template.Must(tpl.Parse(contents[0]))
	for _, cont := range contents[1:] {
		tpl = template.Must(tpl.Parse(cont))
	}

	var buf bytes.Buffer
	if err := tpl.ExecuteTemplate(&buf, "geomap", gm); err != nil {
		buf.WriteString(err.Error())
	}

	content := pat.ReplaceAll(buf.Bytes(), []byte(""))

	if _, err := gm.output.Write(content); err != nil {
		gm.output.Write([]byte(err.Error()))
	}
}

var pat = regexp.MustCompile(`(__f__")|("__f__)|(__f__)`)
