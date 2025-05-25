package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
)

// MetricsMiddleware returns a middleware that collects HTTP metrics
func MetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Create a response writer wrapper to capture status code
		ww := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		// Process the request
		next.ServeHTTP(ww, r)

		// Calculate duration
		duration := time.Since(start).Seconds()

		// Get the route pattern from chi router
		routePattern := getRoutePattern(r)

		// Record metrics
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

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// getRoutePattern extracts the route pattern from the request
func getRoutePattern(r *http.Request) string {
	// Try to get the route pattern from chi router context
	if routeCtx := chi.RouteContext(r.Context()); routeCtx != nil {
		if routeCtx.RoutePattern() != "" {
			return routeCtx.RoutePattern()
		}
	}

	// Fallback to the request path
	path := r.URL.Path
	if path == "" {
		path = "/"
	}
	return path
}
