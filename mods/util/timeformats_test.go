package util_test

import (
	"testing"
	"time"

	"github.com/machbase/neo-server/mods/util"
	"github.com/stretchr/testify/require"
)

func TestTimeFormatParser(t *testing.T) {
	var ret time.Time
	var err error

	ret, err = util.ParseTime("1691800174123456789", "ns", nil)
	require.Nil(t, err)
	ts := time.Unix(1691800174, 123456789)
	require.Equal(t, ts, ret)

	ret, err = util.ParseTime("1691800174123456", "us", nil)
	require.Nil(t, err)
	ts = time.Unix(1691800174, 123456000)
	require.Equal(t, ts, ret)

	ret, err = util.ParseTime("1691800174123", "ms", nil)
	require.Nil(t, err)
	ts = time.Unix(1691800174, 123000000)
	require.Equal(t, ts, ret)

	ret, err = util.ParseTime("1691800174", "s", nil)
	require.Nil(t, err)
	ts = time.Unix(1691800174, 0)
	require.Equal(t, ts, ret)

	require.Nil(t, err)
	ret, err = util.ParseTime("2023-08-12 00:29:34.123", "2006-01-02 15:04:05.999", time.UTC)
	require.Nil(t, err)
	ts = time.Unix(1691800174, 123000000).UTC()
	require.Equal(t, ts, ret)
}
