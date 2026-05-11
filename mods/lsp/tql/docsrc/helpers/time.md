# time

## Kind

helper

## Category

time

## Signatures

```text
time(number)
time(string)
```

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| value | yes | no | literal:string|number | 'now', 'now-10s', epoch ns |

## Description

`time()` returns a time value from `now` expressions, relative time strings, or epoch nanoseconds.

## Examples

### Basic

```js
SQL(`select to_char(time), value from example where time < ?`, time('now'))
CSV()
```

## Related

timeAdd, parseTime, roundTime, tz
