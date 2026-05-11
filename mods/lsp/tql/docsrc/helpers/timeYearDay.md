# timeYearDay

## Kind

helper

## Category

time

## Signatures

```text
timeYearDay(time)
timeYearDay(time, timezone)
```

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| time | yes | no | time | time, value |
| timezone | no | no | helper:tz | tz |

## Description

Returns the day of the year specified by the time.

## Examples

### Basic

```js
MAPVALUE(0, timeYearDay(time('now')))
CSV()
```

## Related

time, tz
