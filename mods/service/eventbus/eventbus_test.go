package eventbus_test

import (
	"sync"
	"testing"
	"time"

	"github.com/machbase/neo-server/v8/mods/service/eventbus"
	"github.com/stretchr/testify/require"
)

func TestEventBus(t *testing.T) {
	var wg sync.WaitGroup

	wg.Add(1)
	fnOne := func(msg string) {
		require.Equal(t, "hello one", msg)
		wg.Done()
	}
	eventbus.Default.Subscribe("test:one", fnOne)
	eventbus.Default.Publish("test:one", "hello one")
	wg.Wait()
	eventbus.Default.Unsubscribe("test:one", fnOne)

	wg.Add(2)
	eventbus.Default.SubscribeAsync("test:async", func(msg string) {
		require.Equal(t, "hello two", msg)
		wg.Done()
	}, false)
	eventbus.Default.Publish("test:async", "hello two")
	eventbus.Default.Publish("test:async", "hello two")
	wg.Wait()

	// Events
	var expect func(*eventbus.Event)
	eventbus.Default.SubscribeAsync("test:event", func(in *eventbus.Event) {
		expect(in)
		wg.Done()
	}, false)

	// PING
	wg.Add(1)
	tick := time.Now()
	expect = func(in *eventbus.Event) {
		require.Equal(t, eventbus.EVT_PING, in.Type)
		require.Equal(t, tick.UnixNano(), in.Ping.Tick)
	}
	eventbus.PublishPing("test:event", tick)
	wg.Wait()

	// LOG
	wg.Add(1)
	expect = func(in *eventbus.Event) {
		require.Equal(t, eventbus.EVT_LOG, in.Type)
		require.Equal(t, "INFO", in.Log.Level)
		require.Equal(t, "hello world", in.Log.Message)
	}
	eventbus.PublishLog("test:event", "INFO", "hello world")
	wg.Wait()

	wg.Add(1)
	expect = func(in *eventbus.Event) {
		require.Equal(t, eventbus.EVT_LOG, in.Type)
		require.Equal(t, "INFO", in.Log.Level)
		require.Equal(t, "task#1", in.Log.Task)
		require.Equal(t, "hello world", in.Log.Message)
	}
	eventbus.PublishLogTask("test:event", "INFO", "task#1", "hello world")
	wg.Wait()

}
