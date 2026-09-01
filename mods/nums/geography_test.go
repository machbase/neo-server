package nums

import (
	"encoding/json"
	"math"
	"reflect"
	"testing"
)

func TestLatLon(t *testing.T) {
	seoul := NewLatLon(37.5665, 126.9780)
	busan := NewLatLon(35.1796, 129.0756)

	if got, want := seoul.String(), "[37.5665,126.978]"; got != want {
		t.Fatalf("String() was %q, want %q", got, want)
	}
	if got, want := seoul.Array(), []float64{37.5665, 126.9780}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Array() was %v, want %v", got, want)
	}
	data, err := json.Marshal(seoul)
	if err != nil {
		t.Fatalf("json.Marshal() error: %v", err)
	}
	if got, want := string(data), "[37.5665,126.978]"; got != want {
		t.Fatalf("JSON was %s, want %s", got, want)
	}

	distance := Distance(*seoul, *busan)
	if distance < 320_000 || distance > 330_000 {
		t.Fatalf("Seoul-Busan distance was %f", distance)
	}
	if got := seoul.Distance(seoul); math.Abs(got) > 0.001 {
		t.Fatalf("distance to the same point was %f", got)
	}
	if got := seoul.Distance(busan); math.Abs(got-distance) > 0.001 {
		t.Fatalf("method distance was %f, function distance was %f", got, distance)
	}
}

func TestLatLonBound(t *testing.T) {
	if got := NewLatLonBound(); got != nil {
		t.Fatalf("NewLatLonBound() was %v, want nil", got)
	}

	bound := NewLatLonBound(NewLatLon(3, 4), NewLatLon(1, 8), NewLatLon(5, 2))
	if got, want := bound.String(), "[[1,2],[5,8]]"; got != want {
		t.Fatalf("String() was %q, want %q", got, want)
	}
	if bound.IsEmpty() || bound.IsPoint() {
		t.Fatalf("unexpected bound state: empty=%v point=%v", bound.IsEmpty(), bound.IsPoint())
	}
	if !bound.Contains(NewLatLon(3, 4)) || !bound.ContainsLatLon(1, 2) {
		t.Fatal("bound did not contain an interior or edge point")
	}
	for _, point := range []*LatLon{NewLatLon(0, 4), NewLatLon(6, 4), NewLatLon(3, 1), NewLatLon(3, 9)} {
		if bound.Contains(point) {
			t.Fatalf("bound contained outside point %v", point)
		}
	}
	if got, want := bound.Center(), NewLatLon(3, 5); !reflect.DeepEqual(got, want) {
		t.Fatalf("Center() was %v, want %v", got, want)
	}
	if bound.Top() != 5 || bound.Bottom() != 1 || bound.Left() != 2 || bound.Right() != 8 {
		t.Fatalf("unexpected edges: top=%v bottom=%v left=%v right=%v", bound.Top(), bound.Bottom(), bound.Left(), bound.Right())
	}
	if got, want := bound.LeftTop(), NewLatLon(5, 2); !reflect.DeepEqual(got, want) {
		t.Fatalf("LeftTop() was %v, want %v", got, want)
	}
	if got, want := bound.RightBottom(), NewLatLon(1, 8); !reflect.DeepEqual(got, want) {
		t.Fatalf("RightBottom() was %v, want %v", got, want)
	}

	if got := bound.Extend(NewLatLon(2, 3)); got != bound {
		t.Fatal("extending with an interior point returned a new bound")
	}
	extended := bound.ExtendLatLon(-1, 10)
	if got, want := extended.String(), "[[-1,2],[5,10]]"; got != want {
		t.Fatalf("extended bound was %s, want %s", got, want)
	}
	empty := &LatLonBound{Min: NewLatLon(1, 1), Max: NewLatLon(0, 0)}
	if !empty.IsEmpty() {
		t.Fatal("inverted bound was not empty")
	}
	if got := extended.Union(empty); got != extended {
		t.Fatal("union with an empty bound changed the receiver")
	}
	union := bound.Union(NewLatLonBound(NewLatLon(-2, -3), NewLatLon(7, 9)))
	if got, want := union.String(), "[[-2,-3],[7,9]]"; got != want {
		t.Fatalf("union was %s, want %s", got, want)
	}
	point := NewLatLonBound(NewLatLon(1, 1))
	if !point.IsPoint() {
		t.Fatal("single-coordinate bound was not a point")
	}
	point.Pad(2)
	if got, want := point.String(), "[[-1,-1],[3,3]]"; got != want {
		t.Fatalf("padded bound was %s, want %s", got, want)
	}
}

func TestGeoConstructorsAndJSON(t *testing.T) {
	point := NewLatLon(37.5, 127.0)
	geoPoint := NewGeoPoint(point, `"name":"seoul"`)
	if geoPoint.LatLon() != point || !reflect.DeepEqual(geoPoint.Coordinates(), [][]float64{{37.5, 127.0}}) {
		t.Fatalf("unexpected point coordinates: %v", geoPoint.Coordinates())
	}
	if geoPoint.Properties()["name"] != "seoul" {
		t.Fatalf("unexpected point properties: %v", geoPoint.Properties())
	}
	assertGeoJSON(t, geoPoint, "Point", []any{127.0, 37.5})

	emptyPoint := NewGeoPoint(nil, nil)
	assertGeoJSON(t, emptyPoint, "Point", []any{})
	if _, err := emptyPoint.MarshalJSON(); err != nil {
		t.Fatalf("MarshalJSON() error: %v", err)
	}

	properties := map[string]any{"color": "red"}
	circle := NewGeoCircle(point, 12.5, properties)
	properties["color"] = "blue"
	if circle.Properties()["color"] != "red" || circle.Properties()["radius"] != 12.5 {
		t.Fatalf("unexpected circle properties: %v", circle.Properties())
	}
	if !reflect.DeepEqual(circle.Coordinates(), [][]float64{{37.5, 127.0}}) {
		t.Fatalf("unexpected circle coordinates: %v", circle.Coordinates())
	}
	if got := NewGeoCircle(point, 3, `{"radius":7}`).Properties()["radius"]; got != float64(7) {
		t.Fatalf("explicit radius was %v, want 7", got)
	}
	if got := NewGeoCircle(point, 3, "invalid").Properties()["radius"]; got != float64(3) {
		t.Fatalf("fallback radius was %v, want 3", got)
	}

	points := []*LatLon{NewLatLon(1, 2), NewLatLon(3, 4)}
	multi := NewGeoMultiPoint(points, map[string]any{"size": 2})
	if multi.Type() != "MultiPoint" || !reflect.DeepEqual(multi.Coordinates(), [][]float64{{1, 2}, {3, 4}}) {
		t.Fatalf("unexpected multipoint: type=%s coordinates=%v", multi.Type(), multi.Coordinates())
	}
	multi.Add(NewLatLon(5, 6))
	if len(multi.Coordinates()) != 3 || multi.Properties()["size"] != 2 {
		t.Fatalf("unexpected multipoint after Add(): %v %v", multi.Coordinates(), multi.Properties())
	}
	assertGeoJSON(t, multi, "MultiPoint", []any{[]any{2.0, 1.0}, []any{4.0, 3.0}, []any{6.0, 5.0}})
	if _, err := multi.MarshalJSON(); err != nil {
		t.Fatalf("MarshalJSON() error: %v", err)
	}

	from := NewLatLon(10, 20)
	to := NewLatLon(30, 40)
	line := NewGeoLineStringFunc(from, to, `"weight":3`)
	if line.Type() != "LineString" || line.Properties()["weight"] != float64(3) {
		t.Fatalf("unexpected line: %s %v", line.Type(), line.Properties())
	}
	if got := NewGeoLineString(points, nil).Type(); got != "LineString" {
		t.Fatalf("line type was %q", got)
	}
	if got := NewGeoPolygon(points, nil).Type(); got != "Polygon" {
		t.Fatalf("polygon type was %q", got)
	}
	polygon := NewGeoPolygonFunc(from, to, map[string]any{"fill": true})
	if polygon.Type() != "Polygon" || polygon.Properties()["fill"] != true {
		t.Fatalf("unexpected polygon: %s %v", polygon.Type(), polygon.Properties())
	}
	functionalMulti := NewGeoMultiPointFunc(from, to, `"label":"pair"`)
	if len(functionalMulti.Coordinates()) != 2 || functionalMulti.Properties()["label"] != "pair" {
		t.Fatalf("unexpected functional multipoint: %v %v", functionalMulti.Coordinates(), functionalMulti.Properties())
	}
}

func TestGeoMarkersAndProperties(t *testing.T) {
	point := NewLatLon(1, 2)
	pointMarker := NewGeoPointMarker(point, nil)
	if pointMarker.Marker() != "marker" || pointMarker.LatLon() != point {
		t.Fatalf("unexpected point marker: %s %v", pointMarker.Marker(), pointMarker.LatLon())
	}
	circleMarker := NewGeoCircleMarker(point, 10, nil)
	if circleMarker.Marker() != "circleMarker" || circleMarker.Properties()["radius"] != float64(10) {
		t.Fatalf("unexpected circle marker: %s %v", circleMarker.Marker(), circleMarker.Properties())
	}

	properties, err := NewGeoPropertiesParse(`"name":"neo","visible":true,"count":3`)
	if err != nil {
		t.Fatalf("NewGeoPropertiesParse() error: %v", err)
	}
	properties.Copy(GeoProperties{"color": "red", "icon": "markerIcon", "ratio": 1.5})
	if value, ok := properties.PopString("count"); !ok || value != "3" {
		t.Fatalf("PopString(count) was %q, %v", value, ok)
	}
	if value, ok := properties.PopString("missing"); ok || value != "" {
		t.Fatalf("PopString(missing) was %q, %v", value, ok)
	}
	if value, ok := properties.PopBool("visible"); !ok || !value {
		t.Fatalf("PopBool(visible) was %v, %v", value, ok)
	}
	properties["enabled"] = "true"
	if value, ok := properties.PopBool("enabled"); !ok || !value {
		t.Fatalf("PopBool(enabled) was %v, %v", value, ok)
	}
	properties["invalid_bool"] = "perhaps"
	if value, ok := properties.PopBool("invalid_bool"); ok || value {
		t.Fatalf("PopBool(invalid_bool) was %v, %v", value, ok)
	}
	if value, ok := properties.PopBool("missing"); ok || value {
		t.Fatalf("PopBool(missing) was %v, %v", value, ok)
	}

	js, err := properties.MarshalJS()
	if err != nil {
		t.Fatalf("MarshalJS() error: %v", err)
	}
	if got, want := js, `{color:"red",icon:markerIcon,name:"neo",ratio:1.5}`; got != want {
		t.Fatalf("MarshalJS() was %s, want %s", got, want)
	}
	if _, err := NewGeoPropertiesParse("{"); err == nil {
		t.Fatal("NewGeoPropertiesParse() returned nil for invalid JSON")
	}
}

func assertGeoJSON(t *testing.T, value interface{ MarshalGeoJSON() ([]byte, error) }, wantType string, wantCoordinates any) {
	t.Helper()
	data, err := value.MarshalGeoJSON()
	if err != nil {
		t.Fatalf("MarshalGeoJSON() error: %v", err)
	}
	var document struct {
		Type     string `json:"type"`
		Geometry struct {
			Type        string `json:"type"`
			Coordinates any    `json:"coordinates"`
		} `json:"geometry"`
	}
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatalf("json.Unmarshal() error: %v", err)
	}
	if document.Type != "Feature" || document.Geometry.Type != wantType || !reflect.DeepEqual(document.Geometry.Coordinates, wantCoordinates) {
		t.Fatalf("unexpected GeoJSON: %s", data)
	}
}
