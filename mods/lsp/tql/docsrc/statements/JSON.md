# JSON

## Kind

statement sink

## Category

json encoder

## Signatures

```text
JSON(options...)
```

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| options | no | yes | helper | tz, sqlTimeformat, ansiTimeformat, cache |

## Description

`JSON()` generates JSON output from incoming records. Options can control time formatting, row numbering, float precision, transposed column output, flattened rows, or array-of-object rows.

## Examples

### Default JSON

```js
FAKE(arrange(1, 3, 1))
MAPVALUE(1, value(0) * 10, 'x10')
JSON()
```

## Related

CSV, NDJSON, tz, sqlTimeformat, ansiTimeformat, cache
