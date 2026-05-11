# escapeParam

## Kind

helper

## Category

string

## Signatures

```text
escapeParam(str)
```

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| str | yes | no | literal:string|expression | SQL or URL text |

## Description

`escapeParam()` escapes a string so it can be safely placed inside a URL query.

## Examples

### Basic

```js
CSV(file(`http://127.0.0.1:5654/db/query?format=csv&q=` + escapeParam(`select count(*) from example`)))
CSV()
```

## Related

file, CSV
