# list

## Kind

helper

## Category

list

## Signatures

```text
list(args...)
```

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| args | no | yes | expression | value, literals |

## Description

`list()` returns a new tuple containing its arguments as elements.

## Examples

### Basic

```js
MAPVALUE(0, list(value(0), value(1)))
CSV()
```

## Related

dict, count, value
