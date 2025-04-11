package geomap

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

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
	Type        string         `json:"type"`
	Coordinates any            `json:"coordinates"`
	Properties  map[string]any `json:"properties,omitempty"`
}

func (l *Layer) LeafletJS() string {
	if l == nil {
		return "null"
	}
	if js, err := MarshalJS(l.Coordinates); err != nil {
		return fmt.Sprintf("error: %v", err)
	} else {
		if l.Type == "geoJSON" {
			return fmt.Sprintf("L.%s(%s,opt.geojson)", l.Type, js)
		} else {
			props, err := MarshalJS(l.Properties)
			if err != nil {
				props = `{error: "` + err.Error() + `"}`
			}
			return fmt.Sprintf("L.%s(%s,%s)", l.Type, js, props)
		}
	}
}

func ConvCoordinates(coord any, extendLatLon func(lat, long float64)) any {
	if coord == nil {
		return nil
	}
	switch value := coord.(type) {
	case [][]any:
		ret := make([]any, len(value))
		for i := range value {
			ret[i] = ConvCoordinates(value[i], extendLatLon)
		}
		return ret
	case []any:
		retAny := make([]any, len(value))
		for i := range value {
			switch val := value[i].(type) {
			case []any:
				sub := make([]any, len(val))
				for j := range val {
					sub[j] = ConvCoordinates(val[j], extendLatLon)
				}
				if len(sub) == 2 {
					if lat, ok := sub[0].(float64); ok {
						if lon, ok := sub[1].(float64); ok {
							if extendLatLon != nil {
								extendLatLon(lat, lon)
							}
						}
					}
				}
				retAny[i] = sub
			default:
				retAny[i] = ConvCoordinates(val, extendLatLon)
			}
		}
		if len(retAny) == 2 {
			if lat, ok := retAny[0].(float64); ok {
				if lon, ok := retAny[1].(float64); ok {
					if extendLatLon != nil {
						extendLatLon(lat, lon)
					}
				}
			}
		}
		return retAny
	case []float64:
		ret := value
		if len(ret) == 2 {
			if extendLatLon != nil {
				extendLatLon(ret[0], ret[1])
			}
		}
		return ret
	case [][]float64:
		if extendLatLon != nil {
			for i := range value {
				if len(value[i]) == 2 {
					extendLatLon(value[i][0], value[i][1])
				}
			}
		}
		return value
	case []int64:
		ret := make([]float64, len(value))
		for i, val := range value {
			ret[i] = float64(val)
		}
		if len(ret) == 2 {
			if extendLatLon != nil {
				extendLatLon(ret[0], ret[1])
			}
		}
		return ret
	case *float64:
		return *value
	case float64:
		return value
	case *int64:
		return float64(*value)
	case int64:
		return float64(value)
	case *int:
		return float64(*value)
	case int:
		return float64(value)
	default:
		fmt.Printf("unknown type value %T %v\n", value, value)
	}
	return nil
}

func NewLayer(m map[string]interface{}, extendLatLon func(float64, float64)) (*Layer, error) {
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
	case "marker", "circleMarker", "circle", "polyline", "polygon":
		// Caution!!
		// leaflet is [lat,lon] order
		layer := &Layer{Type: typeString}
		if coord, ok := m["coordinates"]; ok {
			layer.Coordinates = ConvCoordinates(coord, extendLatLon)
		} else {
			return nil, errors.New("marker value not found")
		}
		if prop, ok := m["properties"]; ok {
			if propMap, ok := prop.(map[string]any); ok {
				layer.Properties = propMap
			}
		}
		return layer, nil
	case "FeatureCollection":
		// Caution!!
		// geojson and orb is [lon,lat] order
		jsonBytes, _ := json.Marshal(m)
		obj, err := geojson.UnmarshalFeatureCollection(jsonBytes)
		if err != nil {
			return nil, fmt.Errorf("invalid geojson %s", err.Error())
		}
		layer := &Layer{Type: "geoJSON", Coordinates: m}
		if extendLatLon != nil {
			for _, f := range obj.Features {
				b := f.Geometry.Bound()
				extendLatLon(b.Min.Lat(), b.Min.Lon())
				extendLatLon(b.Max.Lat(), b.Max.Lon())
			}
		}
		return layer, nil
	case "Feature":
		// Caution!!
		// geojson and orb is [lon,lat] order
		jsonBytes, _ := json.Marshal(m)
		obj, err := geojson.UnmarshalFeature(jsonBytes)
		if err != nil {
			return nil, fmt.Errorf("invalid geojson %s", err.Error())
		}
		layer := &Layer{Type: "geoJSON", Coordinates: m, Properties: obj.Properties}
		if extendLatLon != nil {
			b := obj.Geometry.Bound()
			extendLatLon(b.Min.Lat(), b.Min.Lon())
			extendLatLon(b.Max.Lat(), b.Max.Lon())
		}
		return layer, nil
	case "Point", "MultiPoint", "LineString", "MultiLineString", "Polygon", "MultiPolygon", "GeometryCollection":
		// Caution!!
		// geojson and orb is [lon,lat] order
		jsonBytes, _ := json.Marshal(m)
		obj, err := geojson.UnmarshalGeometry(jsonBytes)
		if err != nil {
			return nil, fmt.Errorf("invalid geojson %s", err.Error())
		}
		layer := &Layer{Type: "geoJSON", Coordinates: m}
		if extendLatLon != nil {
			b := obj.Coordinates.Bound()
			extendLatLon(b.Min.Lat(), b.Min.Lon())
			extendLatLon(b.Max.Lat(), b.Max.Lon())
		}
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
	case [][][][]float64:
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
