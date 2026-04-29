package usrlib_test

import (
	"testing"

	"github.com/machbase/neo-server/v8/jsh/test_engine"
)

func TestUsrLibCmdline(t *testing.T) {
	testCases := []test_engine.TestCase{
		{
			Name: "cmdline_bridge_exec_preserves_quoted_sql",
			Script: `
				const { splitCmdLine } = require('/usr/lib/cmdline');
				const env = { alias(name) { return null; } };
				const line = 'bridge exec mem "CREATE TABLE IF NOT EXISTS mem_example (id INTEGER NOT NULL PRIMARY KEY, company TEXT, employee INTEGER, discount REAL, code TEXT, valid BOOLEAN, memo BLOB, created_on DATETIME NOT NULL);"';
				const result = splitCmdLine(env, line);
				console.println(JSON.stringify(result));
			`,
			Output: []string{
				`["bridge","exec","mem","CREATE TABLE IF NOT EXISTS mem_example (id INTEGER NOT NULL PRIMARY KEY, company TEXT, employee INTEGER, discount REAL, code TEXT, valid BOOLEAN, memo BLOB, created_on DATETIME NOT NULL);"]`,
			},
		},
		{
			Name: "cmdline_bridge_query_preserves_sql_quotes",
			Script: `
				const { splitCmdLine } = require('/usr/lib/cmdline');
				const env = { alias(name) { return null; } };
				const line = 'bridge query mem "SELECT * FROM sqlite_schema WHERE name = \'mem_example\';"';
				const result = splitCmdLine(env, line);
				console.println(JSON.stringify(result));
			`,
			Output: []string{
				`["bridge","query","mem","SELECT * FROM sqlite_schema WHERE name = 'mem_example';"]`,
			},
		},
		{
			Name: "cmdline_sql_command_preserves_quoted_sql",
			Script: `
				const { splitCmdLine } = require('/usr/lib/cmdline');
				const env = { alias(name) { return null; } };
				const line = 'sql -T --format csv "SELECT * FROM sqlite_schema WHERE name = \'mem_example\';"';
				const result = splitCmdLine(env, line);
				console.println(JSON.stringify(result));
			`,
			Output: []string{
				`["sql","-T","--format","csv","SELECT * FROM sqlite_schema WHERE name = 'mem_example';"]`,
			},
		},
		{
			Name: "cmdline_sql_command_preserves_unquoted_sql",
			Script: `
				const { splitCmdLine } = require('/usr/lib/cmdline');
				const env = { alias(name) { return null; } };
				const line = 'sql -T --format csv SELECT * FROM sqlite_schema WHERE name = \'mem_example\';';
				const result = splitCmdLine(env, line);
				console.println(JSON.stringify(result));
			`,
			Output: []string{
				`["sql","-T","--format","csv","SELECT * FROM sqlite_schema WHERE name = 'mem_example';"]`,
			},
		},
		{
			Name: "cmdline_implicit_sql_command_normalizes_to_sql",
			Script: `
				const { splitCmdLine } = require('/usr/lib/cmdline');
				const env = { alias(name) { return null; } };
				const line = 'SELECT * FROM sqlite_schema WHERE name = \'mem_example\';';
				const result = splitCmdLine(env, line);
				console.println(JSON.stringify(result));
			`,
			Output: []string{
				`["sql","SELECT * FROM sqlite_schema WHERE name = 'mem_example';"]`,
			},
		},
		{
			Name: "cmdline_explain_command_preserves_quoted_sql",
			Script: `
				const { splitCmdLine } = require('/usr/lib/cmdline');
				const env = { alias(name) { return null; } };
				const line = 'explain -T --format csv "SELECT * FROM sqlite_schema WHERE name = \'mem_example\';"';
				const result = splitCmdLine(env, line);
				console.println(JSON.stringify(result));
			`,
			Output: []string{
				`["explain","-T","--format","csv","SELECT * FROM sqlite_schema WHERE name = 'mem_example';"]`,
			},
		},
		{
			Name: "cmdline_explain_command_preserves_unquoted_sql",
			Script: `
				const { splitCmdLine } = require('/usr/lib/cmdline');
				const env = { alias(name) { return null; } };
				const line = 'explain -T --format csv SELECT * FROM sqlite_schema WHERE name = \'mem_example\';';
				const result = splitCmdLine(env, line);
				console.println(JSON.stringify(result));
			`,
			Output: []string{
				`["explain","-T","--format","csv","SELECT * FROM sqlite_schema WHERE name = 'mem_example';"]`,
			},
		},
	}

	for _, tc := range testCases {
		test_engine.RunTest(t, tc)
	}
}
