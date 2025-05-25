package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httprate"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/alexraskin/swarmctl/internal/metrics"
	"github.com/alexraskin/swarmctl/internal/middle"
)

func (s *Server) Routes() http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Heartbeat("/ping"))
	r.Use(middle.AuthMiddleware(s.config.AuthToken))
	r.Use(metrics.MetricsMiddleware)

	r.Use(httprate.Limit(
		10,
		time.Minute,
		httprate.WithLimitHandler(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				metrics.IncrementRateLimitedRequests()
				http.Error(w, "Too many requests", http.StatusTooManyRequests)
			}),
		),
	))

	r.Get("/version", s.serverVersion)
	r.Get("/metrics", s.metricsHandler)

	r.Group(func(r chi.Router) {
		r.Use(middle.IPCheck)
		r.Post("/update/{serviceName}", s.updateService)
	})

	r.NotFound(s.notFound)

	return r
}

func (s *Server) notFound(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "Not found", http.StatusNotFound)
}

func (s *Server) serverVersion(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprint(w, s.version.Format())
}

func (s *Server) metricsHandler(w http.ResponseWriter, r *http.Request) {
	count := 0
	s.recentEvents.Range(func(key, value any) bool {
		count++
		return true
	})
	metrics.UpdateRecentEventsCount(count)

	promhttp.Handler().ServeHTTP(w, r)
}

func (s *Server) updateService(w http.ResponseWriter, r *http.Request) {
	serviceName := chi.URLParam(r, "serviceName")
	image := r.URL.Query().Get("image")

	if serviceName == "" || image == "" {
		http.Error(w, "Missing serviceName in path or image in query", http.StatusBadRequest)
		return
	}

	start := time.Now()
	response, err := s.dockerClient.UpdateDockerService(serviceName, image, r.Context())
	duration := time.Since(start).Seconds()

	if err != nil {
		metrics.RecordDockerServiceUpdate(serviceName, "error", duration)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	metrics.RecordDockerServiceUpdate(serviceName, "success", duration)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
