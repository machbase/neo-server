package matrix

import (
	"github.com/dop251/goja"
	"gonum.org/v1/gonum/mat"
)

func new_qr(rt *goja.Runtime) func(goja.ConstructorCall) *goja.Object {
	return func(call goja.ConstructorCall) *goja.Object {
		ret := &QR{value: mat.QR{}, rt: rt}
		return ret.toValue()
	}
}

type QR struct {
	value mat.QR
	rt    *goja.Runtime
}

func (qr *QR) toValue() *goja.Object {
	obj := qr.rt.NewObject()
	obj.Set("factorize", qr.Factorize)
	obj.Set("QTo", qr.QTo)
	obj.Set("RTo", qr.RTo)
	obj.Set("solveTo", qr.SolveTo)
	return obj
}

// Factorize computes the QR factorization of an m×n matrix a where m >= n. The QR
// factorization always exists even if A is singular.
//
// The QR decomposition is a factorization of the matrix A such that A = Q * R.
// The matrix Q is an orthonormal m×m matrix, and R is an m×n upper triangular matrix.
// Q and R can be extracted using the QTo and RTo methods.
func (qr *QR) Factorize(call goja.FunctionCall) goja.Value {
	if len(call.Arguments) == 0 {
		return goja.Undefined()
	}
	a, ok := call.Arguments[0].(*goja.Object).Get("$").Export().(*mat.Dense)
	if !ok {
		return qr.rt.ToValue("add: not a Dense matrix")
	}
	qr.value.Factorize(a)
	return goja.Undefined()
}

func (qr *QR) QTo(call goja.FunctionCall) goja.Value {
	if len(call.Arguments) == 0 {
		return goja.Undefined()
	}
	a, ok := call.Arguments[0].(*goja.Object).Get("$").Export().(*mat.Dense)
	if !ok {
		return qr.rt.ToValue("add: not a Dense matrix")
	}
	qr.value.QTo(a)
	return goja.Undefined()
}

func (qr *QR) RTo(call goja.FunctionCall) goja.Value {
	if len(call.Arguments) == 0 {
		return goja.Undefined()
	}
	a, ok := call.Arguments[0].(*goja.Object).Get("$").Export().(*mat.Dense)
	if !ok {
		return qr.rt.ToValue("add: not a Dense matrix")
	}
	qr.value.RTo(a)
	return goja.Undefined()
}

func (qr *QR) SolveTo(call goja.FunctionCall) goja.Value {
	if len(call.Arguments) != 3 {
		return goja.Undefined()
	}
	a, ok := call.Arguments[0].(*goja.Object).Get("$").Export().(*mat.Dense)
	if !ok {
		return qr.rt.ToValue("add: not a Dense matrix")
	}
	var trans bool
	if err := qr.rt.ExportTo(call.Arguments[1], &trans); err != nil {
		return qr.rt.ToValue(qr.rt.ToValue(err))
	}
	b, ok := call.Arguments[2].(*goja.Object).Get("$").Export().(*mat.Dense)
	if !ok {
		return qr.rt.ToValue("add: not a Dense matrix")
	}
	err := qr.value.SolveTo(a, trans, b)
	if err != nil {
		return qr.rt.ToValue(qr.rt.ToValue(err))
	}
	return goja.Undefined()
}
