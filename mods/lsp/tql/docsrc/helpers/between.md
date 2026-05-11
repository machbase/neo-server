# between

## Kind

helper

## Category

database source

## Signatures

```text
between(fromTime, toTime)
between(fromTime, toTime, period)
```

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| fromTime | yes | no | time|string|number | 'last-10s', parseTime |
| toTime | yes | no | time|string|number | 'last', time |
| period | no | no | duration|string|number | '1s', '1m' |

## Description

`between()` supplies a time range condition to `SQL_SELECT()`. If period is supplied, it can generate grouped time expressions with aggregation SQL functions.

## Examples

### Basic

```js
SQL_SELECT('time', 'value', from('example', 'temperature'), between('last-10s', 'last'))
CSV()
```

## Related

SQL_SELECT, from, limit, parseTime, tz
