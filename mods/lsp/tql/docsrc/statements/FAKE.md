# FAKE

## Kind

statement source

## Category

generator source

## Signatures

```text
FAKE(generator)
```

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| generator | yes | no | helper | oscillator, meshgrid, linspace, arrange, csv, json |

## Description

`FAKE()` produces artificial records from a generator helper. It is commonly used for examples, tests, synthetic wave data, and inline CSV or JSON data.

## Examples

### Generate rows

```js
FAKE(arrange(1, 2, 0.5))
CSV()
```

### Generate JSON rows

```js
FAKE(json({
    ['A', 1, true],
    ['B', 2, false],
    ['C', 3, true]
}))
CSV()
```

## Related

oscillator, freq, range, arrange, linspace, meshgrid, csv, json, random
