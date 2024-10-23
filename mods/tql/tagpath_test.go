package tql_test

import (
	"testing"

	"github.com/machbase/neo-server/mods/expression"
	. "github.com/machbase/neo-server/mods/tql"
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
