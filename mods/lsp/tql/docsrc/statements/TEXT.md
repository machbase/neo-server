# TEXT

## Kind

statement sink

## Category

template encoder

## Signatures

```text
TEXT(templates...)
```

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| templates | no | yes | literal:string|expression | template strings |

## Description

`TEXT()` generates text output using the provided templates. It behaves similarly to `HTML()`, but does not perform HTML escaping on data.

## Examples

### Text template

```js
SQL(`select * from example limit 10`)
TEXT(`{{ . }}`)
```

## Related

HTML, MARKDOWN, JSON
