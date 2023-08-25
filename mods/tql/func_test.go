package tql_test

import (
	"fmt"
	"testing"
	"time"

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
	ret, err := tc.f(tc.args...)
	if tc.expectErr != "" {
		require.NotNil(t, err)
		require.Equal(t, tc.expectErr, err.Error())
		return
	}
	require.Nil(t, err)
	require.Equal(t, tc.expect, ret)
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

func TestRound(t *testing.T) {
	node := tql.NewNode(tql.NewTask())
	FunctionTestCase{f: node.Function("round"),
		args:      []any{},
		expectErr: "f(round) invalid number of args; expect:2, actual:0",
	}.run(t)
	FunctionTestCase{f: node.Function("round"),
		args:   []any{123.4567, 2.0},
		expect: float64(122),
	}.run(t)
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
		input:     `PUSHKEY('err')`,
		params:    FuncParamMock(extime, []int{1, 2, 3}),
		expectErr: "f(PUSHKEY) arg(0) Value should be array, but []int",
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