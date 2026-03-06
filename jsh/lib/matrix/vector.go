package matrix

import (
	"fmt"

	"github.com/dop251/goja"
	"gonum.org/v1/gonum/mat"
)

type Vector struct {
	Matrix
}

func (v *Vector) toValue() *goja.Object {
	obj := v.Matrix.toValue()
	obj.Set("atVec", v.AtVec)
	obj.Set("len", v.Len)
	return obj
}

func (v *Vector) AtVec(call goja.FunctionCall) goja.Value {
	if len(call.Arguments) == 0 {
		return goja.Undefined()
	}
	row := int(call.Arguments[0].ToInteger())
	vec := v.value.(mat.Vector)
	ret := vec.AtVec(row)
	return v.rt.ToValue(ret)
}

func (v *Vector) Len(call goja.FunctionCall) goja.Value {
	ret := v.value.(mat.Vector).Len()
	return v.rt.ToValue(ret)
}

func new_vecDense(rt *goja.Runtime) func(goja.ConstructorCall) *goja.Object {
	return func(call goja.ConstructorCall) *goja.Object {
		defer func() {
			if r := recover(); r != nil {
				panic(rt.ToValue(fmt.Sprintf("VecDense: %v", r)))
			}
		}()
		if len(call.Arguments) == 0 {
			m := &VecDense{Vector: Vector{Matrix{value: &mat.VecDense{}, rt: rt}}}
			return m.toValue()
		}
		var nCols int
		var data []float64
		if len(call.Arguments) > 0 {
			nCols = int(call.Arguments[0].ToInteger())
		}
		if len(call.Arguments) > 1 {
			if err := rt.ExportTo(call.Arguments[1], &data); err != nil {
				panic(rt.ToValue(fmt.Sprintf("VecDense: %v", err)))
			}
		}
		m := &VecDense{Vector: Vector{Matrix{value: mat.NewVecDense(nCols, data), rt: rt}}}
		return m.toValue()
	}
}

type VecDense struct {
	Vector
}

func (m *VecDense) toValue() *goja.Object {
	obj := m.Vector.toValue()
	obj.Set("cap", m.Cap)
	obj.Set("setVec", m.SetVec)
	obj.Set("addVec", m.AddVec)
	obj.Set("subVec", m.SubVec)
	obj.Set("mulVec", m.MulVec)
	obj.Set("mulElemVec", m.MulElemVec)
	obj.Set("solveVec", m.SolveVec)
	obj.Set("scaleVec", m.ScaleVec)
	return obj
}

func (vec *VecDense) Cap(call goja.FunctionCall) goja.Value {
	dense := vec.value.(*mat.VecDense)
	if dense == nil {
		return vec.rt.ToValue("cap: not a VecDense matrix")
	}
	cap := dense.Cap()
	return vec.rt.ToValue(cap)
}

func (vec *VecDense) SetVec(call goja.FunctionCall) goja.Value {
	if len(call.Arguments) < 2 {
		return vec.rt.ToValue("set: not enough arguments")
	}
	row := int(call.Arguments[0].ToInteger())
	val := call.Arguments[1].ToFloat()
	dense := vec.value.(*mat.VecDense)
	if dense == nil {
		return vec.rt.ToValue("cap: not a VecDense matrix")
	}
	if row < 0 || row >= dense.Len() {
		return vec.rt.ToValue("set: out of range")
	}
	dense.SetVec(row, val)
	return goja.Undefined()
}

func (vec *VecDense) AddVec(call goja.FunctionCall) goja.Value {
	a := call.Arguments[0].(*goja.Object).Get("$").Export().(*mat.VecDense)
	b := call.Arguments[1].(*goja.Object).Get("$").Export().(*mat.VecDense)
	dense := vec.value.(*mat.VecDense)
	if dense == nil {
		return vec.rt.ToValue("cap: not a VecDense matrix")
	}
	dense.AddVec(a, b)
	return goja.Undefined()
}

func (vec *VecDense) SubVec(call goja.FunctionCall) goja.Value {
	a := call.Arguments[0].(*goja.Object).Get("$").Export().(*mat.VecDense)
	b := call.Arguments[1].(*goja.Object).Get("$").Export().(*mat.VecDense)
	dense := vec.value.(*mat.VecDense)
	if dense == nil {
		return vec.rt.ToValue("cap: not a VecDense matrix")
	}
	dense.SubVec(a, b)
	return goja.Undefined()
}

func (vec *VecDense) MulVec(call goja.FunctionCall) goja.Value {
	a := call.Arguments[0].(*goja.Object).Get("$").Export().(*mat.Dense)
	b := call.Arguments[1].(*goja.Object).Get("$").Export().(*mat.VecDense)
	dense := vec.value.(*mat.VecDense)
	if dense == nil {
		return vec.rt.ToValue("cap: not a VecDense matrix")
	}
	dense.MulVec(a, b)
	return goja.Undefined()
}

func (vec *VecDense) MulElemVec(call goja.FunctionCall) goja.Value {
	a := call.Arguments[0].(*goja.Object).Get("$").Export().(*mat.VecDense)
	b := call.Arguments[1].(*goja.Object).Get("$").Export().(*mat.VecDense)
	dense := vec.value.(*mat.VecDense)
	if dense == nil {
		return vec.rt.ToValue("cap: not a VecDense matrix")
	}
	dense.MulElemVec(a, b)
	return goja.Undefined()
}

func (vec *VecDense) ScaleVec(call goja.FunctionCall) goja.Value {
	alpha := call.Arguments[0].ToFloat()
	b := call.Arguments[1].(*goja.Object).Get("$").Export().(*mat.VecDense)
	dense := vec.value.(*mat.VecDense)
	if dense == nil {
		return vec.rt.ToValue("cap: not a VecDense matrix")
	}
	dense.ScaleVec(alpha, b)
	return goja.Undefined()
}

func (vec *VecDense) SolveVec(call goja.FunctionCall) goja.Value {
	a := call.Arguments[0].(*goja.Object).Get("$").Export().(mat.Matrix)
	b := call.Arguments[1].(*goja.Object).Get("$").Export().(*mat.VecDense)
	dense := vec.value.(*mat.VecDense)
	if dense == nil {
		return vec.rt.ToValue("cap: not a VecDense matrix")
	}
	dense.SolveVec(a, b)
	return goja.Undefined()
}
