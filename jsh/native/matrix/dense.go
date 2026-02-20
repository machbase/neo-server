package matrix

import (
	"fmt"

	"github.com/dop251/goja"
	"gonum.org/v1/gonum/mat"
)

func new_dense(rt *goja.Runtime) func(goja.ConstructorCall) *goja.Object {
	return func(call goja.ConstructorCall) *goja.Object {
		if len(call.Arguments) == 0 {
			m := &Dense{Matrix: Matrix{value: &mat.Dense{}, rt: rt}}
			return m.toValue()
		}
		var rows, cols int
		var data []float64
		if len(call.Arguments) > 0 {
			rows = int(call.Arguments[0].ToInteger())
		}
		if len(call.Arguments) > 1 {
			cols = int(call.Arguments[1].ToInteger())
		}
		if len(call.Arguments) > 2 {
			if err := rt.ExportTo(call.Arguments[2], &data); err != nil {
				panic(rt.ToValue(fmt.Sprintf("Dense: %v", err)))
			}
		}
		m := &Dense{Matrix: Matrix{value: mat.NewDense(rows, cols, data), rt: rt}}
		return m.toValue()
	}
}

type Dense struct {
	Matrix
}

func (d *Dense) toValue() *goja.Object {
	obj := d.Matrix.toValue()
	obj.Set("reuseAs", d.ReuseAs)
	obj.Set("zero", d.Zero)
	obj.Set("reset", d.Reset)
	obj.Set("isEmpty", d.IsEmpty)
	obj.Set("cloneFrom", d.CloneFrom)
	obj.Set("caps", d.Caps)
	obj.Set("set", d.Set)
	obj.Set("add", d.Add)
	obj.Set("sub", d.Sub)
	obj.Set("mul", d.Mul)
	obj.Set("mulElem", d.MulElem)
	obj.Set("divElem", d.DivElem)
	obj.Set("inverse", d.Inverse)
	obj.Set("solve", d.Solve)
	obj.Set("exp", d.Exp)
	obj.Set("pow", d.Pow)
	obj.Set("scale", d.Scale)
	return obj
}

func (d *Dense) ReuseAs(call goja.FunctionCall) goja.Value {
	if len(call.Arguments) < 2 {
		return d.rt.ToValue("reuseAs: not enough arguments")
	}
	rows := int(call.Arguments[0].ToInteger())
	cols := int(call.Arguments[1].ToInteger())
	if rows < 0 || cols < 0 {
		return d.rt.ToValue("reuseAs: negative size")
	}
	dense := d.value.(*mat.Dense)
	if dense == nil {
		return d.rt.ToValue("reuseAs: nil matrix")
	}
	dense.ReuseAs(rows, cols)
	return goja.Undefined()
}

func (d *Dense) Zero(call goja.FunctionCall) goja.Value {
	dense := d.value.(*mat.Dense)
	if dense == nil {
		return d.rt.ToValue("zero: nil matrix")
	}
	dense.Zero()
	return goja.Undefined()
}

func (d *Dense) Reset(call goja.FunctionCall) goja.Value {
	dense := d.value.(*mat.Dense)
	if dense == nil {
		return d.rt.ToValue("reset: nil matrix")
	}
	dense.Reset()
	return goja.Undefined()
}

func (d *Dense) IsEmpty(call goja.FunctionCall) goja.Value {
	dense := d.value.(*mat.Dense)
	if dense == nil {
		return d.rt.ToValue("isEmpty: nil matrix")
	}
	ret := dense.IsEmpty()
	return d.rt.ToValue(ret)
}

func (d *Dense) CloneFrom(call goja.FunctionCall) goja.Value {
	if len(call.Arguments) == 0 {
		return d.rt.ToValue("denseCopyOf: not enough arguments")
	}
	a, ok := call.Arguments[0].(*goja.Object).Get("$").Export().(mat.Matrix)
	if !ok {
		return d.rt.ToValue("denseCopyOf: not a Dense matrix")
	}
	dense := d.value.(*mat.Dense)
	if dense == nil {
		return d.rt.ToValue("denseCopyOf: nil matrix")
	}
	dense.CloneFrom(a)
	return goja.Undefined()
}

func (d *Dense) Caps(call goja.FunctionCall) goja.Value {
	dense := d.value.(*mat.Dense)
	if dense == nil {
		return d.rt.ToValue("caps: nil matrix")
	}
	r, c := dense.Caps()
	ret := d.rt.NewObject()
	ret.Set("rows", r)
	ret.Set("cols", c)
	return ret
}

func (d *Dense) Set(call goja.FunctionCall) goja.Value {
	if len(call.Arguments) < 3 {
		return d.rt.ToValue("set: not enough arguments")
	}
	row := int(call.Arguments[0].ToInteger())
	col := int(call.Arguments[1].ToInteger())
	val := call.Arguments[2].ToFloat()
	if row < 0 || col < 0 {
		return d.rt.ToValue("set: negative index")
	}
	dense := d.value.(*mat.Dense)
	r, c := dense.Dims()
	if uint(row) >= uint(r) || uint(col) >= uint(c) {
		return d.rt.ToValue("set: index out of range")
	}
	dense.Set(row, col, val)
	return goja.Undefined()
}

func (m *Dense) Add(call goja.FunctionCall) goja.Value {
	a, ok := call.Arguments[0].(*goja.Object).Get("$").Export().(mat.Matrix)
	if !ok {
		return m.rt.ToValue("add: not a Dense matrix")
	}
	b, ok := call.Arguments[1].(*goja.Object).Get("$").Export().(mat.Matrix)
	if !ok {
		return m.rt.ToValue("add: not a Dense matrix")
	}
	dense := m.value.(*mat.Dense)
	dense.Add(a, b)
	return goja.Undefined()
}

func (m *Dense) Sub(call goja.FunctionCall) goja.Value {
	a, ok := call.Arguments[0].(*goja.Object).Get("$").Export().(mat.Matrix)
	if !ok {
		return m.rt.ToValue("sub: not a Dense matrix")
	}
	b, ok := call.Arguments[1].(*goja.Object).Get("$").Export().(mat.Matrix)
	if !ok {
		return m.rt.ToValue("sub: not a Dense matrix")
	}
	dense := m.value.(*mat.Dense)
	dense.Sub(a, b)
	return goja.Undefined()
}

func (m *Dense) Mul(call goja.FunctionCall) goja.Value {
	a, ok := call.Arguments[0].(*goja.Object).Get("$").Export().(mat.Matrix)
	if !ok {
		return m.rt.ToValue("mul: not a Dense matrix")
	}
	b := call.Arguments[1].(*goja.Object).Get("$").Export().(mat.Matrix)
	if !ok {
		return m.rt.ToValue("mul: not a Dense matrix")
	}
	dense := m.value.(*mat.Dense)
	dense.Mul(a, b)
	return goja.Undefined()
}

func (m *Dense) MulElem(call goja.FunctionCall) goja.Value {
	a, ok := call.Arguments[0].(*goja.Object).Get("$").Export().(mat.Matrix)
	if !ok {
		return m.rt.ToValue("mulElem: not a Dense matrix")
	}
	b, ok := call.Arguments[1].(*goja.Object).Get("$").Export().(mat.Matrix)
	if !ok {
		return m.rt.ToValue("mulElem: not a Dense matrix")
	}
	dense := m.value.(*mat.Dense)
	dense.MulElem(a, b)
	return goja.Undefined()
}

func (m *Dense) DivElem(call goja.FunctionCall) goja.Value {
	a, ok := call.Arguments[0].(*goja.Object).Get("$").Export().(mat.Matrix)
	if !ok {
		return m.rt.ToValue("divElem: not a Dense matrix")
	}
	b, ok := call.Arguments[1].(*goja.Object).Get("$").Export().(mat.Matrix)
	if !ok {
		return m.rt.ToValue("divElem: not a Dense matrix")
	}
	dense := m.value.(*mat.Dense)
	dense.DivElem(a, b)
	return goja.Undefined()
}

func (m *Dense) Scale(call goja.FunctionCall) goja.Value {
	a := call.Arguments[0].ToFloat()
	b, ok := call.Arguments[1].(*goja.Object).Get("$").Export().(mat.Matrix)
	if !ok {
		return m.rt.ToValue("scale: not a Dense matrix")
	}
	dense := m.value.(*mat.Dense)
	dense.Scale(a, b)
	return goja.Undefined()
}

func (m *Dense) Inverse(call goja.FunctionCall) goja.Value {
	a := call.Arguments[0].(*goja.Object).Get("$").Export().(mat.Matrix)
	dense := m.value.(*mat.Dense)
	err := dense.Inverse(a)
	if err != nil {
		return m.rt.ToValue(fmt.Sprintf("inverse: %v", err))
	}
	return goja.Undefined()
}

// The Inverse operation, however, should typically be avoided.
// If the goal is to solve a linear system
//
//	A * X = B,
//
// then the inverse is not needed and computing the solution as
// X = A^{-1} * B is slower and has worse stability properties than
// solving the original problem. In this case, the SolveVec method of
// VecDense (if B is a vector) or Solve method of Dense (if B is a
// matrix) should be used instead of computing the Inverse of A.
func (m *Dense) Solve(call goja.FunctionCall) goja.Value {
	a, ok := call.Arguments[0].(*goja.Object).Get("$").Export().(mat.Matrix)
	if !ok {
		return m.rt.ToValue("solve: not a Dense matrix")
	}
	b, ok := call.Arguments[1].(*goja.Object).Get("$").Export().(mat.Matrix)
	if !ok {
		return m.rt.ToValue("solve: not a Dense matrix")
	}
	dense := m.value.(*mat.Dense)
	err := dense.Solve(a, b)
	if err != nil {
		return m.rt.ToValue(fmt.Sprintf("solve: %v", err))
	}
	return goja.Undefined()
}

func (m *Dense) Exp(call goja.FunctionCall) goja.Value {
	a, ok := call.Arguments[0].(*goja.Object).Get("$").Export().(mat.Matrix)
	if !ok {
		return m.rt.ToValue("exp: not a Dense matrix")
	}
	dense := m.value.(*mat.Dense)
	dense.Exp(a)
	return goja.Undefined()
}

func (m *Dense) Pow(call goja.FunctionCall) goja.Value {
	a, ok := call.Arguments[0].(*goja.Object).Get("$").Export().(mat.Matrix)
	if !ok {
		return m.rt.ToValue("pow: not a Dense matrix")
	}
	b := call.Arguments[1].ToInteger()
	if b < 0 {
		return m.rt.ToValue("pow: negative exponent")
	}
	dense := m.value.(*mat.Dense)
	dense.Pow(a, int(b))
	return goja.Undefined()
}
