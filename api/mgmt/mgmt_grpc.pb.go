// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.5.1
// - protoc             v5.28.3
// source: mgmt.proto

package mgmt

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
	Management_ListKey_FullMethodName       = "/mgmt.Management/ListKey"
	Management_GenKey_FullMethodName        = "/mgmt.Management/GenKey"
	Management_DelKey_FullMethodName        = "/mgmt.Management/DelKey"
	Management_ServerKey_FullMethodName     = "/mgmt.Management/ServerKey"
	Management_ListSshKey_FullMethodName    = "/mgmt.Management/ListSshKey"
	Management_AddSshKey_FullMethodName     = "/mgmt.Management/AddSshKey"
	Management_DelSshKey_FullMethodName     = "/mgmt.Management/DelSshKey"
	Management_Shutdown_FullMethodName      = "/mgmt.Management/Shutdown"
	Management_ListShell_FullMethodName     = "/mgmt.Management/ListShell"
	Management_AddShell_FullMethodName      = "/mgmt.Management/AddShell"
	Management_DelShell_FullMethodName      = "/mgmt.Management/DelShell"
	Management_ServicePorts_FullMethodName  = "/mgmt.Management/ServicePorts"
	Management_ServerInfo_FullMethodName    = "/mgmt.Management/ServerInfo"
	Management_Sessions_FullMethodName      = "/mgmt.Management/Sessions"
	Management_KillSession_FullMethodName   = "/mgmt.Management/KillSession"
	Management_LimitSession_FullMethodName  = "/mgmt.Management/LimitSession"
	Management_HttpDebugMode_FullMethodName = "/mgmt.Management/HttpDebugMode"
)

// ManagementClient is the client API for Management service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type ManagementClient interface {
	ListKey(ctx context.Context, in *ListKeyRequest, opts ...grpc.CallOption) (*ListKeyResponse, error)
	GenKey(ctx context.Context, in *GenKeyRequest, opts ...grpc.CallOption) (*GenKeyResponse, error)
	DelKey(ctx context.Context, in *DelKeyRequest, opts ...grpc.CallOption) (*DelKeyResponse, error)
	ServerKey(ctx context.Context, in *ServerKeyRequest, opts ...grpc.CallOption) (*ServerKeyResponse, error)
	ListSshKey(ctx context.Context, in *ListSshKeyRequest, opts ...grpc.CallOption) (*ListSshKeyResponse, error)
	AddSshKey(ctx context.Context, in *AddSshKeyRequest, opts ...grpc.CallOption) (*AddSshKeyResponse, error)
	DelSshKey(ctx context.Context, in *DelSshKeyRequest, opts ...grpc.CallOption) (*DelSshKeyResponse, error)
	Shutdown(ctx context.Context, in *ShutdownRequest, opts ...grpc.CallOption) (*ShutdownResponse, error)
	ListShell(ctx context.Context, in *ListShellRequest, opts ...grpc.CallOption) (*ListShellResponse, error)
	AddShell(ctx context.Context, in *AddShellRequest, opts ...grpc.CallOption) (*AddShellResponse, error)
	DelShell(ctx context.Context, in *DelShellRequest, opts ...grpc.CallOption) (*DelShellResponse, error)
	ServicePorts(ctx context.Context, in *ServicePortsRequest, opts ...grpc.CallOption) (*ServicePortsResponse, error)
	ServerInfo(ctx context.Context, in *ServerInfoRequest, opts ...grpc.CallOption) (*ServerInfoResponse, error)
	Sessions(ctx context.Context, in *SessionsRequest, opts ...grpc.CallOption) (*SessionsResponse, error)
	KillSession(ctx context.Context, in *KillSessionRequest, opts ...grpc.CallOption) (*KillSessionResponse, error)
	LimitSession(ctx context.Context, in *LimitSessionRequest, opts ...grpc.CallOption) (*LimitSessionResponse, error)
	HttpDebugMode(ctx context.Context, in *HttpDebugModeRequest, opts ...grpc.CallOption) (*HttpDebugModeResponse, error)
}

type managementClient struct {
	cc grpc.ClientConnInterface
}

func NewManagementClient(cc grpc.ClientConnInterface) ManagementClient {
	return &managementClient{cc}
}

func (c *managementClient) ListKey(ctx context.Context, in *ListKeyRequest, opts ...grpc.CallOption) (*ListKeyResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(ListKeyResponse)
	err := c.cc.Invoke(ctx, Management_ListKey_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *managementClient) GenKey(ctx context.Context, in *GenKeyRequest, opts ...grpc.CallOption) (*GenKeyResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(GenKeyResponse)
	err := c.cc.Invoke(ctx, Management_GenKey_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *managementClient) DelKey(ctx context.Context, in *DelKeyRequest, opts ...grpc.CallOption) (*DelKeyResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(DelKeyResponse)
	err := c.cc.Invoke(ctx, Management_DelKey_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *managementClient) ServerKey(ctx context.Context, in *ServerKeyRequest, opts ...grpc.CallOption) (*ServerKeyResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(ServerKeyResponse)
	err := c.cc.Invoke(ctx, Management_ServerKey_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *managementClient) ListSshKey(ctx context.Context, in *ListSshKeyRequest, opts ...grpc.CallOption) (*ListSshKeyResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(ListSshKeyResponse)
	err := c.cc.Invoke(ctx, Management_ListSshKey_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *managementClient) AddSshKey(ctx context.Context, in *AddSshKeyRequest, opts ...grpc.CallOption) (*AddSshKeyResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(AddSshKeyResponse)
	err := c.cc.Invoke(ctx, Management_AddSshKey_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *managementClient) DelSshKey(ctx context.Context, in *DelSshKeyRequest, opts ...grpc.CallOption) (*DelSshKeyResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(DelSshKeyResponse)
	err := c.cc.Invoke(ctx, Management_DelSshKey_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *managementClient) Shutdown(ctx context.Context, in *ShutdownRequest, opts ...grpc.CallOption) (*ShutdownResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(ShutdownResponse)
	err := c.cc.Invoke(ctx, Management_Shutdown_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *managementClient) ListShell(ctx context.Context, in *ListShellRequest, opts ...grpc.CallOption) (*ListShellResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(ListShellResponse)
	err := c.cc.Invoke(ctx, Management_ListShell_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *managementClient) AddShell(ctx context.Context, in *AddShellRequest, opts ...grpc.CallOption) (*AddShellResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(AddShellResponse)
	err := c.cc.Invoke(ctx, Management_AddShell_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *managementClient) DelShell(ctx context.Context, in *DelShellRequest, opts ...grpc.CallOption) (*DelShellResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(DelShellResponse)
	err := c.cc.Invoke(ctx, Management_DelShell_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *managementClient) ServicePorts(ctx context.Context, in *ServicePortsRequest, opts ...grpc.CallOption) (*ServicePortsResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(ServicePortsResponse)
	err := c.cc.Invoke(ctx, Management_ServicePorts_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *managementClient) ServerInfo(ctx context.Context, in *ServerInfoRequest, opts ...grpc.CallOption) (*ServerInfoResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(ServerInfoResponse)
	err := c.cc.Invoke(ctx, Management_ServerInfo_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *managementClient) Sessions(ctx context.Context, in *SessionsRequest, opts ...grpc.CallOption) (*SessionsResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(SessionsResponse)
	err := c.cc.Invoke(ctx, Management_Sessions_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *managementClient) KillSession(ctx context.Context, in *KillSessionRequest, opts ...grpc.CallOption) (*KillSessionResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(KillSessionResponse)
	err := c.cc.Invoke(ctx, Management_KillSession_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *managementClient) LimitSession(ctx context.Context, in *LimitSessionRequest, opts ...grpc.CallOption) (*LimitSessionResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(LimitSessionResponse)
	err := c.cc.Invoke(ctx, Management_LimitSession_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *managementClient) HttpDebugMode(ctx context.Context, in *HttpDebugModeRequest, opts ...grpc.CallOption) (*HttpDebugModeResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(HttpDebugModeResponse)
	err := c.cc.Invoke(ctx, Management_HttpDebugMode_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ManagementServer is the server API for Management service.
// All implementations must embed UnimplementedManagementServer
// for forward compatibility.
type ManagementServer interface {
	ListKey(context.Context, *ListKeyRequest) (*ListKeyResponse, error)
	GenKey(context.Context, *GenKeyRequest) (*GenKeyResponse, error)
	DelKey(context.Context, *DelKeyRequest) (*DelKeyResponse, error)
	ServerKey(context.Context, *ServerKeyRequest) (*ServerKeyResponse, error)
	ListSshKey(context.Context, *ListSshKeyRequest) (*ListSshKeyResponse, error)
	AddSshKey(context.Context, *AddSshKeyRequest) (*AddSshKeyResponse, error)
	DelSshKey(context.Context, *DelSshKeyRequest) (*DelSshKeyResponse, error)
	Shutdown(context.Context, *ShutdownRequest) (*ShutdownResponse, error)
	ListShell(context.Context, *ListShellRequest) (*ListShellResponse, error)
	AddShell(context.Context, *AddShellRequest) (*AddShellResponse, error)
	DelShell(context.Context, *DelShellRequest) (*DelShellResponse, error)
	ServicePorts(context.Context, *ServicePortsRequest) (*ServicePortsResponse, error)
	ServerInfo(context.Context, *ServerInfoRequest) (*ServerInfoResponse, error)
	Sessions(context.Context, *SessionsRequest) (*SessionsResponse, error)
	KillSession(context.Context, *KillSessionRequest) (*KillSessionResponse, error)
	LimitSession(context.Context, *LimitSessionRequest) (*LimitSessionResponse, error)
	HttpDebugMode(context.Context, *HttpDebugModeRequest) (*HttpDebugModeResponse, error)
	mustEmbedUnimplementedManagementServer()
}

// UnimplementedManagementServer must be embedded to have
// forward compatible implementations.
//
// NOTE: this should be embedded by value instead of pointer to avoid a nil
// pointer dereference when methods are called.
type UnimplementedManagementServer struct{}

func (UnimplementedManagementServer) ListKey(context.Context, *ListKeyRequest) (*ListKeyResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ListKey not implemented")
}
func (UnimplementedManagementServer) GenKey(context.Context, *GenKeyRequest) (*GenKeyResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GenKey not implemented")
}
func (UnimplementedManagementServer) DelKey(context.Context, *DelKeyRequest) (*DelKeyResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method DelKey not implemented")
}
func (UnimplementedManagementServer) ServerKey(context.Context, *ServerKeyRequest) (*ServerKeyResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ServerKey not implemented")
}
func (UnimplementedManagementServer) ListSshKey(context.Context, *ListSshKeyRequest) (*ListSshKeyResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ListSshKey not implemented")
}
func (UnimplementedManagementServer) AddSshKey(context.Context, *AddSshKeyRequest) (*AddSshKeyResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method AddSshKey not implemented")
}
func (UnimplementedManagementServer) DelSshKey(context.Context, *DelSshKeyRequest) (*DelSshKeyResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method DelSshKey not implemented")
}
func (UnimplementedManagementServer) Shutdown(context.Context, *ShutdownRequest) (*ShutdownResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Shutdown not implemented")
}
func (UnimplementedManagementServer) ListShell(context.Context, *ListShellRequest) (*ListShellResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ListShell not implemented")
}
func (UnimplementedManagementServer) AddShell(context.Context, *AddShellRequest) (*AddShellResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method AddShell not implemented")
}
func (UnimplementedManagementServer) DelShell(context.Context, *DelShellRequest) (*DelShellResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method DelShell not implemented")
}
func (UnimplementedManagementServer) ServicePorts(context.Context, *ServicePortsRequest) (*ServicePortsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ServicePorts not implemented")
}
func (UnimplementedManagementServer) ServerInfo(context.Context, *ServerInfoRequest) (*ServerInfoResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ServerInfo not implemented")
}
func (UnimplementedManagementServer) Sessions(context.Context, *SessionsRequest) (*SessionsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Sessions not implemented")
}
func (UnimplementedManagementServer) KillSession(context.Context, *KillSessionRequest) (*KillSessionResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method KillSession not implemented")
}
func (UnimplementedManagementServer) LimitSession(context.Context, *LimitSessionRequest) (*LimitSessionResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method LimitSession not implemented")
}
func (UnimplementedManagementServer) HttpDebugMode(context.Context, *HttpDebugModeRequest) (*HttpDebugModeResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method HttpDebugMode not implemented")
}
func (UnimplementedManagementServer) mustEmbedUnimplementedManagementServer() {}
func (UnimplementedManagementServer) testEmbeddedByValue()                    {}

// UnsafeManagementServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to ManagementServer will
// result in compilation errors.
type UnsafeManagementServer interface {
	mustEmbedUnimplementedManagementServer()
}

func RegisterManagementServer(s grpc.ServiceRegistrar, srv ManagementServer) {
	// If the following call pancis, it indicates UnimplementedManagementServer was
	// embedded by pointer and is nil.  This will cause panics if an
	// unimplemented method is ever invoked, so we test this at initialization
	// time to prevent it from happening at runtime later due to I/O.
	if t, ok := srv.(interface{ testEmbeddedByValue() }); ok {
		t.testEmbeddedByValue()
	}
	s.RegisterService(&Management_ServiceDesc, srv)
}

func _Management_ListKey_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ListKeyRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ManagementServer).ListKey(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Management_ListKey_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ManagementServer).ListKey(ctx, req.(*ListKeyRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Management_GenKey_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GenKeyRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ManagementServer).GenKey(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Management_GenKey_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ManagementServer).GenKey(ctx, req.(*GenKeyRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Management_DelKey_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(DelKeyRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ManagementServer).DelKey(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Management_DelKey_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ManagementServer).DelKey(ctx, req.(*DelKeyRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Management_ServerKey_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ServerKeyRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ManagementServer).ServerKey(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Management_ServerKey_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ManagementServer).ServerKey(ctx, req.(*ServerKeyRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Management_ListSshKey_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ListSshKeyRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ManagementServer).ListSshKey(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Management_ListSshKey_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ManagementServer).ListSshKey(ctx, req.(*ListSshKeyRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Management_AddSshKey_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(AddSshKeyRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ManagementServer).AddSshKey(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Management_AddSshKey_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ManagementServer).AddSshKey(ctx, req.(*AddSshKeyRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Management_DelSshKey_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(DelSshKeyRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ManagementServer).DelSshKey(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Management_DelSshKey_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ManagementServer).DelSshKey(ctx, req.(*DelSshKeyRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Management_Shutdown_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ShutdownRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ManagementServer).Shutdown(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Management_Shutdown_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ManagementServer).Shutdown(ctx, req.(*ShutdownRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Management_ListShell_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ListShellRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ManagementServer).ListShell(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Management_ListShell_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ManagementServer).ListShell(ctx, req.(*ListShellRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Management_AddShell_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(AddShellRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ManagementServer).AddShell(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Management_AddShell_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ManagementServer).AddShell(ctx, req.(*AddShellRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Management_DelShell_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(DelShellRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ManagementServer).DelShell(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Management_DelShell_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ManagementServer).DelShell(ctx, req.(*DelShellRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Management_ServicePorts_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ServicePortsRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ManagementServer).ServicePorts(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Management_ServicePorts_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ManagementServer).ServicePorts(ctx, req.(*ServicePortsRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Management_ServerInfo_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ServerInfoRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ManagementServer).ServerInfo(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Management_ServerInfo_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ManagementServer).ServerInfo(ctx, req.(*ServerInfoRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Management_Sessions_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(SessionsRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ManagementServer).Sessions(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Management_Sessions_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ManagementServer).Sessions(ctx, req.(*SessionsRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Management_KillSession_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(KillSessionRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ManagementServer).KillSession(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Management_KillSession_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ManagementServer).KillSession(ctx, req.(*KillSessionRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Management_LimitSession_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(LimitSessionRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ManagementServer).LimitSession(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Management_LimitSession_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ManagementServer).LimitSession(ctx, req.(*LimitSessionRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Management_HttpDebugMode_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(HttpDebugModeRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ManagementServer).HttpDebugMode(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Management_HttpDebugMode_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ManagementServer).HttpDebugMode(ctx, req.(*HttpDebugModeRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// Management_ServiceDesc is the grpc.ServiceDesc for Management service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var Management_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "mgmt.Management",
	HandlerType: (*ManagementServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "ListKey",
			Handler:    _Management_ListKey_Handler,
		},
		{
			MethodName: "GenKey",
			Handler:    _Management_GenKey_Handler,
		},
		{
			MethodName: "DelKey",
			Handler:    _Management_DelKey_Handler,
		},
		{
			MethodName: "ServerKey",
			Handler:    _Management_ServerKey_Handler,
		},
		{
			MethodName: "ListSshKey",
			Handler:    _Management_ListSshKey_Handler,
		},
		{
			MethodName: "AddSshKey",
			Handler:    _Management_AddSshKey_Handler,
		},
		{
			MethodName: "DelSshKey",
			Handler:    _Management_DelSshKey_Handler,
		},
		{
			MethodName: "Shutdown",
			Handler:    _Management_Shutdown_Handler,
		},
		{
			MethodName: "ListShell",
			Handler:    _Management_ListShell_Handler,
		},
		{
			MethodName: "AddShell",
			Handler:    _Management_AddShell_Handler,
		},
		{
			MethodName: "DelShell",
			Handler:    _Management_DelShell_Handler,
		},
		{
			MethodName: "ServicePorts",
			Handler:    _Management_ServicePorts_Handler,
		},
		{
			MethodName: "ServerInfo",
			Handler:    _Management_ServerInfo_Handler,
		},
		{
			MethodName: "Sessions",
			Handler:    _Management_Sessions_Handler,
		},
		{
			MethodName: "KillSession",
			Handler:    _Management_KillSession_Handler,
		},
		{
			MethodName: "LimitSession",
			Handler:    _Management_LimitSession_Handler,
		},
		{
			MethodName: "HttpDebugMode",
			Handler:    _Management_HttpDebugMode_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "mgmt.proto",
}
