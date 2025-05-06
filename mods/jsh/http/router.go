package http

import (
	js "github.com/dop251/goja"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/render"
	"github.com/machbase/neo-server/v8/mods/util/ssfs"
)

type Router struct {
	ir  gin.IRouter `json:"-"`
	rt  *js.Runtime `json:"-"`
	obj *js.Object  `json:"-"`
}

func (r *Router) All(call js.FunctionCall) js.Value    { return r.handle("ANY", call) }
func (r *Router) Get(call js.FunctionCall) js.Value    { return r.handle("GET", call) }
func (r *Router) Post(call js.FunctionCall) js.Value   { return r.handle("POST", call) }
func (r *Router) Put(call js.FunctionCall) js.Value    { return r.handle("PUT", call) }
func (r *Router) Delete(call js.FunctionCall) js.Value { return r.handle("DELETE", call) }

func (r *Router) handle(method string, call js.FunctionCall) js.Value {
	var path string
	var callback js.Callable

	if err := r.rt.ExportTo(call.Arguments[0], &path); err != nil {
		panic(r.rt.ToValue("http.Router.All: invalid path " + err.Error()))
	}
	if err := r.rt.ExportTo(call.Arguments[1], &callback); err != nil {
		panic(r.rt.ToValue("http.Router.All: invalid callback " + err.Error()))
	}

	var methodHandler func(string, ...gin.HandlerFunc) gin.IRoutes
	switch method {
	case "GET":
		methodHandler = r.ir.GET
	case "POST":
		methodHandler = r.ir.POST
	case "PUT":
		methodHandler = r.ir.PUT
	case "DELETE":
		methodHandler = r.ir.DELETE
	case "ANY":
		methodHandler = r.ir.Any
	default:
		panic(r.rt.ToValue("http.Router.All: invalid method " + method))
	}

	methodHandler(path, func(ctx *gin.Context) {
		ctxObj := mkCtx(ctx, r.rt)
		if _, err := callback(js.Undefined(), ctxObj); err != nil {
			panic(r.rt.ToValue("http.Router.All: callback error " + err.Error()))
		}
	})
	return js.Undefined()
}

// StaticFile(string, string) IRoutes
// StaticFileFS(string, string, http.FileSystem) IRoutes
// StaticFS(string, http.FileSystem) IRoutes

func (r *Router) Static(call js.FunctionCall) js.Value {
	if len(call.Arguments) < 2 {
		panic(r.rt.ToValue("http.Router.Static: missing path or root"))
	}
	var path string
	if err := r.rt.ExportTo(call.Arguments[0], &path); err != nil {
		panic(r.rt.ToValue("http.Router.Static: invalid path " + err.Error()))
	}
	var root string
	if err := r.rt.ExportTo(call.Arguments[1], &root); err != nil {
		panic(r.rt.ToValue("http.Router.Static: invalid root " + err.Error()))
	}
	realPath, err := ssfs.Default().FindRealPath(root)
	if err != nil {
		panic(r.rt.ToValue("http.Router.Static: invalid root " + err.Error()))
	}
	r.ir.Static(path, realPath.AbsPath)
	return js.Undefined()
}

func (r *Router) StaticFile(call js.FunctionCall) js.Value {
	if len(call.Arguments) < 2 {
		panic(r.rt.ToValue("http.Router.StaticFile: missing path or file"))
	}
	var path string
	if err := r.rt.ExportTo(call.Arguments[0], &path); err != nil {
		panic(r.rt.ToValue("http.Router.StaticFile: invalid path " + err.Error()))
	}
	var file string
	if err := r.rt.ExportTo(call.Arguments[1], &file); err != nil {
		panic(r.rt.ToValue("http.Router.StaticFile: invalid file " + err.Error()))
	}
	realPath, err := ssfs.Default().FindRealPath(file)
	if err != nil {
		panic(r.rt.ToValue("http.Router.Static: invalid root " + err.Error()))
	}
	r.ir.StaticFile(path, realPath.AbsPath)
	return js.Undefined()
}

func mkCtx(ctx *gin.Context, rt *js.Runtime) js.Value {
	req := rt.NewObject()
	req.Set("header", ctx.Request.Header)
	req.Set("method", ctx.Request.Method)
	req.Set("url", ctx.Request.URL)
	req.Set("query", ctx.Request.URL.Query())
	req.Set("body", ctx.Request.Body)

	obj := rt.NewObject()
	obj.Set("writer", ctx.Writer)
	obj.Set("request", req)
	obj.Set("abort", func(call js.FunctionCall) js.Value {
		ctx.Abort()
		return js.Undefined()
	})
	obj.Set("redirect", func(call js.FunctionCall) js.Value {
		if len(call.Arguments) != 2 {
			panic(rt.ToValue("ctx.redirect: missing code, url"))
		}
		var code int
		if err := rt.ExportTo(call.Arguments[0], &code); err != nil {
			panic(rt.ToValue("ctx.redirect: invalid code " + err.Error()))
		}
		var url string
		if err := rt.ExportTo(call.Arguments[1], &url); err != nil {
			panic(rt.ToValue("ctx.redirect: invalid url " + err.Error()))
		}
		ctx.Redirect(code, url)
		return js.Undefined()
	})
	obj.Set("getHeader", func(call js.FunctionCall) js.Value {
		if len(call.Arguments) == 0 {
			panic(rt.ToValue("ctx.getHeader: missing header name"))
		}
		var name string
		if err := rt.ExportTo(call.Arguments[0], &name); err != nil {
			panic(rt.ToValue("ctx.getHeader: invalid header name " + err.Error()))
		}
		value := ctx.Request.Header.Get(name)
		return rt.ToValue(value)
	})
	obj.Set("setHeader", func(call js.FunctionCall) js.Value {
		var name string
		var value string
		if len(call.Arguments) < 2 {
			panic(rt.ToValue("ctx.setHeader: missing header name or value"))
		}
		if err := rt.ExportTo(call.Arguments[0], &name); err != nil {
			panic(rt.ToValue("ctx.setHeader: invalid header name " + err.Error()))
		}
		if err := rt.ExportTo(call.Arguments[1], &value); err != nil {
			panic(rt.ToValue("ctx.setHeader: invalid header value " + err.Error()))
		}
		ctx.Writer.Header().Set(name, value)
		return js.Undefined()
	})
	obj.Set("param", func(call js.FunctionCall) js.Value {
		if len(call.Arguments) == 0 {
			panic(rt.ToValue("ctx.param: missing param name"))
		}
		var name string
		if err := rt.ExportTo(call.Arguments[0], &name); err != nil {
			panic(rt.ToValue("ctx.param: invalid param name " + err.Error()))
		}
		value := ctx.Param(name)
		return rt.ToValue(value)
	})
	obj.Set("query", func(call js.FunctionCall) js.Value {
		if len(call.Arguments) == 0 {
			panic(rt.ToValue("ctx.query: missing query name"))
		}
		var name string
		if err := rt.ExportTo(call.Arguments[0], &name); err != nil {
			panic(rt.ToValue("ctx.query: invalid query name " + err.Error()))
		}
		value := ctx.Query(name)
		return rt.ToValue(value)
	})
	obj.Set("TEXT", renderType(ctx, rt, "string"))
	obj.Set("JSON", renderType(ctx, rt, "json"))
	obj.Set("HTML", renderType(ctx, rt, "html"))
	obj.Set("XML", renderType(ctx, rt, "xml"))
	obj.Set("YAML", renderType(ctx, rt, "yaml"))
	obj.Set("TOML", renderType(ctx, rt, "toml"))
	return obj
}

func renderType(ctx *gin.Context, rt *js.Runtime, typ string) func(js.FunctionCall) js.Value {
	return func(call js.FunctionCall) js.Value {
		if len(call.Arguments) < 2 {
			panic(rt.ToValue("ctx.string: missing status or string"))
		}
		var code int
		var data any
		var str string
		var values []any
		if err := rt.ExportTo(call.Arguments[0], &code); err != nil {
			panic(rt.ToValue("ctx.render: invalid status " + err.Error()))
		}
		if typ == "string" {
			if err := rt.ExportTo(call.Arguments[1], &str); err != nil {
				panic(rt.ToValue("ctx.render: invalid string " + err.Error()))
			}
			for i := 2; i < len(call.Arguments); i++ {
				var v any
				if err := rt.ExportTo(call.Arguments[i], &v); err != nil {
					panic(rt.ToValue("ctx.render: invalid value " + err.Error()))
				}
				values = append(values, v)
			}
		} else {
			if err := rt.ExportTo(call.Arguments[1], &data); err != nil {
				panic(rt.ToValue("ctx.render: invalid render " + err.Error()))
			}
		}
		var r render.Render
		switch typ {
		case "string":
			r = render.String{Format: str, Data: values}
		case "json":
			r = render.JSON{Data: data}
		case "indentJSON":
			r = render.IndentedJSON{Data: data}
		case "xml":
			r = render.XML{Data: data}
		case "yaml":
			r = render.YAML{Data: data}
		case "html":
			r = render.HTML{Data: data}
		case "toml":
			r = render.ProtoBuf{Data: data}
		default:
			panic(rt.ToValue("ctx.render: invalid render type " + typ))
		}
		ctx.Render(code, r)
		return js.Undefined()
	}
}
