# strTrimSuffix

## Kind

helper

## Category

string

## Signatures

```text
strTrimSuffix(str, suffix)
```

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| args | yes | yes | expression | string arguments |

## Description

Returns the string without the provided trailing suffix when it is present.

## Examples

### Basic

```js
MAPVALUE(0, strTrimSuffix(value(0)))
CSV()
```

## Related

value, MAPVALUE
