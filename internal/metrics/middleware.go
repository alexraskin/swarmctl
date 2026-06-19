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

		ActiveConnections.Inc()
		defer ActiveConnections.Dec()

		ww := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(ww, r)

		routePattern := getRoutePattern(r)

		// Don't record the metrics scrape itself; it would dominate the
		// HTTP request rate with Prometheus poll traffic.
		if routePattern == "/metrics" {
			return
		}

		duration := time.Since(start).Seconds()

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

	// No matched route (404s, scanner traffic). Use a fixed label instead of
	// the raw path to avoid unbounded label cardinality in Prometheus.
	return "unmatched"
}
