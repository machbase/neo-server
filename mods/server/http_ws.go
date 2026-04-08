package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"reflect"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/machbase/neo-server/v8/jsh/service"
	"github.com/machbase/neo-server/v8/mods/eventbus"
	"github.com/machbase/neo-server/v8/mods/logging"
)

type WebConsoleProcessor interface {
	Process(ctx context.Context, line string)
	Input(line string)
	Control(ctrl string)
}

type WebConsole struct {
	log       logging.Log
	username  string
	consoleId string
	topic     string
	conn      *websocket.Conn
	connMutex sync.Mutex
	closeOnce sync.Once
	closed    bool

	messages      []*eventbus.Event
	lastFlushTime time.Time
	flushPeriod   time.Duration
	processor     WebConsoleProcessor
	rpcController *service.Controller
}

type webConsoleRpcNotifier struct {
	cons *WebConsole
}

func (n *webConsoleRpcNotifier) NotifyJsonRpc(session string, payload map[string]any) error {
	if n == nil || n.cons == nil {
		return nil
	}
	n.cons.connMutex.Lock()
	defer n.cons.connMutex.Unlock()
	return n.cons.conn.WriteJSON(map[string]any{
		"type":    eventbus.EVT_RPC_RSP,
		"session": session,
		"rpc":     payload,
	})
}

func NewWebConsole(username string, consoleId string, conn *websocket.Conn, rpcController *service.Controller) *WebConsole {
	if rpcController == nil {
		rpcController = defaultJsonRpcController
	}
	ret := &WebConsole{
		log:           logging.GetLog(fmt.Sprintf("console-%s-%s", username, consoleId)),
		topic:         fmt.Sprintf("console:%s:%s", username, consoleId),
		username:      username,
		consoleId:     consoleId,
		conn:          conn,
		lastFlushTime: time.Now(),
		flushPeriod:   300 * time.Millisecond,
		rpcController: rpcController,
	}
	eventbus.Default.SubscribeAsync(ret.topic, ret.Send, true)
	return ret
}

func (cons *WebConsole) Run() {
	go cons.readerLoop()
	go cons.flushLoop()
}

func (cons *WebConsole) Close() {
	cons.closeOnce.Do(func() {
		cons.closed = true
		eventbus.Default.Unsubscribe(cons.topic, cons.Send)
		if cons.conn != nil {
			cons.conn.Close()
		}
	})
}

func (cons *WebConsole) readerLoop() {
	defer func() {
		cons.Close()
		if e := recover(); e != nil {
			cons.log.Error("panic recover %s", e)
		}
	}()

	if cons.log.TraceEnabled() {
		cons.log.Trace("websocket: established", cons.conn.RemoteAddr().String())
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	for {
		evt := &eventbus.Event{}
		err := cons.conn.ReadJSON(evt)
		if err != nil {
			if we, ok := err.(*websocket.CloseError); ok {
				cons.log.Trace(we.Error())
			} else if !errors.Is(err, io.EOF) {
				cons.log.Warn("ERR", err.Error())
			}
			cons.connMutex.Lock()
			cons.conn.WriteControl(websocket.CloseMessage, []byte{}, time.Now().Add(200*time.Millisecond))
			cons.connMutex.Unlock()
			return
		}
		switch evt.Type {
		case eventbus.EVT_PING:
			if evt.Ping != nil {
				cons.handlePing(ctx, evt.Ping)
			}
		case eventbus.EVT_RPC_REQ:
			if evt.Rpc != nil {
				go cons.handleRpc(ctx, evt.Session, evt.Rpc)
			}
		}
	}
}

func (cons *WebConsole) flushLoop() {
	ticker := time.NewTicker(cons.flushPeriod)
	for range ticker.C {
		if cons.closed {
			break
		}
		cons.Send(nil)
	}
	ticker.Stop()
}

func (cons *WebConsole) Send(evt *eventbus.Event) {
	shouldAppend := true
	forceFlush := false

	cons.connMutex.Lock()
	defer cons.connMutex.Unlock()

	if evt != nil && evt.Type == eventbus.EVT_LOG &&
		len(cons.messages) > 0 &&
		cons.messages[len(cons.messages)-1].Type == eventbus.EVT_LOG {

		lastLog := cons.messages[len(cons.messages)-1].Log
		if lastLog.Message == evt.Log.Message {
			if lastLog.Repeat == 0 {
				lastLog.Repeat = 1
			}
			lastLog.Repeat += 1
			shouldAppend = false
		}
	} else if evt != nil && evt.Type != eventbus.EVT_LOG {
		forceFlush = true
	}

	if evt != nil && shouldAppend {
		cons.messages = append(cons.messages, evt)
	}

	if !forceFlush && time.Since(cons.lastFlushTime) < cons.flushPeriod {
		// do not flush for now
		return
	}

	for _, msg := range cons.messages {
		err := cons.conn.WriteJSON(msg)
		if err != nil {
			cons.log.Warn("ERR", err.Error())
			cons.Close()
			break
		}
	}
	cons.lastFlushTime = time.Now()
	cons.messages = cons.messages[0:0]
}

func (cons *WebConsole) handlePing(_ context.Context, evt *eventbus.Ping) {
	rsp := eventbus.NewPing(evt.Tick)
	cons.connMutex.Lock()
	cons.conn.WriteJSON(rsp)
	cons.connMutex.Unlock()
}

func (cons *WebConsole) handleRpc(ctx context.Context, session string, evt *eventbus.RPC) {
	rpcCtx := service.WithJsonRpcNotificationWriter(ctx, &webConsoleRpcNotifier{cons: cons})
	rpcCtx = service.WithJsonRpcSession(rpcCtx, session)

	rsp := map[string]any{
		"jsonrpc": "2.0",
		"id":      evt.ID,
	}
	result, rpcErr := cons.rpcController.CallJsonRpc(evt.Method, evt.Params, func(paramType reflect.Type) (reflect.Value, bool) {
		switch {
		case paramType == webConsoleType:
			return reflect.ValueOf(cons), true
		case paramType == contextType:
			return reflect.ValueOf(rpcCtx), true
		default:
			return reflect.Value{}, false
		}
	})
	if rpcErr == nil {
		rsp["result"] = result
	} else {
		code := rpcErr.Code
		message := rpcErr.Message
		if code == -32603 {
			code = -32000
		}
		if rpcErr.Code == -32601 {
			message = "Method not found"
		}
		rsp["error"] = map[string]any{
			"code":    code,
			"message": message,
		}
	}
	cons.connMutex.Lock()
	cons.conn.WriteJSON(map[string]any{
		"type":    eventbus.EVT_RPC_RSP,
		"session": session,
		"rpc":     rsp,
	})
	cons.connMutex.Unlock()
}
