# payload

## Kind

helper

## Category

context

## Signatures

```text
payload()
```

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| none | no | no | none | none |

## Description

`payload()` returns the current input stream sent by the caller. For HTTP it is the POST request body; for MQTT it is the PUBLISH message payload.

## Examples

### Basic

```js
CSV(payload() ?? file('/absolute/path/to/data.csv'))
CSV()
```

## Related

CSV, BYTES, STRING, param
