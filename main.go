package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/adaptor/v2"
	"github.com/gofiber/fiber/v2"
	mlogger "github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/mailgun/groupcache"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"

	_ "net/http/pprof"
)

func main() {
	var (
		app    = fiber.New()
		logger = configureLogger()
		ttl    = time.Duration(1 * time.Minute)
	)

	peers, self, err := configurePeerMaintainer(logger)
	if err != nil {
		logger.WithError(err).Fatalln("failed creating peer maintainer")
		return
	}

	logger.WithFields(logrus.Fields{"self": self}).Debug("configured peer maintainer")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// create the pool which describes all the nodes participating
	httpPoolOptions := &groupcache.HTTPPoolOptions{
		BasePath: "/_groupcache/",
	}

	pool := groupcache.NewHTTPPoolOpts(self, httpPoolOptions)
	// TODO: this needs to go into the errgroup
	go func() {
		err := peers.Maintain(ctx, func(peers ...string) {
			pool.Set(peers...)
		})
		if err != nil {
			logger.WithError(err).Fatalln("failed maintaining peers")
		}
	}()

	// set up the backend impl which will be used to fetch data on cache misses
	backend := backendImpl{}

	// create the group that the pool will use to fetch data from the underlying backend
	//  - this is only called by this instance when it is determined to own the key requested
	group := groupcache.NewGroup("data", 3000000, groupcache.GetterFunc(
		func(_ groupcache.Context, key string, dest groupcache.Sink) error {
			logger.WithField("key", key).Debug("fetching key from backend")

			ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
			defer cancel()

			data, err := backend.Get(ctx, key)
			if err != nil {
				return fmt.Errorf("failed getting data from backend: %w", err)
			}

			bs, err := json.Marshal(data)
			if err != nil {
				return fmt.Errorf("failed marshaling data: %w", err)
			}

			return dest.SetBytes(bs, time.Now().Add(ttl))
		},
	))

	// create a route to get stats for the various caches
	app.Get("/stats/:type?", func(ctx *fiber.Ctx) error {
		switch ctx.Params("type") {
		case "hot":
			return ctx.JSON(group.CacheStats(groupcache.HotCache))
		case "main":
			return ctx.JSON(group.CacheStats(groupcache.MainCache))
		case "":
			return ctx.JSON(group.Stats)
		default:
			ctx.Status(http.StatusBadRequest)
			return ctx.SendString("unknown cache")
		}

	})

	// create the caching backend with the group
	cachingBackend := backendCacheImpl{cache: group}

	// set up the route other services will call
	app.Use("/data/:guid", mlogger.New())
	app.Get("/data/:guid", func(ctx *fiber.Ctx) error {
		guid := ctx.Params("guid")
		if guid == "" {
			return ctx.Status(http.StatusBadRequest).SendString("guid missing")
		}

		// use the caching backend implementation to get the data
		data, err := cachingBackend.Get(ctx.UserContext(), guid)
		if err != nil {
			return fmt.Errorf("failed getting data: %w", err)
		}

		return ctx.JSON(data)
	})

	// set up the groupcache routes (these are internal to groupcache and how it communicates with peers
	groupCachePath := fmt.Sprintf("%s+", httpPoolOptions.BasePath)
	app.Use(groupCachePath, mlogger.New())
	app.Get(fmt.Sprintf("%s+", httpPoolOptions.BasePath), adaptor.HTTPHandler(pool))

	// add pprof endpoints if enabled
	if b, _ := strconv.ParseBool(os.Getenv("PPROF_ENABLED")); b {
		pprofGroup := app.Group("/debug/pprof")
		pprofGroup.Get("/cmdline", adaptor.HTTPHandlerFunc(pprof.Cmdline))
		pprofGroup.Get("/profile", adaptor.HTTPHandlerFunc(pprof.Profile))
		pprofGroup.Get("/symbol", adaptor.HTTPHandlerFunc(pprof.Symbol))
		pprofGroup.Get("/trace", adaptor.HTTPHandlerFunc(pprof.Trace))
		pprofGroup.Get("/:profile?", adaptor.HTTPHandlerFunc(pprof.Index))
	}

	// print group stats with the logger
	go monitorGroup(group, logger)

	if err := run(app, logger); err != nil {
		logger.WithError(err).Fatal("app exiting with error")
	}

	logger.Println("app exiting cleanly")
}

func run(app *fiber.App, logger *logrus.Logger) error {
	grp, ctx := errgroup.WithContext(context.Background())

	errShuttingDown := errors.New("shutting down")
	grp.Go(func() error {
		ctx, cancel := signal.NotifyContext(ctx, os.Interrupt)
		defer cancel()
		<-ctx.Done()

		return errShuttingDown
	})

	grp.Go(func() error {
		<-ctx.Done()
		if err := app.Shutdown(); err != nil {
			return fmt.Errorf("error in shutdown: %w", err)
		}

		logger.Println("server shutdown complete")

		return errShuttingDown
	})

	grp.Go(func() error {
		var err error
		if err = app.Listen(":3000"); err != nil {
			logger.Println("error in listen:", err)
			return err
		}

		logger.Println("server stopped")
		return errShuttingDown
	})

	if err := grp.Wait(); err != nil && !errors.Is(err, errShuttingDown) {
		return err
	}

	return nil
}

func monitorGroup(group *groupcache.Group, logger *logrus.Logger) {
	for range time.Tick(15 * time.Second) {
		logger.WithFields(logrus.Fields{
			"stats":      group.Stats,
			"main_cache": group.CacheStats(groupcache.MainCache),
			"hot_cache":  group.CacheStats(groupcache.HotCache),
		}).Debug("cache stats")
	}
}

func configurePeerMaintainer(logger *logrus.Logger) (Peers, string, error) {
	peersType := os.Getenv("PEERS_TYPE")

	var (
		self  = os.Getenv("PEERS_SELF")
		peers Peers
	)

	switch peersType {
	case "pods":
		namespace := os.Getenv("GUBERNATOR_NAMESPACE")
		selector := os.Getenv("GUBERNATOR_SELECTOR")
		podPort := os.Getenv("GUBERNATOR_POD_PORT")
		podIP := os.Getenv("GUBERNATOR_POD_IP")

		if self == "" {
			if podIP == "" {
				return nil, "", errors.New("one of GUBERNATOR_POD_IP or PEERS_SELF must be set")
			}

			self = fmt.Sprintf("http://%s:%s", podIP, podPort)
		}

		logger.WithFields(logrus.Fields{"namespace": namespace, "selector": selector, "self": self, "podPort": podPort}).
			Debug("configuring kubernetes peers maintainer")

		peers = NewKubernetesPeers(namespace, selector, self, podPort, logger)
	case "set":
		peers = PeerSet(strings.Split(os.Getenv("PEERS_SET"), ",")...)
	default:
		return nil, "", fmt.Errorf("unsupported PEERS_TYPE: %s", peersType)
	}

	return peers, self, nil
}

func configureLogger() *logrus.Logger {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	logger.SetFormatter(&logrus.JSONFormatter{})

	return logger
}
