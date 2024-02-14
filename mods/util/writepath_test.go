package util_test

import (
	"testing"

	. "github.com/machbase/neo-server/mods/util"
	"github.com/stretchr/testify/require"
)

type CaseWritePath struct {
	path   string
	expect *WritePath
}

func TestWritePath(t *testing.T) {
	testWritePath(t, CaseWritePath{"table_1", &WritePath{"TABLE_1", "", ""}}, "")
	testWritePath(t, CaseWritePath{"table_1:csv", &WritePath{"TABLE_1", "csv", ""}}, "")
	testWritePath(t, CaseWritePath{"table_1:json", &WritePath{"TABLE_1", "json", ""}}, "")
	testWritePath(t, CaseWritePath{"table_1:csv:GZIP", &WritePath{"TABLE_1", "csv", "gzip"}}, "")
}

func testWritePath(t *testing.T, tc CaseWritePath, expectedErr string) {
	p, err := ParseWritePath(tc.path)
	if expectedErr != "" {
		require.NotNil(t, err)
		require.Equal(t, err.Error(), expectedErr)
		return
	}
	require.Nil(t, err)
	require.NotNil(t, p)
	require.Equal(t, tc.expect.Table, p.Table)
	require.Equal(t, tc.expect.Format, p.Format)
	require.Equal(t, tc.expect.Compress, p.Compress)
}
