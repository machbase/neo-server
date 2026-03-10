package spatial_test

import (
	"testing"

	"github.com/machbase/neo-server/v8/jsh/test_engine"
)

func TestHaversine(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "js-haversine",
			Script: `
				m = require("mathx/spatial");
				//buenos aires
				lat1 = -34.83333;
				lon1 = -58.5166646;
				//paris
				lat2 = 49.0083899664;
				lon2 = 2.53844117956;
				distance = m.haversine([lat1, lon1], [lat2, lon2]);
				console.println(distance.toFixed(0));
			`,
			Output: []string{"11099540"},
		},
		{
			Name: "js-haversine-latlon",
			Script: `
				m = require("mathx/spatial");
				//buenos aires
				coord1 = [-34.83333, -58.5166646];
				//paris
				coord2 = [49.0083899664, 2.53844117956];
				distance = m.haversine(coord1, coord2);
				console.println(distance.toFixed(0));
			`,
			Output: []string{"11099540"},
		},
		{
			Name: "js-haversine-coordinates-1",
			Script: `
				m = require("mathx/spatial");
				//buenos aires
				coord1 = [-34.83333, -58.5166646];
				//paris
				coord2 = [49.0083899664, 2.53844117956];
				distance = m.haversine(coord1, coord2, radius=6371);
				console.println(distance.toFixed(0));
			`,
			Output: []string{"11100"},
		},
		{
			Name: "js-haversine-cities",
			Script: `
				m = require("mathx/spatial");
				l = [
					[   {city: "Rio de Janeiro", coord: [22.55, 43.12]},
						{city: "Bangkok", coord: [13.45, 100.28]},
					],[	
						{city: "Port Louis, Mauritius", coord: [20.10, 57.30]},
						{city: "Padang, Indoesia", coord: [0.57, 100.21]},
					],[
						{city: "Oxford, United Kingdom", coord: [51.45, 1.15]},
						{city: "Vatican, City Vatican City", coord: [41.54, 12.27]},
					],[
						{city: "Windhoek, Namibia", coord: [22.34, 17.05]},
						{city: "Rotterdam, Netherlands", coord: [51.56, 4.29]},
					],[
						{city: "Esperanza, Argentina", coord: [63.24, 56.59]},
						{city: "Luanda, Angola", coord: [8.50, 13.14]},
					],[
						{city: "North/South Pole", coord:[90.0, 0.0]},
						{city: "Paris, France", coord:[48.51, 2.21]},
					],[
						{city: "Turin, Italy", coord: [45.04, 7.42]},
						{city: "Kuala Lumpur, Malaysia", coord: [3.09, 101.42]},
					]
				];
				for( t of l ) {
					try{
						distance = m.haversine(t[0].coord, t[1].coord, 6371);
						console.println(t[0].city, "-", t[1].city, "=>", distance.toFixed(3));
					}catch(e) {
						console.println("Error", e);
					}
				}
			`,
			Output: []string{
				"Rio de Janeiro - Bangkok => 6094.544",
				"Port Louis, Mauritius - Padang, Indoesia => 5145.526",
				"Oxford, United Kingdom - Vatican, City Vatican City => 1389.179",
				"Windhoek, Namibia - Rotterdam, Netherlands => 3429.893",
				"Esperanza, Argentina - Luanda, Angola => 6996.186",
				"North/South Pole - Paris, France => 4613.478",
				"Turin, Italy - Kuala Lumpur, Malaysia => 10078.112",
			},
		},
	}

	for _, tc := range tests {
		test_engine.RunTest(t, tc)
	}
}

func TestSimplify(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "js-simplify",
			Script: `
				m = require("mathx/spatial");
				points = [[0, 0], [1, 2], [2, 7], [3, 1], [4, 8], [5, 2], [6, 8], [7, 3], [8, 3], [9, 0]];
				console.println(m.simplify(0, ...points));
				console.println(m.simplify(2, ...points));
				console.println(m.simplify(100, ...points));
			`,
			Output: []string{
				"[[0, 0], [1, 2], [2, 7], [3, 1], [4, 8], [5, 2], [6, 8], [7, 3], [8, 3], [9, 0]]",
				"[[0, 0], [2, 7], [3, 1], [4, 8], [5, 2], [6, 8], [9, 0]]",
				"[[0, 0], [9, 0]]",
			},
		},
	}

	for _, tc := range tests {
		test_engine.RunTest(t, tc)
	}
}

func TestParseGeoJSON(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "js-parse-geojson",
			Script: `
				m = require("mathx/spatial");
				geojson = {
					type: "FeatureCollection",
					features: [
						{
							type: "Feature",
							properties: { name: "Point A" },
							geometry: {
								type: "Point",
								coordinates: [102.0, 0.5]
							}
						},
						{
							type: "Feature",
							properties: { name: "Line B" },
							geometry: {
								type: "LineString",
								coordinates: [
									[102.0, 0.0],
									[103.0, 1.0],
									[104.0, 0.0],
									[105.0, 1.0]
								]
							}
						},
						{
							type: "Feature",
							properties: { name: "Polygon C" },
							geometry: {
								type: "Polygon",
								coordinates: [
									[
										[100.0, 0.0],
										[101.0, 0.0],
										[101.0, 1.0],
										[100.0, 1.0],
										[100.0, 0.0]
									]
								]
							}
						}
					]
				};
				let features = m.parseGeoJSON(geojson);
				console.println('type:', features.type);
				console.println('features count:', features.features.length);
				console.println('first feature type:', features.features[0].type);
				console.println('first feature geometry:', features.features[0].geometry);
				console.println('first feature name:', features.features[0].properties);
			`,
			Output: []string{
				"type: FeatureCollection",
				"features count: 3",
				"first feature type: Feature",
				"first feature geometry: [102 0.5](orb.Point)",
				"first feature name: map[name:Point A](geojson.Properties)",
			},
		},
	}
	for _, tc := range tests {
		test_engine.RunTest(t, tc)
	}
}
