package geomap

import "github.com/machbase/neo-server/mods/nums"

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

type Layer struct {
	Name   string             `json:"name"`
	Type   string             `json:"type"`
	Coord  string             `json:"coord"`
	Option nums.GeoProperties `json:"option,omitempty"`
	Popup  *Popup             `json:"popup,omitempty"`
	Style  string             `json:"pointStyle,omitempty"`
}

type Popup struct {
	Content string `json:"content"`
	Open    bool   `json:"open,omitempty"`
}
