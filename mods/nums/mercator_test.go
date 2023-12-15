package nums

import (
	"fmt"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func floatEquals(a, b float64) bool {
	return math.Abs(a-b) < 0.00000001
}

func TestResolution(t *testing.T) {
	zoom := 10
	expected := 152.8740565703525
	res := Resolution(zoom)
	if !floatEquals(res, expected) {
		t.Errorf("Resolution(%d) == %f, want %f", zoom, res, expected)
	}
}

func TestZoom(t *testing.T) {
	res := 152.8740565703525
	expected := 10
	zoom := Zoom(res)
	if zoom != expected {
		t.Errorf("Zoom(%f) == %d, want %d", res, zoom, expected)
	}
}

func TestLatLngToMeters(t *testing.T) {
	lat, lng := 62.3, 14.1
	expectedX, expectedY := 1569604.8201851572, 8930630.669201756
	x, y := LatLngToMeters(lat, lng)
	if !floatEquals(x, expectedX) || !floatEquals(y, expectedY) {
		t.Errorf("LatLngToMeters(%f, %f) == %f, %f, want %f, %f", lat, lng, x, y, expectedX, expectedY)
	}
}

func TestMetersToLatLng(t *testing.T) {
	x, y := 1569604.8201851572, 8930630.669201756
	expectedLat, expectedLng := 62.3, 14.1
	lat, lon := MetersToLatLng(x, y)
	if !floatEquals(lat, expectedLat) || !floatEquals(lon, expectedLng) {
		t.Errorf("MetersToLatLng(%f, %f) == %f, %f, want %f, %f", x, y, lat, lon, expectedLat, expectedLng)
	}
}

func TestPixelsToMeters(t *testing.T) {
	px, py, zoom := 123456789.0, 123456789.0, 15
	expectedX, expectedY := 569754371.206588, 569754371.206588
	x, y := PixelsToMeters(px, py, zoom)
	if !floatEquals(x, expectedX) || !floatEquals(y, expectedY) {
		t.Errorf("PixelsToMeters(%d, %d, %d) == %f, %f, want %f, %f", int(px), int(py), zoom, x, y, expectedX, expectedY)
	}
}

func TestMetersToPixels(t *testing.T) {
	x, y, zoom := 569754371.206588, 569754371.206588, 15
	expectedPx, expectedPy := 123456789.0, 123456789.0
	px, py := MetersToPixels(x, y, zoom)
	if !floatEquals(px, expectedPx) || !floatEquals(py, expectedPy) {
		t.Errorf("MetersToPixels(%f, %f, %d) == %d, %d, want %d, %d", x, y, zoom, int(px), int(py), int(expectedPx), int(expectedPy))
	}
}

func TestLatLngToPixels(t *testing.T) {
	lat, lng, zoom := 62.3, 14.1, 15
	expectedPx, expectedPy := 4522857.8133333335, 6063687.123767246
	px, py := LatLngToPixels(lat, lng, zoom)
	if !floatEquals(px, expectedPx) || !floatEquals(py, expectedPy) {
		t.Errorf("LatLngToPixels(%f, %f, %d) == %f, %f, want %f, %f", lat, lng, zoom, px, py, expectedPx, expectedPy)
	}
}

func TestPixelsToLatLon(t *testing.T) {
	px, py, zoom := 4522857.8133333335, 6063687.123767246, 15
	expectedLat, expectedLng := 62.3, 14.1
	lat, lon := PixelsToLatLng(px, py, zoom)
	if !floatEquals(lat, expectedLat) || !floatEquals(lon, expectedLng) {
		t.Errorf("PixelsToLatLng(%f, %f, %d) == %f, %f, want %f, %f", px, py, zoom, lat, lon, expectedLat, expectedLng)
	}
}

func TestPixelsToTile(t *testing.T) {
	px, py := 123456789.0, 123456789.0
	expectedTileX, expectedTileY := 482253, 482253
	tileX, tileY := PixelsToTile(px, py)
	if tileX != expectedTileX || tileY != expectedTileY {
		t.Errorf("PixelsToTile(%f, %f) == %d, %d, want %d, %d", px, py, tileX, tileY, expectedTileX, expectedTileY)
	}
}

func TestMetersToTile(t *testing.T) {
	x, y, zoom := 569754371.206588, 569754371.206588, 15
	expectedTileX, expectedTileY := 482253, 482253
	tileX, tileY := MetersToTile(x, y, zoom)
	if tileX != expectedTileX || tileY != expectedTileY {
		t.Errorf("MetersToTile(%f, %f, %d) == %d, %d, want %d, %d", x, y, zoom, tileX, tileY, expectedTileX, expectedTileY)
	}
}

func TestLatLngToTile(t *testing.T) {
	lat, lon, zoom := 62.3, 14.1, 15
	expectedTileX, expectedTileY := 17667, 23686
	tileX, tileY := LatLngToTile(lat, lon, zoom)
	if tileX != expectedTileX || tileY != expectedTileY {
		t.Errorf("LatLngToTile(%f, %f, %d) == %d, %d, want %d, %d", lat, lon, zoom, tileX, tileY, expectedTileX, expectedTileY)
	}
}

func TestTileToLatLon(t *testing.T) {
	zoom, tileX, tileY := 15, 17667, 23686
	expectedLat, expectedLon := 62.3, 14.1

	maxPixels := TileSize * math.Pow(2, float64(zoom))
	fmt.Printf("maxPixels    : %.f\n", maxPixels)

	maxTiles := math.Round(maxPixels / TileSize)
	fmt.Printf("maxTiles     : %.f\n", maxTiles)

	pixelX := float64(tileX * TileSize)
	pixelY := float64(tileY * TileSize)
	fmt.Printf("pixelX       : %.f\n", pixelX)
	fmt.Printf("pixelY       : %.f\n", pixelY)

	lat, lng := PixelsToLatLng(pixelX, pixelY, zoom)
	fmt.Printf("lat:%f lng:%f\n", lat, lng)
	lat, lng = math.Round(lat*10)/10, math.Round(lng*10)/10
	assert.Equal(t, expectedLat, lat)
	assert.Equal(t, expectedLon, lng)
}
