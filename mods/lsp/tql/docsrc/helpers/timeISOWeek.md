# timeISOWeek

## Kind

helper

## Category

time

## Signatures

```text
timeISOWeek(time)
timeISOWeek(time, timezone)
```

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| time | yes | no | time | time, value |
| timezone | no | no | helper:tz | tz |

## Description

Returns the ISO 8601 week number in which the time occurs.

## Examples

### Basic

```js
MAPVALUE(0, timeISOWeek(time('now')))
CSV()
```

## Related

time, tz
