package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/machbase/neo-server/v8/mods/eventbus"
	"github.com/machbase/neo-server/v8/mods/logging"
	"github.com/stretchr/testify/require"
)

func newTestWebsocketPair(t *testing.T) (*websocket.Conn, *websocket.Conn) {
	t.Helper()

	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	serverConnCh := make(chan *websocket.Conn, 1)
	errCh := make(chan error, 1)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			errCh <- err
			return
		}
		serverConnCh <- conn
	}))
	t.Cleanup(ts.Close)

	clientConn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(ts.URL, "http"), nil)
	require.NoError(t, err)

	var serverConn *websocket.Conn
	select {
	case err := <-errCh:
		require.NoError(t, err)
	case serverConn = <-serverConnCh:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for websocket upgrade")
	}

	t.Cleanup(func() {
		clientConn.Close()
		if serverConn != nil {
			serverConn.Close()
		}
	})

	return clientConn, serverConn
}

func newTestWebConsole(conn *websocket.Conn) *WebConsole {
	return &WebConsole{
		log:           logging.GetLog("test-web-console"),
		topic:         "console:test:ws",
		conn:          conn,
		messages:      []*eventbus.Event{},
		lastFlushTime: time.Now(),
		flushPeriod:   time.Hour,
	}
}

func TestWsReadWriterRead(t *testing.T) {
	t.Run("continues across frame boundaries", func(t *testing.T) {
		clientConn, serverConn := newTestWebsocketPair(t)
		reader := &WsReadWriter{Conn: clientConn}

		require.NoError(t, serverConn.WriteMessage(websocket.BinaryMessage, []byte("hello")))
		require.NoError(t, serverConn.WriteMessage(websocket.BinaryMessage, []byte("world")))
		require.NoError(t, clientConn.SetReadDeadline(time.Now().Add(time.Second)))

		buf := make([]byte, 3)
		n, err := reader.Read(buf)
		require.NoError(t, err)
		require.Equal(t, "hel", string(buf[:n]))

		buf = make([]byte, 2)
		n, err = reader.Read(buf)
		require.NoError(t, err)
		require.Equal(t, "lo", string(buf[:n]))

		buf = make([]byte, 5)
		n, err = reader.Read(buf)
		require.NoError(t, err)
		require.Equal(t, "world", string(buf[:n]))
	})

	t.Run("propagates next reader errors after frame eof", func(t *testing.T) {
		clientConn, serverConn := newTestWebsocketPair(t)
		reader := &WsReadWriter{Conn: clientConn}

		require.NoError(t, serverConn.WriteMessage(websocket.BinaryMessage, []byte("hello")))
		require.NoError(t, clientConn.SetReadDeadline(time.Now().Add(time.Second)))

		buf := make([]byte, 3)
		n, err := reader.Read(buf)
		require.NoError(t, err)
		require.Equal(t, 3, n)
		require.Equal(t, "hel", string(buf[:n]))

		buf = make([]byte, 2)
		n, err = reader.Read(buf)
		require.NoError(t, err)
		require.Equal(t, 2, n)
		require.Equal(t, "lo", string(buf[:n]))

		require.NoError(t, serverConn.Close())

		buf = make([]byte, 8)
		n, err = reader.Read(buf)
		require.Zero(t, n)
		require.Error(t, err)
	})
}

func TestWsReadWriterWrite(t *testing.T) {
	t.Run("writes binary frames", func(t *testing.T) {
		clientConn, serverConn := newTestWebsocketPair(t)
		writer := &WsReadWriter{Conn: clientConn}

		require.NoError(t, serverConn.SetReadDeadline(time.Now().Add(time.Second)))
		n, err := writer.Write([]byte("payload"))
		require.NoError(t, err)
		require.Equal(t, len("payload"), n)

		msgType, payload, err := serverConn.ReadMessage()
		require.NoError(t, err)
		require.Equal(t, websocket.BinaryMessage, msgType)
		require.Equal(t, []byte("payload"), payload)
	})

	t.Run("returns write errors", func(t *testing.T) {
		clientConn, _ := newTestWebsocketPair(t)
		writer := &WsReadWriter{Conn: clientConn}

		require.NoError(t, clientConn.Close())
		n, err := writer.Write([]byte("payload"))
		require.Zero(t, n)
		require.Error(t, err)
	})
}

func TestWebConsoleSend(t *testing.T) {
	t.Run("coalesces repeated log messages", func(t *testing.T) {
		clientConn, serverConn := newTestWebsocketPair(t)
		cons := newTestWebConsole(clientConn)

		cons.Send(eventbus.NewLog("INFO", "same message"))
		cons.Send(eventbus.NewLog("INFO", "same message"))

		require.Len(t, cons.messages, 1)
		require.Equal(t, 2, cons.messages[0].Log.Repeat)

		cons.lastFlushTime = time.Now().Add(-2 * time.Hour)
		require.NoError(t, serverConn.SetReadDeadline(time.Now().Add(time.Second)))
		cons.Send(nil)

		evt := &eventbus.Event{}
		require.NoError(t, serverConn.ReadJSON(evt))
		require.Equal(t, eventbus.EVT_LOG, evt.Type)
		require.Equal(t, "same message", evt.Log.Message)
		require.Equal(t, 2, evt.Log.Repeat)
		require.Empty(t, cons.messages)
	})

	t.Run("non log events force pending logs to flush", func(t *testing.T) {
		clientConn, serverConn := newTestWebsocketPair(t)
		cons := newTestWebConsole(clientConn)

		cons.Send(eventbus.NewLog("INFO", "pending log"))
		require.NoError(t, serverConn.SetReadDeadline(time.Now().Add(time.Second)))
		cons.Send(&eventbus.Event{Type: eventbus.EVT_OPEN_FILE, OpenFile: &eventbus.OpenFile{Path: "/tmp/result.txt"}})

		first := &eventbus.Event{}
		second := &eventbus.Event{}
		require.NoError(t, serverConn.ReadJSON(first))
		require.NoError(t, serverConn.ReadJSON(second))
		require.Equal(t, eventbus.EVT_LOG, first.Type)
		require.Equal(t, "pending log", first.Log.Message)
		require.Equal(t, eventbus.EVT_OPEN_FILE, second.Type)
		require.Equal(t, "/tmp/result.txt", second.OpenFile.Path)
	})

	t.Run("write failure closes the console", func(t *testing.T) {
		clientConn, _ := newTestWebsocketPair(t)
		cons := newTestWebConsole(clientConn)
		cons.flushPeriod = 0
		cons.lastFlushTime = time.Now().Add(-time.Second)

		require.NoError(t, clientConn.Close())
		cons.Send(eventbus.NewLog("INFO", "will fail"))

		require.True(t, cons.closed.Load())
	})
}
