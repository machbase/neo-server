# ansiTimeformat

## Kind

helper

## Category

time

## Signatures

```text
ansiTimeformat(format)
```

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| format | yes | no | literal:string | yyyy-mm-dd hh:nn:ss.ffffff |

## Description

`ansiTimeformat()` specifies an ANSI-style time formatting pattern. Tokens include `yyyy`, `mm`, `dd`, `hh`, `nn`, `ss`, and fractional second `f` digits.

## Examples

### Basic

```js
CSV(ansiTimeformat('yyyy-mm-dd hh:nn:ss.ffffff'), tz('UTC'))
```

## Related

sqlTimeformat, tz, strTime, CSV, JSON
