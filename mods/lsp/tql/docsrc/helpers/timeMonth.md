# timeMonth

## Kind

helper

## Category

time

## Signatures

```text
timeMonth(time)
timeMonth(time, timezone)
```

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| time | yes | no | time | time, value |
| timezone | no | no | helper:tz | tz |

## Description

Returns the month of the year specified by the time.

## Examples

### Basic

```js
MAPVALUE(0, timeMonth(time('now')))
CSV()
```

## Related

time, tz
