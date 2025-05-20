package mat

import (
	"fmt"

	js "github.com/dop251/goja"
	"gonum.org/v1/gonum/mat"
)

func new_vecDense(rt *js.Runtime) func(c js.ConstructorCall) *js.Object {
	return func(call js.ConstructorCall) *js.Object {
		defer func() {
			if r := recover(); r != nil {
				panic(rt.ToValue(fmt.Sprintf("VecDense: %v", r)))
			}
		}()
		if len(call.Arguments) == 0 {
			m := &VecDense{value: &mat.VecDense{}, rt: rt}
			return m.toValue()
		}
		var ncol int
		var data []float64
		if len(call.Arguments) > 0 {
			ncol = int(call.Arguments[0].ToInteger())
		}
		if len(call.Arguments) > 1 {
			if err := rt.ExportTo(call.Arguments[1], &data); err != nil {
				panic(rt.ToValue(fmt.Sprintf("VecDense: %v", err)))
			}
		}
		m := &VecDense{value: mat.NewVecDense(ncol, data), rt: rt}
		return m.toValue()
	}
}

type VecDense struct {
	value *mat.VecDense
	rt    *js.Runtime
}

func (m *VecDense) toValue() *js.Object {
	obj := m.rt.NewObject()
	obj.Set("dims", m.Dims)
	obj.Set("cap", m.Cap)
	obj.Set("set", m.Value)
	obj.Set("add", m.Add)
	obj.Set("sub", m.Sub)
	obj.Set("mul", m.Mul)
	obj.Set("mulElem", m.MulElem)
	obj.Set("solveVec", m.Solve)
	obj.Set("scale", m.Scale)
	obj.Set("$", m.value)
	return obj
}

func (vec *VecDense) Dims(call js.FunctionCall) js.Value {
	r, c := vec.value.Dims()
	ret := vec.rt.NewObject()
	ret.Set("rows", r)
	ret.Set("cols", c)
	return ret
}

func (vec *VecDense) Cap(call js.FunctionCall) js.Value {
	cap := vec.value.Cap()
	ret := vec.rt.NewObject()
	ret.Set("rows", cap)
	ret.Set("cols", 1)
	return ret
}

func (vec *VecDense) Value(call js.FunctionCall) js.Value {
	if len(call.Arguments) < 2 {
		return vec.rt.ToValue("set: not enough arguments")
	}
	row := int(call.Arguments[0].ToInteger())
	val := call.Arguments[1].ToFloat()
	if row < 0 || row >= vec.value.Len() {
		return vec.rt.ToValue("set: out of range")
	}
	vec.value.SetVec(row, val)
	return js.Undefined()
}

func (m *VecDense) Add(call js.FunctionCall) js.Value {
	a := call.Arguments[0].(*js.Object).Get("$").Export().(*mat.VecDense)
	b := call.Arguments[1].(*js.Object).Get("$").Export().(*mat.VecDense)
	m.value.AddVec(a, b)
	return js.Undefined()
}

func (m *VecDense) Sub(call js.FunctionCall) js.Value {
	a := call.Arguments[0].(*js.Object).Get("$").Export().(*mat.VecDense)
	b := call.Arguments[1].(*js.Object).Get("$").Export().(*mat.VecDense)
	m.value.SubVec(a, b)
	return js.Undefined()
}

func (m *VecDense) Mul(call js.FunctionCall) js.Value {
	a := call.Arguments[0].(*js.Object).Get("$").Export().(*mat.Dense)
	b := call.Arguments[1].(*js.Object).Get("$").Export().(*mat.VecDense)
	m.value.MulVec(a, b)
	return js.Undefined()
}

func (m *VecDense) MulElem(call js.FunctionCall) js.Value {
	a := call.Arguments[0].(*js.Object).Get("$").Export().(*mat.VecDense)
	b := call.Arguments[1].(*js.Object).Get("$").Export().(*mat.VecDense)
	m.value.MulElemVec(a, b)
	return js.Undefined()
}

func (m *VecDense) Scale(call js.FunctionCall) js.Value {
	alpha := call.Arguments[0].ToFloat()
	b := call.Arguments[1].(*js.Object).Get("$").Export().(*mat.VecDense)
	m.value.ScaleVec(alpha, b)
	return js.Undefined()
}

func (m *VecDense) Solve(call js.FunctionCall) js.Value {
	a := call.Arguments[0].(*js.Object).Get("$").Export().(*mat.Dense)
	b := call.Arguments[1].(*js.Object).Get("$").Export().(*mat.VecDense)
	m.value.SolveVec(a, b)
	return js.Undefined()
}
