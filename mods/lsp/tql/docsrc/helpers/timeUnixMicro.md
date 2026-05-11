# timeUnixMicro

## Kind

helper

## Category

time

## Signatures

```text
timeUnixMicro(time)
```

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| time | yes | no | time | time, value |

## Description

`timeUnixMicro()` returns time as Unix time in microseconds elapsed since January 1, 1970 UTC.

## Examples

### Basic

```js
MAPVALUE(0, timeUnixMicro(time('now')))
CSV()
```

## Related

time
