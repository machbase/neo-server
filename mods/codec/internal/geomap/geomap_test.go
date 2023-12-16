package geomap_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/machbase/neo-server/mods/codec/facility"
	"github.com/machbase/neo-server/mods/codec/internal/geomap"
	"github.com/machbase/neo-server/mods/nums"
	"github.com/machbase/neo-server/mods/stream"
	"github.com/stretchr/testify/require"
)

func HTMLEq(t *testing.T, expect string, actual string) bool {
	matched := false
	t.Helper()
	re := strings.Split(actual, "\n")
	ex := strings.Split(expect, "\n")
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

func TestGeoMapHtml(t *testing.T) {
	buffer := &bytes.Buffer{}
	c := geomap.New()
	c.SetLogger(facility.TestLogger(t))
	c.SetOutputStream(stream.NewOutputStreamWriter(buffer))
	c.SetMapId("WejMYXCGcYNL")
	c.SetInitialLocation(nums.NewLatLon(51.505, -0.09), 13)
	c.SetPointStyle("rec", "circleMarker", `"color": "#ff0000"`)
	require.Equal(t, "text/html", c.ContentType())

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

	expect, err := os.ReadFile(filepath.Join("test", "geomap_test.html"))
	if err != nil {
		fmt.Println("Error", err.Error())
		t.Fail()
	}
	expectStr := string(expect)
	if !HTMLEq(t, expectStr, buffer.String()) {
		require.Equal(t, expectStr, buffer.String(), "html result unmatched\n%s", buffer.String())
	}
}

func TestGeoMapJson(t *testing.T) {
	buffer := &bytes.Buffer{}
	c := geomap.New()
	c.SetLogger(facility.TestLogger(t))
	c.SetOutputStream(stream.NewOutputStreamWriter(buffer))
	c.SetMapId("WejMYXCGcYNL")
	c.SetInitialLocation(nums.NewLatLon(51.505, -0.09), 13)
	c.SetGeoMapJson(true)
	require.Equal(t, "application/json", c.ContentType())

	tick := time.Unix(0, 1692670838086467000)

	c.Open()
	c.AddRow([]any{tick.Add(0 * time.Second), nums.NewGeoPoint(nums.NewLatLon(51.505, -0.09), "")})
	c.Close()

	expect, err := os.ReadFile(filepath.Join("test", "geomap_test.json"))
	if err != nil {
		fmt.Println("Error", err.Error())
		t.Fail()
	}
	require.JSONEq(t, string(expect), buffer.String(), "json result unmatched\n%s", buffer.String())
}
