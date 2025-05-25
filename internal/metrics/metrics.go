package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "swarmctl_http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "endpoint", "status_code"},
	)

	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "swarmctl_http_request_duration_seconds",
			Help:    "Duration of HTTP requests in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "endpoint"},
	)

	DockerServiceUpdatesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "swarmctl_docker_service_updates_total",
			Help: "Total number of Docker service updates",
		},
		[]string{"service_name", "status"},
	)

	DockerServiceUpdateDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "swarmctl_docker_service_update_duration_seconds",
			Help:    "Duration of Docker service updates in seconds",
			Buckets: []float64{0.1, 0.5, 1.0, 2.5, 5.0, 10.0, 30.0, 60.0},
		},
		[]string{"service_name"},
	)

	DockerEventsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "swarmctl_docker_events_total",
			Help: "Total number of Docker events processed",
		},
		[]string{"event_type", "service_name"},
	)

	ActiveConnections = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "swarmctl_active_connections",
			Help: "Number of active connections",
		},
	)

	RecentEventsCount = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "swarmctl_recent_events_count",
			Help: "Number of recent events in cache",
		},
	)

	RateLimitedRequestsTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "swarmctl_rate_limited_requests_total",
			Help: "Total number of rate limited requests",
		},
	)

	AuthFailuresTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "swarmctl_auth_failures_total",
			Help: "Total number of authentication failures",
		},
	)

	CloudflareSyncTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "swarmctl_cloudflare_sync_total",
			Help: "Total number of Cloudflare sync operations",
		},
		[]string{"status"},
	)

	CloudflareSyncDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "swarmctl_cloudflare_sync_duration_seconds",
			Help:    "Duration of Cloudflare sync operations in seconds",
			Buckets: []float64{0.1, 0.5, 1.0, 2.5, 5.0, 10.0},
		},
	)

	PushoverNotificationsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "swarmctl_pushover_notifications_total",
			Help: "Total number of Pushover notifications sent",
		},
		[]string{"status"},
	)
)

func UpdateRecentEventsCount(count int) {
	RecentEventsCount.Set(float64(count))
}

func RecordDockerServiceUpdate(serviceName, status string, duration float64) {
	DockerServiceUpdatesTotal.WithLabelValues(serviceName, status).Inc()
	DockerServiceUpdateDuration.WithLabelValues(serviceName).Observe(duration)
}

func RecordDockerEvent(eventType, serviceName string) {
	DockerEventsTotal.WithLabelValues(eventType, serviceName).Inc()
}

func RecordCloudflareSync(status string, duration float64) {
	CloudflareSyncTotal.WithLabelValues(status).Inc()
	CloudflareSyncDuration.Observe(duration)
}

func RecordPushoverNotification(status string) {
	PushoverNotificationsTotal.WithLabelValues(status).Inc()
}

func IncrementAuthFailures() {
	AuthFailuresTotal.Inc()
}

func IncrementRateLimitedRequests() {
	RateLimitedRequestsTotal.Inc()
}
