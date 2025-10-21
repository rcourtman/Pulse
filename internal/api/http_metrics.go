package api

import (
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	httpMetricsOnce sync.Once

	apiRequestDuration *prometheus.HistogramVec
	apiRequestTotal    *prometheus.CounterVec
	apiRequestErrors   *prometheus.CounterVec
)

func initHTTPMetrics() {
	apiRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "pulse",
			Subsystem: "http",
			Name:      "request_duration_seconds",
			Help:      "HTTP request duration observed at the API layer.",
			Buckets:   []float64{0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		},
		[]string{"method", "route", "status"},
	)

	apiRequestTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "pulse",
			Subsystem: "http",
			Name:      "requests_total",
			Help:      "Total number of HTTP requests handled by the API.",
		},
		[]string{"method", "route", "status"},
	)

	apiRequestErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "pulse",
			Subsystem: "http",
			Name:      "request_errors_total",
			Help:      "Total number of HTTP errors surfaced to clients.",
		},
		[]string{"method", "route", "status_class"},
	)

	prometheus.MustRegister(apiRequestDuration, apiRequestTotal, apiRequestErrors)
}

func recordAPIRequest(method, route string, status int, elapsed time.Duration) {
	httpMetricsOnce.Do(initHTTPMetrics)

	statusCode := strconv.Itoa(status)

	apiRequestDuration.WithLabelValues(method, route, statusCode).Observe(elapsed.Seconds())
	apiRequestTotal.WithLabelValues(method, route, statusCode).Inc()

	if status >= 400 {
		apiRequestErrors.WithLabelValues(method, route, classifyStatus(status)).Inc()
	}
}

func classifyStatus(status int) string {
	switch {
	case status >= 500:
		return "server_error"
	case status >= 400:
		return "client_error"
	default:
		return "none"
	}
}

func normalizeRoute(path string) string {
	if path == "" || path == "/" {
		return "/"
	}

	// Strip query parameters.
	if idx := strings.Index(path, "?"); idx >= 0 {
		path = path[:idx]
	}

	segments := strings.Split(path, "/")
	normSegments := make([]string, 0, len(segments))
	count := 0
	for _, seg := range segments {
		if seg == "" {
			continue
		}
		count++
		if count > 5 {
			break
		}
		normSegments = append(normSegments, normalizeSegment(seg))
	}

	if len(normSegments) == 0 {
		return "/"
	}

	return "/" + strings.Join(normSegments, "/")
}

func normalizeSegment(seg string) string {
	if isNumeric(seg) {
		return ":id"
	}
	if looksLikeUUID(seg) {
		return ":uuid"
	}
	if len(seg) > 32 {
		return ":token"
	}
	return seg
}

func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func looksLikeUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, r := range s {
		switch {
		case r == '-':
			if i != 8 && i != 13 && i != 18 && i != 23 {
				return false
			}
		case (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F'):
			continue
		default:
			return false
		}
	}
	return true
}
