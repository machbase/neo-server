package geomap_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/machbase/neo-server/v8/mods/codec/facility"
	"github.com/machbase/neo-server/v8/mods/codec/internal/geomap"
	"github.com/machbase/neo-server/v8/mods/nums"
	"github.com/machbase/neo-server/v8/mods/stream"
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
	for _, output := range []string{"json", "html"} {
		buffer := &bytes.Buffer{}
		fsmock := &VolatileFileWriterMock{}

		c := geomap.New()
		c.SetLogger(facility.TestLogger(t))
		c.SetOutputStream(stream.NewOutputStreamWriter(buffer))
		c.SetVolatileFileWriter(fsmock)
		c.SetMapId("WejMYXCGcYNL")
		c.SetGeoMapJson(output == "json")
		c.SetInitialLocation(nums.NewLatLon(51.505, -0.09), 13)
		c.SetPointStyle("rec", "circleMarker", `"color": "#ff0000"`)
		if output == "html" {
			require.Equal(t, "text/html", c.ContentType())
		} else {
			require.Equal(t, "application/json", c.ContentType())
		}

		c.Open()

		c.AddRow([]any{
			nums.GeoPointMarker{
				GeoPoint: nums.NewGeoPoint(&nums.LatLon{Lat: 37.497850, Lon: 127.027756}, map[string]any{
					"popup.content": "<b>Gangname</b><br/>Hello World?",
					"popup.open":    true,
				}),
			},
			nums.GeoCircleMarker{
				GeoCircle: nums.NewGeoCircle(&nums.LatLon{Lat: 37.503058, Lon: 127.018666}, 100, `{
				"popup.content": "<b>circle1</b>"
			}`),
			},
			nums.NewGeoPoint(
				&nums.LatLon{Lat: 37.496727, Lon: 127.026612},
				map[string]any{"popup.content": "<b>point1</b>"},
			),
		})
		c.Close()

		expect, err := os.ReadFile(filepath.Join("test", fmt.Sprintf("geomap_test.%s", output)))
		if err != nil {
			fmt.Println("Error", err.Error())
			t.Fail()
		}
		expectStr := string(expect)
		if output == "json" {
			require.JSONEq(t, expectStr, buffer.String(), "%s result unmatched\n%s", output, buffer.String())
		} else {
			if !StringsEq(t, expectStr, buffer.String()) {
				require.Equal(t, expectStr, buffer.String(), "%s result unmatched\n%s", output, buffer.String())
			}
		}

		expect, err = os.ReadFile(filepath.Join("test", "geomap_test.js"))
		if err != nil {
			fmt.Println("Error", err.Error())
			t.Fail()
		}
		expectStr = string(expect)
		if !StringsEq(t, expectStr, fsmock.buff.String()) {
			require.Equal(t, expectStr, fsmock.buff.String(), "js result unmatched\n%s", fsmock.buff.String())
		}
	}
}
