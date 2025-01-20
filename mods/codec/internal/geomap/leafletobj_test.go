package geomap

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMarshalJS(t *testing.T) {
	tests := []struct {
		name   string
		input  any
		expect string
	}{
		{
			name:   "null",
			input:  nil,
			expect: "null",
		},
		{
			name:   "empty",
			input:  map[string]any{},
			expect: "{}",
		},
		{
			name: "marker",
			input: map[string]any{
				"type":        "marker",
				"coordinates": []float64{12.34, 56.78},
				"properties":  map[string]any{},
			},
			expect: `{coordinates:[12.34,56.78],properties:{},type:"marker"}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual, err := MarshalJS(tc.input)
			require.NoError(t, err)
			require.Equal(t, tc.expect, actual)
		})
	}
}

func TestNewLayer(t *testing.T) {
	tests := []struct {
		name   string
		input  map[string]any
		expect string
	}{
		{
			name: "marker",
			input: map[string]any{
				"type":   "marker",
				"value":  []float64{12.34, 56.78},
				"option": map[string]any{},
			},
			expect: `L.marker([12.34,56.78],{})`,
		},
		{
			name: "marker-any",
			input: map[string]any{
				"type":   "marker",
				"value":  []any{12.34, 56},
				"option": map[string]any{},
			},
			expect: `L.marker([12.34,56],{})`,
		},
		{
			name: "marker-style",
			input: map[string]any{
				"type":   "marker",
				"value":  []float64{12.34, 56.78},
				"option": map[string]any{"style": map[string]any{"color": "red"}},
			},
			expect: `L.marker([12.34,56.78],{style:{color:"red"}})`,
		},
		{
			name: "circleMarker",
			input: map[string]any{
				"type":  "circleMarker",
				"value": []float64{12.34, 56.78},
				"option": map[string]any{
					"radius": 10,
					"style":  map[string]any{"color": "red"},
				},
			},
			expect: `L.circleMarker([12.34,56.78],{radius:10,style:{color:"red"}})`,
		},
		{
			name: "polyline",
			input: map[string]any{
				"type":  "polyline",
				"value": [][]float64{{45.51, -122.68}, {37.77, -122.43}, {34.04, -118.2}},
			},
			expect: `L.polyline([[45.51,-122.68],[37.77,-122.43],[34.04,-118.2]],{})`,
		},
		{
			name: "polyline-any",
			input: map[string]any{
				"type":  "polyline",
				"value": [][]any{{45.51, -122.68}, {37.77, -122.43}, {34.04, -118}},
			},
			expect: `L.polyline([[45.51,-122.68],[37.77,-122.43],[34.04,-118]],{})`,
		},
		{
			name: "geojson-feature-collection",
			input: map[string]any{
				"type": "FeatureCollection",
				"features": []map[string]any{
					{
						"type": "Feature",
						"geometry": map[string]any{
							"type":        "Point",
							"coordinates": []float64{12.34, 56.78},
						},
						"properties": map[string]any{},
					},
				},
			},
			expect: `L.geoJSON({features:[{geometry:{coordinates:[12.34,56.78],type:"Point"},properties:{},type:"Feature"}],type:"FeatureCollection"},opt.geojson)`,
		},
		{
			name: "geojson-feature",
			input: map[string]any{
				"type": "Feature",
				"geometry": map[string]any{
					"type":        "Point",
					"coordinates": []float64{12.34, 56.78},
				},
				"properties": map[string]any{
					"popup": map[string]any{
						"content": "<b>Dinagat Islands</b>",
						"open":    true,
					},
				},
			},
			expect: `L.geoJSON({geometry:{coordinates:[12.34,56.78],type:"Point"},properties:{popup:{content:"<b>Dinagat Islands</b>",open:true}},type:"Feature"},opt.geojson)`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual, err := NewLayer(tc.input)
			require.NoError(t, err)
			require.Equal(t, tc.expect, actual.LeafletJS())
		})
	}
}
