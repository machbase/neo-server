package httpd

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/machbase/neo-server/mods/service/eventbus"
	"github.com/stretchr/testify/require"
)

func TestConsoleWs(t *testing.T) {
	w := httptest.NewRecorder()
	s, _, _ := NewMockServer(w)
	defer s.Shutdown()

	err := s.Login("sys", "manager")
	require.Nil(t, err)

	// Convert http://127.0.0.1 to ws://127.0.0.1
	u := "ws" + strings.TrimPrefix(s.URL(), "http")
	u = u + "/web/api/console/1234/data?token=" + s.AccessToken()
	ws, _, err := websocket.DefaultDialer.Dial(u, nil)
	if err != nil {
		t.Logf("Status: %v", w.Code)
		t.Logf("Body: %v", w.Body.String())
		t.Fatalf("%v", err)
	}
	require.Nil(t, err)
	defer ws.Close()

	// PING
	ping := eventbus.NewPingTime(time.Now())
	ws.WriteJSON(ping)

	evt := eventbus.Event{}
	ws.ReadJSON(&evt)
	require.Equal(t, eventbus.EVT_PING, evt.Type)
	require.Equal(t, ping.Ping.Tick, evt.Ping.Tick)

	// LOG
	topic := "console:sys:1234"
	eventbus.PublishLog(topic, "INFO", "test message")

	evt = eventbus.Event{}
	ws.ReadJSON(&evt)
	require.Equal(t, eventbus.EVT_LOG, evt.Type)
	require.Equal(t, "test message", evt.Log.Message)
}

func TestTqlLog(t *testing.T) {
	w := httptest.NewRecorder()
	s, ctx, engine := NewMockServer(w)
	defer s.Shutdown()

	err := s.Login("sys", "manager")
	require.Nil(t, err)

	// Convert http://127.0.0.1 to ws://127.0.0.1
	u := "ws" + strings.TrimPrefix(s.URL(), "http")
	u = u + "/web/api/console/123456/data?token=" + s.AccessToken()
	ws, _, err := websocket.DefaultDialer.Dial(u, nil)
	if err != nil {
		t.Logf("Status: %v", w.Code)
		t.Logf("Body: %v", w.Body.String())
		t.Fatalf("%v", err)
	}
	require.Nil(t, err)
	defer ws.Close()

	expectLines := []string{
		"1 [0]",
		"2 [0.25]",
		"3 [0.5]",
		"4 [0.75]",
		"5 [1]",
	}
	expectCount := len(expectLines)
	wg := sync.WaitGroup{}
	// websocket
	wg.Add(1)
	go func() {
		ping := &eventbus.Ping{Tick: time.Now().UnixNano()}
		pong := &eventbus.Event{}
		err := ws.WriteJSON(eventbus.Event{Type: eventbus.EVT_PING, Ping: ping})
		if err != nil {
			t.Log("ERR write json", err.Error())
		}
		err = ws.ReadJSON(pong)
		if err != nil {
			t.Log("ERR read json", err.Error())
		}
		require.Equal(t, ping.Tick, pong.Ping.Tick)

		for i := 0; i < expectCount; i++ {
			evt := eventbus.Event{}
			err := ws.ReadJSON(&evt)
			require.Nil(t, err, "read websocket failed")
			require.Equal(t, expectLines[i], evt.Log.Message)
		}
		wg.Done()
	}()
	// Tql Log
	wg.Add(1)
	go func() {
		reader := bytes.NewBufferString(`
			FAKE(linspace(0,1,5))
			SCRIPT({
				ctx := import("context")
				ctx.print(ctx.key(), ctx.value())
				ctx.yieldKey(ctx.key(), ctx.value()...)
			})
			CSV(precision(2))
		`)
		ctx.Request, _ = http.NewRequest(http.MethodPost, "/web/api/tql", reader)
		ctx.Request.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.AccessToken()))
		ctx.Request.Header.Set("X-Console-Id", "123456 console-log-level=INFO log-level=ERROR")
		engine.HandleContext(ctx)
		require.Equal(t, 200, w.Result().StatusCode)
		require.Equal(t, strings.Join([]string{"1,0.00", "2,0.25", "3,0.50", "4,0.75", "5,1.00", ""}, "\n"), w.Body.String())
		wg.Done()
	}()
	wg.Wait()
}
