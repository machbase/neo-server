package geomap

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"regexp"
	"strings"
	"time"

	"github.com/machbase/neo-server/mods/codec/facility"
	"github.com/machbase/neo-server/mods/nums"
	"github.com/machbase/neo-server/mods/stream/spec"
	"github.com/machbase/neo-server/mods/util/snowflake"
)

type GeoMap struct {
	logger facility.Logger
	output spec.OutputStream

	MapID  string
	Width  string
	Height string

	toJsonOutput bool

	InitialLatLon    *nums.LatLon
	InitialZoomLevel int

	mapOption string

	tileGrayscale float64
	tileTemplate  string
	tileOption    string

	JSCodes   []string
	JSAssets  []string
	CSSAssets []string
	PageTitle string

	objs        []nums.Geography
	icons       []*Icon
	pointStyles map[string]*PointStyle
}

var idGen, _ = snowflake.NewNode(time.Now().Unix() % 1024)

func New() *GeoMap {
	var id = "rainyDayMap"
	if idGen != nil {
		id = strings.TrimSuffix(idGen.Generate().Base64(), "=")
	}
	return &GeoMap{
		logger:      facility.DiscardLogger,
		MapID:       id,
		Width:       "600px",
		Height:      "600px",
		pointStyles: map[string]*PointStyle{},
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

func (gm *GeoMap) SetOutputStream(o spec.OutputStream) {
	gm.output = o
}

func (gm *GeoMap) SetMapId(id string) {
	gm.MapID = id
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

func (gm *GeoMap) SetLayer(obj nums.Geography) {
	gm.objs = append(gm.objs, obj)
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

var pointTypeNames = map[string]string{
	"marker":       "marker",
	"circle":       "circle",
	"circlemarker": "circleMarker",
}

func (gm *GeoMap) SetPointStyle(name string, typ string, opt string) {
	if rn, ok := pointTypeNames[strings.ToLower(typ)]; !ok {
		gm.logger.LogWarnf("GEOMAP pointStyle unknown type %q", typ)
	} else {
		typ = rn
	}
	if !strings.HasPrefix(strings.TrimSpace(opt), "{") {
		opt = "{" + opt + "}"
	}
	pstyle := &PointStyle{Name: name, Type: typ, Properties: map[string]any{}}
	pstyle.Properties.Copy(defaultPointStyle.Properties)
	if err := json.Unmarshal([]byte(opt), &pstyle.Properties); err != nil {
		gm.logger.LogWarnf("GEOMAP pointStyle option", err.Error())
		return
	}
	pstyle.Name = name
	gm.pointStyles[name] = pstyle
}

func (gm *GeoMap) Flush(heading bool) {
}

func (gm *GeoMap) Open() error {
	return nil
}

func (gm *GeoMap) AddRow(values []any) error {
	for _, val := range values {
		if v, ok := val.(nums.Geography); ok {
			gm.objs = append(gm.objs, v)
		}
	}
	return nil
}

func (gm *GeoMap) Close() {
	if gm.output == nil {
		return
	}
	if gm.InitialLatLon == nil {
		gm.InitialLatLon = nums.NewLatLon(51.505, -0.09) // <- London
	}
	if gm.InitialZoomLevel == 0 {
		gm.InitialZoomLevel = 13
	}
	if gm.tileTemplate == "" {
		gm.tileTemplate = `https://tile.openstreetmap.org/{z}/{x}/{y}.png`
	} else if gm.tileTemplate == "kakao" {
		gm.tileTemplate = `http://map{s}.daumcdn.net/map_2d_hd/2106wof/L{z}/{y}/{x}.png`
		gm.tileOption = `{"tms": true, "subdomains": "01234", "zoomReverse":true, "zoomOffset": 1, "maxZoom":13, "minZoom":0 }`
		crsVar := "kakaoCrs"
		gm.mapOption = fmt.Sprintf(`{crs: %s}`, crsVar)
		// Leaflet and proj4 must be loaded first
		gm.JSAssets = append(gm.JSAssets, "/web/geomap/leaflet.js")
		// https://github.com/proj4js/proj4js/releases/tag/2.9.2
		gm.JSAssets = append(gm.JSAssets, "/web/geomap/proj4.js")
		// https://github.com/kartena/Proj4Leaflet/releases/tag/1.0.1
		gm.JSAssets = append(gm.JSAssets, "/web/geomap/proj4leaflet.js")

		gm.JSCodes = append(gm.JSCodes, crsMarshalJS(nums.KakaoCRS, crsVar))
	}
	if gm.tileOption == "" {
		gm.tileOption = `{"maxZoom":19}`
	}
	if gm.mapOption == "" {
		gm.mapOption = "{}"
	}
	if len(gm.JSAssets) == 0 {
		// https://unpkg.com/leaflet@1.9.4/dist/leaflet.js
		gm.JSAssets = append(gm.JSAssets, "/web/geomap/leaflet.js")
	}
	if len(gm.CSSAssets) == 0 {
		// https://unpkg.com/leaflet@1.9.4/dist/leaflet.css
		gm.CSSAssets = append(gm.CSSAssets, "/web/geomap/leaflet.css")
	}

	if gm.toJsonOutput {
		gm.renderJSON()
	} else {
		gm.renderHTML()
	}
}

func crsMarshalJS(c *nums.CRS, varname string) string {
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
		varname,
		c.Code, c.Proj, strings.Join(res, ","), org, bound)
}

func (gm *GeoMap) Layers() []*Layer {
	var ret []*Layer
	for i, obj := range gm.objs {
		layer := &Layer{
			Name: fmt.Sprintf("geo_obj_%d_%s", i, gm.MapID),
			Type: "marker",
		}

		if mkr, ok := obj.(nums.GeoMarker); ok {
			layer.Type = mkr.Marker()
		} else {
			switch ov := obj.(type) {
			case *nums.Circle:
				layer.Type = "circle"
			case *nums.SingleLatLon:
				layer.Type = "point"
			case *nums.MultiLatLon:
				switch ov.Type() {
				case "Polygon":
					layer.Type = "polygon"
				case "LineString":
					layer.Type = "polyline"
				}
			}
		}

		if props := obj.Properties(); props != nil {
			if v, ok := props.PopString("popup.content"); ok {
				layer.Popup = &Popup{Content: v}
				if flag, ok := props.PopBool("popup.open"); ok && flag {
					layer.Popup.Open = true
				}
			}
			pointStyleName := ""
			if ps, ok := props.PopString("point.style"); ok {
				pointStyleName = ps
			}
			if layer.Type == "point" {
				layer.Type = defaultPointStyle.Type
				layer.Option = defaultPointStyle.Properties
				if st, ok := gm.pointStyles[pointStyleName]; ok {
					layer.Type = st.Type
					layer.Option = st.Properties
				}
			} else {
				layer.Option = props
			}
		}

		switch obj.(type) {
		case nums.GeoCircleMarker, nums.GeoPointMarker, nums.GeoCircle, *nums.SingleLatLon:
			coord := obj.Coordinates()
			if len(coord) > 0 {
				layer.Coord = fmt.Sprintf("[%v,%v]", coord[0][0], coord[0][1])
			}
		default:
			coord := obj.Coordinates()
			arr := []string{}
			for _, p := range coord {
				arr = append(arr, fmt.Sprintf("[%v,%v]", p[0], p[1]))
			}
			layer.Coord = "[" + strings.Join(arr, ",") + "]"
		}

		ret = append(ret, layer)
	}
	return ret
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

func (gm *GeoMap) TileOptionNoEscaped() template.HTML {
	return template.HTML(gm.tileOption)
}

func (gm *GeoMap) IconsNoEscaped() template.HTML {
	if len(gm.icons) == 0 {
		return "[]"
	}
	if b, err := json.Marshal(gm.icons); err != nil {
		gm.logger.LogError("GEOMAP marshal icons", err.Error())
		return "[]"
	} else {
		return template.HTML(string(b))
	}
}

func (gm *GeoMap) LayersNoEscaped() template.HTML {
	list := gm.Layers()
	if len(list) == 0 {
		return "[]"
	}
	if b, err := json.Marshal(list); err != nil {
		gm.logger.LogError("GEOMAP marshal layers", err.Error())
		return "[]"
	} else {
		return template.HTML(string(b))
	}
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
	gm.JSCodes = append(gm.JSCodes, fmt.Sprintf(`var geomap_%s = L.map("%s", %s).setView(%s, %d);`,
		gm.MapID, gm.MapID, gm.mapOption, gm.InitialLatLon.String(), gm.InitialZoomLevel))
	gm.JSCodes = append(gm.JSCodes, fmt.Sprintf(`L.tileLayer("%s", %s).addTo(geomap_%s);`, gm.tileTemplate, gm.tileOption, gm.MapID))

	for _, icn := range gm.icons {
		var icnJson string
		if cnt, err := json.Marshal(icn); err != nil {
			continue
		} else {
			icnJson = string(cnt)
		}
		gm.JSCodes = append(gm.JSCodes, fmt.Sprintf(`var %s = L.icon(%s);`, icn.Name, icnJson))
	}

	for _, layer := range gm.Layers() {
		opt, err := layer.Option.MarshalJS()
		if err != nil {
			gm.logger.LogWarnf("GEOMAP render %q option %s", layer.Name, err.Error())
			opt = "{}"
		}
		gm.JSCodes = append(gm.JSCodes, fmt.Sprintf(`var %s = L.%s(%s, %s).addTo(geomap_%s);`,
			layer.Name, layer.Type, layer.Coord, opt, gm.MapID))
		if layer.Popup != nil {
			gm.JSCodes = append(gm.JSCodes, fmt.Sprintf("%s.bindPopup(%q);", layer.Name, layer.Popup.Content))
			if layer.Popup.Open {
				gm.JSCodes = append(gm.JSCodes, fmt.Sprintf("%s.openPopup();", layer.Name))
			}
		}
	}

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
