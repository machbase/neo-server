package http

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"reflect"
	"strings"

	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/eventloop"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/render"
	"github.com/gorilla/websocket"
	"github.com/machbase/neo-server/v8/jsh/engine"
	wsmod "github.com/machbase/neo-server/v8/jsh/lib/ws"
)

type Router struct {
	ir   *gin.Engine          `json:"-"`
	env  *engine.Env          `json:"-"`
	loop *eventloop.EventLoop `json:"-"`
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

func (r *Router) WebSocket(path string, verifyClient func(*WebSocketRequest) bool, handleProtocols func([]string, *WebSocketRequest) string, callback func(*wsmod.WebSocket, *WebSocketRequest)) {
	upgrader := wsmod.NewUpgrader()
	r.ir.GET(path, func(ctx *gin.Context) {
		if !wsmod.IsWebSocketUpgrade(ctx.Request) {
			ctx.String(http.StatusBadRequest, "websocket upgrade required")
			return
		}

		request := NewWebSocketRequest(ctx.Request)
		if verifyClient != nil {
			accepted, err := r.callWebSocketVerify(verifyClient, request)
			if err != nil {
				ctx.String(http.StatusInternalServerError, err.Error())
				return
			}
			if !accepted {
				ctx.String(http.StatusForbidden, "websocket upgrade rejected")
				return
			}
		}

		responseHeader := http.Header{}
		if handleProtocols != nil {
			selected, err := r.callWebSocketProtocols(handleProtocols, websocket.Subprotocols(ctx.Request), request)
			if err != nil {
				ctx.String(http.StatusInternalServerError, err.Error())
				return
			}
			if selected != "" {
				responseHeader.Set("Sec-WebSocket-Protocol", selected)
			}
		}

		conn, err := upgrader.Upgrade(ctx.Writer, ctx.Request, responseHeader)
		if err != nil {
			return
		}

		accepted := wsmod.NewAcceptedWebSocket(conn, r.loop)
		if err := r.callWebSocketHandler(callback, accepted, request); err != nil {
			_ = accepted.Close()
			return
		}
		accepted.RunDirect()
	})
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
		if err := r.callHandler(callback, ctxObj); err != nil {
			ctx.String(http.StatusInternalServerError, err.Error())
		}
	})
}

func (r *Router) callHandler(callback func(*RouterContext), ctxObj *RouterContext) (err error) {
	if r.loop == nil {
		defer func() {
			if recovered := recover(); recovered != nil {
				err = fmt.Errorf("%v", recovered)
			}
		}()
		callback(ctxObj)
		return nil
	}

	done := make(chan error, 1)
	if ok := r.loop.RunOnLoop(func(vm *goja.Runtime) {
		defer func() {
			if recovered := recover(); recovered != nil {
				done <- fmt.Errorf("%v", recovered)
			} else {
				done <- nil
			}
		}()
		callback(ctxObj)
	}); !ok {
		return errors.New("http.Router: event loop is closed")
	}
	return <-done
}

func (r *Router) callWebSocketVerify(callback func(*WebSocketRequest) bool, request *WebSocketRequest) (result bool, err error) {
	if r.loop == nil {
		defer func() {
			if recovered := recover(); recovered != nil {
				err = fmt.Errorf("%v", recovered)
			}
		}()
		return callback(request), nil
	}

	type verifyResult struct {
		result bool
		err    error
	}
	done := make(chan verifyResult, 1)
	if ok := r.loop.RunOnLoop(func(vm *goja.Runtime) {
		defer func() {
			if recovered := recover(); recovered != nil {
				done <- verifyResult{err: fmt.Errorf("%v", recovered)}
			}
		}()
		done <- verifyResult{result: callback(request)}
	}); !ok {
		return false, errors.New("http.Router: event loop is closed")
	}
	ret := <-done
	return ret.result, ret.err
}

func (r *Router) callWebSocketProtocols(callback func([]string, *WebSocketRequest) string, protocols []string, request *WebSocketRequest) (selected string, err error) {
	if r.loop == nil {
		defer func() {
			if recovered := recover(); recovered != nil {
				err = fmt.Errorf("%v", recovered)
			}
		}()
		return callback(protocols, request), nil
	}

	type protocolResult struct {
		selected string
		err      error
	}
	done := make(chan protocolResult, 1)
	if ok := r.loop.RunOnLoop(func(vm *goja.Runtime) {
		defer func() {
			if recovered := recover(); recovered != nil {
				done <- protocolResult{err: fmt.Errorf("%v", recovered)}
			}
		}()
		done <- protocolResult{selected: callback(protocols, request)}
	}); !ok {
		return "", errors.New("http.Router: event loop is closed")
	}
	ret := <-done
	return ret.selected, ret.err
}

func (r *Router) callWebSocketHandler(callback func(*wsmod.WebSocket, *WebSocketRequest), socket *wsmod.WebSocket, request *WebSocketRequest) (err error) {
	if r.loop == nil {
		defer func() {
			if recovered := recover(); recovered != nil {
				err = fmt.Errorf("%v", recovered)
			}
		}()
		callback(socket, request)
		return nil
	}

	done := make(chan error, 1)
	if ok := r.loop.RunOnLoop(func(vm *goja.Runtime) {
		defer func() {
			if recovered := recover(); recovered != nil {
				done <- fmt.Errorf("%v", recovered)
			} else {
				done <- nil
			}
		}()
		callback(socket, request)
	}); !ok {
		return errors.New("http.Router: event loop is closed")
	}
	return <-done
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

type WebSocketRequest struct {
	URL           string
	Method        string
	Headers       map[string]any
	RawHeaders    []string
	Path          string
	RemoteAddress string
	Host          string
	Proto         string
	RequestURI    string

	req *http.Request
}

func NewWebSocketRequest(req *http.Request) *WebSocketRequest {
	headers := map[string]any{}
	rawHeaders := make([]string, 0, len(req.Header)*2)
	for key, values := range req.Header {
		if len(values) == 1 {
			headers[key] = values[0]
		} else {
			copied := make([]string, len(values))
			copy(copied, values)
			headers[key] = copied
		}
		for _, value := range values {
			rawHeaders = append(rawHeaders, key, value)
		}
	}
	return &WebSocketRequest{
		URL:           req.URL.String(),
		Method:        req.Method,
		Headers:       headers,
		RawHeaders:    rawHeaders,
		Path:          req.URL.Path,
		RemoteAddress: req.RemoteAddr,
		Host:          req.Host,
		Proto:         req.Proto,
		RequestURI:    req.RequestURI,
		req:           req,
	}
}

func (req *WebSocketRequest) Query(name string) string {
	if req == nil || req.req == nil {
		return ""
	}
	return req.req.URL.Query().Get(name)
}

func (req *WebSocketRequest) GetHeader(name string) string {
	if req == nil || req.req == nil {
		return ""
	}
	return req.req.Header.Get(name)
}

func (req *WebSocketRequest) HasHeader(name string) bool {
	if req == nil || req.req == nil {
		return false
	}
	return req.req.Header.Get(name) != ""
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

func (ctx *RouterContext) HasQuery(name string) (string, bool) {
	return ctx.Context.GetQuery(name)
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
func (ctx *RouterContext) Xml(status int, data any, opt any) error {
	return ctx.renderType("xml", status, data, opt)
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
		Space string `json:"space"`
		// XML
		Root string `json:"root"`
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
					case "space":
						opts.Space = jsonIndentFromSpace(v)
					case "root":
						if root, ok := v.(string); ok {
							opts.Root = root
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
	if typ == "xml" {
		data = normalizeXMLData(data)
		if opts.Root != "" {
			data = xmlRootElement{Name: opts.Root, Data: data}
		}
	}
	// decide render type
	var r render.Render
	switch typ {
	case "text":
		r = render.String{Format: textFormat, Data: textArgs}
	case "json":
		if opts.Space != "" {
			r = jsonRender{Data: data, Indent: opts.Space}
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

func normalizeXMLData(value any) any {
	if value == nil {
		return nil
	}
	switch typed := value.(type) {
	case xmlMap:
		result := make(xmlMap, len(typed))
		for key, child := range typed {
			result[key] = normalizeXMLData(child)
		}
		return result
	case gin.H:
		result := make(xmlMap, len(typed))
		for key, child := range typed {
			result[key] = normalizeXMLData(child)
		}
		return result
	case map[string]any:
		result := make(xmlMap, len(typed))
		for key, child := range typed {
			result[key] = normalizeXMLData(child)
		}
		return result
	case []any:
		result := make([]any, len(typed))
		for idx, child := range typed {
			result[idx] = normalizeXMLData(child)
		}
		return result
	case []byte:
		return typed
	}

	rv := reflect.ValueOf(value)
	if !rv.IsValid() {
		return nil
	}
	switch rv.Kind() {
	case reflect.Interface, reflect.Pointer:
		if rv.IsNil() {
			return nil
		}
		return normalizeXMLData(rv.Elem().Interface())
	case reflect.Map:
		if rv.Type().Key().Kind() != reflect.String {
			return value
		}
		result := make(xmlMap, rv.Len())
		iter := rv.MapRange()
		for iter.Next() {
			result[iter.Key().String()] = normalizeXMLData(iter.Value().Interface())
		}
		return result
	case reflect.Slice, reflect.Array:
		result := make([]any, rv.Len())
		for idx := 0; idx < rv.Len(); idx++ {
			result[idx] = normalizeXMLData(rv.Index(idx).Interface())
		}
		return result
	default:
		return value
	}
}

func jsonIndentFromSpace(value any) string {
	if value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return truncateJSONSpaceString(typed)
	case int:
		return strings.Repeat(" ", clampJSONSpace(typed))
	case int8:
		return strings.Repeat(" ", clampJSONSpace(int(typed)))
	case int16:
		return strings.Repeat(" ", clampJSONSpace(int(typed)))
	case int32:
		return strings.Repeat(" ", clampJSONSpace(int(typed)))
	case int64:
		return strings.Repeat(" ", clampJSONSpace(int(typed)))
	case uint:
		return strings.Repeat(" ", clampJSONSpace(int(typed)))
	case uint8:
		return strings.Repeat(" ", clampJSONSpace(int(typed)))
	case uint16:
		return strings.Repeat(" ", clampJSONSpace(int(typed)))
	case uint32:
		return strings.Repeat(" ", clampJSONSpace(int(typed)))
	case uint64:
		return strings.Repeat(" ", clampJSONSpace(int(typed)))
	case float32:
		return strings.Repeat(" ", clampJSONSpace(int(math.Floor(float64(typed)))))
	case float64:
		return strings.Repeat(" ", clampJSONSpace(int(math.Floor(typed))))
	}

	rv := reflect.ValueOf(value)
	if !rv.IsValid() {
		return ""
	}
	switch rv.Kind() {
	case reflect.String:
		return truncateJSONSpaceString(rv.String())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strings.Repeat(" ", clampJSONSpace(int(rv.Int())))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return strings.Repeat(" ", clampJSONSpace(int(rv.Uint())))
	case reflect.Float32, reflect.Float64:
		return strings.Repeat(" ", clampJSONSpace(int(math.Floor(rv.Float()))))
	default:
		return ""
	}
}

func truncateJSONSpaceString(space string) string {
	runes := []rune(space)
	if len(runes) > 10 {
		runes = runes[:10]
	}
	return string(runes)
}

func clampJSONSpace(value int) int {
	if value < 0 {
		return 0
	}
	if value > 10 {
		return 10
	}
	return value
}

type jsonRender struct {
	Data   any
	Indent string
}

func (r jsonRender) Render(w http.ResponseWriter) error {
	r.WriteContentType(w)
	var (
		data []byte
		err  error
	)
	if r.Indent == "" {
		data, err = json.Marshal(r.Data)
	} else {
		data, err = json.MarshalIndent(r.Data, "", r.Indent)
	}
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

func (r jsonRender) WriteContentType(w http.ResponseWriter) {
	if w.Header().Get("Content-Type") == "" {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
	}
}

type xmlMap map[string]any

type xmlRootElement struct {
	Name string
	Data any
}

func (e xmlRootElement) MarshalXML(enc *xml.Encoder, start xml.StartElement) error {
	if e.Name == "" {
		return enc.EncodeElement(e.Data, start)
	}
	start.Name.Local = e.Name
	return enc.EncodeElement(e.Data, start)
}

func (m xmlMap) MarshalXML(enc *xml.Encoder, start xml.StartElement) error {
	if start.Name.Local == "" || start.Name.Local == "xmlMap" {
		start.Name.Local = "map"
	}
	if err := enc.EncodeToken(start); err != nil {
		return err
	}
	for key, value := range m {
		elem := xml.StartElement{Name: xml.Name{Local: key}}
		if err := enc.EncodeElement(value, elem); err != nil {
			return err
		}
	}
	return enc.EncodeToken(xml.EndElement{Name: start.Name})
}
