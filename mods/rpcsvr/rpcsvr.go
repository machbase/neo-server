package rpcsvr

import (
	"context"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"sync/atomic"
	"time"

	mach "github.com/machbase/dbms-mach-go"
	"github.com/machbase/dbms-mach-go/machrpc"
	"github.com/machbase/dbms-mach-go/pbconv"
	cmap "github.com/orcaman/concurrent-map"
	"google.golang.org/grpc/stats"
)

type Config struct {
}

/**
machrpcSvr, err := machrpcsvr.New(&machrpcsvr.Config{})
// gRPC options
grpcOpt := []grpc.ServerOption{ grpc.StatsHandler(machrpcSvr) }
// crete gRpc server
this.grpcd = grpc.NewServer(grpcOpt...)
// register gRpc server
machrpc.RegisterMachbaseServer(this.grpcd, machrpcSvr)
*/

type Server interface {
	stats.Handler
	machrpc.MachbaseServer // machrpc server interface
}

func New(conf *Config) (Server, error) {
	return &svr{
		conf:     conf,
		ctxMap:   cmap.New(),
		machbase: mach.New(),
	}, nil
}

type svr struct {
	machrpc.MachbaseServer // machrpc server interface

	conf     *Config
	ctxMap   cmap.ConcurrentMap
	machbase *mach.Database
}

func (s *svr) Start() error {
	return nil
}

func (s *svr) Stop() {

}

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
	rows    *mach.Rows
	release func()
}

const contextCtxKey = "machrpc-client-context"

var contextIdSerial int64

//// grpc stat handler

func (s *svr) TagRPC(ctx context.Context, nfo *stats.RPCTagInfo) context.Context {
	return ctx
}

func (s *svr) HandleRPC(ctx context.Context, stat stats.RPCStats) {
}

func (s *svr) TagConn(ctx context.Context, nfo *stats.ConnTagInfo) context.Context {
	id := strconv.FormatInt(atomic.AddInt64(&contextIdSerial, 1), 10)
	ctx = &sessionCtx{Context: ctx, Id: id}
	s.ctxMap.Set(id, ctx)
	return ctx
}

func (s *svr) HandleConn(ctx context.Context, stat stats.ConnStats) {
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

//// machrpc server handler

func (s *svr) Exec(pctx context.Context, req *machrpc.ExecRequest) (*machrpc.ExecResponse, error) {
	rsp := &machrpc.ExecResponse{}
	tick := time.Now()
	defer func() {
		rsp.Elapse = time.Since(tick).String()
	}()

	params := pbconv.ConvertPbToAny(req.Params)
	if _, err := s.machbase.Exec(req.Sql, params...); err == nil {
		rsp.Success = true
		rsp.Reason = "success"
	} else {
		rsp.Success = false
		rsp.Reason = err.Error()
	}
	return rsp, nil
}

func (s *svr) QueryRow(pctx context.Context, req *machrpc.QueryRowRequest) (*machrpc.QueryRowResponse, error) {
	rsp := &machrpc.QueryRowResponse{}

	tick := time.Now()
	defer func() {
		rsp.Elapse = time.Since(tick).String()
	}()

	// val := pctx.Value(contextCtxKey)
	// ctx, ok := val.(*sessionCtx)
	// if !ok {
	// 	return nil, fmt.Errorf("invlaid session context %T", pctx)
	// }

	params := pbconv.ConvertPbToAny(req.Params)
	row := s.machbase.QueryRow(req.Sql, params...)

	// fmt.Printf("QueryRow : %s  %s   rows: %d\n", ctx.Id, req.Sql, len(row.Values()))

	if row.Err() != nil {
		rsp.Reason = row.Err().Error()
		return rsp, nil
	}

	var err error
	rsp.Success = true
	rsp.Reason = "success"
	rsp.Values, err = pbconv.ConvertAnyToPb(row.Values())
	if err != nil {
		rsp.Success = false
		rsp.Reason = err.Error()
	}

	return rsp, err
}

func (s *svr) Query(pctx context.Context, req *machrpc.QueryRequest) (*machrpc.QueryResponse, error) {
	rsp := &machrpc.QueryResponse{}

	tick := time.Now()
	defer func() {
		rsp.Elapse = time.Since(tick).String()
	}()

	// val := pctx.Value(contextCtxKey)
	// ctx, ok := val.(*sessionCtx)
	// if !ok {
	// 	return nil, fmt.Errorf("invlaid session context %T", pctx)
	// }
	// fmt.Printf("Query : %s %s\n", ctx.Id, req.Sql)

	params := pbconv.ConvertPbToAny(req.Params)
	realRows, err := s.machbase.Query(req.Sql, params...)
	if err != nil {
		rsp.Reason = err.Error()
		return rsp, nil
	}

	handle := strconv.FormatInt(atomic.AddInt64(&contextIdSerial, 1), 10)
	// TODO leak detector
	s.ctxMap.Set(handle, &rowsWrap{
		id:   handle,
		rows: realRows,
		release: func() {
			s.ctxMap.RemoveCb(handle, func(key string, v interface{}, exists bool) bool {
				// fmt.Printf("close rows: %v\n", handle)
				realRows.Close()
				return true
			})
		},
	})

	rsp.Success = true
	rsp.Reason = "success"
	rsp.RowsHandle = &machrpc.RowsHandle{
		Handle: handle,
	}

	return rsp, nil
}

func (s *svr) RowsFetch(ctx context.Context, rows *machrpc.RowsHandle) (*machrpc.RowsFetchResponse, error) {
	rsp := &machrpc.RowsFetchResponse{}
	tick := time.Now()
	defer func() {
		rsp.Elapse = time.Since(tick).String()
	}()

	rowsWrapVal, exists := s.ctxMap.Get(rows.Handle)
	if !exists {
		rsp.Reason = fmt.Sprintf("handle '%s' not found", rows.Handle)
		return rsp, nil
	}
	rowsWrap, ok := rowsWrapVal.(*rowsWrap)
	if !ok {
		rsp.Reason = fmt.Sprintf("handle '%s' is not valid", rows.Handle)
		return rsp, nil
	}

	values, next, err := rowsWrap.rows.Fetch()
	if err != nil {
		rsp.Success = false
		rsp.Reason = err.Error()
		rsp.HasNoRows = !next
		return rsp, nil
	}
	if !next {
		rsp.Success = true
		rsp.Reason = "success"
		rsp.HasNoRows = true
		return rsp, nil
	}

	rsp.Values, err = pbconv.ConvertAnyToPb(values)
	if err != nil {
		rsp.Success = false
		rsp.Reason = err.Error()
		return rsp, nil
	}
	rsp.Success = true
	rsp.Reason = "success"
	return rsp, nil
}

func (s *svr) RowsClose(ctx context.Context, rows *machrpc.RowsHandle) (*machrpc.RowsCloseResponse, error) {
	rsp := &machrpc.RowsCloseResponse{}
	tick := time.Now()
	defer func() {
		rsp.Elapse = time.Since(tick).String()
	}()

	rowsWrapVal, exists := s.ctxMap.Get(rows.Handle)
	if !exists {
		rsp.Reason = fmt.Sprintf("handle '%s' not found", rows.Handle)
		return rsp, nil
	}
	rowsWrap, ok := rowsWrapVal.(*rowsWrap)
	if !ok {
		rsp.Reason = fmt.Sprintf("handle '%s' is not valid", rows.Handle)
		return rsp, nil
	}

	rowsWrap.release()
	rsp.Success = true
	rsp.Reason = "success"
	return rsp, nil
}

func (s *svr) Appender(ctx context.Context, req *machrpc.AppenderRequest) (*machrpc.AppenderResponse, error) {
	rsp := &machrpc.AppenderResponse{}
	tick := time.Now()
	defer func() {
		rsp.Elapse = time.Since(tick).String()
	}()
	realAppender, err := s.machbase.Appender(req.TableName)
	if err != nil {
		rsp.Reason = err.Error()
		return rsp, nil
	}
	handle := strconv.FormatInt(atomic.AddInt64(&contextIdSerial, 1), 10)
	s.ctxMap.Set(handle, &appenderWrap{
		id:       handle,
		appender: realAppender,
		release: func() {
			s.ctxMap.RemoveCb(handle, func(key string, v interface{}, exists bool) bool {
				// fmt.Printf("close appender: %v\n", handle)
				realAppender.Close()
				return true
			})
		},
	})
	rsp.Success = true
	rsp.Reason = "success"
	rsp.Handle = handle
	return rsp, nil
}

type appenderWrap struct {
	id       string
	appender *mach.Appender
	release  func()
}

func (s *svr) Append(stream machrpc.Machbase_AppendServer) error {
	var wrap *appenderWrap
	defer func() {
		if wrap == nil {
			return
		}
		// fmt.Printf("--- release %s\n", wrap.id)
		wrap.release()
	}()

	tick := time.Now()
	for {
		m, err := stream.Recv()
		if err == io.EOF {
			// caution: m is nil
			return stream.SendAndClose(&machrpc.AppendDone{
				Success: true,
				Reason:  "success",
				Elapse:  time.Since(tick).String(),
			})
		} else if err != nil {
			return err
		}

		if wrap == nil {
			appenderWrapVal, exists := s.ctxMap.Get(m.Handle)
			if !exists {
				// fmt.Println("ERR>>", "not found", m.Handle)
				return fmt.Errorf("handle '%s' not found", m.Handle)
			}
			appenderWrap, ok := appenderWrapVal.(*appenderWrap)
			if !ok {
				// fmt.Println("ERR>>", "invalid", m.Handle)
				return fmt.Errorf("handle '%s' is not valid", m.Handle)
			}
			wrap = appenderWrap
		}

		if wrap.id != m.Handle {
			// fmt.Println("ERR>>", "handle changed", m.Handle)
			return fmt.Errorf("not allowed changing handle in a stream")
		}

		values := pbconv.ConvertPbToAny(m.Params)
		err = wrap.appender.Append(values...)
		if err != nil {
			// fmt.Println("ERR>>", "append", err.Error())
			return stream.SendAndClose(&machrpc.AppendDone{
				Success: false,
				Reason:  err.Error(),
				Elapse:  time.Since(tick).String(),
			})
		}
	}
}
