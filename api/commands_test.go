package api_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/machbase/neo-server/v8/api"
	"github.com/stretchr/testify/require"
)

func TestParseCommandLine(t *testing.T) {
	tests := []struct {
		input  string
		expect []string
	}{
		{
			input:  "show tables -a",
			expect: []string{"show", "tables", "-a"},
		},
		{
			input:  `sql 'select * from tt where A=\'a\''`,
			expect: []string{"sql", "select * from tt where A='a'"},
		},
		{
			input:  `sql select * from tt where A='a'`,
			expect: []string{"sql", "select * from tt where A='a'"},
		},
		{
			input:  `sql --format xyz --heading -- select * from example`,
			expect: []string{"sql", "--format", "xyz", "--heading", "select * from example"},
		},
		{
			input:  `explain --full select * from example`,
			expect: []string{"explain", "--full", "select * from example"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := api.ParseCommandLine(tc.input)
			require.Equal(t, tc.expect, result)
		})
	}
}

func ExampleCommandHandler_NewShowCommand() {
	h := &api.CommandHandler{
		Database: func(ctx context.Context) (api.Conn, error) {
			return testServer.DatabaseSVR().Connect(ctx, api.WithPassword("sys", "manager"))
		},
		ShowTables: func(ti *api.TableInfo, nrow int64) bool {
			fmt.Println(nrow, ti.User, ti.Name, ti.Type)
			return true
		},
	}
	err := h.Exec(context.TODO(), api.ParseCommandLine("show tables"))
	if err != nil {
		panic(err)
	}
	// Output:
	// 1 SYS LOG_DATA LogTable
	// 2 SYS TAG_DATA TagTable
	// 3 SYS TAG_SIMPLE TagTable
}

func TestCommands(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		expect     []string
		expectErr  string
		expectFunc func(t *testing.T, actual string)
	}{
		{
			name:      "wrong command",
			input:     "cmd_not_exist",
			expectErr: `unknown command "cmd_not_exist" for "machbase-neo"`,
		},
		{
			name:  "show tables",
			input: "show tables",
			expect: []string{
				"1 SYS LOG_DATA LogTable",
				"2 SYS TAG_DATA TagTable",
				"3 SYS TAG_SIMPLE TagTable",
			},
		},
		{
			name:  "show tables all",
			input: "show tables --all",
			expect: []string{
				"1 SYS LOG_DATA LogTable",
				"2 SYS TAG_DATA TagTable",
				"3 SYS TAG_SIMPLE TagTable",
				"4 SYS _TAG_DATA_DATA_0 KeyValueTable",
				"5 SYS _TAG_DATA_META LookupTable",
				"6 SYS _TAG_SIMPLE_DATA_0 KeyValueTable",
				"7 SYS _TAG_SIMPLE_META LookupTable",
			},
		},
		{
			name:  "show table log_data",
			input: "show table log_data",
			expect: []string{
				"TIME datetime 31  ",
				"SHORT_VALUE short 6  ",
				"USHORT_VALUE ushort 5  ",
				"INT_VALUE integer 11  ",
				"UINT_VALUE uinteger 10  ",
				"LONG_VALUE long 20  ",
				"ULONG_VALUE ulong 20  ",
				"DOUBLE_VALUE double 17  ",
				"FLOAT_VALUE float 17  ",
				"STR_VALUE varchar 400  ",
				"JSON_VALUE json 32767  ",
				"IPV4_VALUE ipv4 15  ",
				"IPV6_VALUE ipv6 45  ",
				"TEXT_VALUE text 67108864  ",
				"BIN_VALUE binary 67108864  "},
		},
		{
			name:  "show table log_data all",
			input: "show table -a log_data",
			expect: []string{
				"_ARRIVAL_TIME datetime 31  ",
				"TIME datetime 31  ",
				"SHORT_VALUE short 6  ",
				"USHORT_VALUE ushort 5  ",
				"INT_VALUE integer 11  ",
				"UINT_VALUE uinteger 10  ",
				"LONG_VALUE long 20  ",
				"ULONG_VALUE ulong 20  ",
				"DOUBLE_VALUE double 17  ",
				"FLOAT_VALUE float 17  ",
				"STR_VALUE varchar 400  ",
				"JSON_VALUE json 32767  ",
				"IPV4_VALUE ipv4 15  ",
				"IPV6_VALUE ipv6 45  ",
				"TEXT_VALUE text 67108864  ",
				"BIN_VALUE binary 67108864  ",
				"_RID long 20  "},
		},
		{
			name:  "desc table tag_data all",
			input: "desc -a tag_data",
			expect: []string{
				"NAME varchar 100 tag name ",
				"TIME datetime 31 basetime ",
				"VALUE double 17 summarized ",
				"SHORT_VALUE short 6  ",
				"USHORT_VALUE ushort 5  ",
				"INT_VALUE integer 11  ",
				"UINT_VALUE uinteger 10  ",
				"LONG_VALUE long 20  ",
				"ULONG_VALUE ulong 20  ",
				"STR_VALUE varchar 400  ",
				"JSON_VALUE json 32767  ",
				"IPV4_VALUE ipv4 15  ",
				"IPV6_VALUE ipv6 45  ",
				"_RID long 20  "},
		},
		{
			name:  "show indexes",
			input: `show indexes`,
			expect: []string{
				"1 SYS __PK_IDX__TAG_DATA_META_1 REDBLACK _TAG_DATA_META _ID",
				"2 SYS _TAG_DATA_META_NAME REDBLACK _TAG_DATA_META NAME",
				"3 SYS __PK_IDX__TAG_SIMPLE_META_1 REDBLACK _TAG_SIMPLE_META _ID",
				"4 SYS _TAG_SIMPLE_META_NAME REDBLACK _TAG_SIMPLE_META NAME",
			},
		},
		{
			name:  "show tags tag_data",
			input: "show tags tag_data",
			expectFunc: func(t *testing.T, actual string) {
				lines := strings.Split(actual, "\n")
				require.Greater(t, len(lines), 100)
			},
		},
		{
			name:   "explain -- select * from log_data",
			input:  `explain -- select * from log_data`,
			expect: []string{" PROJECT", "  FULL SCAN (LOG_DATA)", ""},
		},
		{
			name:  "explain -f -- select * from tag_data",
			input: `explain -f -- select * from tag_data`,
			expectFunc: func(t *testing.T, actual string) {
				require.Greater(t, len(actual), 5000, actual)
				require.Contains(t, actual, "EXECUTE")
			},
		},
		{
			name:  "sql select",
			input: `sql -- select * from tag_data limit 0`,
			expect: []string{
				"NAME,TIME,VALUE,SHORT_VALUE,USHORT_VALUE,INT_VALUE,UINT_VALUE,LONG_VALUE,ULONG_VALUE,STR_VALUE,JSON_VALUE,IPV4_VALUE,IPV6_VALUE",
				"no rows fetched.",
			},
		},
	}

	h := &api.CommandHandler{
		Database: func(ctx context.Context) (api.Conn, error) {
			return testServer.DatabaseSVR().Connect(ctx, api.WithPassword("sys", "manager"))
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	output := &bytes.Buffer{}
	h.ShowTables = showTables(t, output)
	h.ShowIndexes = showIndexes(t, output)
	h.DescribeTable = descTable(t, output)
	h.ShowTags = showTags(t, output)
	h.Explain = explain(t, output)
	h.SqlQuery = sqlQuery(t, output)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := h.Exec(context.TODO(), api.ParseCommandLine(tt.input))
			if err != nil {
				if tt.expectErr != "" {
					require.Contains(t, err.Error(), tt.expectErr)
					return
				} else {
					t.Errorf("cmd.Execute() error = %v", err)
					return
				}
			}
			if tt.expectFunc != nil {
				tt.expectFunc(t, output.String())
			} else {
				actual := strings.Split(output.String(), "\n")
				if actual[len(actual)-1] == "" {
					// remove last empty line that comes from split by '\n'
					actual = actual[:len(actual)-1]
				}
				require.Equal(t, tt.expect, actual)
			}
			output.Reset()
		})
	}
}

func showTables(t *testing.T, output io.Writer) func(ti *api.TableInfo, nrow int64) bool {
	return func(ti *api.TableInfo, nrow int64) bool {
		if ti.Err != nil {
			if ti.Err != io.EOF {
				t.Fatal(ti.Err)
			}
			return false
		}
		fmt.Fprintln(output, nrow, ti.User, ti.Name, ti.Type)
		return true
	}
}

func showIndexes(t *testing.T, output io.Writer) func(ti *api.IndexInfo, nrow int64) bool {
	return func(ti *api.IndexInfo, nrow int64) bool {
		if ti.Err != nil {
			if ti.Err != io.EOF {
				t.Fatal(ti.Err)
			}
			return false
		}
		fmt.Fprintln(output, nrow, ti.User, ti.Name, ti.Type, ti.Table, strings.Join(ti.Cols, ","))
		return true
	}
}

func descTable(_ *testing.T, output io.Writer) func(desc *api.TableDescription) {
	return func(desc *api.TableDescription) {
		for _, col := range desc.Columns {
			indexes := []string{}
			for _, idxDesc := range desc.Indexes {
				for _, colName := range idxDesc.Cols {
					if colName == col.Name {
						indexes = append(indexes, idxDesc.Name)
						break
					}
				}
			}
			fmt.Fprintln(output, col.Name, col.Type, col.Width(), col.Flag, strings.Join(indexes, ","))
		}
	}
}

func showTags(t *testing.T, output io.Writer) func(ti *api.TagInfo, nrow int64) bool {
	return func(ti *api.TagInfo, nrow int64) bool {
		if ti.Err != nil {
			if ti.Err != io.EOF {
				t.Fatal(ti.Err)
			}
			return false
		}
		if ti.Stat != nil {
			fmt.Fprintln(output, nrow, ti.Id, ti.Name, ti.Database, ti.User, ti.Table, ti.Stat.RowCount)
		} else {
			fmt.Fprintln(output, nrow, ti.Id, ti.Name, ti.Database, ti.User, ti.Table, "NULL")
		}
		return true
	}
}

func explain(t *testing.T, output io.Writer) func(plan string, err error) {
	return func(plan string, err error) {
		if err != nil {
			t.Fatal(err)
		}
		fmt.Fprintln(output, plan)
	}
}

func sqlQuery(t *testing.T, output io.Writer) func(q *api.Query, nrow int64) bool {
	return func(q *api.Query, nrow int64) bool {
		if nrow == 0 {
			columns := q.Columns()
			line := []string{}
			for _, c := range columns {
				line = append(line, c.Name)
			}
			fmt.Fprintln(output, strings.Join(line, ","))
		} else if nrow > 0 {
			columns := q.Columns()
			buffer, err := columns.MakeBuffer()
			if err != nil {
				t.Fatal(err)
			}
			q.Scan(buffer...)
			line := []string{}
			for _, c := range buffer {
				line = append(line, fmt.Sprintf("%v", c))
			}
			fmt.Fprintln(output, nrow, strings.Join(line, ","))
		} else {
			fmt.Fprintln(output, q.UserMessage())
		}
		return true
	}
}
