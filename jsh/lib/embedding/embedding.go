package embedding

import (
	"context"
	"fmt"

	"github.com/dop251/goja"
	service "github.com/machbase/neo-server/v8/mods/embedding"
)

func Module(ctx context.Context, rt *goja.Runtime, module *goja.Object) {
	exports := module.Get("exports").(*goja.Object)
	exports.Set("embed", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 || len(call.Arguments) > 2 {
			panic(rt.NewGoError(fmt.Errorf("embed(text, options) requires text")))
		}
		provider := providerOption(rt, call.Arguments)
		result, err := service.Default().Embed(ctx, call.Arguments[0].String(), provider)
		promise, resolve, reject := rt.NewPromise()
		if err != nil {
			_ = reject(err.Error())
		} else {
			_ = resolve(resultMap(result))
		}
		return rt.ToValue(promise)
	})
	exports.Set("embedMany", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 || len(call.Arguments) > 2 {
			panic(rt.NewGoError(fmt.Errorf("embedMany(texts, options) requires an array")))
		}
		var texts []string
		if err := rt.ExportTo(call.Arguments[0], &texts); err != nil {
			panic(rt.NewGoError(err))
		}
		provider := providerOption(rt, call.Arguments)
		results, err := service.Default().EmbedMany(ctx, texts, provider)
		promise, resolve, reject := rt.NewPromise()
		if err != nil {
			_ = reject(err.Error())
		} else {
			values := make([]map[string]any, len(results))
			for i, result := range results {
				values[i] = resultMap(result)
			}
			_ = resolve(values)
		}
		return rt.ToValue(promise)
	})
}

func resultMap(result service.Result) map[string]any {
	return map[string]any{
		"vector":    result.Vector,
		"dimension": result.Dimension,
		"space":     result.Space,
		"model":     result.Model,
	}
}

func providerOption(rt *goja.Runtime, args []goja.Value) string {
	if len(args) < 2 || goja.IsUndefined(args[1]) || goja.IsNull(args[1]) {
		return ""
	}
	var options struct {
		Provider string `json:"provider"`
	}
	if err := rt.ExportTo(args[1], &options); err != nil {
		panic(rt.NewGoError(err))
	}
	return options.Provider
}
