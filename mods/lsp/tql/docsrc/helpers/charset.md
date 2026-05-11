# charset

## Kind

helper

## Category

csv source

## Signatures

```text
charset(name)
```

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| name | yes | no | literal:string | UTF-8, EUC-KR, SHIFT_JIS |

## Description

`charset()` specifies the character set for non UTF-8 CSV input. The manual lists common encodings including UTF-8, EUC-KR, SJIS, CP932, SHIFT_JIS, EUC-JP, UTF-16 variants, ISO-8859 variants, KOI8, Windows code pages, and others.

## Examples

### Basic

```js
CSV(file('/absolute/path/to/data.csv'), charset('EUC-KR'))
CSV()
```

## Related

CSV, file, payload
