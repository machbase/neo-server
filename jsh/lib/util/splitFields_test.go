package util_test

import (
	"testing"

	"github.com/machbase/neo-server/v8/jsh/test_engine"
)

func TestUtilSplitFields(t *testing.T) {
	testCases := []test_engine.TestCase{
		{
			Name: "util_splitFields_basic",
			Script: `
				const {splitFields} = require("/lib/util");
				const result = splitFields("  foo   bar baz  ");
				console.println("Fields:", JSON.stringify(result));
			`,
			Output: []string{
				"Fields: [\"foo\",\"bar\",\"baz\"]",
			},
		},
		{
			Name: "util_splitFields_double_quotes",
			Script: `
				const {splitFields} = require("/lib/util");
				const result = splitFields('hello "world foo" bar');
				console.println("Fields:", JSON.stringify(result));
			`,
			Output: []string{
				"Fields: [\"hello\",\"world foo\",\"bar\"]",
			},
		},
		{
			Name: "util_splitFields_single_quotes",
			Script: `
				const {splitFields} = require("/lib/util");
				const result = splitFields("hello 'world foo' bar");
				console.println("Fields:", JSON.stringify(result));
			`,
			Output: []string{
				"Fields: [\"hello\",\"world foo\",\"bar\"]",
			},
		},
		{
			Name: "util_splitFields_mixed_quotes",
			Script: `
				const {splitFields} = require("/lib/util");
				const result = splitFields("a \"b c\" d 'e f' g");
				console.println("Fields:", JSON.stringify(result));
			`,
			Output: []string{
				"Fields: [\"a\",\"b c\",\"d\",\"e f\",\"g\"]",
			},
		},
		{
			Name: "util_splitFields_empty_string",
			Script: `
				const {splitFields} = require("/lib/util");
				const result = splitFields("");
				console.println("Fields:", JSON.stringify(result));
			`,
			Output: []string{
				"Fields: []",
			},
		},
		{
			Name: "util_splitFields_only_whitespace",
			Script: `
				const {splitFields} = require("/lib/util");
				const result = splitFields("   \t  \n  ");
				console.println("Fields:", JSON.stringify(result));
			`,
			Output: []string{
				"Fields: []",
			},
		},
		{
			Name: "util_splitFields_tabs_and_newlines",
			Script: `
				const {splitFields} = require("/lib/util");
				const result = splitFields("foo\tbar\nbaz");
				console.println("Fields:", JSON.stringify(result));
			`,
			Output: []string{
				"Fields: [\"foo\",\"bar\",\"baz\"]",
			},
		},
		{
			Name: "util_splitFields_quoted_with_tabs",
			Script: `
				const {splitFields} = require("/lib/util");
				const result = splitFields('a "b\tc" d');
				console.println("Fields:", JSON.stringify(result));
			`,
			Output: []string{
				"Fields: [\"a\",\"b\\tc\",\"d\"]",
			},
		},
		{
			Name: "util_splitFields_multiple_quoted",
			Script: `
				const {splitFields} = require("/lib/util");
				const result = splitFields('cmd "arg 1" "arg 2" "arg 3"');
				console.println("Fields:", JSON.stringify(result));
			`,
			Output: []string{
				"Fields: [\"cmd\",\"arg 1\",\"arg 2\",\"arg 3\"]",
			},
		},
		{
			Name: "util_splitFields_no_spaces",
			Script: `
				const {splitFields} = require("/lib/util");
				const result = splitFields("hello");
				console.println("Fields:", JSON.stringify(result));
			`,
			Output: []string{
				"Fields: [\"hello\"]",
			},
		},
	}
	for _, tc := range testCases {
		test_engine.RunTest(t, tc)
	}
}
