# timeWeekDay

## Kind

helper

## Category

time

## Signatures

```text
timeWeekDay(time)
timeWeekDay(time, timezone)
```

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| time | yes | no | time | time, value |
| timezone | no | no | helper:tz | tz |

## Description

Returns the day of the week specified by the time, with Sunday as 0.

## Examples

### Basic

```js
MAPVALUE(0, timeWeekDay(time('now')))
CSV()
```

## Related

time, tz
