package nums

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

const (
	// EarthRadius is the radius of the earth in meters.
	// To keep things consistent, this value matches WGS84 Web Mercator (EPSG:3867).
	EarthRadius = 6378137.0 // meters
)

type LatLon struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

func NewLatLon(lat, lon float64) *LatLon {
	return &LatLon{Lat: lat, Lon: lon}
}

func (ll *LatLon) String() string {
	return fmt.Sprintf("[%v,%v]", ll.Lat, ll.Lon)
}

func (ll *LatLon) MarshalJSON() ([]byte, error) {
	return json.Marshal([]float64{ll.Lat, ll.Lon})
}

func (ll *LatLon) Array() []float64 {
	return []float64{ll.Lat, ll.Lon}
}

// Distance calculates the shortest path in meters between two coordinates on the surface
// of the Earth (harversine).
func Distance(p, q LatLon) float64 {
	lat1 := DegreesToRadians(p.Lat)
	lon1 := DegreesToRadians(p.Lon)
	lat2 := DegreesToRadians(q.Lat)
	lon2 := DegreesToRadians(q.Lon)
	diffLat := lat2 - lat1
	diffLon := lon2 - lon1
	a := math.Pow(math.Sin(diffLat/2), 2) + math.Cos(lat1)*math.Cos(lat2)*math.Pow(math.Sin(diffLon/2), 2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return c * EarthRadius
}

// Distance returns the shortest path to the other point in meters.
func (ll *LatLon) Distance(pt *LatLon) float64 {
	return Distance(*ll, *pt)
}

type Circle struct {
	center     *LatLon
	radius     float64
	properties GeoProperties
}

type GeoCircle = *Circle

func NewGeoCircle(center *LatLon, radius float64, opt any) GeoCircle {
	ret := &Circle{center: center, radius: radius}
	switch v := opt.(type) {
	case string:
		if prop, err := NewGeoPropertiesParse(v); err == nil {
			ret.properties = prop
		}
	case map[string]any:
		ret.properties = map[string]any{}
		ret.properties.Copy(v)
	}
	if ret.properties == nil {
		ret.properties = map[string]any{}
	}
	if _, hasRadius := ret.properties["radius"]; !hasRadius {
		ret.properties["radius"] = radius
	}
	return ret
}

func (cr *Circle) Coordinates() [][]float64 {
	return [][]float64{{cr.center.Lat, cr.center.Lon}}
}

func (sp *Circle) Properties() GeoProperties {
	return sp.properties
}

type SingleLatLon struct {
	typ        string
	point      *LatLon
	properties GeoProperties
}

func (sp *SingleLatLon) Coordinates() [][]float64 {
	return [][]float64{{sp.point.Lat, sp.point.Lon}}
}

func (sp *SingleLatLon) Properties() GeoProperties {
	return sp.properties
}

func (sp *SingleLatLon) MarshalGeoJSON() ([]byte, error) {
	coord := []float64{}
	if sp.point != nil {
		coord = []float64{sp.point.Lon, sp.point.Lat}
	}
	obj := map[string]any{
		"type": "Feature",
		"geometry": map[string]any{
			"type":        sp.typ,
			"coordinates": coord,
		},
	}
	if sp.properties != nil {
		obj["properties"] = sp.properties
	}
	return json.Marshal(obj)
}

type MultiLatLon struct {
	typ        string
	points     []*LatLon
	properties GeoProperties
}

func NewMultiLatLon(typ string, pts []*LatLon, opt any) *MultiLatLon {
	ret := &MultiLatLon{typ: typ}

	ret.points = append(ret.points, pts...)
	switch v := opt.(type) {
	case string:
		if prop, err := NewGeoPropertiesParse(v); err == nil {
			ret.properties = prop
		}
	case map[string]any:
		ret.properties = map[string]any{}
		ret.properties.Copy(v)
	}
	if ret.properties == nil {
		ret.properties = map[string]any{}
	}
	return ret
}

func NewMultiLatLonFunc(typ string, args ...any) *MultiLatLon {
	var pts []*LatLon
	var opt any

	for _, arg := range args {
		switch v := arg.(type) {
		case *LatLon:
			pts = append(pts, v)
		case map[string]any:
			opt = v
		case string:
			opt = v
		}
	}
	return NewMultiLatLon(typ, pts, opt)
}

func (mp *MultiLatLon) Type() string {
	return mp.typ
}

func (mp *MultiLatLon) Add(pt *LatLon) *MultiLatLon {
	mp.points = append(mp.points, pt)
	return mp
}

func (mp *MultiLatLon) Coordinates() [][]float64 {
	ret := [][]float64{}
	for _, p := range mp.points {
		ret = append(ret, []float64{p.Lat, p.Lon})
	}
	return ret
}

func (mp *MultiLatLon) Properties() GeoProperties {
	return mp.properties
}

func (mp *MultiLatLon) MarshalGeoJSON() ([]byte, error) {
	coord := [][]float64{}
	for _, pt := range mp.points {
		coord = append(coord, []float64{pt.Lon, pt.Lat})
	}
	obj := map[string]any{
		"type": "Feature",
		"geometry": map[string]any{
			"type":        mp.typ,
			"coordinates": coord,
		},
	}
	if mp.properties != nil {
		obj["properties"] = mp.properties
	}
	return json.Marshal(obj)
}

type GeoPoint = *SingleLatLon

func NewGeoPoint(ll *LatLon, opt any) GeoPoint {
	ret := &SingleLatLon{typ: "Point", point: ll}
	switch v := opt.(type) {
	case string:
		if prop, err := NewGeoPropertiesParse(v); err == nil {
			ret.properties = prop
		}
	case map[string]any:
		ret.properties = map[string]any{}
		ret.properties.Copy(v)
	}
	if ret.properties == nil {
		ret.properties = map[string]any{}
	}
	return ret
}

type GeoMultiPoint = *MultiLatLon

func NewGeoMultiPoint(pts []*LatLon, opt any) GeoMultiPoint {
	return NewMultiLatLon("MultiPoint", pts, opt)
}

func NewGeoMultiPointFunc(args ...any) GeoMultiPoint {
	return NewMultiLatLonFunc("MultiPoint", args...)
}

type GeoLineString = *MultiLatLon

func NewGeoLineString(pts []*LatLon, opt any) GeoLineString {
	return NewMultiLatLon("LineString", pts, opt)
}

func NewGeoLineStringFunc(args ...any) GeoLineString {
	return NewMultiLatLonFunc("LineString", args)
}

type GeoPolygon = *MultiLatLon

func NewGeoPolygon(pts []*LatLon, opt any) GeoPolygon {
	return NewMultiLatLon("Polygon", pts, opt)
}

func NewGeoPolygonFunc(args ...any) GeoPolygon {
	return NewMultiLatLonFunc("Polygon", args...)
}

type Geography interface {
	Properties() GeoProperties
	Coordinates() [][]float64
}

var (
	_ Geography = &SingleLatLon{}
	_ Geography = &Circle{}
	_ Geography = &MultiLatLon{}
	_ Geography = GeoPoint(&SingleLatLon{})
	_ Geography = GeoCircle(&Circle{})
	_ Geography = GeoMultiPoint(&MultiLatLon{})
	_ Geography = GeoLineString(&MultiLatLon{})
	_ Geography = GeoPolygon(&MultiLatLon{})
)

type GeoMarker interface {
	Marker() string
	Properties() GeoProperties
	Coordinates() [][]float64
}

var (
	_ GeoMarker = &GeoPointMarker{}
	_ GeoMarker = &GeoCircleMarker{}
)

type GeoPointMarker struct {
	GeoPoint
}

func NewGeoPointMarker(ll *LatLon, opt any) GeoPointMarker {
	return GeoPointMarker{NewGeoPoint(ll, opt)}
}

func (gm GeoPointMarker) Marker() string {
	return "marker"
}

type GeoCircleMarker struct {
	GeoCircle
}

func NewGeoCircleMarker(center *LatLon, radius float64, opt any) GeoCircleMarker {
	return GeoCircleMarker{GeoCircle: NewGeoCircle(center, radius, opt)}
}

func (cm GeoCircleMarker) Marker() string {
	return "circleMarker"
}

type GeoProperties map[string]any

func NewGeoPropertiesParse(opt string) (GeoProperties, error) {
	if !strings.HasPrefix(strings.TrimSpace(opt), "{") {
		opt = "{" + opt + "}"
	}
	ret := GeoProperties{}
	err := json.Unmarshal([]byte(opt), &ret)
	return ret, err
}

func (gp GeoProperties) Copy(other GeoProperties) {
	for k, v := range other {
		gp[k] = v
	}
}

func (gp GeoProperties) PopString(name string) (string, bool) {
	if v, ok := gp[name]; ok {
		delete(gp, name)
		if str, ok := v.(string); ok {
			return str, true
		} else {
			return fmt.Sprintf("%v", v), true
		}
	}
	return "", false
}

func (gp GeoProperties) PopBool(name string) (bool, bool) {
	if v, ok := gp[name]; ok {
		delete(gp, name)
		if b, ok := v.(bool); ok {
			return b, true
		} else if str, ok := v.(string); ok {
			if b, err := strconv.ParseBool(str); err == nil {
				return b, true
			}
		}
	}
	return false, false
}

func (gp GeoProperties) MarshalJS() (string, error) {
	keys := []string{}
	for k := range gp {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return keys[i] < keys[j]
	})
	fields := []string{}
	for _, k := range keys {
		vv := gp[k]
		var line string
		if k == "icon" {
			line = fmt.Sprintf("%s:%v", k, vv)
		} else {
			switch v := vv.(type) {
			case int:
				line = fmt.Sprintf("%s:%d", k, v)
			case float64:
				line = fmt.Sprintf("%s:%v", k, v)
			case bool:
				line = fmt.Sprintf("%s:%t", k, v)
			default:
				line = fmt.Sprintf("%s:%q", k, v)
			}
		}
		fields = append(fields, line)
	}
	return "{" + strings.Join(fields, ",") + "}", nil
}
