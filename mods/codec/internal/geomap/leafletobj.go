package geomap

import "github.com/machbase/neo-server/mods/nums"

type Icon struct {
	Name         string    `json:"-"`
	IconUrl      string    `json:"iconUrl"`
	IconSize     []float64 `json:"iconSize,omitempty"`
	IconAnchor   []float64 `json:"iconAnchor,omitempty"`
	PopupAnchor  []float64 `json:"popupAnchor,omitempty"`
	ShadowUrl    string    `json:"shadowUrl,omitempty"`
	ShadowSize   []float64 `json:"shadowSize,omitempty"`
	ShadowAnchor []float64 `json:"shadowAnchor,omitempty"`
}

type PointStyle struct {
	Name       string             `json:"-"`
	Type       string             `json:"type"`
	Properties nums.GeoProperties `json:"options"`
}

var defaultPointStyle = PointStyle{
	Type: "circleMarker",
	Properties: nums.GeoProperties{
		"radius":      7,
		"stroke":      false,
		"color":       "#ff0000",
		"opacity":     1.0,
		"fillOpacity": 1.0,
	},
}
