# tz

## Kind

helper

## Category

time

## Signatures

```text
tz(name)
```

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| name | yes | no | literal:string | 'local', 'UTC', 'EST', 'Europe/Paris' |

## Description

`tz()` returns the timezone matching the given name.

## Examples

### Basic

```js
CSV(sqlTimeformat('YYYY-MM-DD HH24:MI:SS.nnn'), tz('Asia/Seoul'))
```

## Related

time, parseTime, sqlTimeformat, ansiTimeformat
