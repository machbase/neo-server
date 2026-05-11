# json

## Kind

helper

## Category

generator

## Signatures

```text
json(value)
```

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| args | yes | yes | expression | generator arguments |

## Description

`json()` generates records from inline JSON array data for `FAKE()`.

## Examples

### Basic

```js
FAKE(json({
    ['A', 1, true],
    ['B', 2, false],
    ['C', 3, true]
}))
CSV()
```

## Related

FAKE, freq, range, linspace, meshgrid, csv, json
