package nums

import (
	"encoding/json"
	"fmt"
	"math"
)

const (
	// EarthRadius is the radius of the earth in meters.
	// To keep things consistent, this value matches WGS84 Web Mercator (EPSG:3867).
	EarthRadius = 6378137.0 // meters
)

type LatLng struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
	Alt float64 `json:"alt,omitempty"`
}

func NewLatLng(lat, lng float64) *LatLng {
	return &LatLng{Lat: lat, Lng: lng}
}

func (ll *LatLng) String() string {
	return fmt.Sprintf("[%v,%v]", ll.Lat, ll.Lng)
}

func (ll *LatLng) MarshalJSON() ([]byte, error) {
	return json.Marshal([]float64{ll.Lat, ll.Lng})
}

func (ll *LatLng) Array() []float64 {
	return []float64{ll.Lat, ll.Lng}
}

// degreesToRadians converts from degrees to radians.
func degreesToRadians(d float64) float64 {
	return d * math.Pi / 180
}

// Distance calculates the shortest path in meters between two coordinates on the surface
// of the Earth (harversine).
func Distance(p, q LatLng) float64 {
	lat1 := degreesToRadians(p.Lat)
	lon1 := degreesToRadians(p.Lng)
	lat2 := degreesToRadians(q.Lat)
	lon2 := degreesToRadians(q.Lng)
	diffLat := lat2 - lat1
	diffLon := lon2 - lon1
	a := math.Pow(math.Sin(diffLat/2), 2) + math.Cos(lat1)*math.Cos(lat2)*math.Pow(math.Sin(diffLon/2), 2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return c * EarthRadius
}

// Distance returns the shortest path to the other point in meters.
func (ll *LatLng) Distance(pt *LatLng) float64 {
	return Distance(*ll, *pt)
}

type Circle struct {
	center     *LatLng
	radius     float64
	properties GeoProperties
}

type GeoCircle = *Circle

func NewGeoCircle(center *LatLng, radius float64, opt any) GeoCircle {
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
	return [][]float64{{cr.center.Lat, cr.center.Lng}}
}

func (sp *Circle) Properties() GeoProperties {
	return sp.properties
}

type SingleLatLng struct {
	typ        string
	point      *LatLng
	properties GeoProperties
}

func (sp *SingleLatLng) Coordinates() [][]float64 {
	return [][]float64{{sp.point.Lat, sp.point.Lng}}
}

func (sp *SingleLatLng) Properties() GeoProperties {
	return sp.properties
}

func (sp *SingleLatLng) MarshalGeoJSON() ([]byte, error) {
	coord := []float64{}
	if sp.point != nil {
		coord = []float64{sp.point.Lng, sp.point.Lat}
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

type MultiLatLng struct {
	typ        string
	points     []*LatLng
	properties GeoProperties
}

func NewMultiLatLng(typ string, pts []*LatLng, opt any) *MultiLatLng {
	ret := &MultiLatLng{typ: typ}

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

func NewMultiLatLngFunc(typ string, args ...any) *MultiLatLng {
	var pts []*LatLng
	var opt any

	for _, arg := range args {
		switch v := arg.(type) {
		case *LatLng:
			pts = append(pts, v)
		case map[string]any:
			opt = v
		case string:
			opt = v
		}
	}
	return NewMultiLatLng(typ, pts, opt)
}

func (mp *MultiLatLng) Type() string {
	return mp.typ
}

func (mp *MultiLatLng) Add(pt *LatLng) *MultiLatLng {
	mp.points = append(mp.points, pt)
	return mp
}

func (mp *MultiLatLng) Coordinates() [][]float64 {
	ret := [][]float64{}
	for _, p := range mp.points {
		ret = append(ret, []float64{p.Lat, p.Lng})
	}
	return ret
}

func (mp *MultiLatLng) Properties() GeoProperties {
	return mp.properties
}

func (mp *MultiLatLng) MarshalGeoJSON() ([]byte, error) {
	coord := [][]float64{}
	for _, pt := range mp.points {
		coord = append(coord, []float64{pt.Lng, pt.Lat})
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

type GeoPoint = *SingleLatLng

func NewGeoPoint(ll *LatLng, opt any) GeoPoint {
	ret := &SingleLatLng{typ: "Point", point: ll}
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

type GeoMultiPoint = *MultiLatLng

func NewGeoMultiPoint(pts []*LatLng, opt any) GeoMultiPoint {
	return NewMultiLatLng("MultiPoint", pts, opt)
}

func NewGeoMultiPointFunc(args ...any) GeoMultiPoint {
	return NewMultiLatLngFunc("MultiPoint", args...)
}

type GeoLineString = *MultiLatLng

func NewGeoLineString(pts []*LatLng, opt any) GeoLineString {
	return NewMultiLatLng("LineString", pts, opt)
}

func NewGeoLineStringFunc(args ...any) GeoLineString {
	return NewMultiLatLngFunc("LineString", args)
}

type GeoPolygon = *MultiLatLng

func NewGeoPolygon(pts []*LatLng, opt any) GeoPolygon {
	return NewMultiLatLng("Polygon", pts, opt)
}

func NewGeoPolygonFunc(args ...any) GeoPolygon {
	return NewMultiLatLngFunc("Polygon", args...)
}

type Geography interface {
	Properties() GeoProperties
	Coordinates() [][]float64
}

var (
	_ Geography = &SingleLatLng{}
	_ Geography = &Circle{}
	_ Geography = &MultiLatLng{}
	_ Geography = GeoPoint(&SingleLatLng{})
	_ Geography = GeoCircle(&Circle{})
	_ Geography = GeoMultiPoint(&MultiLatLng{})
	_ Geography = GeoLineString(&MultiLatLng{})
	_ Geography = GeoPolygon(&MultiLatLng{})
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

func NewGeoPointMarker(ll *LatLng, opt any) GeoPointMarker {
	return GeoPointMarker{NewGeoPoint(ll, opt)}
}

func (gm GeoPointMarker) Marker() string {
	return "marker"
}

type GeoCircleMarker struct {
	GeoCircle
}

func NewGeoCircleMarker(center *LatLng, radius float64, opt any) GeoCircleMarker {
	return GeoCircleMarker{GeoCircle: NewGeoCircle(center, radius, opt)}
}

func (cm GeoCircleMarker) Marker() string {
	return "circleMarker"
}
