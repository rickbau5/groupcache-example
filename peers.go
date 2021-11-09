package main

import (
	"context"
	"log"
	"net/url"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"

	"github.com/mailgun/gubernator/v2"
)

type Peers interface {
	Maintain(ctx context.Context, setter func(...string)) error
}

type PeersFunc func(ctx context.Context, setter func(...string)) error

func (fn PeersFunc) Maintain(ctx context.Context, setter func(...string)) error {
	return fn(ctx, setter)
}

func PeerSet(peers ...string) PeersFunc {
	return PeersFunc(func(_ context.Context, setter func(...string)) error {
		setter(peers...)

		log.Println("setting peers:", peers)

		return nil
	})
}

type KubernetesPeers struct {
	poolConfig gubernator.K8sPoolConfig
	logger     *logrus.Logger
	setter     func(...string)

	init sync.Once
}

func NewKubernetesPeers(namespace, selector, podIP, podPort string, logger *logrus.Logger) *KubernetesPeers {
	var kp *KubernetesPeers

	kp = &KubernetesPeers{
		poolConfig: gubernator.K8sPoolConfig{
			OnUpdate: func(infos []gubernator.PeerInfo) {
				kp.set(infos)
			},
			Logger:    logger,
			Mechanism: gubernator.WatchPods,
			Namespace: namespace,
			Selector:  selector,
			PodIP:     podIP,
			PodPort:   podPort,
		},
		logger: logger,

		setter: func(s ...string) {
			logger.WithFields(logrus.Fields{"peers": s}).Warn("setter not ready")
		},

		init: sync.Once{},
	}

	return kp
}

func (kp *KubernetesPeers) Maintain(ctx context.Context, setter func(...string)) error {
	kp.setter = setter

	var (
		pool *gubernator.K8sPool
		err  error
	)

	kp.init.Do(func() {
		pool, err = gubernator.NewK8sPool(kp.poolConfig)
	})

	if err != nil {
		return err
	}

	<-ctx.Done()
	if pool != nil {
		// this is nil if this call didn't initialize the pool
		pool.Close()
	}

	return ctx.Err()
}

func (kp *KubernetesPeers) set(infos []gubernator.PeerInfo) {
	if kp == nil || kp.setter == nil {
		return
	}

	var peers []string
	for _, info := range infos {
		var addr string
		if info.HTTPAddress != "" {
			addr = info.HTTPAddress
		} else {
			addr = info.GRPCAddress

			if !strings.HasPrefix(addr, "http") {
				addr = "http://" + strings.Trim(addr, "/")
			}
		}

		if addr == "" {
			kp.logger.WithFields(logrus.Fields{"info": info}).Warn("missing address for peer info")
			continue
		}

		u, err := url.Parse(addr)
		if err != nil {
			kp.logger.WithFields(logrus.Fields{"error": err, "addr": addr, "info": info}).
				Warn("error parsing peer address")
			continue
		}
		u.Scheme = "http"

		peer := u.String()
		kp.logger.WithFields(logrus.Fields{"info": info, "peer": peer}).Debug("found peer")

		peers = append(peers, peer)
	}

	kp.logger.WithFields(logrus.Fields{"peers": peers, "count": len(peers)}).Debug("found peers")
	kp.setter(peers...)
}
