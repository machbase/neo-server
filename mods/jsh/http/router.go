package http

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"

	js "github.com/dop251/goja"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/render"
	"github.com/machbase/neo-server/v8/mods/util/ssfs"
)

type Router struct {
	ir  *gin.Engine `json:"-"`
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
		ctxObj := r.mkCtx(ctx)
		if _, err := callback(js.Undefined(), ctxObj); err != nil {
			panic(r.rt.ToValue("http.Router.All: callback error " + err.Error()))
		}
	})
	return js.Undefined()
}

func (r *Router) LoadHTMLFiles(call js.FunctionCall) js.Value {
	if len(call.Arguments) < 1 {
		panic(r.rt.ToValue("http.Router.LoadHTMLFiles: missing template"))
	}

	root := ssfs.Default()
	paths := make([]string, 0, len(call.Arguments))
	for i := 0; i < len(call.Arguments); i++ {
		var arg string
		if err := r.rt.ExportTo(call.Arguments[i], &arg); err != nil {
			panic(r.rt.ToValue("http.Router.LoadHTMLFiles: invalid template " + err.Error()))
		}
		realPath, err := root.FindRealPath(arg)
		if err != nil {
			panic(r.rt.ToValue("http.Router.LoadHTMLFiles: invalid template " + err.Error()))
		}
		paths = append(paths, realPath.AbsPath)
	}
	fmt.Println("LoadHTMLFiles", paths)
	r.ir.LoadHTMLFiles(paths...)
	return js.Undefined()
}

func (r *Router) LoadHTMLGlob(call js.FunctionCall) js.Value {
	if len(call.Arguments) < 2 {
		panic(r.rt.ToValue("http.Router.LoadTemplate: missing template directory and pattern"))
	}
	var path string
	if err := r.rt.ExportTo(call.Arguments[0], &path); err != nil {
		panic(r.rt.ToValue("http.Router.LoadTemplate: invalid template directory" + err.Error()))
	}
	var pattern string
	if err := r.rt.ExportTo(call.Arguments[1], &pattern); err != nil {
		panic(r.rt.ToValue("http.Router.LoadTemplate: invalid template " + err.Error()))
	}
	realPath, err := ssfs.Default().FindRealPath(path)
	if err != nil {
		panic(r.rt.ToValue("http.Router.LoadTemplate: invalid template " + err.Error()))
	}
	pathPattern := filepath.Join(realPath.AbsPath, pattern)
	fmt.Println("LoadHTMLGlob", pathPattern)
	r.ir.LoadHTMLGlob(pathPattern)
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

func (r *Router) mkCtx(ctx *gin.Context) js.Value {
	rt := r.rt
	req := rt.NewObject()
	contentType := ctx.ContentType()
	if contentType == "application/json" {
		obj := make(map[string]any)
		dec := json.NewDecoder(ctx.Request.Body)
		if err := dec.Decode(&obj); err != nil {
			panic(rt.ToValue("http.Router.All: invalid json " + err.Error()))
		} else {
			req.Set("body", rt.ToValue(obj))
		}
	} else if contentType == "text/plain" {
		if bs, err := io.ReadAll(ctx.Request.Body); err != nil {
			panic(rt.ToValue("http.Router.All: invalid text " + err.Error()))
		} else {
			req.Set("body", rt.ToValue(string(bs)))
		}
	} else {
		if bs, err := io.ReadAll(ctx.Request.Body); err != nil {
			panic(rt.ToValue("http.Router.All: invalid body " + err.Error()))
		} else {
			req.Set("body", rt.NewArrayBuffer(bs))
		}
	}
	req.Set("header", ctx.Request.Header)
	req.Set("method", ctx.Request.Method)
	req.Set("remoteAddress", ctx.Request.RemoteAddr)
	req.Set("host", ctx.Request.Host)
	req.Set("path", ctx.Request.URL.Path)
	req.Set("query", ctx.Request.URL.Query())
	req.Set("getHeader", func(call js.FunctionCall) js.Value {
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

	obj := rt.NewObject()
	obj.Set("response", ctx.Writer)
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
	obj.Set("TEXT", r.renderType(ctx, "text"))
	obj.Set("JSON", r.renderType(ctx, "json"))
	obj.Set("HTML", r.renderType(ctx, "html"))
	obj.Set("XML", r.renderType(ctx, "xml"))
	obj.Set("YAML", r.renderType(ctx, "yaml"))
	obj.Set("TOML", r.renderType(ctx, "toml"))
	return obj
}

func (r *Router) renderType(ctx *gin.Context, typ string) func(js.FunctionCall) js.Value {
	rt := r.rt
	return func(call js.FunctionCall) js.Value {
		if len(call.Arguments) < 2 {
			panic(rt.ToValue("ctx.render: missing status or string"))
		}
		var code int
		var data any
		var opts = struct {
			// JSON
			Indent bool `json:"indent"`
		}{}
		// HTML
		var htmlTemplate string
		// TEXT
		var textFormat string
		var textArgs []any
		if err := rt.ExportTo(call.Arguments[0], &code); err != nil {
			panic(rt.ToValue("ctx.render: invalid status " + err.Error()))
		}
		if typ == "text" {
			if err := rt.ExportTo(call.Arguments[1], &textFormat); err != nil {
				panic(rt.ToValue("ctx.render: invalid string " + err.Error()))
			}
			for i := 2; i < len(call.Arguments); i++ {
				var v any
				if err := rt.ExportTo(call.Arguments[i], &v); err != nil {
					panic(rt.ToValue("ctx.render: invalid value " + err.Error()))
				}
				textArgs = append(textArgs, v)
			}
		} else if typ == "html" {
			if err := rt.ExportTo(call.Arguments[1], &htmlTemplate); err != nil {
				panic(rt.ToValue("ctx.render: invalid template " + err.Error()))
			}
			if len(call.Arguments) > 2 {
				if err := rt.ExportTo(call.Arguments[2], &data); err != nil {
					panic(rt.ToValue("ctx.render: invalid data " + err.Error()))
				}
			}
		} else {
			if err := rt.ExportTo(call.Arguments[1], &data); err != nil {
				panic(rt.ToValue("ctx.render: invalid render " + err.Error()))
			}
			if len(call.Arguments) > 2 {
				if err := rt.ExportTo(call.Arguments[2], &opts); err != nil {
					panic(rt.ToValue("ctx.render: invalid options " + err.Error()))
				}
			}
		}
		// type casting to gin.H to be supported by pre-defined marshaler
		if m, ok := data.(map[string]any); ok {
			data = gin.H(m)
		}
		// decide render type
		var r render.Render
		switch typ {
		case "text":
			r = render.String{Format: textFormat, Data: textArgs}
		case "json":
			if opts.Indent {
				r = render.IndentedJSON{Data: data}
			} else {
				r = render.JSON{Data: data}
			}
		case "yaml":
			r = render.YAML{Data: data}
		case "toml":
			r = render.TOML{Data: data}
		case "xml":
			r = render.XML{Data: data}
		case "html":
			if htmlTemplate == "" {
				panic(rt.ToValue("ctx.render: missing template"))
			}
			ctx.HTML(code, htmlTemplate, data)
			return js.Undefined()
		default:
			panic(rt.ToValue("ctx.render: invalid render type " + typ))
		}
		ctx.Render(code, r)
		return js.Undefined()
	}
}
