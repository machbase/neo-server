package nums

import "math"

func Tile2LatLon(X, Y, Z int) (lat float64, lon float64) {
	n := math.Pi - 2.0*math.Pi*float64(Y)/math.Exp2(float64(Z))
	lat = 180.0 / math.Pi * math.Atan(0.5*(math.Exp(n)-math.Exp(-n)))
	lon = float64(X)/math.Exp2(float64(Z))*360.0 - 180.0
	return lat, lon
}

func LatLon2Tile(lat, lon float64, z int) (x, y int) {
	n := math.Exp2(float64(z))
	x = int(math.Floor((lon + 180.0) / 360.0 * n))
	if float64(x) >= n {
		x = int(n - 1)
	}
	y = int(math.Floor((1.0 - math.Log(math.Tan(lat*math.Pi/180.0)+1.0/math.Cos(lat*math.Pi/180.0))/math.Pi) / 2.0 * n))
	return
}

// calculates the resolution (meters/pixel) for given zoom level
func TileResolution(zoom int) float64 {
	initialResolution := 2 * math.Pi * 6378137 / TileSize
	return initialResolution / math.Pow(2, float64(zoom))
}

// gives the zoom level for given resolution (measured at Equator)
func TileZoom(resolution float64) int {
	var round = func(a float64) float64 {
		if a < 0 {
			return math.Ceil(a - 0.5)
		}
		return math.Floor(a + 0.5)
	}

	initialResolution := 2 * math.Pi * 6378137 / TileSize
	zoom := round(math.Log(initialResolution/resolution) / math.Log(2))
	return int(zoom)
}
