# timeHour

## Kind

helper

## Category

time

## Signatures

```text
timeHour(time)
timeHour(time, timezone)
```

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| time | yes | no | time | time, value |
| timezone | no | no | helper:tz | tz |

## Description

Returns the hour within the day specified by the time.

## Examples

### Basic

```js
MAPVALUE(0, timeHour(time('now')))
CSV()
```

## Related

time, tz
