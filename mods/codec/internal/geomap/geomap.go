package geomap

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/machbase/neo-server/v8/mods/codec/facility"
	"github.com/machbase/neo-server/v8/mods/codec/internal"
	"github.com/machbase/neo-server/v8/mods/nums"
	"github.com/machbase/neo-server/v8/mods/util/snowflake"
	"github.com/paulmach/orb/geojson"
)

type GeoMap struct {
	internal.RowsEncoderBase
	output io.Writer

	MapID  string
	Width  string
	Height string

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

	crs         string
	objs        []nums.Geography
	layers      []*Layer
	icons       []*Icon
	pointStyles map[string]*PointStyle
}

func New() *GeoMap {
	return &GeoMap{
		logger:      facility.DiscardLogger,
		MapID:       snowflake.Generate(),
		Width:       "600px",
		Height:      "600px",
		pointStyles: map[string]*PointStyle{},
		crs:         "L.CRS.EPSG3857",
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

var _sliceGeometries = []string{"Point", "MultiPoint", "LineString", "MultiLineString", "Polygon", "MultiPolygon", "GeometryCollection"}

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
		if v, ok := val.(nums.Geography); ok {
			gm.addGeoObject(v)
			gm.objs = append(gm.objs, v)
		} else if m, ok := val.(map[string]any); ok {
			if err := gm.addGeoJSON(m); err != nil {
				return err
			}
		}
	}
	return nil
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
		gm.JSCodes = append(gm.JSCodes, crsMarshalJS(nums.KakaoCRS, gm.crs))
	}

	gm.JSCodes = append(gm.JSCodes, fmt.Sprintf(`var map = L.map("%s", {crs: %s, attributionControl:false});`, gm.MapID, gm.crs))
	if gm.tileOption != "" {
		gm.JSCodes = append(gm.JSCodes, fmt.Sprintf(`L.tileLayer("%s", %s).addTo(map);`, gm.tileTemplate, gm.tileOption))
	} else {
		gm.JSCodes = append(gm.JSCodes, fmt.Sprintf(`L.tileLayer("%s").addTo(map);`, gm.tileTemplate))
	}

	if gm.Bound != nil && !gm.Bound.IsEmpty() {
		gm.JSCodes = append(gm.JSCodes, fmt.Sprintf("map.fitBounds(%s);", gm.Bound.String()))
	} else {
		gm.JSCodes = append(gm.JSCodes, fmt.Sprintf("map.setView(%s, %d);", gm.InitialLatLon.String(), gm.InitialZoomLevel))
	}

	if js, err := MarshalJS(defaultPointStyle.Properties); err == nil {
		gm.JSCodes = append(gm.JSCodes, fmt.Sprintf("var %s = %s;", defaultPointStyleVarName, js))
	}
	for n, v := range gm.pointStyles {
		if js, err := v.Properties.MarshalJS(); err != nil {
			gm.logger.LogWarnf("GEOMAP invalid point style %s", err.Error())
		} else {
			gm.JSCodes = append(gm.JSCodes, fmt.Sprintf("var %s = %s;", n, js))
		}
	}
	for _, icn := range gm.icons {
		var icnJson string
		if cnt, err := json.Marshal(icn); err != nil {
			continue
		} else {
			icnJson = string(cnt)
		}
		gm.JSCodes = append(gm.JSCodes, fmt.Sprintf(`var %s = L.icon(%s);`, icn.Name, icnJson))
	}

	for _, layer := range gm.layers {
		var opt string
		if layer.Style == "" {
			if v, err := layer.Option.MarshalJS(); err != nil {
				gm.logger.LogWarnf("GEOMAP render %q option %s", layer.Name, err.Error())
				opt = "{}"
			} else {
				opt = v
			}
		} else {
			opt = layer.Style
		}
		gm.JSCodes = append(gm.JSCodes, fmt.Sprintf(`var %s = L.%s(%s, %s).addTo(map);`,
			layer.Name, layer.Type, layer.Jsonized, opt))
		if layer.Popup != nil {
			if layer.Popup.Open {
				gm.JSCodes = append(gm.JSCodes, fmt.Sprintf("%s.bindPopup(%q).openPopup();", layer.Name, layer.Popup.Content))
			} else {
				gm.JSCodes = append(gm.JSCodes, fmt.Sprintf("%s.bindPopup(%q);", layer.Name, layer.Popup.Content))
			}
		}
	}

	if gm.volatileFileWriter != nil {
		prefix := gm.volatileFileWriter.VolatileFilePrefix()
		path := fmt.Sprintf("%s/%s.js", strings.TrimSuffix(prefix, "/"), gm.MapID)
		jscode := fmt.Sprintf("(()=>{\n%s\n})();", strings.Join(gm.JSCodes, "\n"))
		gm.volatileFileWriter.VolatileFileWrite(path, []byte(jscode), time.Now().Add(30*time.Second))
		gm.JSCodeAssets = append(gm.JSCodeAssets, path)
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

func (gm *GeoMap) addGeoJSON(m map[string]any) error {
	// Caution!!: geojson is "lon,lat" order
	typeAny, ok := m["type"]
	if !ok {
		return nil
	}
	typeString, ok := typeAny.(string)
	if !ok {
		return nil
	}
	if typeString == "FeatureCollection" {
		jsonBytes, _ := json.Marshal(m)
		obj, err := geojson.UnmarshalFeatureCollection(jsonBytes)
		if err != nil {
			return fmt.Errorf("GEOMAP invalid geojson %s", err.Error())
		}
		for _, f := range obj.Features {
			bound := f.Geometry.Bound()
			gm.extendBound(bound.Min.Lat(), bound.Min.Lon())
			gm.extendBound(bound.Max.Lat(), bound.Max.Lon())
		}
		layer := &Layer{
			Name: fmt.Sprintf("obj%d", len(gm.layers)),
			Type: "geoJSON",
		}
		layer.Popup = NewPopupMap(obj.ExtraMembers)
		jsonBytes, _ = obj.MarshalJSON()
		layer.Jsonized = string(jsonBytes)
		gm.layers = append(gm.layers, layer)
	} else if typeString == "Feature" {
		jsonBytes, _ := json.Marshal(m)
		obj, err := geojson.UnmarshalFeature(jsonBytes)
		if err != nil {
			return fmt.Errorf("GEOMAP invalid geojson %s", err.Error())
		}
		bound := obj.Geometry.Bound()
		gm.extendBound(bound.Min.Lat(), bound.Min.Lon())
		gm.extendBound(bound.Max.Lat(), bound.Max.Lon())
		layer := &Layer{
			Name: fmt.Sprintf("obj%d", len(gm.layers)),
			Type: "geoJSON",
		}
		layer.Popup = NewPopupMap(obj.Properties)
		jsonBytes, _ = obj.MarshalJSON()
		layer.Jsonized = string(jsonBytes)
		gm.layers = append(gm.layers, layer)
	} else if slices.Contains(_sliceGeometries, typeString) {
		jsonBytes, _ := json.Marshal(m)
		obj, err := geojson.UnmarshalGeometry(jsonBytes)
		if err != nil {
			return fmt.Errorf("GEOMAP invalid geojson %s", err.Error())
		}
		bound := obj.Coordinates.Bound()
		gm.extendBound(bound.Min.Lat(), bound.Min.Lon())
		gm.extendBound(bound.Max.Lat(), bound.Max.Lon())
		layer := &Layer{
			Name:     fmt.Sprintf("obj%d", len(gm.layers)),
			Type:     "geoJSON",
			Jsonized: string(jsonBytes),
		}
		gm.layers = append(gm.layers, layer)
	} else {
		return fmt.Errorf("GEOMAP invalid geojson %q", typeString)
	}
	return nil
}

func (gm *GeoMap) addGeoObject(obj nums.Geography) {
	// Caution!!: nums.Geography is "lat,lon" order
	layer := &Layer{
		Name: fmt.Sprintf("obj%d", len(gm.layers)),
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

	if orgProps := obj.Properties(); orgProps != nil {
		props := nums.GeoProperties{}
		props.Copy(orgProps)
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
			layer.Style = defaultPointStyleVarName
			if st, ok := gm.pointStyles[pointStyleName]; ok {
				layer.Type = st.Type
				layer.Style = pointStyleName
			}
		} else {
			layer.Option = props
		}
	}

	switch obj.(type) {
	case nums.GeoCircleMarker, nums.GeoPointMarker, nums.GeoCircle, *nums.SingleLatLon:
		coord := obj.Coordinates()
		if len(coord) > 0 {
			layer.Jsonized = fmt.Sprintf("[%v,%v]", coord[0][0], coord[0][1])
		}
	case *nums.MultiLatLon:
		coord := obj.Coordinates()
		buff := []string{}
		for _, c := range coord {
			buff = append(buff, fmt.Sprintf("[%v,%v]", c[0], c[1]))
		}
		layer.Jsonized = fmt.Sprintf("[%s]", strings.Join(buff, ","))
	default:
		coord := obj.Coordinates()
		arr := []string{}
		for _, p := range coord {
			arr = append(arr, fmt.Sprintf("[%v,%v]", p[0], p[1]))
		}
		layer.Jsonized = "[" + strings.Join(arr, ",") + "]"
	}

	for _, coord := range obj.Coordinates() {
		gm.extendBound(coord[0], coord[1])
	}

	gm.layers = append(gm.layers, layer)
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
