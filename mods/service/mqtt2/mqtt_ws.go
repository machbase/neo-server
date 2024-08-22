package mqtt2

import (
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/mochi-mqtt/server/v2/listeners"
)

type WsListener struct {
	sync.RWMutex
	svr       *mqtt2
	id        string
	addr      string
	log       *slog.Logger
	upgrader  *websocket.Upgrader
	establish listeners.EstablishFn
}

var _ = listeners.Listener((*WsListener)(nil))

func (l *WsListener) ID() string {
	return l.id
}

func (l *WsListener) Address() string {
	return l.addr
}

func (l *WsListener) Protocol() string {
	return "ws"
}

func (l *WsListener) Init(log *slog.Logger) error {
	l.upgrader = &websocket.Upgrader{
		Subprotocols: []string{"mqtt"},
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
	return nil
}
func (l *WsListener) Close(closeClients listeners.CloseFn) {
	l.Lock()
	defer l.Unlock()

	closeClients(l.id)
}

func (l *WsListener) Serve(establish listeners.EstablishFn) {
	l.establish = establish
}

func (l *WsListener) WsHandler(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if r := recover(); r != nil {
			l.log.Warn("panic", "error", r)
		}
	}()
	c, err := l.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer c.Close()

	err = l.establish(l.id, &wsConn{Conn: c.UnderlyingConn(), c: c})
	if err != nil {
		l.log.Warn("mqtt-v2-ws", "error", err)
	}
}

type wsConn struct {
	net.Conn
	c *websocket.Conn

	// reader for the current message (can be nil)
	r io.Reader
}

// Read reads the next span of bytes from the websocket connection and returns the number of bytes read.
func (ws *wsConn) Read(p []byte) (int, error) {
	if ws.r == nil {
		op, r, err := ws.c.NextReader()
		if err != nil {
			return 0, err
		}
		if op != websocket.BinaryMessage {
			err = listeners.ErrInvalidMessage
			return 0, err
		}
		ws.r = r
	}

	var n int
	for {
		// buffer is full, return what we've read so far
		if n == len(p) {
			return n, nil
		}
		br, err := ws.r.Read(p[n:])
		n += br
		if err != nil {
			// when ANY error occurs, we consider this the end of the current message (either because it really is, via
			// io.EOF, or because something bad happened, in which case we want to drop the remainder)
			ws.r = nil

			if errors.Is(err, io.EOF) {
				err = nil
			}
			return n, err
		}
	}
}

// Write writes bytes to the websocket connection.
func (ws *wsConn) Write(p []byte) (int, error) {
	err := ws.c.WriteMessage(websocket.BinaryMessage, p)
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

// Close signals the underlying websocket conn to close.
func (ws *wsConn) Close() error {
	return ws.Conn.Close()
}
