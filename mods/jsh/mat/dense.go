package mat

import (
	"fmt"

	js "github.com/dop251/goja"
	"gonum.org/v1/gonum/mat"
)

func new_dense(rt *js.Runtime) func(js.ConstructorCall) *js.Object {
	return func(call js.ConstructorCall) *js.Object {
		if len(call.Arguments) == 0 {
			m := &Dense{value: &mat.Dense{}, rt: rt}
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
		m := &Dense{value: mat.NewDense(rows, cols, data), rt: rt}
		return m.toValue()
	}
}

type Dense struct {
	value *mat.Dense
	rt    *js.Runtime
}

func (d *Dense) toValue() *js.Object {
	obj := d.rt.NewObject()
	obj.Set("dims", d.Dims)
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
	obj.Set("$", d.value)
	return obj
}

func (d *Dense) Dims(call js.FunctionCall) js.Value {
	r, c := d.value.Dims()
	ret := d.rt.NewObject()
	ret.Set("rows", r)
	ret.Set("cols", c)
	return ret
}

func (d *Dense) Set(call js.FunctionCall) js.Value {
	if len(call.Arguments) < 3 {
		return d.rt.ToValue("set: not enough arguments")
	}
	row := int(call.Arguments[0].ToInteger())
	col := int(call.Arguments[1].ToInteger())
	val := call.Arguments[2].ToFloat()
	if row < 0 || col < 0 {
		return d.rt.ToValue("set: negative index")
	}
	if row >= d.value.RawMatrix().Rows || col >= d.value.RawMatrix().Cols {
		return d.rt.ToValue("set: index out of range")
	}
	d.value.Set(row, col, val)
	return js.Undefined()
}

func (m *Dense) Add(call js.FunctionCall) js.Value {
	a, ok := call.Arguments[0].(*js.Object).Get("$").Export().(*mat.Dense)
	if !ok {
		return m.rt.ToValue("add: not a Dense matrix")
	}
	b, ok := call.Arguments[1].(*js.Object).Get("$").Export().(*mat.Dense)
	if !ok {
		return m.rt.ToValue("add: not a Dense matrix")
	}
	m.value.Add(a, b)
	return js.Undefined()
}

func (m *Dense) Sub(call js.FunctionCall) js.Value {
	a, ok := call.Arguments[0].(*js.Object).Get("$").Export().(*mat.Dense)
	if !ok {
		return m.rt.ToValue("sub: not a Dense matrix")
	}
	b, ok := call.Arguments[1].(*js.Object).Get("$").Export().(*mat.Dense)
	if !ok {
		return m.rt.ToValue("sub: not a Dense matrix")
	}
	m.value.Sub(a, b)
	return js.Undefined()
}

func (m *Dense) Mul(call js.FunctionCall) js.Value {
	a, ok := call.Arguments[0].(*js.Object).Get("$").Export().(*mat.Dense)
	if !ok {
		return m.rt.ToValue("mul: not a Dense matrix")
	}
	b := call.Arguments[1].(*js.Object).Get("$").Export().(*mat.Dense)
	if !ok {
		return m.rt.ToValue("mul: not a Dense matrix")
	}
	m.value.Mul(a, b)
	return js.Undefined()
}

func (m *Dense) MulElem(call js.FunctionCall) js.Value {
	a, ok := call.Arguments[0].(*js.Object).Get("$").Export().(*mat.Dense)
	if !ok {
		return m.rt.ToValue("mulElem: not a Dense matrix")
	}
	b, ok := call.Arguments[1].(*js.Object).Get("$").Export().(*mat.Dense)
	if !ok {
		return m.rt.ToValue("mulElem: not a Dense matrix")
	}
	m.value.MulElem(a, b)
	return js.Undefined()
}

func (m *Dense) DivElem(call js.FunctionCall) js.Value {
	a, ok := call.Arguments[0].(*js.Object).Get("$").Export().(*mat.Dense)
	if !ok {
		return m.rt.ToValue("divElem: not a Dense matrix")
	}
	b, ok := call.Arguments[1].(*js.Object).Get("$").Export().(*mat.Dense)
	if !ok {
		return m.rt.ToValue("divElem: not a Dense matrix")
	}
	m.value.DivElem(a, b)
	return js.Undefined()
}

func (m *Dense) Scale(call js.FunctionCall) js.Value {
	a := call.Arguments[0].ToFloat()
	b, ok := call.Arguments[1].(*js.Object).Get("$").Export().(*mat.Dense)
	if !ok {
		return m.rt.ToValue("scale: not a Dense matrix")
	}
	m.value.Scale(a, b)
	return js.Undefined()
}

func (m *Dense) Inverse(call js.FunctionCall) js.Value {
	a := call.Arguments[0].(*js.Object).Get("$").Export().(*mat.Dense)
	err := m.value.Inverse(a)
	if err != nil {
		return m.rt.ToValue(fmt.Sprintf("inverse: %v", err))
	}
	return js.Undefined()
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
func (m *Dense) Solve(call js.FunctionCall) js.Value {
	a, ok := call.Arguments[0].(*js.Object).Get("$").Export().(*mat.Dense)
	if !ok {
		return m.rt.ToValue("solve: not a Dense matrix")
	}
	b, ok := call.Arguments[1].(*js.Object).Get("$").Export().(*mat.Dense)
	if !ok {
		return m.rt.ToValue("solve: not a Dense matrix")
	}
	err := m.value.Solve(a, b)
	if err != nil {
		return m.rt.ToValue(fmt.Sprintf("solve: %v", err))
	}
	return js.Undefined()
}

func (m *Dense) Exp(call js.FunctionCall) js.Value {
	a, ok := call.Arguments[0].(*js.Object).Get("$").Export().(*mat.Dense)
	if !ok {
		return m.rt.ToValue("exp: not a Dense matrix")
	}
	m.value.Exp(a)
	return js.Undefined()
}

func (m *Dense) Pow(call js.FunctionCall) js.Value {
	a, ok := call.Arguments[0].(*js.Object).Get("$").Export().(*mat.Dense)
	if !ok {
		return m.rt.ToValue("pow: not a Dense matrix")
	}
	b := call.Arguments[1].ToInteger()
	if b < 0 {
		return m.rt.ToValue("pow: negative exponent")
	}
	m.value.Pow(a, int(b))
	return js.Undefined()
}
