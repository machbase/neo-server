# timeAdd

## Kind

helper

## Category

time

## Signatures

```text
timeAdd(value)
timeAdd(value, expression)
```

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| value | yes | no | time|string|number | time, 'now' |
| expression | no | no | duration|string|number | '-10s50ms', '1m' |

## Description

`timeAdd()` returns a time adjusted by a duration expression. It can accept `now`, epoch values, or existing time values.

## Examples

### Basic

```js
SQL(`select to_char(time), value from example where time < ?`, timeAdd('now', '-10s'))
CSV()
```

## Related

time, roundTime, parseTime
