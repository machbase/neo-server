package echart_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/machbase/neo-server/mods/codec/internal/echart"
	"github.com/machbase/neo-server/mods/codec/opts"
	"github.com/machbase/neo-server/mods/stream"
	"github.com/stretchr/testify/require"
)

func TestTimeformat(t *testing.T) {
	buffer := &bytes.Buffer{}

	line := echart.NewLine()
	opts := []opts.Option{
		opts.OutputStream(stream.NewOutputStreamWriter(buffer)),
		opts.ChartJson(true),
		opts.TimeLocation(time.UTC),
		opts.XAxis(0, "time", "time"),
		opts.Timeformat("15:04:05.999999999"),
	}
	for _, o := range opts {
		o(line)
	}
	tick := time.Unix(0, 1692670838086467000)
	line.AddRow([]any{tick.Add(0 * time.Second), 0.0})
	line.AddRow([]any{tick.Add(1 * time.Second), 1.0})
	line.AddRow([]any{tick.Add(2 * time.Second), 2.0})
	line.Close()

	substr := `"xAxis":[{"name":"time","show":true,"data":["02:20:38.086467","02:20:39.086467","02:20:40.086467"]}]`
	require.True(t, strings.Contains(buffer.String(), substr))
}
