package mat

import (
	"context"
	_ "embed"
	"fmt"

	"github.com/dop251/goja"
	"gonum.org/v1/gonum/mat"
)

//go:embed matrix.js
var matrix_js []byte

func Files() map[string][]byte {
	return map[string][]byte{
		"mathx/mat.js": matrix_js,
	}
}

func Module(_ context.Context, rt *goja.Runtime, module *goja.Object) {
	// m = require("@jsh/mathx/mat")
	o := module.Get("exports").(*goja.Object)
	// format("%v", m, opts...)
	o.Set("format", Format)
	// new Dense(rows, cols, []float64)
	o.Set("Dense", new_dense)
	// new SymDense(dims, []float64)
	o.Set("SymDense", new_symDense)
	// new QR()
	o.Set("QR", new_qr)
	// new VecDense(n, []float64)
	o.Set("VecDense", new_vecDense)
}

func new_dense(rows, cols int, data []float64) *mat.Dense {
	if rows <= 0 || cols <= 0 {
		return &mat.Dense{}
	}
	return mat.NewDense(rows, cols, data)
}

func new_symDense(n int, data []float64) *mat.SymDense {
	if n <= 0 {
		return &mat.SymDense{}
	}
	return mat.NewSymDense(n, data)
}

func new_vecDense(n int, data []float64) *mat.VecDense {
	if n <= 0 {
		return &mat.VecDense{}
	}
	return mat.NewVecDense(n, data)
}

func new_qr() *mat.QR {
	return &mat.QR{}
}

type FormatOption struct {
	Format  string `json:"format"`
	Prefix  string `json:"prefix,omitempty"`
	Excerpt int    `json:"excerpt,omitempty"`
	Squeeze bool   `json:"squeeze,omitempty"`
}

func Format(v mat.Matrix, opts FormatOption) string {
	if opts.Format == "" {
		opts.Format = "%v"
	}

	o := []mat.FormatOption{}
	if opts.Prefix != "" {
		o = append(o, mat.Prefix(opts.Prefix))
	}
	if opts.Excerpt > 0 {
		o = append(o, mat.Excerpt(opts.Excerpt))
	}
	if opts.Squeeze {
		o = append(o, mat.Squeeze())
	}
	f := mat.Formatted(v, o...)
	return fmt.Sprintf(opts.Format, f)
}
