package httpd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/machbase/neo-server/mods/logging"
	"github.com/machbase/neo-server/mods/service/eventbus"
	"github.com/pkg/errors"
)

func (svr *httpd) handleConsoleData(ctx *gin.Context) {
	consoleId := ctx.Param("console_id")
	if len(consoleId) == 0 {
		ctx.String(http.StatusBadRequest, "invalid consoleId")
		return
	}
	// current websocket spec requires pass the token through handshake process
	token := ctx.Query("token")
	claim, err := svr.verifyAccessToken(token)
	if err != nil {
		ctx.String(http.StatusUnauthorized, "unauthorized access")
		return
	}
	conn, err := upgrader.Upgrade(ctx.Writer, ctx.Request, nil)
	if err != nil {
		svr.log.Errorf("console ws upgrade fail %s", err.Error())
		ctx.String(http.StatusBadRequest, err.Error())
		return
	}

	cons := NewConsole(claim.Subject, consoleId, conn)
	go cons.run()
}

type Console struct {
	log       logging.Log
	username  string
	consoleId string
	topic     string
	conn      *websocket.Conn
	connMutex sync.Mutex
	closeOnce sync.Once
}

func NewConsole(username string, consoleId string, conn *websocket.Conn) *Console {
	ret := &Console{
		log:       logging.GetLog(fmt.Sprintf("console-%s-%s", username, consoleId)),
		topic:     fmt.Sprintf("console:%s:%s", username, consoleId),
		username:  username,
		consoleId: consoleId,
		conn:      conn,
	}
	eventbus.Default.SubscribeAsync(ret.topic, ret.sendMessage, true)
	return ret
}

func (cons *Console) Close() {
	cons.closeOnce.Do(func() {
		eventbus.Default.Unsubscribe(cons.topic, cons.sendMessage)
		if cons.conn != nil {
			cons.conn.Close()
		}
	})
}

func (cons *Console) run() {
	defer func() {
		cons.Close()
		if e := recover(); e != nil {
			cons.log.Error("panic recover %s", e)
		}
	}()

	for {
		evt := &eventbus.Event{}
		err := cons.conn.ReadJSON(evt)
		if err != nil {
			if !errors.Is(err, io.EOF) {
				cons.log.Warn("ERR", err.Error())
			}
			cons.connMutex.Lock()
			cons.conn.WriteControl(websocket.CloseMessage, []byte{}, time.Now().Add(200*time.Millisecond))
			cons.connMutex.Unlock()
			return
		}
		switch evt.Type {
		case eventbus.EVT_PING:
			rsp := eventbus.NewPing(evt.Ping.Tick)
			cons.connMutex.Lock()
			cons.conn.WriteJSON(rsp)
			cons.connMutex.Unlock()
		}
	}
}

func (cons *Console) sendMessage(evt *eventbus.Event) {
	cons.connMutex.Lock()
	err := cons.conn.WriteJSON(evt)
	cons.connMutex.Unlock()
	if err != nil {
		cons.log.Warn("ERR", err.Error())
		cons.Close()
	} else {
		if cons.log.TraceEnabled() {
			w := &bytes.Buffer{}
			enc := json.NewEncoder(w)
			enc.Encode(evt)
			cons.log.Trace("NOTI", strings.TrimSpace(w.String()))
		}
	}
}
