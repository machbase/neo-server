package opcua

import (
	"context"
	"errors"
	"io"
	"strings"
	"time"

	js "github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/require"
	"github.com/gopcua/opcua"
	"github.com/gopcua/opcua/ua"
)

func NewModuleLoader(ctx context.Context) require.ModuleLoader {
	return func(r *js.Runtime, module *js.Object) {
		// m = require("@jsh/opcua")
		o := module.Get("exports").(*js.Object)

		o.Set("Client", new_client(ctx, r))
		// MessageSecurityMode
		o.Set("MessageSecurityMode", r.ToValue(map[string]any{
			"None":           ua.MessageSecurityModeNone,
			"Sign":           ua.MessageSecurityModeSign,
			"SignAndEncrypt": ua.MessageSecurityModeSignAndEncrypt,
			"Invalid":        ua.MessageSecurityModeInvalid,
		}))
		// TimestampsToReturn
		o.Set("TimestampsToReturn", r.ToValue(map[string]any{
			"Source":  ua.TimestampsToReturnSource,
			"Server":  ua.TimestampsToReturnServer,
			"Both":    ua.TimestampsToReturnBoth,
			"Neither": ua.TimestampsToReturnNeither,
			"Invalid": ua.TimestampsToReturnInvalid,
		}))
	}
}

func new_client(ctx context.Context, rt *js.Runtime) func(call js.ConstructorCall) *js.Object {
	return func(call js.ConstructorCall) *js.Object {
		if len(call.Arguments) == 0 {
			panic(rt.ToValue("missing arguments"))
		}
		opts := struct {
			Endpoint            string                 `json:"endpoint"`
			ReadRetryInterval   time.Duration          `json:"readRetryInterval"`
			MessageSecurityMode ua.MessageSecurityMode `json:"messageSecurityMode"`
		}{
			MessageSecurityMode: ua.MessageSecurityModeNone,
		}
		if err := rt.ExportTo(call.Arguments[0], &opts); err != nil {
			panic(rt.NewGoError(err))
		}
		if opts.ReadRetryInterval < 100*time.Millisecond {
			opts.ReadRetryInterval = 100 * time.Millisecond
		}

		client, err := opcua.NewClient(opts.Endpoint, opcua.SecurityMode(opts.MessageSecurityMode))
		if err != nil {
			panic(rt.NewGoError(err))
		}

		if err := client.Connect(ctx); err != nil {
			panic(rt.NewGoError(err))
		}

		c := &Client{
			ctx:           ctx,
			rt:            rt,
			client:        client,
			retryInterval: opts.ReadRetryInterval,
			securityMode:  opts.MessageSecurityMode,
		}
		ret := rt.NewObject()
		ret.Set("close", c.Close)
		ret.Set("read", c.Read)
		if cleaner, ok := ctx.(Cleaner); ok {
			tok := cleaner.AddCleanup(func(w io.Writer) {
				if c.client != nil {
					io.WriteString(w, "WARNING: opcua client not closed!!!\n")
					c.Close(js.FunctionCall{})
				}
			})
			c.cancelCleaner = func() {
				cleaner.RemoveCleanup(tok)
			}
		}
		return ret
	}
}

type Cleaner interface {
	AddCleanup(func(io.Writer)) int64
	RemoveCleanup(int64)
}

type Client struct {
	ctx    context.Context
	rt     *js.Runtime
	client *opcua.Client

	retryInterval time.Duration
	securityMode  ua.MessageSecurityMode

	cancelCleaner func()
}

func (c *Client) Close(call js.FunctionCall) js.Value {
	if c.client != nil {
		if err := c.client.Close(context.Background()); err != nil {
			return c.rt.NewGoError(err)
		}
		c.client = nil
	}
	return js.Undefined()
}

func (c *Client) Read(call js.FunctionCall) js.Value {
	if len(call.Arguments) != 1 {
		panic(c.rt.ToValue("missing argument"))
	}
	arg := struct {
		MaxAge             float64               `json:"maxAge"`
		Nodes              []string              `json:"nodes"`
		TimestampsToReturn ua.TimestampsToReturn `json:"timestampsToReturn"`
	}{
		TimestampsToReturn: ua.TimestampsToReturnNeither,
	}
	if err := c.rt.ExportTo(call.Arguments[0], &arg); err != nil {
		panic(c.rt.NewGoError(err))
	}
	if len(arg.Nodes) == 0 {
		panic(c.rt.ToValue("missing nodes"))
	}

	var err error
	var rsp *ua.ReadResponse
	var req = &ua.ReadRequest{
		MaxAge:             arg.MaxAge,
		TimestampsToReturn: arg.TimestampsToReturn,
	}

	for _, n := range arg.Nodes {
		id, err := ua.ParseNodeID(n)
		if err != nil {
			panic(c.rt.NewGoError(err))
		}
		req.NodesToRead = append(req.NodesToRead, &ua.ReadValueID{NodeID: id})
	}

	for {
		rsp, err = c.client.Read(c.ctx, req)
		if err == nil {
			break
		}
		switch {
		case err == io.EOF && c.client.State() != opcua.Closed:
			// has to be retried unless user closed the connection
			time.Sleep(c.retryInterval)
			continue
		case errors.Is(err, ua.StatusBadSessionIDInvalid), // Session is not activated has to be retried. Session will be recreated internally.
			errors.Is(err, ua.StatusBadSessionNotActivated),    // Session is invalid has to be retried. Session will be recreated internally.
			errors.Is(err, ua.StatusBadSecureChannelIDInvalid): // secure channel will be recreated internally.
			time.Sleep(c.retryInterval)
			continue
		default:
			panic(c.rt.NewGoError(err))
		}
	}
	ret := []js.Value{}
	for _, data := range rsp.Results {
		code := ""
		if c, ok := ua.StatusCodes[data.Status]; ok {
			code = c.Name
		}
		val := data.Value.Value()
		typ := strings.TrimPrefix(data.Value.Type().String(), "TypeID")
		ent := map[string]any{
			"status":          uint32(data.Status),
			"statusText":      data.Status.Error(),
			"statusCode":      code,
			"value":           val,
			"type":            typ,
			"sourceTimestamp": data.SourceTimestamp,
			"serverTimestamp": data.ServerTimestamp,
		}
		ret = append(ret, c.rt.ToValue(ent))
	}
	return c.rt.ToValue(ret)
}
