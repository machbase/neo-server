# HTML

## Kind

statement sink

## Category

template encoder

## Signatures

```text
HTML(templates...)
```

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| templates | no | yes | literal:string|expression | template strings |

## Description

`HTML()` generates an HTML document using the provided templates. It is documented as the HTML-oriented template sink.

## Examples

### HTML template

```js
SQL(`select * from example limit 10`)
HTML(`<html><body>{{ . }}</body></html>`)
```

## Related

TEXT, MARKDOWN, JSON
