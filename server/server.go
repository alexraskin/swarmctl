package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/alexraskin/swarmctl/internal/ver"
	"github.com/docker/docker/client"
)

type Server struct {
	ctx              context.Context
	version          ver.Version
	config           *Config
	port             string
	server           *http.Server
	dockerClient     *client.Client
	cloudflareClient *CloudflareClient
	pushoverClient   *PushoverClient
	logger           *slog.Logger
	recentEvents     sync.Map
}

func NewServer(
	ctx context.Context,
	version ver.Version,
	config *Config,
	port string,
	dockerClient *client.Client,
	cloudflareClient *CloudflareClient,
	pushoverClient *PushoverClient,
	logger *slog.Logger,
) *Server {

	s := &Server{
		ctx:              ctx,
		version:          version,
		config:           config,
		port:             port,
		dockerClient:     dockerClient,
		cloudflareClient: cloudflareClient,
		pushoverClient:   pushoverClient,
		logger:           logger,
		recentEvents:     sync.Map{},
	}

	s.server = &http.Server{
		Addr:    ":" + port,
		Handler: s.Routes(),
	}

	return s
}

func (s *Server) Start() {

	go s.startDockerMonitor()
	go s.startCloudflare()
	go s.startEventCleanup(5*time.Minute, 10*time.Minute)

	if err := s.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		s.logger.Error("Error while listening", slog.Any("err", err))
		os.Exit(-1)
	}
}

func (s *Server) Close() {
	if err := s.server.Close(); err != nil {
		s.logger.Error("Error while closing server", slog.Any("err", err))
	}
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}
