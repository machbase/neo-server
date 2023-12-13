package geomap_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

func TestGeoMap(t *testing.T) {
	buffer := &bytes.Buffer{}
	c := geomap.New()
	c.SetOutputStream(stream.NewOutputStreamWriter(buffer))
	c.SetMapId("WejMYXCGcYNL")
	c.SetInitialLocation(nums.NewLatLng(51.505, -0.09), 13)
	require.Equal(t, "text/html", c.ContentType())

	tick := time.Unix(0, 1692670838086467000)

	c.Open()
	c.AddRow([]any{tick.Add(0 * time.Second), 0.0})
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
