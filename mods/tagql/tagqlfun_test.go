package tagql

import (
	"fmt"
	"testing"
	"time"

	"github.com/d5/tengo/v2/require"
	"github.com/machbase/neo-server/mods/expression"
)

// TestCase
type MapFuncTestCase struct {
	input     string
	params    expression.Parameters
	expect    *ExecutionParam
	expectErr string
}

func TestMapFunc_MODTIME(t *testing.T) {
	MapFuncTestCase{
		input:     `MODTIME('x', 'y')`,
		params:    FuncParamMock(1, ""),
		expectErr: "f(MODTIME) invalid number of args (n:4)",
	}.run(t)
	MapFuncTestCase{
		input:     `MODTIME('100ms')`,
		params:    FuncParamMock(123456, ""),
		expectErr: "f(MODTIME) K should be time, but float64",
	}.run(t)
	MapFuncTestCase{
		input:     `MODTIME('100ms')`,
		params:    FuncParamMock(time.Unix(100, 200300400), ""),
		expectErr: "f(MODTIME) V should be []any, but string",
	}.run(t)
	MapFuncTestCase{
		input:     `MODTIME('100x')`,
		params:    FuncParamMock(time.Unix(100, 200300400), []any{0, 1, 2, 3}),
		expectErr: `f(MODTIME) 1st arg should be duration, time: unknown unit "x" in duration "100x"`,
	}.run(t)
	MapFuncTestCase{
		input:  `MODTIME('100ms')`,
		params: FuncParamMock(time.Unix(100, 200300400), []any{0, 1, 2, 3}),
		expect: &ExecutionParam{K: time.Unix(100, 200000000), V: []any{time.Unix(100, 200300400), 0, 1, 2, 3}},
	}.run(t)
	MapFuncTestCase{
		input:  `MODTIME('100us')`,
		params: FuncParamMock(time.Unix(100, 200300400), []any{0, 1, 2, 3}),
		expect: &ExecutionParam{K: time.Unix(100, 200300000), V: []any{time.Unix(100, 200300400), 0, 1, 2, 3}},
	}.run(t)
}

func TestMapFunc_PUSHKEY(t *testing.T) {
	extime := time.Unix(123, 0)
	MapFuncTestCase{
		input:     `PUSHKEY()`,
		params:    FuncParamMock(extime, []any{1, 2, 3}),
		expectErr: "f(PUSHKEY) invalid number of args (n:2)",
	}.run(t)
	MapFuncTestCase{
		input:     `PUSHKEY('err')`,
		params:    FuncParamMock(extime, []int{1, 2, 3}),
		expectErr: "f(PUSHKEY) V should be []any, but []int",
	}.run(t)
	MapFuncTestCase{
		input:  `PUSHKEY('sam')`,
		params: FuncParamMock(extime, []any{1, 2, 3}),
		expect: &ExecutionParam{K: "sam", V: []any{extime, 1, 2, 3}},
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
		expect: &ExecutionParam{K: 1, V: []any{2, 3}},
	}.run(t)
	MapFuncTestCase{
		input:  `POPKEY()`,
		params: FuncParamMock("x", []any{[]int{10, 11, 12}, []int{20, 21, 22}, []int{30, 31, 32}}),
		expect: &ExecutionParam{K: []int{10, 11, 12}, V: []any{[]int{20, 21, 22}, []int{30, 31, 32}}},
	}.run(t)
	MapFuncTestCase{
		input:     `POPKEY(0)`,
		params:    FuncParamMock("x", []int{1, 2, 3}),
		expectErr: "f(POPKEY) V should be []any or [][]any, but []int",
	}.run(t)
	MapFuncTestCase{
		input:  `POPKEY(1)`,
		params: FuncParamMock("x", []any{"K", 1, 2}),
		expect: &ExecutionParam{K: 1, V: []any{"K", 2}},
	}.run(t)
}

func TestMapFunc_FILTER(t *testing.T) {
	MapFuncTestCase{
		input:  `FILTER(10<100)`,
		params: FuncParamMock("x", []any{1, 2, 3}),
		expect: &ExecutionParam{K: "x", V: []any{1, 2, 3}},
	}.run(t)
	MapFuncTestCase{
		input:  `FILTER(10>100)`,
		params: FuncParamMock("x", []any{1, 2, 3}),
		expect: nil,
	}.run(t)
	MapFuncTestCase{
		input:  `FILTER(len(V) > 2)`,
		params: FuncParamMock("x", []any{1, 2, 3}),
		expect: &ExecutionParam{K: "x", V: []any{1, 2, 3}},
	}.run(t)
	MapFuncTestCase{
		input:  `FILTER(len(V) > 4)`,
		params: FuncParamMock("x", []any{1, 2, 3}),
		expect: nil,
	}.run(t)
}

func (tc MapFuncTestCase) run(t *testing.T) {
	strExpr := normalizeMapFuncExpr(tc.input)
	msg := fmt.Sprintf("TestCase %s => %s", tc.input, strExpr)
	expr, err := expression.NewWithFunctions(strExpr, mapFunctions)
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
	if retParam, ok := ret.(*ExecutionParam); !ok {
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
	return &ExecutionParam{K: k, V: v}
}

type paramMock struct {
	back func(name string) (any, error)
}

func (mock *paramMock) Get(name string) (any, error) {
	return mock.back(name)
}
