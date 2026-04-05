package uuid

import (
	"context"
	_ "embed"

	"github.com/dop251/goja"
	"github.com/gofrs/uuid/v5"
)

//go:embed uuid.js
var uuid_js []byte

func Files() map[string][]byte {
	return map[string][]byte{
		"uuid.js": uuid_js,
	}
}

func Module(_ context.Context, rt *goja.Runtime, module *goja.Object) {
	o := module.Get("exports").(*goja.Object)
	o.Set("generator", new_uuid)
}

func new_uuid() uuid.Generator {
	return uuid.NewGen()

	// return func(call goja.ConstructorCall) *goja.Object {
	// 	version := 1
	// 	if len(call.Arguments) > 0 {
	// 		if err := rt.ExportTo(call.Arguments[0], &version); err != nil {
	// 			panic(rt.ToValue("uuid: invalid argument"))
	// 		}
	// 	}
	// 	if !slices.Contains([]int{1, 4, 6, 7}, version) {
	// 		panic(rt.ToValue(fmt.Sprintf("uuid: unsupported version %d", version)))
	// 	}

	// 	var gen uuid.Generator
	// 	ret := rt.NewObject()
	// 	ret.Set("eval", func(call goja.FunctionCall) goja.Value {
	// 		if gen == nil {
	// 		}
	// 		var uid uuid.UUID
	// 		switch version {
	// 		case 1:
	// 			uid, _ = gen.NewV1()
	// 		case 4:
	// 			uid, _ = gen.NewV4()
	// 		case 6:
	// 			uid, _ = gen.NewV6()
	// 		case 7:
	// 			uid, _ = gen.NewV7()
	// 		}
	// 		return rt.ToValue(uid.String())
	// 	})
	// 	return ret
	// }
}
