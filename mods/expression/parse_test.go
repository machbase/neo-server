package expression

import (
	"bytes"
	"fmt"
	"reflect"
	"testing"
	"time"
	"unicode"
)

// Represents a test of parsing all tokens correctly from a string
type TokenParsingTest struct {
	Name      string
	Input     string
	Functions map[string]Function
	Expected  []Token
}

func TestConstantParsing(test *testing.T) {
	ParseStringToTime = true
	tokenParsingTests := []TokenParsingTest{
		{
			Name:  "Single numeric",
			Input: "1",
			Expected: []Token{
				{
					Kind:  NUMERIC,
					Value: 1.0,
				},
			},
		},
		{
			Name:  "Single two-digit numeric",
			Input: "50",
			Expected: []Token{
				{
					Kind:  NUMERIC,
					Value: 50.0,
				},
			},
		},
		{
			Name:  "Zero",
			Input: "0",
			Expected: []Token{
				{
					Kind:  NUMERIC,
					Value: 0.0,
				},
			},
		},
		{
			Name:  "One digit hex",
			Input: "0x1",
			Expected: []Token{
				{
					Kind:  NUMERIC,
					Value: 1.0,
				},
			},
		},
		{
			Name:  "Two digit hex",
			Input: "0x10",
			Expected: []Token{
				{
					Kind:  NUMERIC,
					Value: 16.0,
				},
			},
		},
		{
			Name:  "Hex with lowercase",
			Input: "0xabcdef",
			Expected: []Token{
				{
					Kind:  NUMERIC,
					Value: 11259375.0,
				},
			},
		},
		{
			Name:  "Hex with uppercase",
			Input: "0xABCDEF",
			Expected: []Token{
				{
					Kind:  NUMERIC,
					Value: 11259375.0,
				},
			},
		},
		{
			Name:  "Single string",
			Input: "'foo'",
			Expected: []Token{
				{
					Kind:  STRING,
					Value: "foo",
				},
			},
		},
		{
			Name:  "Single time, RFC3339, only date",
			Input: "'2014-01-02'",
			Expected: []Token{
				{
					Kind:  TIME,
					Value: time.Date(2014, time.January, 2, 0, 0, 0, 0, time.Local),
				},
			},
		},
		{
			Name:  "Single time, RFC3339, with hh:mm",
			Input: "'2014-01-02 14:12'",
			Expected: []Token{
				{
					Kind:  TIME,
					Value: time.Date(2014, time.January, 2, 14, 12, 0, 0, time.Local),
				},
			},
		},
		{
			Name:  "Single time, RFC3339, with hh:mm:ss",
			Input: "'2014-01-02 14:12:22'",
			Expected: []Token{
				{
					Kind:  TIME,
					Value: time.Date(2014, time.January, 2, 14, 12, 22, 0, time.Local),
				},
			},
		},
		{
			Name:  "Single boolean",
			Input: "true",
			Expected: []Token{
				{
					Kind:  BOOLEAN,
					Value: true,
				},
			},
		},
		{
			Name:  "Single large numeric",
			Input: "1234567890",
			Expected: []Token{
				{
					Kind:  NUMERIC,
					Value: 1234567890.0,
				},
			},
		},
		{
			Name:  "Single floating-point",
			Input: "0.5",
			Expected: []Token{
				{
					Kind:  NUMERIC,
					Value: 0.5,
				},
			},
		},
		{
			Name:  "Single large floating point",
			Input: "3.14567471",
			Expected: []Token{
				{
					Kind:  NUMERIC,
					Value: 3.14567471,
				},
			},
		},
		{
			Name:  "Single false boolean",
			Input: "false",
			Expected: []Token{
				{
					Kind:  BOOLEAN,
					Value: false,
				},
			},
		},
		{
			Name:  "Single internationalized string",
			Input: "'ÆŦǽഈᚥஇคٸ'",
			Expected: []Token{
				{
					Kind:  STRING,
					Value: "ÆŦǽഈᚥஇคٸ",
				},
			},
		},
		{
			Name:  "Single internationalized parameter",
			Input: "ÆŦǽഈᚥஇคٸ",
			Expected: []Token{
				{
					Kind:  VARIABLE,
					Value: "ÆŦǽഈᚥஇคٸ",
				},
			},
		},
		{
			Name:      "Parameterless function",
			Input:     "foo()",
			Functions: map[string]Function{"foo": noop},
			Expected: []Token{
				{
					Kind:  FUNCTION,
					Value: noop,
				},
				{
					Kind: CLAUSE,
				},
				{
					Kind: CLAUSE_CLOSE,
				},
			},
		},
		{
			Name:      "Single parameter function",
			Input:     "foo('bar')",
			Functions: map[string]Function{"foo": noop},
			Expected: []Token{
				{
					Kind:  FUNCTION,
					Value: noop,
				},
				{
					Kind: CLAUSE,
				},
				{
					Kind:  STRING,
					Value: "bar",
				},
				{
					Kind: CLAUSE_CLOSE,
				},
			},
		},
		{
			Name:      "Multiple parameter function",
			Input:     "foo('bar', 1.0)",
			Functions: map[string]Function{"foo": noop},
			Expected: []Token{
				{
					Kind:  FUNCTION,
					Value: noop,
				},
				{
					Kind: CLAUSE,
				},
				{
					Kind:  STRING,
					Value: "bar",
				},
				{
					Kind: SEPARATOR,
				},
				{
					Kind:  NUMERIC,
					Value: 1.0,
				},
				{
					Kind: CLAUSE_CLOSE,
				},
			},
		},
		{
			Name:      "Nested function",
			Input:     "foo(foo('bar'), 1.0, foo(2.0))",
			Functions: map[string]Function{"foo": noop},
			Expected: []Token{
				{
					Kind:  FUNCTION,
					Value: noop,
				},
				{
					Kind: CLAUSE,
				},
				{
					Kind:  FUNCTION,
					Value: noop,
				},
				{
					Kind: CLAUSE,
				},
				{
					Kind:  STRING,
					Value: "bar",
				},
				{
					Kind: CLAUSE_CLOSE,
				},
				{
					Kind: SEPARATOR,
				},
				{
					Kind:  NUMERIC,
					Value: 1.0,
				},
				{
					Kind: SEPARATOR,
				},
				{
					Kind:  FUNCTION,
					Value: noop,
				},
				{
					Kind: CLAUSE,
				},
				{
					Kind:  NUMERIC,
					Value: 2.0,
				},
				{
					Kind: CLAUSE_CLOSE,
				},
				{
					Kind: CLAUSE_CLOSE,
				},
			},
		},
		{
			Name:      "Function with modifier afterwards (#28)",
			Input:     "foo() + 1",
			Functions: map[string]Function{"foo": noop},
			Expected: []Token{
				{
					Kind:  FUNCTION,
					Value: noop,
				},
				{
					Kind: CLAUSE,
				},
				{
					Kind: CLAUSE_CLOSE,
				},
				{
					Kind:  MODIFIER,
					Value: "+",
				},
				{
					Kind:  NUMERIC,
					Value: 1.0,
				},
			},
		},
		{
			Name:      "Function with modifier afterwards and comparator",
			Input:     "(foo()-1) > 3",
			Functions: map[string]Function{"foo": noop},
			Expected: []Token{
				{
					Kind: CLAUSE,
				},
				{
					Kind:  FUNCTION,
					Value: noop,
				},
				{
					Kind: CLAUSE,
				},
				{
					Kind: CLAUSE_CLOSE,
				},
				{
					Kind:  MODIFIER,
					Value: "-",
				},
				{
					Kind:  NUMERIC,
					Value: 1.0,
				},
				{
					Kind: CLAUSE_CLOSE,
				},
				{
					Kind:  COMPARATOR,
					Value: ">",
				},
				{
					Kind:  NUMERIC,
					Value: 3.0,
				},
			},
		},
		{
			Name:  "Double-quoted string added to square-brackted param (#59)",
			Input: "\"a\" + [foo]",
			Expected: []Token{
				{
					Kind:  STRING,
					Value: "a",
				},
				{
					Kind:  MODIFIER,
					Value: "+",
				},
				{
					Kind:  VARIABLE,
					Value: "foo",
				},
			},
		},
		{
			Name:  "Accessor variable",
			Input: "foo.Var",
			Expected: []Token{
				{
					Kind:  ACCESSOR,
					Value: []string{"foo", "Var"},
				},
			},
		},
		{
			Name:  "Accessor function",
			Input: "foo.Operation()",
			Expected: []Token{
				{
					Kind:  ACCESSOR,
					Value: []string{"foo", "Operation"},
				},
				{
					Kind: CLAUSE,
				},
				{
					Kind: CLAUSE_CLOSE,
				},
			},
		},
	}

	tokenParsingTests = combineWhitespaceExpressions(tokenParsingTests)
	runTokenParsingTest(tokenParsingTests, test)
}

func TestScriptBlock(test *testing.T) {
	tokenParsingTests := []TokenParsingTest{
		{
			Name:      "Block with script",
			Input:     "script({l = a + b\n return l\n})",
			Functions: map[string]Function{"script": noop},
			Expected: []Token{
				{
					Kind:  FUNCTION,
					Value: noop,
				},
				{
					Kind: CLAUSE,
				},
				{
					Kind:  BLOCK,
					Value: "l = a + b\n return l\n",
				},
				{
					Kind: CLAUSE_CLOSE,
				},
			},
		},
		{
			Name:      "Block with script",
			Input:     "script('tengo', {l = a + b\n return l\n})",
			Functions: map[string]Function{"script": noop},
			Expected: []Token{
				{
					Kind:  FUNCTION,
					Value: noop,
				},
				{
					Kind: CLAUSE,
				},
				{
					Kind:  STRING,
					Value: "tengo",
				},
				{
					Kind: SEPARATOR,
				},
				{
					Kind:  BLOCK,
					Value: "l = a + b\n return l\n",
				},
				{
					Kind: CLAUSE_CLOSE,
				},
			},
		},
	}
	runTokenParsingTest(tokenParsingTests, test)
}

func TestLogicalOperatorParsing(test *testing.T) {
	tokenParsingTests := []TokenParsingTest{
		{
			Name:  "Boolean AND",
			Input: "true && false",
			Expected: []Token{
				{
					Kind:  BOOLEAN,
					Value: true,
				},
				{
					Kind:  LOGICALOP,
					Value: "&&",
				},
				{
					Kind:  BOOLEAN,
					Value: false,
				},
			},
		},
		{
			Name:  "Boolean OR",
			Input: "true || false",
			Expected: []Token{
				{
					Kind:  BOOLEAN,
					Value: true,
				},
				{
					Kind:  LOGICALOP,
					Value: "||",
				},
				{
					Kind:  BOOLEAN,
					Value: false,
				},
			},
		},
		{
			Name:  "Multiple logical operators",
			Input: "true || false && true",
			Expected: []Token{
				{
					Kind:  BOOLEAN,
					Value: true,
				},
				{
					Kind:  LOGICALOP,
					Value: "||",
				},
				{
					Kind:  BOOLEAN,
					Value: false,
				},
				{
					Kind:  LOGICALOP,
					Value: "&&",
				},
				{
					Kind:  BOOLEAN,
					Value: true,
				},
			},
		},
	}

	tokenParsingTests = combineWhitespaceExpressions(tokenParsingTests)
	runTokenParsingTest(tokenParsingTests, test)
}

func TestComparatorParsing(test *testing.T) {
	tokenParsingTests := []TokenParsingTest{
		{
			Name:  "Numeric EQ",
			Input: "1 == 2",
			Expected: []Token{
				{
					Kind:  NUMERIC,
					Value: 1.0,
				},
				{
					Kind:  COMPARATOR,
					Value: "==",
				},
				{
					Kind:  NUMERIC,
					Value: 2.0,
				},
			},
		},
		{
			Name:  "Numeric NEQ",
			Input: "1 != 2",
			Expected: []Token{
				{
					Kind:  NUMERIC,
					Value: 1.0,
				},
				{
					Kind:  COMPARATOR,
					Value: "!=",
				},
				{
					Kind:  NUMERIC,
					Value: 2.0,
				},
			},
		},
		{
			Name:  "Numeric GT",
			Input: "1 > 0",
			Expected: []Token{
				{
					Kind:  NUMERIC,
					Value: 1.0,
				},
				{
					Kind:  COMPARATOR,
					Value: ">",
				},
				{
					Kind:  NUMERIC,
					Value: 0.0,
				},
			},
		},
		{
			Name:  "Numeric LT",
			Input: "1 < 2",
			Expected: []Token{
				{
					Kind:  NUMERIC,
					Value: 1.0,
				},
				{
					Kind:  COMPARATOR,
					Value: "<",
				},
				{
					Kind:  NUMERIC,
					Value: 2.0,
				},
			},
		},
		{
			Name:  "Numeric GTE",
			Input: "1 >= 2",
			Expected: []Token{
				{
					Kind:  NUMERIC,
					Value: 1.0,
				},
				{
					Kind:  COMPARATOR,
					Value: ">=",
				},
				{
					Kind:  NUMERIC,
					Value: 2.0,
				},
			},
		},
		{
			Name:  "Numeric LTE",
			Input: "1 <= 2",
			Expected: []Token{
				{
					Kind:  NUMERIC,
					Value: 1.0,
				},
				{
					Kind:  COMPARATOR,
					Value: "<=",
				},
				{
					Kind:  NUMERIC,
					Value: 2.0,
				},
			},
		},
		{
			Name:  "String LT",
			Input: "'ab.cd' < 'abc.def'",
			Expected: []Token{
				{
					Kind:  STRING,
					Value: "ab.cd",
				},
				{
					Kind:  COMPARATOR,
					Value: "<",
				},
				{
					Kind:  STRING,
					Value: "abc.def",
				},
			},
		},
		{
			Name:  "String LTE",
			Input: "'ab.cd' <= 'abc.def'",
			Expected: []Token{
				{
					Kind:  STRING,
					Value: "ab.cd",
				},
				{
					Kind:  COMPARATOR,
					Value: "<=",
				},
				{
					Kind:  STRING,
					Value: "abc.def",
				},
			},
		},
		{
			Name:  "String GT",
			Input: "'ab.cd' > 'abc.def'",
			Expected: []Token{
				{
					Kind:  STRING,
					Value: "ab.cd",
				},
				{
					Kind:  COMPARATOR,
					Value: ">",
				},
				{
					Kind:  STRING,
					Value: "abc.def",
				},
			},
		},
		{
			Name:  "String GTE",
			Input: "'ab.cd' >= 'abc.def'",
			Expected: []Token{
				{
					Kind:  STRING,
					Value: "ab.cd",
				},
				{
					Kind:  COMPARATOR,
					Value: ">=",
				},
				{
					Kind:  STRING,
					Value: "abc.def",
				},
			},
		},
		{
			Name:  "String REQ",
			Input: "'foobar' =~ 'bar'",
			Expected: []Token{
				{
					Kind:  STRING,
					Value: "foobar",
				},
				{
					Kind:  COMPARATOR,
					Value: "=~",
				},

				// it's not particularly clean to test for the contents of a pattern, (since it means modifying the harness below)
				// so pattern contents are left untested.
				{
					Kind: PATTERN,
				},
			},
		},
		{
			Name:  "String NREQ",
			Input: "'foobar' !~ 'bar'",
			Expected: []Token{
				{
					Kind:  STRING,
					Value: "foobar",
				},
				{
					Kind:  COMPARATOR,
					Value: "!~",
				},
				{
					Kind: PATTERN,
				},
			},
		},
		{
			Name:  "Comparator against modifier string additive (#22)",
			Input: "'foo' == '+'",
			Expected: []Token{
				{
					Kind:  STRING,
					Value: "foo",
				},
				{
					Kind:  COMPARATOR,
					Value: "==",
				},
				{
					Kind:  STRING,
					Value: "+",
				},
			},
		},
		{
			Name:  "Comparator against modifier string multiplicative (#22)",
			Input: "'foo' == '/'",
			Expected: []Token{
				{
					Kind:  STRING,
					Value: "foo",
				},
				{
					Kind:  COMPARATOR,
					Value: "==",
				},
				{
					Kind:  STRING,
					Value: "/",
				},
			},
		},
		{
			Name:  "Comparator against modifier string exponential (#22)",
			Input: "'foo' == '**'",
			Expected: []Token{
				{
					Kind:  STRING,
					Value: "foo",
				},
				{
					Kind:  COMPARATOR,
					Value: "==",
				},
				{
					Kind:  STRING,
					Value: "**",
				},
			},
		},
		{
			Name:  "Comparator against modifier string bitwise (#22)",
			Input: "'foo' == '^'",
			Expected: []Token{
				{
					Kind:  STRING,
					Value: "foo",
				},
				{
					Kind:  COMPARATOR,
					Value: "==",
				},
				{
					Kind:  STRING,
					Value: "^",
				},
			},
		},
		{
			Name:  "Comparator against modifier string shift (#22)",
			Input: "'foo' == '>>'",
			Expected: []Token{
				{
					Kind:  STRING,
					Value: "foo",
				},
				{
					Kind:  COMPARATOR,
					Value: "==",
				},
				{
					Kind:  STRING,
					Value: ">>",
				},
			},
		},
		{
			Name:  "Comparator against modifier string ternary (#22)",
			Input: "'foo' == '?'",
			Expected: []Token{
				{
					Kind:  STRING,
					Value: "foo",
				},
				{
					Kind:  COMPARATOR,
					Value: "==",
				},
				{
					Kind:  STRING,
					Value: "?",
				},
			},
		},
		{
			Name:  "Array membership lowercase",
			Input: "'foo' in ('foo', 'bar')",
			Expected: []Token{
				{
					Kind:  STRING,
					Value: "foo",
				},
				{
					Kind:  COMPARATOR,
					Value: "in",
				},
				{
					Kind: CLAUSE,
				},
				{
					Kind:  STRING,
					Value: "foo",
				},
				{
					Kind: SEPARATOR,
				},
				{
					Kind:  STRING,
					Value: "bar",
				},
				{
					Kind: CLAUSE_CLOSE,
				},
			},
		},
		{
			Name:  "Array membership uppercase",
			Input: "'foo' IN ('foo', 'bar')",
			Expected: []Token{
				{
					Kind:  STRING,
					Value: "foo",
				},
				{
					Kind:  COMPARATOR,
					Value: "in",
				},
				{
					Kind: CLAUSE,
				},
				{
					Kind:  STRING,
					Value: "foo",
				},
				{
					Kind: SEPARATOR,
				},
				{
					Kind:  STRING,
					Value: "bar",
				},
				{
					Kind: CLAUSE_CLOSE,
				},
			},
		},
	}

	tokenParsingTests = combineWhitespaceExpressions(tokenParsingTests)
	runTokenParsingTest(tokenParsingTests, test)
}

func TestModifierParsing(test *testing.T) {
	tokenParsingTests := []TokenParsingTest{
		{
			Name:  "Numeric PLUS",
			Input: "1 + 1",
			Expected: []Token{
				{
					Kind:  NUMERIC,
					Value: 1.0,
				},
				{
					Kind:  MODIFIER,
					Value: "+",
				},
				{
					Kind:  NUMERIC,
					Value: 1.0,
				},
			},
		},
		{
			Name:  "Numeric MINUS",
			Input: "1 - 1",
			Expected: []Token{
				{
					Kind:  NUMERIC,
					Value: 1.0,
				},
				{
					Kind:  MODIFIER,
					Value: "-",
				},
				{
					Kind:  NUMERIC,
					Value: 1.0,
				},
			},
		},
		{
			Name:  "Numeric MULTIPLY",
			Input: "1 * 1",
			Expected: []Token{
				{
					Kind:  NUMERIC,
					Value: 1.0,
				},
				{
					Kind:  MODIFIER,
					Value: "*",
				},
				{
					Kind:  NUMERIC,
					Value: 1.0,
				},
			},
		},
		{
			Name:  "Numeric DIVIDE",
			Input: "1 / 1",
			Expected: []Token{
				{
					Kind:  NUMERIC,
					Value: 1.0,
				},
				{
					Kind:  MODIFIER,
					Value: "/",
				},
				{
					Kind:  NUMERIC,
					Value: 1.0,
				},
			},
		},
		{
			Name:  "Numeric MODULUS",
			Input: "1 % 1",
			Expected: []Token{
				{
					Kind:  NUMERIC,
					Value: 1.0,
				},
				{
					Kind:  MODIFIER,
					Value: "%",
				},
				{
					Kind:  NUMERIC,
					Value: 1.0,
				},
			},
		},
		{
			Name:  "Numeric BITWISE_AND",
			Input: "1 & 1",
			Expected: []Token{
				{
					Kind:  NUMERIC,
					Value: 1.0,
				},
				{
					Kind:  MODIFIER,
					Value: "&",
				},
				{
					Kind:  NUMERIC,
					Value: 1.0,
				},
			},
		},
		{
			Name:  "Numeric BITWISE_OR",
			Input: "1 | 1",
			Expected: []Token{
				{
					Kind:  NUMERIC,
					Value: 1.0,
				},
				{
					Kind:  MODIFIER,
					Value: "|",
				},
				{
					Kind:  NUMERIC,
					Value: 1.0,
				},
			},
		},
		{
			Name:  "Numeric BITWISE_XOR",
			Input: "1 ^ 1",
			Expected: []Token{
				{
					Kind:  NUMERIC,
					Value: 1.0,
				},
				{
					Kind:  MODIFIER,
					Value: "^",
				},
				{
					Kind:  NUMERIC,
					Value: 1.0,
				},
			},
		},
		{
			Name:  "Numeric BITWISE_LSHIFT",
			Input: "1 << 1",
			Expected: []Token{
				{
					Kind:  NUMERIC,
					Value: 1.0,
				},
				{
					Kind:  MODIFIER,
					Value: "<<",
				},
				{
					Kind:  NUMERIC,
					Value: 1.0,
				},
			},
		},
		{
			Name:  "Numeric BITWISE_RSHIFT",
			Input: "1 >> 1",
			Expected: []Token{
				{
					Kind:  NUMERIC,
					Value: 1.0,
				},
				{
					Kind:  MODIFIER,
					Value: ">>",
				},
				{
					Kind:  NUMERIC,
					Value: 1.0,
				},
			},
		},
	}

	tokenParsingTests = combineWhitespaceExpressions(tokenParsingTests)
	runTokenParsingTest(tokenParsingTests, test)
}

func TestPrefixParsing(test *testing.T) {
	testCases := []TokenParsingTest{
		{
			Name:  "Sign prefix",
			Input: "-1",
			Expected: []Token{
				{
					Kind:  PREFIX,
					Value: "-",
				},
				{
					Kind:  NUMERIC,
					Value: 1.0,
				},
			},
		},
		{
			Name:  "Sign prefix on variable",
			Input: "-foo",
			Expected: []Token{
				{
					Kind:  PREFIX,
					Value: "-",
				},
				{
					Kind:  VARIABLE,
					Value: "foo",
				},
			},
		},
		{
			Name:  "Boolean prefix",
			Input: "!true",
			Expected: []Token{
				{
					Kind:  PREFIX,
					Value: "!",
				},
				{
					Kind:  BOOLEAN,
					Value: true,
				},
			},
		},
		{
			Name:  "Boolean prefix on variable",
			Input: "!foo",
			Expected: []Token{
				{
					Kind:  PREFIX,
					Value: "!",
				},
				{
					Kind:  VARIABLE,
					Value: "foo",
				},
			},
		},
		{
			Name:  "Bitwise not prefix",
			Input: "~1",
			Expected: []Token{
				{
					Kind:  PREFIX,
					Value: "~",
				},
				{
					Kind:  NUMERIC,
					Value: 1.0,
				},
			},
		},
		{
			Name:  "Bitwise not prefix on variable",
			Input: "~foo",
			Expected: []Token{
				{
					Kind:  PREFIX,
					Value: "~",
				},
				{
					Kind:  VARIABLE,
					Value: "foo",
				},
			},
		},
	}

	testCases = combineWhitespaceExpressions(testCases)
	runTokenParsingTest(testCases, test)
}

func TestEscapedParameters(test *testing.T) {
	testCases := []TokenParsingTest{
		{
			Name:  "Single escaped parameter",
			Input: "[foo]",
			Expected: []Token{
				{
					Kind:  VARIABLE,
					Value: "foo",
				},
			},
		},
		{
			Name:  "Single escaped parameter with whitespace",
			Input: "[foo bar]",
			Expected: []Token{
				{
					Kind:  VARIABLE,
					Value: "foo bar",
				},
			},
		},
		{
			Name:  "Single escaped parameter with escaped closing bracket",
			Input: "[foo[bar\\]]",
			Expected: []Token{
				{
					Kind:  VARIABLE,
					Value: "foo[bar]",
				},
			},
		},
		{

			Name:  "Escaped parameters and unescaped parameters",
			Input: "[foo] > bar",
			Expected: []Token{
				{
					Kind:  VARIABLE,
					Value: "foo",
				},
				{
					Kind:  COMPARATOR,
					Value: ">",
				},
				{
					Kind:  VARIABLE,
					Value: "bar",
				},
			},
		},
		{

			Name:  "Unescaped parameter with space",
			Input: "foo\\ bar > bar",
			Expected: []Token{
				{
					Kind:  VARIABLE,
					Value: "foo bar",
				},
				{
					Kind:  COMPARATOR,
					Value: ">",
				},
				{
					Kind:  VARIABLE,
					Value: "bar",
				},
			},
		},
		{
			Name:  "Unescaped parameter with space",
			Input: "response\\-time > bar",
			Expected: []Token{
				{
					Kind:  VARIABLE,
					Value: "response-time",
				},
				{
					Kind:  COMPARATOR,
					Value: ">",
				},
				{
					Kind:  VARIABLE,
					Value: "bar",
				},
			},
		},
		{
			Name:  "Parameters with snake_case",
			Input: "foo_bar > baz_quux",
			Expected: []Token{
				{
					Kind:  VARIABLE,
					Value: "foo_bar",
				},
				{
					Kind:  COMPARATOR,
					Value: ">",
				},
				{
					Kind:  VARIABLE,
					Value: "baz_quux",
				},
			},
		},
		{
			Name:  "String literal uses backslash to escape",
			Input: "\"foo\\'bar\"",
			Expected: []Token{
				{
					Kind:  STRING,
					Value: "foo'bar",
				},
			},
		},
	}

	runTokenParsingTest(testCases, test)
}

func TestTernaryParsing(test *testing.T) {
	tokenParsingTests := []TokenParsingTest{
		{
			Name:  "Ternary after Boolean",
			Input: "true ? 1",
			Expected: []Token{
				{
					Kind:  BOOLEAN,
					Value: true,
				},
				{
					Kind:  TERNARY,
					Value: "?",
				},
				{
					Kind:  NUMERIC,
					Value: 1.0,
				},
			},
		},
		{
			Name:  "Ternary after Comperator",
			Input: "1 == 0 ? true",
			Expected: []Token{
				{
					Kind:  NUMERIC,
					Value: 1.0,
				},
				{
					Kind:  COMPARATOR,
					Value: "==",
				},
				{
					Kind:  NUMERIC,
					Value: 0.0,
				},
				{
					Kind:  TERNARY,
					Value: "?",
				},
				{
					Kind:  BOOLEAN,
					Value: true,
				},
			},
		},
		{
			Name:  "Null coalesce left",
			Input: "1 ?? 2",
			Expected: []Token{
				{
					Kind:  NUMERIC,
					Value: 1.0,
				},
				{
					Kind:  TERNARY,
					Value: "??",
				},
				{
					Kind:  NUMERIC,
					Value: 2.0,
				},
			},
		},
	}

	runTokenParsingTest(tokenParsingTests, test)
}

// Tests to make sure that the String() reprsentation of an expression exactly matches what is given to the parse function.
func TestOriginalString(test *testing.T) {

	// include all the token types, to be sure there's no shenaniganery going on.
	expressionString := "2 > 1 &&" +
		"'something' != 'nothing' || " +
		"'2014-01-20' < 'Wed Jul  8 23:07:35 MDT 2015' && " +
		"[escapedVariable name with spaces] <= unescaped\\-variableName &&" +
		"modifierTest + 1000 / 2 > (80 * 100 % 2) && true ? true : false"

	expression, err := New(expressionString)
	if err != nil {

		test.Logf("failed to parse original string test: %v", err)
		test.Fail()
		return
	}

	if expression.String() != expressionString {
		test.Logf("String() did not give the same expression as given to parse")
		test.Fail()
	}
}

// Tests to make sure that the Vars() reprsentation of an expression identifies all variables contained within the expression.
func TestOriginalVars(test *testing.T) {
	// include all the token types, to be sure there's no shenaniganery going on.
	expressionString := "2 > 1 &&" +
		"'something' != 'nothing' || " +
		"'2014-01-20' < 'Wed Jul  8 23:07:35 MDT 2015' && " +
		"[escapedVariable name with spaces] <= unescaped\\-variableName &&" +
		"modifierTest + 1000 / 2 > (80 * 100 % 2) && true ? true : false"

	expectedVars := [3]string{"escapedVariable name with spaces",
		"modifierTest",
		"unescaped-variableName"}

	expression, err := New(expressionString)
	if err != nil {

		test.Logf("failed to parse original var test: %v", err)
		test.Fail()
		return
	}

	if len(expression.Vars()) == len(expectedVars) {
		variableMap := make(map[string]string)
		for _, v := range expression.Vars() {
			variableMap[v] = v
		}
		for _, v := range expectedVars {
			if _, ok := variableMap[v]; !ok {
				test.Logf("Vars() did not correctly identify all variables contained within the expression")
				test.Fail()
			}
		}
	} else {
		test.Logf("Vars() did not correctly identify all variables contained within the expression")
		test.Fail()
	}
}

func combineWhitespaceExpressions(testCases []TokenParsingTest) []TokenParsingTest {
	var currentCase, strippedCase TokenParsingTest
	caseLength := len(testCases)
	for i := 0; i < caseLength; i++ {
		currentCase = testCases[i]

		strippedCase = TokenParsingTest{
			Name:      (currentCase.Name + " (without whitespace)"),
			Input:     stripUnquotedWhitespace(currentCase.Input),
			Expected:  currentCase.Expected,
			Functions: currentCase.Functions,
		}

		testCases = append(testCases, strippedCase, currentCase)
	}

	return testCases
}

func stripUnquotedWhitespace(expression string) string {
	var expressionBuffer bytes.Buffer
	var quoted bool

	for _, character := range expression {
		if !quoted && unicode.IsSpace(character) {
			continue
		}
		if character == '\'' {
			quoted = !quoted
		}
		expressionBuffer.WriteString(string(character))
	}
	return expressionBuffer.String()
}

func runTokenParsingTest(tokenParsingTests []TokenParsingTest, test *testing.T) {
	var parsingTest TokenParsingTest
	var expression *Expression
	var actualTokens []Token
	var actualToken Token
	var expectedTokenKindString, actualTokenKindString string
	var expectedTokenLength, actualTokenLength int
	var err error

	fmt.Printf("Running %d parsing test cases...\n", len(tokenParsingTests))
	// defer func() {
	//     if r := recover(); r != nil {
	//         test.Logf("Panic in test '%s': %v", parsingTest.Name, r)
	// 		test.Fail()
	//     }
	// }()

	// Run the test cases.
	for _, parsingTest = range tokenParsingTests {

		if parsingTest.Functions != nil {
			expression, err = NewWithFunctions(parsingTest.Input, parsingTest.Functions)
		} else {
			expression, err = New(parsingTest.Input)
		}

		if err != nil {

			test.Logf("Test '%s' failed to parse: %s", parsingTest.Name, err)
			test.Logf("Expression: '%s'", parsingTest.Input)
			test.Fail()
			continue
		}

		actualTokens = expression.Tokens()

		expectedTokenLength = len(parsingTest.Expected)
		actualTokenLength = len(actualTokens)

		if actualTokenLength != expectedTokenLength {
			test.Logf("Test '%s' failed:", parsingTest.Name)
			test.Logf("Expected %d tokens, actually found %d", expectedTokenLength, actualTokenLength)
			test.Fail()
			continue
		}

		for i, expectedToken := range parsingTest.Expected {
			actualToken = actualTokens[i]
			if actualToken.Kind != expectedToken.Kind {
				actualTokenKindString = actualToken.Kind.String()
				expectedTokenKindString = expectedToken.Kind.String()

				test.Logf("Test '%s' failed:", parsingTest.Name)
				test.Logf("Expected token kind '%v' does not match '%v'", expectedTokenKindString, actualTokenKindString)
				test.Fail()
				continue
			}

			if expectedToken.Value == nil {
				continue
			}

			reflectedKind := reflect.TypeOf(expectedToken.Value).Kind()
			if reflectedKind == reflect.Func {
				continue
			}

			// gotta be an accessor
			if reflectedKind == reflect.Slice {
				if actualToken.Value == nil {
					test.Logf("Test '%s' failed:", parsingTest.Name)
					test.Logf("Expected token value '%v' does not match nil", expectedToken.Value)
					test.Fail()
				}

				for z, actual := range actualToken.Value.([]string) {
					if actual != expectedToken.Value.([]string)[z] {

						test.Logf("Test '%s' failed:", parsingTest.Name)
						test.Logf("Expected token value '%v' does not match '%v'", expectedToken.Value, actualToken.Value)
						test.Fail()
					}
				}
				continue
			}

			if actualToken.Value != expectedToken.Value {
				test.Logf("Test '%s' failed:", parsingTest.Name)
				test.Logf("Expected token value '%v' does not match '%v'", expectedToken.Value, actualToken.Value)
				test.Fail()
				continue
			}
		}
	}
}

func noop(arguments ...interface{}) (interface{}, error) {
	return nil, nil
}
