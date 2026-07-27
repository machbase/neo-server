# katex

katex is a Goldmark extension for rendering inline and block math.

## Purpose

This extension renders Markdown math in mdconv using KaTeX via katexdsl.

## Math Syntax

### Inline Math

Inline math uses `$` + backtick payload + `$`.

~~~markdown
The equation is $`x^2 + y^2 = z^2`$.
~~~

### Block Math

Block math uses `$$` opening and closing markers.

~~~markdown
$$
\int_0^1 x^2 dx
$$
~~~

Blank lines inside a block are supported.

## Block Option Syntax

Block options are supported on the opening `$$` line using Hugo-style inline options.

~~~markdown
$$ {align=left,width=70%,class="math-note",leqno=true}
\begin{matrix}
  a & b \\
  c & d
\end{matrix}
$$
~~~

Notes:

- A whitespace is required between `$$` and `{...}` to be recognized as option syntax.
- Option form is `key=value`, separated by commas.
- Quoted strings are supported.
- If option parsing fails, the opening content is treated as equation text (no parse error is raised).

## Supported Block Options

The following keys are currently implemented for `$$` blocks.

| Key | Type | Example | Applied behavior |
| --- | --- | --- | --- |
| align | string | `align=left` | Adds wrapper flex alignment for block placement (`justify-content:flex-start|center|flex-end`) |
| width | string | `width=70%` | Adds wrapper style `width:<value>` |
| class | string | `class="math-note"` | Appends class name to block wrapper |
| style | string | `style="margin:8px 0"` | Appends raw inline style text to block wrapper |
| throwOnError | bool | `throwOnError=false` | Overrides KaTeX error policy for this block |
| output | string | `output=mathml` | Overrides KaTeX output mode for this block |
| leqno | bool | `leqno=true` | Passes `leqno` to KaTeX render option |
| fleqn | bool | `fleqn=true` | Passes `fleqn` to KaTeX render option |

Current behavior details:

- Unknown keys are ignored.
- `align` applies only for `left`, `center`, `right`; other values are ignored.
- `align` is implemented with wrapper styles `display:flex` and `justify-content:*` to affect block-level placement.
- `width` and `style` are not strictly validated.
- `output` is passed through as provided.

## Option Precedence

Current precedence order:

1. Block options (`$$ { ... }`)
2. Extender-level defaults (`RenderOptions`)
3. Built-in defaults

Built-in defaults include `output=mathml`.

## Extender-Level Options

`RenderOptions` currently supports:

- `ThrowOnError`
- `InlineWrapperClass`
- `BlockWrapperClass`
- `Output`
- `Leqno`
- `Fleqn`

## Examples

### Alignment and width

~~~markdown
$$ {align=left,width=60%}
x = y^\pi
$$
~~~

### Wrapper class and style

~~~markdown
$$ {class="math-callout",style="margin:12px 0"}
\sum_{i=1}^{n} i = \frac{n(n+1)}{2}
$$
~~~

### Per-block error override

~~~markdown
$$ {throwOnError=false}
\frac{
$$
~~~
