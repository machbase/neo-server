package nums

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
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

func (ll *LatLng) DistanceTo(pt *LatLng) float64 {
	return 0
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

func NewMultiLatLng(typ string, pts []any, opt any) *MultiLatLng {
	ret := &MultiLatLng{typ: typ}
	for _, p := range pts {
		if v, ok := p.(*LatLng); ok {
			ret.points = append(ret.points, v)
		}
	}
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

func NewGeoMultiPoint(pts []any, opt any) GeoMultiPoint {
	return NewMultiLatLng("MultiPoint", pts, opt)
}

type GeoLineString = *MultiLatLng

func NewGeoLineString(pts []any, opt any) GeoLineString {
	return NewMultiLatLng("LineString", pts, opt)
}

type GeoPolygon = *MultiLatLng

func NewGeoPolygon(pts []any, opt any) GeoPolygon {
	return NewMultiLatLng("Polygon", pts, opt)
}

type Geography interface {
	Properties() GeoProperties
	Coordinates() [][]float64
}

var (
	_ Geography = &SingleLatLng{}
	_ Geography = &Circle{}
	_ Geography = &MultiLatLng{}
	_ Geography = NewGeoPoint(nil, nil)
	_ Geography = GeoCircle(&Circle{})
	_ Geography = NewGeoMultiPoint(nil, nil)
	_ Geography = NewGeoLineString(nil, nil)
	_ Geography = NewGeoPolygon(nil, nil)
)

// Marker: point, circle, icon
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
