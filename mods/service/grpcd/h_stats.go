package grpcd

import (
	"context"
	"reflect"
	"strconv"
	"sync/atomic"
	"time"

	spi "github.com/machbase/neo-spi"
	"google.golang.org/grpc/stats"
)

type sessionCtx struct {
	context.Context
	Id     string
	values map[any]any
}

type stringer interface {
	String() string
}

func contextName(c context.Context) string {
	if s, ok := c.(stringer); ok {
		return s.String()
	}
	return reflect.TypeOf(c).String()
}

func (c *sessionCtx) String() string {
	return contextName(c.Context) + "(" + c.Id + ")"
}

func (c *sessionCtx) Value(key any) any {
	if key == contextCtxKey {
		return c
	}
	if v, ok := c.values[key]; ok {
		return v
	}
	return c.Context.Value(key)
}

type rowsWrap struct {
	id      string
	rows    spi.Rows
	release func()

	enlistInfo string
	enlistTime time.Time
}

const contextCtxKey = "machrpc-client-context"

var contextIdSerial int64

//// grpc stat handler

var _ stats.Handler = &grpcd{}

func (s *grpcd) TagRPC(ctx context.Context, nfo *stats.RPCTagInfo) context.Context {
	return ctx
}

func (s *grpcd) HandleRPC(ctx context.Context, stat stats.RPCStats) {
}

func (s *grpcd) TagConn(ctx context.Context, nfo *stats.ConnTagInfo) context.Context {
	id := strconv.FormatInt(atomic.AddInt64(&contextIdSerial, 1), 10)
	ctx = &sessionCtx{Context: ctx, Id: id}
	s.ctxMap.Set(id, ctx)
	return ctx
}

func (s *grpcd) HandleConn(ctx context.Context, stat stats.ConnStats) {
	if sessCtx, ok := ctx.(*sessionCtx); ok {
		switch stat.(type) {
		case *stats.ConnBegin:
			// fmt.Printf("get connBegin: %v\n", sessCtx.Id)
		case *stats.ConnEnd:
			s.ctxMap.RemoveCb(sessCtx.Id, func(key string, v interface{}, exists bool) bool {
				// fmt.Printf("get connEnd: %v\n", sessCtx.Id)
				return true
			})
		}
	}
}
