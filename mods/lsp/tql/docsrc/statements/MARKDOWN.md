# MARKDOWN

## Kind

statement sink

## Category

text encoder

## Signatures

```text
MARKDOWN(options...)
```

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| options | no | yes | helper | tz, sqlTimeformat, ansiTimeformat |

## Description

`MARKDOWN()` renders incoming records as a Markdown table, or as HTML when the HTML option is enabled. It can also omit long result rows with brief output options.

## Examples

### Markdown table

```js
FAKE(csv(`
10,The first line
20,2nd line
30,Third line
`))
MARKDOWN()
```

## Related

HTML, TEXT, CSV, tz, sqlTimeformat, ansiTimeformat
