# APPEND

## Kind

statement sink

## Category

database sink

## Signatures

```text
APPEND(table)
```

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| table | yes | no | helper:table | table |

## Description

`APPEND()` stores incoming records into a database table through the machbase-neo append method. It is generally more suitable than `INSERT()` for larger record batches.

## Examples

### Append records

```js
FAKE(json({
    ['temperature', 1708582794, 12.34],
    ['temperature', 1708582795, 13.45]
}))
MAPVALUE(1, value(1) * 1000000000)
APPEND(table('example'))
```

## Related

table, INSERT, CSV
