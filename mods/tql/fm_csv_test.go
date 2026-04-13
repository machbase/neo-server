package tql

import (
	"testing"

	"github.com/machbase/neo-client/api"
	"github.com/stretchr/testify/require"
)

func TestCsvHelpers(t *testing.T) {
	src := &csvSource{}
	src.SetHeading(true)
	require.True(t, src.hasHeader)
	src.SetHeading(false)
	require.False(t, src.hasHeader)

	node := NewNode(NewTask())
	ret, err := node.fmLogProgress()
	require.NoError(t, err)
	require.Equal(t, PrintProgressCount(500000), ret)

	ret, err = node.fmLogProgress(float64(25))
	require.NoError(t, err)
	require.Equal(t, PrintProgressCount(25), ret)

	_, err = node.fmLogProgress("bad")
	require.EqualError(t, err, "f(printProgressCount) argument should be int")

	ret, err = node.fmField(float64(1), "string", "name")
	require.NoError(t, err)
	field, ok := ret.(*columnOpt)
	require.True(t, ok)
	require.Equal(t, 1, field.idx)
	require.Equal(t, "name", field.label)
	require.Equal(t, api.DataTypeAny, field.dataType.dataType())

	ret, err = node.fmField(float64(2), &doubleOpt{}, "score")
	require.NoError(t, err)
	field = ret.(*columnOpt)
	require.Equal(t, api.DataTypeFloat64, field.dataType.dataType())

	_, err = node.fmField("bad", "string", "name")
	require.EqualError(t, err, "f(field) first argument should be int")
	_, err = node.fmField(float64(1), 10, "name")
	require.EqualError(t, err, "f(field) second argument should be data type")
	_, err = node.fmField(float64(1), "string", 10)
	require.EqualError(t, err, "f(field) third argument should be label")

	ret, err = node.fmCol(float64(3), "bool", "flag")
	require.NoError(t, err)
	field = ret.(*columnOpt)
	require.Equal(t, 3, field.idx)
	require.Equal(t, "flag", field.label)
}

func TestCsvTypeHelpers(t *testing.T) {
	node := NewNode(NewTask())

	ret, err := node.fmStringType()
	require.NoError(t, err)
	require.Equal(t, api.DataTypeString, ret.(*stringOpt).dataType())

	ret, err = node.fmDoubleType()
	require.NoError(t, err)
	require.Equal(t, api.DataTypeFloat64, ret.(*doubleOpt).dataType())

	ret, err = node.fmBoolType()
	require.NoError(t, err)
	require.Equal(t, api.DataTypeBoolean, ret.(*boolOpt).dataType())

	ret, err = node.fmDatetimeType("ns")
	require.NoError(t, err)
	require.Equal(t, int64(1), ret.(*epochTimeOpt).unit)

	ret, err = node.fmDatetimeType("us")
	require.NoError(t, err)
	require.Equal(t, int64(1000), ret.(*epochTimeOpt).unit)

	ret, err = node.fmDatetimeType("ms")
	require.NoError(t, err)
	require.Equal(t, int64(1000000), ret.(*epochTimeOpt).unit)

	ret, err = node.fmDatetimeType("s")
	require.NoError(t, err)
	require.Equal(t, int64(1000000000), ret.(*epochTimeOpt).unit)

	ret, err = node.fmDatetimeType("2006-01-02 15:04:05", "UTC")
	require.NoError(t, err)
	dt := ret.(*datetimeOpt)
	require.Equal(t, "2006-01-02 15:04:05", dt.timeformat)
	require.Equal(t, "UTC", dt.timeLocation.String())

	_, err = node.fmDatetimeType()
	require.EqualError(t, err, "f(datetime) invalid number of args; expect:2, actual:0")

	_, err = node.fmDatetimeType("2006-01-02", "Not/AZone")
	require.Error(t, err)
}
