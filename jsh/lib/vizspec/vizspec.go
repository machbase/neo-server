package vizspec

import (
	_ "embed"
	"encoding/json"
	"fmt"

	"github.com/dop251/goja"
	"github.com/machbase/neo-server/v8/jsh/advn"
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
	jsonParse, jsonStringify := mustJSONFunctions(rt)
	o.Set("parse", func(text string) goja.Value {
		spec, err := advn.ParseString(text)
		if err != nil {
			panic(rt.NewGoError(err))
		}
		advn.NormalizeSpecTimeValues(spec)
		return mustToValue(rt, jsonParse, spec)
	})
	o.Set("stringify", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) == 0 {
			panic(rt.NewTypeError("vizspec.stringify: spec is required"))
		}
		spec, err := decodeSpec(rt, jsonStringify, call.Arguments[0])
		if err != nil {
			panic(rt.NewGoError(err))
		}
		buf, err := advn.Marshal(spec)
		if err != nil {
			panic(rt.NewGoError(err))
		}
		return rt.ToValue(string(buf))
	})
	o.Set("validate", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) == 0 {
			panic(rt.NewTypeError("vizspec.validate: spec is required"))
		}
		spec, err := decodeSpec(rt, jsonStringify, call.Arguments[0])
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
			return mustToValue(rt, jsonParse, (&advn.Spec{}).Normalize())
		}
		spec, err := decodeSpec(rt, jsonStringify, call.Arguments[0])
		if err != nil {
			panic(rt.NewGoError(err))
		}
		spec.Normalize()
		if err := spec.Validate(); err != nil {
			panic(rt.NewGoError(err))
		}
		advn.NormalizeSpecTimeValues(spec)
		return mustToValue(rt, jsonParse, spec)
	})
	o.Set("createSpec", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) == 0 || goja.IsUndefined(call.Arguments[0]) || goja.IsNull(call.Arguments[0]) {
			return mustToValue(rt, jsonParse, (&advn.Spec{}).Normalize())
		}
		spec, err := decodeSpec(rt, jsonStringify, call.Arguments[0])
		if err != nil {
			panic(rt.NewGoError(err))
		}
		spec.Normalize()
		if err := spec.Validate(); err != nil {
			panic(rt.NewGoError(err))
		}
		advn.NormalizeSpecTimeValues(spec)
		return mustToValue(rt, jsonParse, spec)
	})
	o.Set("toEChartsOption", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) == 0 {
			panic(rt.NewTypeError("vizspec.toEChartsOption: spec is required"))
		}
		spec, err := decodeSpec(rt, jsonStringify, call.Arguments[0])
		if err != nil {
			panic(rt.NewGoError(err))
		}
		options, err := decodeEChartsOptions(rt, call.Arguments, 1)
		if err != nil {
			panic(rt.NewGoError(err))
		}
		option, err := advn.ToEChartsOptionWithOptions(spec, options)
		if err != nil {
			panic(rt.NewGoError(err))
		}
		return mustValueToJS(rt, jsonParse, option)
	})
	o.Set("toTUIBlocks", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) == 0 {
			panic(rt.NewTypeError("vizspec.toTUIBlocks: spec is required"))
		}
		spec, err := decodeSpec(rt, jsonStringify, call.Arguments[0])
		if err != nil {
			panic(rt.NewGoError(err))
		}
		options, err := decodeTUIOptions(rt, call.Arguments, 1)
		if err != nil {
			panic(rt.NewGoError(err))
		}
		blocks, err := advn.ToTUIBlocksWithOptions(spec, options)
		if err != nil {
			panic(rt.NewGoError(err))
		}
		return mustValueToJS(rt, jsonParse, blocks)
	})
	o.Set("toTUILines", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) == 0 {
			panic(rt.NewTypeError("vizspec.toTUILines: spec is required"))
		}
		spec, err := decodeSpec(rt, jsonStringify, call.Arguments[0])
		if err != nil {
			panic(rt.NewGoError(err))
		}
		options, err := decodeTUIOptions(rt, call.Arguments, 1)
		if err != nil {
			panic(rt.NewGoError(err))
		}
		lines, err := advn.ToSparklineWithOptions(spec, options)
		if err != nil {
			panic(rt.NewGoError(err))
		}
		return mustValueToJS(rt, jsonParse, lines)
	})
	o.Set("toSVG", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) == 0 {
			panic(rt.NewTypeError("vizspec.toSVG: spec is required"))
		}
		spec, err := decodeSpec(rt, jsonStringify, call.Arguments[0])
		if err != nil {
			panic(rt.NewGoError(err))
		}
		options, err := decodeSVGOptions(rt, call.Arguments, 1)
		if err != nil {
			panic(rt.NewGoError(err))
		}
		svg, err := advn.ToSVG(spec, options)
		if err != nil {
			panic(rt.NewGoError(err))
		}
		return rt.ToValue(string(svg))
	})
	o.Set("toPNG", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) == 0 {
			panic(rt.NewTypeError("vizspec.toPNG: spec is required"))
		}
		spec, err := decodeSpec(rt, jsonStringify, call.Arguments[0])
		if err != nil {
			panic(rt.NewGoError(err))
		}
		svgOptions, err := decodeSVGOptions(rt, call.Arguments, 1)
		if err != nil {
			panic(rt.NewGoError(err))
		}
		pngOptions, err := decodePNGOptions(rt, call.Arguments, 2)
		if err != nil {
			panic(rt.NewGoError(err))
		}
		data, err := advn.ToPNG(spec, svgOptions, pngOptions)
		if err != nil {
			panic(rt.NewGoError(err))
		}
		return rt.ToValue(rt.NewArrayBuffer(data))
	})
}

func decodeEChartsOptions(rt *goja.Runtime, args []goja.Value, index int) (*advn.EChartsOptions, error) {
	if len(args) <= index || goja.IsUndefined(args[index]) || goja.IsNull(args[index]) {
		return nil, nil
	}
	obj := args[index].ToObject(rt)
	return &advn.EChartsOptions{
		Timeformat: optionalString(obj, "timeformat"),
		TZ:         optionalString(obj, "tz"),
	}, nil
}

func decodeTUIOptions(rt *goja.Runtime, args []goja.Value, index int) (*advn.TUIOptions, error) {
	if len(args) <= index || goja.IsUndefined(args[index]) || goja.IsNull(args[index]) {
		return nil, nil
	}
	obj := args[index].ToObject(rt)
	return &advn.TUIOptions{
		Height:     optionalInt(obj, "height"),
		Width:      optionalInt(obj, "width"),
		Rows:       optionalInt(obj, "rows"),
		Compact:    optionalBool(obj, "compact"),
		SeriesID:   optionalString(obj, "seriesId"),
		Timeformat: optionalString(obj, "timeformat"),
		TZ:         optionalString(obj, "tz"),
	}, nil
}

func decodeSVGOptions(rt *goja.Runtime, args []goja.Value, index int) (*advn.SVGOptions, error) {
	if len(args) <= index || goja.IsUndefined(args[index]) || goja.IsNull(args[index]) {
		return nil, nil
	}
	obj := args[index].ToObject(rt)
	return &advn.SVGOptions{
		Width:      optionalInt(obj, "width"),
		Height:     optionalInt(obj, "height"),
		Padding:    optionalInt(obj, "padding"),
		Background: optionalString(obj, "background"),
		FontFamily: optionalString(obj, "fontFamily"),
		FontSize:   optionalInt(obj, "fontSize"),
		ShowLegend: optionalBoolPtr(obj, "showLegend"),
		Title:      optionalString(obj, "title"),
		Timeformat: optionalString(obj, "timeformat"),
		TZ:         optionalString(obj, "tz"),
	}, nil
}

func decodePNGOptions(rt *goja.Runtime, args []goja.Value, index int) (*advn.PNGOptions, error) {
	if len(args) <= index || goja.IsUndefined(args[index]) || goja.IsNull(args[index]) {
		return nil, nil
	}
	obj := args[index].ToObject(rt)
	return &advn.PNGOptions{
		Scale:      optionalFloat(obj, "scale"),
		DPI:        optionalInt(obj, "dpi"),
		Background: optionalString(obj, "background"),
		Theme:      optionalString(obj, "theme"),
	}, nil
}

func decodeSpec(rt *goja.Runtime, jsonStringify goja.Callable, value goja.Value) (*advn.Spec, error) {
	text, err := stringifyValue(rt, jsonStringify, value)
	if err != nil {
		return nil, err
	}
	ret, err := advn.ParseString(text)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

func mustToValue(rt *goja.Runtime, jsonParse goja.Callable, spec *advn.Spec) goja.Value {
	return mustValueToJS(rt, jsonParse, spec)
}

func mustValueToJS(rt *goja.Runtime, jsonParse goja.Callable, value any) goja.Value {
	buf, err := json.Marshal(value)
	if err != nil {
		panic(rt.NewGoError(err))
	}
	ret, err := jsonParse(goja.Undefined(), rt.ToValue(string(buf)))
	if err != nil {
		panic(rt.NewGoError(err))
	}
	return ret
}

func mustJSONFunctions(rt *goja.Runtime) (goja.Callable, goja.Callable) {
	jsonObject := rt.Get("JSON").ToObject(rt)
	parseFn, ok := goja.AssertFunction(jsonObject.Get("parse"))
	if !ok {
		panic(rt.NewTypeError("vizspec: JSON.parse is not available"))
	}
	stringifyFn, ok := goja.AssertFunction(jsonObject.Get("stringify"))
	if !ok {
		panic(rt.NewTypeError("vizspec: JSON.stringify is not available"))
	}
	return parseFn, stringifyFn
}

func stringifyValue(rt *goja.Runtime, jsonStringify goja.Callable, value goja.Value) (string, error) {
	ret, err := jsonStringify(goja.Undefined(), value)
	if err != nil {
		return "", err
	}
	if goja.IsUndefined(ret) {
		return "", fmt.Errorf("vizspec: value cannot be stringified")
	}
	return ret.String(), nil
}

func optionalString(obj *goja.Object, key string) string {
	value := obj.Get(key)
	if value == nil || goja.IsUndefined(value) || goja.IsNull(value) {
		return ""
	}
	return value.String()
}

func optionalInt(obj *goja.Object, key string) int {
	value := obj.Get(key)
	if value == nil || goja.IsUndefined(value) || goja.IsNull(value) {
		return 0
	}
	return int(value.ToInteger())
}

func optionalFloat(obj *goja.Object, key string) float64 {
	value := obj.Get(key)
	if value == nil || goja.IsUndefined(value) || goja.IsNull(value) {
		return 0
	}
	return value.ToFloat()
}

func optionalBool(obj *goja.Object, key string) bool {
	value := obj.Get(key)
	if value == nil || goja.IsUndefined(value) || goja.IsNull(value) {
		return false
	}
	return value.ToBoolean()
}

func optionalBoolPtr(obj *goja.Object, key string) *bool {
	value := obj.Get(key)
	if value == nil || goja.IsUndefined(value) || goja.IsNull(value) {
		return nil
	}
	ret := value.ToBoolean()
	return &ret
}
