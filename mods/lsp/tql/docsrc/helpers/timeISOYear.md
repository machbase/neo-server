# timeISOYear

## Kind

helper

## Category

time

## Signatures

```text
timeISOYear(time)
timeISOYear(time, timezone)
```

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| time | yes | no | time | time, value |
| timezone | no | no | helper:tz | tz |

## Description

Returns the ISO 8601 year number in which the time occurs.

## Examples

### Basic

```js
MAPVALUE(0, timeISOYear(time('now')))
CSV()
```

## Related

time, tz
