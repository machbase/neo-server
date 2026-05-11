# strReplace

## Kind

helper

## Category

string

## Signatures

```text
strReplace(str, old, new, n)
```

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| args | yes | yes | expression | string arguments |

## Description

Returns a copy of the string with the first n non-overlapping instances of old replaced by new; a negative n means no limit.

## Examples

### Basic

```js
MAPVALUE(0, strReplace(value(0)))
CSV()
```

## Related

value, MAPVALUE
