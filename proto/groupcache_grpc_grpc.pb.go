// Code generated by protoc-gen-go-grpc. DO NOT EDIT.

package proto

import (
	context "context"
	groupcachepb "github.com/mailgun/groupcache/groupcachepb"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

// GroupCacheClient is the client API for GroupCache service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type GroupCacheClient interface {
	Get(ctx context.Context, in *groupcachepb.GetRequest, opts ...grpc.CallOption) (*groupcachepb.GetResponse, error)
}

type groupCacheClient struct {
	cc grpc.ClientConnInterface
}

func NewGroupCacheClient(cc grpc.ClientConnInterface) GroupCacheClient {
	return &groupCacheClient{cc}
}

func (c *groupCacheClient) Get(ctx context.Context, in *groupcachepb.GetRequest, opts ...grpc.CallOption) (*groupcachepb.GetResponse, error) {
	out := new(groupcachepb.GetResponse)
	err := c.cc.Invoke(ctx, "/proto.GroupCache/Get", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// GroupCacheServer is the server API for GroupCache service.
// All implementations must embed UnimplementedGroupCacheServer
// for forward compatibility
type GroupCacheServer interface {
	Get(context.Context, *groupcachepb.GetRequest) (*groupcachepb.GetResponse, error)
	mustEmbedUnimplementedGroupCacheServer()
}

// UnimplementedGroupCacheServer must be embedded to have forward compatible implementations.
type UnimplementedGroupCacheServer struct {
}

func (UnimplementedGroupCacheServer) Get(context.Context, *groupcachepb.GetRequest) (*groupcachepb.GetResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Get not implemented")
}
func (UnimplementedGroupCacheServer) mustEmbedUnimplementedGroupCacheServer() {}

// UnsafeGroupCacheServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to GroupCacheServer will
// result in compilation errors.
type UnsafeGroupCacheServer interface {
	mustEmbedUnimplementedGroupCacheServer()
}

func RegisterGroupCacheServer(s grpc.ServiceRegistrar, srv GroupCacheServer) {
	s.RegisterService(&GroupCache_ServiceDesc, srv)
}

func _GroupCache_Get_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(groupcachepb.GetRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(GroupCacheServer).Get(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/proto.GroupCache/Get",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(GroupCacheServer).Get(ctx, req.(*groupcachepb.GetRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// GroupCache_ServiceDesc is the grpc.ServiceDesc for GroupCache service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var GroupCache_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "proto.GroupCache",
	HandlerType: (*GroupCacheServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Get",
			Handler:    _GroupCache_Get_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "groupcache_grpc.proto",
}
