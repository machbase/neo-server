package util_test

import (
	"testing"

	"github.com/machbase/neo-server/v8/mods/util"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"
)

type NumberTestCase struct {
	v      int64
	expect string
}

func TestNumber(t *testing.T) {
	tcs := []NumberTestCase{
		{0, "0"},
		{1, "1"},
		{12, "12"},
		{123, "123"},
		{1234, "1,234"},
		{123456789, "123,456,789"},
		{-0, "0"},
		{-1, "-1"},
		{-12, "-12"},
		{-123, "-123"},
		{-1234, "-1,234"},
		{-123456789, "-123,456,789"},
	}

	for _, tc := range tcs {
		s := util.NumberFormat(tc.v)
		require.Equal(t, tc.expect, s)
	}
}

func TestBytesUnit(t *testing.T) {
	var ret string
	ret = util.BytesUnit(512, language.German)
	require.Equal(t, "512,0", ret)
	ret = util.BytesUnit(1024+512, language.Greek)
	require.Equal(t, "1,5 KB", ret)
	ret = util.BytesUnit((1024+512)*1024, language.BritishEnglish)
	require.Equal(t, "1.5 MB", ret)
	ret = util.BytesUnit((1024+512)*1024*1024, language.BritishEnglish)
	require.Equal(t, "1.5 GB", ret)
	ret = util.BytesUnit((1024+512)*1024*1024*1024, language.BritishEnglish)
	require.Equal(t, "1.5 TB", ret)
}
