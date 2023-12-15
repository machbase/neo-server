package nums

import (
	"fmt"
	"testing"

	"github.com/wroge/wgs84"
)

func TestLatLonTileCoord(t *testing.T) {
	// https://tile.openstreetmap.org/17/111812/50783.png
	zoom, tileX, tileY := 17, 111812, 50783
	fmt.Printf("tileX:%d   tileY:%d  zoom:%d\n", tileX, tileY, zoom)
	fmt.Printf("https://tile.openstreetmap.org/%d/%d/%d.png\n\n", zoom, tileX, tileY)

	// 20553782 / 50784
	lat, lng := Tile2LatLng(tileX, tileY, zoom)
	//east, north := FromPixels(float64(tileX*256), float64(tileY*256), zoom)
	fmt.Printf("lng:%f  lat:%f  %f,%f\n", lng, lat, lat, lng)

	tileX, tileY = LatLng2Tile(lat, lng, zoom)
	fmt.Printf("https://tile.openstreetmap.org/%d/%d/%d.png\n\n", zoom, tileX, tileY)
	// fmt.Printf("pixelX:%.f pixelY:%.f\n", pixelX, pixelY)

	east, north := PixelsToMeters(float64(tileX), float64(tileY), zoom)
	fmt.Printf("east1 :%.f north1:%.f\n", east, north)

	east, north, _ = wgs84.LonLat().To(wgs84.WebMercator())(lng, lat, 0)
	fmt.Printf("east2 :%.f north2:%.f\n", east, north)

	tileX, tileY = MetersToTile(east, north, zoom)
	fmt.Printf("tileX:%d   tileY:%d  zoom:%d\n", tileX, tileY, zoom)
}
