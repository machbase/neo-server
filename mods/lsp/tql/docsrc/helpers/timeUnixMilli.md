# timeUnixMilli

## Kind

helper

## Category

time

## Signatures

```text
timeUnixMilli(time)
```

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| time | yes | no | time | time, value |

## Description

`timeUnixMilli()` returns time as Unix time in milliseconds elapsed since January 1, 1970 UTC.

## Examples

### Basic

```js
MAPVALUE(0, timeUnixMilli(time('now')))
CSV()
```

## Related

time
