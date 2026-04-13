package tql

import (
	"testing"

	"github.com/machbase/neo-server/v8/mods/codec/opts"
	"github.com/stretchr/testify/require"
)

type chartOptionRecorder struct {
	value string
}

func (r *chartOptionRecorder) SetChartOption(opt string) {
	r.value = opt
}

func TestNodeContextAccessors(t *testing.T) {
	node := NewNode(NewTask())
	ctx := node.GetContext()
	require.NotNil(t, ctx)
	require.Same(t, node, ctx.node)

	require.Nil(t, node.GetRecordKey())
	require.Nil(t, node.GetRequestParam("missing"))

	node.SetInflight(NewRecord("key", []any{"value0", "value1"}))
	require.Equal(t, "key", node.GetRecordKey())

	value, err := node.GetRecordValue()
	require.NoError(t, err)
	require.Equal(t, []any{"value0", "value1"}, value)

	value, err = node.GetRecordValue(1)
	require.NoError(t, err)
	require.Equal(t, "value1", value)

	value, err = node.GetRecordValue(float32(0))
	require.NoError(t, err)
	require.Equal(t, "value0", value)

	value, err = node.GetRecordValue(float64(1))
	require.NoError(t, err)
	require.Equal(t, "value1", value)

	value, err = node.GetRecordValue("0")
	require.NoError(t, err)
	require.Equal(t, "value0", value)

	_, err = node.GetRecordValue("bad")
	require.Error(t, err)
	_, err = node.GetRecordValue(2)
	require.EqualError(t, err, "f(value) arg(0) 2 is out of range of the value(len:2) in ")

	node.SetInflight(NewRecord("scalar-key", 42))
	value, err = node.GetRecordValue(0)
	require.NoError(t, err)
	require.Equal(t, 42, value)
	_, err = node.GetRecordValue(1)
	require.EqualError(t, err, "f(value) arg(0) out of index value tuple in ")

	node.SetInflight(NewRecord("nil-key", nil))
	value, err = node.GetRecordValue()
	require.NoError(t, err)
	require.Nil(t, value)
}

func TestNodeArgsAndParams(t *testing.T) {
	task := NewTask()
	task.params = map[string][]string{
		"one":   {"1"},
		"multi": {"a", "b"},
	}
	task.argValues = []any{"arg0", 99}
	node := NewNode(task)

	require.Equal(t, "1", node.GetRequestParam("one"))
	require.Equal(t, []string{"a", "b"}, node.GetRequestParam("multi"))
	require.Nil(t, node.GetRequestParam("missing"))

	ret, err := node.fmArgsParam()
	require.NoError(t, err)
	require.Equal(t, []any{"arg0", 99}, ret)

	ret, err = node.fmArgsParam(1)
	require.NoError(t, err)
	require.Equal(t, 99, ret)

	ret, err = node.fmArgsParam("0")
	require.NoError(t, err)
	require.Equal(t, "arg0", ret)

	_, err = node.fmArgsParam(3)
	require.EqualError(t, err, "f(arg) arg(0) 3 is out of range of the arg(len:2)")

	node.name = "FAKE()"
	ret, err = node.fmArgsParam(0)
	require.NoError(t, err)
	raw, ok := ret.(*rawdata)
	require.True(t, ok)
	require.Equal(t, "arg0", raw.data)

	emptyTask := NewTask()
	emptyNode := NewNode(emptyTask)
	ret, err = emptyNode.fmArgsParam()
	require.NoError(t, err)
	require.Equal(t, []any{}, ret)
}

func TestNodeOption(t *testing.T) {
	node := NewNode(NewTask())
	node.name = "CHART()"
	ret, err := node.fmOption("legend")
	require.NoError(t, err)
	option, ok := ret.(opts.Option)
	require.True(t, ok)
	recorder := &chartOptionRecorder{}
	option(recorder)
	require.Equal(t, "legend", recorder.value)

	_, err = node.fmOption(123)
	require.EqualError(t, err, "invalid use of option()")

	node.name = "TEXT()"
	_, err = node.fmOption("legend")
	require.EqualError(t, err, "invalid use of option()")
}
