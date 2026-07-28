# jshext

`jshext` is a markdown extension for `jsh` fenced code blocks that can execute the block and render the result alongside the source.

## Overview

By default, a `jsh` fence is rendered as a normal syntax-highlighted code block.
When `execute=true` is specified, the block is executed through the JSH engine and the rendered output includes:

- the source preview
- the execution result
- `exitCode` metadata

## Options

The extension currently supports the following options:

- `execute` (`true|false`, default: `false`)
  - `true`: execute the block
  - `false`: render as a normal code fence
- `source` (`hide|all` or a line selection string/array-like value)
  - `hide`: default for `execute=true`; hide the source preview
  - `all`: show the full source
  - `"2-3"` or `["2-3"]`: show only the selected lines
- `result` (`default|json|none`, default: `default`)
  - `default`: render a human-readable result box
  - `json`: reserved for structured machine-readable payloads
  - `none`: hide the result area
- `timeout` (Go duration string)
  - examples: `1s`, `100ms`, `1000us`
  - apply a timeout to execution

## Examples

### Default behavior

~~~markdown
```jsh
console.log("hello");
```
~~~

This renders as a normal syntax-highlighted code fence.

### Execute and show the result

~~~markdown
```jsh {execute=true}
console.log("hello from jsh");
```
~~~

The fence is executed and rendered with the result box. The source preview is hidden by default for `execute=true`; specify `source=all` or a line selection to show it.

### Select source lines

~~~markdown
```jsh {execute=true,source=["2-3"]}
console.log("line1");
console.log("line2");
console.log("line3");
```
~~~

Only the requested lines are shown in the source preview.

## Notes

- `execute=false` remains the safe default.
- The current implementation is a first pass and focuses on rendering and basic execution integration.
- `timeout` is supported as the initial execution limit policy.
