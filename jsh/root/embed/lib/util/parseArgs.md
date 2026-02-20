# parseArgs

A Node.js-compatible command-line argument parser for JSH runtime, compatible with Node.js `util.parseArgs()` API.

## Installation

```javascript
const { parseArgs } = require('/lib/util');
```

## Usage

### Basic Example

```javascript
const { parseArgs } = require('/lib/util');

const args = ['-f', '--bar', 'value', 'positional'];
const result = parseArgs(args, {
    options: {
        foo: { type: 'boolean', short: 'f' },
        bar: { type: 'string' }
    },
    allowPositionals: true
});

console.log(result.values);      // { foo: true, bar: 'value' }
console.log(result.positionals); // ['positional']
```

### Sub-command Example

```javascript
const { parseArgs } = require('/lib/util');

// Define configs for different sub-commands
const commitConfig = {
    command: 'commit',
    options: {
        message: { type: 'string', short: 'm' },
        all: { type: 'boolean', short: 'a' }
    }
};

const pushConfig = {
    command: 'push',
    options: {
        force: { type: 'boolean', short: 'f' }
    },
    positionals: ['remote'],
    allowPositionals: true
};

// Parse 'commit' sub-command
const result = parseArgs(['commit', '-am', 'Fix bug'], commitConfig, pushConfig);
console.log(result.command);  // 'commit'
console.log(result.values);   // { all: true, message: 'Fix bug' }
```

## API

### Function Signature

```javascript
parseArgs(args, ...configs)
```

- `args` (Array, required): Array of strings to parse. Must be an array.
- `...configs` (Object, optional): One or more configuration objects. When multiple configs are provided with `command` properties, parseArgs will select the appropriate config based on the first element of `args`.

### Configuration Object

| Property | Type | Default | Description |
|----------|------|---------|-------------|
| `command` | string | `undefined` | Sub-command name to match against `args[0]`. Used for sub-command routing. |
| `options` | Object | `{}` | Option definitions (see below) |
| `strict` | boolean | `true` | Throw error on unknown options |
| `allowPositionals` | boolean | `!strict` | Allow positional arguments |
| `allowNegative` | boolean | `false` | Allow `--no-` prefix for boolean options |
| `tokens` | boolean | `false` | Return detailed parsing tokens |
| `positionals` | Array | `undefined` | Named positional arguments definition (see below) |

### Option Definition

Each option in the `options` object can have:

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| `type` | string | Yes | One of: `'boolean'`, `'string'`, `'integer'`, or `'float'` |
| `short` | string | No | Single character short option (e.g., `'f'` for `-f`) |
| `multiple` | boolean | No | Allow option to be specified multiple times (collects values in array) |
| `default` | any | No | Default value if option is not provided |

**Supported Types:**
- `'boolean'`: True/false flag, does not take a value
- `'string'`: Accepts any string value
- `'integer'`: Parses value as integer, validates that no decimal point is present
- `'float'`: Parses value as floating-point number, allows decimal values

**Note on Naming Conventions:**
- **Option names** automatically convert from camelCase to kebab-case for CLI flags. For example, `userName` becomes `--user-name`.
- **Positional argument names** automatically convert from kebab-case to camelCase for `namedPositionals` keys. For example, `tql-name` becomes `tqlName`.

This allows you to use JavaScript naming conventions (camelCase) in code while following traditional Linux CLI conventions (kebab-case) on the command line.

### Positional Definition

The `positionals` array allows you to assign names to positional arguments. Each element can be:

**String format** (simple):
```javascript
positionals: ['inputFile', 'outputFile']
```

**Object format** (advanced):

| Property | Type | Default | Description |
|----------|------|---------|-------------|
| `name` | string | Required | Name for this positional argument |
| `optional` | boolean | `false` | Whether this argument is optional |
| `default` | any | `undefined` | Default value if not provided (requires `optional: true`) |
| `variadic` | boolean | `false` | Collect all remaining arguments (must be last) |

**Important Rules:**
- Variadic positionals must be the last in the array
- Required positionals cannot come after optional ones
- Missing required positionals will throw an error

### Return Value

Returns an object with:

- `command` (string, optional): The matched sub-command name (only present when sub-command routing is used)
- `values` (Object): Parsed option values
- `positionals` (Array): Positional arguments (always present)
- `namedPositionals` (Object, optional): Named positional arguments (only if `positionals` config provided)
- `tokens` (Array, optional): Detailed parsing information (only if `tokens: true`)

## Examples

### CamelCase to Kebab-Case Conversion

Option names are automatically converted from camelCase to kebab-case for CLI flags:

```javascript
const result = parseArgs(['--user-name', 'Alice', '--max-retry-count', '5', '--enable-debug'], {
    options: {
        userName: { type: 'string' },           // Becomes --user-name
        maxRetryCount: { type: 'string' },      // Becomes --max-retry-count
        enableDebug: { type: 'boolean' }        // Becomes --enable-debug
    }
});
// result.values: { userName: 'Alice', maxRetryCount: '5', enableDebug: true }
```

This allows you to use JavaScript naming conventions (camelCase) in your code while following traditional Linux CLI conventions (kebab-case) on the command line. Simple names without capital letters (like `port`, `verbose`) are not converted.

### Long Options

```javascript
const result = parseArgs(['--verbose', '--output', 'file.txt'], {
    options: {
        verbose: { type: 'boolean' },
        output: { type: 'string' }
    }
});
// result.values: { verbose: true, output: 'file.txt' }
```

### Short Options

```javascript
const result = parseArgs(['-v', '-o', 'out.txt'], {
    options: {
        verbose: { type: 'boolean', short: 'v' },
        output: { type: 'string', short: 'o' }
    }
});
// result.values: { verbose: true, output: 'out.txt' }
```

### Integer and Float Options

Use `integer` type for whole numbers and `float` type for decimal numbers:

```javascript
const result = parseArgs(['--port', '8080', '--ratio', '0.75', '-c', '10'], {
    options: {
        port: { type: 'integer' },           // Parses as integer
        ratio: { type: 'float' },            // Parses as float
        count: { type: 'integer', short: 'c' }
    }
});
// result.values: { port: 8080, ratio: 0.75, count: 10 }
// All numeric values are JavaScript numbers (typeof === 'number')
```

**Integer validation:**
```javascript
// This will throw an error because 3.14 contains a decimal point
parseArgs(['--count', '3.14'], {
    options: { count: { type: 'integer' } }
});
// TypeError: Option --count requires an integer value, got: 3.14

// This will throw an error because 'abc' is not a valid number
parseArgs(['--port', 'abc'], {
    options: { port: { type: 'integer' } }
});
// TypeError: Option --port requires a valid integer value, got: abc
```

### Inline Values

Supports both `--option=value` and `-o=value` formats:

```javascript
const result = parseArgs(['--output=file.txt', '-o=out.txt'], {
    options: {
        output: { type: 'string', short: 'o' }
    }
});
// result.values: { output: 'out.txt' } // Last value wins
```

### Multiple Values

Collect multiple values for the same option:

```javascript
const result = parseArgs(['--include', 'a.js', '--include', 'b.js', '-I', 'c.js'], {
    options: {
        include: { type: 'string', short: 'I', multiple: true }
    }
});
// result.values: { include: ['a.js', 'b.js', 'c.js'] }
```

### Default Values

```javascript
const result = parseArgs(['--foo'], {
    options: {
        foo: { type: 'boolean' },
        bar: { type: 'string', default: 'default_value' },
        count: { type: 'string', default: '0' }
    }
});
// result.values: { foo: true, bar: 'default_value', count: '0' }
```

### Short Option Groups

Bundle multiple boolean short options together:

```javascript
const result = parseArgs(['-abc'], {
    options: {
        a: { type: 'boolean', short: 'a' },
        b: { type: 'boolean', short: 'b' },
        c: { type: 'boolean', short: 'c' }
    }
});
// result.values: { a: true, b: true, c: true }
```

### Option Terminator

Use `--` to separate options from positional arguments:

```javascript
const result = parseArgs(['--foo', '--', '--bar', 'baz'], {
    options: {
        foo: { type: 'boolean' },
        bar: { type: 'boolean' }
    },
    allowPositionals: true
});
// result.values: { foo: true }
// result.positionals: ['--bar', 'baz']
```

### Negative Options

Enable `--no-` prefix to set boolean options to false. Works with camelCase option names:

```javascript
const result = parseArgs(['--no-color', '--verbose', '--no-enable-debug'], {
    options: {
        color: { type: 'boolean' },
        verbose: { type: 'boolean' },
        enableDebug: { type: 'boolean' }      // --no-enable-debug sets to false
    },
    allowNegative: true
});
// result.values: { color: false, verbose: true, enableDebug: false }
```

### Named Positionals

Assign names to positional arguments for easier access. Positional argument names are automatically converted from kebab-case to camelCase:

```javascript
const result = parseArgs(['my-tql-file'], {
    options: {},
    allowPositionals: true,
    positionals: ['tql-name']  // kebab-case name
});
// result.positionals: ['my-tql-file']
// result.namedPositionals: { tqlName: 'my-tql-file' }  // Converted to camelCase
```

Simple example with multiple positionals:

```javascript
const result = parseArgs(['input.txt', 'output.txt'], {
    options: {},
    allowPositionals: true,
    positionals: ['inputFile', 'outputFile']
});
// result.positionals: ['input.txt', 'output.txt']
// result.namedPositionals: { inputFile: 'input.txt', outputFile: 'output.txt' }
```

Kebab-case positional names example:

```javascript
const result = parseArgs(['input.txt', 'output.txt'], {
    options: {},
    allowPositionals: true,
    positionals: ['input-file', 'output-file']  // kebab-case names
});
// result.positionals: ['input.txt', 'output.txt']
// result.namedPositionals: { 
//     inputFile: 'input.txt',   // Converted to camelCase
//     outputFile: 'output.txt'  // Converted to camelCase
// }
```

### Optional Positionals

Make positional arguments optional with default values. Names are automatically converted to camelCase:

```javascript
const result = parseArgs(['input.txt'], {
    options: {},
    allowPositionals: true,
    positionals: [
        'input-file',  // kebab-case name
        { name: 'output-file', optional: true, default: 'stdout' }
    ]
});
// result.positionals: ['input.txt']
// result.namedPositionals: { 
//     inputFile: 'input.txt',   // Converted to camelCase
//     outputFile: 'stdout'      // Converted to camelCase
// }
```

### Variadic Positionals

Collect remaining arguments into an array. Names are automatically converted to camelCase:

```javascript
const result = parseArgs(['input.txt', 'output.txt', 'file1.js', 'file2.js', 'file3.js'], {
    options: {},
    allowPositionals: true,
    positionals: [
        'input-file',
        'output-file',
        { name: 'source-files', variadic: true }  // kebab-case name
    ]
});
// result.positionals: ['input.txt', 'output.txt', 'file1.js', 'file2.js', 'file3.js']
// result.namedPositionals: {
//     inputFile: 'input.txt',     // Converted to camelCase
//     outputFile: 'output.txt',   // Converted to camelCase
//     sourceFiles: ['file1.js', 'file2.js', 'file3.js']  // Converted to camelCase
// }
```

### Named Positionals with Options

Combine options and named positionals. Positional names are automatically converted to camelCase:

```javascript
const result = parseArgs(['-v', '--config', 'app.json', 'src.js', 'dest.js'], {
    options: {
        verbose: { type: 'boolean', short: 'v' },
        config: { type: 'string' }
    },
    allowPositionals: true,
    positionals: ['source-file', 'destination-file']  // kebab-case names
});
// result.values: { verbose: true, config: 'app.json' }
// result.positionals: ['src.js', 'dest.js']
// result.namedPositionals: { 
//     sourceFile: 'src.js',       // Converted to camelCase
//     destinationFile: 'dest.js'  // Converted to camelCase
// }
```

### Sub-command Routing

Handle different sub-commands with different options and positionals by providing multiple configs with `command` properties:

```javascript
// Define configs for different sub-commands
const addConfig = {
    command: 'add',
    options: {
        force: { type: 'boolean', short: 'f' },
        message: { type: 'string', short: 'm' }
    },
    positionals: ['file'],
    allowPositionals: true
};

const removeConfig = {
    command: 'remove',
    options: {
        recursive: { type: 'boolean', short: 'r' },
        verbose: { type: 'boolean', short: 'v' }
    },
    positionals: ['file'],
    allowPositionals: true
};

// Parse with sub-command
const result1 = parseArgs(['add', '-f', '-m', 'Initial commit', 'file.txt'], addConfig, removeConfig);
// result1.command: 'add'
// result1.values: { force: true, message: 'Initial commit' }
// result1.namedPositionals: { file: 'file.txt' }

const result2 = parseArgs(['remove', '-rv', 'dir/'], addConfig, removeConfig);
// result2.command: 'remove'
// result2.values: { recursive: true, verbose: true }
// result2.namedPositionals: { file: 'dir/' }
```

**Git-like example:**

```javascript
const commitConfig = {
    command: 'commit',
    options: {
        message: { type: 'string', short: 'm' },
        all: { type: 'boolean', short: 'a' },
        amend: { type: 'boolean' }
    }
};

const pushConfig = {
    command: 'push',
    options: {
        force: { type: 'boolean', short: 'f' },
        tags: { type: 'boolean' }
    },
    positionals: ['remote', { name: 'branch', optional: true }],
    allowPositionals: true
};

const result = parseArgs(['commit', '-am', 'Fix bug'], commitConfig, pushConfig);
// result.command: 'commit'
// result.values: { all: true, message: 'Fix bug' }

const result2 = parseArgs(['push', '-f', 'origin', 'main'], commitConfig, pushConfig);
// result2.command: 'push'
// result2.values: { force: true }
// result2.namedPositionals: { remote: 'origin', branch: 'main' }
```

**How sub-command routing works:**

1. When multiple configs are provided, parseArgs checks `args[0]` against each config's `command` property
2. If a match is found, that config is used and `args[0]` is removed from processing
3. The matched command name is returned in `result.command`
4. If no command matches, the first config without a `command` property is used (fallback/default config)
5. Each sub-command can have its own options, positionals, and other settings

This allows building CLI tools with git-like sub-command structures where each sub-command has different flags and arguments.

### Tokens Mode

Get detailed parsing information:

```javascript
const result = parseArgs(['-f', '--bar', 'value'], {
    options: {
        foo: { type: 'boolean', short: 'f' },
        bar: { type: 'string' }
    },
    tokens: true
});

// result.tokens: [
//   { kind: 'option', name: 'foo', rawName: '-f', index: 0, value: undefined, inlineValue: undefined },
//   { kind: 'option', name: 'bar', rawName: '--bar', index: 1, value: 'value', inlineValue: false }
// ]
```

### Token Structure

Each token has:

- **All tokens:**
  - `kind`: `'option'`, `'positional'`, or `'option-terminator'`
  - `index`: Position in the args array

- **Option tokens:**
  - `name`: Long option name
  - `rawName`: How the option was specified (e.g., `-f`, `--foo`)
  - `value`: Option value (undefined for boolean options)
  - `inlineValue`: Whether value was specified inline (e.g., `--foo=bar`)

- **Positional tokens:**
  - `value`: The positional argument value

## Error Handling

The parser throws `TypeError` in the following cases:

- Unknown option when `strict: true` (default)
- Missing value for string, integer, or float option
- Invalid number format for integer or float option
- Decimal value for integer option (e.g., `3.14` when integer is expected)
- Unexpected positional argument when `allowPositionals: false` and `strict: true`
- Using `--no-` prefix on non-boolean option when `strict: true`
- Boolean option with inline value when `strict: true`
- Missing required positional argument (when using named positionals)
- Variadic positional not in last position (when using named positionals)

### Example

```javascript
try {
    const result = parseArgs(['--unknown'], {
        options: {},
        strict: true
    });
} catch (error) {
    console.error(error.message); // "Unknown option: --unknown"
}
```

```javascript
// Missing required positional
try {
    parseArgs(['input.txt'], {
        positionals: ['inputFile', 'outputFile']  // outputFile required
    });
} catch (error) {
    console.error(error.message); // "Missing required positional argument: outputFile"
}
```

```javascript
// Variadic not last
try {
    parseArgs([], {
        positionals: [
            { name: 'files', variadic: true },
            'output'  // Error: cannot come after variadic
        ]
    });
} catch (error) {
    console.error(error.message); // "Variadic positional argument must be the last argument"
}
```

## Generating Help Messages

The `formatHelp()` function generates formatted help text from configuration objects. It supports both single-command and multi-command (sub-command) configurations.

### Basic Usage

```javascript
const help = parseArgs.formatHelp({
    usage: 'Usage: myapp [options] <file>',
    options: {
        verbose: { type: 'boolean', short: 'v', description: 'Enable verbose output' },
        output: { type: 'string', short: 'o', description: 'Output file', default: 'out.txt' }
    },
    positionals: [
        { name: 'file', description: 'Input file to process' }
    ]
});

console.log(help);
// Output:
// Usage: myapp [options] <file>
//
// Positional arguments:
//   file - Input file to process
//
// Options:
//   -v, --verbose      Enable verbose output
//   -o, --output       Output file  (default: out.txt)
```

### Multi-Command Help

When you provide multiple configurations with `command` properties, `formatHelp()` generates comprehensive help showing all available commands and their individual options:

```javascript
const help = parseArgs.formatHelp(
    // Global/default config
    {
        usage: 'Usage: git <command> [options]',
        options: {
            version: { type: 'boolean', description: 'Show version' },
            help: { type: 'boolean', short: 'h', description: 'Show help' }
        }
    },
    // Sub-command configs
    {
        command: 'commit',
        description: 'Record changes to the repository',
        options: {
            message: { type: 'string', short: 'm', description: 'Commit message' },
            all: { type: 'boolean', short: 'a', description: 'Stage all changes' },
            amend: { type: 'boolean', description: 'Amend previous commit' }
        }
    },
    {
        command: 'push',
        description: 'Update remote refs along with associated objects',
        options: {
            force: { type: 'boolean', short: 'f', description: 'Force push' },
            tags: { type: 'boolean', description: 'Push tags' }
        },
        positionals: [
            { name: 'remote', description: 'Remote name' },
            { name: 'branch', optional: true, description: 'Branch name' }
        ]
    }
);

console.log(help);
// Output:
// Usage: git <command> [options]
//
// Commands:
//   commit  Record changes to the repository
//   push    Update remote refs along with associated objects
//
// Global options:
//   --version    Show version
//   -h, --help   Show help
//
// Command: commit
//   Record changes to the repository
//
//   Options:
//     -m, --message  Commit message
//     -a, --all      Stage all changes
//     --amend        Amend previous commit
//
// Command: push
//   Update remote refs along with associated objects
//
//   Positional arguments:
//     remote - Remote name
//     branch (optional) - Branch name
//
//   Options:
//     -f, --force  Force push
//     --tags       Push tags
```

**formatHelp() features:**
- Automatically formats camelCase option names to kebab-case
- Aligns option descriptions for better readability
- Shows default values when specified
- Marks optional positionals and variadic arguments
- Separates global options from command-specific options
- Lists all available commands with descriptions

## Compatibility

This implementation is compatible with Node.js `util.parseArgs()` API, supporting:

- ✅ Long options (`--option`)
- ✅ Short options (`-o`)
- ✅ Short option groups (`-abc`)
- ✅ Inline values (`--option=value`, `-o=value`)
- ✅ Boolean and string types
- ✅ Multiple values
- ✅ Default values
- ✅ Positional arguments
- ✅ Option terminator (`--`)
- ✅ Negative options (`--no-option`)
- ✅ Strict mode
- ✅ Tokens mode

## Extended Features

Beyond Node.js `util.parseArgs()`, this implementation adds:

- ✅ **Integer and float types** - Numeric types with automatic parsing and validation
- ✅ **Named positionals** - Assign names to positional arguments (kebab-case names automatically converted to camelCase)
- ✅ **Optional positionals** - Make positional arguments optional with defaults
- ✅ **Variadic positionals** - Collect remaining arguments into an array
- ✅ **Positional validation** - Automatic validation of required arguments
- ✅ **CamelCase to kebab-case conversion** - Automatically converts option names from camelCase to kebab-case for CLI flags
- ✅ **Kebab-case to camelCase conversion** - Automatically converts positional argument names from kebab-case to camelCase for `namedPositionals` keys
- ✅ **Sub-command routing** - Support for git-like sub-commands with different options per command

## License

See project license.
