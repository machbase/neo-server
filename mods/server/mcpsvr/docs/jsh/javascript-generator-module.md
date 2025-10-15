# Machbase Neo JavaScript Generator Module

## arrange()

Returns array of numbers.

**Syntax**

```js
arrange(start, end, step)
```

**Parameters**

- `start` `Number` start from
- `end` `Number` end to
- `step` `Number` increments

**Return value**

`Number[]` generated numbers in an array.

**Usage example**

```js
const { arrange } = require("@jsh/generator")
arrange(0, 6, 3).forEach((i) => console.log(i))

// 0
// 3
// 6
```

## linspace()

Returns array of numbers.

**Syntax**

```js
linspace(start, end, count)
```

**Parameters**

- `start` `Number` start from
- `end` `Number` end to
- `count` `Number` total count of numbers to generate

**Return value**

`Number[]` generated numbers in an array.

**Usage example**

```js
const { linspace } = require("@jsh/generator")
linspace(0, 1, 3).forEach((i) => console.log(i))

// 0
// 1.5
// 1
```

## meshgrid()

Returns array of numbers array.

**Syntax**

```js
meshgrid(arr1, arr2)
```

**Parameters**

- `arr1` `Number[]`
- `arr2` `Number[]`

**Return value**

`Number[][]` generated numbers in an array of numbers.

**Usage example**

```js
const { meshgrid } = require("@jsh/generator")

const gen = meshgrid([1, 2, 3], [4, 5]);
for(i=0; i < gen.length; i++) {
    console.log(JSON.stringify(gen[i]));
}

// [1,4]
// [1,5]
// [2,4]
// [2,5]
// [3,4]
// [3,5]
```

## random()

Returns a random number between [0.0, 1.0).

**Syntax**

```js
random()
```

**Parameters**

None.

**Return value**

`Number` random number between 0.0 and 1.0 : `[0.0, 1.0)`

**Usage example**

```js
const { random } = require("@jsh/generator")
for(i=0; i < 3; i++) {
    console.log(random().toFixed(2))
}

// 0.54
// 0.12
// 0.84
```

## Simplex

A noise generator based on the Simplex noise algorithm.

**Syntax**

```js
new Simplex(seed)
```

**Parameters**

`seed` seed number.

**Return value**

A new Simplex generator object.

### eval()

Returns a random noise value. Repeated calls with the same args inputs will have the same output.

**Syntax**

```js
eval(arg1)
eval(arg1, arg2)
eval(arg1, arg2, arg3)
eval(arg1, arg2, arg3, arg4)
```

**Parameters**

`args` `Number` A variable-length list of numbers, representing dimensions. The function accepts a minimum of one argument (1-dimensional) and a maximum of four arguments (4-dimensional).

**Return value**

`Number` random noise value

**Usage example**

```js
const g = require("@jsh/generator")
simplex = new g.Simplex(123);
for(i=0; i < 5; i++) {
    noise = simplex.eval(i, i * 0.6).toFixed(3);
    console.log(i, (i*0.6).toFixed(1), "=>", noise);
}

// 0 0.0 => 0.000
// 1 0.6 => 0.349
// 2 1.2 => 0.319
// 3 1.8 => 0.038
// 4 2.4 => -0.364
```

## UUID

UUID generator

**Syntax**

```js
new UUID(ver)
```

**Parameters**

`ver` UUID version number. It should be one of 1, 4, 6, 7.

**Return value**

a new UUID generator object.

### eval()

**Syntax**

```js
eval()
```

**Parameters**

None.

**Return value**

`String` a new UUID.

**Usage example**

```js
const {UUID} = require("@jsh/generator")
gen = new UUID(1);
for(i=0; i < 3; i++) {
    console.log(gen.eval());
}

// 868c8ec0-2180-11f0-b223-8a17cad8d69c
// 868c97b2-2180-11f0-b223-8a17cad8d69c
// 868c98d4-2180-11f0-b223-8a17cad8d69c
```