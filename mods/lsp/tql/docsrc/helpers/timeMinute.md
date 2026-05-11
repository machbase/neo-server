# timeMinute

## Kind

helper

## Category

time

## Signatures

```text
timeMinute(time)
timeMinute(time, timezone)
```

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| time | yes | no | time | time, value |
| timezone | no | no | helper:tz | tz |

## Description

Returns the minute offset within the hour specified by the time.

## Examples

### Basic

```js
MAPVALUE(0, timeMinute(time('now')))
CSV()
```

## Related

time, tz
