# dict

## Kind

helper

## Category

list

## Signatures

```text
dict(name1, value1, pairs...)
```

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| pairs | yes | yes | literal:string, expression | name, value |

## Description

`dict()` returns a new dictionary containing name/value pairs.

## Examples

### Basic

```js
MAPVALUE(0, dict('name', value(0), 'value', value(1)))
JSON()
```

## Related

list, value, JSON
