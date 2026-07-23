# httpext

`httpext` is a Goldmark extender for `mdconv` that executes `http` code fences and renders request/response as plain code blocks.

## Purpose

This extender is designed for lightweight HTTP REST API testing directly from Markdown.

## Fence Language

Use `http` as the fence language:

~~~
```http
GET http://127.0.0.1:5654/db/query
    ?q=select count(*) from example
    &format=ndjson
```
~~~

## Output Behavior

- Default behavior: show both request and response.
- Request and response are rendered inside a single box separated by a simple divider.
- HTTP meta tokens (method/path/query/header/status) are rendered with `httpext-*` classes.
- JSON body highlighting uses the same Chroma lexer stack used by the existing markdown highlighting path.
- CSV body highlighting applies rainbow-style column coloring to make column boundaries easier to scan.
- CSV body delimiter is auto-detected from the response body (`,`, `|`, `;`, tab), with comma as fallback.
- Default renderer wraps long lines for readability.
- Line numbers are opt-in with `line-numbers=true`.
- JSON body indentation is enabled by default with two-space indentation.
- `indent=false` keeps JSON body formatting as-is.

## Options

Inline options use Hugo-style braces:

~~~
```http {hide-request=true}
GET http://127.0.0.1:5654/health
```
~~~

Supported keys:

- `hide-request=true`: hide the request block (response only).
- `line-numbers=true`: render with line numbers.
- `indent=false`: keep JSON body formatting as-is.
- `style-method="..."`
- `style-path="..."`
- `style-param-name="..."`
- `style-param-value="..."`
- `style-request-protocol="..."`
- `style-header-key="..."`
- `style-header-value="..."`
- `style-response-protocol="..."`
- `style-status-code="..."`
- `style-status-message="..."`
- `style-body="..."`
- `style-json-key="..."`
- `style-json-string="..."`
- `style-json-number="..."`
- `style-json-boolean="..."`
- `style-json-null="..."`
- `style-json-punct="..."`
- `style-csv-delim="..."`
- `style-csv-col-N="..."` (for example `style-csv-col-0`, `style-csv-col-1`)

Style key policy:

- `style-*` keys are strictly whitelisted.
- Unknown `style-*` keys are ignored and a visible warning is rendered in the output.

Example:

~~~
```http {style-method="color:#ff0000", style-json-key="font-weight:700"}
GET http://127.0.0.1:5654/db/query
    ?q=select count(*) from example
    &format=json
```

CSV example:

~~~
```http
GET http://127.0.0.1:5654/db/query?q=select%20*%20from%20example&format=csv
```
~~~

When the response content-type is CSV (for example `text/csv`), each column is wrapped with `httpext-csv-col-N` classes and a cyclic rainbow palette is applied by default.
Delimiter is inferred from the response body, so non-comma CSV such as pipe-delimited output is rendered with correct column boundaries.
~~~

## Raw Header Capture

To preserve response header raw bytes, `httpext` performs transport-level capture:

- It writes a raw HTTP request over TCP/TLS.
- It reads raw response bytes from the socket before `net/http` canonicalizes headers.
- It then renders captured bytes as-is.

This preserves header case/order from the wire response.

## Request Body Encoding

- If request headers include `Content-Encoding: gzip` and request body is not empty, `httpext` compresses the request body with gzip before sending.
- `Content-Length` is recalculated from the final bytes that are actually sent on the wire.
- If a `Content-Length` header is already provided in the fence, its value is replaced with the recalculated value for non-empty bodies.

## Request Body File Directives

`httpext` supports file directives in request body content, compatible with the existing HTTP DSL behavior.

- `< /ssfs/path/file`:
    load body content from server-side file system (SSFS).
- `< @/os/abs_path.file`:
    load body content from OS file path.

Notes:

- File path strings are UTF-8 and may include Korean characters.
- For multipart bodies, directives can be mixed inside each part payload line.
- Loaded file content is appended with a newline, same as existing HTTP DSL behavior.
- Legacy form `<@utf-8 /ssfs/path/file` is accepted for backward compatibility.
- Loader selection rule: if parsed path starts with `@`, OS loader is used; otherwise SSFS loader is used.

Multipart example:

~~~
```http
POST http://127.0.0.1:5654/db/write/STASH
Content-Type: multipart/form-data; boundary=----Boundary7MA4YWxkTrZu0gW

------Boundary7MA4YWxkTrZu0gW
Content-Disposition: form-data; name="NAME"

camera-1
------Boundary7MA4YWxkTrZu0gW
Content-Disposition: form-data; name="DATA"; filename="image.svg"
Content-Type: image/svg

< @/tmp/image.svg
------Boundary7MA4YWxkTrZu0gW--
```
~~~

## Response Body Decoding

- If `Content-Encoding: gzip` and the `Content-Type` is printable (for example JSON or text), `httpext` displays the decompressed body.
- Binary response bodies remain unchanged.
