# strReplaceAll

## Kind

helper

## Category

string

## Signatures

```text
strReplaceAll(str, old, new)
```

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| args | yes | yes | expression | string arguments |

## Description

Returns a copy of the string with all non-overlapping instances of old replaced by new.

## Examples

### Basic

```js
MAPVALUE(0, strReplaceAll(value(0)))
CSV()
```

## Related

value, MAPVALUE
