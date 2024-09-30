package tql

import (
	"testing"

	"github.com/stretchr/testify/require"
)

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
	require.Equal(t, "ETC/GMT", runTest("ETC/GMT"))
	require.Equal(t, "Asia/Seoul", runTest("KST"))
	require.Equal(t, "Asia/Seoul", runTest("Asia/Seoul"))
	require.Equal(t, "Africa/Cairo", runTest("EEST"))
	require.Equal(t, "Africa/Cairo", runTest("Africa/Cairo"))
}
