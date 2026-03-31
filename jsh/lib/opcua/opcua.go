package opcua

import (
	"context"
	_ "embed"
	"errors"
	"io"
	"strings"
	"time"

	"github.com/dop251/goja"
	"github.com/gopcua/opcua"
	"github.com/gopcua/opcua/id"
	"github.com/gopcua/opcua/ua"
)

//go:embed opcua.js
var opcua_js []byte

func Files() map[string][]byte {
	return map[string][]byte{
		"opcua.js": opcua_js,
	}
}

func Module(rt *goja.Runtime, module *goja.Object) {
	// m = require("@jsh/opcua")
	o := module.Get("exports").(*goja.Object)

	o.Set("Client", new_client(rt))
	// BrowseDirection
	o.Set("BrowseDirection", rt.ToValue(map[string]any{
		"Forward": ua.BrowseDirectionForward,
		"Inverse": ua.BrowseDirectionInverse,
		"Both":    ua.BrowseDirectionBoth,
		"Invalid": ua.BrowseDirectionInvalid,
	}))
	// NodeClass
	o.Set("NodeClass", rt.ToValue(map[string]any{
		"Unspecified":   ua.NodeClassUnspecified,
		"Object":        ua.NodeClassObject,
		"Variable":      ua.NodeClassVariable,
		"Method":        ua.NodeClassMethod,
		"ObjectType":    ua.NodeClassObjectType,
		"VariableType":  ua.NodeClassVariableType,
		"ReferenceType": ua.NodeClassReferenceType,
		"DataType":      ua.NodeClassDataType,
		"View":          ua.NodeClassView,
	}))
	// BrowseResultMask
	o.Set("BrowseResultMask", rt.ToValue(map[string]any{
		"None":              ua.BrowseResultMaskNone,
		"ReferenceTypeId":   ua.BrowseResultMaskReferenceTypeID,
		"IsForward":         ua.BrowseResultMaskIsForward,
		"NodeClass":         ua.BrowseResultMaskNodeClass,
		"BrowseName":        ua.BrowseResultMaskBrowseName,
		"DisplayName":       ua.BrowseResultMaskDisplayName,
		"TypeDefinition":    ua.BrowseResultMaskTypeDefinition,
		"All":               ua.BrowseResultMaskAll,
		"ReferenceTypeInfo": ua.BrowseResultMaskReferenceTypeInfo,
		"TargetInfo":        ua.BrowseResultMaskTargetInfo,
	}))
	// MessageSecurityMode
	o.Set("MessageSecurityMode", rt.ToValue(map[string]any{
		"None":           ua.MessageSecurityModeNone,
		"Sign":           ua.MessageSecurityModeSign,
		"SignAndEncrypt": ua.MessageSecurityModeSignAndEncrypt,
		"Invalid":        ua.MessageSecurityModeInvalid,
	}))
	// TimestampsToReturn
	o.Set("TimestampsToReturn", rt.ToValue(map[string]any{
		"Source":  ua.TimestampsToReturnSource,
		"Server":  ua.TimestampsToReturnServer,
		"Both":    ua.TimestampsToReturnBoth,
		"Neither": ua.TimestampsToReturnNeither,
		"Invalid": ua.TimestampsToReturnInvalid,
	}))
}

func new_client(rt *goja.Runtime) func(call goja.ConstructorCall) *goja.Object {
	ctx := context.Background()
	return func(call goja.ConstructorCall) *goja.Object {
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
		ret.Set("write", c.Write)
		ret.Set("browse", c.Browse)
		return ret
	}
}

type Client struct {
	ctx    context.Context
	rt     *goja.Runtime
	client *opcua.Client

	retryInterval time.Duration
	securityMode  ua.MessageSecurityMode

	cancelCleaner func()
}

func (c *Client) Close(call goja.FunctionCall) goja.Value {
	if c.client != nil {
		if err := c.client.Close(context.Background()); err != nil {
			return c.rt.NewGoError(err)
		}
		c.client = nil
	}
	return goja.Undefined()
}

func (c *Client) Read(call goja.FunctionCall) goja.Value {
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
	ret := []goja.Value{}
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
			"sourceTimestamp": data.SourceTimestamp.UnixMilli(),
			"serverTimestamp": data.ServerTimestamp.UnixMilli(),
		}
		ret = append(ret, c.rt.ToValue(ent))
	}
	return c.rt.ToValue(ret)
}

func (c *Client) Browse(call goja.FunctionCall) goja.Value {
	if len(call.Arguments) == 0 {
		panic(c.rt.ToValue("missing arguments"))
	}
	arg := struct {
		Nodes           []string            `json:"nodes"`
		BrowseDirection ua.BrowseDirection  `json:"browseDirection"`
		ReferenceTypeID string              `json:"referenceTypeId"`
		IncludeSubtypes bool                `json:"includeSubtypes"`
		NodeClassMask   uint32              `json:"nodeClassMask"`
		ResultMask      ua.BrowseResultMask `json:"resultMask"`
	}{
		BrowseDirection: ua.BrowseDirectionForward,
		IncludeSubtypes: true,
		ResultMask:      ua.BrowseResultMaskAll,
	}
	if err := c.rt.ExportTo(call.Arguments[0], &arg); err != nil {
		panic(c.rt.NewGoError(err))
	}
	if len(arg.Nodes) == 0 {
		panic(c.rt.ToValue("missing nodes"))
	}

	refTypeID := ua.NewNumericNodeID(0, id.HierarchicalReferences)
	if arg.ReferenceTypeID != "" {
		parsed, err := ua.ParseNodeID(arg.ReferenceTypeID)
		if err != nil {
			panic(c.rt.NewGoError(err))
		}
		refTypeID = parsed
	}

	descs := make([]*ua.BrowseDescription, 0, len(arg.Nodes))
	for _, nodeStr := range arg.Nodes {
		nodeID, err := ua.ParseNodeID(nodeStr)
		if err != nil {
			panic(c.rt.NewGoError(err))
		}
		descs = append(descs, &ua.BrowseDescription{
			NodeID:          nodeID,
			BrowseDirection: arg.BrowseDirection,
			ReferenceTypeID: refTypeID,
			IncludeSubtypes: arg.IncludeSubtypes,
			NodeClassMask:   arg.NodeClassMask,
			ResultMask:      uint32(arg.ResultMask),
		})
	}

	req := &ua.BrowseRequest{
		NodesToBrowse: descs,
	}
	rsp, err := c.client.Browse(c.ctx, req)
	if err != nil {
		panic(c.rt.NewGoError(err))
	}

	ret := make([]goja.Value, 0, len(rsp.Results))
	for _, result := range rsp.Results {
		refs := make([]any, 0, len(result.References))
		for _, ref := range result.References {
			refs = append(refs, map[string]any{
				"referenceTypeId": ref.ReferenceTypeID.String(),
				"isForward":       ref.IsForward,
				"nodeId":          ref.NodeID.NodeID.String(),
				"browseName":      ref.BrowseName.Name,
				"displayName":     ref.DisplayName.Text,
				"nodeClass":       uint32(ref.NodeClass),
				"typeDefinition":  ref.TypeDefinition.NodeID.String(),
			})
		}
		ent := map[string]any{
			"status":     uint32(result.StatusCode),
			"statusText": result.StatusCode.Error(),
			"references": refs,
		}
		ret = append(ret, c.rt.ToValue(ent))
	}
	return c.rt.ToValue(ret)
}

func (c *Client) Write(call goja.FunctionCall) goja.Value {
	if len(call.Arguments) == 0 {
		panic(c.rt.ToValue("missing argument"))
	}
	var req = &ua.WriteRequest{}
	for _, arg := range call.Arguments {
		lst := struct {
			Node  string `json:"node"`
			Value any    `json:"value"`
		}{}
		if err := c.rt.ExportTo(arg, &lst); err != nil {
			panic(c.rt.ToValue(err.Error()))
		}
		nodeID, err := ua.ParseNodeID(lst.Node)
		if err != nil {
			panic(c.rt.ToValue(err.Error()))
		}
		value, err := ua.NewVariant(lst.Value)
		if err != nil {
			panic(c.rt.ToValue(err.Error()))
		}
		nodeToWrite := &ua.WriteValue{
			NodeID:      nodeID,
			AttributeID: ua.AttributeIDValue,
			Value: &ua.DataValue{
				EncodingMask: ua.DataValueValue,
				Value:        value,
			},
		}
		req.NodesToWrite = append(req.NodesToWrite, nodeToWrite)
	}

	rsp, err := c.client.Write(c.ctx, req)

	ret := c.rt.NewObject()
	ret.Set("error", err)
	ret.Set("timestamp", rsp.ResponseHeader.Timestamp.UnixMilli())
	ret.Set("requestHandle", rsp.ResponseHeader.RequestHandle)
	ret.Set("serviceResult", uint32(rsp.ResponseHeader.ServiceResult))
	ret.Set("stringTable", rsp.ResponseHeader.StringTable)
	results := make([]any, len(rsp.Results))
	for i, data := range rsp.Results {
		results[i] = uint32(data)
	}
	ret.Set("results", c.rt.NewArray(results...))
	return ret
}
