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

	"github.com/alexraskin/swarmctl/internal/cloudflare"
	"github.com/alexraskin/swarmctl/internal/docker"
	"github.com/alexraskin/swarmctl/internal/pushover"
	"github.com/alexraskin/swarmctl/internal/ver"
	"github.com/alexraskin/swarmctl/server"
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

	dockerClient, err := docker.NewDockerClient()
	if err != nil {
		log.Fatal(err)
	}

	cloudflareClient, err := cloudflare.NewCloudflareClient(config.CloudflareAPIKey, config.CloudflareAPIEmail, config.CloudflareTunnelID, config.CloudflareAccountID)
	if err != nil {
		log.Fatal(err)
	}

	cfSyncer := cloudflare.NewSyncer(cloudflareClient)

	pushoverClient := pushover.NewPushoverClient(config.PushoverAPIKey)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s := server.NewServer(
		ctx,
		version,
		config,
		*port,
		dockerClient,
		pushoverClient,
		logger,
		cfSyncer,
	)

	go s.Start()

	logger.Debug("Starting swarmctl...", slog.String("port", *port), slog.String("version", version.Version), slog.String("revision", version.Revision), slog.String("build-time", version.BuildTime))

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
