package grpcpool

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/rickbau5/groupcache-example/proto"

	"google.golang.org/grpc"

	"github.com/mailgun/groupcache"
	"github.com/mailgun/groupcache/consistenthash"
	"github.com/mailgun/groupcache/groupcachepb"
)

type Options struct {
	Replicas    int
	HashFn      consistenthash.Hash
	DialOptions func(addr string) []grpc.DialOption
	DialFn      func(context.Context, string, ...grpc.DialOption) (*grpc.ClientConn, error)
	Logger      *logrus.Logger
}

type Pool struct {
	self string
	opts Options

	mu      sync.Mutex
	peers   *consistenthash.Map
	getters map[string]*grpcGetter
	conns   map[string]*grpc.ClientConn

	logger *logrus.Logger
}

var grpcPoolMade bool

func NewGRPCPool(self string, opt *Options) *Pool {
	if grpcPoolMade {
		panic("groupcache: NewGRPCPool must only be called once")
	}
	grpcPoolMade = true

	pool := &Pool{
		self:    self,
		getters: make(map[string]*grpcGetter),
		conns:   make(map[string]*grpc.ClientConn),
	}
	if opt != nil {
		pool.opts = *opt
	}

	if pool.opts.Replicas == 0 {
		pool.opts.Replicas = 50
	}
	if pool.opts.DialFn == nil {
		pool.opts.DialFn = grpc.DialContext
	}
	if pool.opts.DialOptions == nil {
		pool.opts.DialOptions = func(string) []grpc.DialOption { return nil }
	}

	pool.logger = pool.opts.Logger
	if pool.logger == nil {
		pool.logger = logrus.New()
		pool.logger.SetLevel(logrus.WarnLevel)
	}

	pool.peers = consistenthash.New(pool.opts.Replicas, pool.opts.HashFn)

	groupcache.RegisterPeerPicker(func() groupcache.PeerPicker { return pool })

	go func() {
		for range time.Tick(time.Second * 15) {
			pool.cleanupConns()
		}
	}()

	return pool
}

func (pool *Pool) Set(peers ...string) {
	pool.mu.Lock()
	defer pool.mu.Unlock()
	pool.peers = consistenthash.New(pool.opts.Replicas, pool.opts.HashFn)
	pool.getters = make(map[string]*grpcGetter, len(peers))
	for _, peer := range peers {
		conn, ok := pool.conns[peer]
		if !ok {
			// TODO: maybe lazily do this
			// attempt to create a connection
			pool.logger.WithField("peer", peer).Debug("dialing peer")
			var err error
			ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
			conn, err = pool.opts.DialFn(ctx, peer, pool.opts.DialOptions(peer)...)
			if err != nil {
				pool.logger.WithField("peer", peer).Warn("failed dialing peer, removing from pool")
				cancel()
				continue
			}
			cancel()
			pool.logger.WithField("peer", peer).Debug("connected to peer")

			pool.conns[peer] = conn
		}
		pool.getters[peer] = &grpcGetter{
			client: proto.NewGroupCacheClient(conn),
			peer:   peer,
			logger: pool.logger,
		}
		pool.peers.Add(peer)
	}
}

func (pool *Pool) cleanupConns() {
	pool.mu.Lock()
	defer pool.mu.Unlock()
	var evict []string
	for peer, conn := range pool.conns {
		if _, ok := pool.getters[peer]; ok {
			// in use
			pool.logger.WithField("peer", peer).Debug("open connection still in use")
			continue
		}
		if err := conn.Close(); err != nil {
			pool.logger.WithField("peer", peer).WithError(err).Warning("error closing former peer connection")
		}
		evict = append(evict, peer)
	}

	if len(evict) > 0 {
		pool.logger.WithField("peers", evict).Info("evicting former peer connections")
		for _, peer := range evict {
			delete(pool.conns, peer)
		}
	}
}

func (pool *Pool) GetAll() []groupcache.ProtoGetter {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	res := make([]groupcache.ProtoGetter, 0)
	for peer, v := range pool.getters {
		if peer == pool.self {
			// don't return self as a peer - used to prevent a distributed deadlock during removal
			continue
		}
		if v == nil {
			pool.logger.WithField("peer", peer).Warn("peer getter is nil")
			continue
		}
		res = append(res, v)
	}
	return res
}

func (pool *Pool) PickPeer(key string) (groupcache.ProtoGetter, bool) {
	pool.mu.Lock()
	defer pool.mu.Unlock()
	if pool.peers.IsEmpty() {
		return nil, false
	}
	if peer := pool.peers.Get(key); peer != pool.self {
		return pool.getters[peer], true
	}
	return nil, false
}

type grpcGetter struct {
	client proto.GroupCacheClient
	peer   string
	logger *logrus.Logger
}

func (g *grpcGetter) Get(ctx groupcache.Context, in *groupcachepb.GetRequest, out *groupcachepb.GetResponse) error {
	l := g.logger.WithFields(logrus.Fields{"peer": g.peer, "group": in.GetGroup(), "key": in.GetKey()})
	l.WithField("in", in).Debug("getting from peer")
	if out == nil {
		return errors.New("out must not be nil")
	}

	resp, err := g.client.Get(ctx.(context.Context), in)
	if err != nil {
		l.WithError(err).Warning("error calling peer")
		return err
	}
	if resp == nil {
		l.Warning("got nil response from call")
		return errors.New("got nil response")
	}

	*out = *resp
	return nil
}

func (g *grpcGetter) Remove(ctx groupcache.Context, in *groupcachepb.GetRequest) error {
	g.logger.WithFields(logrus.Fields{"peer": g.peer, "group": in.GetGroup(), "key": in.GetKey()}).
		Debug("removing from peer")

	_, err := g.client.Remove(ctx.(context.Context), &proto.RemoveRequest{
		Group: in.GetGroup(),
		Key:   in.GetKey(),
	})
	if err != nil {
		g.logger.WithFields(logrus.Fields{"peer": g.peer, "group": in.GetGroup(), "key": in.GetKey()}).
			WithError(err).
			Warning("error removing from peer")
		return err
	}
	return nil
}
