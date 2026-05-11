# timeDay

## Kind

helper

## Category

time

## Signatures

```text
timeDay(time)
timeDay(time, timezone)
```

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| time | yes | no | time | time, value |
| timezone | no | no | helper:tz | tz |

## Description

Returns the day of the month specified by the time.

## Examples

### Basic

```js
MAPVALUE(0, timeDay(time('now')))
CSV()
```

## Related

time, tz
