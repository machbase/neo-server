package api_test

//go:generate moq -out ./tables_mock_test.go -pkg api_test . Database Conn Row Result

import (
	"context"
	"testing"

	"github.com/machbase/neo-server/api"
	"github.com/machbase/neo-server/api/types"
	"github.com/stretchr/testify/require"
)

func TestExists(t *testing.T) {
	mockdb := &DatabaseMock{
		ConnectFunc: func(ctx context.Context, options ...api.ConnectOption) (api.Conn, error) {
			conn := &ConnMock{}
			conn.CloseFunc = func() error { return nil }
			conn.QueryRowFunc = func(ctx context.Context, sqlText string, params ...any) api.Row {
				switch sqlText {
				case "select count(*) from M$SYS_TABLES T, M$SYS_USERS U where U.NAME = ? and U.USER_ID = T.USER_ID AND T.NAME = ?":
					return &RowMock{
						ScanFunc: func(cols ...any) error {
							if len(params) == 2 {
								if params[1] == "EXAMPLE" {
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
							*(cols[0].(*int)) = int(types.TableTypeTag)
							return nil
						},
					}
				default:
					t.Logf("QueryRow sqlText: %s, params:%v", sqlText, params)
					return &RowMock{}
				}
			}
			conn.ExecFunc = func(ctx context.Context, sqlText string, params ...any) api.Result {
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
			}
			return conn, nil
		},
	}

	ctx := context.TODO()
	conn, err := mockdb.Connect(ctx)
	if err != nil {
		t.Error(err.Error())
	}
	{ // case: exists true
		exists, err := api.ExistsTable(ctx, conn, "example")
		if err != nil {
			t.Errorf("ExistsTable %s", err)
		}
		require.True(t, exists)
	}

	{ // case: exists true
		exists, err := api.ExistsTable(ctx, conn, "example-x")
		if err != nil {
			t.Errorf("ExistsTable %s", err)
		}
		require.False(t, exists)
	}

	{ // case: ExistsTableOrCreate for existing table
		exists, created, truncated, err := api.ExistsTableOrCreate(ctx, conn, "example", true, true)
		require.True(t, exists)
		require.False(t, created)
		require.True(t, truncated)
		require.Nil(t, err)
	}

	{ // case: ExistsTableOrCreate for non-existing table with 'create=false'
		exists, created, truncated, err := api.ExistsTableOrCreate(ctx, conn, "example_x", false, true)
		require.False(t, exists)
		require.False(t, created)
		require.False(t, truncated)
		require.Nil(t, err)
	}

	{ // case: ExistsTableOrCreate for non-existing table with 'create=true'
		exists, created, truncated, err := api.ExistsTableOrCreate(ctx, conn, "example_x", true, true)
		require.False(t, exists)
		require.True(t, created)
		require.False(t, truncated)
		require.Nil(t, err)
	}
}
