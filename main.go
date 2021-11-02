package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/gofiber/adaptor/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/mailgun/groupcache"
	"golang.org/x/sync/errgroup"
)

func main() {
	app := fiber.New()

	stdoutLogger := log.New(os.Stdout, "", log.LstdFlags)

	var (
		me    = os.Getenv("GROUPCACHE_ADDR_SELF")
		peers = append(strings.Split(os.Getenv("GROUPCACHE_ADDR_PEERS"), ","), me)
		ttl   = time.Duration(1 * time.Minute)
	)

	stdoutLogger.Printf("groupcache:\n\tmy address: %s\n\tpeer addresses: %v", me, peers)

	// set up the backend impl which will be used to fetch data on cache misses
	backend := backendImpl{}

	// create the pool which describes all the nodes participating
	httpPoolOptions := &groupcache.HTTPPoolOptions{
		BasePath: "/_groupcache/",
	}
	pool := groupcache.NewHTTPPoolOpts(me, httpPoolOptions)
	pool.Set(peers...)

	// create the group that the pool will use to fetch data from the underlying backend
	//  - this is only called by this instance when it is determined to own the key requested
	group := groupcache.NewGroup("data", 3000000, groupcache.GetterFunc(
		func(_ groupcache.Context, key string, dest groupcache.Sink) error {
			stdoutLogger.Println("fetching key from backend:", key)

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
	app.Use("/data/:guid", logger.New())
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
	app.Use(groupCachePath, logger.New())
	app.Get(fmt.Sprintf("%s+", httpPoolOptions.BasePath), adaptor.HTTPHandler(pool))

	// print group stats with the logger
	go monitorGroup(group, stdoutLogger)

	if err := run(app, stdoutLogger); err != nil {
		stdoutLogger.Fatalln("app exiting with error:", err)
	}

	stdoutLogger.Println("app exiting cleanly")
}

func run(app *fiber.App, logger *log.Logger) error {
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

func monitorGroup(group *groupcache.Group, logger *log.Logger) {
	for range time.Tick(15 * time.Second) {
		print := func(message string, i interface{}) {
			bs, _ := json.Marshal(i)
			logger.Println(message, string(bs))
		}
		print("stats", group.Stats)
		print("main cache", group.CacheStats(groupcache.MainCache))
		print("hot cache", group.CacheStats(groupcache.HotCache))
	}
}
