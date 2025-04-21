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

	"github.com/alexraskin/swarmctl/server"
	"github.com/docker/docker/client"
)

var (
	version   = "unknown"
	commit    = "unknown"
	buildTime = "unknown"
)

func main() {
	port := flag.String("port", "8080", "port to listen on")
	debug := flag.Bool("debug", false, "enable debug logging")
	flag.Parse()

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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	srv := server.NewServer(ctx, server.FormatBuildVersion(version, commit, buildTime), config, *port, dockerClient, cloudflareClient, logger)

	go srv.Start()

	go srv.StartCloudflare()

	logger.Debug("started web server", slog.Any("listen_addr", *port), slog.Any("version", version), slog.Any("commit", commit), slog.Any("build_time", buildTime))

	si := make(chan os.Signal, 1)
	signal.Notify(si, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)

	<-si
	logger.Debug("shutting down web server")

	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("graceful shutdown failed", slog.Any("err", err))
		srv.Close()
	}
}
