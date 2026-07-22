# d2ext

`d2ext` is a Goldmark extension for rendering D2 fenced code blocks into SVG.

## Fence Option Syntax

Use Hugo-style fence options:

~~~markdown
```d2 {layout=dagre,theme="Cool Classics",sketch=true,nodesep=120,edgesep=45}
a -> b: connect
```
~~~

Option values follow `key=value` form.

- Strings: `theme="Cool Classics"`
- Booleans: `sketch=true`
- Integers: `nodesep=120`
- Floats: `scale=1.25`
- Arrays: `hl_lines=[2,3,"10-20"]` (parsed by the option parser; currently not consumed by D2 renderer)

## Supported d2ext Options

The following options are currently applied by `d2ext`:

- `theme` (string): D2 theme name. Must match one of the `d2themescatalog` `Theme.Name` values.
- `layout` (string): D2 layout engine name. Common values are `dagre` and `elk`.
- `algorithm` (string): ELK layout algorithm. Only valid when `layout=elk`.
- `sketch` (bool): Enables/disables sketch rendering.
- `nodesep` (int): Dagre node separation. Only valid when `layout=dagre`.
- `edgesep` (int): Dagre edge separation. Only valid when `layout=dagre`.
- `themeID` (int, optional): Directly sets D2 theme id. If both `theme` and `themeID` are set, `themeID` wins.
- `pad` (int, optional): SVG padding.
- `scale` (float, optional): SVG render scale.

### ELK `algorithm` List

When `layout=elk`, `d2ext` currently supports the following algorithms:

- `layered` (default)
- `mrtree`
- `random`

Example:

~~~markdown
```d2 {layout=elk,algorithm="mrtree"}
a -> b: connect
```
~~~

Note: Other ELK algorithms are rejected by `d2ext` to avoid runtime instability in the current Goja execution path.

## Theme Names (from d2themescatalog)

Use these exact `Theme.Name` values.

### Light Catalog

- Neutral Default
- Neutral Grey
- Flagship Terrastruct
- Cool Classics
- Mixed Berry Blue
- Grape Soda
- Aubergine
- Colorblind Clear
- Vanilla Nitro Cola
- Orange Creamsicle
- Shirley Temple
- Earth Tones
- Everglade Green
- Buttered Toast
- Terminal
- Terminal Grayscale
- Origami
- C4

### Dark Catalog

- Dark Mauve
- Dark Flagship Terrastruct

## Notes

- `theme` matching is case-insensitive.
- `layout` matching is case-insensitive.
- `algorithm` is only applied when `layout=elk` (for example: `algorithm="layered"`, `algorithm="mrtree"`).
- Unknown `theme` values return an error during rendering.
