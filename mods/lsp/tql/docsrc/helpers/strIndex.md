# strIndex

## Kind

helper

## Category

string

## Signatures

```text
strIndex(str, substr)
```

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| args | yes | yes | expression | string arguments |

## Description

Returns the index of the first instance of substr in str, or -1 if substr is not present.

## Examples

### Basic

```js
MAPVALUE(0, strIndex(value(0)))
CSV()
```

## Related

value, MAPVALUE
