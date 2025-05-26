package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
)

func MetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		ww := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(ww, r)

		duration := time.Since(start).Seconds()

		routePattern := getRoutePattern(r)

		HTTPRequestsTotal.WithLabelValues(
			r.Method,
			routePattern,
			strconv.Itoa(ww.statusCode),
		).Inc()

		HTTPRequestDuration.WithLabelValues(
			r.Method,
			routePattern,
		).Observe(duration)
	})
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func getRoutePattern(r *http.Request) string {
	if routeCtx := chi.RouteContext(r.Context()); routeCtx != nil {
		if routeCtx.RoutePattern() != "" {
			return routeCtx.RoutePattern()
		}
	}

	path := r.URL.Path
	if path == "" {
		path = "/"
	}
	return path
}
