package tql

import (
	"bytes"
	"testing"
	"time"

	"github.com/machbase/neo-client/api"
	"github.com/machbase/neo-server/v8/mods/codec/facility"
	"github.com/machbase/neo-server/v8/mods/nums"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/encoding/korean"
)

type convVolatileFileWriterMock struct{}

func (convVolatileFileWriterMock) VolatileFilePrefix() string                                     { return "/tmp" }
func (convVolatileFileWriterMock) VolatileFileWrite(name string, data []byte, deadline time.Time) {}

func TestConvString(t *testing.T) {
	runTest := func(input any) string {
		ret, err := convString([]any{input}, 0, "test", "TestConvString")
		if err != nil {
			t.Logf("Fail TestConvString, %s", err.Error())
			t.Fail()
		}
		return ret
	}
	require.Equal(t, "text", runTest("text"))
	require.Equal(t, "123.456", runTest(123.456))
	require.Equal(t, "123", runTest(123))
	require.Equal(t, "123", runTest(int16(123)))
	require.Equal(t, "123", runTest(int32(123)))
	require.Equal(t, "123", runTest(int64(123)))
	require.Equal(t, "true", runTest(true))
	require.Equal(t, "false", runTest(false))
}

func TestConvInt(t *testing.T) {
	runTest := func(input any) int {
		ret, err := convInt([]any{input}, 0, "test", "TestConvInt")
		if err != nil {
			t.Logf("Fail TestConvInt, %s", err.Error())
			t.Fail()
		}
		return ret
	}
	require.Equal(t, 123, runTest(123))
	require.Equal(t, 123, runTest(int16(123)))
	require.Equal(t, 123, runTest(int32(123)))
	require.Equal(t, 123, runTest(int64(123)))
	require.Equal(t, 123, runTest(123.4456))
	require.Equal(t, 123, runTest("123"))
}

func TestConvTimeLocation(t *testing.T) {
	runTest := func(input any) string {
		ret, err := convTimeLocation([]any{input}, 0, "test", "TestConvTimeLocation")
		if err != nil {
			t.Logf("Fail TestConvTimeLocation, %s", err.Error())
			t.Fail()
		}
		return ret.String()
	}
	require.Equal(t, "UTC", runTest("UTC"))
	require.Equal(t, "Africa/Abidjan", runTest("GMT"))
	require.Equal(t, "Europe/London", runTest("Europe/London"))
	require.Equal(t, "Asia/Seoul", runTest("KST"))
	require.Equal(t, "Asia/Seoul", runTest("Asia/Seoul"))
	require.Equal(t, "Africa/Cairo", runTest("EEST"))
	require.Equal(t, "Africa/Cairo", runTest("Africa/Cairo"))
}

func TestConvMisc(t *testing.T) {
	reader := bytes.NewBufferString("input")
	retReader, err := convInputStream([]any{reader}, 0, "test", "reader")
	require.NoError(t, err)
	require.Same(t, reader, retReader)
	_, err = convInputStream([]any{}, 0, "test", "reader")
	require.EqualError(t, err, "f(test) invalid number of args; expect:1, actual:0")
	_, err = convInputStream([]any{123}, 0, "test", "reader")
	require.EqualError(t, err, "f(test) arg(0) should be reader, but int")

	writer := &bytes.Buffer{}
	retWriter, err := convOutputStream([]any{writer}, 0, "test", "writer")
	require.NoError(t, err)
	require.Same(t, writer, retWriter)
	_, err = convOutputStream([]any{"x"}, 0, "test", "writer")
	require.EqualError(t, err, "f(test) arg(0) should be writer, but string")

	vfw := convVolatileFileWriterMock{}
	retVfw, err := convVolatileFileWriter([]any{vfw}, 0, "test", "volatile")
	require.NoError(t, err)
	require.NotNil(t, retVfw)
	_, err = convVolatileFileWriter([]any{writer}, 0, "test", "volatile")
	require.EqualError(t, err, "f(test) arg(0) should be volatile, but *bytes.Buffer")

	logger := facility.DiscardLogger
	retLogger, err := convLogger([]any{logger}, 0, "test", "logger")
	require.NoError(t, err)
	require.Same(t, logger, retLogger)
	_, err = convLogger([]any{writer}, 0, "test", "logger")
	require.EqualError(t, err, "f(test) arg(0) should be logger, but *bytes.Buffer")

	latlon := nums.NewLatLon(37.5665, 126.9780)
	retLatLon, err := convLatLon([]any{latlon}, 0, "test", "latlon")
	require.NoError(t, err)
	require.Same(t, latlon, retLatLon)
	_, err = convLatLon([]any{"seoul"}, 0, "test", "latlon")
	require.EqualError(t, err, "f(test) arg(0) should be latlon, but string")

	retCharset, err := convCharset([]any{korean.EUCKR}, 0, "test", "charset")
	require.NoError(t, err)
	require.NotNil(t, retCharset)
	_, err = convCharset([]any{"euckr"}, 0, "test", "charset")
	require.EqualError(t, err, "f(test) arg(0) should be charset, but string")

	retFloat32, err := convFloat32([]any{float64(1.25)}, 0, "test", "float32")
	require.NoError(t, err)
	require.Equal(t, float32(1.25), retFloat32)
	v := 2.5
	retFloat32, err = convFloat32([]any{&v}, 0, "test", "float32")
	require.NoError(t, err)
	require.Equal(t, float32(2.5), retFloat32)
	retFloat32, err = convFloat32([]any{"3.75"}, 0, "test", "float32")
	require.NoError(t, err)
	require.Equal(t, float32(3.75), retFloat32)
	_, err = convFloat32([]any{"bad"}, 0, "test", "float32")
	require.EqualError(t, err, "f(test) arg(0) should be float32, but string")

	retDataType, err := convDataType([]any{"s"}, 0, "test", "datatype")
	require.NoError(t, err)
	require.Equal(t, api.DataType("s"), retDataType)
	dt := "d"
	retDataType, err = convDataType([]any{&dt}, 0, "test", "datatype")
	require.NoError(t, err)
	require.Equal(t, api.DataType("d"), retDataType)
	_, err = convDataType([]any{"too-long"}, 0, "test", "datatype")
	require.EqualError(t, err, "f(test) arg(0) should be a data type")
	_, err = convDataType([]any{123}, 0, "test", "datatype")
	require.EqualError(t, err, "f(test) arg(0) should be datatype, but int")
}
