package spatial

import (
	"bytes"
	"strings"
	"testing"

	"github.com/machbase/neo-server/v8/jsh/engine"
	"github.com/machbase/neo-server/v8/jsh/root"
)

type TestCase struct {
	name   string
	script string
	output []string
	err    string
	vars   map[string]any
}

func RunTest(t *testing.T, tc TestCase) {
	t.Helper()
	t.Run(tc.name, func(t *testing.T) {
		t.Helper()
		conf := engine.Config{
			Name: tc.name,
			Code: tc.script,
			FSTabs: []engine.FSTab{
				root.RootFSTab(),
			},
			Env: map[string]any{
				"PATH": "/sbin:/lib:/usr/bin:/usr/lib:/work",
				"PWD":  "/work",
			},
			Reader: &bytes.Buffer{},
			Writer: &bytes.Buffer{},
		}
		jr, err := engine.New(conf)
		if err != nil {
			t.Fatalf("Failed to create JSRuntime: %v", err)
		}
		jr.RegisterNativeModule("@jsh/process", jr.Process)
		jr.RegisterNativeModule("@jsh/spatial", Module)

		for k, v := range tc.vars {
			jr.Env.Set(k, v)
		}
		if err := jr.Run(); err != nil {
			if tc.err == "" || !strings.Contains(err.Error(), tc.err) {
				t.Fatalf("Unexpected error: %v", err)
			}
			return
		}

		gotOutput := conf.Writer.(*bytes.Buffer).String()
		lines := strings.Split(gotOutput, "\n")
		if len(lines) != len(tc.output)+1 { // +1 for trailing newline
			t.Fatalf("Expected %d output lines, got %d\n%s", len(tc.output), len(lines)-1, gotOutput)
		}
		for i, expectedLine := range tc.output {
			if lines[i] != expectedLine {
				t.Errorf("Output line %d: expected %q, got %q", i, expectedLine, lines[i])
			}
		}
	})
}

func TestHaversine(t *testing.T) {
	tests := []TestCase{
		{
			name: "js-haversine",
			script: `
				m = require("spatial");
				//buenos aires
				lat1 = -34.83333;
				lon1 = -58.5166646;
				//paris
				lat2 = 49.0083899664;
				lon2 = 2.53844117956;
				distance = m.haversine([lat1, lon1], [lat2, lon2]);
				console.println(distance.toFixed(0));
			`,
			output: []string{"11099540"},
		},
		{
			name: "js-haversine-latlon",
			script: `
				m = require("spatial");
				//buenos aires
				coord1 = [-34.83333, -58.5166646];
				//paris
				coord2 = [49.0083899664, 2.53844117956];
				distance = m.haversine(coord1, coord2);
				console.println(distance.toFixed(0));
			`,
			output: []string{"11099540"},
		},
		{
			name: "js-haversine-coordinates-1",
			script: `
				m = require("spatial");
				//buenos aires
				coord1 = [-34.83333, -58.5166646];
				//paris
				coord2 = [49.0083899664, 2.53844117956];
				distance = m.haversine(coord1, coord2, radius=6371);
				console.println(distance.toFixed(0));
			`,
			output: []string{"11100"},
		},
		{
			name: "js-haversine-cities",
			script: `
				m = require("spatial");
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
			output: []string{
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
		t.Run(tc.name, func(t *testing.T) {
			RunTest(t, tc)
		})
	}
}

func TestSimplify(t *testing.T) {
	tests := []TestCase{
		{
			name: "js-simplify",
			script: `
				m = require("spatial");
				points = [[0, 0], [1, 2], [2, 7], [3, 1], [4, 8], [5, 2], [6, 8], [7, 3], [8, 3], [9, 0]];
				console.println(m.simplify(0, ...points));
				console.println(m.simplify(2, ...points));
				console.println(m.simplify(100, ...points));
			`,
			output: []string{
				"[[0, 0], [1, 2], [2, 7], [3, 1], [4, 8], [5, 2], [6, 8], [7, 3], [8, 3], [9, 0]]",
				"[[0, 0], [2, 7], [3, 1], [4, 8], [5, 2], [6, 8], [9, 0]]",
				"[[0, 0], [9, 0]]",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			RunTest(t, tc)
		})
	}
}
