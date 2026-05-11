# timeType

## Kind

helper

## Category

csv source

## Signatures

```text
timeType(format, timezone)
```

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| args | no | yes | expression | time format or timezone |

## Description

Datetime type helper used by `field()` when parsing CSV input. It accepts epoch units such as `s`, `ms`, `us`, `ns`, or named time formats with an optional timezone.

## Examples

### Basic

```js
CSV(payload(), field(0, timeType(), 'value'))
CSV()
```

## Related

CSV, field
