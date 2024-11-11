package util_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/machbase/neo-server/api"
	"github.com/machbase/neo-server/mods/util"
	"github.com/stretchr/testify/require"
)

func TestSplitFields(t *testing.T) {
	testSplitFields(t, true,
		`--data "C:\Users\user\work\neo-download\neo 0.1.2\machbase_home"`,
		[]string{"--data", `C:\Users\user\work\neo-download\neo 0.1.2\machbase_home`})
	testSplitFields(t, false,
		`--data "C:\Users\user\work\neo-download\neo 0.1.2\machbase_home"`,
		[]string{"--data", `"C:\Users\user\work\neo-download\neo 0.1.2\machbase_home"`})
}

func testSplitFields(t *testing.T, stripQuotes bool, args string, expects []string) {
	toks := util.SplitFields(args, stripQuotes)
	require.Equal(t, len(expects), len(toks))
	for i, tok := range toks {
		require.Equal(t, expects[i], tok)
	}
}

func TestStripQuote(t *testing.T) {
	ret := util.StripQuote(`"str abc"`)
	require.Equal(t, "str abc", ret)

	ret = util.StripQuote(`"str abc'`)
	require.Equal(t, "str abc'", ret)

	ret = util.StripQuote("`str abc'")
	require.Equal(t, "`str abc'", ret)

	ret = util.StripQuote("")
	require.Equal(t, "", ret)
}

func TestStringFields(t *testing.T) {
	ts := time.Unix(1691800174, 123456789).UTC()

	vals := util.StringFields([]any{&ts}, "ns", nil, 0)
	expects := []string{"1691800174123456789"}
	require.Equal(t, expects[0], vals[0])

	vals = util.StringFields([]any{&ts}, "us", nil, 0)
	expects = []string{"1691800174123456"}
	require.Equal(t, expects[0], vals[0])

	vals = util.StringFields([]any{&ts}, "ms", nil, 0)
	expects = []string{"1691800174123"}
	require.Equal(t, expects[0], vals[0])

	vals = util.StringFields([]any{&ts}, "s", nil, 0)
	expects = []string{"1691800174"}
	require.Equal(t, expects[0], vals[0])

	vals = util.StringFields([]any{ts}, "ns", nil, 0)
	expects = []string{"1691800174123456789"}
	require.Equal(t, expects[0], vals[0])

	vals = util.StringFields([]any{ts}, "us", nil, 0)
	expects = []string{"1691800174123456"}
	require.Equal(t, expects[0], vals[0])

	vals = util.StringFields([]any{ts}, "ms", nil, 0)
	expects = []string{"1691800174123"}
	require.Equal(t, expects[0], vals[0])

	vals = util.StringFields([]any{ts}, "s", nil, 0)
	expects = []string{"1691800174"}
	require.Equal(t, expects[0], vals[0])

	vals = util.StringFields([]any{9, "123", ts, 456.789}, util.GetTimeformat("KITCHEN"), time.UTC, -1)
	expects = []string{"9", "123", "12:29:34AM", "456.789000"}
	for i, expect := range expects {
		require.Equal(t, expect, vals[i])
	}

	vals = util.StringFields([]any{9, "123", ts, 456.789}, util.GetTimeformat("KITCHEN"), nil, 0)
	expects = []string{"9", "123", "12:29:34AM", "457"}
	for i, expect := range expects {
		require.Equal(t, expect, vals[i])
	}

	iVal := 9
	sVal := "123"
	fVal := 456.789
	vals = util.StringFields([]any{&iVal, &sVal, &ts, &fVal, nil}, util.GetTimeformat("KITCHEN"), nil, 1)
	expects = []string{"9", "123", "12:29:34AM", "456.8", "NULL"}
	for i, expect := range expects {
		require.Equal(t, expect, vals[i])
	}

	tz, _ := util.ParseTimeLocation("EST", nil)
	vals = util.StringFields([]any{&iVal, &sVal, &ts, &fVal, nil}, util.GetTimeformat("KITCHEN"), tz, 4)
	expects = []string{"9", "123", "7:29:34PM", "456.7890", "NULL"}
	for i, expect := range expects {
		require.Equal(t, expect, vals[i])
	}

	bVal := int8(0x67)
	i16val := int16(0x16)
	i32val := int32(0x32)
	i64val := int64(0x64)
	netip := net.ParseIP("127.0.0.1")

	vals = util.StringFields([]any{&bVal, &i16val, &i32val, &i64val, &fVal, &netip, &util.NameValuePair{Name: "name", Value: `value "here"`}}, "", nil, -1)
	expects = []string{"103", "22", "50", "100", "456.789000", "127.0.0.1", `name="value \"here\""`}
	for i, expect := range expects {
		require.Equal(t, expect, vals[i])
	}

	vals = util.StringFields([]any{bVal, i16val, i32val, i64val, fVal, netip, util.NameValuePair{Name: "name", Value: "value"}}, "", nil, -1)
	expects = []string{"103", "22", "50", "100", "456.789000", "127.0.0.1", "util.NameValuePair"}
	for i, expect := range expects {
		require.Equal(t, expect, vals[i])
	}
}

func TestSplitSqlStatementsSingleLine(t *testing.T) {
	input := "SELECT 2 FROM T WHERE name = '--abc';"
	expect := []*util.SqlStatement{
		{BeginLine: 1, EndLine: 1, IsComment: false, Text: "SELECT 2 FROM T WHERE name = '--abc';", Env: &util.SqlStatementEnv{}},
	}
	statements, err := util.SplitSqlStatements(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}

	for n, stmt := range statements {
		require.EqualValues(t, expect[n], stmt, stmt.Text)
	}
}

func TestSplitSqlStatements(t *testing.T) {
	tests := []struct {
		inputFile  string
		expectFile string
	}{
		{"splitter_sql_1.sql", "splitter_sql_1.json"},
		{"splitter_sql_2.sql", "splitter_sql_2.json"},
	}
	for _, tc := range tests {
		b, err := os.ReadFile(filepath.Join("testdata", tc.inputFile))
		if err != nil {
			t.Fatal(err, tc.inputFile)
		}
		stmts, err := util.SplitSqlStatements(bytes.NewReader(b))
		if err != nil {
			t.Fatal(err, tc.inputFile)
		}
		result := []map[string]any{}
		output, err := json.Marshal(stmts)
		if err != nil {
			t.Fatal(err, tc.inputFile)
		} else {
			if err := json.Unmarshal(output, &result); err != nil {
				t.Fatal(err, tc.inputFile)
			}
			if runtime.GOOS == "windows" {
				for _, stmt := range stmts {
					stmt.Text = strings.ReplaceAll(stmt.Text, "\r\n", "\n")
				}
			}
		}
		expect := []map[string]any{}
		if expectContent, err := os.ReadFile(filepath.Join("testdata", tc.expectFile)); err != nil {
			t.Fatal(err, tc.inputFile)
		} else {
			if err := json.Unmarshal(expectContent, &expect); err != nil {
				t.Fatal(err, tc.inputFile, string(output))
			}
		}
		require.Equal(t, expect, result, string(output))
	}
}

func ExampleSplitSqlStatements() {
	input := `SELECT 1; SELECT 2 FROM T WHERE name = '--abc';
	-- comment
	
	SELECT *  -- start of statement
	FROM
		table 
	WHERE
		name = 'a;b--c'; -- end of statement
	SELECT 4;

	wrong statement
	`
	statements, err := util.SplitSqlStatements(strings.NewReader(input))
	if err != nil {
		fmt.Println(err)
		return
	}

	for n, stmt := range statements {
		if !stmt.IsComment {
			stmt.Text = api.SqlTidy(stmt.Text)
		}
		fmt.Println(n, stmt.BeginLine, stmt.EndLine, stmt.IsComment, stmt.Text)
	}

	// Output:
	// 0 1 1 false SELECT 1;
	// 1 1 1 false SELECT 2 FROM T WHERE name = '--abc';
	// 2 2 2 true -- comment
	// 3 4 4 true -- start of statement
	// 4 4 8 false SELECT *  	FROM table WHERE name = 'a;b--c';
	// 5 8 8 true -- end of statement
	// 6 9 9 false SELECT 4;
	// 7 11 12 false wrong statement
}

func ExampleParseNameValuePairs() {
	input := `name1=value1 name2="value \"with\" spaces" name3=value3 name4 `
	result := util.ParseNameValuePairs(input)
	for _, pair := range result {
		fmt.Printf("%s=%s\n", pair.Name, pair.Value)
	}

	// Output:
	// name1=value1
	// name2=value "with" spaces
	// name3=value3
	// name4=
}
