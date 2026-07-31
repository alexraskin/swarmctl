package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/alexraskin/swarmctl/internal/cloudflare"
	"github.com/alexraskin/swarmctl/internal/docker"
	"github.com/alexraskin/swarmctl/internal/logger"
	"github.com/alexraskin/swarmctl/internal/notify"
	"github.com/alexraskin/swarmctl/internal/ver"
	"github.com/alexraskin/swarmctl/server"
)

func main() {
	port := flag.String("port", "8080", "port to listen on")
	debug := flag.Bool("debug", false, "enable debug logging")
	flag.Parse()

	version := ver.Load()

	config := server.LoadConfig()

	logLevel := slog.LevelInfo
	if *debug {
		logLevel = slog.LevelDebug
	}

	logger := logger.New(logLevel, config.Environment, "swarmctl")

	dockerClient, err := docker.NewDockerClient()
	if err != nil {
		logger.Error("failed to create docker client", "error", err)
		os.Exit(-1)
	}

	cloudflareClient, err := cloudflare.NewCloudflareClient(config.CloudflareAPIToken, config.CloudflareTunnelID, config.CloudflareAccountID)
	if err != nil {
		logger.Error("failed to create cloudflare client", "error", err)
		os.Exit(-1)
	}

	cfSyncer := cloudflare.NewSyncer(cloudflareClient)

	notifier, err := notify.New(config.NotificationURLs)
	if err != nil {
		logger.Error("failed to create notifier", "error", err)
		os.Exit(-1)
	}
	if !notifier.Enabled() {
		logger.Warn("NOTIFICATION_URLS is unset, container event alerts are disabled")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s := server.NewServer(
		ctx,
		version,
		config,
		*port,
		dockerClient,
		notifier,
		logger,
		cloudflareClient,
		cfSyncer,
	)

	go s.Start()

	logger.Info("Starting swarmctl...", slog.String("port", *port), slog.String("version", version.Version), slog.String("revision", version.Revision), slog.String("build-time", version.BuildTime))

	si := make(chan os.Signal, 1)
	signal.Notify(si, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)

	<-si
	logger.Debug("shutting down web server")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := s.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
		s.Close()
	}
}
