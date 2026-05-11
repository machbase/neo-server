# SQL_SELECT

## Kind

statement source

## Category

database source

## Signatures

```text
SQL_SELECT(fields..., from, between)
SQL_SELECT(fields..., from, between, limit)
```

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| fields | yes | yes | literal:string | 'time', 'value' |
| from | yes | no | helper:from | from |
| between | yes | no | helper:between | between |
| limit | no | no | helper:limit | limit |

## Description

`SQL_SELECT()` provides the same source behavior as `SQL()`, but builds a SELECT query from standardized option helpers. It is convenient for time range queries against tag tables.

## Examples

### Select tag data

```js
SQL_SELECT(
    'time', 'value',
    from('example', 'temperature'),
    between('last-10s', 'last')
)
CSV()
```

## Related

from, between, limit, parseTime, tz
