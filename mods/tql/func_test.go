package tql_test

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/machbase/neo-server/mods/codec/opts"
	"github.com/machbase/neo-server/mods/expression"
	"github.com/machbase/neo-server/mods/tql"
	"github.com/machbase/neo-server/mods/util"
	"github.com/stretchr/testify/require"
)

type FunctionTestCase struct {
	f         func(args ...any) (any, error)
	args      []any
	expect    any
	expectErr string
}

func (tc FunctionTestCase) run(t *testing.T) {
	t.Helper()
	ret, err := tc.f(tc.args...)
	if tc.expectErr != "" {
		require.NotNil(t, err)
		require.Equal(t, tc.expectErr, err.Error())
		return
	}
	require.Nil(t, err)
	require.Equal(t, tc.expect, ret)
}

func TestParseFloat(t *testing.T) {
	node := tql.NewNode(tql.NewTask())
	FunctionTestCase{f: node.Function("parseFloat"),
		args:   []any{"0"},
		expect: 0.0,
	}.run(t)
	FunctionTestCase{f: node.Function("parseFloat"),
		args:   []any{"-1.23"},
		expect: -1.23,
	}.run(t)
}

func TestParseBool(t *testing.T) {
	node := tql.NewNode(tql.NewTask())
	FunctionTestCase{f: node.Function("parseBool"),
		args:   []any{"true"},
		expect: true,
	}.run(t)
	FunctionTestCase{f: node.Function("parseBool"),
		args:   []any{"0"},
		expect: false,
	}.run(t)
	FunctionTestCase{f: node.Function("parseBool"),
		args:      []any{"some other text"},
		expectErr: "parseBool: parsing \"some other text\": invalid syntax",
	}.run(t)
}

func TestStrTime(t *testing.T) {
	now := time.Unix(0, 1704871917655327000)
	node := tql.NewNode(tql.NewTask())
	FunctionTestCase{f: node.Function("strTime"),
		args:   []any{now, "RFC822", time.UTC},
		expect: "10 Jan 24 07:31 UTC",
	}.run(t)
	FunctionTestCase{f: node.Function("strTime"),
		args:   []any{now, "2006/01/02 15:04:05.999999", time.UTC},
		expect: "2024/01/10 07:31:57.655327",
	}.run(t)
	FunctionTestCase{f: node.Function("strTime"),
		args:   []any{now, opts.Timeformat(util.ToTimeformatSql("YYYY/MM/DD HH24:MI:SS.nnnnnn")), time.UTC},
		expect: "2024/01/10 07:31:57.655327",
	}.run(t)
	FunctionTestCase{f: node.Function("strTime"),
		args:   []any{now, "ns", time.UTC},
		expect: "1704871917655327000",
	}.run(t)
	FunctionTestCase{f: node.Function("strTime"),
		args:   []any{now, "us"},
		expect: "1704871917655327",
	}.run(t)
	FunctionTestCase{f: node.Function("strTime"),
		args:   []any{now, "ms", time.UTC},
		expect: "1704871917655",
	}.run(t)
	FunctionTestCase{f: node.Function("strTime"),
		args:   []any{now, "s"},
		expect: "1704871917",
	}.run(t)
}

func TestStrTrimSpace(t *testing.T) {
	node := tql.NewNode(tql.NewTask())
	FunctionTestCase{f: node.Function("strTrimSpace"),
		args:   []any{"  text\t\n"},
		expect: "text",
	}.run(t)
	FunctionTestCase{f: node.Function("strTrimSpace"),
		args:   []any{"   "},
		expect: "",
	}.run(t)
}

func TestStrTrimPrefix(t *testing.T) {
	node := tql.NewNode(tql.NewTask())
	FunctionTestCase{f: node.Function("strTrimPrefix"),
		args:   []any{"  text\t\n", "  "},
		expect: "text\t\n",
	}.run(t)
	FunctionTestCase{f: node.Function("strTrimPrefix"),
		args:   []any{"__text", "_"},
		expect: "_text",
	}.run(t)
}

func TestStrTrimSuffix(t *testing.T) {
	node := tql.NewNode(tql.NewTask())
	FunctionTestCase{f: node.Function("strTrimSuffix"),
		args:   []any{"  text\t\n", "\t\n"},
		expect: "  text",
	}.run(t)
	FunctionTestCase{f: node.Function("strTrimSuffix"),
		args:   []any{"__text", "text"},
		expect: "__",
	}.run(t)
}

func TestStrReplace(t *testing.T) {
	node := tql.NewNode(tql.NewTask())
	FunctionTestCase{f: node.Function("strReplace"),
		args:   []any{"apple", "a", "A", 1},
		expect: "Apple",
	}.run(t)
	FunctionTestCase{f: node.Function("strReplace"),
		args:   []any{"apple", "p", "P", 1},
		expect: "aPple",
	}.run(t)
	FunctionTestCase{f: node.Function("strReplace"),
		args:   []any{"apple", "p", "P", -1},
		expect: "aPPle",
	}.run(t)
}

func TestStrReplaceAll(t *testing.T) {
	node := tql.NewNode(tql.NewTask())
	FunctionTestCase{f: node.Function("strReplaceAll"),
		args:   []any{"apple", "a", "A"},
		expect: "Apple",
	}.run(t)
	FunctionTestCase{f: node.Function("strReplaceAll"),
		args:   []any{"apple", "p", "P"},
		expect: "aPPle",
	}.run(t)
}

func TestStrSprintf(t *testing.T) {
	node := tql.NewNode(tql.NewTask())
	FunctionTestCase{f: node.Function("strSprintf"),
		args:   []any{"hello %s %1.2f", "world", 3.141592},
		expect: "hello world 3.14",
	}.run(t)
}

func TestStrSub(t *testing.T) {
	node := tql.NewNode(tql.NewTask())
	FunctionTestCase{f: node.Function("strSub"),
		args:   []any{"HelLo 😀 World"},
		expect: "HelLo 😀 World",
	}.run(t)
	FunctionTestCase{f: node.Function("strSub"),
		args:   []any{"😀HelLo World", 0, 3},
		expect: "😀He",
	}.run(t)
	FunctionTestCase{f: node.Function("strSub"),
		args:   []any{"HelLo 😀 World", 6, -2},
		expect: "😀 World",
	}.run(t)
	FunctionTestCase{f: node.Function("strSub"),
		args:   []any{"HelLo 😀 World", -7},
		expect: "😀 World",
	}.run(t)
	FunctionTestCase{f: node.Function("strSub"),
		args:   []any{"HelLo 😀 World", -7, 3},
		expect: "😀 W",
	}.run(t)
	FunctionTestCase{f: node.Function("strSub"),
		args:   []any{"HelLo 😀 World", -0},
		expect: "HelLo 😀 World",
	}.run(t)
	FunctionTestCase{f: node.Function("strSub"),
		args:   []any{"HelLo 😀 World", -1},
		expect: "d",
	}.run(t)
	FunctionTestCase{f: node.Function("strSub"),
		args:   []any{"HelLo 😀 World", -30},
		expect: "",
	}.run(t)
	FunctionTestCase{f: node.Function("strSub"),
		args:   []any{"HelLo 😀 World", 0, 30},
		expect: "HelLo 😀 World",
	}.run(t)
	FunctionTestCase{f: node.Function("strSub"),
		args:   []any{"HelLo 😀 World", 30, 30},
		expect: "",
	}.run(t)
}

func TestStrIndex(t *testing.T) {
	node := tql.NewNode(tql.NewTask())
	FunctionTestCase{f: node.Function("strIndex"),
		args:   []any{"HelLo 😀 World", "😀"},
		expect: 6,
	}.run(t)
	FunctionTestCase{f: node.Function("strIndex"),
		args:   []any{"HelLo 😀 World", "o"},
		expect: 4,
	}.run(t)
	FunctionTestCase{f: node.Function("strIndex"),
		args:   []any{"HelLo 😀 World", "l"},
		expect: 2,
	}.run(t)
}

func TestStrLastIndex(t *testing.T) {
	node := tql.NewNode(tql.NewTask())
	FunctionTestCase{f: node.Function("strLastIndex"),
		args:   []any{"HelLo 😀 World", "😀"},
		expect: 6,
	}.run(t)
	FunctionTestCase{f: node.Function("strLastIndex"),
		args:   []any{"HelLo 😀 World", "o"},
		expect: 12,
	}.run(t)
	FunctionTestCase{f: node.Function("strLastIndex"),
		args:   []any{"HelLo 😀 World", "H"},
		expect: 0,
	}.run(t)
	FunctionTestCase{f: node.Function("strLastIndex"),
		args:   []any{"HelLo 😀 World", "l"},
		expect: 14,
	}.run(t)
}

func TestList(t *testing.T) {
	node := tql.NewNode(tql.NewTask())
	FunctionTestCase{f: node.Function("list"),
		args:   []any{"HelLo 😀", 3.14, true},
		expect: []any{"HelLo 😀", 3.14, true},
	}.run(t)
}

func TestGlob(t *testing.T) {
	node := tql.NewNode(tql.NewTask())
	FunctionTestCase{f: node.Function("glob"),
		args:   []any{"test*me", "test123me"},
		expect: true,
	}.run(t)
	FunctionTestCase{f: node.Function("glob"),
		args:   []any{"test*me", "testme"},
		expect: true,
	}.run(t)
	FunctionTestCase{f: node.Function("glob"),
		args:   []any{"test*me", "test123not"},
		expect: false,
	}.run(t)
}

func TestRegexp(t *testing.T) {
	node := tql.NewNode(tql.NewTask())
	FunctionTestCase{f: node.Function("regexp"),
		args:      []any{`^test[0-9$`, "test123"},
		expectErr: "error parsing regexp: missing closing ]: `[0-9$`",
	}.run(t)
	FunctionTestCase{f: node.Function("regexp"),
		args:   []any{`^test[0-9]{3}$`, "test123"},
		expect: true,
	}.run(t)
	FunctionTestCase{f: node.Function("regexp"),
		args:   []any{`^test[0-9]{3}$`, "test12"},
		expect: false,
	}.run(t)
	FunctionTestCase{f: node.Function("regexp"),
		args:   []any{`^test\d{3}$`, "test12345x"},
		expect: false,
	}.run(t)
	FunctionTestCase{f: node.Function("regexp"),
		args:   []any{`^test\d{3}$`, "test999"},
		expect: true,
	}.run(t)
	FunctionTestCase{f: node.Function("regexp"),
		args:   []any{`^test\d{5}x$`, "test12345x"},
		expect: true,
	}.run(t)
}

func TestTime(t *testing.T) {
	node := tql.NewNode(tql.NewTask())

	tick := time.Now()
	util.StandardTimeNow = func() time.Time { return tick }
	// invalid number of args
	FunctionTestCase{f: node.Function("time"),
		args:      []any{},
		expectErr: "f(time) invalid number of args; expect:1, actual:0",
	}.run(t)
	// first args should be time, but %s",
	FunctionTestCase{f: node.Function("time"),
		args:      []any{"last"},
		expectErr: "invalid time expression: incompatible conv 'last' (string) to time.Time",
	}.run(t)
	// first args should be time, but
	FunctionTestCase{f: node.Function("time"),
		args:      []any{true},
		expectErr: "invalid time expression: incompatible conv 'true' (bool) to time.Time",
	}.run(t)
	// f(time) second args should be time, but %s
	FunctionTestCase{f: node.Function("time"),
		args:      []any{"oned2h"},
		expectErr: "invalid time expression: incompatible conv 'oned2h' (string) to time.Time",
	}.run(t)
	// f(time) second args should be time, but %s
	FunctionTestCase{f: node.Function("time"),
		args:      []any{"1d27h"},
		expectErr: "invalid time expression: incompatible conv '1d27h' (string) to time.Time",
	}.run(t)
	// f(time) second args should be duration, but %s
	FunctionTestCase{f: node.Function("timeAdd"),
		args:      []any{tick, "-2x"},
		expectErr: "invalid time expression: time: unknown unit \"x\" in duration \"-2x\"",
	}.run(t)
	FunctionTestCase{f: node.Function("time"),
		args:   []any{123456789.0},
		expect: time.Unix(0, 123456789),
	}.run(t)
	FunctionTestCase{f: node.Function("time"),
		args:   []any{"now"},
		expect: tick,
	}.run(t)
	FunctionTestCase{f: node.Function("timeAdd"),
		args:   []any{"now", "1s"},
		expect: tick.Add(1 * time.Second),
	}.run(t)
	FunctionTestCase{f: node.Function("timeAdd"),
		args:   []any{"now", "1d"},
		expect: tick.Add(24 * time.Hour),
	}.run(t)
	FunctionTestCase{f: node.Function("timeAdd"),
		args:   []any{"now", "-2d"},
		expect: tick.Add(-24 * 2 * time.Hour),
	}.run(t)
	FunctionTestCase{f: node.Function("timeAdd"),
		args:   []any{"now", "-1d12h"},
		expect: tick.Add(-24 * 1.5 * time.Hour),
	}.run(t)
	FunctionTestCase{f: node.Function("timeAdd"),
		args:   []any{"now", "-1d2h3m4s"},
		expect: tick.Add(-24*1*time.Hour - 2*time.Hour - 3*time.Minute - 4*time.Second),
	}.run(t)
	FunctionTestCase{f: node.Function("timeAdd"),
		args:   []any{"now-1s", 1000000000},
		expect: tick,
	}.run(t)
	FunctionTestCase{f: node.Function("timeAdd"),
		args:      []any{"now-1x", 1000000000},
		expectErr: "invalid time expression: incompatible conv 'now-1x', time: unknown unit \"x\" in duration \"1x\"",
	}.run(t)
	// time.Time
	FunctionTestCase{f: node.Function("time"),
		args:   []any{tick},
		expect: tick,
	}.run(t)
	// *time.Time
	FunctionTestCase{f: node.Function("time"),
		args:   []any{&tick},
		expect: tick,
	}.run(t)

	FunctionTestCase{f: node.Function("timeUnix"),
		args:   []any{&tick},
		expect: float64(tick.Unix()),
	}.run(t)
	FunctionTestCase{f: node.Function("timeUnixMilli"),
		args:   []any{&tick},
		expect: float64(tick.UnixMilli()),
	}.run(t)
	FunctionTestCase{f: node.Function("timeUnixMicro"),
		args:   []any{tick},
		expect: float64(tick.UnixMicro()),
	}.run(t)
	FunctionTestCase{f: node.Function("timeUnixNano"),
		args:   []any{tick},
		expect: float64(tick.UnixNano()),
	}.run(t)

}

func TestParseTime(t *testing.T) {
	node := tql.NewNode(tql.NewTask())
	FunctionTestCase{f: node.Function("tz"),
		args:   []any{"local"},
		expect: time.Local,
	}.run(t)
	FunctionTestCase{f: node.Function("tz"),
		args:   []any{"utc"},
		expect: time.UTC,
	}.run(t)
	FunctionTestCase{f: node.Function("tz"),
		args:      []any{"wrong/place"},
		expectErr: "unknown timezone 'wrong/place'",
	}.run(t)
	FunctionTestCase{f: node.Function("parseTime"),
		args:   []any{"2023-03-01 14:01:02", "DEFAULT", time.Local},
		expect: time.Time(time.Date(2023, time.March, 1, 14, 1, 2, 0, time.Local)),
	}.run(t)

	FunctionTestCase{f: node.Function("parseTime"),
		args:   []any{"2023-03-01 14:01:02", "DEFAULT", time.UTC},
		expect: time.Time(time.Date(2023, time.March, 1, 14, 1, 2, 0, time.UTC)),
	}.run(t)
}

func TestRoundTime(t *testing.T) {
	node := tql.NewNode(tql.NewTask())
	FunctionTestCase{f: node.Function("roundTime"),
		args:      []any{time.Unix(123, 456789123), "0s"},
		expectErr: "f(roundTime) arg(1) zero duration is not allowed",
	}.run(t)
	FunctionTestCase{f: node.Function("roundTime"),
		args:      []any{true, "500ms"},
		expectErr: "f(roundTime) arg(0) incompatible conv 'true' (bool) to time.Time",
	}.run(t)
	FunctionTestCase{f: node.Function("roundTime"),
		args:   []any{time.Unix(123, 456789123), "1s"},
		expect: time.Unix(123, 000000000),
	}.run(t)
	FunctionTestCase{f: node.Function("roundTime"),
		args:   []any{time.Unix(123, 456789123), "10ms"},
		expect: time.Unix(123, 450000000),
	}.run(t)
	FunctionTestCase{f: node.Function("roundTime"),
		args:   []any{time.Unix(123, 456789123), "10us"},
		expect: time.Unix(123, 456780000),
	}.run(t)
	FunctionTestCase{f: node.Function("roundTime"),
		args:   []any{123456789123.0, "10us"},
		expect: time.Unix(123, 456780000),
	}.run(t)
}

func TestRangeTime(t *testing.T) {
	node := tql.NewNode(tql.NewTask())
	FunctionTestCase{f: node.Function("range"),
		args:      []any{false, "1s", "100ms"},
		expectErr: "f(range) arg(0) should be time, but bool",
	}.run(t)
	FunctionTestCase{f: node.Function("range"),
		args:      []any{0, "1x", "100ms"},
		expectErr: "f(range) arg(1) should be duration, but string",
	}.run(t)
	FunctionTestCase{f: node.Function("range"),
		args:      []any{0, "1s", "100x"},
		expectErr: "f(range) arg(2) should be period, but string",
	}.run(t)
	FunctionTestCase{f: node.Function("range"),
		args:      []any{0, "500ms", "1s"},
		expectErr: "f(range) arg(2) period should be smaller than duration",
	}.run(t)
	FunctionTestCase{f: node.Function("range"),
		args:   []any{0, "1s"},
		expect: &tql.TimeRange{Time: time.Unix(0, 0), Duration: time.Second},
	}.run(t)
}

func TestLen(t *testing.T) {
	node := tql.NewNode(tql.NewTask())
	FunctionTestCase{f: node.Function("len"),
		args:   []any{[]string{"1", "2", "3", "4"}},
		expect: 4.0,
	}.run(t)
	FunctionTestCase{f: node.Function("len"),
		args:   []any{"1234"},
		expect: 4.0,
	}.run(t)
}

func TestElement(t *testing.T) {
	node := tql.NewNode(tql.NewTask())
	// invalid number of args
	FunctionTestCase{f: node.Function("element"),
		args:      []any{1, 2},
		expectErr: "f(element) invalud number of args (n:2)",
	}.run(t)
	// out of index
	FunctionTestCase{f: node.Function("element"),
		args:      []any{0.0, 1.0, 2.0, 3.0, 4.0, 5.0},
		expectErr: "f(element) out of index 5 / 5",
	}.run(t)
	// invalid index
	FunctionTestCase{f: node.Function("element"),
		args:      []any{0.0, 1.0, 2.0, 3.0, 4.0, "4"},
		expectErr: "f(element) index of element should be int, but string",
	}.run(t)
	// unsupported type
	FunctionTestCase{f: node.Function("element"),
		args:      []any{0.0, 1.0, 2.0, 3.0, time.Duration(1), 4},
		expectErr: "f(element) unsupported type time.Duration",
	}.run(t)
	FunctionTestCase{f: node.Function("element"),
		args:   []any{0.0, 1.0, 2.0, 3.0, 4.0, 1.0},
		expect: 1.0,
	}.run(t)
	FunctionTestCase{f: node.Function("element"),
		args:   []any{0.0, 1.0, 2.0, 3.0, 4.0, 4},
		expect: 4.0,
	}.run(t)
	FunctionTestCase{f: node.Function("element"),
		args:   []any{"abc", "bcd", "cde", "def", "efg", 4},
		expect: "efg",
	}.run(t)
	FunctionTestCase{f: node.Function("element"),
		args:   []any{"abc", "bcd", "cde", "def", true, 4},
		expect: true,
	}.run(t)
	FunctionTestCase{f: node.Function("element"),
		args:   []any{"abc", "bcd", "cde", "def", 123, 4},
		expect: 123.0,
	}.run(t)
	FunctionTestCase{f: node.Function("element"),
		args:   []any{"abc", "bcd", "cde", "def", int64(12345), 4},
		expect: 12345.0,
	}.run(t)
	FunctionTestCase{f: node.Function("element"),
		args:   []any{0.0, 1.0, 2.0, 3.0, time.Unix(123, int64(456)*int64(time.Millisecond)), 4},
		expect: 123.456 * 1000000000,
	}.run(t)
	tick1 := time.Unix(123, int64(456)*int64(time.Millisecond))
	FunctionTestCase{f: node.Function("element"),
		args:   []any{0.0, 1.0, 2.0, 3.0, &tick1, 4},
		expect: 123.456 * 1000000000,
	}.run(t)
}

func TestMathFunctions(t *testing.T) {
	node := tql.NewNode(tql.NewTask())
	FunctionTestCase{f: node.Function("round"),
		args:      []any{},
		expectErr: "f(round) invalid number of args; expect:1, actual:0",
	}.run(t)
	FunctionTestCase{f: node.Function("round"),
		args:      []any{"not_a_number"},
		expectErr: "f(round) arg(0) should be float64, but string",
	}.run(t)
	FunctionTestCase{f: node.Function("round"),
		args:      nil,
		expectErr: "f(round) <nil> is invalid argument",
	}.run(t)

	FunctionTestCase{f: node.Function("pow10"),
		args:      []any{},
		expectErr: "f(pow10) invalid number of args; expect:1, actual:0",
	}.run(t)
	FunctionTestCase{f: node.Function("pow10"),
		args:      []any{"not_a_number"},
		expectErr: "f(pow10) arg(0) should be int, but string",
	}.run(t)
	FunctionTestCase{f: node.Function("pow10"),
		args:      nil,
		expectErr: "f(pow10) <nil> is invalid argument",
	}.run(t)

	FunctionTestCase{f: node.Function("pow"),
		args:      []any{},
		expectErr: "f(pow) invalid number of args; expect:2, actual:0",
	}.run(t)
	FunctionTestCase{f: node.Function("pow"),
		args:      []any{1.0},
		expectErr: "f(pow) invalid number of args; expect:2, actual:1",
	}.run(t)
	FunctionTestCase{f: node.Function("pow"),
		args:      []any{"not_a_number", "2.0"},
		expectErr: "f(pow) arg(0) should be float64, but string",
	}.run(t)
	FunctionTestCase{f: node.Function("pow"),
		args:      []any{"1.0", "not_a_number"},
		expectErr: "f(pow) arg(1) should be float64, but string",
	}.run(t)
	FunctionTestCase{f: node.Function("pow"),
		args:   []any{1.0, nil},
		expect: nil,
	}.run(t)

	tests := []FunctionTestCase{
		{f: node.Function("abs"), args: []any{1.1}, expect: 1.1},
		{f: node.Function("abs"), args: []any{-1.1}, expect: float64(1.1)},
		{f: node.Function("acos"), args: []any{math.Cos(math.Pi)}, expect: math.Pi},
		{f: node.Function("asin"), args: []any{math.Sin(math.Pi / 2)}, expect: math.Pi / 2},
		{f: node.Function("ceil"), args: []any{3.1415}, expect: 4.0},
		{f: node.Function("cos"), args: []any{math.Pi}, expect: -1.0},
		{f: node.Function("exp"), args: []any{0.0}, expect: 1.0},
		{f: node.Function("exp2"), args: []any{2.0}, expect: 4.0},
		{f: node.Function("floor"), args: []any{3.14}, expect: 3.0},
		{f: node.Function("log"), args: []any{1.0}, expect: 0.0},
		{f: node.Function("log2"), args: []any{8.0}, expect: 3.0},
		{f: node.Function("log10"), args: []any{100.0}, expect: 2.0},
		{f: node.Function("min"), args: []any{1.0, 1.1}, expect: float64(1.0)},
		{f: node.Function("max"), args: []any{1.0, 1.1}, expect: float64(1.1)},
		{f: node.Function("mod"), args: []any{5.0, 2.0}, expect: float64(1.0)},
		{f: node.Function("pow"), args: []any{2.0, 3.0}, expect: float64(8.0)},
		{f: node.Function("pow10"), args: []any{3.0}, expect: float64(1000.0)},
		{f: node.Function("remainder"), args: []any{5.0, 2.0}, expect: float64(1.0)},
		{f: node.Function("round"), args: []any{123.4567}, expect: float64(123)},
		{f: node.Function("round"), args: []any{234.5678}, expect: float64(235)},
		{f: node.Function("sin"), args: []any{math.Pi / 2}, expect: 1.0},
		{f: node.Function("sqrt"), args: []any{4.0}, expect: 2.0},
		{f: node.Function("tan"), args: []any{0.0}, expect: 0.0},
		{f: node.Function("trunc"), args: []any{math.Pi}, expect: 3.0},
	}
	for _, tt := range tests {
		tt.run(t)
	}
}

// TestCase
type MapFuncTestCase struct {
	input     string
	params    *tql.Node // expression.Parameters
	expect    *tql.Record
	expectErr string
}

func TestMapFunc_roundTime(t *testing.T) {
	MapFuncTestCase{
		input:     `roundTime()`,
		params:    FuncParamMock(1, ""),
		expectErr: "f(roundTime) invalid number of args; expect:2, actual:0",
	}.run(t)
	MapFuncTestCase{
		input:     `roundTime(123, '1x')`,
		params:    FuncParamMock(1, ""),
		expectErr: "time: unknown unit \"x\" in duration \"1x\"",
	}.run(t)
}

func TestMapFunc_TAKE(t *testing.T) {
	MapFuncTestCase{
		input:  `TAKE(1)`,
		params: FuncParamMock("sam", []any{1, 2, 3}),
		expect: tql.NewRecord("sam", []any{1, 2, 3}),
	}.run(t)
}

func TestMapFunc_PUSHKEY(t *testing.T) {
	extime := time.Unix(123, 0)
	MapFuncTestCase{
		input:     `PUSHKEY()`,
		params:    FuncParamMock(extime, []any{1, 2, 3}),
		expectErr: "f(PUSHKEY) invalid number of args; expect:1, actual:0",
	}.run(t)
	MapFuncTestCase{
		input:  `PUSHKEY('sam')`,
		params: FuncParamMock(extime, []any{1, 2, 3}),
		expect: tql.NewRecord("sam", []any{extime, 1, 2, 3}),
	}.run(t)
	tick := time.Now()
	tick100ms := time.Unix(0, (tick.UnixNano()/100000000)*100000000)
	MapFuncTestCase{
		input:  `PUSHKEY(roundTime(key(), '100ms'))`,
		params: FuncParamMock(tick, []any{"v"}),
		expect: tql.NewRecord(tick100ms, []any{tick, "v"}),
	}.run(t)
}

func TestMapFunc_POPKEY(t *testing.T) {
	MapFuncTestCase{
		input:     `POPKEY()`,
		params:    FuncParamMock("x", []int{1, 2, 3}),
		expectErr: "f(POPKEY) V should be []any or [][]any, but []int",
	}.run(t)
	MapFuncTestCase{
		input:  `POPKEY()`,
		params: FuncParamMock("x", []any{1, 2, 3}),
		expect: tql.NewRecord(1, []any{2, 3}),
	}.run(t)
	MapFuncTestCase{
		input:  `POPKEY()`,
		params: FuncParamMock("x", []any{[]int{10, 11, 12}, []int{20, 21, 22}, []int{30, 31, 32}}),
		expect: tql.NewRecord([]int{10, 11, 12}, []any{[]int{20, 21, 22}, []int{30, 31, 32}}),
	}.run(t)
	MapFuncTestCase{
		input:     `POPKEY(0)`,
		params:    FuncParamMock("x", []int{1, 2, 3}),
		expectErr: "f(POPKEY) V should be []any or [][]any, but []int",
	}.run(t)
	MapFuncTestCase{
		input:  `POPKEY(1)`,
		params: FuncParamMock("x", []any{"K", 1, 2}),
		expect: tql.NewRecord(1, []any{"K", 2}),
	}.run(t)
}

func TestMapFunc_FILTER(t *testing.T) {
	MapFuncTestCase{
		input:  `FILTER(10<100)`,
		params: FuncParamMock("x", []any{1, 2, 3}),
		expect: tql.NewRecord("x", []any{1, 2, 3}),
	}.run(t)
	MapFuncTestCase{
		input:  `FILTER(10>100)`,
		params: FuncParamMock("x", []any{1, 2, 3}),
		expect: nil,
	}.run(t)
	MapFuncTestCase{
		input:  `FILTER(key() == 'x')`,
		params: FuncParamMock("x", []any{1, 2, 3}),
		expect: tql.NewRecord("x", []any{1, 2, 3}),
	}.run(t)
	MapFuncTestCase{
		input:  `FILTER(key() != 'x')`,
		params: FuncParamMock("x", []any{1, 2, 3}),
		expect: nil,
	}.run(t)
	MapFuncTestCase{
		input:  `FILTER(key() != 'y')`,
		params: FuncParamMock("x", []any{1, 2, 3}),
		expect: tql.NewRecord("x", []any{1, 2, 3}),
	}.run(t)
	MapFuncTestCase{
		input:  `FILTER(len(value()) > 2)`,
		params: FuncParamMock("x", []any{1, 2, 3}),
		expect: tql.NewRecord("x", []any{1, 2, 3}),
	}.run(t)
	MapFuncTestCase{
		input:  `FILTER(len(value()) > 4)`,
		params: FuncParamMock("x", []any{1, 2, 3}),
		expect: nil,
	}.run(t)
	MapFuncTestCase{
		input:  `FILTER(element(value(), 0) >= 1)`,
		params: FuncParamMock("x", []any{1, 2, 3}),
		expect: tql.NewRecord("x", []any{1, 2, 3}),
	}.run(t)
	MapFuncTestCase{
		input:  `FILTER(element(value(), 0) > 0)`,
		params: FuncParamMock("x", []any{1, 2, 3}),
		expect: tql.NewRecord("x", []any{1, 2, 3}),
	}.run(t)
}

func TestMapFunc_GROUPBYKEY(t *testing.T) {
	MapFuncTestCase{
		input:  `GROUPBYKEY()`,
		params: FuncParamMock("x", []any{1, 2, 3}),
		expect: nil,
	}.run(t)
}

func (tc MapFuncTestCase) run(t *testing.T) {
	msg := fmt.Sprintf("TestCase %s", tc.input)
	expr, err := tc.params.Parse(tc.input)
	require.Nil(t, err, msg)
	require.NotNil(t, expr, msg)

	ret, err := expr.Eval(tc.params)
	if tc.expectErr != "" {
		require.NotNil(t, err, msg)
		require.Equal(t, tc.expectErr, err.Error(), fmt.Sprintf(`"%s"`, msg))
		return
	}
	require.Nil(t, err, msg)

	if tc.expect == nil {
		require.Nil(t, ret)
		return
	}
	require.NotNil(t, ret, msg)
	// compare key
	if retParam, ok := ret.(*tql.Record); !ok {
		t.Fatalf("invalid return type: %T", ret)
	} else {
		require.True(t, tc.expect.EqualKey(retParam), "K's are different")
		require.True(t, tc.expect.EqualValue(retParam), "V's are different")
	}
}

// Mock expression.Parameters
func FuncParamMockFunc(back func(name string) (any, error)) expression.Parameters {
	return &paramMock{
		back: back,
	}
}

func FuncParamMock(k any, v any) *tql.Node {
	task := tql.NewTask()
	node := tql.NewNode(task)
	node.SetInflight(tql.NewRecord(k, v))
	return node
}

type paramMock struct {
	back func(name string) (any, error)
}

func (mock *paramMock) Get(name string) (any, error) {
	return mock.back(name)
}
