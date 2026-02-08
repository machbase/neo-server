package generator

import (
	"fmt"
	"math"
	"math/rand/v2"
	"slices"

	"github.com/dop251/goja"
	"github.com/gofrs/uuid/v5"
	"github.com/machbase/neo-server/v8/mods/nums"
	"github.com/machbase/neo-server/v8/mods/nums/opensimplex"
)

func Module(rt *goja.Runtime, module *goja.Object) {
	o := module.Get("exports").(*goja.Object)
	// m.random()
	o.Set("random", func() goja.Value {
		return rt.ToValue(rand.Float64())
	})
	// gen = new Simplex(seed)
	// gen.eval(x, y, z, ...)
	o.Set("Simplex", new_simplex(rt))

	// gen = new UUID(version)
	// gen.eval()
	o.Set("UUID", new_uuid(rt))

	// gen = new Oscillator()
	// gen.eval(time)

	// m.arrange(begin, end, step) => returns []float64
	o.Set("arrange", func(start, stop, step float64) goja.Value {
		if step == 0 {
			return rt.NewGoError(fmt.Errorf("arrange: step must not be 0"))
		}
		if start == stop {
			return rt.NewGoError(fmt.Errorf("arrange: start and stop must not be equal"))
		}
		if start < stop && step < 0 {
			return rt.NewGoError(fmt.Errorf("arrange: step must be positive"))
		}
		if start > stop && step > 0 {
			return rt.NewGoError(fmt.Errorf("arrange: step must be negative"))
		}
		length := int(math.Abs((stop-start)/step)) + 1
		arr := make([]float64, length)
		for i := 0; i < length; i++ {
			arr[i] = start + float64(i)*step
		}
		return rt.ToValue(arr)
	})
	// m.linspace(begin, end, count) => returns []float64
	o.Set("linspace", func(start, stop float64, count int) goja.Value {
		return rt.ToValue(nums.Linspace(start, stop, count))
	})
	// m.meshgrid(arr1, arr2) => returns [][]float64
	o.Set("meshgrid", func(arr1, arr2 []float64) goja.Value {
		len_x := len(arr1)
		len_y := len(arr2)
		arr := make([][]float64, len_x*len_y)
		for x, v1 := range arr1 {
			for y, v2 := range arr2 {
				arr[x*len_y+y] = []float64{v1, v2}
			}
		}
		return rt.ToValue(arr)
	})
}

func new_simplex(rt *goja.Runtime) func(call goja.ConstructorCall) *goja.Object {
	return func(call goja.ConstructorCall) *goja.Object {
		seed := int64(0)
		if len(call.Arguments) > 0 {
			if err := rt.ExportTo(call.Arguments[0], &seed); err != nil {
				panic(rt.ToValue("simplex: invalid argument"))
			}
		}
		gen := opensimplex.New(seed)
		ret := rt.NewObject()
		ret.Set("eval", func(call goja.FunctionCall) goja.Value {
			if len(call.Arguments) == 0 {
				panic(rt.ToValue("simplex: no argument"))
			}
			dim := make([]float64, len(call.Arguments))
			for i, arg := range call.Arguments {
				if err := rt.ExportTo(arg, &dim[i]); err != nil {
					panic(rt.ToValue("simplex: invalid argument"))
				}
			}
			return rt.ToValue(gen.Eval(dim...))
		})
		return ret
	}
}

func new_uuid(rt *goja.Runtime) func(call goja.ConstructorCall) *goja.Object {
	return func(call goja.ConstructorCall) *goja.Object {
		version := 1
		if len(call.Arguments) > 0 {
			if err := rt.ExportTo(call.Arguments[0], &version); err != nil {
				panic(rt.ToValue("uuid: invalid argument"))
			}
		}
		if !slices.Contains([]int{1, 4, 6, 7}, version) {
			panic(rt.ToValue(fmt.Sprintf("uuid: unsupported version %d", version)))
		}

		var gen uuid.Generator
		ret := rt.NewObject()
		ret.Set("eval", func(call goja.FunctionCall) goja.Value {
			if gen == nil {
				gen = uuid.NewGen()
			}
			var uid uuid.UUID
			switch version {
			case 1:
				uid, _ = gen.NewV1()
			case 4:
				uid, _ = gen.NewV4()
			case 6:
				uid, _ = gen.NewV6()
			case 7:
				uid, _ = gen.NewV7()
			}
			return rt.ToValue(uid.String())
		})
		return ret
	}
}
