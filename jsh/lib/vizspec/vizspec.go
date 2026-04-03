package vizspec

import (
	"bytes"
	_ "embed"
	"encoding/json"

	"github.com/dop251/goja"
	model "github.com/machbase/neo-server/v8/jsh/advn"
)

//go:embed vizspec.js
var vizspecJS []byte

func Files() map[string][]byte {
	return map[string][]byte{
		"vizspec.js": vizspecJS,
	}
}

func Module(rt *goja.Runtime, module *goja.Object) {
	o := module.Get("exports").(*goja.Object)
	o.Set("parse", func(text string) goja.Value {
		spec, err := model.ParseString(text)
		if err != nil {
			panic(rt.NewGoError(err))
		}
		model.NormalizeSpecTimeValues(spec)
		return mustToValue(rt, spec)
	})
	o.Set("stringify", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) == 0 {
			panic(rt.NewTypeError("vizspec.stringify: spec is required"))
		}
		spec, err := decodeSpec(rt, call.Arguments[0])
		if err != nil {
			panic(rt.NewGoError(err))
		}
		buf, err := model.Marshal(spec)
		if err != nil {
			panic(rt.NewGoError(err))
		}
		return rt.ToValue(string(buf))
	})
	o.Set("validate", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) == 0 {
			panic(rt.NewTypeError("vizspec.validate: spec is required"))
		}
		spec, err := decodeSpec(rt, call.Arguments[0])
		if err != nil {
			panic(rt.NewGoError(err))
		}
		if err := spec.Validate(); err != nil {
			panic(rt.NewGoError(err))
		}
		return rt.ToValue(true)
	})
	o.Set("normalize", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) == 0 || goja.IsUndefined(call.Arguments[0]) || goja.IsNull(call.Arguments[0]) {
			return mustToValue(rt, (&model.Spec{}).Normalize())
		}
		spec, err := decodeSpec(rt, call.Arguments[0])
		if err != nil {
			panic(rt.NewGoError(err))
		}
		spec.Normalize()
		if err := spec.Validate(); err != nil {
			panic(rt.NewGoError(err))
		}
		model.NormalizeSpecTimeValues(spec)
		return mustToValue(rt, spec)
	})
	o.Set("createSpec", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) == 0 || goja.IsUndefined(call.Arguments[0]) || goja.IsNull(call.Arguments[0]) {
			return mustToValue(rt, (&model.Spec{}).Normalize())
		}
		spec, err := decodeSpec(rt, call.Arguments[0])
		if err != nil {
			panic(rt.NewGoError(err))
		}
		spec.Normalize()
		if err := spec.Validate(); err != nil {
			panic(rt.NewGoError(err))
		}
		model.NormalizeSpecTimeValues(spec)
		return mustToValue(rt, spec)
	})
	o.Set("toEChartsOption", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) == 0 {
			panic(rt.NewTypeError("vizspec.toEChartsOption: spec is required"))
		}
		spec, err := decodeSpec(rt, call.Arguments[0])
		if err != nil {
			panic(rt.NewGoError(err))
		}
		option, err := model.ToEChartsOption(spec)
		if err != nil {
			panic(rt.NewGoError(err))
		}
		return mustValueToJS(rt, option)
	})
	o.Set("toTUIBlocks", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) == 0 {
			panic(rt.NewTypeError("vizspec.toTUIBlocks: spec is required"))
		}
		spec, err := decodeSpec(rt, call.Arguments[0])
		if err != nil {
			panic(rt.NewGoError(err))
		}
		options, err := decodeTUIOptions(rt, call.Arguments, 1)
		if err != nil {
			panic(rt.NewGoError(err))
		}
		blocks, err := model.ToTUIBlocksWithOptions(spec, options)
		if err != nil {
			panic(rt.NewGoError(err))
		}
		return mustValueToJS(rt, blocks)
	})
	o.Set("toSVG", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) == 0 {
			panic(rt.NewTypeError("vizspec.toSVG: spec is required"))
		}
		spec, err := decodeSpec(rt, call.Arguments[0])
		if err != nil {
			panic(rt.NewGoError(err))
		}
		options, err := decodeSVGOptions(rt, call.Arguments, 1)
		if err != nil {
			panic(rt.NewGoError(err))
		}
		svg, err := model.ToSVG(spec, options)
		if err != nil {
			panic(rt.NewGoError(err))
		}
		return rt.ToValue(string(svg))
	})
}

func decodeTUIOptions(rt *goja.Runtime, args []goja.Value, index int) (*model.TUIOptions, error) {
	if len(args) <= index || goja.IsUndefined(args[index]) || goja.IsNull(args[index]) {
		return nil, nil
	}
	var input any
	if err := rt.ExportTo(args[index], &input); err != nil {
		return nil, err
	}
	buf, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	ret := &model.TUIOptions{}
	if err := json.Unmarshal(buf, ret); err != nil {
		return nil, err
	}
	return ret, nil
}

func decodeSVGOptions(rt *goja.Runtime, args []goja.Value, index int) (*model.SVGOptions, error) {
	if len(args) <= index || goja.IsUndefined(args[index]) || goja.IsNull(args[index]) {
		return nil, nil
	}
	var input any
	if err := rt.ExportTo(args[index], &input); err != nil {
		return nil, err
	}
	buf, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	ret := &model.SVGOptions{}
	if err := json.Unmarshal(buf, ret); err != nil {
		return nil, err
	}
	return ret, nil
}

func decodeSpec(rt *goja.Runtime, value goja.Value) (*model.Spec, error) {
	var input any
	if err := rt.ExportTo(value, &input); err != nil {
		return nil, err
	}
	buf, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	ret := &model.Spec{}
	decoder := json.NewDecoder(bytes.NewReader(buf))
	decoder.UseNumber()
	if err := decoder.Decode(ret); err != nil {
		return nil, err
	}
	ret.Normalize()
	return ret, nil
}

func mustToValue(rt *goja.Runtime, spec *model.Spec) goja.Value {
	return mustValueToJS(rt, spec)
}

func mustValueToJS(rt *goja.Runtime, value any) goja.Value {
	buf, err := json.Marshal(value)
	if err != nil {
		panic(rt.NewGoError(err))
	}
	jsonObject := rt.Get("JSON").ToObject(rt)
	parseFn, ok := goja.AssertFunction(jsonObject.Get("parse"))
	if !ok {
		panic(rt.NewTypeError("vizspec: JSON.parse is not available"))
	}
	ret, err := parseFn(goja.Undefined(), rt.ToValue(string(buf)))
	if err != nil {
		panic(rt.NewGoError(err))
	}
	return ret
}
