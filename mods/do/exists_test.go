package do_test

//go:generate moq -out ./mock_test.go -pkg do_test ../../../neo-spi Database Row Result

import (
	"testing"

	"github.com/machbase/neo-server/mods/do"
	spi "github.com/machbase/neo-spi"
	"github.com/stretchr/testify/require"
)

func TestExists(t *testing.T) {
	mockdb := &DatabaseMock{
		QueryRowFunc: func(sqlText string, params ...any) spi.Row {
			switch sqlText {
			case "select count(*) from M$SYS_TABLES where name = ?":
				return &RowMock{
					ScanFunc: func(cols ...any) error {
						if len(params) == 1 {
							if params[0] == "EXAMPLE" {
								*(cols[0].(*int)) = 1
							} else {
								*(cols[0].(*int)) = 0
							}
						}
						return nil
					},
				}
			case "select type from M$SYS_TABLES where name = ?":
				return &RowMock{
					ScanFunc: func(cols ...any) error {
						*(cols[0].(*int)) = spi.TagTableType
						return nil
					},
				}
			default:
				t.Logf("QueryRow sqlText: %s, params:%v", sqlText, params)
				return &RowMock{}
			}
		},
		ExecFunc: func(sqlText string, params ...any) spi.Result {
			switch sqlText {
			case "delete from example":
				return &ResultMock{
					ErrFunc:          func() error { return nil },
					MessageFunc:      func() string { return "mocking delete all" },
					RowsAffectedFunc: func() int64 { return 1 },
				}
			case "create tag table example_x (name varchar(100) primary key, time datetime basetime, value double)":
				return &ResultMock{
					ErrFunc:          func() error { return nil },
					MessageFunc:      func() string { return "mocking create table" },
					RowsAffectedFunc: func() int64 { return 0 },
				}
			default:
				t.Logf("Exec sqlText: %s, params:%v", sqlText, params)
			}
			return &ResultMock{}
		},
	}

	{ // case: exists true
		exists, err := do.ExistsTable(mockdb, "example")
		if err != nil {
			t.Errorf("ExistsTable %s", err)
		}
		require.True(t, exists)
	}

	{ // case: exists true
		exists, err := do.ExistsTable(mockdb, "example-x")
		if err != nil {
			t.Errorf("ExistsTable %s", err)
		}
		require.False(t, exists)
	}

	{ // case: ExistsTableOrCreate for existing table
		exists, created, truncated, err := do.ExistsTableOrCreate(mockdb, "example", true, true)
		require.True(t, exists)
		require.False(t, created)
		require.True(t, truncated)
		require.Nil(t, err)
	}

	{ // case: ExistsTableOrCreate for non-existing table with 'create=false'
		exists, created, truncated, err := do.ExistsTableOrCreate(mockdb, "example_x", false, true)
		require.False(t, exists)
		require.False(t, created)
		require.False(t, truncated)
		require.Nil(t, err)
	}

	{ // case: ExistsTableOrCreate for non-existing table with 'create=true'
		exists, created, truncated, err := do.ExistsTableOrCreate(mockdb, "example_x", true, true)
		require.False(t, exists)
		require.True(t, created)
		require.False(t, truncated)
		require.Nil(t, err)
	}
}
