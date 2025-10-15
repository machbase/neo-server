# Machbase Neo JavaScript Mat Module

## Dense()

Dense Matrix

**Creation**

```js
new Dense(r, c, data)
```

**Parameters**

- `r` `Number` rows
- `c` `Number` cols
- `data` `Number[]`

creates a new dense matrix with r rows and c columns.

### dims()

### at()

### set()

### T()

creates a new dense matrix of transposed.

```js
const mat = require("@jsh/mat");
A = new mat.Dense(2, 2, [
    1, 2,
    3, 4,
])
B = A.T()
console.log(mat.format(B))

// ⎡1 3⎤
// ⎣2 4⎦
```

### add()

```js
const mat = require("@jsh/mat");
A = new mat.Dense(2, 2, [
    1, 2,
    3, 4,
])
B = new mat.Dense(2, 2, [
    10, 20,
    30, 40,
])
C = new mat.Dense()
C.add(A, B) // C = A + B
console.log(mat.format(C))

// ⎡11 22⎤ 
// ⎣33 44⎦
```

### sub()

```js
const mat = require("@jsh/mat");
A = new mat.Dense(2, 2, [
    1, 2,
    3, 4,
])
B = new mat.Dense(2, 2, [
    10, 20,
    30, 40,
])
C = new mat.Dense()
C.sub(B, A) // C = B - A
console.log(mat.format(C))

// ⎡ 9 18⎤ 
// ⎣27 36⎦
```

### mul()

```js
const mat = require("@jsh/mat");
A = new mat.Dense(2, 2, [
    1, 2,
    3, 4,
])
B = new mat.Dense(2, 2, [
    10, 20,
    30, 40,
])
C = new mat.Dense()
C.mul(A, B) // C = A * B
console.log(mat.format(C))

// ⎡ 70 100⎤ 
// ⎣150 220⎦
```

### mulElem()

```js
const mat = require("@jsh/mat");
A = new mat.Dense(2, 2, [
    1, 2,
    3, 4,
])
B = new mat.Dense(2, 2, [
    10, 20,
    30, 40,
])
C = new mat.Dense()
C.mulElem(A, B)
console.log(mat.format(C))

// ⎡ 10 40⎤ 
// ⎣ 90 160⎦
```

### divElem()
### inverse()

```js
const mat = require("@jsh/mat");
A = new mat.Dense(2, 2, [
    1, 2,
    3, 4,
])

B = new mat.Dense()
B.inverse(A)

C = new mat.Dense()
C.mul(A, B)

console.log(mat.format(B, {format:"B=%.f", prefix:"  "}))
console.log(mat.format(C, {format:"C=%.f", prefix:"  "}))

//B=⎡-2  1⎤
//  ⎣ 1 -0⎦
//C=⎡1 0⎤
//  ⎣0 1⎦
```

### solve()
### exp()
### pow()
### scale()

## VecDense

Vector

**Creation**

```js
new VecDense(n, data)
```

**Parameters**

- `n` `Number` Creates a new VecDense of length n. It should be larger than 0.
- `data` `Number[]` Array of elements. If data is omit, blank array is assigned.

### cap()
### len()
### atVec()
### setVec()
### addVec()
### subVec()
### mulVec()
### mulElemVec()
### scaleVec()
### solveVec()

## QR

**QR factorization** is a decomposition of a matrix *A* into a product `A = QR` of an orthonormal matrix *Q* and a upper triangular matrix *R*.
QR decomposition is often used to solve the linear least squares (LLS) problem and is the basis for a particular eigenvalue algorithm, the QR algorithm.

Any real square matrix *A* may be decomposed as

```
A = QR
```

where *Q* is an orthogonal matrix and *R* is an upper triangular matrix.
If *A* is invertible, then the factorization is unique if we require the diagonal elements of *R* to be positive.

**Usage example**

```js
const m = require("@jsh/mat")
A = new m.Dense(4, 2, [
    0, 1,
    1, 1,
    1, 1,
    2, 1,
])

qr = new m.QR()
qr.factorize(A)

Q = new m.Dense()
qr.QTo(Q)

R = new m.Dense()
qr.RTo(R)

B = new m.Dense(4, 1, [1, 0, 2, 1])
x = new m.Dense()
qr.solveTo(x, false, B)
console.log(m.format(x, { format: "x = %.2f", prefix: "    " }))

// x = ⎡0.00⎤
//     ⎣1.00⎦
```

## format()

```js
const m = require("@jsh/mat")
A = new m.Dense(100, 100)
for (let i = 0; i < 100; i++) {
    for (let j = 0; j < 100; j++) {
        A.set(i, j, i + j)
    }
}
console.log(m.format(A, {
    format: "A = %v",
    prefix: "    ",
    squeeze: true,
    excerpt: 3,
}))

// A = Dims(100, 100)
//     ⎡ 0    1    2  ...  ...   97   98   99⎤
//     ⎢ 1    2    3             98   99  100⎥
//     ⎢ 2    3    4             99  100  101⎥
//      .
//      .
//      .
//     ⎢97   98   99            194  195  196⎥
//     ⎢98   99  100            195  196  197⎥
//     ⎣99  100  101  ...  ...  196  197  198⎦
```