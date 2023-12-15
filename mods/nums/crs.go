package nums

import "math"

// Coordinate Reference System (CRS) or Spatial Reference System (SRS)
type CRS struct {
	Code    string         `json:"code"`
	Proj    string         `json:"proj"`
	Options map[string]any `json:"option"`
}

// reference: https://kartena.github.io/Proj4Leaflet/
var KakaoCRS = &CRS{
	Code: "EPSG:5181",
	Proj: `+proj=tmerc +lat_0=38 +lon_0=127 +k=1 +x_0=200000 +y_0=500000 +ellps=GRS80 +towgs84=0,0,0,0,0,0,0 +units=m +no_defs`,
	Options: map[string]any{
		"resolutions": []float64{2048, 1024, 512, 256, 128, 64, 32, 16, 8, 4, 2, 1, 0.5, 0.25},
		"origin":      []float64{-30000, -60000},
		"bounds":      [][]float64{{-30000 - math.Pow(2, 19)*4, -60000}, {-30000 + math.Pow(2, 19)*5, -60000 + math.Pow(2, 19)*5}},
	},
}
