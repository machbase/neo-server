package util_test

import (
	"testing"
	"time"

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

func TestWriteDescriptor(t *testing.T) {
	tzAsiaSeoul, _ := time.LoadLocation("Asia/Seoul")

	tests := []struct {
		path   string
		expect *WriteDescriptor
	}{
		{"db/abc.tql",
			&WriteDescriptor{
				TqlPath:    "db/abc.tql",
				Timeformat: "ns", TimeLocation: time.UTC, Delimiter: ",", Heading: false,
			},
		},
		{"/test.tql?timeformat=ms",
			&WriteDescriptor{
				TqlPath:    "/test.tql",
				Timeformat: "ms", TimeLocation: time.UTC, Delimiter: ",", Heading: false,
			},
		},
		{"/project/test.tql?timeformat=Default&tz=KST&heading=true",
			&WriteDescriptor{
				TqlPath:    "/project/test.tql",
				Timeformat: "Default", TimeLocation: tzAsiaSeoul, Delimiter: ",", Heading: true,
			},
		},
		{"db/write/example",
			&WriteDescriptor{
				Method: "insert", Table: "EXAMPLE", Format: "json", Compress: "",
				Timeformat: "ns", TimeLocation: time.UTC, Delimiter: ",", Heading: false,
			},
		},
		{"db/append/example:csv:gzip?timeformat=Kitchen&tz=Local&heading=true",
			&WriteDescriptor{
				Method: "append", Table: "EXAMPLE", Format: "csv", Compress: "gzip",
				Timeformat: "Kitchen", TimeLocation: time.Local, Delimiter: ",", Heading: true,
			},
		},
	}

	for _, tt := range tests {
		actual, err := NewWriteDescriptor(tt.path)
		if err != nil {
			t.Errorf("path: %s, %s", tt.path, err.Error())
			t.Fail()
		}

		require.Equal(t, tt.expect.TqlPath, actual.TqlPath, tt.path)
		require.Equal(t, tt.expect.Method, actual.Method, tt.path)
		require.Equal(t, tt.expect.Table, actual.Table, tt.path)
		require.Equal(t, tt.expect.Format, actual.Format, tt.path)
		require.Equal(t, tt.expect.Compress, actual.Compress, tt.path)
		require.Equal(t, tt.expect.Timeformat, actual.Timeformat, tt.path)
		require.Equal(t, tt.expect.Delimiter, actual.Delimiter, tt.path)
		require.Equal(t, tt.expect.Heading, actual.Heading, tt.path)
	}
}
