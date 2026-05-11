# timeSecond

## Kind

helper

## Category

time

## Signatures

```text
timeSecond(time)
timeSecond(time, timezone)
```

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| time | yes | no | time | time, value |
| timezone | no | no | helper:tz | tz |

## Description

Returns the second offset within the minute specified by the time.

## Examples

### Basic

```js
MAPVALUE(0, timeSecond(time('now')))
CSV()
```

## Related

time, tz
