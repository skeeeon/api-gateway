// Package metrics provides Prometheus metrics for the API gateway
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics contains all Prometheus metrics for the API gateway
type Metrics struct {
	RequestsTotal      *prometheus.CounterVec
	RequestDuration    *prometheus.HistogramVec
	AuthFailures       *prometheus.CounterVec
	CacheRefreshes     prometheus.Counter
	CacheSize          *prometheus.GaugeVec
	ActiveConnections  prometheus.Gauge
}

// NewMetrics creates and registers all metrics
func NewMetrics(namespace string) *Metrics {
	return &Metrics{
		RequestsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "requests_total",
				Help:      "Total number of HTTP requests processed",
			},
			[]string{"method", "path", "status"},
		),
		
		RequestDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "request_duration_seconds",
				Help:      "Duration of HTTP requests in seconds",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{"method", "path"},
		),
		
		AuthFailures: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "auth_failures_total",
				Help:      "Total number of authentication failures",
			},
			[]string{"reason"},
		),
		
		CacheRefreshes: promauto.NewCounter(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "cache_refreshes_total",
				Help:      "Total number of cache refresh operations",
			},
		),
		
		CacheSize: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "cache_size",
				Help:      "Number of items in cache",
			},
			[]string{"type"},
		),
		
		ActiveConnections: promauto.NewGauge(
			prometheus.GaugeOpts{  // Changed from CounterOpts to GaugeOpts
				Namespace: namespace,
				Name:      "active_connections",
				Help:      "Number of active connections",
			},
		),
	}
}

// RecordRequest increments the request counter with the given parameters
func (m *Metrics) RecordRequest(method, path string, status int) {
	m.RequestsTotal.WithLabelValues(method, path, string(rune(status))).Inc()
}

// ObserveRequestDuration records the duration of a request
func (m *Metrics) ObserveRequestDuration(method, path string, duration float64) {
	m.RequestDuration.WithLabelValues(method, path).Observe(duration)
}

// RecordAuthFailure increments the auth failure counter with the given reason
func (m *Metrics) RecordAuthFailure(reason string) {
	m.AuthFailures.WithLabelValues(reason).Inc()
}

// RecordCacheRefresh increments the cache refresh counter
func (m *Metrics) RecordCacheRefresh() {
	m.CacheRefreshes.Inc()
}

// UpdateCacheSize updates the cache size metrics
func (m *Metrics) UpdateCacheSize(users, roles int) {
	m.CacheSize.WithLabelValues("users").Set(float64(users))
	m.CacheSize.WithLabelValues("roles").Set(float64(roles))
}

// IncActiveConnections increments the active connections counter
func (m *Metrics) IncActiveConnections() {
	m.ActiveConnections.Inc()
}

// DecActiveConnections decrements the active connections counter
func (m *Metrics) DecActiveConnections() {
	m.ActiveConnections.Dec()
}
