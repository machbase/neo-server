# splitFields

A function that splits a string by whitespace characters, while treating quoted substrings as single fields.

## Usage

```javascript
const { splitFields } = require('/lib/util');

// Basic usage
const result = splitFields('foo bar baz');
// ['foo', 'bar', 'baz']

// Double quote handling
const result2 = splitFields('hello "world foo" bar');
// ['hello', 'world foo', 'bar']

// Single quote handling
const result3 = splitFields("hello 'world foo' bar");
// ['hello', 'world foo', 'bar']

// Mixed quotes
const result4 = splitFields('a "b c" d \'e f\' g');
// ['a', 'b c', 'd', 'e f', 'g']
```

## Function Signature

```javascript
splitFields(str, options)
```

### Parameters

- `str` (string): The input string to split
- `options` (object, optional): Additional options (currently unused)

### Return Value

- `string[]`: Array of split fields

## Features

- **Whitespace Splitting**: Splits the string by space (` `), tab (`\t`), newline (`\n`), and carriage return (`\r`) characters
- **Quote Handling**: 
  - Strings enclosed in double quotes (`"`) are treated as a single field even if they contain whitespace
  - Strings enclosed in single quotes (`'`) are treated the same way
- **Empty Field Removal**: Consecutive whitespace characters are ignored, and empty fields are not included in the result

## Examples

### Basic Whitespace Splitting

```javascript
splitFields('  foo   bar baz  ');
// ['foo', 'bar', 'baz']
```

### Tab and Newline Handling

```javascript
splitFields('foo\tbar\nbaz');
// ['foo', 'bar', 'baz']
```

### Preserving Whitespace Within Quotes

```javascript
splitFields('cmd "arg 1" "arg 2" "arg 3"');
// ['cmd', 'arg 1', 'arg 2', 'arg 3']
```

### Empty String and Whitespace-Only Cases

```javascript
splitFields('');
// []

splitFields('   \t  \n  ');
// []
```

## Notes

- If a quote is not closed, the rest of the string up to the end is treated as a single field
- Quote characters themselves are not included in the result strings
- Escape sequences are not processed (e.g., `\"`, `\''`)
