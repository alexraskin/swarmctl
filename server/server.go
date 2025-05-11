package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/alexraskin/swarmctl/internal/cloudflare"
	"github.com/alexraskin/swarmctl/internal/docker"
	"github.com/alexraskin/swarmctl/internal/pushover"
	"github.com/alexraskin/swarmctl/internal/ver"
)

type Server struct {
	ctx            context.Context
	version        ver.Version
	config         *Config
	port           string
	server         *http.Server
	dockerClient   *docker.DockerClient
	pushoverClient *pushover.PushoverClient
	logger         *slog.Logger
	recentEvents   sync.Map
	cfSyncer       *cloudflare.Syncer
	cacheMu        sync.RWMutex
}

func NewServer(
	ctx context.Context,
	version ver.Version,
	config *Config,
	port string,
	dockerClient *docker.DockerClient,
	pushoverClient *pushover.PushoverClient,
	logger *slog.Logger,
	cfSyncer *cloudflare.Syncer,
) *Server {

	s := &Server{
		ctx:            ctx,
		version:        version,
		config:         config,
		port:           port,
		dockerClient:   dockerClient,
		pushoverClient: pushoverClient,
		logger:         logger,
		recentEvents:   sync.Map{},
		cfSyncer:       cfSyncer,
	}

	s.server = &http.Server{
		Addr:    ":" + port,
		Handler: s.Routes(),
	}

	return s
}

func (s *Server) Start() {

	go s.startDockerMonitor()
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
