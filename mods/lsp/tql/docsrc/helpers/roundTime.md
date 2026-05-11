# roundTime

## Kind

helper

## Category

time

## Signatures

```text
roundTime(time, duration)
```

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| time | yes | no | time | time('now'), value |
| duration | yes | no | literal:string | '1h', '1s' |

## Description

`roundTime()` returns a time rounded to the given duration.

## Examples

### Basic

```js
MAPVALUE(0, roundTime(time('now'), '1s'))
CSV()
```

## Related

time, timeAdd
