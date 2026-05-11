# timeUnixNano

## Kind

helper

## Category

time

## Signatures

```text
timeUnixNano(time)
```

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| time | yes | no | time | time, value |

## Description

`timeUnixNano()` returns time as Unix time in nanoseconds elapsed since January 1, 1970 UTC.

## Examples

### Basic

```js
MAPVALUE(0, timeUnixNano(time('now')))
CSV()
```

## Related

time
