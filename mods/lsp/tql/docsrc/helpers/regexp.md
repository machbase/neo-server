# regexp

## Kind

helper

## Category

string match

## Signatures

```text
regexp(expression, text)
```

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| expression | yes | no | literal:string | regular expression |
| text | yes | no | literal:string|expression | value |

## Description

`regexp()` returns true if text matches the regular expression.

## Examples

### Basic

```js
WHEN(regexp(`^map\.[2,3]$`, value(0)), doLog('found', value(1)))
CSV()
```

## Related

glob, WHEN, doLog
