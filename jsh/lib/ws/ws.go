package ws

import (
	"context"
	_ "embed"
	"fmt"
	"net/http"
	"sync"

	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/eventloop"
	"github.com/gorilla/websocket"
	"github.com/machbase/neo-server/v8/jsh/engine"
)

//go:embed ws.js
var ws_js []byte

func Files() map[string][]byte {
	return map[string][]byte{
		"ws.js": ws_js,
	}
}

func Module(_ context.Context, rt *goja.Runtime, module *goja.Object) {
	// Export native functions
	m := module.Get("exports").(*goja.Object)
	m.Set("NewWebSocket", NewNativeWebSocket)
	m.Set("BindWebSocket", BindWebSocket)
	m.Set("CreateUpgrader", NewUpgrader)
	m.Set("IsWebSocketUpgrade", IsWebSocketUpgrade)
}

func NewNativeWebSocket(obj *goja.Object, url string, protocols []string, dispatch engine.EventDispatchFunc) *WebSocket {
	return &WebSocket{
		obj:       obj,
		url:       url,
		protocols: protocols,
		emit: func(event string, data any) {
			dispatch(obj, event, data)
		},
	}
}

type WebSocket struct {
	obj  *goja.Object
	url  string
	emit func(event string, data any)
	conn *websocket.Conn

	protocols []string
	loop      *eventloop.EventLoop

	runOnce sync.Once

	onMessage func(map[string]any)
	onClose   func(any)
	onError   func(any)
}

func NewAcceptedWebSocket(conn *websocket.Conn, loop *eventloop.EventLoop) *WebSocket {
	return &WebSocket{conn: conn, loop: loop}
}

func BindWebSocket(obj *goja.Object, raw *WebSocket, dispatch engine.EventDispatchFunc) *WebSocket {
	raw.obj = obj
	raw.emit = func(event string, data any) {
		dispatch(obj, event, data)
	}
	raw.startReadLoop()
	return raw
}

func (ws *WebSocket) Bind(obj *goja.Object, dispatch engine.EventDispatchFunc) *WebSocket {
	ws.obj = obj
	ws.emit = func(event string, data any) {
		dispatch(obj, event, data)
	}
	ws.startReadLoop()
	return ws
}

func (ws *WebSocket) OnMessage(callback func(map[string]any)) {
	ws.onMessage = callback
}

func (ws *WebSocket) OnClose(callback func(any)) {
	ws.onClose = callback
}

func (ws *WebSocket) OnError(callback func(any)) {
	ws.onError = callback
}

func (ws *WebSocket) Start() {
	ws.startReadLoop()
}

func (ws *WebSocket) RunDirect() {
	ws.run()
}

func NewUpgrader() *websocket.Upgrader {
	return &websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
}

func IsWebSocketUpgrade(r *http.Request) bool {
	return websocket.IsWebSocketUpgrade(r)
}

func (ws *WebSocket) Connect() error {
	headers := http.Header{}
	if len(ws.protocols) > 0 {
		headers.Set("Sec-WebSocket-Protocol", joinProtocols(ws.protocols))
	}
	if conn, _, err := websocket.DefaultDialer.Dial(ws.url, headers); err != nil {
		return err
	} else {
		ws.conn = conn
		ws.startReadLoop()
		return nil
	}
}

func joinProtocols(protocols []string) string {
	if len(protocols) == 0 {
		return ""
	}
	result := protocols[0]
	for i := 1; i < len(protocols); i++ {
		result += ", " + protocols[i]
	}
	return result
}

func (ws *WebSocket) Close() error {
	if ws.conn == nil {
		return nil
	}
	return ws.conn.Close()
}

func (ws *WebSocket) Send(typ int, message any) error {
	if ws.conn == nil {
		return fmt.Errorf("websocket is not open")
	}
	switch val := message.(type) {
	case string:
		return ws.conn.WriteMessage(typ, []byte(val))
	case []byte:
		return ws.conn.WriteMessage(typ, val)
	default:
		return fmt.Errorf("unsupported message type: %T", val)
	}
}

func (ws *WebSocket) Protocol() string {
	if ws.conn == nil {
		return ""
	}
	return ws.conn.Subprotocol()
}

func (ws *WebSocket) startReadLoop() {
	ws.runOnce.Do(func() {
		go ws.run()
	})
}

func (ws *WebSocket) run() {
	for {
		typ, message, err := ws.conn.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway, websocket.CloseNoStatusReceived) {
				payload := map[string]any{"error": err.Error()}
				if ws.onClose != nil {
					ws.dispatchCallback(func() { ws.onClose(payload) })
				} else if ws.emit != nil {
					ws.emit("close", payload)
				}
			} else {
				payload := map[string]any{"error": err.Error()}
				if ws.onError != nil {
					ws.dispatchCallback(func() { ws.onError(err) })
				} else if ws.emit != nil {
					ws.emit("error", err)
				}
				if ws.onClose != nil {
					ws.dispatchCallback(func() { ws.onClose(payload) })
				} else if ws.emit != nil {
					ws.emit("close", payload)
				}
			}
			return
		}
		data := map[string]any{
			"type": typ,
		}
		if typ == websocket.TextMessage {
			data["data"] = string(message)
		} else {
			data["data"] = message
		}
		if ws.onMessage != nil {
			ws.dispatchCallback(func() { ws.onMessage(data) })
		} else if ws.emit != nil {
			ws.emit("message", data)
		}
	}
}

func (ws *WebSocket) dispatchCallback(callback func()) {
	if ws.loop == nil {
		callback()
		return
	}
	done := make(chan struct{})
	if ok := ws.loop.RunOnLoop(func(vm *goja.Runtime) {
		defer close(done)
		callback()
	}); !ok {
		return
	}
	<-done
}
