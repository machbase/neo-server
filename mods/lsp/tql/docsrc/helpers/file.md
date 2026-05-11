# file

## Kind

helper

## Category

input source

## Signatures

```text
file(path)
```

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| path | yes | no | literal:string | absolute path or http url |

## Description

`file()` opens a local file or retrieves an HTTP/HTTPS URL and returns an input stream for source functions such as `CSV()`, `BYTES()`, and `STRING()`.

## Examples

### Basic

```js
CSV(file(`http://127.0.0.1:5654/db/query?format=csv&q=` + escapeParam(`select * from example limit 10`)))
CSV()
```

## Related

CSV, BYTES, STRING, escapeParam
