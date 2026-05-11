# glob

## Kind

helper

## Category

string match

## Signatures

```text
glob(pattern, text)
```

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| pattern | yes | no | literal:string | *.3 |
| text | yes | no | literal:string|expression | value |

## Description

`glob()` returns true if text matches the glob pattern.

## Examples

### Basic

```js
WHEN(glob('*.3', value(0)), doLog('found', value(1)))
CSV()
```

## Related

regexp, WHEN, doLog
