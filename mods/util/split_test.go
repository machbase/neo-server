package util_test

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/machbase/neo-server/mods/util"
	"github.com/stretchr/testify/require"
)

func TestSplitFields(t *testing.T) {
	testSplitFields(t, true,
		`--data "C:\Users\user\work\neo-download\neo 0.1.2\machbase_home"`,
		[]string{"--data", `C:\Users\user\work\neo-download\neo 0.1.2\machbase_home`})
	testSplitFields(t, false,
		`--data "C:\Users\user\work\neo-download\neo 0.1.2\machbase_home"`,
		[]string{"--data", `"C:\Users\user\work\neo-download\neo 0.1.2\machbase_home"`})
}

func testSplitFields(t *testing.T, stripQutes bool, args string, expects []string) {
	toks := util.SplitFields(args, stripQutes)
	require.Equal(t, len(expects), len(toks))
	for i, tok := range toks {
		require.Equal(t, expects[i], tok)
	}
}

func TestStripQuote(t *testing.T) {
	ret := util.StripQuote(`"str abc"`)
	require.Equal(t, "str abc", ret)

	ret = util.StripQuote(`"str abc'`)
	require.Equal(t, "str abc'", ret)

	ret = util.StripQuote("`str abc'")
	require.Equal(t, "`str abc'", ret)

	ret = util.StripQuote("")
	require.Equal(t, "", ret)
}

func TestStringFields(t *testing.T) {
	ts := time.Unix(1691800174, 123456789).UTC()

	vals := util.StringFields([]any{&ts}, "ns", nil, 0)
	expects := []string{"1691800174123456789"}
	require.Equal(t, expects[0], vals[0])

	vals = util.StringFields([]any{&ts}, "us", nil, 0)
	expects = []string{"1691800174123456"}
	require.Equal(t, expects[0], vals[0])

	vals = util.StringFields([]any{&ts}, "ms", nil, 0)
	expects = []string{"1691800174123"}
	require.Equal(t, expects[0], vals[0])

	vals = util.StringFields([]any{&ts}, "s", nil, 0)
	expects = []string{"1691800174"}
	require.Equal(t, expects[0], vals[0])

	vals = util.StringFields([]any{ts}, "ns", nil, 0)
	expects = []string{"1691800174123456789"}
	require.Equal(t, expects[0], vals[0])

	vals = util.StringFields([]any{ts}, "us", nil, 0)
	expects = []string{"1691800174123456"}
	require.Equal(t, expects[0], vals[0])

	vals = util.StringFields([]any{ts}, "ms", nil, 0)
	expects = []string{"1691800174123"}
	require.Equal(t, expects[0], vals[0])

	vals = util.StringFields([]any{ts}, "s", nil, 0)
	expects = []string{"1691800174"}
	require.Equal(t, expects[0], vals[0])

	vals = util.StringFields([]any{9, "123", ts, 456.789}, util.GetTimeformat("KITCHEN"), time.UTC, -1)
	expects = []string{"9", "123", "12:29:34AM", "456.789000"}
	for i, expect := range expects {
		require.Equal(t, expect, vals[i])
	}

	vals = util.StringFields([]any{9, "123", ts, 456.789}, util.GetTimeformat("KITCHEN"), nil, 0)
	expects = []string{"9", "123", "12:29:34AM", "457"}
	for i, expect := range expects {
		require.Equal(t, expect, vals[i])
	}

	ival := 9
	sval := "123"
	fval := 456.789
	vals = util.StringFields([]any{&ival, &sval, &ts, &fval, nil}, util.GetTimeformat("KITCHEN"), nil, 1)
	expects = []string{"9", "123", "12:29:34AM", "456.8", "NULL"}
	for i, expect := range expects {
		require.Equal(t, expect, vals[i])
	}

	tz, _ := util.ParseTimeLocation("EST", nil)
	vals = util.StringFields([]any{&ival, &sval, &ts, &fval, nil}, util.GetTimeformat("KITCHEN"), tz, 4)
	expects = []string{"9", "123", "7:29:34PM", "456.7890", "NULL"}
	for i, expect := range expects {
		require.Equal(t, expect, vals[i])
	}

	bval := int8(0x67)
	i16val := int16(0x16)
	i32val := int32(0x32)
	i64val := int64(0x64)
	netip := net.ParseIP("127.0.0.1")

	vals = util.StringFields([]any{&bval, &i16val, &i32val, &i64val, &fval, &netip, &RandomVal{Name: "name", Value: "value"}}, "", nil, -1)
	expects = []string{"103", "22", "50", "100", "456.789000", "127.0.0.1", "name=value"}
	for i, expect := range expects {
		require.Equal(t, expect, vals[i])
	}

	vals = util.StringFields([]any{bval, i16val, i32val, i64val, fval, netip, RandomVal{Name: "name", Value: "value"}}, "", nil, -1)
	expects = []string{"103", "22", "50", "100", "456.789000", "127.0.0.1", "util_test.RandomVal"}
	for i, expect := range expects {
		require.Equal(t, expect, vals[i])
	}
}

type RandomVal struct {
	Name  string
	Value string
}

func (v *RandomVal) String() string {
	return fmt.Sprintf("%s=%s", v.Name, v.Value)
}
