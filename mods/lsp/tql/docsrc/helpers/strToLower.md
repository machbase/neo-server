# strToLower

## Kind

helper

## Category

string

## Signatures

```text
strToLower(str)
```

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| args | yes | yes | expression | string arguments |

## Description

Returns the string with Unicode letters mapped to lower case.

## Examples

### Basic

```js
MAPVALUE(0, strToLower(value(0)))
CSV()
```

## Related

value, MAPVALUE
