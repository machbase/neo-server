package geomap_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/machbase/neo-server/v8/mods/codec/facility"
	"github.com/machbase/neo-server/v8/mods/codec/internal/geomap"
	"github.com/machbase/neo-server/v8/mods/nums"
	"github.com/stretchr/testify/require"
)

type VolatileFileWriterMock struct {
	name     string
	deadline time.Time
	buff     bytes.Buffer
}

func (v *VolatileFileWriterMock) VolatileFilePrefix() string { return "/web/api/tql-assets/" }

func (v *VolatileFileWriterMock) VolatileFileWrite(name string, data []byte, deadline time.Time) {
	v.buff.Write(data)
	v.name = name
	v.deadline = deadline
}

func StringsEq(t *testing.T, expect string, actual string) bool {
	matched := false
	t.Helper()
	re := strings.Split(strings.TrimSpace(actual), "\n")
	ex := strings.Split(strings.TrimSpace(expect), "\n")
	if len(re) == len(ex) {
		for i := range re {
			if strings.TrimSpace(re[i]) != strings.TrimSpace(ex[i]) {
				t.Logf("Expect: %s", strings.TrimSpace(ex[i]))
				t.Logf("Actual: %s", strings.TrimSpace(re[i]))
				goto mismatched
			}
		}
		matched = true
	}
mismatched:
	return matched
}

func TestGeoMap(t *testing.T) {
	tests := []struct {
		name       string
		input      []string
		expectJSON string
		expectHTML string
		expectJS   string
	}{
		{
			name: "geomap_test",
			input: []string{
				`{
					"type": "marker",
					"value": [37.497850, 127.027756],
					"option": {
						"popup": {
							"content": "<b>Gangname</b><br/>Hello World?",
							"open": true
						}
					}
				}`,
				`{
					"type": "circleMarker",
					"value": [37.503058, 127.018666],
					"option": {
						"radius": 100,
						"popup": {
							"content": "<b>circle1</b>"
						}
					}
				}`,
			},
			expectJSON: "geomap_test.json",
			expectHTML: "geomap_test.html",
			expectJS:   "geomap_test.js",
		},
		{
			name: "geojson",
			input: []string{
				`{ "type": "FeatureCollection",
					"features": [
						{ "type": "Feature",
							"geometry": {"type": "Point", "coordinates": [102.0, 0.5]},
							"properties": {"prop0": "value0"}
						},
						{ "type": "Feature",
							"geometry": {
								"type": "LineString",
								"coordinates": [
									[102.0, 0.0], [103.0, 1.0], [104.0, 0.0], [105.0, 1.0]
								]
							},
							"properties": {
								"prop0": "value0",
								"prop1": 0.0
							}
						},
						{ "type": "Feature",
							"geometry": {
								"type": "Polygon",
								"coordinates": [
									[ [100.0, 0.0], [101.0, 0.0], [101.0, 1.0],
										[100.0, 1.0], [100.0, 0.0] ]
								]
							},
							"properties": {
								"prop0": "value0",
								"prop1": {"this": "that"}
							}
						}
					],
					"popup": {
						"content": "<b>GeoJSON</b>",
						"open": 0
					}
				}`,
				`{ "type": "Feature",
					"geometry": {
						"type": "Point",
						"coordinates": [125.6, 10.1]
					},
					"properties": {
						"name": "Dinagat Islands",
						"popup": {
							"content": "<b>Dinagat Islands</b>",
							"open": true
						}
					}
				}`,
				`{ "type": "Point",
					"coordinates": [135.7, 20.1]
				}`,
			},
			expectJSON: "geomap_test_geojson.json",
			expectHTML: "geomap_test_geojson.html",
			expectJS:   "geomap_test_geojson.js",
		},
	}

	for _, tc := range tests {
		outputs := []string{}
		if tc.expectJSON != "" {
			outputs = append(outputs, "json")
		}
		if tc.expectHTML != "" {
			outputs = append(outputs, "html")
		}
		for _, output := range outputs {
			t.Run(tc.name+"-"+output, func(t *testing.T) {
				buffer := &bytes.Buffer{}
				fsmock := &VolatileFileWriterMock{}

				c := geomap.New()
				c.SetLogger(facility.TestLogger(t))
				c.SetOutputStream(buffer)
				c.SetVolatileFileWriter(fsmock)
				c.SetMapId("WejMYXCGcYNL")
				c.SetGeoMapJson(output == "json")
				c.SetInitialLocation(nums.NewLatLon(51.505, -0.09), 13)
				if output == "json" {
					require.Equal(t, "application/json", c.ContentType())
				} else {
					require.Equal(t, "text/html", c.ContentType())
				}

				c.Open()
				for _, jsonString := range tc.input {
					obj := map[string]any{}
					err := json.Unmarshal([]byte(jsonString), &obj)
					if err != nil {
						fmt.Println("Error", err.Error())
						t.Fail()
					}
					c.AddRow([]any{obj})
				}
				c.Close()

				if output == "json" {
					expect, err := os.ReadFile(filepath.Join("test", tc.expectJSON))
					if err != nil {
						fmt.Println("Error", err.Error())
						t.Fail()
					}
					expect = bytes.ReplaceAll(expect, []byte("\r\n"), []byte("\n"))
					require.JSONEq(t, string(expect), buffer.String(), "%s result unmatched\n%s", output, buffer.String())

					if tc.expectJS != "" {
						require.Equal(t, fsmock.name, "/web/api/tql-assets/WejMYXCGcYNL.js")
						expect, err := os.ReadFile(filepath.Join("test", tc.expectJS))
						if err != nil {
							fmt.Println("Error", err.Error())
							t.Fail()
						}
						expect = bytes.ReplaceAll(expect, []byte("\r\n"), []byte("\n"))
						require.Equal(t, string(expect), fsmock.buff.String(), fsmock.buff.String())
					}
				}
				if output == "html" {
					expect, err := os.ReadFile(filepath.Join("test", tc.expectHTML))
					if err != nil {
						fmt.Println("Error", err.Error())
						t.Fail()
					}
					expect = bytes.ReplaceAll(expect, []byte("\r\n"), []byte("\n"))
					require.Equal(t, string(expect), buffer.String(), "%s result unmatched\n%s", output, buffer.String())
					expectStr := string(expect)
					if !StringsEq(t, expectStr, buffer.String()) {
						require.Equal(t, expectStr, buffer.String(), "%s result unmatched\n%s", output, buffer.String())
					}
					require.Equal(t, fsmock.name, "")
					require.Zero(t, fsmock.buff.String())
				}
			})
		}
	}
}

// func TestNewPopupMap(t *testing.T) {
// 	tests := []struct {
// 		name   string
// 		input  map[string]interface{}
// 		expect *geomap.Popup
// 	}{
// 		{
// 			name: "empty",
// 			input: map[string]interface{}{
// 				"popup": nil,
// 			},
// 			expect: nil,
// 		},
// 		{
// 			name: "no_popup",
// 			input: map[string]interface{}{
// 				"other": "value",
// 			},
// 			expect: nil,
// 		},
// 		{
// 			name: "popup_open",
// 			input: map[string]interface{}{
// 				"popup": map[string]any{
// 					"content": "Hello World",
// 					"open":    true,
// 				},
// 			},
// 			expect: &geomap.Popup{
// 				Content: "Hello World",
// 				Open:    true,
// 			},
// 		},
// 		{
// 			name: "popup_no_open",
// 			input: map[string]interface{}{
// 				"popup": map[string]any{
// 					"content": "Hello World",
// 				},
// 			},
// 			expect: &geomap.Popup{
// 				Content: "Hello World",
// 				Open:    false,
// 			},
// 		},
// 	}

// 	for _, tc := range tests {
// 		t.Run(tc.name, func(t *testing.T) {
// 			actual := geomap.NewPopupMap(tc.input)
// 			require.Equal(t, tc.expect, actual)
// 		})
// 	}
// }
