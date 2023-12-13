package geomap

import (
	"bytes"
	"fmt"
	"html/template"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/machbase/neo-server/mods/codec/logger"
	"github.com/machbase/neo-server/mods/stream/spec"
	"github.com/machbase/neo-server/mods/util/snowflake"
)

type GeoMap struct {
	logger logger.Logger
	output spec.OutputStream

	MapID  string
	Width  string
	Height string

	JSCodes   []string
	JSAssets  []string
	CSSAssets []string
	PageTitle string
}

var idGen, _ = snowflake.NewNode(time.Now().Unix() % 1024)

func New() *GeoMap {
	var id = "rainyDayMap"
	if idGen != nil {
		id = strings.TrimSuffix(idGen.Generate().Base64(), "=")
	}
	return &GeoMap{
		MapID:  id,
		Width:  "600px",
		Height: "600px",
	}
}

func (gm *GeoMap) ContentType() string {
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

func (gm *GeoMap) Flush(heading bool) {
}

func (gm *GeoMap) Open() error {
	return nil
}

func (gm *GeoMap) AddRow(values []any) error {
	return nil
}

func (gm *GeoMap) Close() {
	if gm.output == nil {
		return
	}

	if len(gm.JSAssets) == 0 {
		gm.JSAssets = append(gm.JSAssets, "https://unpkg.com/leaflet@1.9.4/dist/leaflet.js")
	}
	if len(gm.CSSAssets) == 0 {
		gm.CSSAssets = append(gm.CSSAssets, "https://unpkg.com/leaflet@1.9.4/dist/leaflet.css")
	}
	// gm.JSCodes = append(gm.JSCodes, fmt.Sprintf(`L.marker([51.5, -0.09]).addTo(geomap_%s);`, gm.MapID))
	// gm.JSCodes = append(gm.JSCodes, fmt.Sprintf(`L.circle([51.508, -0.11], { color: 'red', fillColor: '#f03', fillOpacity: 0.5, radius: 500 }).addTo(geomap_%s);`, gm.MapID))
	// gm.JSCodes = append(gm.JSCodes, fmt.Sprintf(`L.polygon([[51.509, -0.08],[51.503, -0.06],[51.51, -0.047]]).addTo(geomap_%s);`, gm.MapID))
	gm.Render()
}

func (gm *GeoMap) Render() {
	contents := []string{HeaderTemplate, BaseTemplate, HtmlTemplate}
	tpl := template.New("geomap").Funcs(template.FuncMap{
		"safeJS": func(s interface{}) template.JS {
			return template.JS(fmt.Sprint(s))
		},
		"isSet": isSet,
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

// isSet check if the field exist in the chart instance
// Shamed copy from https://stackoverflow.com/questions/44675087/golang-template-variable-isset
func isSet(name string, data interface{}) bool {
	v := reflect.ValueOf(data)

	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		return false
	}

	return v.FieldByName(name).IsValid()
}

var pat = regexp.MustCompile(`(__f__")|("__f__)|(__f__)`)
