# strSub

## Kind

helper

## Category

string

## Signatures

```text
strSub(str, offset)
```

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| args | yes | yes | expression | string arguments |

## Description

Returns a substring of str.

## Examples

### Basic

```js
MAPVALUE(0, strSub(value(0)))
CSV()
```

## Related

value, MAPVALUE
