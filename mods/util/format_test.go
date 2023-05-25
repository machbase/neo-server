package util_test

import (
	"testing"

	"github.com/machbase/neo-server/mods/util"
	"github.com/stretchr/testify/require"
)

type NumberTestCase struct {
	v      int64
	expect string
}

func (tc NumberTestCase) run(t *testing.T) {
	s := util.NumberFormat(tc.v)
	require.Equal(t, tc.expect, s)
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
		tc.run(t)
	}
}
