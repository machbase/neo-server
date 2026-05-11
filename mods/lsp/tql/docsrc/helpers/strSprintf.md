# strSprintf

## Kind

helper

## Category

string

## Signatures

```text
strSprintf(format, args...)
```

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| args | yes | yes | expression | string arguments |

## Description

Formats according to a printf-style format specifier and returns the resulting string.

## Examples

### Basic

```js
MAPVALUE(0, strSprintf(value(0)))
CSV()
```

## Related

value, MAPVALUE
