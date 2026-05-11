# csv

## Kind

helper

## Category

generator

## Signatures

```text
csv(content)
```

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| args | yes | yes | expression | generator arguments |

## Description

`csv()` generates records from inline CSV content for `FAKE()`.

## Examples

### Basic

```js
FAKE(csv(strTrimSpace(`
A,1,true
B,2,false
C,3,true
`)))
CSV()
```

## Related

FAKE, freq, range, linspace, meshgrid, csv, json
