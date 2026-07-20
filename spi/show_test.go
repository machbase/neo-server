package spi_test

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/machbase/neo-client/api"
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

type ShowFixture struct {
	db     *sql.DB
	dbConn *sql.Conn
	conn   api.Conn
}

func (sf *ShowFixture) Close() {
	if sf.conn != nil {
		sf.conn.Close()
	}
	if sf.db != nil {
		sf.db.Close()
	}
	if sf.dbConn != nil {
		sf.dbConn.Close()
	}
}

func newShowDatabase(ctx context.Context) *ShowFixture {
	dsn := fmt.Sprintf("server=127.0.0.1:%d;user=sys;password=manager;fetch_rows=100", testServer.MachPort())
	db, _ := sql.Open("machbase", dsn)
	dbConn, _ := db.Conn(ctx)
	// Wrap the sql.Conn with spi.Conn
	conn := spi.WrapSqlConn(dbConn)
	return &ShowFixture{
		db:     db,
		dbConn: dbConn,
		conn:   conn,
	}
}

func TestShowLicense(t *testing.T) {
	fixture := newShowDatabase(t.Context())
	defer fixture.Close()
	tests := []ResultSetTestCase{
		{
			name:    "ShowLicense",
			fn:      func() spi.ResultSet { return spi.ResultSet(spi.ShowLicense(t.Context(), fixture.dbConn)) },
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
				require.Equal(t, "VALID", row[7])
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

func TestShowUsers(t *testing.T) {
	fixture := newShowDatabase(t.Context())
	defer fixture.Close()
	tests := []ResultSetTestCase{
		{
			name:    "ShowUsers",
			fn:      func() spi.ResultSet { return spi.ResultSet(spi.ShowUsers(t.Context(), fixture.dbConn)) },
			columns: []string{"USER_ID", "NAME"},
			expects: [][]any{
				{int64(1), "SYS"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runResultSetTestCases(t, tt)
		})
	}
}

func TestShowMetaTables(t *testing.T) {
	fixture := newShowDatabase(t.Context())
	defer fixture.Close()
	tests := []ResultSetTestCase{
		{
			name:    "ShowMetaTables",
			fn:      func() spi.ResultSet { return spi.ResultSet(spi.ShowMetaTables(t.Context(), fixture.dbConn)) },
			columns: []string{"ID", "NAME", "TYPE"},
			expectFunc: func(values [][]any) {
				require.GreaterOrEqual(t, len(values), 1)
				for _, row := range values {
					require.GreaterOrEqual(t, row[0], int64(1)) // ID
					require.NotEmpty(t, row[1])                 // NAME
					require.Equal(t, row[2], "Fixed")           // TYPE
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runResultSetTestCases(t, tt)
		})
	}
}

func TestShowVirtualTables(t *testing.T) {
	fixture := newShowDatabase(t.Context())
	defer fixture.Close()
	tests := []ResultSetTestCase{
		{
			name:    "ShowVirtualTables",
			fn:      func() spi.ResultSet { return spi.ResultSet(spi.ShowVirtualTables(t.Context(), fixture.dbConn)) },
			columns: []string{"ID", "NAME", "TYPE"},
			expectFunc: func(values [][]any) {
				require.GreaterOrEqual(t, len(values), 1)
				for _, row := range values {
					require.GreaterOrEqual(t, row[0], int64(1))                    // ID
					require.NotEmpty(t, row[1])                                    // NAME
					require.True(t, row[2] == "Fixed" || row[2] == "Fixed (stat)") // TYPE : "Fixed" or "Fixed (stat)"
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runResultSetTestCases(t, tt)
		})
	}
}

func TestShowSessions(t *testing.T) {
	fixture := newShowDatabase(t.Context())
	defer fixture.Close()
	tests := []ResultSetTestCase{
		{
			name:    "QuerySessions",
			fn:      func() spi.ResultSet { return spi.ResultSet(spi.ShowSessions(t.Context(), fixture.dbConn)) },
			columns: []string{"ID", "USER_NAME", "USER_ID", "LOGIN_TIME", "TYPE", "USER_IP", "MAX_QPX_MEM"},
			expectFunc: func(values [][]any) {
				row := values[0]
				require.Greater(t, row[0], int64(0))                                   // ID
				require.Equal(t, "SYS", row[1])                                        // USER_NAME
				require.GreaterOrEqual(t, row[2], int64(0))                            // USER_ID
				require.Greater(t, row[3], time.Time{})                                // LOGIN_TIME
				require.Equal(t, "CLI", row[4])                                        // TYPE
				require.Equal(t, "127.0.0.1", row[5])                                  // USER_IP
				require.Regexp(t, regexp.MustCompile(`^\d+(\.\d+)?[KMGT]?B$`), row[6]) // MAX_QPX_MEM
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runResultSetTestCases(t, tt)
		})
	}
}

func TestShowStatements(t *testing.T) {
	fixture := newShowDatabase(t.Context())
	defer fixture.Close()

	tests := []ResultSetTestCase{
		{
			name:    "ShowStatements",
			fn:      func() spi.ResultSet { return spi.ResultSet(spi.ShowStatements(t.Context(), fixture.dbConn)) },
			columns: []string{"ID", "SESSION_ID", "STATE", "RECORD_SIZE", "QUERY"},
			expectFunc: func(values [][]any) {
				row := values[0]
				require.GreaterOrEqual(t, row[0], int64(0)) // ID
				require.NotEmpty(t, row[2])                 // STATE
				require.NotEmpty(t, row[4])                 // QUERY
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runResultSetTestCases(t, tt)
		})
	}
}

func TestShowTables(t *testing.T) {
	fixture := newShowDatabase(t.Context())
	defer fixture.Close()
	conn := fixture.dbConn

	// Create a test table for the tests
	conn.ExecContext(t.Context(), "CREATE TAG TABLE RS_DATA(NAME VARCHAR(80) PRIMARY KEY, TIME DATETIME basetime, VALUE DOUBLE summarized) with rollup tag_partition_count = 1")
	defer conn.ExecContext(t.Context(), "DROP TAG TABLE RS_DATA CASCADE")
	conn.ExecContext(t.Context(), "INSERT INTO RS_DATA VALUES('test1', '2024-01-01 00:00:00', 1.0)")
	conn.ExecContext(t.Context(), "INSERT INTO RS_DATA VALUES('test1', '2024-01-02 00:00:00', 2.0)")
	conn.ExecContext(t.Context(), "exec table_flush('RS_DATA')")

	parseTime := func(str string) time.Time {
		tm, err := time.ParseInLocation("2006-01-02 15:04:05", str, time.Local)
		require.NoError(t, err)
		return tm
	}
	_ = parseTime

	tests := []ResultSetTestCase{
		{
			name:    "ShowTables",
			fn:      func() spi.ResultSet { return spi.ResultSet(spi.ShowTables(t.Context(), fixture.dbConn, false)) },
			columns: []string{"DATABASE_NAME", "USER_NAME", "TABLE_NAME", "TABLE_ID", "TABLE_TYPE", "TABLE_FLAG"},
			expects: [][]any{
				{"MACHBASEDB", "SYS", "RS_DATA", int64(11), "Tag", ""},
			},
		},
		{
			name:    "ShowTables_all",
			fn:      func() spi.ResultSet { return spi.ResultSet(spi.ShowTables(t.Context(), fixture.dbConn, true)) },
			columns: []string{"DATABASE_NAME", "USER_NAME", "TABLE_NAME", "TABLE_ID", "TABLE_TYPE", "TABLE_FLAG"},
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
			name:    "ShowTable",
			fn:      func() spi.ResultSet { return spi.ResultSet(spi.ShowTable(t.Context(), conn, "RS_DATA", false)) },
			columns: []string{"COLUMN", "TYPE", "LENGTH", "FLAG", "INDEX"},
			expects: [][]any{
				{"NAME", "varchar", 80, "tag name", ""},
				{"TIME", "datetime", 31, "base time", ""},
				{"VALUE", "double", 17, "", ""},
			},
		},
		{
			name:    "ShowTable_all",
			fn:      func() spi.ResultSet { return spi.ResultSet(spi.ShowTable(t.Context(), conn, "RS_DATA", true)) },
			columns: []string{"COLUMN", "TYPE", "LENGTH", "FLAG", "INDEX"},
			expects: [][]any{
				{"NAME", "varchar", 80, "tag name", ""},
				{"TIME", "datetime", 31, "base time", ""},
				{"VALUE", "double", 17, "", ""},
				{"_RID", "long", 20, "", ""},
			},
		},
		{
			name:    "ShowTable_meta",
			fn:      func() spi.ResultSet { return spi.ResultSet(spi.ShowTable(t.Context(), conn, "M$SYS_TABLES", false)) },
			columns: []string{"COLUMN", "TYPE", "LENGTH", "FLAG", "INDEX"},
			expects: [][]any{
				{"NAME", "varchar", 100, "", ""},
				{"TYPE", "integer", 11, "", ""},
				{"DATABASE_ID", "long", 20, "", ""},
				{"ID", "long", 20, "", ""},
				{"USER_ID", "integer", 11, "", ""},
				{"COLCOUNT", "integer", 11, "", ""},
				{"FLAG", "integer", 11, "", ""},
			},
		},
		{
			name:    "ShowIndexes",
			fn:      func() spi.ResultSet { return spi.ResultSet(spi.ShowIndexes(t.Context(), conn)) },
			columns: []string{"ID", "DATABASE", "USER", "TABLE", "COLUMN", "INDEX_NAME", "INDEX_TYPE", "KEY_COMPRESS", "MAX_LEVEL", "PART_VALUE_COUNT", "BITMAP_ENCODE"},
			expects: [][]any{
				{int64(6), "MACHBASEDB", "SYS", "_RS_DATA_META", "_ID", "__PK_IDX__RS_DATA_META_1", "REDBLACK", "UNCOMPRESS", int64(0), int64(100000), "EQUAL"},
				{int64(7), "MACHBASEDB", "SYS", "_RS_DATA_META", "NAME", "_RS_DATA_META_NAME", "REDBLACK", "UNCOMPRESS", int64(0), int64(100000), "EQUAL"},
				{int64(9), "MACHBASEDB", "SYS", "_RS_DATA_META", "_LAST_UPDATE_TIME", "_RS_DATA_META__LAST_UPDATE_TIME", "REDBLACK", "UNCOMPRESS", int64(0), int64(100000), "EQUAL"},
			},
		},
		{
			name:    "ShowIndex",
			fn:      func() spi.ResultSet { return spi.ResultSet(spi.ShowIndex(t.Context(), conn, "_RS_DATA_META_NAME")) },
			columns: []string{"ID", "TABLE", "COLUMN", "INDEX_NAME", "INDEX_TYPE", "KEY_COMPRESS", "MAX_LEVEL", "PART_VALUE_COUNT", "BITMAP_ENCODE"},
			expects: [][]any{
				{int64(0), "_RS_DATA_META", "NAME", "_RS_DATA_META_NAME", "REDBLACK", "UNCOMPRESSED", int64(0), int64(100000), "EQUAL"},
			},
		},
		{
			name:    "ShowStorage",
			fn:      func() spi.ResultSet { return spi.ResultSet(spi.ShowStorage(t.Context(), conn)) },
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
			name:    "ShowTableUsage",
			fn:      func() spi.ResultSet { return spi.ResultSet(spi.ShowTableUsage(t.Context(), conn)) },
			columns: []string{"DATABASE", "USER", "TABLE", "STORAGE_USAGE"},
			expectFunc: func(values [][]any) {
				names := []string{"RS_DATA", "_RS_DATA_DATA_0", "_RS_DATA_META", "_RS_DATA_ROLLUP_HOUR", "_RS_DATA_ROLLUP_MIN", "_RS_DATA_ROLLUP_SEC"}
				require.Equal(t, len(names), len(values))
				for _, row := range values {
					require.Equal(t, "MACHBASEDB", row[0])
					require.Contains(t, names, row[2])
					require.GreaterOrEqual(t, row[3], int64(0))
				}
			},
		},
		{
			name:    "ShowLsm",
			fn:      func() spi.ResultSet { return spi.ResultSet(spi.ShowLsm(t.Context(), conn)) },
			columns: []string{"TABLE_NAME", "INDEX_NAME", "LEVEL", "COUNT"},
			expects: [][]any{},
		},
		{
			name:    "ShowIndexGap",
			fn:      func() spi.ResultSet { return spi.ResultSet(spi.ShowIndexGap(t.Context(), conn)) },
			columns: []string{"INDEX_ID", "TABLE_NAME", "INDEX_NAME", "GAP"},
			expects: [][]any{},
		},
		{
			name:    "ShowTagIndexGap",
			fn:      func() spi.ResultSet { return spi.ResultSet(spi.ShowTagIndexGap(t.Context(), conn)) },
			columns: []string{"TABLE_ID", "TABLE_NAME", "STATUS", "DISK_GAP", "MEMORY_GAP"},
			expectFunc: func(values [][]any) {
				row := values[0]
				require.GreaterOrEqual(t, row[0], int64(1))
				require.NotEmpty(t, row[1])
				require.NotEmpty(t, row[2]) // "IDLE[0/0]"
				require.GreaterOrEqual(t, row[3], int64(1))
				require.GreaterOrEqual(t, row[4], int64(0))
			},
		},
		{
			name:    "ShowRollupGap",
			fn:      func() spi.ResultSet { return spi.ResultSet(spi.ShowRollupGap(t.Context(), conn)) },
			columns: []string{"USER_NAME", "ROLLUP_NAME", "SRC_TABLE", "ROLLUP_TABLE", "SRC_END_RID", "ROLLUP_END_RID", "GAP", "RUN_STATE", "LAST_ELAPSED_MSEC", "LAST_WAKEUP_TIME", "NEXT_WAKEUP_TIME"},
			expectFunc: func(values [][]any) {
				require.Equal(t, "_RS_DATA_ROLLUP_SEC", values[0][1]) // rollup name
				require.Equal(t, "_RS_DATA_DATA_0", values[0][2])     // src table
				require.Equal(t, "_RS_DATA_ROLLUP_SEC", values[0][3]) // rollup table
			},
		},
		{
			name:    "ShowTags",
			fn:      func() spi.ResultSet { return spi.ResultSet(spi.ShowTags(t.Context(), conn, "rs_data", "test1")) },
			columns: []string{"ID", "NAME", "ROW_COUNT", "MIN_TIME", "MAX_TIME", "RECENT_ROW_TIME", "MIN_VALUE", "MIN_VALUE_TIME", "MAX_VALUE", "MAX_VALUE_TIME"},
			expects: [][]any{
				{int64(1), "test1", int64(2), parseTime("2024-01-01 00:00:00.000"), parseTime("2024-01-02 00:00:00.000"), parseTime("2024-01-02 00:00:00"), float64(1), parseTime("2024-01-01 00:00:00.000"), float64(2), parseTime("2024-01-02 00:00:00.000")},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runResultSetTestCases(t, tt)
		})
	}
}
