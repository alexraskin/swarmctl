package server

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httprate"
)

var statsStartTime = time.Now()

func (s *Server) Routes() http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Heartbeat("/ping"))
	r.Use(authMiddleware(s.config.AuthToken))
	r.Use(httprate.Limit(
		10,
		time.Minute,
		httprate.WithLimitHandler(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "Too many requests", http.StatusTooManyRequests)
			}),
		),
	))

	r.Get("/version", s.serverVersion)
	r.Get("/stats", s.stats)
	r.Post("/v1/update/{serviceName}", s.updateService)

	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Not found", http.StatusNotFound)
	})

	return r
}

func (s *Server) stats(w http.ResponseWriter, r *http.Request) {
	stats := runtime.MemStats{}
	runtime.ReadMemStats(&stats)

	data := struct {
		Go               string
		Uptime           string
		MemoryUsed       string
		TotalMemory      string
		GarbageCollected string
		Goroutines       int
	}{
		Go:               runtime.Version(),
		Uptime:           getDurationString(time.Since(statsStartTime)),
		MemoryUsed:       humanize.Bytes(stats.Alloc),
		TotalMemory:      humanize.Bytes(stats.Sys),
		GarbageCollected: humanize.Bytes(stats.TotalAlloc),
		Goroutines:       runtime.NumGoroutine(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func (s *Server) serverVersion(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s.version)
}

func (s *Server) updateService(w http.ResponseWriter, r *http.Request) {
	serviceName := chi.URLParam(r, "serviceName")
	image := r.URL.Query().Get("image")

	if serviceName == "" || image == "" {
		http.Error(w, "Missing serviceName in path or image in query", http.StatusBadRequest)
		return
	}

	response, err := s.updateDockerService(serviceName, image)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func authMiddleware(expectedToken string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := r.Header.Get("Authorization")
			if len(token) < 8 || subtle.ConstantTimeCompare([]byte(token[7:]), []byte(expectedToken)) != 1 {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func getDurationString(duration time.Duration) string {
	return fmt.Sprintf(
		"%0.2d:%02d:%02d",
		int(duration.Hours()),
		int(duration.Minutes())%60,
		int(duration.Seconds())%60,
	)
}
