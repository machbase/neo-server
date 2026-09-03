package util

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

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
	toks := SplitFields(args, stripQuotes)
	require.Equal(t, len(expects), len(toks))
	for i, tok := range toks {
		require.Equal(t, expects[i], tok)
	}
}

func TestStripQuote(t *testing.T) {
	ret := StripQuote(`"str abc"`)
	require.Equal(t, "str abc", ret)

	ret = StripQuote(`"str abc'`)
	require.Equal(t, "str abc'", ret)

	ret = StripQuote("`str abc'")
	require.Equal(t, "`str abc'", ret)

	ret = StripQuote("")
	require.Equal(t, "", ret)
}

func TestStringFields(t *testing.T) {
	ts := time.Unix(1691800174, 123456789).UTC()

	vals := StringFields([]any{&ts}, "ns", nil, 0)
	expects := []string{"1691800174123456789"}
	require.Equal(t, expects[0], vals[0])

	vals = StringFields([]any{&ts}, "us", nil, 0)
	expects = []string{"1691800174123456"}
	require.Equal(t, expects[0], vals[0])

	vals = StringFields([]any{&ts}, "ms", nil, 0)
	expects = []string{"1691800174123"}
	require.Equal(t, expects[0], vals[0])

	vals = StringFields([]any{&ts}, "s", nil, 0)
	expects = []string{"1691800174"}
	require.Equal(t, expects[0], vals[0])

	vals = StringFields([]any{ts}, "ns", nil, 0)
	expects = []string{"1691800174123456789"}
	require.Equal(t, expects[0], vals[0])

	vals = StringFields([]any{ts}, "us", nil, 0)
	expects = []string{"1691800174123456"}
	require.Equal(t, expects[0], vals[0])

	vals = StringFields([]any{ts}, "ms", nil, 0)
	expects = []string{"1691800174123"}
	require.Equal(t, expects[0], vals[0])

	vals = StringFields([]any{ts}, "s", nil, 0)
	expects = []string{"1691800174"}
	require.Equal(t, expects[0], vals[0])

	vals = StringFields([]any{9, "123", ts, 456.789}, GetTimeformat("KITCHEN"), time.UTC, -1)
	expects = []string{"9", "123", "12:29:34AM", "456.789000"}
	for i, expect := range expects {
		require.Equal(t, expect, vals[i])
	}

	vals = StringFields([]any{9, "123", ts, 456.789}, GetTimeformat("KITCHEN"), nil, 0)
	expects = []string{"9", "123", "12:29:34AM", "457"}
	for i, expect := range expects {
		require.Equal(t, expect, vals[i])
	}

	iVal := 9
	sVal := "123"
	fVal := 456.789
	vals = StringFields([]any{&iVal, &sVal, &ts, &fVal, nil}, GetTimeformat("KITCHEN"), nil, 1)
	expects = []string{"9", "123", "12:29:34AM", "456.8", "NULL"}
	for i, expect := range expects {
		require.Equal(t, expect, vals[i])
	}

	tz, _ := ParseTimeLocation("EST", nil)
	vals = StringFields([]any{&iVal, &sVal, &ts, &fVal, nil}, GetTimeformat("KITCHEN"), tz, 4)
	expects = []string{"9", "123", "7:29:34PM", "456.7890", "NULL"}
	for i, expect := range expects {
		require.Equal(t, expect, vals[i])
	}

	bVal := int8(0x67)
	i16val := int16(0x16)
	i32val := int32(0x32)
	i64val := int64(0x64)
	netip := net.ParseIP("127.0.0.1")

	vals = StringFields([]any{&bVal, &i16val, &i32val, &i64val, &fVal, &netip, &NameValuePair{Name: "name", Value: `value "here"`}}, "", nil, -1)
	expects = []string{"103", "22", "50", "100", "456.789000", "127.0.0.1", `name="value \"here\""`}
	for i, expect := range expects {
		require.Equal(t, expect, vals[i])
	}

	vals = StringFields([]any{bVal, i16val, i32val, i64val, fVal, netip, NameValuePair{Name: "name", Value: "value"}}, "", nil, -1)
	expects = []string{"103", "22", "50", "100", "456.789000", "127.0.0.1", "util.NameValuePair{Name:\"name\", Value:\"value\"}"}
	for i, expect := range expects {
		require.Equal(t, expect, vals[i])
	}
}

func TestSplitSqlStatementsSingleLine(t *testing.T) {
	input := "SELECT 2 FROM T WHERE name = '--abc';"
	expect := []*SqlStatement{
		{BeginLine: 1, EndLine: 1, IsComment: false, Text: "SELECT 2 FROM T WHERE name = '--abc';", StmtType: "select", Env: &SqlStatementEnv{}},
	}
	statements, err := SplitSqlStatements(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}

	for n, stmt := range statements {
		require.EqualValues(t, expect[n], stmt, stmt.Text)
	}
}

func TestSplitSqlStatementsDoubleDashFlags(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect []*SqlStatement
	}{
		{
			name:  "explain long flag",
			input: "explain --full select * from example;",
			expect: []*SqlStatement{
				{BeginLine: 1, EndLine: 1, IsComment: false, Text: "explain --full select * from example;", StmtType: "explain", Env: &SqlStatementEnv{}},
			},
		},
		{
			name:  "show tables long flag",
			input: "show tables --all;",
			expect: []*SqlStatement{
				{BeginLine: 1, EndLine: 1, IsComment: false, Text: "show tables --all;", StmtType: "show", Env: &SqlStatementEnv{}},
			},
		},
		{
			name:  "explain keeps later comment",
			input: "explain --full select * from example -- comment\nwhere id = 1;",
			expect: []*SqlStatement{
				{BeginLine: 1, EndLine: 1, IsComment: true, Text: "-- comment", Env: &SqlStatementEnv{}},
				{BeginLine: 1, EndLine: 2, IsComment: false, Text: "explain --full select * from example where id = 1;", StmtType: "explain", Env: &SqlStatementEnv{}},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			statements, err := SplitSqlStatements(strings.NewReader(tc.input))
			require.NoError(t, err)
			require.EqualValues(t, tc.expect, statements)
		})
	}
}

func TestSplitSqlStatementsEnv(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect []*SqlStatement
	}{
		{
			name:  "bridge",
			input: "-- env: bridge=sqlite\nSELECT 1;",
			expect: []*SqlStatement{
				{BeginLine: 1, EndLine: 1, IsComment: true, Text: "-- env: bridge=sqlite", Env: &SqlStatementEnv{Bridge: "sqlite"}},
				{BeginLine: 2, EndLine: 2, IsComment: false, Text: "SELECT 1;", StmtType: "select", Env: &SqlStatementEnv{Bridge: "sqlite"}},
			},
		},
		{
			name:  "bridge and use accumulate",
			input: "-- env: bridge=sqlite\n-- env: use=mydb\nSELECT 1;",
			expect: []*SqlStatement{
				{BeginLine: 1, EndLine: 1, IsComment: true, Text: "-- env: bridge=sqlite", Env: &SqlStatementEnv{Bridge: "sqlite"}},
				{BeginLine: 2, EndLine: 2, IsComment: true, Text: "-- env: use=mydb", Env: &SqlStatementEnv{Bridge: "sqlite", Use: "mydb"}},
				{BeginLine: 3, EndLine: 3, IsComment: false, Text: "SELECT 1;", StmtType: "select", Env: &SqlStatementEnv{Bridge: "sqlite", Use: "mydb"}},
			},
		},
		{
			name:  "reset clears env",
			input: "-- env: bridge=sqlite use=mydb\n-- env: reset\nSELECT 1;",
			expect: []*SqlStatement{
				{BeginLine: 1, EndLine: 1, IsComment: true, Text: "-- env: bridge=sqlite use=mydb", Env: &SqlStatementEnv{Bridge: "sqlite", Use: "mydb"}},
				{BeginLine: 2, EndLine: 2, IsComment: true, Text: "-- env: reset", Env: &SqlStatementEnv{}},
				{BeginLine: 3, EndLine: 3, IsComment: false, Text: "SELECT 1;", StmtType: "select", Env: &SqlStatementEnv{}},
			},
		},
		{
			name:  "unknown env key",
			input: "-- env: foo=bar\nSELECT 1;",
			expect: []*SqlStatement{
				{BeginLine: 1, EndLine: 1, IsComment: true, Text: "-- env: foo=bar", Env: &SqlStatementEnv{Error: "unknown env: foo"}},
				{BeginLine: 2, EndLine: 2, IsComment: false, Text: "SELECT 1;", StmtType: "select", Env: &SqlStatementEnv{Error: "unknown env: foo"}},
			},
		},
		{
			name:  "non env comment is ignored",
			input: "-- just a comment\nSELECT 1;",
			expect: []*SqlStatement{
				{BeginLine: 1, EndLine: 1, IsComment: true, Text: "-- just a comment", Env: &SqlStatementEnv{}},
				{BeginLine: 2, EndLine: 2, IsComment: false, Text: "SELECT 1;", StmtType: "select", Env: &SqlStatementEnv{}},
			},
		},
		{
			name:  "named single pair",
			input: `-- env: named.name=Alice` + "\n" + `select * from example where name = :name;`,
			expect: []*SqlStatement{
				{BeginLine: 1, EndLine: 1, IsComment: true, Text: "-- env: named.name=Alice", Env: &SqlStatementEnv{Named: map[string]string{"name": "Alice"}}},
				{BeginLine: 2, EndLine: 2, IsComment: false, Text: "select * from example where name = :name;", StmtType: "select", Env: &SqlStatementEnv{Named: map[string]string{"name": "Alice"}}},
			},
		},
		{
			name:  "named multiple pairs with quoted values",
			input: `-- env: named.name=Alice named.from="2024-01-01" named.to="2024-01-08"` + "\n" + `select * from example where name = :name and time between :from and :to;`,
			expect: []*SqlStatement{
				{BeginLine: 1, EndLine: 1, IsComment: true, Text: `-- env: named.name=Alice named.from="2024-01-01" named.to="2024-01-08"`, Env: &SqlStatementEnv{Named: map[string]string{"name": "Alice", "from": "2024-01-01", "to": "2024-01-08"}}},
				{BeginLine: 2, EndLine: 2, IsComment: false, Text: "select * from example where name = :name and time between :from and :to;", StmtType: "select", Env: &SqlStatementEnv{Named: map[string]string{"name": "Alice", "from": "2024-01-01", "to": "2024-01-08"}}},
			},
		},
		{
			name:  "named accumulates across lines and merges with bridge/use",
			input: "-- env: bridge=sqlite named.name=Alice\n-- env: use=mydb named.from=2024-01-01\nSELECT 1;",
			expect: []*SqlStatement{
				{BeginLine: 1, EndLine: 1, IsComment: true, Text: "-- env: bridge=sqlite named.name=Alice", Env: &SqlStatementEnv{Bridge: "sqlite", Named: map[string]string{"name": "Alice"}}},
				{BeginLine: 2, EndLine: 2, IsComment: true, Text: "-- env: use=mydb named.from=2024-01-01", Env: &SqlStatementEnv{Bridge: "sqlite", Use: "mydb", Named: map[string]string{"name": "Alice", "from": "2024-01-01"}}},
				{BeginLine: 3, EndLine: 3, IsComment: false, Text: "SELECT 1;", StmtType: "select", Env: &SqlStatementEnv{Bridge: "sqlite", Use: "mydb"}},
			},
		},
		{
			name:  "named key overwrites earlier value",
			input: "-- env: named.name=Alice\n-- env: named.name=Bob\nSELECT 1;",
			expect: []*SqlStatement{
				{BeginLine: 1, EndLine: 1, IsComment: true, Text: "-- env: named.name=Alice", Env: &SqlStatementEnv{Named: map[string]string{"name": "Alice"}}},
				{BeginLine: 2, EndLine: 2, IsComment: true, Text: "-- env: named.name=Bob", Env: &SqlStatementEnv{Named: map[string]string{"name": "Bob"}}},
				{BeginLine: 3, EndLine: 3, IsComment: false, Text: "SELECT 1;", StmtType: "select", Env: &SqlStatementEnv{}},
			},
		},
		{
			name:  "named values are filtered for each statement",
			input: "-- env: named.name=my-car named.value=1.5432\nINSERT INTO example VALUES(:name, now, :value);\nSELECT * FROM example WHERE name = :name;\n-- env: reset",
			expect: []*SqlStatement{
				{BeginLine: 1, EndLine: 1, IsComment: true, Text: "-- env: named.name=my-car named.value=1.5432", Env: &SqlStatementEnv{Named: map[string]string{"name": "my-car", "value": "1.5432"}}},
				{BeginLine: 2, EndLine: 2, IsComment: false, Text: "INSERT INTO example VALUES(:name, now, :value);", StmtType: "insert", Env: &SqlStatementEnv{Named: map[string]string{"name": "my-car", "value": "1.5432"}}},
				{BeginLine: 3, EndLine: 3, IsComment: false, Text: "SELECT * FROM example WHERE name = :name;", StmtType: "select", Env: &SqlStatementEnv{Named: map[string]string{"name": "my-car"}}},
			},
		},
		{
			name:  "missing named marker is reported before execution",
			input: "-- env: named.name=my-car\nSELECT * FROM example WHERE name = :name2;",
			expect: []*SqlStatement{
				{BeginLine: 1, EndLine: 1, IsComment: true, Text: "-- env: named.name=my-car", Env: &SqlStatementEnv{Named: map[string]string{"name": "my-car"}}},
				{BeginLine: 2, EndLine: 2, IsComment: false, Text: "SELECT * FROM example WHERE name = :name2;", StmtType: "select", Env: &SqlStatementEnv{Error: `named parameter "name2" is not defined in the environment`}},
			},
		},
		{
			name:  "named marker scanner ignores literals comments and invalid candidates",
			input: "-- env: named.name=Alice named.ignored=unused\nSELECT ':ignored', \"quoted:name\", :name, :NAME, col::type, :1, :a-b;",
			expect: []*SqlStatement{
				{BeginLine: 1, EndLine: 1, IsComment: true, Text: "-- env: named.name=Alice named.ignored=unused", Env: &SqlStatementEnv{Named: map[string]string{"name": "Alice", "ignored": "unused"}}},
				{BeginLine: 2, EndLine: 2, IsComment: false, Text: "SELECT ':ignored', \"quoted:name\", :name, :NAME, col::type, :1, :a-b;", StmtType: "select", Env: &SqlStatementEnv{Named: map[string]string{"name": "Alice"}}},
			},
		},
		{
			name:  "reset clears named",
			input: "-- env: named.name=Alice\n-- env: reset\nSELECT 1;",
			expect: []*SqlStatement{
				{BeginLine: 1, EndLine: 1, IsComment: true, Text: "-- env: named.name=Alice", Env: &SqlStatementEnv{Named: map[string]string{"name": "Alice"}}},
				{BeginLine: 2, EndLine: 2, IsComment: true, Text: "-- env: reset", Env: &SqlStatementEnv{}},
				{BeginLine: 3, EndLine: 3, IsComment: false, Text: "SELECT 1;", StmtType: "select", Env: &SqlStatementEnv{}},
			},
		},
		{
			name:  "named without bind name is an error",
			input: "-- env: named.=Alice\nSELECT 1;",
			expect: []*SqlStatement{
				{BeginLine: 1, EndLine: 1, IsComment: true, Text: "-- env: named.=Alice", Env: &SqlStatementEnv{Error: "unknown env: named."}},
				{BeginLine: 2, EndLine: 2, IsComment: false, Text: "SELECT 1;", StmtType: "select", Env: &SqlStatementEnv{Error: "unknown env: named."}},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			statements, err := SplitSqlStatements(strings.NewReader(tc.input))
			require.NoError(t, err)
			require.EqualValues(t, tc.expect, statements)
		})
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
		stmts, err := SplitSqlStatements(bytes.NewReader(b))
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
		}
		expect := []map[string]any{}
		if expectContent, err := os.ReadFile(filepath.Join("testdata", tc.expectFile)); err != nil {
			t.Fatal(err, tc.inputFile)
		} else {
			if err := json.Unmarshal(expectContent, &expect); err != nil {
				t.Fatal(err, tc.inputFile, string(output))
			}
		}
		if runtime.GOOS == "windows" {
			for i, stmt := range expect {
				txt := stmt["text"].(string)
				txt = strings.ReplaceAll(txt, "\r\n", "\n")
				expect[i]["text"] = txt
			}
			for i, stmt := range result {
				txt := stmt["text"].(string)
				txt = strings.ReplaceAll(txt, "\r\n", "\n")
				result[i]["text"] = txt
			}
		}
		require.Equal(t, expect, result, string(output))
	}
}

func TestNamedMarkers(t *testing.T) {
	tests := []struct {
		name string
		text string
		want []string
	}{
		{name: "markers", text: "select :name, :from_1, :name", want: []string{"name", "from_1"}},
		{name: "case insensitive duplicate", text: "select :Name, :name", want: []string{"Name"}},
		{name: "ignored contexts", text: "select ':string', \"quoted:name\", :name -- :line\n/* :block */", want: []string{"name"}},
		{name: "invalid candidates", text: "select col::type, :1, :, :name-value", want: nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := namedMarkers(tc.text); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("namedMarkers(%q) = %#v, want %#v", tc.text, got, tc.want)
			}
		})
	}
}

func SqlTidy(sqlTextLines ...string) string {
	sqlText := strings.Join(sqlTextLines, "\n")
	lines := strings.Split(sqlText, "\n")
	for i, ln := range lines {
		lines[i] = strings.TrimSpace(ln)
	}
	return strings.Join(lines, " ")
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
	statements, err := SplitSqlStatements(strings.NewReader(input))
	if err != nil {
		fmt.Println(err)
		return
	}

	for n, stmt := range statements {
		if !stmt.IsComment {
			stmt.Text = SqlTidy(stmt.Text)
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
	input := `name1=value1 name2="value \"with\" spaces" name3=value3 name4 log-level=info`
	result := ParseNameValuePairs(input)
	for _, pair := range result {
		fmt.Printf("%s=%s\n", pair.Name, pair.Value)
	}

	// Output:
	// name1=value1
	// name2=value "with" spaces
	// name3=value3
	// name4=
	// log-level=info
}

func ExampleSplitHttpStatements() {
	input := `
POST /api/echo HTTP/1.1
Content-Type: application/json

{"key": "value"}
`
	statements, err := SplitHttpStatements(strings.NewReader(input))
	if err != nil {
		fmt.Println(err)
		return
	}
	for n, stmt := range statements {
		fmt.Println(n, stmt.BeginLine, stmt.EndLine)
		fmt.Println(stmt.Text)
	}
	// Output:
	//
	// 0 1 5
	//
	// POST /api/echo HTTP/1.1
	// Content-Type: application/json
	//
	// {"key": "value"}
}
