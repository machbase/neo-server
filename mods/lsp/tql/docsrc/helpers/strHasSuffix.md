# strHasSuffix

## Kind

helper

## Category

string

## Signatures

```text
strHasSuffix(str, suffix)
```

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| args | yes | yes | expression | string arguments |

## Description

Tests whether the string ends with suffix.

## Examples

### Basic

```js
MAPVALUE(0, strHasSuffix(value(0)))
CSV()
```

## Related

value, MAPVALUE
