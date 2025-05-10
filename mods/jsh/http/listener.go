package http

import (
	"context"
	"net"
	"net/http"
	"os"

	js "github.com/dop251/goja"
	"github.com/gin-gonic/gin"
	"github.com/machbase/neo-server/v8/mods/jsh/builtin"
)

func new_server(ctx context.Context, rt *js.Runtime) func(js.ConstructorCall) *js.Object {
	return func(call js.ConstructorCall) *js.Object {
		base := BaseListener{
			Network: "tcp",
			Address: "",
			rt:      rt,
		}
		if jshCtx, ok := ctx.(builtin.JshContext); ok {
			base.ctx = jshCtx
		} else {
			panic(rt.ToValue("http.Listener: invalid context"))
		}

		var lsnr Listener
		if len(call.Arguments) > 0 {
			if err := rt.ExportTo(call.Arguments[0], &base); err != nil {
				panic(rt.ToValue("http.Listener invalid config: " + err.Error()))
			}
			base.router = &Router{ir: gin.New(), rt: rt}
			lsnr = &RListener{
				BaseListener: base,
			}
		} else {
			base.router = &Router{ir: DefaultRouter(), rt: rt}
			if base.router == nil {
				panic(rt.ToValue("http.Listener: default router does not exist"))
			}
			lsnr = &PListener{
				BaseListener: base,
			}
		}

		ret := rt.NewObject()
		ret.Set("router", lsnr.Router)
		ret.Set("all", lsnr.All)
		ret.Set("get", lsnr.Get)
		ret.Set("post", lsnr.Post)
		ret.Set("put", lsnr.Put)
		ret.Set("delete", lsnr.Delete)
		ret.Set("static", lsnr.Static)
		ret.Set("staticFile", lsnr.StaticFile)
		ret.Set("loadHTMLGlob", lsnr.LoadHTMLGlob)
		ret.Set("loadHTMLFiles", lsnr.LoadHTMLFiles)
		ret.Set("serve", lsnr.Serve)
		ret.Set("close", lsnr.Close)
		return ret
	}
}

type Listener interface {
	Router(call js.FunctionCall) js.Value
	All(call js.FunctionCall) js.Value
	Get(call js.FunctionCall) js.Value
	Post(call js.FunctionCall) js.Value
	Put(call js.FunctionCall) js.Value
	Delete(call js.FunctionCall) js.Value
	Static(call js.FunctionCall) js.Value
	StaticFile(call js.FunctionCall) js.Value
	LoadHTMLGlob(call js.FunctionCall) js.Value
	LoadHTMLFiles(call js.FunctionCall) js.Value
	Serve(call js.FunctionCall) js.Value
	Close(call js.FunctionCall) js.Value
}

type BaseListener struct {
	Network string `json:"network"`
	Address string `json:"address"`

	ctx    builtin.JshContext `json:"-"`
	rt     *js.Runtime        `json:"-"`
	router *Router            `json:"-"`
}

func (l *BaseListener) Router(call js.FunctionCall) js.Value {
	if l.router.obj != nil {
		return l.router.obj
	}

	obj := l.rt.NewObject()
	obj.Set("all", l.router.All)
	obj.Set("get", l.router.Get)
	obj.Set("post", l.router.Post)
	obj.Set("put", l.router.Put)
	obj.Set("delete", l.router.Delete)
	obj.Set("static", l.router.Static)
	obj.Set("staticFile", l.router.StaticFile)
	obj.Set("loadHTMLGlob", l.router.LoadHTMLGlob)
	obj.Set("loadHTMLFiles", l.router.LoadHTMLFiles)
	l.router.obj = obj

	return l.router.obj
}

func (l *BaseListener) All(call js.FunctionCall) js.Value        { return l.router.All(call) }
func (l *BaseListener) Get(call js.FunctionCall) js.Value        { return l.router.Get(call) }
func (l *BaseListener) Post(call js.FunctionCall) js.Value       { return l.router.Post(call) }
func (l *BaseListener) Put(call js.FunctionCall) js.Value        { return l.router.Put(call) }
func (l *BaseListener) Delete(call js.FunctionCall) js.Value     { return l.router.Delete(call) }
func (l *BaseListener) Static(call js.FunctionCall) js.Value     { return l.router.Static(call) }
func (l *BaseListener) StaticFile(call js.FunctionCall) js.Value { return l.router.StaticFile(call) }

func (l *BaseListener) LoadHTMLGlob(call js.FunctionCall) js.Value {
	return l.router.LoadHTMLGlob(call)
}

func (l *BaseListener) LoadHTMLFiles(call js.FunctionCall) js.Value {
	return l.router.LoadHTMLFiles(call)
}

type PListener struct {
	BaseListener
}

func (l *PListener) Serve(call js.FunctionCall) js.Value {
	return js.Undefined()
}
func (l *PListener) Close(call js.FunctionCall) js.Value {
	return js.Undefined()
}

type RListener struct {
	BaseListener
	lsnr    net.Listener
	closeCh chan struct{}
}

func (l *RListener) Serve(call js.FunctionCall) js.Value {
	if lsnr, err := net.Listen(l.Network, l.Address); err != nil {
		panic(l.rt.ToValue("http.Listener.Listen: " + err.Error()))
	} else {
		l.lsnr = lsnr
	}

	var callback js.Callable
	if len(call.Arguments) > 0 {
		if err := l.rt.ExportTo(call.Arguments[0], &callback); err != nil {
			panic(l.rt.ToValue("http.Listener.Listen: invalid callback " + err.Error()))
		}
	}
	if callback != nil {
		obj := l.rt.NewObject()
		obj.Set("network", l.lsnr.Addr().Network())
		obj.Set("address", l.lsnr.Addr().String())

		if _, err := callback(js.Undefined(), obj); err != nil {
			panic(l.rt.ToValue("http.Listener.Listen: callback error " + err.Error()))
		}
	}
	svr := &http.Server{}
	svr.Handler = l.router.ir

	done := make(chan struct{})
	go func() {
		defer close(done)
		svr.Serve(l.lsnr)
	}()

	l.closeCh = make(chan struct{})
	select {
	case <-l.ctx.Done():
	case <-done:
	case <-l.closeCh:
	case <-l.ctx.Signal():
	}

	l.lsnr.Close()
	l.lsnr = nil
	svr.Close()

	if l.Network == "unix" {
		os.Remove(l.Address)
	}
	if l.closeCh != nil {
		close(l.closeCh)
		l.closeCh = nil
	}

	return js.Undefined()
}

func (l *RListener) Close(call js.FunctionCall) js.Value {
	if l.closeCh != nil {
		close(l.closeCh)
		l.closeCh = nil
	}
	return js.Undefined()
}
