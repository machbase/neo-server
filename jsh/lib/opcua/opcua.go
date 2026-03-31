package opcua

import (
	"context"
	_ "embed"
	"encoding/base64"
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

	o.Set("newClient", NewClient)
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

type ClientOptions struct {
	Endpoint            string                 `json:"endpoint"`
	ReadRetryInterval   time.Duration          `json:"readRetryInterval"`
	MessageSecurityMode ua.MessageSecurityMode `json:"messageSecurityMode"`
}

type ReadRequest struct {
	MaxAge             float64               `json:"maxAge"`
	Nodes              []string              `json:"nodes"`
	TimestampsToReturn ua.TimestampsToReturn `json:"timestampsToReturn"`
}

type ReadResult struct {
	Status          uint32 `json:"status"`
	StatusText      string `json:"statusText"`
	StatusCode      string `json:"statusCode"`
	Value           any    `json:"value"`
	Type            string `json:"type"`
	SourceTimestamp int64  `json:"sourceTimestamp"`
	ServerTimestamp int64  `json:"serverTimestamp"`
}

type BrowseRequest struct {
	Nodes            []string            `json:"nodes"`
	BrowseDirection  ua.BrowseDirection  `json:"browseDirection"`
	ReferenceTypeID  string              `json:"referenceTypeId"`
	IncludeSubtypes  bool                `json:"includeSubtypes"`
	NodeClassMask    uint32              `json:"nodeClassMask"`
	ResultMask       ua.BrowseResultMask `json:"resultMask"`
	RequestedMaxRefs uint32              `json:"requestedMaxReferencesPerNode"`
}

type BrowseNextRequest struct {
	ReleaseContinuationPoints bool     `json:"releaseContinuationPoints"`
	ContinuationPoints        []string `json:"continuationPoints"`
}

type BrowseResult struct {
	Status            uint32            `json:"status"`
	StatusText        string            `json:"statusText"`
	ContinuationPoint string            `json:"continuationPoint"`
	References        []BrowseReference `json:"references"`
}

type BrowseReference struct {
	ReferenceTypeId string `json:"referenceTypeId"`
	IsForward       bool   `json:"isForward"`
	NodeId          string `json:"nodeId"`
	BrowseName      string `json:"browseName"`
	DisplayName     string `json:"displayName"`
	NodeClass       uint32 `json:"nodeClass"`
	TypeDefinition  string `json:"typeDefinition"`
}

type ChildrenRequest struct {
	Node          string       `json:"node"`
	NodeClassMask ua.NodeClass `json:"nodeClassMask"`
}

type ChildrenResult struct {
	ReferenceTypeId string `json:"referenceTypeId"`
	IsForward       bool   `json:"isForward"`
	NodeId          string `json:"nodeId"`
	BrowseName      string `json:"browseName"`
	DisplayName     string `json:"displayName"`
	NodeClass       uint32 `json:"nodeClass"`
	TypeDefinition  string `json:"typeDefinition"`
}

type WriteValue struct {
	Node  string `json:"node"`
	Value any    `json:"value"`
}

type WriteResult struct {
	Timestamp     int64    `json:"timestamp"`
	RequestHandle uint32   `json:"requestHandle"`
	ServiceResult uint32   `json:"serviceResult"`
	StringTable   []string `json:"stringTable"`
	Results       []uint32 `json:"results"`
	Error         error    `json:"error"`
}

func NewClient(opts ClientOptions) (*Client, error) {
	if opts.ReadRetryInterval < 100*time.Millisecond {
		opts.ReadRetryInterval = 100 * time.Millisecond
	}

	ctx := context.Background()
	client, err := opcua.NewClient(opts.Endpoint, opcua.SecurityMode(opts.MessageSecurityMode))
	if err != nil {
		return nil, err
	}
	if err := client.Connect(ctx); err != nil {
		return nil, err
	}

	return &Client{
		ctx:           ctx,
		client:        client,
		retryInterval: opts.ReadRetryInterval,
	}, nil
}

type Client struct {
	ctx    context.Context
	client *opcua.Client

	retryInterval time.Duration
}

func (c *Client) Close() error {
	if c.client != nil {
		if err := c.client.Close(context.Background()); err != nil {
			return err
		}
		c.client = nil
	}
	return nil
}

func (c *Client) Read(request ReadRequest) ([]ReadResult, error) {
	if len(request.Nodes) == 0 {
		return nil, errors.New("missing nodes")
	}

	var err error
	var rsp *ua.ReadResponse
	var req = &ua.ReadRequest{
		MaxAge:             request.MaxAge,
		TimestampsToReturn: request.TimestampsToReturn,
	}

	for _, n := range request.Nodes {
		id, err := ua.ParseNodeID(n)
		if err != nil {
			return nil, err
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
			return nil, err
		}
	}
	ret := make([]ReadResult, 0, len(rsp.Results))
	for _, data := range rsp.Results {
		code := ""
		if c, ok := ua.StatusCodes[data.Status]; ok {
			code = c.Name
		}
		val := data.Value.Value()
		typ := strings.TrimPrefix(data.Value.Type().String(), "TypeID")
		ret = append(ret, ReadResult{
			Status:          uint32(data.Status),
			StatusText:      data.Status.Error(),
			StatusCode:      code,
			Value:           val,
			Type:            typ,
			SourceTimestamp: data.SourceTimestamp.UnixMilli(),
			ServerTimestamp: data.ServerTimestamp.UnixMilli(),
		})
	}
	return ret, nil
}

func (c *Client) Write(writes ...WriteValue) (*WriteResult, error) {
	if len(writes) == 0 {
		return nil, errors.New("missing argument")
	}
	var req = &ua.WriteRequest{}
	for _, write := range writes {
		nodeID, err := ua.ParseNodeID(write.Node)
		if err != nil {
			return nil, err
		}
		value, err := ua.NewVariant(write.Value)
		if err != nil {
			return nil, err
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
	ret := &WriteResult{
		Error:   err,
		Results: []uint32{},
	}
	if rsp == nil {
		return ret, nil
	}

	ret.Timestamp = rsp.ResponseHeader.Timestamp.UnixMilli()
	ret.RequestHandle = rsp.ResponseHeader.RequestHandle
	ret.ServiceResult = uint32(rsp.ResponseHeader.ServiceResult)
	ret.StringTable = rsp.ResponseHeader.StringTable
	ret.Results = make([]uint32, len(rsp.Results))
	for i, data := range rsp.Results {
		ret.Results[i] = uint32(data)
	}
	return ret, nil
}

func (c *Client) Browse(request BrowseRequest) ([]BrowseResult, error) {
	if len(request.Nodes) == 0 {
		return nil, errors.New("missing nodes")
	}

	var refTypeID *ua.NodeID
	if request.ReferenceTypeID != "" {
		parsed, err := ua.ParseNodeID(request.ReferenceTypeID)
		if err != nil {
			return nil, err
		}
		refTypeID = parsed
	} else {
		refTypeID = ua.NewNumericNodeID(0, id.References)
	}

	nodes := make([]*ua.BrowseDescription, 0, len(request.Nodes))
	for _, nodeStr := range request.Nodes {
		nodeID, err := ua.ParseNodeID(nodeStr)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, &ua.BrowseDescription{
			NodeID:          nodeID,
			BrowseDirection: request.BrowseDirection,
			ReferenceTypeID: refTypeID,
			IncludeSubtypes: request.IncludeSubtypes,
			NodeClassMask:   request.NodeClassMask,
			ResultMask:      uint32(request.ResultMask),
		})
	}

	req := &ua.BrowseRequest{
		RequestedMaxReferencesPerNode: request.RequestedMaxRefs,
		NodesToBrowse:                 nodes,
	}
	rsp, err := c.client.Browse(c.ctx, req)
	if err != nil {
		return nil, err
	}

	return toBrowseResults(rsp.Results), nil
}

func (c *Client) BrowseNext(request BrowseNextRequest) ([]BrowseResult, error) {
	if len(request.ContinuationPoints) == 0 {
		return nil, errors.New("missing continuation points")
	}

	continuationPoints := make([][]byte, 0, len(request.ContinuationPoints))
	for _, point := range request.ContinuationPoints {
		decoded, err := decodeContinuationPoint(point)
		if err != nil {
			return nil, err
		}
		continuationPoints = append(continuationPoints, decoded)
	}

	rsp, err := c.client.BrowseNext(c.ctx, &ua.BrowseNextRequest{
		ReleaseContinuationPoints: request.ReleaseContinuationPoints,
		ContinuationPoints:        continuationPoints,
	})
	if err != nil {
		return nil, err
	}

	return toBrowseResults(rsp.Results), nil
}

func toBrowseResults(results []*ua.BrowseResult) []BrowseResult {
	ret := make([]BrowseResult, 0, len(results))
	for _, result := range results {
		refs := make([]BrowseReference, 0, len(result.References))
		for _, ref := range result.References {
			var nodeIDStr string
			if ref.NodeID != nil && ref.NodeID.NodeID != nil {
				nodeIDStr = ref.NodeID.NodeID.String()
			}
			var browseName string
			if ref.BrowseName != nil {
				browseName = ref.BrowseName.Name
			}
			var displayName string
			if ref.DisplayName != nil {
				displayName = ref.DisplayName.Text
			}
			var typeDefStr string
			if ref.TypeDefinition != nil && ref.TypeDefinition.NodeID != nil {
				typeDefStr = ref.TypeDefinition.NodeID.String()
			}
			refs = append(refs, BrowseReference{
				ReferenceTypeId: ref.ReferenceTypeID.String(),
				IsForward:       ref.IsForward,
				NodeId:          nodeIDStr,
				BrowseName:      browseName,
				DisplayName:     displayName,
				NodeClass:       uint32(ref.NodeClass),
				TypeDefinition:  typeDefStr,
			})
		}
		ret = append(ret, BrowseResult{
			Status:            uint32(result.StatusCode),
			StatusText:        result.StatusCode.Error(),
			ContinuationPoint: encodeContinuationPoint(result.ContinuationPoint),
			References:        refs,
		})
	}
	return ret
}

func encodeContinuationPoint(point []byte) string {
	if len(point) == 0 {
		return ""
	}
	return base64.StdEncoding.EncodeToString(point)
}

func decodeContinuationPoint(point string) ([]byte, error) {
	if point == "" {
		return nil, errors.New("missing continuation point")
	}
	decoded, err := base64.StdEncoding.DecodeString(point)
	if err != nil {
		return nil, err
	}
	return decoded, nil
}

func (c *Client) Children(request ChildrenRequest) ([]ChildrenResult, error) {
	if request.Node == "" {
		return nil, errors.New("missing node")
	}

	nodeID, err := ua.ParseNodeID(request.Node)
	if err != nil {
		return nil, err
	}

	refs, err := c.client.Node(nodeID).References(c.ctx, id.HierarchicalReferences, ua.BrowseDirectionForward, request.NodeClassMask, true)
	if err != nil {
		return nil, err
	}

	ret := make([]ChildrenResult, 0, len(refs))
	for _, ref := range refs {
		var nodeIDStr string
		if ref.NodeID != nil && ref.NodeID.NodeID != nil {
			nodeIDStr = ref.NodeID.NodeID.String()
		}
		var browseName string
		if ref.BrowseName != nil {
			browseName = ref.BrowseName.Name
		}
		var displayName string
		if ref.DisplayName != nil {
			displayName = ref.DisplayName.Text
		}
		var typeDefStr string
		if ref.TypeDefinition != nil && ref.TypeDefinition.NodeID != nil {
			typeDefStr = ref.TypeDefinition.NodeID.String()
		}
		ret = append(ret, ChildrenResult{
			ReferenceTypeId: ref.ReferenceTypeID.String(),
			IsForward:       ref.IsForward,
			NodeId:          nodeIDStr,
			BrowseName:      browseName,
			DisplayName:     displayName,
			NodeClass:       uint32(ref.NodeClass),
			TypeDefinition:  typeDefStr,
		})
	}
	return ret, nil
}
