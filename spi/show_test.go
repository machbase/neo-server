package spi_test

import (
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/machbase/neo-server/v8/mods/model"
	"github.com/machbase/neo-server/v8/spi"
	"github.com/stretchr/testify/require"
)

type ResultSetTestCase struct {
	name       string
	fn         func() spi.ResultSet
	message    string
	columns    []string
	expects    [][]any
	expectFunc func([][]any)
}

func runResultSetTestCases(t *testing.T, tc ResultSetTestCase) {
	t.Helper()
	rs := tc.fn()
	require.NoError(t, rs.Err())
	require.Equal(t, tc.columns, rs.Columns().Names())
	values := make([][]interface{}, 0)
	rs.Iter(func(row []interface{}) bool {
		values = append(values, row)
		return true
	})
	actual := strings.Builder{}
	for _, row := range values {
		actual.WriteString("{")
		for _, col := range row {
			if col == nil {
				actual.WriteString("nil,")
				continue
			}
			switch v := col.(type) {
			case string:
				actual.WriteString(fmt.Sprintf("%q,", v))
			case int64:
				actual.WriteString(fmt.Sprintf("int64(%d),", v))
			case time.Time:
				actual.WriteString(fmt.Sprintf("parseTime(%q),", v.In(time.Local).Format("2006-01-02 15:04:05")))
			default:
				actual.WriteString(fmt.Sprintf("%v,", v))
			}
		}
		actual.WriteString("},\n")
	}
	if tc.expectFunc != nil {
		tc.expectFunc(values)
	} else {
		require.Equal(t, tc.expects, values, actual.String())
	}
	require.Equal(t, rs.Message(), tc.message)
}

func TestShowInfo(t *testing.T) {
	spi.SetServerInfoProvider(func() map[string]any {
		return map[string]any{
			"Name":    "test",
			"Version": "1.0.0",
		}
	})
	tests := []ResultSetTestCase{
		{
			name:    "ShowInfo",
			fn:      func() spi.ResultSet { return spi.ResultSet(spi.ShowInfo()) },
			columns: []string{"NAME", "VALUE"},
			expects: [][]any{
				{"Name", "test"},
				{"Version", "1.0.0"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runResultSetTestCases(t, tt)
		})
	}
}

func TestShowPorts(t *testing.T) {
	spi.SetServerPortsProvider(func(string) ([]*model.ServicePort, error) {
		return []*model.ServicePort{
			{Service: "servicectl", Address: "tcp://127.0.0.1:40257"},
		}, nil
	})
	tests := []ResultSetTestCase{
		{
			name:    "ShowPorts",
			fn:      func() spi.ResultSet { return spi.ResultSet(spi.ShowPorts("")) },
			columns: []string{"PORT", "ADDRESS"},
			expects: [][]any{
				{"servicectl", "tcp://127.0.0.1:40257"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runResultSetTestCases(t, tt)
		})
	}
}

func TestQueryResultSet(t *testing.T) {
	dsn := fmt.Sprintf("server=127.0.0.1:%d;user=sys;password=manager;fetch_rows=100", testServer.MachPort())
	db, err := sql.Open("machbase", dsn)
	require.NoError(t, err)
	defer db.Close()
	dbConn, err := db.Conn(t.Context())
	require.NoError(t, err)
	defer dbConn.Close()
	// Create a test table for the tests
	dbConn.ExecContext(t.Context(), "CREATE TAG TABLE RS_DATA(NAME VARCHAR(80) PRIMARY KEY, TIME DATETIME basetime, VALUE DOUBLE summarized) with rollup tag_partition_count = 1")
	defer dbConn.ExecContext(t.Context(), "DROP TAG TABLE RS_DATA CASCADE")
	dbConn.ExecContext(t.Context(), "INSERT INTO RS_DATA VALUES('test1', '2024-01-01 00:00:00', 1.0)")
	dbConn.ExecContext(t.Context(), "INSERT INTO RS_DATA VALUES('test1', '2024-01-02 00:00:00', 2.0)")
	dbConn.ExecContext(t.Context(), "exec table_flush('RS_DATA')")

	parseTime := func(str string) time.Time {
		tm, err := time.ParseInLocation("2006-01-02 15:04:05", str, time.Local)
		require.NoError(t, err)
		return tm.In(time.UTC)
	}
	_ = parseTime

	// Wrap the sql.Conn with spi.Conn
	conn := spi.WrapSqlConn(dbConn)

	tests := []struct {
		name       string
		fn         func() spi.ResultSet
		message    string
		columns    []string
		expects    [][]any
		expectFunc func([][]any)
	}{
		{
			name:    "QueryLicense",
			fn:      func() spi.ResultSet { return spi.ResultSet(spi.QueryLicense(t.Context(), conn)) },
			columns: []string{"ID", "TYPE", "CUSTOMER", "PROJECT", "COUNTRY_CODE", "INSTALL_DATE", "ISSUE_DATE", "STATUS"},
			expectFunc: func(values [][]any) {
				row := values[0]
				require.Equal(t, "00000000", row[0])
				require.Equal(t, "COMMUNITY", row[1])
				require.Equal(t, "NONE", row[2])
				require.Equal(t, "NONE", row[3])
				require.Equal(t, "KR", row[4])
				require.NotEmpty(t, row[5])
				require.NotEmpty(t, row[6])
				require.Equal(t, "Valid", row[7])
			},
		},
		{
			name:    "QueryTables",
			fn:      func() spi.ResultSet { return spi.ResultSet(spi.QueryTables(t.Context(), conn, false)) },
			columns: []string{"DATABASE", "USER", "NAME", "ID", "TYPE", "FLAG"},
			expects: [][]any{
				{"MACHBASEDB", "SYS", "RS_DATA", int64(11), "Tag", ""},
			},
		},
		{
			name:    "QueryTables_all",
			fn:      func() spi.ResultSet { return spi.ResultSet(spi.QueryTables(t.Context(), conn, true)) },
			columns: []string{"DATABASE", "USER", "NAME", "ID", "TYPE", "FLAG"},
			expects: [][]any{
				{"MACHBASEDB", "SYS", "RS_DATA", int64(11), "Tag", ""},
				{"MACHBASEDB", "SYS", "_RS_DATA_DATA_0", int64(1), "KeyValue", "Data"},
				{"MACHBASEDB", "SYS", "_RS_DATA_META", int64(2), "Lookup", "Meta"},
				{"MACHBASEDB", "SYS", "_RS_DATA_ROLLUP_HOUR", int64(5), "KeyValue", "Rollup"},
				{"MACHBASEDB", "SYS", "_RS_DATA_ROLLUP_MIN", int64(4), "KeyValue", "Rollup"},
				{"MACHBASEDB", "SYS", "_RS_DATA_ROLLUP_SEC", int64(3), "KeyValue", "Rollup"},
			},
		},
		{
			name:    "QueryTable",
			fn:      func() spi.ResultSet { return spi.ResultSet(spi.QueryTable(t.Context(), conn, "RS_DATA", false)) },
			columns: []string{"COLUMN", "TYPE", "LENGTH", "FLAG", "INDEX"},
			expects: [][]any{
				{"NAME", "varchar", 80, "tag name", ""},
				{"TIME", "datetime", 31, "base time", ""},
				{"VALUE", "double", 17, "", ""},
			},
		},
		{
			name:    "QueryIndexes",
			fn:      func() spi.ResultSet { return spi.ResultSet(spi.QueryIndexes(t.Context(), conn)) },
			columns: []string{"ID", "DATABASE", "USER", "TABLE_NAME", "COLUMN_NAME", "INDEX_NAME", "INDEX_TYPE", "KEY_COMPRESS", "MAX_LEVEL", "PART_VALUE_COUNT", "BITMAP_ENCODE"},
			expects: [][]any{
				{int64(6), "MACHBASEDB", "SYS", "_RS_DATA_META", "_ID", "__PK_IDX__RS_DATA_META_1", "REDBLACK", "UNCOMPRESS", int64(0), int64(100000), "EQUAL"},
				{int64(7), "MACHBASEDB", "SYS", "_RS_DATA_META", "NAME", "_RS_DATA_META_NAME", "REDBLACK", "UNCOMPRESS", int64(0), int64(100000), "EQUAL"},
				{int64(9), "MACHBASEDB", "SYS", "_RS_DATA_META", "_LAST_UPDATE_TIME", "_RS_DATA_META__LAST_UPDATE_TIME", "REDBLACK", "UNCOMPRESS", int64(0), int64(100000), "EQUAL"},
			},
		},
		{
			name:    "QueryIndex",
			fn:      func() spi.ResultSet { return spi.ResultSet(spi.QueryIndex(t.Context(), conn, "_RS_DATA_META_NAME")) },
			columns: []string{"TABLE_NAME", "COLUMN_NAME", "INDEX_NAME", "INDEX_TYPE", "KEY_COMPRESS", "MAX_LEVEL", "PART_VALUE_COUNT", "BITMAP_ENCODE"},
			expects: [][]any{
				{"_RS_DATA_META", "NAME", "_RS_DATA_META_NAME", "REDBLACK", "UNCOMPRESSED", int64(0), int64(100000), "EQUAL"},
			},
		},
		{
			name:    "QueryLsmIndexes",
			fn:      func() spi.ResultSet { return spi.ResultSet(spi.QueryLsmIndexes(t.Context(), conn)) },
			columns: []string{"TABLE_NAME", "INDEX_NAME", "LEVEL", "COUNT"},
			expects: [][]any{},
		},
		{
			name:    "QueryTags",
			fn:      func() spi.ResultSet { return spi.ResultSet(spi.QueryTags(t.Context(), conn, "rs_data", "test1")) },
			columns: []string{"_ID", "NAME", "ROW_COUNT", "MIN_TIME", "MAX_TIME", "RECENT_ROW_TIME", "MIN_VALUE", "MIN_VALUE_TIME", "MAX_VALUE", "MAX_VALUE_TIME"},
			expects: [][]any{
				{int64(1), "test1", int64(2), parseTime("2024-01-01 00:00:00"), parseTime("2024-01-02 00:00:00"), parseTime("2024-01-02 00:00:00"), float64(1), parseTime("2024-01-01 00:00:00"), float64(2), parseTime("2024-01-02 00:00:00")},
			},
		},
		{
			name:    "QueryIndexGap",
			fn:      func() spi.ResultSet { return spi.ResultSet(spi.QueryIndexGap(t.Context(), conn)) },
			columns: []string{"ID", "TABLE", "INDEX", "GAP"},
			expects: [][]any{},
		},
		{
			name:    "QueryTagIndexGap",
			fn:      func() spi.ResultSet { return spi.ResultSet(spi.QueryTagIndexGap(t.Context(), conn)) },
			columns: []string{"ID", "STATUS", "DISK_GAP", "MEMORY_GAP"},
			expectFunc: func(values [][]any) {
				row := values[0]
				require.GreaterOrEqual(t, row[0], int64(1))
				require.NotEmpty(t, "IDLE[0/0]")
				require.GreaterOrEqual(t, row[2], int64(1))
				require.GreaterOrEqual(t, row[3], int64(0))
			},
		},
		{
			name:    "QueryRollupGap",
			fn:      func() spi.ResultSet { return spi.ResultSet(spi.QueryRollupGap(t.Context(), conn)) },
			columns: []string{"SRC_TABLE", "ROLLUP_TABLE", "SRC_END_RID", "ROLLUP_END_RID", "GAP", "LAST_TIME"},
			expectFunc: func(values [][]any) {
				require.Equal(t, "_RS_DATA_DATA_0", values[0][0])     // src table
				require.Equal(t, "_RS_DATA_ROLLUP_SEC", values[0][1]) // rollup table

				require.Equal(t, "_RS_DATA_ROLLUP_MIN", values[1][0])  // src table
				require.Equal(t, "_RS_DATA_ROLLUP_HOUR", values[1][1]) // rollup table

				require.Equal(t, "_RS_DATA_ROLLUP_SEC", values[2][0]) // src table
				require.Equal(t, "_RS_DATA_ROLLUP_MIN", values[2][1]) // rollup table
			},
		},
		{
			name:    "QueryStorage",
			fn:      func() spi.ResultSet { return spi.ResultSet(spi.QueryStorage(t.Context(), conn)) },
			columns: []string{"TABLE_NAME", "DATA_SIZE", "INDEX_SIZE", "TOTAL_SIZE"},
			expectFunc: func(values [][]any) {
				names := []string{"RS_DATA", "_RS_DATA_DATA_0", "_RS_DATA_META", "_RS_DATA_ROLLUP_HOUR", "_RS_DATA_ROLLUP_MIN", "_RS_DATA_ROLLUP_SEC"}
				require.Equal(t, len(names), len(values))
				for _, row := range values {
					require.Contains(t, names, row[0])
					require.GreaterOrEqual(t, row[1], int64(0))
					require.GreaterOrEqual(t, row[2], int64(0))
					require.GreaterOrEqual(t, row[3], int64(0))
				}
			},
		},
		{
			name:    "QueryTableUsage",
			fn:      func() spi.ResultSet { return spi.ResultSet(spi.QueryTableUsage(t.Context(), conn)) },
			columns: []string{"TABLE_NAME", "STORAGE_USAGE"},
			expectFunc: func(values [][]any) {
				names := []string{"RS_DATA", "_RS_DATA_DATA_0", "_RS_DATA_META", "_RS_DATA_ROLLUP_HOUR", "_RS_DATA_ROLLUP_MIN", "_RS_DATA_ROLLUP_SEC"}
				require.Equal(t, len(names), len(values))
				for _, row := range values {
					require.Contains(t, names, row[0])
					require.GreaterOrEqual(t, row[1], int64(0))
				}
			},
		},
		{
			name:    "QueryStatements",
			fn:      func() spi.ResultSet { return spi.ResultSet(spi.QueryStatements(t.Context(), conn)) },
			columns: []string{"ID", "SESSION_ID", "STATE", "TYPE", "RECORD_SIZE", "APPEND_SUCCESS_CNT", "APPEND_FAILURE_CNT", "QUERY"},
			expectFunc: func(values [][]any) {
				// {int64(20), int64(2), "Fetch prepared", "", int64(32851), nil, nil, "SELECT ID, SESS_ID, STATE, RECORD_SIZE, QUERY FROM V$STMT"},
				row := values[0]
				require.Greater(t, row[0], int64(0)) // ID
				require.NotEmpty(t, row[2])          // STATE
				require.NotEmpty(t, row[7])          // QUERY
			},
		},
		{
			name:    "QuerySessions",
			fn:      func() spi.ResultSet { return spi.ResultSet(spi.QuerySessions(t.Context(), conn)) },
			columns: []string{"ID", "USER_ID", "USER_NAME", "TYPE", "LOGIN_TIME", "MAX_QPX_MEM", "STMT_COUNT"},
			expectFunc: func(values [][]any) {
				row := values[0]
				require.Equal(t, int64(2), row[0])
				require.Equal(t, int64(0), row[1])
				require.Equal(t, "SYS", row[2])
				require.Equal(t, "CLI", row[3])
				require.NotEmpty(t, row[4])
				require.Equal(t, int64(268435456), row[5])
				require.Nil(t, row[6])
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rs := tt.fn()
			require.NoError(t, rs.Err())
			require.Equal(t, tt.columns, rs.Columns().Names())
			values := make([][]interface{}, 0)
			rs.Iter(func(row []interface{}) bool {
				values = append(values, row)
				return true
			})
			actual := strings.Builder{}
			for _, row := range values {
				actual.WriteString("{")
				for _, col := range row {
					if col == nil {
						actual.WriteString("nil,")
						continue
					}
					switch v := col.(type) {
					case string:
						actual.WriteString(fmt.Sprintf("%q,", v))
					case int64:
						actual.WriteString(fmt.Sprintf("int64(%d),", v))
					case time.Time:
						actual.WriteString(fmt.Sprintf("parseTime(%q),", v.In(time.Local).Format("2006-01-02 15:04:05")))
					default:
						actual.WriteString(fmt.Sprintf("%v,", v))
					}
				}
				actual.WriteString("},\n")
			}
			if tt.expectFunc != nil {
				tt.expectFunc(values)
			} else {
				require.Equal(t, tt.expects, values, actual.String())
			}
			require.Equal(t, rs.Message(), tt.message)
		})
	}
}
