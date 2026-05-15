package engine

import (
	"context"
	"testing"
	"time"

	"github.com/dop251/goja"
	"github.com/stretchr/testify/require"
)

type eventValueProviderStub struct {
	value any
}

func (p eventValueProviderStub) EventValue(vm *goja.Runtime) goja.Value {
	return vm.ToValue(p.value)
}

func TestEventLoopContextHelpers(t *testing.T) {
	loop := NewEventLoop()
	var nilCtx context.Context
	require.Nil(t, EventLoopFromContext(nilCtx))
	require.Same(t, loop, EventLoopFromContext(ContextWithEventLoop(nilCtx, loop)))

	ctx := ContextWithEventLoop(context.Background(), loop)
	require.Same(t, loop, EventLoopFromContext(ctx))
	require.Same(t, loop, EventLoopFromContext(ContextWithEventLoop(context.Background(), loop)))
}

func TestDispatchEvent(t *testing.T) {
	loop := NewEventLoop()
	loop.Start()
	t.Cleanup(func() { loop.Stop() })

	objC := make(chan *goja.Object, 1)
	done := make(chan []any, 1)
	require.True(t, loop.RunOnLoop(func(vm *goja.Runtime) {
		obj := vm.NewObject()
		require.NoError(t, obj.Set("emit", func(call goja.FunctionCall) goja.Value {
			args := make([]any, len(call.Arguments))
			for i, arg := range call.Arguments {
				args[i] = arg.Export()
			}
			done <- args
			return goja.Undefined()
		}))
		objC <- obj
	}))

	var obj *goja.Object
	select {
	case obj = <-objC:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event object")
	}

	require.True(t, dispatchEvent(loop)(obj, "ready", eventValueProviderStub{value: "provided"}, "plain"))
	select {
	case got := <-done:
		require.Equal(t, []any{"ready", "provided", "plain"}, got)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for dispatched event")
	}

	loop.Stop()
}
