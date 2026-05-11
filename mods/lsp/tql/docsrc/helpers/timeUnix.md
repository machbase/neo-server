# timeUnix

## Kind

helper

## Category

time

## Signatures

```text
timeUnix(time)
```

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| time | yes | no | time | time, value |

## Description

`timeUnix()` returns time as Unix time in seconds elapsed since January 1, 1970 UTC.

## Examples

### Basic

```js
MAPVALUE(0, timeUnix(time('now')))
CSV()
```

## Related

time
