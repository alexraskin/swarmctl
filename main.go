package main

import (
	"context"
	"flag"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/alexraskin/swarmctl/internal/ver"
	"github.com/alexraskin/swarmctl/server"

	"github.com/docker/docker/client"
)

func main() {
	port := flag.String("port", "8080", "port to listen on")
	debug := flag.Bool("debug", false, "enable debug logging")
	flag.Parse()

	version := ver.Load()

	logger := slog.Default()
	if *debug {
		logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}))
	}

	config := server.NewConfigFromEnv()

	dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Fatal(err)
	}

	cloudflareClient, err := server.NewCloudflareClient(config)
	if err != nil {
		log.Fatal(err)
	}

	pushoverClient := server.NewPushoverClient(config)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s := server.NewServer(
		ctx,
		version,
		config,
		*port,
		dockerClient,
		cloudflareClient,
		pushoverClient,
		logger,
	)
	go s.Start()

	logger.Debug("Starting SwarmCTL", slog.String("version", version.Version), slog.String("commit", version.Revision), slog.String("build-time", version.BuildTime))

	si := make(chan os.Signal, 1)
	signal.Notify(si, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)

	<-si
	logger.Debug("shutting down web server")

	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.Shutdown(ctx); err != nil {
		logger.Error("graceful shutdown failed", slog.Any("err", err))
		s.Close()
	}
}
