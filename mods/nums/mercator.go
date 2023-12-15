package nums

import "math"

// mercator.go contains conversion tools for the Spherical Mercator coordinate system
// See http://www.maptiler.org/google-maps-coordinates-tile-bounds-projection/

const (
	TileSize          = 256.0
	initialResolution = 2 * math.Pi * 6378137 / TileSize
	originShift       = 2 * math.Pi * 6378137 / 2
)

func round(a float64) float64 {
	if a < 0 {
		return math.Ceil(a - 0.5)
	}
	return math.Floor(a + 0.5)
}

// Resolution calculates the resolution (meters/pixel) for given zoom level (measured at Equator)
func Resolution(zoom int) float64 {
	return initialResolution / math.Pow(2, float64(zoom))
}

// Zoom gives the zoom level for given resolution (measured at Equator)
func Zoom(resolution float64) int {
	zoom := round(math.Log(initialResolution/resolution) / math.Log(2))
	return int(zoom)
}

// LatLonToMeters converts given lat/lon in WGS84 Datum to XY in Spherical Mercator EPSG:900913
func LatLonToMeters(lat, lon float64) (float64, float64) {
	x := lon * originShift / 180
	y := math.Log(math.Tan((90+lat)*math.Pi/360)) / (math.Pi / 180)
	y = y * originShift / 180
	return x, y
}

// MetersToLatLon converts XY point from Spherical Mercator EPSG:900913 to lat/lon in WGS84 Datum
func MetersToLatLon(x, y float64) (float64, float64) {
	lon := (x / originShift) * 180
	lat := (y / originShift) * 180
	lat = 180 / math.Pi * (2*math.Atan(math.Exp(lat*math.Pi/180)) - math.Pi/2)
	return lat, lon
}

// PixelsToMeters converts pixel coordinates in given zoom level of pyramid to EPSG:900913
func PixelsToMeters(px, py float64, zoom int) (float64, float64) {
	res := Resolution(zoom)
	x := px*res - originShift
	y := py*res - originShift
	return x, y
}

// MetersToPixels converts EPSG:900913 to pixel coordinates in given zoom level
func MetersToPixels(x, y float64, zoom int) (float64, float64) {
	res := Resolution(zoom)
	px := (x + originShift) / res
	py := (y + originShift) / res
	return px, py
}

// LatLonToPixels converts given lat/lon in WGS84 Datum to pixel coordinates in given zoom level
func LatLonToPixels(lat, lon float64, zoom int) (float64, float64) {
	x, y := LatLonToMeters(lat, lon)
	return MetersToPixels(x, y, zoom)
}

// PixelsToLatLon converts pixel coordinates in given zoom level to lat/lon in WGS84 Datum
func PixelsToLatLon(px, py float64, zoom int) (float64, float64) {
	x, y := PixelsToMeters(px, py, zoom)
	return MetersToLatLon(x, y)
}

// PixelsToTile returns a tile covering region in given pixel coordinates
func PixelsToTile(px, py float64) (int, int) {
	tileX := int(math.Floor(px / TileSize))
	tileY := int(math.Floor(py / TileSize))
	return tileX, tileY
}

// MetersToTile returns tile for given mercator coordinates
func MetersToTile(x, y float64, zoom int) (int, int) {
	px, py := MetersToPixels(x, y, zoom)
	return PixelsToTile(px, py)
}

// LatLonToTile returns tile for given lat/lon coordinates
func LatLonToTile(lat, lon float64, zoom int) (int, int) {
	px, py := LatLonToPixels(lat, lon, zoom)
	return PixelsToTile(px, py)
}
