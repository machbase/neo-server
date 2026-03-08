package util_test

import (
	"testing"

	"github.com/machbase/neo-server/v8/jsh/test_engine"
)

func TestUtilParseArgs(t *testing.T) {
	testCases := []test_engine.TestCase{
		{
			Name: "util_parseArgs_basic",
			Script: `
				const {parseArgs} = require("util");
				const result = parseArgs(['-f', '--bar', 'value', 'positional'], {
					options: {
						foo: { type: 'boolean', short: 'f' },
						bar: { type: 'string' }
					},
					allowPositionals: true
				});
				console.println("Values:", JSON.stringify(result.values));
				console.println("Positionals:", JSON.stringify(result.positionals));
			`,
			Output: []string{
				"Values: {\"foo\":true,\"bar\":\"value\"}",
				"Positionals: [\"positional\"]",
			},
		},
		{
			Name: "util_parseArgs_long_options",
			Script: `
				const {parseArgs} = require("util");
				const result = parseArgs(['--verbose', '--output', 'file.txt'], {
					options: {
						verbose: { type: 'boolean' },
						output: { type: 'string' }
					}
				});
				console.println("Values:", JSON.stringify(result.values));
			`,
			Output: []string{
				"Values: {\"verbose\":true,\"output\":\"file.txt\"}",
			},
		},
		{
			Name: "util_parseArgs_short_options",
			Script: `
				const {parseArgs} = require("util");
				const result = parseArgs(['-v', '-o', 'out.txt'], {
					options: {
						verbose: { type: 'boolean', short: 'v' },
						output: { type: 'string', short: 'o' }
					}
				});
				console.println("Values:", JSON.stringify(result.values));
			`,
			Output: []string{
				"Values: {\"verbose\":true,\"output\":\"out.txt\"}",
			},
		},
		{
			Name: "util_parseArgs_inline_value",
			Script: `
				const {parseArgs} = require("util");
				const result = parseArgs(['--output=file.txt', '-o=out.txt'], {
					options: {
						output: { type: 'string', short: 'o' }
					}
				});
				console.println("Values:", JSON.stringify(result.values));
			`,
			Output: []string{
				"Values: {\"output\":\"out.txt\"}",
			},
		},
		{
			Name: "util_parseArgs_multiple",
			Script: `
				const {parseArgs} = require("util");
				const result = parseArgs(['--include', 'a.js', '--include', 'b.js', '-I', 'c.js'], {
					options: {
						include: { type: 'string', short: 'I', multiple: true }
					}
				});
				console.println("Values:", JSON.stringify(result.values));
			`,
			Output: []string{
				"Values: {\"include\":[\"a.js\",\"b.js\",\"c.js\"]}",
			},
		},
		{
			Name: "util_parseArgs_default_values",
			Script: `
				const {parseArgs} = require("util");
				const result = parseArgs(['--foo'], {
					options: {
						foo: { type: 'boolean' },
						bar: { type: 'string', default: 'default_value' },
						count: { type: 'string', default: '0' }
					}
				});
				console.println("Values:", JSON.stringify(result.values));
			`,
			Output: []string{
				"Values: {\"bar\":\"default_value\",\"count\":\"0\",\"foo\":true}",
			},
		},
		{
			Name: "util_parseArgs_short_group",
			Script: `
				const {parseArgs} = require("util");
				const result = parseArgs(['-abc'], {
					options: {
						a: { type: 'boolean', short: 'a' },
						b: { type: 'boolean', short: 'b' },
						c: { type: 'boolean', short: 'c' }
					}
				});
				console.println("Values:", JSON.stringify(result.values));
			`,
			Output: []string{
				"Values: {\"a\":true,\"b\":true,\"c\":true}",
			},
		},
		{
			Name: "util_parseArgs_terminator",
			Script: `
				const {parseArgs} = require("util");
				const result = parseArgs(['--foo', '--', '--bar', 'baz'], {
					options: {
						foo: { type: 'boolean' },
						bar: { type: 'boolean' }
					},
					allowPositionals: true
				});
				console.println("Values:", JSON.stringify(result.values));
				console.println("Positionals:", JSON.stringify(result.positionals));
			`,
			Output: []string{
				"Values: {\"foo\":true}",
				"Positionals: [\"--bar\",\"baz\"]",
			},
		},
		{
			Name: "util_parseArgs_allow_negative",
			Script: `
				const {parseArgs} = require("util");
				const result = parseArgs(['--no-color', '--verbose'], {
					options: {
						color: { type: 'boolean' },
						verbose: { type: 'boolean' }
					},
					allowNegative: true
				});
				console.println("Values:", JSON.stringify(result.values));
			`,
			Output: []string{
				"Values: {\"color\":false,\"verbose\":true}",
			},
		},
		{
			Name: "util_parseArgs_tokens",
			Script: `
				const {parseArgs} = require("util");
				const result = parseArgs(['-f', '--bar', 'value'], {
					options: {
						foo: { type: 'boolean', short: 'f' },
						bar: { type: 'string' }
					},
					tokens: true
				});
				console.println("Token count:", result.tokens.length);
				console.println("First token kind:", result.tokens[0].kind);
				console.println("First token name:", result.tokens[0].name);
			`,
			Output: []string{
				"Token count: 2",
				"First token kind: option",
				"First token name: foo",
			},
		},
		{
			Name: "util_parseArgs_old_signature",
			Script: `
				const {parseArgs} = require("util");
				const result = parseArgs(['-v', '--output', 'file.txt'], {
					options: {
						verbose: { type: 'boolean', short: 'v' },
						output: { type: 'string' }
					}
				});
				console.println("Values:", JSON.stringify(result.values));
			`,
			Output: []string{
				"Values: {\"verbose\":true,\"output\":\"file.txt\"}",
			},
		},
		{
			Name: "util_parseArgs_named_positionals",
			Script: `
				const {parseArgs} = require("util");
				const result = parseArgs(['input.txt', 'output.txt'], {
					options: {},
					allowPositionals: true,
					positionals: ['inputFile', 'outputFile']
				});
				console.println("Positionals:", JSON.stringify(result.positionals));
				console.println("Named:", JSON.stringify(result.namedPositionals));
			`,
			Output: []string{
				"Positionals: [\"input.txt\",\"output.txt\"]",
				"Named: {\"inputFile\":\"input.txt\",\"outputFile\":\"output.txt\"}",
			},
		},
		{
			Name: "util_parseArgs_optional_positionals",
			Script: `
				const {parseArgs} = require("util");
				const result = parseArgs(['input.txt'], {
					options: {},
					allowPositionals: true,
					positionals: [
						'inputFile',
						{ name: 'outputFile', optional: true, default: 'stdout' }
					]
				});
				console.println("Named:", JSON.stringify(result.namedPositionals));
			`,
			Output: []string{
				"Named: {\"inputFile\":\"input.txt\",\"outputFile\":\"stdout\"}",
			},
		},
		{
			Name: "util_parseArgs_variadic_positionals",
			Script: `
				const {parseArgs} = require("util");
				const result = parseArgs(['input.txt', 'out.txt', 'a.js', 'b.js'], {
					options: {},
					allowPositionals: true,
					positionals: [
						'inputFile',
						'outputFile',
						{ name: 'files', variadic: true }
					]
				});
				console.println("Named:", JSON.stringify(result.namedPositionals));
			`,
			Output: []string{
				"Named: {\"inputFile\":\"input.txt\",\"outputFile\":\"out.txt\",\"files\":[\"a.js\",\"b.js\"]}",
			},
		},
	}
	for _, tc := range testCases {
		test_engine.RunTest(t, tc)
	}
}
