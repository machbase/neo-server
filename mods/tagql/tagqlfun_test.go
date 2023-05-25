package tagql

import (
	"errors"
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
		input:     `MODTIME(K, V, '100ms')`,
		params:    FuncParamMock(123456, ""),
		expectErr: "f(MODTIME) 1st arg should be time, but float64",
	}.run(t)
	MapFuncTestCase{
		input:     `MODTIME(K, V, '100ms')`,
		params:    FuncParamMock(time.Unix(100, 200300400), ""),
		expectErr: "f(MODTIME) 2nd arg should be []any, but string",
	}.run(t)
	MapFuncTestCase{
		input:     `MODTIME(K, V, '100x')`,
		params:    FuncParamMock(time.Unix(100, 200300400), []any{0, 1, 2, 3}),
		expectErr: `f(MODTIME) 3rd arg should be duration, time: unknown unit "x" in duration "100x"`,
	}.run(t)
	MapFuncTestCase{
		input:  `MODTIME(K, V, '100ms')`,
		params: FuncParamMock(time.Unix(100, 200300400), []any{0, 1, 2, 3}),
		expect: &ExecutionParam{K: time.Unix(100, 200000000), V: []any{time.Unix(100, 200300400), 0, 1, 2, 3}},
	}.run(t)
	MapFuncTestCase{
		input:  `MODTIME(K, V, '100us')`,
		params: FuncParamMock(time.Unix(100, 200300400), []any{0, 1, 2, 3}),
		expect: &ExecutionParam{K: time.Unix(100, 200300000), V: []any{time.Unix(100, 200300400), 0, 1, 2, 3}},
	}.run(t)
}

func TestMapFunc_PUSHKEY(t *testing.T) {
	MapFuncTestCase{
		input:     `PUSHKEY(K, V)`,
		params:    FuncParamMock(time.Unix(123, 0), []any{1, 2, 3}),
		expectErr: "f(PUSHKEY) invalid number of args (n:2)",
	}.run(t)
	MapFuncTestCase{
		input:     `PUSHKEY('err', K, V)`,
		params:    FuncParamMock(time.Unix(123, 0), []int{1, 2, 3}),
		expectErr: "f(PUSHKEY) 3rd arg should be []any, but time.Time",
	}.run(t)
	MapFuncTestCase{
		input:  `PUSHKEY('sam', K, V)`,
		params: FuncParamMock(time.Unix(123, 0), []any{1, 2, 3}),
		expect: &ExecutionParam{K: "sam", V: []any{time.Unix(123, 0), 1, 2, 3}},
	}.run(t)
}

func TestMapFunc_POPKEY(t *testing.T) {
	MapFuncTestCase{
		input:     `POPKEY(K, V)`,
		params:    FuncParamMock("x", []any{1, 2, 3}),
		expectErr: "f(POPKEY) invalid number of args (n:2)",
	}.run(t)
	MapFuncTestCase{
		input:     `POPKEY(V)`,
		params:    FuncParamMock("x", []int{1, 2, 3}),
		expectErr: "f(POPKEY) arg should be []any or [][]any, but []int",
	}.run(t)
	MapFuncTestCase{
		input:     `POPKEY(V)`,
		params:    FuncParamMock("x", []any{"K", 1, 2}),
		expectErr: "f(POPKEY) invalid number of args (n:3)",
	}.run(t)
}

func (tc MapFuncTestCase) run(t *testing.T) {
	msg := fmt.Sprintf("TestCase %s", tc.input)
	expr, err := expression.NewWithFunctions(tc.input, mapFunctions)
	require.Nil(t, err, msg)
	require.NotNil(t, expr, msg)

	ret, err := expr.Eval(tc.params)
	if tc.expectErr != "" {
		require.NotNil(t, err, msg)
		require.Equal(t, tc.expectErr, err.Error(), msg)
		return
	}
	require.Nil(t, err, msg)
	require.NotNil(t, ret, msg)

	if tc.expect == nil {
		require.Nil(t, ret)
		return
	}
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
	return &paramMock{
		back: func(name string) (any, error) {
			switch name {
			case "K", "k":
				return k, nil
			case "V", "v":
				return v, nil
			default:
				return nil, errors.New("unknown parameter")
			}
		},
	}
}

type paramMock struct {
	back func(name string) (any, error)
}

func (mock *paramMock) Get(name string) (any, error) {
	return mock.back(name)
}
