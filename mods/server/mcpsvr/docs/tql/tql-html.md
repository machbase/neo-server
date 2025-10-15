# Machbase Neo TQL HTML

The `HTML()` SINK generates an HTML document or an element as output, using the provided template language for formatting. This allows you to fully customize the structure and appearance of the HTML output based on your query results.

**Syntax**: `HTML(templates...)`

*Version 8.0.53 or later*

**Parameters**:
- `templates`: One or more template strings or `file(path)` references. Each argument can be a direct template string or a file path using `file(path)` to load the template from a file. The template content uses the Go HTML template language. For more information, see the [template documentation](https://pkg.go.dev/html/template).
- `cache()`: Cache result data. See [Cache Result Data](../reading/#cache-result-data) for details.

Within the template, you have access to a value object that exposes the current record's field values and row number. The following fields and properties are available within the HTML template context:

## Methods

- `{{ .Columns }}`
- `{{ .Column <idx>}}`
- `{{ .Values }}`
- `{{ .Value <idx> }}`
- `{{ .ValueHTMLAttr <idx> }}`
- `{{ .ValueCSS <idx> }}`
- `{{ .ValueJS <idx> }}`
- `{{ .ValueURL <idx> }}`
- `{{ .ValueString <idx> }}`
- `{{ .V.<field> }}`
- `{{ .Num }}`

## Functions

### timeformat

**Syntax**

```
{{ timeformat <format> <timezone> }}
```

**Usage Example**

```html
SCRIPT({
    const { now } = require("@jsh/system");
    $.yield(now(), "Hello World");
})
HTML({
    <li>{{ $.Value 0 | timeformat "RFC3339" "UTC" }}
    <li>{{ $.Value 1 }}
})
```

**Output:**

```html
<li>2025-05-29T08:32:33Z
<li>Hello World
```

### format

**Usage Example**

```html
SCRIPT({
    $.yield(3.1415, "Hello World");
})
HTML({
    <li>{{ $.Value 0 | format "%.2f" }}
    <li>{{ $.Value 1 | format "Say: %s?" }}
})
```

**Output:**

```html
<li> 3.14
<li> Say: Hello World?
```

### param

```html
SCRIPT({
    $.yield(3.1415, "Hello World");
})
HTML({
    <li> {{ param "prefix" }} {{ $.Value 0 }}
    <li> {{ param "prefix" }} {{ $.Value 1 }}
})
```

**Note**: Invoke the TQL script with parameters `?param=Line`.

**Output:**

```html
<li> Line 3.1415
<li> Line Hello World
```

### paramDefault

```html
SCRIPT({
    $.yield(3.1415, "Hello World");
})
HTML({
    <li> {{ paramDefault "prefix1" "Line1" }} {{ $.Value 0 }}
    <li> {{ paramDefault "prefix2" "Line2" }} {{ $.Value 1 }}
})
```

**Output:**

```html
<li> Line1 3.1415
<li> Line2 Hello World
```

### toLower

```html
SCRIPT({
    $.yield(3.1415, "Hello World");
})
HTML({
    <li> {{ $.Value 0 | format "%.2f" }}
    <li> {{ $.Value 1 | toLower | format "Say: %s?" }}
})
```

**Output:**

```html
<li> 3.14
<li> Say: hello world?
```

### toUpper

```html
SCRIPT({
    $.yield(3.1415, "Hello World");
})
HTML({
    <li> {{ $.Value 0 | format "%.2f" }}
    <li> {{ $.Value 1 | toUpper | format "Say: %s?" }}
})
```

**Output:**

```html
<li> 3.14
<li> Say: HELLO WORLD?
```

## Usage Examples

### Using .V (Field Map)

`.V` is a map object containing field names as keys and their corresponding values.

```html
SQL(`SELECT NAME, TIME, VALUE FROM EXAMPLE LIMIT 5`)
HTML({
  {{ if .IsFirst }}
    <html>
    <body>
      <h2>HTML Template Example</h2>
      <hr>
      <table>
      <tr>
        {{range .Columns}}
          <th>{{ . }}</th>
        {{end}}
      </tr>
  {{ end }}
      <tr>
        <td>{{ .V.NAME }}</td>
        <td>{{ .V.TIME | timeformat "RFC3339" "Asia/Seoul"}}</td>
        <td>{{ .V.VALUE }}</td>
      </tr>
  {{ if .IsLast }}
      </table>
      <hr>
        Total: {{ .Num }}
    </body>
    </html>
  {{ end }}
})
```

### Using .Value (Index Access)

`.Value` is a function that accesses the fields of the current record by their index.

```html
FAKE( csv(`
10,The first line 
20,2nd line
30,Third line
40,4th line
50,The last is 5th
`))
HTML({
    {{ if .IsFirst }}
        <html>
        <body>
            <h2>HTML Template Example</h2>
            <hr>
    {{ end }}

    <li>{{ .Value 0 }} : {{ .Value 1 }}
    
    {{ if .IsLast }}
        <hr>
        Total: {{ .Num }}
        </body>
        </html>
    {{ end }}
})
```

### Using .Values (Array Access)

`.Values` is an array containing all field values of the current record.

```html
FAKE( csv(`
10,The first line 
20,2nd line
30,Third line
40,4th line
50,The last is 5th
`))
HTML({
    {{ if .IsFirst }}
        <html>
        <body>
            <h2>HTML Template Example</h2>
            <hr>
    {{ end }}

    <li>{{ (index .Values 0) }} : {{ (index .Values 1 ) }}
    
    {{ if .IsLast }}
        <hr>
        Total: {{ .Num }}
        </body>
        </html>
    {{ end }}
})
```

## Context-Aware Escaping

The template understands HTML, CSS, JavaScript and URIs. It adds sanitizing functions to each simple action pipeline, so given the excerpt.

Each `{{.Value 0}}`, `{{.Value 1}}`, and `{{.Value 2}}` is overwritten to add escaping functions as necessary.

**Example:**

```html
SCRIPT({
    $.yield(
        `http://maven.org/`,
        ``,
        "Java")
    $.yield(
        `http://npmjs.com/`,
        ``,
        "JavaScript")
})
HTML({
    <li>
        <a href="{{.Value 0}}">
        {{.ValueHTML 1}}{{.Value 2}}
        </a>
    </li>
})
```

**Output:**

```html
<li>
  <a href="http://maven.org/">
    Java
  </a>
</li>
<li>
  <a href="http://npmjs.com/">
    JavaScript
  </a>
</li>
```

### HTML Context Escaping

Assuming `{{.Value 0}}` is `O'Reilly: How are <i>you</i>?`, the examples below show how `{{.Value 0}}` appears when used in contexts.

**In HTML Body:**

```html
SCRIPT({ $.yield(`O'Reilly: How are <i>you</i>?`) })
HTML({
  {{.Value 0}}
})

// Output:
//  O&#39;Reilly: How are &lt;i&gt;you&lt;/i&gt;?
```

**In URL Parameter:**

```html
SCRIPT({ $.yield(`O'Reilly: How are <i>you</i>?`) })
HTML(`<a href="/path?p={{.ValueHTML 0}}">`)

// Output:
//  <a href="/path?p=O%27Reilly%3a%20How%20are%20%3ci%3eyou%3c%2fi%3e%3f">
```

**In JavaScript Context:**

```html
SCRIPT({ $.yield(`O'Reilly: How are <i>you</i>?`) })
HTML(`<a onx='f("{{.Value 0}}")'>`)

// Output:
//  <a onx="f('O\u0027Reilly: How are \u003ci\u003eyou\u003c\/i\u003e?')">
```

**JavaScript Function Example:**

```html
SCRIPT({ $.yield(`Hello World?`, `function doMsg(msg){ console.log(msg); }`) })
HTML({
  <script>
  {{.ValueJS 1}}
  </script>
  <a onClick='doMsg("{{.Value 0}}")'>here</a>
})

// Output:
// <script>
// function doMsg(msg){ console.log(msg); }
// </script>
// <a onClick='doMsg("Hello World?")'>here</a>
```

### Non-String Values in JavaScript

Non-string values can be used in JavaScript contexts. If the record is an object:

```html
SCRIPT({
    $.yield({A: "foo", B:"bar"})
})
HTML({
    <script>var pair = {{ .Value 0 }};</script>
})
```

**Output:**

```html
<script>var pair = {"A":"foo","B":"bar"};</script>
```

## Unescaped Strings

By default, the template assumes that all pipelines produce a plain text string. It adds escaping pipeline stages necessary to correctly and safely embed that plain text string in the appropriate context.

When a data value is not plain text, you can make sure it is not over-escaped by marking it with its type.

Types HTML, JS, URL, and others can carry safe content that is exempted from escaping.

**Example:**

```html
SCRIPT({
    $.yield(`<b>World</b>`)
})
HTML({
    Hello, {{ .ValueHTML 0 }}!
})
```

**Output:**

```html
Hello, <b>World</b>!
```

Instead of:

```
Hello, &lt;b&gt;World&lt;b&gt;!
```