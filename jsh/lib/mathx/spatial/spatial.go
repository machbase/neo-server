package spatial

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"math"

	"github.com/dop251/goja"
	"github.com/machbase/neo-server/v8/mods/nums"
	"github.com/paulmach/orb/geojson"
)

//go:embed spatial.js
var spatial_js []byte

func Files() map[string][]byte {
	return map[string][]byte{
		"mathx/spatial.js": spatial_js,
	}
}

func Module(_ context.Context, r *goja.Runtime, module *goja.Object) {
	o := module.Get("exports").(*goja.Object)
	// m.haversine(lat1, lon1, lat2, lon2)
	// m.haversine([lat1, lon1], [lat2, lon2])
	// m.haversine({radius: 1000, coordinates: [[lat1, lon1], [lat2, lon2]]})
	o.Set("haversine", haversine)
	// m.parseGeoJSON(value)
	o.Set("parseGeoJSON", parseGeoJSON)
	// m.simplify(tolerance, [lat1, lon1], [lat2, lon2], ...)
	o.Set("simplify", simplify)
}

func degreesToRadians(d float64) float64 {
	return d * math.Pi / 180.0
}

func haversine(coord1, coord2 []float64, radius float64) float64 {
	// EarthRadius is the radius of the earth in meters.
	// To keep things consistent, this value matches WGS84 Web Mercator (EPSG:3867).
	EarthRadius := 6371000.0 // meters
	var lat1, lon1, lat2, lon2 float64
	if radius > 0 {
		EarthRadius = radius
	}
	lat1, lon1 = coord1[0], coord1[1]
	lat2, lon2 = coord2[0], coord2[1]

	phi1 := degreesToRadians(lat1)
	phi2 := degreesToRadians(lat2)
	deltaPhi := degreesToRadians(lat2 - lat1)
	deltaLambda := degreesToRadians(lon2 - lon1)

	a := math.Sin(deltaPhi/2)*math.Sin(deltaPhi/2) +
		math.Cos(phi1)*math.Cos(phi2)*
			math.Sin(deltaLambda/2)*math.Sin(deltaLambda/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return c * EarthRadius
}

// Ram-Douglas-Peucker simplify
func simplify(tolerance float64, coords [][]float64) [][]float64 {
	if len(coords) < 3 {
		return coords
	}
	points := make([]nums.Point, len(coords))
	for i, pt := range coords {
		// nums.Point is a [Lng, Lat] 2D point of
		points[i] = nums.Point([2]float64{pt[1], pt[0]})
	}
	simplified := nums.SimplifyPath(points, tolerance)
	ret := make([][]float64, len(simplified))
	for i, p := range simplified {
		ret[i] = []float64{p[1], p[0]}
	}
	return ret
}

func parseGeoJSON(obj map[string]any) (any, error) {
	var typeString string

	if typ, ok := obj["type"]; ok {
		typeString, _ = typ.(string)
	}
	if typeString == "" {
		return nil, errors.New("GeoJSONError missing a GeoJSON type")
	}

	jsonBytes, err := json.Marshal(obj)
	if err != nil {
		return nil, fmt.Errorf("GeoJSONError %s", err.Error())
	}
	var geoObj any
	switch typeString {
	case "FeatureCollection":
		if geo, err := geojson.UnmarshalFeatureCollection(jsonBytes); err == nil {
			geoObj = geo
		} else {
			return nil, fmt.Errorf("GeoJSONError %s", err.Error())
		}
	case "Feature":
		if geo, err := geojson.UnmarshalFeature(jsonBytes); err == nil {
			geoObj = geo
		} else {
			return nil, fmt.Errorf("GeoJSONError %s", err.Error())
		}
	case "Point", "MultiPoint", "LineString", "MultiLineString", "Polygon", "MultiPolygon", "GeometryCollection":
		if geo, err := geojson.UnmarshalGeometry(jsonBytes); err == nil {
			geoObj = geo
		} else {
			return nil, fmt.Errorf("GeoJSONError %s", err.Error())
		}
	default:
		return nil, fmt.Errorf("GeoJSONError %s", "unsupported GeoJSON type")
	}
	return geoObj, nil
}
