# parseTime

## Kind

helper

## Category

time

## Signatures

```text
parseTime(timeText, format)
parseTime(timeText, format, tz)
```

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| timeText | yes | no | literal:string | time text |
| format | yes | no | literal:string | DEFAULT, RFC3339 |
| tz | no | no | helper:tz | tz |

## Description

`parseTime()` parses a time string with the given format and optional timezone.

## Examples

### Basic

```js
between(parseTime('2023-03-01 14:00:00', 'DEFAULT', tz('Local')), parseTime('2023-03-01 14:05:00', 'DEFAULT', tz('Local')))
CSV()
```

## Related

time, tz, between
