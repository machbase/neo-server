package engine

import (
	"context"

	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/eventloop"
)

func NewEventLoop(opts ...eventloop.Option) *eventloop.EventLoop {
	return eventloop.NewEventLoop(opts...)
}

type eventLoopContextKey struct{}

func ContextWithEventLoop(ctx context.Context, loop *eventloop.EventLoop) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, eventLoopContextKey{}, loop)
}

func EventLoopFromContext(ctx context.Context) *eventloop.EventLoop {
	if ctx == nil {
		return nil
	}
	loop, _ := ctx.Value(eventLoopContextKey{}).(*eventloop.EventLoop)
	return loop
}

type EventValueProvider interface {
	EventValue(vm *goja.Runtime) goja.Value
}

// EventDispatchFunc
// returns false if the event loop is already terminated.
type EventDispatchFunc func(obj *goja.Object, event string, args ...any) bool

func dispatchEvent(loop *eventloop.EventLoop) EventDispatchFunc {
	return func(obj *goja.Object, event string, args ...any) bool {
		return loop.RunOnLoop(func(vm *goja.Runtime) {
			values := make([]goja.Value, len(args))
			for i, a := range args {
				if provider, ok := a.(EventValueProvider); ok {
					values[i] = provider.EventValue(vm)
				} else {
					values[i] = vm.ToValue(a)
				}
			}
			if emit, ok := obj.Get("emit").Export().(func(goja.FunctionCall) goja.Value); ok {
				emit(goja.FunctionCall{
					This:      obj,
					Arguments: append([]goja.Value{vm.ToValue(event)}, values...),
				})
			}
		})
	}
}
