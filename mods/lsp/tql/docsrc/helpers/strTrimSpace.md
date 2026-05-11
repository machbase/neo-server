# strTrimSpace

## Kind

helper

## Category

string

## Signatures

```text
strTrimSpace(str)
```

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| args | yes | yes | expression | string arguments |

## Description

Returns the string with leading and trailing white space removed.

## Examples

### Basic

```js
MAPVALUE(0, strTrimSpace(value(0)))
CSV()
```

## Related

value, MAPVALUE
