# strTime

## Kind

helper

## Category

string time

## Signatures

```text
strTime(time, format)
strTime(time, format, tz)
```

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| time | yes | no | time | time, parseTime |
| format | yes | no | literal:string|helper | sqlTimeformat, ansiTimeformat |
| tz | no | no | helper:tz | tz |

## Description

`strTime()` formats a time value to a string according to the given format and timezone.

## Examples

### Basic

```js
MAPVALUE(0, strTime(time('now'), sqlTimeformat('YYYY/MM/DD HH24:MI:SS.nnn'), tz('UTC')), 'result')
MARKDOWN(rownum(true))
```

## Related

time, sqlTimeformat, ansiTimeformat, tz
