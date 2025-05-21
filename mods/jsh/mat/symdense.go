package mat

import (
	"fmt"

	js "github.com/dop251/goja"
	"gonum.org/v1/gonum/mat"
)

func new_symDense(rt *js.Runtime) func(c js.ConstructorCall) *js.Object {
	return func(c js.ConstructorCall) *js.Object {
		if len(c.Arguments) == 0 {
			m := &SymDense{Dense{Matrix{value: &mat.SymDense{}, rt: rt}}}
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
		m := &SymDense{Dense{Matrix{value: mat.NewSymDense(n, data), rt: rt}}}
		return m.toValue()
	}
}

type SymDense struct {
	Dense
}

func (d *SymDense) toValue() *js.Object {
	obj := d.Dense.toValue()
	obj.Set("setSym", d.SetSym)
	obj.Set("subsetSym", d.SubsetSym)
	obj.Set("addSym", d.AddSym)
	return obj
}

// Set sets the elements at (i,j) and (j,i) to the value v.
func (d *SymDense) SetSym(call js.FunctionCall) js.Value {
	if len(call.Arguments) < 3 {
		return d.rt.ToValue("setSym: not enough arguments")
	}
	i := int(call.Arguments[0].ToInteger())
	j := int(call.Arguments[1].ToInteger())
	v := call.Arguments[2].ToFloat()
	symdense := d.value.(*mat.SymDense)
	symdense.SetSym(i, j, v)
	return js.Undefined()
}

func (d *SymDense) AddSym(call js.FunctionCall) js.Value {
	if len(call.Arguments) < 2 {
		return d.rt.ToValue("addSym: not enough arguments")
	}
	a, ok := call.Arguments[0].(*js.Object).Get("$").Export().(*mat.SymDense)
	if !ok {
		return d.rt.ToValue("addSym: not a SymDense matrix")
	}
	b, ok := call.Arguments[1].(*js.Object).Get("$").Export().(*mat.SymDense)
	if !ok {
		return d.rt.ToValue("addSym: not a SymDense matrix")
	}
	symdense := d.value.(*mat.SymDense)
	symdense.AddSym(a, b)
	return js.Undefined()
}

func (d *SymDense) SubsetSym(call js.FunctionCall) js.Value {
	if len(call.Arguments) < 2 {
		return d.rt.ToValue("subsetSym: not enough arguments")
	}
	a, ok := call.Arguments[0].(*js.Object).Get("$").Export().(*mat.SymDense)
	if !ok {
		return d.rt.ToValue("subsetSym: not a SymDense matrix")
	}
	n := []int{}
	if err := d.rt.ExportTo(call.Arguments[1], &n); err != nil {
		return d.rt.ToValue(fmt.Sprintf("subsetSym: %v", err))
	}
	symdense := d.value.(*mat.SymDense)
	symdense.SubsetSym(a, n)
	return js.Undefined()
}
