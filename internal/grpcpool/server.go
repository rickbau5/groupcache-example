package grpcpool

import (
	"context"

	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/mailgun/groupcache"
	"github.com/mailgun/groupcache/groupcachepb"
	"github.com/rickbau5/groupcache-example/proto"
	pb "github.com/rickbau5/groupcache-example/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Server struct {
	pb.GroupCacheServer
}

func (srv *Server) Get(ctx context.Context, get *groupcachepb.GetRequest) (*groupcachepb.GetResponse, error) {
	key := get.GetKey()
	group := groupcache.GetGroup(get.GetGroup())
	group.Stats.ServerRequests.Add(1)

	var b []byte

	value := groupcache.AllocatingByteSliceSink(&b)
	err := group.Get(ctx, key, value)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "error getting key from group '%s': %s", key, err)
	}

	// view := value.view()

	return &groupcachepb.GetResponse{
		Value: b,
		// TODO: how to handle this? ugh
		// Expire: view.Expire().UnixNano(),
		XXX_unrecognized: nil,
	}, nil
}

func (srv *Server) Remove(ctx context.Context, rem *proto.RemoveRequest) (*emptypb.Empty, error) {
	group := groupcache.GetGroup(rem.GetGroup())
	group.Stats.ServerRequests.Add(1)

	// this should be `group.localRemove`, of course that is not exported so we can't.
	return &emptypb.Empty{}, group.Remove(ctx, rem.GetKey())
}
