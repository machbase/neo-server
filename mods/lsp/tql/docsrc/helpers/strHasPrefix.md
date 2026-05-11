# strHasPrefix

## Kind

helper

## Category

string

## Signatures

```text
strHasPrefix(str, prefix)
```

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| args | yes | yes | expression | string arguments |

## Description

Tests whether the string begins with prefix.

## Examples

### Basic

```js
MAPVALUE(0, strHasPrefix(value(0)))
CSV()
```

## Related

value, MAPVALUE
