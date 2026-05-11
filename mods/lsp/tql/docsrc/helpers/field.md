# field

## Kind

helper

## Category

csv source

## Signatures

```text
field(index, typefunc, name)
```

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| index | yes | no | literal:number | 0 |
| typefunc | yes | no | helper | stringType, datetimeType, doubleType, boolType |
| name | yes | no | literal:string | column name |

## Description

`field()` declares the type and name of an input CSV field. It is used by `CSV()` source parsing to map CSV columns to typed record values.

## Examples

### Basic

```js
CSV(payload(), field(0, stringType(), 'name'), field(1, datetimeType('DEFAULT', 'Local'), 'time'))
CSV()
```

## Related

CSV, stringType, datetimeType, timeType, doubleType, floatType, boolType
