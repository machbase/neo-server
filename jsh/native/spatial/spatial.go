package spatial

import (
	"encoding/json"
	"fmt"
	"math"

	js "github.com/dop251/goja"
	"github.com/machbase/neo-server/v8/mods/nums"
	"github.com/paulmach/orb/geojson"
)

func Module(r *js.Runtime, module *js.Object) {
	m := &M{rt: r}
	// m = require("spatial")
	o := module.Get("exports").(*js.Object)
	// m.haversine(lat1, lon1, lat2, lon2)
	// m.haversine([lat1, lon1], [lat2, lon2])
	// m.haversine({radius: 1000, coordinates: [[lat1, lon1], [lat2, lon2]]})
	o.Set("haversine", m.spatial_haversine)
	// m.parseGeoJSON(value)
	o.Set("parseGeoJSON", m.spatial_parseGeoJSON)
	// m.simplify(tolerance, [lat1, lon1], [lat2, lon2], ...)
	o.Set("simplify", m.spatial_simplify)
}

type M struct {
	rt *js.Runtime
}

func degreesToRadians(d float64) float64 {
	return d * math.Pi / 180.0
}
func (m *M) spatial_haversine(call js.FunctionCall) js.Value {
	// EarthRadius is the radius of the earth in meters.
	// To keep things consistent, this value matches WGS84 Web Mercator (EPSG:3867).
	EarthRadius := 6371000.0 // meters
	var lat1, lon1, lat2, lon2 float64
	var err error
	if len(call.Arguments) == 1 {
		arg := struct {
			Radius float64      `json:"radius"`
			Coord  [][2]float64 `json:"coordinates"`
		}{}
		if err = m.rt.ExportTo(call.Arguments[0], &arg); err != nil {
			panic(m.rt.ToValue(fmt.Sprintf("haversine: invalid arguments %s", err.Error())))
		}
		if len(arg.Coord) != 2 {
			panic(m.rt.ToValue("haversine: invalid arguments"))
		}
		if arg.Radius > 0 {
			EarthRadius = arg.Radius
		}
		lat1, lon1 = arg.Coord[0][0], arg.Coord[0][1]
		lat2, lon2 = arg.Coord[1][0], arg.Coord[1][1]
	} else if len(call.Arguments) == 2 {
		var loc1, loc2 []float64
		if err = m.rt.ExportTo(call.Arguments[0], &loc1); err != nil {
			panic(m.rt.ToValue(fmt.Sprintf("haversine: invalid arguments %s", err.Error())))
		}
		if err = m.rt.ExportTo(call.Arguments[1], &loc2); err != nil {
			panic(m.rt.ToValue(fmt.Sprintf("haversine: invalid arguments %s", err.Error())))
		}
		lat1, lon1 = loc1[0], loc1[1]
		lat2, lon2 = loc2[0], loc2[1]
	} else if len(call.Arguments) == 4 {
		if err = m.rt.ExportTo(call.Arguments[0], &lat1); err != nil {
			panic(m.rt.ToValue(fmt.Sprintf("haversine: invalid arguments %s", err.Error())))
		}
		if err = m.rt.ExportTo(call.Arguments[1], &lon1); err != nil {
			panic(m.rt.ToValue(fmt.Sprintf("haversine: invalid arguments %s", err.Error())))
		}
		if err = m.rt.ExportTo(call.Arguments[2], &lat2); err != nil {
			panic(m.rt.ToValue(fmt.Sprintf("haversine: invalid arguments %s", err.Error())))
		}
		if err = m.rt.ExportTo(call.Arguments[3], &lon2); err != nil {
			panic(m.rt.ToValue(fmt.Sprintf("haversine: invalid arguments %s", err.Error())))
		}
	}

	phi1 := degreesToRadians(lat1)
	phi2 := degreesToRadians(lat2)
	deltaPhi := degreesToRadians(lat2 - lat1)
	deltaLambda := degreesToRadians(lon2 - lon1)

	a := math.Sin(deltaPhi/2)*math.Sin(deltaPhi/2) +
		math.Cos(phi1)*math.Cos(phi2)*
			math.Sin(deltaLambda/2)*math.Sin(deltaLambda/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return m.rt.ToValue(c * EarthRadius)
}

// Ram-Douglas-Peucker simplify
func (m *M) spatial_simplify(call js.FunctionCall) js.Value {
	if len(call.Arguments) < 3 {
		return m.rt.NewGoError(fmt.Errorf("simplify: invalid arguments %v", call.Arguments))
	}
	var tolerance float64
	if err := m.rt.ExportTo(call.Arguments[0], &tolerance); err != nil {
		return m.rt.NewGoError(fmt.Errorf("simplify: invalid arguments %s", err.Error()))
	}
	points := make([]nums.Point, len(call.Arguments)-1)
	for i := 1; i < len(call.Arguments); i++ {
		var pt [2]float64
		if err := m.rt.ExportTo(call.Arguments[i], &pt); err != nil {
			return m.rt.NewGoError(fmt.Errorf("simplify: invalid arguments %s", err.Error()))
		}
		// nums.Point is a [Lng, Lat] 2D point of
		points[i-1] = nums.Point([2]float64{pt[1], pt[0]})
	}
	simplified := nums.SimplifyPath(points, tolerance)
	ret := make([][]float64, len(simplified))
	for i, p := range simplified {
		ret[i] = []float64{p[1], p[0]}
	}
	return m.rt.ToValue(ret)
}

func (m *M) spatial_parseGeoJSON(value js.Value) js.Value {
	obj := value.ToObject(m.rt)
	if obj == nil {
		return m.rt.NewGoError(fmt.Errorf("GeoJSONError requires a GeoJSON object, but got %q", value.ExportType()))
	}
	typeString := obj.Get("type")
	if typeString == nil {
		return m.rt.NewGoError(fmt.Errorf("GeoJSONError missing a GeoJSON type"))
	}
	jsonBytes, err := json.Marshal(obj)
	if err != nil {
		return m.rt.NewGoError(fmt.Errorf("GeoJSONError %s", err.Error()))
	}
	var geoObj any
	switch typeString.String() {
	case "FeatureCollection":
		if geo, err := geojson.UnmarshalFeatureCollection(jsonBytes); err == nil {
			geoObj = geo
		} else {
			return m.rt.NewGoError(fmt.Errorf("GeoJSONError %s", err.Error()))
		}
	case "Feature":
		if geo, err := geojson.UnmarshalFeature(jsonBytes); err == nil {
			geoObj = geo
		} else {
			return m.rt.NewGoError(fmt.Errorf("GeoJSONError %s", err.Error()))
		}
	case "Point", "MultiPoint", "LineString", "MultiLineString", "Polygon", "MultiPolygon", "GeometryCollection":
		if geo, err := geojson.UnmarshalGeometry(jsonBytes); err == nil {
			geoObj = geo
		} else {
			return m.rt.NewGoError(fmt.Errorf("GeoJSONError %s", err.Error()))
		}
	default:
		return m.rt.NewGoError(fmt.Errorf("GeoJSONError %s", "unsupported GeoJSON type"))
	}
	var _ = geoObj
	return obj
}
