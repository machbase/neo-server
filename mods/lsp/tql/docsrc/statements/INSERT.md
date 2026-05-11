# INSERT

## Kind

statement sink

## Category

database sink

## Signatures

```text
INSERT(columns..., table)
INSERT(bridge, columns..., table)
INSERT(columns..., table, tag)
```

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| bridge | no | no | helper:bridge | bridge |
| columns | yes | yes | literal:string | column names |
| table | yes | no | helper:table | table |
| tag | no | no | helper:tag | tag |

## Description

`INSERT()` stores incoming records into a database table by executing an INSERT statement for each record. `tag()` can be used for tag tables, and `bridge()` can redirect inserts to a bridged database.

## Examples

### Insert records

```js
FAKE(json({
    ['temperature', 1708582790, 23.45],
    ['temperature', 1708582791, 24.56]
}))
MAPVALUE(1, value(1) * 1000000000)
INSERT('name', 'time', 'value', table('example'))
```

### Insert with tag

```js
FAKE(json({ [1708582792, 32.34], [1708582793, 33.45] }))
MAPVALUE(0, value(0) * 1000000000)
INSERT('time', 'value', table('example'), tag('temperature'))
```

## Related

bridge, table, tag, APPEND, PUSHVALUE, MAPVALUE
