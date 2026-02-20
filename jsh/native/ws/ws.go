package ws

import (
	"fmt"

	"github.com/dop251/goja"
	"github.com/gorilla/websocket"
	"github.com/machbase/neo-server/v8/jsh/engine"
)

func Module(rt *goja.Runtime, module *goja.Object) {
	// Export native functions
	m := module.Get("exports").(*goja.Object)
	m.Set("NewWebSocket", NewNativeWebSocket)
}

func NewNativeWebSocket(obj *goja.Object, url string, dispatch engine.EventDispatchFunc) *WebSocket {
	return &WebSocket{
		obj: obj,
		url: url,
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
}

func (ws *WebSocket) Connect() error {
	if conn, _, err := websocket.DefaultDialer.Dial(ws.url, nil); err != nil {
		return err
	} else {
		ws.conn = conn
		go ws.run()
		return nil
	}
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

func (ws *WebSocket) run() {
	for {
		typ, message, err := ws.conn.ReadMessage()
		if err != nil {
			if !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				ws.emit("close", err)
			} else {
				ws.emit("error", err)
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
		ws.emit("message", data)
	}
}
