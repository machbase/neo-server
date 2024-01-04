package nums

import (
	"github.com/wroge/wgs84"
)

type spheroid struct {
	a, fi float64
}

func (s spheroid) A() float64 {
	return s.a
}
func (s spheroid) Fi() float64 {
	return s.fi
}

// EPSG:5186 (kakao)
// +proj=tmerc +lat_0=38 +lon_0=127 +k=1 +x_0=200000 +y_0=500000 +ellps=GRS80 +units=m +no_defs
func Transformer5186() func(a, b, c float64) (a2, b2, c2 float64) {
	// SPHEROID["GRS 1980",6378137,298.257222101,
	epsg5186 := wgs84.Datum{
		Spheroid: spheroid{
			a: 6378137, fi: 298.257222101,
		},
		Area: wgs84.AreaFunc(func(lon, lat float64) bool {
			if lon < 122.71 || lat < 28.6 || lon > 134.28 || lat > 40.27 {
				return false
			}
			return true
		}),
	}
	proj := epsg5186.TransverseMercator(127, 38, 1, 200000, 600000)
	epsg := wgs84.EPSG()
	epsg.Add(5186, proj)
	return wgs84.Transform(epsg.Code(5186), wgs84.WGS84().LonLat())
}

// EPSG:5179 (naver, UTM-K)
// +proj=tmerc +lat_0=38 +lon_0=127.5 +k=0.9996 +x_0=1000000 +y_0=2000000 +ellps=GRS80 +units=m +no_defs
func Transformer5179() func(a, b, c float64) (a2, b2, c2 float64) {
	// SPHEROID["GRS 1980",6378137,298.257222101
	epsg5179 := wgs84.Datum{
		Spheroid: spheroid{
			a: 6378137, fi: 298.257222101,
		},
		Area: wgs84.AreaFunc(func(lon, lat float64) bool {
			if lon < 122.71 || lat < 28.6 || lon > 134.28 || lat > 40.27 {
				return false
			}
			return true
		}),
	}
	proj := epsg5179.TransverseMercator(127.5, 38, 0.9996, 1000000, 2000000)
	epsg := wgs84.EPSG()
	epsg.Add(5179, proj)
	return wgs84.Transform(epsg.Code(5179), wgs84.WGS84().LonLat())
}
