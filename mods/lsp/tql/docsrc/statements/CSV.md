# CSV

## Kind

statement source_or_sink

## Category

csv source or encoder

## Signatures

```text
CSV(input, options...)
CSV(options...)
```

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| input | no | no | stream|string|helper:file|helper:payload | file, payload |
| options | no | yes | helper | field, charset, logProgress, nullValue, cache, tz, sqlTimeformat, ansiTimeformat |

## Description

As a SRC function, `CSV()` reads CSV data from `file()`, `payload()`, or inline content and yields records. As a SINK function, `CSV()` encodes incoming records as CSV lines. Sink output is terminated by two consecutive newlines.

## Examples

### Read CSV payload

```js
CSV(payload(),
    field(0, stringType(), 'name'),
    field(1, timeType('s'), 'time'),
    field(2, floatType(), 'value'),
    header(false)
)
APPEND(table('example'))
```

### Write CSV

```js
FAKE(arrange(1, 3, 1))
MAPVALUE(1, value(0) * 10, 'x10')
CSV()
```

## Related

file, payload, field, charset, stringType, datetimeType, timeType, doubleType, floatType, boolType, nullValue, cache, tz
