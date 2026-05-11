# DISCARD

## Kind

statement sink

## Category

discard sink

## Signatures

```text
DISCARD()
```

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| none | no | no | none | none |

## Description

`DISCARD()` silently ignores all incoming records and produces no output. It is useful inside nested flows where work is performed for side effects such as logging.

## Examples

### Discard nested output

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

WHEN, do, doLog, ARGS
