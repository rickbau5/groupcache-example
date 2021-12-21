package grpcpool

import (
	"context"
	"errors"
	"log"
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
	Context     func() context.Context
	DialOptions func(addr string) []grpc.DialOption
	DialFn      func(context.Context, string, ...grpc.DialOption) (*grpc.ClientConn, error)
}

type Pool struct {
	self string
	opts Options

	mu      sync.Mutex
	peers   *consistenthash.Map
	getters map[string]*grpcGetter
	conns   map[string]*grpc.ClientConn
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
	if pool.opts.Context == nil {
		pool.opts.Context = context.Background
	}
	if pool.opts.DialOptions == nil {
		pool.opts.DialOptions = func(string) []grpc.DialOption { return nil }
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
			log.Println("dialing peer:", peer)
			var err error
			conn, err = pool.opts.DialFn(pool.opts.Context(), peer, pool.opts.DialOptions(peer)...)
			if err != nil {
				log.Println("failed dialing peer, skipping:", err)
				continue
			}
			log.Println("connected to peer:", peer)

			pool.conns[peer] = conn
		}
		pool.getters[peer] = &grpcGetter{client: proto.NewGroupCacheClient(conn), peer: peer}
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
			log.Printf("peer in use '%s'", peer)
			continue
		}
		log.Printf("evicting former peer connection '%s'", peer)
		if err := conn.Close(); err != nil {
			log.Printf("error closing peer connection '%s': %s", peer, err)
		}
		evict = append(evict, peer)
	}

	for _, peer := range evict {
		delete(pool.conns, peer)
	}
}

func (pool *Pool) GetAll() []groupcache.ProtoGetter {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	var i int
	res := make([]groupcache.ProtoGetter, len(pool.getters))
	for _, v := range pool.getters {
		res[i] = v
		i++
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
}

func (g *grpcGetter) Get(ctx groupcache.Context, in *groupcachepb.GetRequest, out *groupcachepb.GetResponse) error {
	l := logrus.WithFields(logrus.Fields{"peer": g.peer, "group": in.GetGroup(), "key": in.GetKey()})
	l.WithField("in", in).Debug("getting from peer")
	if out == nil {
		l.Warning("out is nil")
		return errors.New("out must not be nil")
	}

	l.Debug("calling peer")
	resp, err := g.client.Get(ctx.(context.Context), in)
	if err != nil {
		l.WithError(err).Warning("error calling peer")
		return err
	}
	if resp == nil {
		return errors.New("got nil response")
	}

	l.WithField("resp", resp).Debug("got response")

	*out = *resp
	return nil
}

func (g *grpcGetter) Remove(context groupcache.Context, in *groupcachepb.GetRequest) error {
	// TODO implement me
	panic("implement me")
}
