# strToUpper

## Kind

helper

## Category

string

## Signatures

```text
strToUpper(str)
```

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| args | yes | yes | expression | string arguments |

## Description

Returns the string with Unicode letters mapped to upper case.

## Examples

### Basic

```js
MAPVALUE(0, strToUpper(value(0)))
CSV()
```

## Related

value, MAPVALUE
