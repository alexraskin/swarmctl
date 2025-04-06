package server

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httprate"
)

func (s *Server) Routes() http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
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
	r.Post("/v1/update/{serviceName}", s.updateService)

	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Not found", http.StatusNotFound)
	})

	return r
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

	response, err := s.updateDockerService(r.Context(), serviceName, image)
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
