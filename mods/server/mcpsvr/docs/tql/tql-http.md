# Machbase Neo TQL HTTP

The `HTTP()` SRC allows you to send HTTP requests and view the responses directly within TQL scripts. This is useful for integrating external APIs or testing HTTP endpoints as part of your data workflows.

**Syntax**: `HTTP(text)`

*Version 8.0.53 or later*

**Parameters:**
- `text` - String, HTTP request description

## TQL Usage

The syntax follows [RFC 2616](https://www.rfc-editor.org/rfc/rfc2616) and supports specifying the request method, headers, and body.

### Example 1: TEXT Output

```js
HTTP({
    GET http://127.0.0.1:5654/db/query
        ?q=select * from example limit 3
        &format=csv
        &timeformat=default
        &tz=UTC
})
TEXT()
```

### Example 2: HTML Simple Output

```html
HTTP({
    POST http://127.0.0.1:5654/db/query
    Content-Type: application/json

    {
        "q": "select * from example limit 3",
        "format": "csv",
        "timeformat": "default"
    }
})
HTML(`<pre>{{ .Value 0 }}</pre>`)
```

Once you prepare a request, execute the TQL. The result view will show the HTTP response, including headers and body.

**Response:**

```
HTTP/1.1 200 OK
Content-Length: 212
Content-Type: text/csv; charset=utf-8
Date: Mon, 02 Jun 2025 03:42:33 GMT

NAME,TIME,VALUE
work-11-0,2025-03-19 01:56:19.824,0.00
work-11-0,2025-03-19 01:56:19.824,1.00
work-11-0,2025-03-19 01:56:19.824,2.00
```

### Worksheet Usage

Use `http` code-fence within a markdown cell.

**Example:**

~~~text
### HTTP Client Example

```http
POST http://127.0.0.1:5654/db/query
    Content-Type: application/json

{
    "q": "select * from example limit 3",
    "format": "box",
    "timeformat": "default"
}
```
~~~

### Markdown Usage

Code-fence `http` works in markdown file `.md`.

~~~
## HTTP Example

Code fence with `http`.

```http
GET http://127.0.0.1:5654/db/query
    ?q=select * from example limit 2
    &format=ndjson
    &timeformat=default&tz=local
```
~~~

## Query Strings

You can include query strings directly in the request line:

~~~
```http
GET https://example.com/comments?page=2&pageSize=10
```
~~~

If there are many query parameters, you can spread them across multiple lines for better readability. Lines immediately after the request line that start with `?` or `&` are parsed as query parameters:

~~~
```http
GET https://example.com/comments
    ?page=2
    &pageSize=10
```
~~~

## Request Headers

Lines immediately after the request line (and any query string lines) up to the first empty line are parsed as request headers. Headers should use the standard `field-name: field-value` format, one per line.

**Example:**

```
User-Agent: http-client
Accept-Language: en-GB,en-US;q=0.8,en;q=0.6,zh-CN;q=0.4
Content-Type: application/json
```

## Request Body

To provide a request body, add a blank line after the headers. All content after this blank line is treated as the request body.

**Example:**

~~~
```http
POST https://example.com/comments HTTP/1.1
Content-Type: application/xml
Authorization: token xxx

<request>
    <name>sample</name>
    <time>Wed, 21 Oct 2015 18:27:50 GMT</time>
</request>
```
~~~

### External File

You can also specify a file as the request body by starting the line with `<` followed by the file path as shown in the file explorer. Alternatively, use the `@` prefix before the path to indicate that it is an absolute path on the operating system.

**File Path Examples:**
- `< /doc.xml` — refers to a file located in the TQL root directory.  
- `< @/home/data/doc.xml` — refers to a file located at an absolute path on the operating system.

~~~
```http
POST https://example.com/comments HTTP/1.1
Content-Type: application/xml
Authorization: token xxx

< /data/demo.xml
```
~~~

## Multipart Form Data

When the request body is `multipart/form-data`, you can mix text and file uploads:

~~~
```http
POST https://api.example.com/user/upload
Content-Type: multipart/form-data; boundary=----Boundary7MA4YWxkTrZu0gW

------Boundary7MA4YWxkTrZu0gW
Content-Disposition: form-data; name="text"

title
------Boundary7MA4YWxkTrZu0gW
Content-Disposition: form-data; name="image"; filename="1.png"
Content-Type: image/png

< /data/1.png
------Boundary7MA4YWxkTrZu0gW--
```
~~~

## x-www-form-urlencoded

For `application/x-www-form-urlencoded` content, you can split the body into multiple lines. Each key-value pair should be on its own line, starting with `&` after the first line:

~~~
```http
POST https://api.example.com/login HTTP/1.1
Content-Type: application/x-www-form-urlencoded

name=foo
&password=bar
```
~~~

---

This flexible HTTP request syntax allows you to easily test and automate API calls directly from your TQL scripts, supporting a wide range of HTTP features and formats.