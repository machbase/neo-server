package spatial_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/machbase/neo-server/v8/mods/jsh"
)

type TestCase struct {
	Name       string
	Script     string
	Expect     []string
	ExpectFunc func(t *testing.T, result string)
}

func runTestCase(t *testing.T, tc TestCase) {
	t.Helper()
	ctx := context.TODO()
	w := &bytes.Buffer{}
	j := jsh.NewJsh(ctx,
		jsh.WithNativeModules("@jsh/process", "@jsh/spatial"),
		jsh.WithJshWriter(w),
	)
	err := j.Run(tc.Name, tc.Script, nil)
	if err != nil {
		t.Fatalf("Error running script: %s", err)
	}

	if tc.ExpectFunc != nil {
		tc.ExpectFunc(t, w.String())
		return
	} else {
		lines := bytes.Split(w.Bytes(), []byte{'\n'})
		for i, line := range lines {
			if i >= len(tc.Expect) {
				break
			}
			if !bytes.Equal(line, []byte(tc.Expect[i])) {
				t.Errorf("Expected %q, got %q", tc.Expect[i], line)
			}
		}
		if len(lines) > len(tc.Expect) {
			t.Errorf("Expected %d lines, got %d", len(tc.Expect), len(lines))
		}
	}
}

func TestHaversine(t *testing.T) {
	tests := []TestCase{
		{
			Name: "js-haversine",
			Script: `
				m = require("@jsh/spatial");
				//buenos aires
				lat1 = -34.83333;
				lon1 = -58.5166646;
				//paris
				lat2 = 49.0083899664;
				lon2 = 2.53844117956;
				distance = m.haversine(lat1, lon1, lat2, lon2);
				console.log(distance.toFixed(0));
			`,
			Expect: []string{"11099540", ""},
		},
		{
			Name: "js-haversine-latlon",
			Script: `
				m = require("@jsh/spatial");
				//buenos aires
				coord1 = [-34.83333, -58.5166646];
				//paris
				coord2 = [49.0083899664, 2.53844117956];
				distance = m.haversine(coord1, coord2);
				console.log(distance.toFixed(0));
			`,
			Expect: []string{"11099540", ""},
		},
		{
			Name: "js-haversine-coordinates-1",
			Script: `
				m = require("@jsh/spatial");
				//buenos aires
				coord1 = [-34.83333, -58.5166646];
				//paris
				coord2 = [49.0083899664, 2.53844117956];
				distance = m.haversine({radius: 6371, coordinates: [coord1, coord2]});
				console.log(distance.toFixed(0));
			`,
			Expect: []string{"11100", ""},
		},
		{
			Name: "js-haversine-cities",
			Script: `
				m = require("@jsh/spatial");
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
						let opt = {radius: 6371, coordinates: [t[0].coord, t[1].coord]};
						distance = m.haversine(opt);
						console.log(t[0].city, "-", t[1].city, "=>", distance.toFixed(3));
					}catch(e) {
						console.log("Error", e);
					}
				}
			`,
			Expect: []string{
				"Rio de Janeiro - Bangkok => 6094.544",
				"Port Louis, Mauritius - Padang, Indoesia => 5145.526",
				"Oxford, United Kingdom - Vatican, City Vatican City => 1389.179",
				"Windhoek, Namibia - Rotterdam, Netherlands => 3429.893",
				"Esperanza, Argentina - Luanda, Angola => 6996.186",
				"North/South Pole - Paris, France => 4613.478",
				"Turin, Italy - Kuala Lumpur, Malaysia => 10078.112",
				"",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			runTestCase(t, tc)
		})
	}
}

func TestSimplify(t *testing.T) {
	tests := []TestCase{
		{
			Name: "js-simplify",
			Script: `
				m = require("@jsh/spatial");
				points = [[0, 0], [1, 2], [2, 7], [3, 1], [4, 8], [5, 2], [6, 8], [7, 3], [8, 3], [9, 0]];
				console.log(m.simplify(0, ...points));
				console.log(m.simplify(2, ...points));
				console.log(m.simplify(100, ...points));
			`,
			Expect: []string{
				"[[0, 0], [1, 2], [2, 7], [3, 1], [4, 8], [5, 2], [6, 8], [7, 3], [8, 3], [9, 0]]",
				"[[0, 0], [2, 7], [3, 1], [4, 8], [5, 2], [6, 8], [9, 0]]",
				"[[0, 0], [9, 0]]",
				"",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			runTestCase(t, tc)
		})
	}
}
