package fcom

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type TestCase struct {
	f         func(args ...any) (any, error)
	args      []any
	expect    any
	expectErr string
}

func TestLen(t *testing.T) {
	TestCase{f: to_len,
		args:   []any{[]string{"1", "2", "3", "4"}},
		expect: 4.0,
	}.run(t)
	TestCase{f: to_len,
		args:   []any{"1234"},
		expect: 4.0,
	}.run(t)
}

func TestElement(t *testing.T) {
	// invalid number of args
	TestCase{f: element,
		args:      []any{1, 2},
		expectErr: "f(element) invalud number of args (n:2)",
	}.run(t)
	// out of index
	TestCase{f: element,
		args:      []any{0.0, 1.0, 2.0, 3.0, 4.0, 5.0},
		expectErr: "f(element) out of index 5 / 5",
	}.run(t)
	// invalid index
	TestCase{f: element,
		args:      []any{0.0, 1.0, 2.0, 3.0, 4.0, "4"},
		expectErr: "f(element) index of element should be int, but string",
	}.run(t)
	// unsupported type
	TestCase{f: element,
		args:      []any{0.0, 1.0, 2.0, 3.0, time.Duration(1), 4},
		expectErr: "f(element) unsupported type time.Duration",
	}.run(t)
	TestCase{f: element,
		args:   []any{0.0, 1.0, 2.0, 3.0, 4.0, 1.0},
		expect: 1.0,
	}.run(t)
	TestCase{f: element,
		args:   []any{0.0, 1.0, 2.0, 3.0, 4.0, 4},
		expect: 4.0,
	}.run(t)
	TestCase{f: element,
		args:   []any{"abc", "bcd", "cde", "def", "efg", 4},
		expect: "efg",
	}.run(t)
	TestCase{f: element,
		args:   []any{"abc", "bcd", "cde", "def", true, 4},
		expect: true,
	}.run(t)
	TestCase{f: element,
		args:   []any{"abc", "bcd", "cde", "def", 123, 4},
		expect: 123.0,
	}.run(t)
	TestCase{f: element,
		args:   []any{"abc", "bcd", "cde", "def", int64(12345), 4},
		expect: 12345.0,
	}.run(t)
	TestCase{f: element,
		args:   []any{0.0, 1.0, 2.0, 3.0, time.Unix(123, int64(456)*int64(time.Millisecond)), 4},
		expect: 123.456 * 1000000000,
	}.run(t)
	tick1 := time.Unix(123, int64(456)*int64(time.Millisecond))
	TestCase{f: element,
		args:   []any{0.0, 1.0, 2.0, 3.0, &tick1, 4},
		expect: 123.456 * 1000000000,
	}.run(t)
}

func TestTime(t *testing.T) {
	tick := time.Now()
	standardTimeNow = func() time.Time { return tick }
	// invalid number of args
	TestCase{f: to_time,
		args:      []any{},
		expectErr: "f(time) invalid number of args (n:0)",
	}.run(t)
	// first args should be time, but %s",
	TestCase{f: to_time,
		args:      []any{"last"},
		expectErr: "f(time) first arg should be time, but last",
	}.run(t)
	// first args should be time, but
	TestCase{f: to_time,
		args:      []any{true},
		expectErr: "f(time) first arg should be time, but bool",
	}.run(t)
	// f(time) second args should be time, but %s
	TestCase{f: to_time,
		args:      []any{"oned2h"},
		expectErr: "f(time) first arg should be time, but oned2h",
	}.run(t)
	// f(time) second args should be time, but %s
	TestCase{f: to_time,
		args:      []any{"1d27h"},
		expectErr: "f(time) first arg should be time, but 1d27h",
	}.run(t)
	// f(time) second args should be duration, but %s
	TestCase{f: to_time,
		args:      []any{tick, "-2x"},
		expectErr: "f(time) second arg should be duration, but -2x",
	}.run(t)
	TestCase{f: to_time,
		args:   []any{123456789.0},
		expect: time.Unix(0, 123456789),
	}.run(t)
	TestCase{f: to_time,
		args:   []any{"now"},
		expect: tick,
	}.run(t)
	TestCase{f: to_time,
		args:   []any{"now", "1s"},
		expect: tick.Add(1 * time.Second),
	}.run(t)
	TestCase{f: to_time,
		args:   []any{"now", "1d"},
		expect: tick.Add(24 * time.Hour),
	}.run(t)
	TestCase{f: to_time,
		args:   []any{"now", "-2d"},
		expect: tick.Add(-24 * 2 * time.Hour),
	}.run(t)
	TestCase{f: to_time,
		args:   []any{"now", "-1d12h"},
		expect: tick.Add(-24 * 1.5 * time.Hour),
	}.run(t)
	TestCase{f: to_time,
		args:   []any{"now", "-1d2h3m4s"},
		expect: tick.Add(-24*1*time.Hour - 2*time.Hour - 3*time.Minute - 4*time.Second),
	}.run(t)
	// time.Time
	TestCase{f: to_time,
		args:   []any{tick},
		expect: tick,
	}.run(t)
	// *time.Time
	TestCase{f: to_time,
		args:   []any{&tick},
		expect: tick,
	}.run(t)

}

func TestRoundTime(t *testing.T) {
	TestCase{f: roundTime,
		args:   []any{time.Unix(123, 456789123), "1s"},
		expect: time.Unix(123, 000000000),
	}.run(t)
	TestCase{f: roundTime,
		args:   []any{time.Unix(123, 456789123), "10ms"},
		expect: time.Unix(123, 450000000),
	}.run(t)
	TestCase{f: roundTime,
		args:   []any{time.Unix(123, 456789123), "10us"},
		expect: time.Unix(123, 456780000),
	}.run(t)
	TestCase{f: roundTime,
		args:   []any{123456789123.0, "10us"},
		expect: time.Unix(123, 456780000),
	}.run(t)
}

func TestRound(t *testing.T) {
	TestCase{f: round,
		args:      []any{},
		expectErr: "f(round) invalid args 'round(int, int)', (n:0)",
	}.run(t)
	TestCase{f: round,
		args:   []any{123.4567, 2.0},
		expect: float64(122),
	}.run(t)
}

func (tc TestCase) run(t *testing.T) {
	ret, err := tc.f(tc.args...)
	if tc.expectErr != "" {
		require.NotNil(t, err)
		require.Equal(t, tc.expectErr, err.Error())
		return
	}
	require.Nil(t, err)
	require.Equal(t, tc.expect, ret)
}
