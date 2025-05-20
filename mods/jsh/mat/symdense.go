package mat

import (
	"fmt"

	js "github.com/dop251/goja"
	"gonum.org/v1/gonum/mat"
)

func new_symDense(rt *js.Runtime) func(c js.ConstructorCall) *js.Object {
	return func(c js.ConstructorCall) *js.Object {
		if len(c.Arguments) == 0 {
			m := &SymDense{value: &mat.SymDense{}, rt: rt}
			return m.toValue()
		}
		var n int
		var data []float64
		if len(c.Arguments) > 0 {
			n = int(c.Arguments[0].ToInteger())
		}
		if len(c.Arguments) > 1 {
			if err := rt.ExportTo(c.Arguments[1], &data); err != nil {
				panic(rt.ToValue(fmt.Sprintf("SymDense: %v", err)))
			}
		}
		m := &SymDense{value: mat.NewSymDense(n, data), rt: rt}
		return m.toValue()
	}
}

type SymDense struct {
	value *mat.SymDense
	rt    *js.Runtime
}

func (d *SymDense) toValue() *js.Object {
	obj := d.rt.NewObject()
	obj.Set("dims", d.Dims)
	obj.Set("set", d.Set)
	obj.Set("subset", d.Subset)
	obj.Set("add", d.Add)
	// obj.Set("mul", d.Mul)
	// obj.Set("mulElem", d.MulElem)
	// obj.Set("divElem", d.DivElem)
	// obj.Set("inverse", d.Inverse)
	// obj.Set("solve", d.Solve)
	// obj.Set("exp", d.Exp)
	// obj.Set("pow", d.Pow)
	// obj.Set("scale", d.Scale)
	obj.Set("$", d.value)
	return obj
}

func (d *SymDense) Dims(call js.FunctionCall) js.Value {
	r, c := d.value.Dims()
	ret := d.rt.NewObject()
	ret.Set("rows", r)
	ret.Set("cols", c)
	return ret
}

// Set sets the elements at (i,j) and (j,i) to the value v.
func (d *SymDense) Set(call js.FunctionCall) js.Value {
	if len(call.Arguments) < 3 {
		return d.rt.ToValue("set: not enough arguments")
	}
	i := int(call.Arguments[0].ToInteger())
	j := int(call.Arguments[1].ToInteger())
	v := call.Arguments[2].ToFloat()
	d.value.SetSym(i, j, v)
	return js.Undefined()
}

func (d *SymDense) Add(call js.FunctionCall) js.Value {
	if len(call.Arguments) < 2 {
		return d.rt.ToValue("add: not enough arguments")
	}
	a, ok := call.Arguments[0].(*js.Object).Get("$").Export().(*mat.SymDense)
	if !ok {
		return d.rt.ToValue("add: not a SymDense matrix")
	}
	b, ok := call.Arguments[1].(*js.Object).Get("$").Export().(*mat.SymDense)
	if !ok {
		return d.rt.ToValue("add: not a SymDense matrix")
	}
	d.value.AddSym(a, b)
	return js.Undefined()
}

func (d *SymDense) Subset(call js.FunctionCall) js.Value {
	if len(call.Arguments) < 2 {
		return d.rt.ToValue("sub: not enough arguments")
	}
	a, ok := call.Arguments[0].(*js.Object).Get("$").Export().(*mat.SymDense)
	if !ok {
		return d.rt.ToValue("sub: not a SymDense matrix")
	}
	n := []int{}
	if err := d.rt.ExportTo(call.Arguments[1], &n); err != nil {
		return d.rt.ToValue(fmt.Sprintf("sub: %v", err))
	}
	d.value.SubsetSym(a, n)
	return js.Undefined()
}
