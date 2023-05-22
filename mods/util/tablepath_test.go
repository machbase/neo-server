package util_test

import (
	"testing"

	. "github.com/machbase/neo-server/mods/util"
	"github.com/machbase/neo-server/mods/util/expression"
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
			Field: TagPathField{Columns: []string{"VALUE"}}},
	})
	testTagPath(t, CaseTagPath{
		path: "table_1/tag_1",
		expect: &TagPath{
			Table: "TABLE_1",
			Tag:   "tag_1",
			Field: TagPathField{Columns: []string{"VALUE"}}},
	})
	testTagPath(t, CaseTagPath{
		path: "table_1/tag_1#v1",
		expect: &TagPath{
			Table: "TABLE_1",
			Tag:   "tag_1",
			Field: TagPathField{Columns: []string{"v1"}}},
	})
	testTagPath(t, CaseTagPath{
		path: "table_1/tag_1#v1 + v2",
		expect: &TagPath{
			Table: "TABLE_1",
			Tag:   "tag_1",
			Field: TagPathField{Columns: []string{"v1", "v2"}}},
	})
	testTagPath(t, CaseTagPath{
		path: "table_1/tag_1#value*0.01",
		expect: &TagPath{
			Table: "TABLE_1",
			Tag:   "tag_1",
			Field: TagPathField{Columns: []string{"value"}}},
	})
	testTagPath(t, CaseTagPath{
		path: "table_1/tag_1#kalman(value)",
		expect: &TagPath{
			Table: "TABLE_1",
			Tag:   "tag_1",
			Field: TagPathField{Columns: []string{"value"}}},
	})
	testTagPath(t, CaseTagPath{
		path: "table_1/tag_1#fft( kalman(value) )",
		expect: &TagPath{
			Table: "TABLE_1",
			Tag:   "tag_1",
			Field: TagPathField{Columns: []string{"value"}}},
	})
}

func testTagPath(t *testing.T, tc CaseTagPath) {
	p, err := ParseTagPathWithFunctions(tc.path, map[string]expression.Function{
		"kalman": func(args ...any) (any, error) { return nil, nil },
		"fft":    func(args ...any) (any, error) { return nil, nil },
	})
	if tc.expect == nil {
		require.NotNil(t, err)
		require.Equal(t, err.Error(), "invalid syntax")
		return
	}
	require.Nil(t, err)
	require.NotNil(t, p)
	require.Equal(t, tc.expect.Table, p.Table)
	require.Equal(t, tc.expect.Tag, p.Tag)
	require.Equal(t, len(tc.expect.Field.Columns), len(p.Field.Columns))
	for i, c := range tc.expect.Field.Columns {
		require.Equal(t, c, p.Field.Columns[i])
	}
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
