# HTTP

## Kind

statement source_or_map

## Category

http

## Signatures

```text
HTTP(args...)
```

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| args | no | yes | expression | HTTP DSL options |

## Description

`HTTP()` performs HTTP requests through TQL's simple HTTP DSL. The official manual points to the dedicated HTTP section for detailed request examples.

## Examples

### Placeholder

```js
HTTP()
```

## Related

payload, file, escapeParam
