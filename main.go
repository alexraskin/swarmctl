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
	flag.Parse()

	config := server.NewConfig(server.GetAuthToken())

	dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Fatal(err)
	}

	srv := server.NewServer(server.FormatBuildVersion(version, commit, buildTime), config, *port, dockerClient)

	go srv.Start()

	slog.Info("started web server", slog.Any("listen_addr", *port))

	si := make(chan os.Signal, 1)
	signal.Notify(si, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)

	<-si
	slog.Info("shutting down web server")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("graceful shutdown failed", slog.Any("err", err))
	}
}
