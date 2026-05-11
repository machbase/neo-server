# NDJSON

## Kind

statement sink

## Category

json encoder

## Signatures

```text
NDJSON(options...)
```

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| options | no | yes | helper | tz, sqlTimeformat, ansiTimeformat, cache |

## Description

`NDJSON()` generates newline-delimited JSON, where each output line is a complete JSON object. It is useful for large or streaming datasets and terminates output with two consecutive newlines.

## Examples

### NDJSON query output

```js
SQL(`select * from example where name = 'neo_load1' limit 3`)
NDJSON(sqlTimeformat('DEFAULT'), tz('local'), rownum(true))
```

## Related

JSON, CSV, tz, sqlTimeformat, ansiTimeformat, cache
