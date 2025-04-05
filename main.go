package main

import (
	"flag"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

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

	server := server.NewServer(server.FormatBuildVersion(version, commit, buildTime), config, *port, dockerClient)

	go server.Start()
	defer server.Close()

	slog.Info("started web server", slog.Any("listen_addr", *port))
	si := make(chan os.Signal, 1)
	signal.Notify(si, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-si
	slog.Info("shutting down web server")
}
