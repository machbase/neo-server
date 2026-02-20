package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/render"
	"github.com/machbase/neo-server/v8/jsh/engine"
)

type Router struct {
	ir  *gin.Engine `json:"-"`
	env *engine.Env `json:"-"`
}

func (r *Router) All(path string, callback func(*RouterContext)) {
	r.handle("ANY", path, callback)
}
func (r *Router) Get(path string, callback func(*RouterContext)) {
	r.handle("GET", path, callback)
}
func (r *Router) Post(path string, callback func(*RouterContext)) {
	r.handle("POST", path, callback)
}
func (r *Router) Put(path string, callback func(*RouterContext)) {
	r.handle("PUT", path, callback)
}
func (r *Router) Delete(path string, callback func(*RouterContext)) {
	r.handle("DELETE", path, callback)
}

func (r *Router) handle(method string, path string, callback func(*RouterContext)) {
	var methodHandler func(string, ...gin.HandlerFunc) gin.IRoutes
	switch method {
	case "GET":
		methodHandler = r.ir.GET
	case "POST":
		methodHandler = r.ir.POST
	case "PUT":
		methodHandler = r.ir.PUT
	case "PATCH":
		methodHandler = r.ir.PATCH
	case "DELETE":
		methodHandler = r.ir.DELETE
	case "HEAD":
		methodHandler = r.ir.HEAD
	case "OPTIONS":
		methodHandler = r.ir.OPTIONS
	case "ANY":
		methodHandler = r.ir.Any
	default:
		panic(errors.New("http.Router.All: invalid method " + method))
	}

	methodHandler(path, func(ctx *gin.Context) {
		ctxObj, err := r.mkCtx(ctx)
		if err != nil {
			panic(fmt.Errorf("http.Router.%s: %v", method, err))
		}
		callback(ctxObj)
	})
}

func (r *Router) LoadHTMLFiles(paths ...string) error {
	fs := r.env.Filesystem().(*engine.FS)
	for i := range paths {
		osPath, err := fs.OSPath(paths[i])
		if err != nil {
			return errors.New("http.Router.LoadHTMLFiles: invalid template " + err.Error())
		}
		paths[i] = osPath
	}
	r.ir.LoadHTMLFiles(paths...)
	return nil
}

func (r *Router) LoadHTMLGlob(pattern string) error {
	fs := r.env.Filesystem().(*engine.FS)
	osPath, err := fs.OSPath(pattern)
	if err != nil {
		return errors.New("http.Router.LoadHTMLFiles: invalid template " + err.Error())
	}
	r.ir.LoadHTMLGlob(osPath)
	return nil
}

func (r *Router) Static(path string, root string) error {
	fs := r.env.Filesystem().(*engine.FS)
	osPath, err := fs.OSPath(root)
	if err != nil {
		return errors.New("http.Router.Static: invalid root, " + err.Error())
	}
	r.ir.Static(path, osPath)
	return nil
}

func (r *Router) StaticFile(path string, file string) error {
	fs := r.env.Filesystem().(*engine.FS)
	osPath, err := fs.OSPath(file)
	if err != nil {
		return errors.New("http.Router.StaticFile: invalid file, " + err.Error())
	}
	r.ir.StaticFile(path, osPath)
	return nil
}

func (r *Router) mkCtx(ctx *gin.Context) (*RouterContext, error) {
	req := &RouterRequest{Request: ctx.Request}
	req.Path = ctx.Request.URL.Path
	req.Query = ctx.Request.URL.RawQuery

	contentType := ctx.ContentType()
	switch contentType {
	case "application/json":
		obj := make(map[string]any)
		dec := json.NewDecoder(ctx.Request.Body)
		if err := dec.Decode(&obj); err != nil {
			return nil, errors.New("http.Router.All: invalid json " + err.Error())
		} else {
			req.Body = obj
		}
	case "text/plain":
		if bs, err := io.ReadAll(ctx.Request.Body); err != nil {
			return nil, errors.New("http.Router.All: invalid text " + err.Error())
		} else {
			req.Body = string(bs)
		}
	default:
		if bs, err := io.ReadAll(ctx.Request.Body); err != nil {
			return nil, errors.New("http.Router.All: invalid body " + err.Error())
		} else {
			req.Body = bs
		}
	}

	ret := &RouterContext{
		Context:  ctx,
		Request:  req,
		Response: ctx.Writer,
	}
	return ret, nil
}

type RouterRequest struct {
	*http.Request
	Body  any
	Path  string
	Query string
}

func (r *RouterRequest) GetHeader(name string) string {
	return r.Request.Header.Get(name)
}

type RouterContext struct {
	*gin.Context
	Request  *RouterRequest
	Response gin.ResponseWriter
}

func (ctx *RouterContext) Abort() {
	ctx.Context.Abort()
}

func (ctx *RouterContext) Redirect(code int, url string) {
	ctx.Context.Redirect(code, url)
}

func (ctx *RouterContext) SetHeader(name string, value string) {
	ctx.Writer.Header().Set(name, value)
}

func (ctx *RouterContext) Param(name string) string {
	return ctx.Context.Param(name)
}
func (ctx *RouterContext) Query(name string) string {
	return ctx.Context.Query(name)
}

func (ctx *RouterContext) Text(status int, text string, args ...any) error {
	return ctx.renderType("text", status, append([]any{text}, args...)...)
}
func (ctx *RouterContext) Json(status int, data any, opt any) error {
	return ctx.renderType("json", status, data, opt)
}
func (ctx *RouterContext) Html(status int, template string, data any) error {
	return ctx.renderType("html", status, template, data)
}
func (ctx *RouterContext) Xml(status int, data any) error {
	return ctx.renderType("xml", status, data)
}
func (ctx *RouterContext) Yaml(status int, data any) error {
	return ctx.renderType("yaml", status, data)
}
func (ctx *RouterContext) Toml(status int, data any) error {
	return ctx.renderType("toml", status, data)
}

func (ctx *RouterContext) renderType(typ string, code int, args ...any) error {
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

	switch typ {
	case "text":
		if str, ok := args[0].(string); !ok {
			return errors.New("ctx.render: invalid string " + str)
		} else {
			textFormat = str
		}
		for i := 1; i < len(args); i++ {
			textArgs = append(textArgs, args[i])
		}
	case "html":
		if str, ok := args[0].(string); !ok {
			return errors.New("ctx.render: invalid template " + str)
		} else {
			htmlTemplate = str
		}
		if len(args) > 1 {
			data = args[1]
		}
	default:
		data = args[0]
		if len(args) > 1 {
			if m, ok := args[1].(map[string]any); ok {
				for k, v := range m {
					switch k {
					case "indent":
						if indent, ok := v.(bool); ok {
							opts.Indent = indent
						}
					}
				}
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
			return errors.New("ctx.render: missing template")
		}
		ctx.HTML(code, htmlTemplate, data)
		return nil
	default:
		return errors.New("ctx.render: invalid render type " + typ)
	}
	ctx.Render(code, r)
	return nil
}
