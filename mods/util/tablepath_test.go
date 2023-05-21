package util_test

import (
	"testing"

	. "github.com/machbase/neo-server/mods/util"
	"github.com/stretchr/testify/require"
)

type CaseTagPath struct {
	path   string
	expect *TagPath
}

func TestTagPath(t *testing.T) {
	testTagPath(t, CaseTagPath{
		path:   "table_1",
		expect: nil,
	})
	testTagPath(t, CaseTagPath{
		path:   "table_1/",
		expect: nil,
	})
	testTagPath(t, CaseTagPath{
		path: "table_1//",
		expect: &TagPath{
			Table: "TABLE_1",
			Tag:   "/",
			Term:  TagPathTerm{Column: "VALUE"}},
	})
	testTagPath(t, CaseTagPath{
		path: "table_1/tag_1",
		expect: &TagPath{
			Table: "TABLE_1",
			Tag:   "tag_1",
			Term:  TagPathTerm{Column: "VALUE"}},
	})
	testTagPath(t, CaseTagPath{
		path: "table_1/tag_1#v1",
		expect: &TagPath{
			Table: "TABLE_1",
			Tag:   "tag_1",
			Term:  TagPathTerm{Column: "V1"}},
	})
	testTagPath(t, CaseTagPath{
		path: "table_1/tag_1#kalman(value)",
		expect: &TagPath{
			Table: "TABLE_1",
			Tag:   "tag_1",
			Term:  TagPathTerm{Func: "KALMAN", Args: []TagPathTerm{{Column: "VALUE"}}}},
	})
	testTagPath(t, CaseTagPath{
		path: "table_1/tag_1#fft( kalman(value) )",
		expect: &TagPath{
			Table: "TABLE_1",
			Tag:   "tag_1",
			Term: TagPathTerm{
				Func: "FFT",
				Args: []TagPathTerm{
					{
						Func: "KALMAN",
						Args: []TagPathTerm{
							{
								Column: "VALUE",
							},
						},
					},
				},
			},
		},
	})
}

func testTagPath(t *testing.T, tc CaseTagPath) {
	p, err := ParseTagPath(tc.path)
	if tc.expect == nil {
		require.NotNil(t, err)
		require.Equal(t, err.Error(), "invalid syntax")
		return
	}
	require.Nil(t, err)
	require.NotNil(t, p)
	require.Equal(t, tc.expect.Table, p.Table)
	require.Equal(t, tc.expect.Tag, p.Tag)
	require.True(t, tc.expect.Term.IsEqual(&p.Term))
}

type CaseWritePath struct {
	path   string
	expect *WritePath
}

func TestWritePath(t *testing.T) {
	testWritePath(t, CaseWritePath{"table_1", &WritePath{"TABLE_1", "", "", ""}}, "")
	testWritePath(t, CaseWritePath{"table_1:csv", &WritePath{"TABLE_1", "csv", "", ""}}, "")
	testWritePath(t, CaseWritePath{"table_1:json", &WritePath{"TABLE_1", "json", "", ""}}, "")
	testWritePath(t, CaseWritePath{"table_1:csv:GZIP", &WritePath{"TABLE_1", "csv", "", "gzip"}}, "")
	testWritePath(t, CaseWritePath{"table_1:json:trans1:gzip", &WritePath{"TABLE_1", "json", "trans1", "gzip"}}, "")
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
	require.Equal(t, tc.expect.Transform, p.Transform)
	require.Equal(t, tc.expect.Compress, p.Compress)
}
