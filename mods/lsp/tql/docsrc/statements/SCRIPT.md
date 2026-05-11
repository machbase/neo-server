# SCRIPT

## Kind

statement source_or_map

## Category

script

## Signatures

```text
SCRIPT(args...)
```

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| args | no | yes | expression | script options |

## Description

`SCRIPT()` supports user-defined script execution in TQL flows. The official manual points to the dedicated SCRIPT section for detailed language-specific examples.

## Examples

### Placeholder

```js
SCRIPT()
```

## Related

ARGS, do, context

## Source

### Kind

statement source

### Category

script source

### Signatures

```text
SCRIPT(args...)
```

### Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| args | no | yes | expression | script options |

### Description

As a SRC function, `SCRIPT()` starts a TQL flow from user-defined script output.

### Related

ARGS, do, context

## Map

### Kind

statement map

### Category

script map

### Signatures

```text
SCRIPT(args...)
```

### Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| args | no | yes | expression | script options |

### Description

As a MAP function, `SCRIPT()` transforms records already flowing through the TQL pipeline.

### Related

ARGS, do, context
