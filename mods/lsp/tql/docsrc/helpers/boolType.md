# boolType

## Kind

helper

## Category

csv source

## Signatures

```text
boolType()
```

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| args | no | yes | expression | time format or timezone |

## Description

Type helper used by `field()` when parsing CSV input.

## Examples

### Basic

```js
CSV(payload(), field(0, boolType(), 'value'))
CSV()
```

## Related

CSV, field
