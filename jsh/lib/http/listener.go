package http

import (
	"errors"
	"net"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/machbase/neo-server/v8/jsh/engine"
)

var defaultRouter *gin.Engine

func SetDefaultRouter(router *gin.Engine) {
	defaultRouter = router
}

func DefaultRouter() *gin.Engine {
	return defaultRouter
}

func NewServer(config map[string]any) (Listener, error) {
	base := BaseListener{
		Network: "tcp",
		Address: "",
	}

	var lsnr Listener
	var env *engine.Env
	if network, ok := config["network"]; ok {
		base.Network = network.(string)
	}
	if address, ok := config["address"]; ok {
		base.Address = address.(string)
	}
	if envObj, ok := config["env"]; ok {
		env = envObj.(*engine.Env)
	}

	if base.Address == "" {
		if DefaultRouter() == nil {
			return nil, errors.New("http.NewServer: address is not set")
		}
		base.router = &Router{env: env, ir: DefaultRouter()}
		// Proxy listener that uses the existing http listener
		lsnr = &PListener{BaseListener: base}
	} else {
		// Regular listener that creates its own http listener
		base.router = &Router{env: env, ir: gin.New()}
		lsnr = &RListener{BaseListener: base}
	}

	return lsnr, nil
}

type Listener interface {
	Router() *Router
	All(path string, callback func(*RouterContext))
	Get(path string, callback func(*RouterContext))
	Post(path string, callback func(*RouterContext))
	Put(path string, callback func(*RouterContext))
	Delete(path string, callback func(*RouterContext))
	Static(path string, realPath string) error
	StaticFile(path string, realPath string) error
	LoadHTMLGlob(pattern string) error
	LoadHTMLFiles(paths ...string) error
	Serve(callback func(map[string]any)) error
	Close()
}

type BaseListener struct {
	Network string `json:"network"`
	Address string `json:"address"`

	router *Router `json:"-"`
}

func (l *BaseListener) Router() *Router {
	return l.router
}
func (l *BaseListener) All(path string, callback func(*RouterContext)) {
	l.router.All(path, callback)
}
func (l *BaseListener) Get(path string, callback func(*RouterContext)) {
	l.router.Get(path, callback)
}
func (l *BaseListener) Post(path string, callback func(*RouterContext)) {
	l.router.Post(path, callback)
}
func (l *BaseListener) Put(path string, callback func(*RouterContext)) {
	l.router.Put(path, callback)
}
func (l *BaseListener) Delete(path string, callback func(*RouterContext)) {
	l.router.Delete(path, callback)
}
func (l *BaseListener) Static(path string, realPath string) error {
	return l.router.Static(path, realPath)
}
func (l *BaseListener) StaticFile(path string, realPath string) error {
	return l.router.StaticFile(path, realPath)
}
func (l *BaseListener) LoadHTMLGlob(pattern string) error {
	return l.router.LoadHTMLGlob(pattern)
}
func (l *BaseListener) LoadHTMLFiles(paths ...string) error {
	return l.router.LoadHTMLFiles(paths...)
}

type PListener struct {
	BaseListener
}

func (l *PListener) Serve(callback func(map[string]any)) error {
	if callback != nil {
		obj := map[string]interface{}{
			"network": l.Network,
			"address": l.Address,
		}
		callback(obj)
	}
	return nil
}

func (l *PListener) Close() {}

type RListener struct {
	BaseListener
	lsnr    net.Listener
	closeCh chan struct{}
}

func (l *RListener) Serve(callback func(map[string]any)) error {
	if lsnr, err := net.Listen(l.Network, l.Address); err != nil {
		return errors.New("http.Listener.Listen: " + err.Error())
	} else {
		l.lsnr = lsnr
	}

	if callback != nil {
		obj := map[string]interface{}{
			"network": l.lsnr.Addr().Network(),
			"address": l.lsnr.Addr().String(),
		}
		callback(obj)
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
	case <-done:
	case <-l.closeCh:
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

	return nil
}

func (l *RListener) Close() {
	if l.closeCh != nil {
		close(l.closeCh)
		l.closeCh = nil
	}
}
