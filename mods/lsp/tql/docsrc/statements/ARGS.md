# ARGS

## Kind

statement source

## Category

context source

## Signatures

```text
ARGS()
```

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| none | no | no | none | none |

## Description

`ARGS()` generates one record from values passed by a parent TQL flow. It is intended as the source of a sub-flow inside `WHEN(..., do(..., { ... }))`.

## Examples

### Use parent flow arguments

```js
FAKE(json({ [1, 'hello'], [2, 'world'] }))
WHEN(value(0) == 2, do(value(0), strToUpper(value(1)), {
    ARGS()
    WHEN(true, doLog('OUTPUT:', value(0), value(1)))
    DISCARD()
}))
CSV()
```

## Related

args, WHEN, do, doLog, DISCARD
