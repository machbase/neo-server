# strTrimPrefix

## Kind

helper

## Category

string

## Signatures

```text
strTrimPrefix(str, prefix)
```

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| args | yes | yes | expression | string arguments |

## Description

Returns the string without the provided leading prefix when it is present.

## Examples

### Basic

```js
MAPVALUE(0, strTrimPrefix(value(0)))
CSV()
```

## Related

value, MAPVALUE
