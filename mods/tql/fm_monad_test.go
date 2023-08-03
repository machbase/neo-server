package tql_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/machbase/neo-server/mods/expression"
	"github.com/machbase/neo-server/mods/tql"
	"github.com/stretchr/testify/require"
)

// TestCase
type MapFuncTestCase struct {
	input     string
	params    expression.Parameters
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
	ctx := tql.NewSubContext(nil)
	MapFuncTestCase{
		input:  `TAKE(1)`,
		params: FuncParamMock("sam", []any{1, 2, 3}),
		expect: ctx.NewRecord("sam", []any{1, 2, 3}),
	}.run(t)
}

func TestMapFunc_PUSHKEY(t *testing.T) {
	ctx := tql.NewSubContext(nil)
	extime := time.Unix(123, 0)
	MapFuncTestCase{
		input:     `PUSHKEY()`,
		params:    FuncParamMock(extime, []any{1, 2, 3}),
		expectErr: "f(PUSHKEY) invalid number of args; expect:4, actual:3",
	}.run(t)
	MapFuncTestCase{
		input:     `PUSHKEY('err')`,
		params:    FuncParamMock(extime, []int{1, 2, 3}),
		expectErr: "f(PUSHKEY) V should be []any, but []int",
	}.run(t)
	MapFuncTestCase{
		input:  `PUSHKEY('sam')`,
		params: FuncParamMock(extime, []any{1, 2, 3}),
		expect: ctx.NewRecord("sam", []any{extime, 1, 2, 3}),
	}.run(t)
	tick := time.Now()
	tick100ms := time.Unix(0, (tick.UnixNano()/100000000)*100000000)
	MapFuncTestCase{
		input:  `PUSHKEY(roundTime(K, '100ms'))`,
		params: FuncParamMock(tick, []any{"v"}),
		expect: ctx.NewRecord(tick100ms, []any{tick, "v"}),
	}.run(t)
}

func TestMapFunc_POPKEY(t *testing.T) {
	ctx := tql.NewSubContext(nil)
	MapFuncTestCase{
		input:     `POPKEY()`,
		params:    FuncParamMock("x", []int{1, 2, 3}),
		expectErr: "f(POPKEY) V should be []any or [][]any, but []int",
	}.run(t)
	MapFuncTestCase{
		input:  `POPKEY()`,
		params: FuncParamMock("x", []any{1, 2, 3}),
		expect: ctx.NewRecord(1, []any{2, 3}),
	}.run(t)
	MapFuncTestCase{
		input:  `POPKEY()`,
		params: FuncParamMock("x", []any{[]int{10, 11, 12}, []int{20, 21, 22}, []int{30, 31, 32}}),
		expect: ctx.NewRecord([]int{10, 11, 12}, []any{[]int{20, 21, 22}, []int{30, 31, 32}}),
	}.run(t)
	MapFuncTestCase{
		input:     `POPKEY(0)`,
		params:    FuncParamMock("x", []int{1, 2, 3}),
		expectErr: "f(POPKEY) V should be []any or [][]any, but []int",
	}.run(t)
	MapFuncTestCase{
		input:  `POPKEY(1)`,
		params: FuncParamMock("x", []any{"K", 1, 2}),
		expect: ctx.NewRecord(1, []any{"K", 2}),
	}.run(t)
}

func TestMapFunc_FILTER(t *testing.T) {
	ctx := tql.NewSubContext(nil)
	MapFuncTestCase{
		input:  `FILTER(10<100)`,
		params: FuncParamMock("x", []any{1, 2, 3}),
		expect: ctx.NewRecord("x", []any{1, 2, 3}),
	}.run(t)
	MapFuncTestCase{
		input:  `FILTER(10>100)`,
		params: FuncParamMock("x", []any{1, 2, 3}),
		expect: nil,
	}.run(t)
	MapFuncTestCase{
		input:  `FILTER(K == 'x')`,
		params: FuncParamMock("x", []any{1, 2, 3}),
		expect: ctx.NewRecord("x", []any{1, 2, 3}),
	}.run(t)
	MapFuncTestCase{
		input:  `FILTER(K != 'x')`,
		params: FuncParamMock("x", []any{1, 2, 3}),
		expect: nil,
	}.run(t)
	MapFuncTestCase{
		input:  `FILTER(K != 'y')`,
		params: FuncParamMock("x", []any{1, 2, 3}),
		expect: ctx.NewRecord("x", []any{1, 2, 3}),
	}.run(t)
	MapFuncTestCase{
		input:  `FILTER(len(V) > 2)`,
		params: FuncParamMock("x", []any{1, 2, 3}),
		expect: ctx.NewRecord("x", []any{1, 2, 3}),
	}.run(t)
	MapFuncTestCase{
		input:  `FILTER(len(V) > 4)`,
		params: FuncParamMock("x", []any{1, 2, 3}),
		expect: nil,
	}.run(t)
	MapFuncTestCase{
		input:  `FILTER(element(V, 0) >= 1)`,
		params: FuncParamMock("x", []any{1, 2, 3}),
		expect: ctx.NewRecord("x", []any{1, 2, 3}),
	}.run(t)
	MapFuncTestCase{
		input:  `FILTER(element(V, 0) > 0)`,
		params: FuncParamMock("x", []any{1, 2, 3}),
		expect: ctx.NewRecord("x", []any{1, 2, 3}),
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
	task := tql.NewTask()
	msg := fmt.Sprintf("TestCase %s", tc.input)
	expr, err := task.Parse(tc.input)
	require.Nil(t, err, msg)
	require.NotNil(t, expr, msg)

	ret, err := expr.Eval(tc.params)
	if tc.expectErr != "" {
		require.NotNil(t, err, msg)
		require.Equal(t, tc.expectErr, err.Error(), msg)
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

func FuncParamMock(k any, v any) expression.Parameters {
	ctx := &tql.SubContext{}
	return ctx.NewRecord(k, v)
}

type paramMock struct {
	back func(name string) (any, error)
}

func (mock *paramMock) Get(name string) (any, error) {
	return mock.back(name)
}
