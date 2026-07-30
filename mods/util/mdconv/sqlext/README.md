# sqlext

`sqlext` is a markdown extension for `sql` fenced code blocks that can execute the block and render the result alongside the source.

## Overview

By default, a `sql` fence is rendered as a normal syntax-highlighted code block.
When `execute=true` is specified, the block is executed through the shared SQL execution path and the output includes both the source preview and the execution result.

## Options

The extension supports the following options:

- `execute` (`true|false`, default: `false`)
  - `true`: execute the block
  - `false`: render as a normal code fence
- `source` (`hide|all` or a line selection string/array-like value)
  - `hide`: default for `execute=true`; hide the source preview
  - `all`: show the full source
  - `"2-3"` or `["2-3"]`: show only the selected lines
- `format` (`table|json|csv|none`, default: `table`)
  - `table`: render a human-readable table result
  - `json`: render a JSON object with result metadata
  - `csv`: render CSV output
  - `none`: hide the result area
- `timeformat` (string)
  - controls how time values are formatted
- `tz` (string)
  - controls the time zone used for formatting
- `binaryformat` (`base64|hex|bytes|preview`, default: `hex`)
  - controls how binary values are formatted
- `preview` (`int` or `all`, default: `10`)
  - controls how many rows are shown when a result set is rendered in preview form
  - when a query returns more rows than the preview size, the renderer shows the first and last rows with an ellipsis in the middle
  - use `all` to disable the compact preview behavior and show the full result
- `params` / `p` (array-like value)
  - supplies bind parameters for the SQL statement
  - example: `params=[1,"neo"]` or `p=[1,"neo"]`
- `header` (`skip|<other>`, default: empty)
  - uses the same semantics as the server-side SQL request handling for header rendering
- `delimiter` (string, default: `,`)
  - custom delimiter for CSV-style output; applied only when `format=csv`
- `timeout` (Go duration string)
  - examples: `1s`, `100ms`, `1000us`
  - apply a timeout to execution

## Examples

### 1. Default behavior

~~~markdown
```sql
select 1 as v;
```
~~~

This renders as a normal syntax-highlighted code fence.

### 2. Execute and show the result

~~~markdown
```sql {execute=true}
select 1 as v;
```
~~~

The fence is executed and rendered with the result block. The source preview is hidden by default for `execute=true`; specify `source=all` or a line selection to show it.

### 3. Format the output

~~~markdown
```sql {execute=true,format=json,timeformat=2006-01-02 15:04:05,tz=Asia/Seoul,binaryformat=hex}
select now() as ts, x'0102' as payload;
```
~~~

The result is rendered using the requested format and formatting options.

### 4. Preview large result sets

~~~markdown
```sql {execute=true,format=table,preview=6}
select * from generate_series(1, 20) as t(v);
```
~~~

When the result contains more rows than the preview size, the output shows the first and last rows with an ellipsis in between. To show the full result instead, set `preview=all`.

### 5. Show source preview lines

~~~markdown
```sql {execute=true,source=["2-3"]}
select 1 as a;
select 2 as b;
select 3 as c;
```
~~~

Only the requested source lines are shown in the preview block.

### 6. Use bind parameters

~~~markdown
```sql {execute=true,format=csv,p=[1,"neo"]}
select ?, ?;
```
~~~

The fenced block can pass bind parameters through the `p` option.

### 7. Render as CSV with custom header behavior

~~~markdown
```sql {execute=true,format=csv,header=skip,delimiter=|}
select 1 as a, 2 as b;
```
~~~

This example shows how to switch to CSV output and control the rendered header behavior.

## Notes

- `execute=false` remains the safe default.
- The implementation uses the shared SQL request path, so Markdown SQL execution follows the same request semantics as the server-side SQL handling flow.
- `timeout` is supported as the initial execution limit policy.
