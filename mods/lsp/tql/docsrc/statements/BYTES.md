# BYTES

## Kind

statement source

## Category

bytes source

## Signatures

```text
BYTES(src)
BYTES(src, options...)
```

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| src | yes | no | stream|string|helper:file|helper:payload | payload, file |
| options | no | yes | helper | separator, trimspace |

## Description

`BYTES()` splits input content by separator and yields records with byte-array values. It behaves like `STRING()` except for the yielded value type.

## Examples

### Read lines

```js
BYTES(payload() ?? file('/absolute/path/to/data.bin'), separator('\n'))
CSV()
```

## Related

STRING, file, payload, separator, trimspace
