package geomap

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/machbase/neo-server/v8/mods/nums"
	"github.com/paulmach/orb/geojson"
)

type Icon struct {
	Name         string    `json:"name"`
	IconUrl      string    `json:"iconUrl"`
	IconSize     []float64 `json:"iconSize,omitempty"`
	IconAnchor   []float64 `json:"iconAnchor,omitempty"`
	PopupAnchor  []float64 `json:"popupAnchor,omitempty"`
	ShadowUrl    string    `json:"shadowUrl,omitempty"`
	ShadowSize   []float64 `json:"shadowSize,omitempty"`
	ShadowAnchor []float64 `json:"shadowAnchor,omitempty"`
}

type Layer struct {
	Type   string            `json:"type"`
	Value  any               `json:"value"`
	Option map[string]any    `json:"option,omitempty"`
	Bound  *nums.LatLonBound `json:"-"`
}

func (l *Layer) LeafletJS() string {
	if l == nil {
		return "null"
	}
	if js, err := MarshalJS(l.Value); err != nil {
		return fmt.Sprintf("error: %v", err)
	} else {
		if l.Type == "geoJSON" {
			return fmt.Sprintf("L.%s(%s,opt.geojson)", l.Type, js)
		} else {
			props, err := MarshalJS(l.Option)
			if err != nil {
				props = `{error: "` + err.Error() + `"}`
			}
			return fmt.Sprintf("L.%s(%s,%s)", l.Type, js, props)
		}
	}
}

func NewLayer(m map[string]interface{}) (*Layer, error) {
	// Caution!!
	// geojson and orb is [lon,lat] order
	// leaflet is [lat,lon] order
	if m == nil {
		return nil, errors.New("unknown layer")
	}
	typeAny, ok := m["type"]
	if !ok || typeAny == nil {
		return nil, errors.New("unknown layer type")
	}
	typeString, ok := typeAny.(string)
	if !ok {
		return nil, fmt.Errorf("unknown layer type %v", typeAny)
	}
	switch typeString {
	case "marker":
		ret := &Layer{Type: "marker"}
		if coord, ok := m["value"]; ok {
			ret.Value = coord
		} else {
			return nil, errors.New("marker value not found")
		}
		if coord, ok := ret.Value.([]float64); ok && len(coord) == 2 {
			ret.Bound = nums.NewLatLonBound(nums.NewLatLon(coord[0], coord[1]))
		}
		if prop, ok := m["option"]; ok {
			if propMap, ok := prop.(map[string]any); ok {
				ret.Option = propMap
			}
		}
		return ret, nil
	case "circleMarker":
		ret := &Layer{Type: "circleMarker"}
		if coord, ok := m["value"]; ok {
			ret.Value = coord
		} else {
			return nil, errors.New("circleMarker value not found")
		}
		if coord, ok := ret.Value.([]float64); ok && len(coord) == 2 {
			ret.Bound = nums.NewLatLonBound(nums.NewLatLon(coord[0], coord[1]))
		}
		if prop, ok := m["option"]; ok {
			if propMap, ok := prop.(map[string]any); ok {
				ret.Option = propMap
			}
		}
		return ret, nil
	case "FeatureCollection":
		jsonBytes, _ := json.Marshal(m)
		obj, err := geojson.UnmarshalFeatureCollection(jsonBytes)
		if err != nil {
			return nil, fmt.Errorf("invalid geojson %s", err.Error())
		}
		layer := &Layer{Type: "geoJSON", Value: m}
		for _, f := range obj.Features {
			b := f.Geometry.Bound()
			if layer.Bound == nil {
				layer.Bound = nums.NewLatLonBound(
					nums.NewLatLon(b.Min.Lat(), b.Min.Lon()),
					nums.NewLatLon(b.Max.Lat(), b.Max.Lon()),
				)
			} else {
				layer.Bound.ExtendLatLon(b.Min.Lat(), b.Min.Lon())
				layer.Bound.ExtendLatLon(b.Max.Lat(), b.Max.Lon())
			}
		}
		return layer, nil
	case "Feature":
		jsonBytes, _ := json.Marshal(m)
		obj, err := geojson.UnmarshalFeature(jsonBytes)
		if err != nil {
			return nil, fmt.Errorf("invalid geojson %s", err.Error())
		}
		b := obj.Geometry.Bound()
		layer := &Layer{Type: "geoJSON", Value: m}
		layer.Bound = nums.NewLatLonBound(
			nums.NewLatLon(b.Min.Lat(), b.Min.Lon()),
			nums.NewLatLon(b.Max.Lat(), b.Max.Lon()),
		)
		return layer, nil
	case "Point", "MultiPoint", "LineString", "MultiLineString", "Polygon", "MultiPolygon", "GeometryCollection":
		jsonBytes, _ := json.Marshal(m)
		obj, err := geojson.UnmarshalGeometry(jsonBytes)
		if err != nil {
			return nil, fmt.Errorf("invalid geojson %s", err.Error())
		}
		b := obj.Coordinates.Bound()
		layer := &Layer{Type: "geoJSON", Value: m}
		layer.Bound = nums.NewLatLonBound(
			nums.NewLatLon(b.Min.Lat(), b.Min.Lon()),
			nums.NewLatLon(b.Max.Lat(), b.Max.Lon()),
		)
		return layer, nil
	default:
		return nil, fmt.Errorf("unknown layer type %s", typeString)
	}
}

func MarshalJS(value any) (string, error) {
	if value == nil {
		return "null", nil
	}
	switch val := value.(type) {
	case map[string]any:
		keys := []string{}
		for k := range val {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool {
			return keys[i] < keys[j]
		})
		fields := []string{}
		for _, k := range keys {
			v := val[k]
			vv, err := MarshalJS(v)
			if err != nil {
				return "", err
			}
			fields = append(fields, fmt.Sprintf("%s:%s", k, vv))
		}
		return "{" + strings.Join(fields, ",") + "}", nil
	case bool, int, int64, int32, int16, int8, uint, uint64, uint32, uint16, uint8, float64:
		return fmt.Sprintf("%v", val), nil
	case string:
		return fmt.Sprintf(`%q`, val), nil
	case []any:
		fields := []string{}
		for _, elm := range val {
			v, err := MarshalJS(elm)
			if err != nil {
				return "", err
			}
			fields = append(fields, v)
		}
		return "[" + strings.Join(fields, ",") + "]", nil
	case []map[string]any:
		fields := []string{}
		for _, elm := range val {
			v, err := MarshalJS(elm)
			if err != nil {
				return "", err
			}
			fields = append(fields, v)
		}
		return "[" + strings.Join(fields, ",") + "]", nil
	case []float64:
		fields := []string{}
		for _, val := range val {
			fields = append(fields, fmt.Sprintf("%v", val))
		}
		return "[" + strings.Join(fields, ",") + "]", nil
	case [][]float64:
		fields := []string{}
		for _, arr := range val {
			elm, err := MarshalJS(arr)
			if err != nil {
				return "", err
			}
			fields = append(fields, elm)
		}
		return "[" + strings.Join(fields, ",") + "]", nil
	case [][][]float64:
		fields := []string{}
		for _, arr := range val {
			elm, err := MarshalJS(arr)
			if err != nil {
				return "", err
			}
			fields = append(fields, elm)
		}
		return "[" + strings.Join(fields, ",") + "]", nil
	default:
		return fmt.Sprintf("unknown(%T)", val), nil
	}
}
