# sqlTimeformat

## Kind

helper

## Category

time

## Signatures

```text
sqlTimeformat(format)
```

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| format | yes | no | literal:string | YYYY-MM-DD HH24:MI:SS.nnnnnn |

## Description

`sqlTimeformat()` specifies an SQL-style time formatting pattern. Tokens include `YYYY`, `YY`, `MM`, `DD`, `HH24`, `HH12`, `MI`, `SS`, `AM`, and fractional second `n` digits.

## Examples

### Basic

```js
CSV(sqlTimeformat('YYYY-MM-DD HH24:MI:SS.nnnnnn'), tz('Asia/Seoul'))
```

## Related

ansiTimeformat, tz, strTime, CSV, JSON
