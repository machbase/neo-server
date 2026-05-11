# STRING

## Kind

statement source

## Category

string source

## Signatures

```text
STRING(src)
STRING(src, options...)
```

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| src | yes | no | stream|string|helper:file|helper:payload | payload, file |
| options | no | yes | helper | separator, trimspace |

## Description

`STRING()` splits input text by separator and yields records with string values. Without a separator it reads the whole input as one record; with `trimspace(true)` it trims spaces from each split value.

## Examples

### Split lines

```js
STRING(payload() ?? `12345
    23456
    78901`, separator('\n'), trimspace(true))
CSV()
```

### Read remote text

```js
STRING(file(`http://example.com/data/words.txt`), separator('\n'))
CSV()
```

## Related

BYTES, file, payload, separator, trimspace
