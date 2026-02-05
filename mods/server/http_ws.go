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
	"github.com/machbase/neo-server/v8/mods/eventbus"
	"github.com/machbase/neo-server/v8/mods/logging"
	"github.com/machbase/neo-server/v8/mods/server/chat"
	"github.com/machbase/neo-server/v8/mods/server/cmd"
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
}

func NewWebConsole(username string, consoleId string, conn *websocket.Conn) *WebConsole {
	ret := &WebConsole{
		log:           logging.GetLog(fmt.Sprintf("console-%s-%s", username, consoleId)),
		topic:         fmt.Sprintf("console:%s:%s", username, consoleId),
		username:      username,
		consoleId:     consoleId,
		conn:          conn,
		lastFlushTime: time.Now(),
		flushPeriod:   300 * time.Millisecond,
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
		case eventbus.EVT_MSG:
			if evt.Message != nil {
				go cons.handleMessage(ctx, evt.Session, evt.Message)
			}
		}
	}
}

var wsRpcHandlers = map[string]any{}
var wsRpcHandlersMutex sync.RWMutex

func RegisterJsonRpcHandler(method string, handler any) {
	wsRpcHandlersMutex.Lock()
	defer wsRpcHandlersMutex.Unlock()
	wsRpcHandlers[method] = handler
}

func UnregisterJsonRpcHandler(method string) {
	wsRpcHandlersMutex.Lock()
	defer wsRpcHandlersMutex.Unlock()
	delete(wsRpcHandlers, method)
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
	wsRpcHandlersMutex.RLock()
	handler, ok := wsRpcHandlers[evt.Method]
	wsRpcHandlersMutex.RUnlock()
	rsp := map[string]any{
		"jsonrpc": "2.0",
		"id":      evt.ID,
	}
	if ok {
		// reflection for the handler method signature
		// convert evt.Params to the expected types of handler function.
		var params []reflect.Value
		handlerType := reflect.TypeOf(handler)
		implicitParams := 0
		for i := 0; i < handlerType.NumIn(); i++ {
			paramType := handlerType.In(i)
			var paramValue reflect.Value
			if paramType.String() == "*server.WebConsole" {
				implicitParams++
				paramValue = reflect.ValueOf(cons)
			} else if paramType.String() == "context.Context" {
				implicitParams++
				paramValue = reflect.ValueOf(ctx)
			} else if i-implicitParams < len(evt.Params) {
				paramValue = reflect.ValueOf(evt.Params[i-implicitParams])
			} else {
				paramValue = reflect.Zero(paramType)
			}
			params = append(params, paramValue)
		}
		// call the handler
		resultValues := reflect.ValueOf(handler).Call(params)
		var result interface{}
		var err error
		if len(resultValues) > 0 {
			result = resultValues[0].Interface()
		}
		if len(resultValues) > 1 {
			if !resultValues[1].IsNil() {
				err = resultValues[1].Interface().(error)
			}
		}
		// send response
		if err == nil {
			rsp["result"] = result
		} else {
			rsp["error"] = map[string]any{
				"code":    -32000,
				"message": err.Error(),
			}
		}
	} else {
		rsp["error"] = map[string]any{
			"code":    -32601,
			"message": "Method not found",
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

func init() {
	chat.Init()
	RegisterJsonRpcHandler("llmGetProviders", chat.RpcLLMGetProviders)
	RegisterJsonRpcHandler("llmGetProviderConfig", chat.RpcLLMGetProviderConfig)
	RegisterJsonRpcHandler("llmSetProviderConfig", chat.RpcLLMSetProviderConfig)
	RegisterJsonRpcHandler("llmGetModels", chat.RpcLLMGetModels)
	RegisterJsonRpcHandler("llmAddModels", chat.RpcLLMAddModels)
	RegisterJsonRpcHandler("llmRemoveModels", chat.RpcLLMRemoveModels)
	RegisterJsonRpcHandler("llmListModels", chat.RpcLLMListModels)
}

func (cons *WebConsole) handleMessage(ctx context.Context, session string, msg *eventbus.Message) {
	if msg.Ver != "1.0" {
		eventbus.PublishLog(cons.topic, "ERROR",
			fmt.Sprintf("unsupported msg.ver: %q", msg.Ver))
		return
	}
	switch msg.Type {
	case eventbus.BodyTypeInput:
		if cons.processor != nil {
			if msg.Body == nil || msg.Body.OfInput == nil {
				eventbus.PublishLog(cons.topic, "ERROR",
					"missing input body")
				return
			}
			input := msg.Body.OfInput
			if input.Text != "" {
				cons.processor.Input(input.Text)
			}
			if input.Control != "" {
				cons.processor.Control(input.Control)
			}
		}
	case eventbus.BodyTypeCommand:
		if msg.Body == nil || msg.Body.OfCommand == nil {
			eventbus.PublishLog(cons.topic, "ERROR",
				"missing command body")
			return
		}
		pc := cmd.Config{
			Topic:   cons.topic,
			MsgID:   msg.ID,
			Session: session,
		}
		p := pc.NewProcessor()
		cons.processor = p
		p.Process(ctx, msg.Body.OfCommand.Line)
	case eventbus.BodyTypeQuestion:
		if msg.Body == nil || msg.Body.OfQuestion == nil {
			eventbus.PublishLog(cons.topic, "ERROR",
				"missing question body")
			return
		}
		question := msg.Body.OfQuestion
		dc := chat.DialogConfig{
			Topic:    cons.topic,
			Provider: question.Provider,
			Model:    question.Model,
			MsgID:    msg.ID,
			Session:  session,
		}
		d := dc.NewDialog()
		cons.processor = d
		d.Process(ctx, question.Text)
	default:
		eventbus.PublishLog(cons.topic, "ERROR",
			fmt.Sprintf("invalid message type %s", msg.Type))
	}
}
