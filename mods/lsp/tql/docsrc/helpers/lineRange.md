# lineRange

## Kind

helper

## Category

arrays and dictionaries

## Signatures

```text
lineRange(-count)
lineRange(offset, count)
```

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| offset | yes | no | integer | `-1000`, `0`, `100` |
| count | no | no | integer | `10`, `100` |

## Description

Selects the lines retained from `SHELL()` command output.

- `lineRange(-count)` keeps the last `count` lines.
- `lineRange(offset, count)` skips `offset` lines, then keeps the next `count` lines.

`offset` is zero-based. A negative offset is valid only in the one-argument tail form.

## Examples

### Basic

```js
SHELL("journalctl", "-n", "10000", lineRange(-1000))
```

### Forward Range

```js
SHELL("seq", "1", "100", lineRange(10, 5))
```

## Related

SHELL
