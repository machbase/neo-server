package maps_test

import (
	"testing"
	"time"

	"github.com/machbase/neo-server/mods/tql/fx"
	"github.com/machbase/neo-server/mods/tql/maps"
	"github.com/stretchr/testify/require"
)

type TestCase struct {
	f         func(args ...any) (any, error)
	args      []any
	expect    any
	expectErr string
}

func TestTime(t *testing.T) {
	tick := time.Now()
	maps.StandardTimeNow = func() time.Time { return tick }
	// invalid number of args
	TestCase{f: fx.GetFunction("time"),
		args:      []any{},
		expectErr: "f(time) invalid number of args; expect:1, actual:0",
	}.run(t)
	// first args should be time, but %s",
	TestCase{f: fx.GetFunction("time"),
		args:      []any{"last"},
		expectErr: "invalid time expression 'last'",
	}.run(t)
	// first args should be time, but
	TestCase{f: fx.GetFunction("time"),
		args:      []any{true},
		expectErr: "invalid time expression 'true bool'",
	}.run(t)
	// f(time) second args should be time, but %s
	TestCase{f: fx.GetFunction("time"),
		args:      []any{"oned2h"},
		expectErr: "invalid time expression 'oned2h'",
	}.run(t)
	// f(time) second args should be time, but %s
	TestCase{f: fx.GetFunction("time"),
		args:      []any{"1d27h"},
		expectErr: "invalid time expression '1d27h'",
	}.run(t)
	// f(time) second args should be duration, but %s
	TestCase{f: fx.GetFunction("timeAdd"),
		args:      []any{tick, "-2x"},
		expectErr: "invalid delta expression '-2x string'",
	}.run(t)
	TestCase{f: fx.GetFunction("time"),
		args:   []any{123456789.0},
		expect: time.Unix(0, 123456789),
	}.run(t)
	TestCase{f: fx.GetFunction("time"),
		args:   []any{"now"},
		expect: tick,
	}.run(t)
	TestCase{f: fx.GetFunction("timeAdd"),
		args:   []any{"now", "1s"},
		expect: tick.Add(1 * time.Second),
	}.run(t)
	TestCase{f: fx.GetFunction("timeAdd"),
		args:   []any{"now", "1d"},
		expect: tick.Add(24 * time.Hour),
	}.run(t)
	TestCase{f: fx.GetFunction("timeAdd"),
		args:   []any{"now", "-2d"},
		expect: tick.Add(-24 * 2 * time.Hour),
	}.run(t)
	TestCase{f: fx.GetFunction("timeAdd"),
		args:   []any{"now", "-1d12h"},
		expect: tick.Add(-24 * 1.5 * time.Hour),
	}.run(t)
	TestCase{f: fx.GetFunction("timeAdd"),
		args:   []any{"now", "-1d2h3m4s"},
		expect: tick.Add(-24*1*time.Hour - 2*time.Hour - 3*time.Minute - 4*time.Second),
	}.run(t)
	TestCase{f: fx.GetFunction("timeAdd"),
		args:   []any{"now-1s", 1000000000},
		expect: tick,
	}.run(t)
	TestCase{f: fx.GetFunction("timeAdd"),
		args:      []any{"now-1x", 1000000000},
		expectErr: "time: unknown unit \"x\" in duration \"-1x\"",
	}.run(t)
	// time.Time
	TestCase{f: fx.GetFunction("time"),
		args:   []any{tick},
		expect: tick,
	}.run(t)
	// *time.Time
	TestCase{f: fx.GetFunction("time"),
		args:   []any{&tick},
		expect: tick,
	}.run(t)
}

func TestRoundTime(t *testing.T) {
	TestCase{f: fx.GetFunction("roundTime"),
		args:   []any{time.Unix(123, 456789123), "1s"},
		expect: time.Unix(123, 000000000),
	}.run(t)
	TestCase{f: fx.GetFunction("roundTime"),
		args:   []any{time.Unix(123, 456789123), "10ms"},
		expect: time.Unix(123, 450000000),
	}.run(t)
	TestCase{f: fx.GetFunction("roundTime"),
		args:   []any{time.Unix(123, 456789123), "10us"},
		expect: time.Unix(123, 456780000),
	}.run(t)
	TestCase{f: fx.GetFunction("roundTime"),
		args:   []any{123456789123.0, "10us"},
		expect: time.Unix(123, 456780000),
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
