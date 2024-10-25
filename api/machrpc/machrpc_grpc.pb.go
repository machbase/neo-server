// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.5.1
// - protoc             v5.28.2
// source: machrpc.proto

package machrpc

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.64.0 or later.
const _ = grpc.SupportPackageIsVersion9

const (
	Machbase_Conn_FullMethodName      = "/machrpc.Machbase/Conn"
	Machbase_ConnClose_FullMethodName = "/machrpc.Machbase/ConnClose"
	Machbase_Ping_FullMethodName      = "/machrpc.Machbase/Ping"
	Machbase_Exec_FullMethodName      = "/machrpc.Machbase/Exec"
	Machbase_QueryRow_FullMethodName  = "/machrpc.Machbase/QueryRow"
	Machbase_Query_FullMethodName     = "/machrpc.Machbase/Query"
	Machbase_Columns_FullMethodName   = "/machrpc.Machbase/Columns"
	Machbase_RowsFetch_FullMethodName = "/machrpc.Machbase/RowsFetch"
	Machbase_RowsClose_FullMethodName = "/machrpc.Machbase/RowsClose"
	Machbase_Appender_FullMethodName  = "/machrpc.Machbase/Appender"
	Machbase_Append_FullMethodName    = "/machrpc.Machbase/Append"
	Machbase_Explain_FullMethodName   = "/machrpc.Machbase/Explain"
	Machbase_UserAuth_FullMethodName  = "/machrpc.Machbase/UserAuth"
)

// MachbaseClient is the client API for Machbase service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type MachbaseClient interface {
	Conn(ctx context.Context, in *ConnRequest, opts ...grpc.CallOption) (*ConnResponse, error)
	ConnClose(ctx context.Context, in *ConnCloseRequest, opts ...grpc.CallOption) (*ConnCloseResponse, error)
	Ping(ctx context.Context, in *PingRequest, opts ...grpc.CallOption) (*PingResponse, error)
	Exec(ctx context.Context, in *ExecRequest, opts ...grpc.CallOption) (*ExecResponse, error)
	QueryRow(ctx context.Context, in *QueryRowRequest, opts ...grpc.CallOption) (*QueryRowResponse, error)
	Query(ctx context.Context, in *QueryRequest, opts ...grpc.CallOption) (*QueryResponse, error)
	Columns(ctx context.Context, in *RowsHandle, opts ...grpc.CallOption) (*ColumnsResponse, error)
	RowsFetch(ctx context.Context, in *RowsHandle, opts ...grpc.CallOption) (*RowsFetchResponse, error)
	RowsClose(ctx context.Context, in *RowsHandle, opts ...grpc.CallOption) (*RowsCloseResponse, error)
	Appender(ctx context.Context, in *AppenderRequest, opts ...grpc.CallOption) (*AppenderResponse, error)
	Append(ctx context.Context, opts ...grpc.CallOption) (grpc.ClientStreamingClient[AppendData, AppendDone], error)
	Explain(ctx context.Context, in *ExplainRequest, opts ...grpc.CallOption) (*ExplainResponse, error)
	UserAuth(ctx context.Context, in *UserAuthRequest, opts ...grpc.CallOption) (*UserAuthResponse, error)
}

type machbaseClient struct {
	cc grpc.ClientConnInterface
}

func NewMachbaseClient(cc grpc.ClientConnInterface) MachbaseClient {
	return &machbaseClient{cc}
}

func (c *machbaseClient) Conn(ctx context.Context, in *ConnRequest, opts ...grpc.CallOption) (*ConnResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(ConnResponse)
	err := c.cc.Invoke(ctx, Machbase_Conn_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *machbaseClient) ConnClose(ctx context.Context, in *ConnCloseRequest, opts ...grpc.CallOption) (*ConnCloseResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(ConnCloseResponse)
	err := c.cc.Invoke(ctx, Machbase_ConnClose_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *machbaseClient) Ping(ctx context.Context, in *PingRequest, opts ...grpc.CallOption) (*PingResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(PingResponse)
	err := c.cc.Invoke(ctx, Machbase_Ping_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *machbaseClient) Exec(ctx context.Context, in *ExecRequest, opts ...grpc.CallOption) (*ExecResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(ExecResponse)
	err := c.cc.Invoke(ctx, Machbase_Exec_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *machbaseClient) QueryRow(ctx context.Context, in *QueryRowRequest, opts ...grpc.CallOption) (*QueryRowResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(QueryRowResponse)
	err := c.cc.Invoke(ctx, Machbase_QueryRow_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *machbaseClient) Query(ctx context.Context, in *QueryRequest, opts ...grpc.CallOption) (*QueryResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(QueryResponse)
	err := c.cc.Invoke(ctx, Machbase_Query_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *machbaseClient) Columns(ctx context.Context, in *RowsHandle, opts ...grpc.CallOption) (*ColumnsResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(ColumnsResponse)
	err := c.cc.Invoke(ctx, Machbase_Columns_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *machbaseClient) RowsFetch(ctx context.Context, in *RowsHandle, opts ...grpc.CallOption) (*RowsFetchResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(RowsFetchResponse)
	err := c.cc.Invoke(ctx, Machbase_RowsFetch_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *machbaseClient) RowsClose(ctx context.Context, in *RowsHandle, opts ...grpc.CallOption) (*RowsCloseResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(RowsCloseResponse)
	err := c.cc.Invoke(ctx, Machbase_RowsClose_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *machbaseClient) Appender(ctx context.Context, in *AppenderRequest, opts ...grpc.CallOption) (*AppenderResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(AppenderResponse)
	err := c.cc.Invoke(ctx, Machbase_Appender_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *machbaseClient) Append(ctx context.Context, opts ...grpc.CallOption) (grpc.ClientStreamingClient[AppendData, AppendDone], error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	stream, err := c.cc.NewStream(ctx, &Machbase_ServiceDesc.Streams[0], Machbase_Append_FullMethodName, cOpts...)
	if err != nil {
		return nil, err
	}
	x := &grpc.GenericClientStream[AppendData, AppendDone]{ClientStream: stream}
	return x, nil
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type Machbase_AppendClient = grpc.ClientStreamingClient[AppendData, AppendDone]

func (c *machbaseClient) Explain(ctx context.Context, in *ExplainRequest, opts ...grpc.CallOption) (*ExplainResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(ExplainResponse)
	err := c.cc.Invoke(ctx, Machbase_Explain_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *machbaseClient) UserAuth(ctx context.Context, in *UserAuthRequest, opts ...grpc.CallOption) (*UserAuthResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(UserAuthResponse)
	err := c.cc.Invoke(ctx, Machbase_UserAuth_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// MachbaseServer is the server API for Machbase service.
// All implementations must embed UnimplementedMachbaseServer
// for forward compatibility.
type MachbaseServer interface {
	Conn(context.Context, *ConnRequest) (*ConnResponse, error)
	ConnClose(context.Context, *ConnCloseRequest) (*ConnCloseResponse, error)
	Ping(context.Context, *PingRequest) (*PingResponse, error)
	Exec(context.Context, *ExecRequest) (*ExecResponse, error)
	QueryRow(context.Context, *QueryRowRequest) (*QueryRowResponse, error)
	Query(context.Context, *QueryRequest) (*QueryResponse, error)
	Columns(context.Context, *RowsHandle) (*ColumnsResponse, error)
	RowsFetch(context.Context, *RowsHandle) (*RowsFetchResponse, error)
	RowsClose(context.Context, *RowsHandle) (*RowsCloseResponse, error)
	Appender(context.Context, *AppenderRequest) (*AppenderResponse, error)
	Append(grpc.ClientStreamingServer[AppendData, AppendDone]) error
	Explain(context.Context, *ExplainRequest) (*ExplainResponse, error)
	UserAuth(context.Context, *UserAuthRequest) (*UserAuthResponse, error)
	mustEmbedUnimplementedMachbaseServer()
}

// UnimplementedMachbaseServer must be embedded to have
// forward compatible implementations.
//
// NOTE: this should be embedded by value instead of pointer to avoid a nil
// pointer dereference when methods are called.
type UnimplementedMachbaseServer struct{}

func (UnimplementedMachbaseServer) Conn(context.Context, *ConnRequest) (*ConnResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Conn not implemented")
}
func (UnimplementedMachbaseServer) ConnClose(context.Context, *ConnCloseRequest) (*ConnCloseResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ConnClose not implemented")
}
func (UnimplementedMachbaseServer) Ping(context.Context, *PingRequest) (*PingResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Ping not implemented")
}
func (UnimplementedMachbaseServer) Exec(context.Context, *ExecRequest) (*ExecResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Exec not implemented")
}
func (UnimplementedMachbaseServer) QueryRow(context.Context, *QueryRowRequest) (*QueryRowResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method QueryRow not implemented")
}
func (UnimplementedMachbaseServer) Query(context.Context, *QueryRequest) (*QueryResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Query not implemented")
}
func (UnimplementedMachbaseServer) Columns(context.Context, *RowsHandle) (*ColumnsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Columns not implemented")
}
func (UnimplementedMachbaseServer) RowsFetch(context.Context, *RowsHandle) (*RowsFetchResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method RowsFetch not implemented")
}
func (UnimplementedMachbaseServer) RowsClose(context.Context, *RowsHandle) (*RowsCloseResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method RowsClose not implemented")
}
func (UnimplementedMachbaseServer) Appender(context.Context, *AppenderRequest) (*AppenderResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Appender not implemented")
}
func (UnimplementedMachbaseServer) Append(grpc.ClientStreamingServer[AppendData, AppendDone]) error {
	return status.Errorf(codes.Unimplemented, "method Append not implemented")
}
func (UnimplementedMachbaseServer) Explain(context.Context, *ExplainRequest) (*ExplainResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Explain not implemented")
}
func (UnimplementedMachbaseServer) UserAuth(context.Context, *UserAuthRequest) (*UserAuthResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method UserAuth not implemented")
}
func (UnimplementedMachbaseServer) mustEmbedUnimplementedMachbaseServer() {}
func (UnimplementedMachbaseServer) testEmbeddedByValue()                  {}

// UnsafeMachbaseServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to MachbaseServer will
// result in compilation errors.
type UnsafeMachbaseServer interface {
	mustEmbedUnimplementedMachbaseServer()
}

func RegisterMachbaseServer(s grpc.ServiceRegistrar, srv MachbaseServer) {
	// If the following call pancis, it indicates UnimplementedMachbaseServer was
	// embedded by pointer and is nil.  This will cause panics if an
	// unimplemented method is ever invoked, so we test this at initialization
	// time to prevent it from happening at runtime later due to I/O.
	if t, ok := srv.(interface{ testEmbeddedByValue() }); ok {
		t.testEmbeddedByValue()
	}
	s.RegisterService(&Machbase_ServiceDesc, srv)
}

func _Machbase_Conn_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ConnRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MachbaseServer).Conn(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Machbase_Conn_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MachbaseServer).Conn(ctx, req.(*ConnRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Machbase_ConnClose_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ConnCloseRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MachbaseServer).ConnClose(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Machbase_ConnClose_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MachbaseServer).ConnClose(ctx, req.(*ConnCloseRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Machbase_Ping_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(PingRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MachbaseServer).Ping(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Machbase_Ping_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MachbaseServer).Ping(ctx, req.(*PingRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Machbase_Exec_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ExecRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MachbaseServer).Exec(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Machbase_Exec_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MachbaseServer).Exec(ctx, req.(*ExecRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Machbase_QueryRow_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(QueryRowRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MachbaseServer).QueryRow(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Machbase_QueryRow_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MachbaseServer).QueryRow(ctx, req.(*QueryRowRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Machbase_Query_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(QueryRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MachbaseServer).Query(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Machbase_Query_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MachbaseServer).Query(ctx, req.(*QueryRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Machbase_Columns_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(RowsHandle)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MachbaseServer).Columns(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Machbase_Columns_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MachbaseServer).Columns(ctx, req.(*RowsHandle))
	}
	return interceptor(ctx, in, info, handler)
}

func _Machbase_RowsFetch_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(RowsHandle)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MachbaseServer).RowsFetch(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Machbase_RowsFetch_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MachbaseServer).RowsFetch(ctx, req.(*RowsHandle))
	}
	return interceptor(ctx, in, info, handler)
}

func _Machbase_RowsClose_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(RowsHandle)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MachbaseServer).RowsClose(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Machbase_RowsClose_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MachbaseServer).RowsClose(ctx, req.(*RowsHandle))
	}
	return interceptor(ctx, in, info, handler)
}

func _Machbase_Appender_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(AppenderRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MachbaseServer).Appender(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Machbase_Appender_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MachbaseServer).Appender(ctx, req.(*AppenderRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Machbase_Append_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(MachbaseServer).Append(&grpc.GenericServerStream[AppendData, AppendDone]{ServerStream: stream})
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type Machbase_AppendServer = grpc.ClientStreamingServer[AppendData, AppendDone]

func _Machbase_Explain_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ExplainRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MachbaseServer).Explain(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Machbase_Explain_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MachbaseServer).Explain(ctx, req.(*ExplainRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Machbase_UserAuth_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(UserAuthRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MachbaseServer).UserAuth(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Machbase_UserAuth_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MachbaseServer).UserAuth(ctx, req.(*UserAuthRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// Machbase_ServiceDesc is the grpc.ServiceDesc for Machbase service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var Machbase_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "machrpc.Machbase",
	HandlerType: (*MachbaseServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Conn",
			Handler:    _Machbase_Conn_Handler,
		},
		{
			MethodName: "ConnClose",
			Handler:    _Machbase_ConnClose_Handler,
		},
		{
			MethodName: "Ping",
			Handler:    _Machbase_Ping_Handler,
		},
		{
			MethodName: "Exec",
			Handler:    _Machbase_Exec_Handler,
		},
		{
			MethodName: "QueryRow",
			Handler:    _Machbase_QueryRow_Handler,
		},
		{
			MethodName: "Query",
			Handler:    _Machbase_Query_Handler,
		},
		{
			MethodName: "Columns",
			Handler:    _Machbase_Columns_Handler,
		},
		{
			MethodName: "RowsFetch",
			Handler:    _Machbase_RowsFetch_Handler,
		},
		{
			MethodName: "RowsClose",
			Handler:    _Machbase_RowsClose_Handler,
		},
		{
			MethodName: "Appender",
			Handler:    _Machbase_Appender_Handler,
		},
		{
			MethodName: "Explain",
			Handler:    _Machbase_Explain_Handler,
		},
		{
			MethodName: "UserAuth",
			Handler:    _Machbase_UserAuth_Handler,
		},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "Append",
			Handler:       _Machbase_Append_Handler,
			ClientStreams: true,
		},
	},
	Metadata: "machrpc.proto",
}
