# param

## Kind

helper

## Category

context

## Signatures

```text
param(name)
```

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| name | yes | no | literal:string | query parameter name |

## Description

`param()` returns a requested query parameter when a TQL script is called via HTTP.

## Examples

### Basic

```js
SQL(`SELECT time, value FROM example WHERE name = ?`, param('name') ?? 'temperature')
CSV()
```

## Related

SQL, payload, escapeParam
