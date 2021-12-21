package grpcpool

import (
	"sync"

	"github.com/mailgun/groupcache"
	"github.com/mailgun/groupcache/consistenthash"
	"github.com/mailgun/groupcache/groupcachepb"
)

type Options struct {
	Replicas int
	HashFn   consistenthash.Hash
}

type Pool struct {
	self string
	opts Options

	mu      sync.Mutex
	peers   *consistenthash.Map
	getters map[string]*grpcGetter
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
	}
	if opt != nil {
		pool.opts = *opt
	}

	if pool.opts.Replicas == 0 {
		pool.opts.Replicas = 50
	}

	pool.peers = consistenthash.New(pool.opts.Replicas, pool.opts.HashFn)

	groupcache.RegisterPeerPicker(func() groupcache.PeerPicker { return pool })

	return pool
}

func (pool *Pool) Set(peers ...string) {
	pool.mu.Lock()
	defer pool.mu.Unlock()
	pool.peers = consistenthash.New(pool.opts.Replicas, pool.opts.HashFn)
	pool.peers.Add(peers...)
	pool.getters = make(map[string]*grpcGetter, len(peers))
	for _, peer := range peers {
		pool.getters[peer] = &grpcGetter{
			// TODO: init grpc getter
		}
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
	// ...
}

func (g *grpcGetter) Get(ctx groupcache.Context, in *groupcachepb.GetRequest, out *groupcachepb.GetResponse) error {
	//TODO implement me
	panic("implement me")
}

func (g *grpcGetter) Remove(context groupcache.Context, in *groupcachepb.GetRequest) error {
	//TODO implement me
	panic("implement me")
}
