package util_test

import (
	"testing"
	"time"

	"github.com/machbase/neo-server/mods/util"
	"github.com/stretchr/testify/require"
)

func TestHumanizeNumber(t *testing.T) {
	tests := []struct {
		in     int64
		expect string
	}{
		{1234, "1,234"},
		{123456789, "123,456,789"},
		{-123456789, "-123,456,789"},
	}
	for _, tt := range tests {
		actual := util.HumanizeNumber(tt.in)
		require.Equal(t, tt.expect, actual)
	}
}

func TestHumanizeByteCount(t *testing.T) {
	ret := util.HumanizeByteCount(512)
	require.Equal(t, "512 B", ret)

	ret = util.HumanizeByteCount(1024)
	require.Equal(t, "1.0 kB", ret)

	ret = util.HumanizeByteCount(1024 + 512)
	require.Equal(t, "1.5 kB", ret)

	ret = util.HumanizeByteCount((1024 + 512) * 1000)
	require.Equal(t, "1.5 MB", ret)

	ret = util.HumanizeByteCount((1024 + 512) * 1000 * 1000)
	require.Equal(t, "1.5 GB", ret)

	ret = util.HumanizeByteCount((1024 + 512) * 1000 * 1000 * 1000)
	require.Equal(t, "1.5 TB", ret)

	ret = util.HumanizeByteCount((1024 + 512) * 1000 * 1000 * 1000 * 1000)
	require.Equal(t, "1.5 PB", ret)

	ret = util.HumanizeByteCount((1024 + 512) * 1000 * 1000 * 1000 * 1000 * 1000)
	require.Equal(t, "1.5 EB", ret)

}

func TestHumanizeDuration(t *testing.T) {
	var ret string

	// under a minute
	ret = util.HumanizeDuration(32 * time.Second)
	require.Equal(t, "32 seconds", ret)

	ret = util.HumanizeDurationWithFormat(32*time.Second, util.HumanizeDurationFormatShort)
	require.Equal(t, "0:32", ret)

	ret = util.HumanizeDurationWithFormat(32*time.Second, util.HumanizeDurationFormatShortPadding)
	require.Equal(t, "00:32", ret)

	ret = util.HumanizeDurationWithFormat(32*time.Second, util.HumanizeDurationFormatSimple)
	require.Equal(t, "32s", ret)

	ret = util.HumanizeDurationWithFormat(3*time.Second, util.HumanizeDurationFormatSimplePadding)
	require.Equal(t, "03s", ret)

	// under an hour
	ret = util.HumanizeDuration(45*time.Minute + 32*time.Second)
	require.Equal(t, "45 minutes 32 seconds", ret)

	ret = util.HumanizeDurationWithFormat(45*time.Minute+32*time.Second, util.HumanizeDurationFormatShort)
	require.Equal(t, "45:32", ret)

	ret = util.HumanizeDurationWithFormat(4*time.Minute+32*time.Second, util.HumanizeDurationFormatShortPadding)
	require.Equal(t, "04:32", ret)

	ret = util.HumanizeDurationWithFormat(45*time.Minute+32*time.Second, util.HumanizeDurationFormatSimple)
	require.Equal(t, "45m 32s", ret)

	ret = util.HumanizeDurationWithFormat(4*time.Minute+3*time.Second, util.HumanizeDurationFormatSimplePadding)
	require.Equal(t, "04m 03s", ret)

	// under a day
	ret = util.HumanizeDuration(1*time.Hour + 1*time.Minute + 1*time.Second)
	require.Equal(t, "1 hour 1 minute 1 second", ret)

	ret = util.HumanizeDurationWithFormat(2*time.Hour+45*time.Minute+32*time.Second, util.HumanizeDurationFormatShort)
	require.Equal(t, "2:45:32", ret)

	ret = util.HumanizeDurationWithFormat(3*time.Hour+4*time.Minute+32*time.Second, util.HumanizeDurationFormatShortPadding)
	require.Equal(t, "03:04:32", ret)

	ret = util.HumanizeDurationWithFormat(13*time.Hour+45*time.Minute+32*time.Second, util.HumanizeDurationFormatSimple)
	require.Equal(t, "13h 45m 32s", ret)

	ret = util.HumanizeDurationWithFormat(5*time.Hour+4*time.Minute+3*time.Second, util.HumanizeDurationFormatSimplePadding)
	require.Equal(t, "05h 04m 03s", ret)

	// days
	ret = util.HumanizeDuration(123*24*time.Hour + 14*time.Hour + 56*time.Minute + 54*time.Second + 32*time.Millisecond)
	require.Equal(t, "123 days 14 hours 56 minutes 54 seconds", ret)

	ret = util.HumanizeDuration(123*24*time.Hour + 14*time.Hour + 78*time.Second + 98*time.Millisecond)
	require.Equal(t, "123 days 14 hours 1 minute 18 seconds", ret)

	ret = util.HumanizeDurationWithFormat(123*24*time.Hour+14*time.Hour+56*time.Minute+78*time.Second+98*time.Millisecond, util.HumanizeDurationFormatShort)
	require.Equal(t, "123 days 14:57:18", ret)

	ret = util.HumanizeDurationWithFormat(24*time.Hour+4*time.Hour+5*time.Minute+6*time.Second+98*time.Millisecond, util.HumanizeDurationFormatShortPadding)
	require.Equal(t, "1 day 04:05:06", ret)

	ret = util.HumanizeDurationWithFormat(24*time.Hour+14*time.Hour+56*time.Minute+78*time.Second+98*time.Millisecond, util.HumanizeDurationFormatSimple)
	require.Equal(t, "1 day 14h 57m 18s", ret)

	ret = util.HumanizeDurationWithFormat(2*24*time.Hour+4*time.Hour+5*time.Minute+6*time.Second+98*time.Millisecond, util.HumanizeDurationFormatSimplePadding)
	require.Equal(t, "2 days 04h 05m 06s", ret)
}
