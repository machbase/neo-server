package tql_test

import (
	"testing"
	"time"

	"github.com/machbase/neo-server/mods/tql"
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
	tql.StandardTimeNow = func() time.Time { return tick }
	// invalid number of args
	FunctionTestCase{f: node.Function("time"),
		args:      []any{},
		expectErr: "f(time) invalid number of args; expect:1, actual:0",
	}.run(t)
	// first args should be time, but %s",
	FunctionTestCase{f: node.Function("time"),
		args:      []any{"last"},
		expectErr: "invalid time expression 'last'",
	}.run(t)
	// first args should be time, but
	FunctionTestCase{f: node.Function("time"),
		args:      []any{true},
		expectErr: "invalid time expression 'true bool'",
	}.run(t)
	// f(time) second args should be time, but %s
	FunctionTestCase{f: node.Function("time"),
		args:      []any{"oned2h"},
		expectErr: "invalid time expression 'oned2h'",
	}.run(t)
	// f(time) second args should be time, but %s
	FunctionTestCase{f: node.Function("time"),
		args:      []any{"1d27h"},
		expectErr: "invalid time expression '1d27h'",
	}.run(t)
	// f(time) second args should be duration, but %s
	FunctionTestCase{f: node.Function("timeAdd"),
		args:      []any{tick, "-2x"},
		expectErr: "invalid delta expression '-2x string'",
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
		expectErr: "time: unknown unit \"x\" in duration \"-1x\"",
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

func TestRoundTime(t *testing.T) {
	node := tql.NewNode(tql.NewTask())
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
