package mat

import (
	"context"
	"fmt"

	js "github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/require"
	"gonum.org/v1/gonum/mat"
)

func NewModuleLoader(context.Context) require.ModuleLoader {
	return func(rt *js.Runtime, module *js.Object) {
		// m = require("@jsh/mat")
		o := module.Get("exports").(*js.Object)
		// format("%v", m, opts...)
		o.Set("format", Format(rt))
		// new Dense(rows, cols, []float64)
		o.Set("Dense", new_dense(rt))
		// new SymDense(dims, []float64)
		o.Set("SymDense", new_symDense(rt))
		// new QR()
		o.Set("QR", new_qr(rt))
		// new VecDense(n, []float64)
		o.Set("VecDense", new_vecDense(rt))
	}
}

func Format(rt *js.Runtime) func(call js.FunctionCall) js.Value {
	return func(call js.FunctionCall) js.Value {
		if len(call.Arguments) == 0 {
			return js.Undefined()
		}
		v, ok := call.Arguments[0].(*js.Object).Get("$").Export().(mat.Matrix)
		if !ok {
			return js.Undefined()
		}

		opts := struct {
			Format  string `json:"format"`
			Prefix  string `json:"prefix,omitempty"`
			Excerpt int    `json:"excerpt,omitempty"`
			Sqeeze  bool   `json:"squeeze,omitempty"`
		}{
			Format: "",
		}

		if len(call.Arguments) > 1 {
			if err := rt.ExportTo(call.Arguments[1], &opts); err != nil {
				panic(rt.ToValue(fmt.Sprintf("format: %v", err)))
			}
		}

		o := []mat.FormatOption{}
		if opts.Prefix != "" {
			o = append(o, mat.Prefix(opts.Prefix))
		}
		if opts.Excerpt > 0 {
			o = append(o, mat.Excerpt(opts.Excerpt))
		}
		if opts.Sqeeze {
			o = append(o, mat.Squeeze())
		}
		f := mat.Formatted(v, o...)
		return rt.ToValue(fmt.Sprintf(opts.Format, f))
	}
}
