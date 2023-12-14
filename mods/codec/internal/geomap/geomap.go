package geomap

import (
	"bytes"
	"fmt"
	"html/template"
	"regexp"
	"strings"
	"time"

	"github.com/machbase/neo-server/mods/codec/logger"
	"github.com/machbase/neo-server/mods/nums"
	"github.com/machbase/neo-server/mods/stream/spec"
	"github.com/machbase/neo-server/mods/util/snowflake"
)

type GeoMap struct {
	logger logger.Logger
	output spec.OutputStream

	MapID  string
	Width  string
	Height string

	toJsonOutput bool

	InitialLatLng    *nums.LatLng
	InitialZoomLevel int

	tileGrayscale float64
	tileTemplate  string
	tileOption    string

	JSCodes   []string
	JSAssets  []string
	CSSAssets []string
	PageTitle string

	objs        []nums.Geography
	icons       []GeoIcon
	pointStyles map[string]nums.GeoPointStyle
}

var defaultPointStyle = nums.GeoPointStyle{
	Type: "circleMarker",
	Properties: nums.GeoProperties{
		"radius":      7,
		"stroke":      false,
		"color":       "#ff0000",
		"opacity":     1.0,
		"fillOpacity": 1.0,
	},
}

var idGen, _ = snowflake.NewNode(time.Now().Unix() % 1024)

func New() *GeoMap {
	var id = "rainyDayMap"
	if idGen != nil {
		id = strings.TrimSuffix(idGen.Generate().Base64(), "=")
	}
	return &GeoMap{
		MapID:       id,
		Width:       "600px",
		Height:      "600px",
		pointStyles: map[string]nums.GeoPointStyle{},
	}
}

func (gm *GeoMap) ContentType() string {
	if gm.toJsonOutput {
		return "application/json"
	}
	return "text/html"
}

func (gm *GeoMap) SetLogger(l logger.Logger) {
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

func (gm *GeoMap) SetInitialLocation(latlng *nums.LatLng, zoomLevel int) {
	gm.InitialLatLng = latlng
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

func (gm *GeoMap) SetGeoPointStyle(name string, props map[string]any) {
	ps := nums.GeoPointStyle{Type: defaultPointStyle.Type, Properties: map[string]any{}}
	ps.Properties.Copy(defaultPointStyle.Properties)
	ps.Properties.Copy(props)
	gm.pointStyles[name] = ps
}

func (gm *GeoMap) SetGeoMarker(marker nums.GeoMarker) {
	gm.objs = append(gm.objs, marker)
}

func (gm *GeoMap) SetGeoIcon(name string, url string, width float64, height float64) {
	gm.icons = append(gm.icons, GeoIcon{
		name:   name,
		url:    url,
		width:  width,
		height: height,
	})
}

type GeoIcon struct {
	name   string
	url    string
	width  float64
	height float64
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
	if gm.InitialLatLng == nil {
		gm.InitialLatLng = nums.NewLatLng(51.505, -0.09) // <- London
	}
	if gm.InitialZoomLevel == 0 {
		gm.InitialZoomLevel = 13
	}
	if gm.tileTemplate == "" {
		gm.tileTemplate = `https://tile.openstreetmap.org/{z}/{x}/{y}.png`
	}
	if gm.tileOption == "" {
		gm.tileOption = `{"maxZoom":19}`
	}
	if len(gm.JSAssets) == 0 {
		gm.JSAssets = append(gm.JSAssets, "https://unpkg.com/leaflet@1.9.4/dist/leaflet.js")
	}
	if len(gm.CSSAssets) == 0 {
		gm.CSSAssets = append(gm.CSSAssets, "https://unpkg.com/leaflet@1.9.4/dist/leaflet.css")
	}

	if gm.toJsonOutput {
		gm.renderJSON()
	} else {
		gm.renderHTML()
	}
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

func (gm *GeoMap) LayersNoEscaped() template.HTML {
	return "[]"
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
	gm.JSCodes = append(gm.JSCodes, fmt.Sprintf(`var geomap_%s = L.map("%s").setView(%s, %d);`, gm.MapID, gm.MapID, gm.InitialLatLng.String(), gm.InitialZoomLevel))
	gm.JSCodes = append(gm.JSCodes, fmt.Sprintf(`L.tileLayer("%s", %s).addTo(geomap_%s);`, gm.tileTemplate, gm.tileOption, gm.MapID))

	for _, icn := range gm.icons {
		gm.JSCodes = append(gm.JSCodes, fmt.Sprintf(`var %s = L.icon({ iconUrl: '%s', iconSize: [%d, %d], iconAnchor: [%d, %d]});`,
			icn.name, icn.url, int(icn.width), int(icn.height), int(icn.width/2), int(icn.height/2)))
	}
	for i, obj := range gm.objs {
		bindPopup := ""
		openPopup := ""
		objName := fmt.Sprintf("geo_obj_%d_%s", i, gm.MapID)
		objOption := "{}"
		pointStyleName := ""

		objType := "marker"
		if mkr, ok := obj.(nums.GeoMarker); ok {
			objType = mkr.Marker()
		} else {
			switch obj.(type) {
			case *nums.Circle:
				objType = "circle"
			case *nums.SingleLatLng:
				objType = "point"
			}
		}

		if props := obj.Properties(); props != nil {
			if v, ok := props.PopString("popup.content"); ok {
				bindPopup = fmt.Sprintf("%s.bindPopup(%q);", objName, v)
			}
			if flag, ok := props.PopBool("popup.open"); ok && flag {
				openPopup = fmt.Sprintf("%s.openPopup()", objName)
			}
			if ps, ok := props.PopString("point.style"); ok {
				pointStyleName = ps
			}
			if objType == "point" {
				style := defaultPointStyle
				if st, ok := gm.pointStyles[pointStyleName]; ok {
					style = st
				}
				objType = style.Type
				props.Copy(style.Properties)
			}
			if js, err := props.MarshalJS(); err == nil {
				objOption = js
			}
		}

		objCoord := ""
		switch obj.(type) {
		case nums.GeoCircleMarker, nums.GeoPointMarker, nums.GeoCircle, *nums.SingleLatLng:
			coord := obj.Coordinates()
			if len(coord) > 0 {
				objCoord = fmt.Sprintf("[%v,%v]", coord[0][0], coord[0][1])
			}
		default:
			coord := obj.Coordinates()
			arr := []string{}
			for _, p := range coord {
				arr = append(arr, fmt.Sprintf("[%v,%v]", p[0], p[1]))
			}
			objCoord = "[" + strings.Join(arr, ",") + "]"
		}

		gm.JSCodes = append(gm.JSCodes, fmt.Sprintf(`var %s = L.%s(%s, %s).addTo(geomap_%s);`,
			objName, objType, objCoord, objOption, gm.MapID))
		if bindPopup != "" {
			gm.JSCodes = append(gm.JSCodes, bindPopup)
		}
		if openPopup != "" {
			gm.JSCodes = append(gm.JSCodes, openPopup)
		}
	}
	// gm.JSCodes = append(gm.JSCodes, fmt.Sprintf(`L.polygon([[51.509, -0.08],[51.503, -0.06],[51.51, -0.047]]).addTo(geomap_%s);`, gm.MapID))

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
