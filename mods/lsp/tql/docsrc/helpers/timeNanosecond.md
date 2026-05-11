# timeNanosecond

## Kind

helper

## Category

time

## Signatures

```text
timeNanosecond(time)
timeNanosecond(time, timezone)
```

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| time | yes | no | time | time, value |
| timezone | no | no | helper:tz | tz |

## Description

Returns the nanosecond offset within the second specified by the time.

## Examples

### Basic

```js
MAPVALUE(0, timeNanosecond(time('now')))
CSV()
```

## Related

time, tz
