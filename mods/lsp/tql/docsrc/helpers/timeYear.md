# timeYear

## Kind

helper

## Category

time

## Signatures

```text
timeYear(time)
timeYear(time, timezone)
```

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| time | yes | no | time | time, value |
| timezone | no | no | helper:tz | tz |

## Description

Returns the year in which the time occurs.

## Examples

### Basic

```js
MAPVALUE(0, timeYear(time('now')))
CSV()
```

## Related

time, tz
