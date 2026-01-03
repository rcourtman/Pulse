package main

import (
	"context"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog/log"
)

const defaultMetricsAddr = "127.0.0.1:9127"

// ProxyMetrics holds Prometheus metrics for the proxy
type ProxyMetrics struct {
	rpcRequests            *prometheus.CounterVec
	rpcLatency             *prometheus.HistogramVec
	sshRequests            *prometheus.CounterVec
	sshLatency             *prometheus.HistogramVec
	queueDepth             prometheus.Gauge
	rateLimitHits          prometheus.Counter
	limiterRejects         *prometheus.CounterVec
	globalConcurrency      prometheus.Gauge
	limiterPenalties       *prometheus.CounterVec
	limiterPeers           prometheus.Gauge
	nodeValidationFailures *prometheus.CounterVec
	readTimeouts           prometheus.Counter
	writeTimeouts          prometheus.Counter
	hostKeyChanges         *prometheus.CounterVec
	sshOutputOversized     *prometheus.CounterVec
	buildInfo              *prometheus.GaugeVec
	server                 *http.Server
	registry               *prometheus.Registry
}

// NewProxyMetrics creates and registers all metrics
func NewProxyMetrics(version string) *ProxyMetrics {
	reg := prometheus.NewRegistry()

	pm := &ProxyMetrics{
		rpcRequests: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "pulse_proxy_rpc_requests_total",
				Help: "Total RPC requests handled by method and result.",
			},
			[]string{"method", "result"},
		),
		rpcLatency: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "pulse_proxy_rpc_latency_seconds",
				Help:    "RPC handler latency.",
				Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1, 2.5, 5},
			},
			[]string{"method"},
		),
		sshRequests: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "pulse_proxy_ssh_requests_total",
				Help: "SSH command executions by node and result.",
			},
			[]string{"node", "result"},
		),
		sshLatency: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "pulse_proxy_ssh_latency_seconds",
				Help:    "SSH command latency per node.",
				Buckets: []float64{0.1, 0.5, 1, 2.5, 5, 10, 30},
			},
			[]string{"node"},
		),
		queueDepth: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "pulse_proxy_queue_depth",
				Help: "Concurrent RPC requests being processed.",
			},
		),
		rateLimitHits: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "pulse_proxy_rate_limit_hits_total",
				Help: "Number of RPC requests rejected due to rate limiting.",
			},
		),
		limiterRejects: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "pulse_proxy_limiter_rejections_total",
				Help: "Limiter rejections by reason.",
			},
			[]string{"reason", "peer"},
		),
		globalConcurrency: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "pulse_proxy_global_concurrency_inflight",
				Help: "Current global concurrency slots in use.",
			},
		),
		limiterPenalties: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "pulse_proxy_limiter_penalties_total",
				Help: "Penalty sleeps applied after validation failures.",
			},
			[]string{"reason", "peer"},
		),
		limiterPeers: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "pulse_proxy_limiter_active_peers",
				Help: "Number of peers tracked by the rate limiter.",
			},
		),
		nodeValidationFailures: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "pulse_proxy_node_validation_failures_total",
				Help: "Node validation failures by reason.",
			},
			[]string{"reason"},
		),
		readTimeouts: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "pulse_proxy_read_timeouts_total",
				Help: "Number of socket read timeouts.",
			},
		),
		writeTimeouts: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "pulse_proxy_write_timeouts_total",
				Help: "Number of socket write timeouts.",
			},
		),
		hostKeyChanges: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "pulse_proxy_hostkey_changes_total",
				Help: "Detected SSH host key changes by node.",
			},
			[]string{"node"},
		),
		sshOutputOversized: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "pulse_proxy_ssh_output_oversized_total",
				Help: "Number of SSH responses rejected for exceeding size limits.",
			},
			[]string{"node"},
		),
		buildInfo: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "pulse_proxy_build_info",
				Help: "Proxy build metadata.",
			},
			[]string{"version"},
		),
		registry: reg,
	}

	reg.MustRegister(
		pm.rpcRequests,
		pm.rpcLatency,
		pm.sshRequests,
		pm.sshLatency,
		pm.queueDepth,
		pm.rateLimitHits,
		pm.limiterRejects,
		pm.globalConcurrency,
		pm.limiterPenalties,
		pm.limiterPeers,
		pm.nodeValidationFailures,
		pm.readTimeouts,
		pm.writeTimeouts,
		pm.hostKeyChanges,
		pm.sshOutputOversized,
		pm.buildInfo,
	)

	pm.buildInfo.WithLabelValues(version).Set(1)

	return pm
}

// Start starts the metrics HTTP server on the specified address
func (m *ProxyMetrics) Start(addr string) error {
	if addr == "" || strings.ToLower(addr) == "disabled" {
		log.Info().Msg("Metrics server disabled")
		return nil
	}

	if addr == "default" {
		addr = defaultMetricsAddr
	}

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{}))

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	m.server = &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		if err := m.server.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Error().Err(err).Str("addr", addr).Msg("Metrics server stopped unexpectedly")
		}
	}()

	log.Info().Str("addr", addr).Msg("Metrics server started")
	return nil
}

// Shutdown gracefully shuts down the metrics server
func (m *ProxyMetrics) Shutdown(ctx context.Context) {
	if m == nil || m.server == nil {
		return
	}
	_ = m.server.Shutdown(ctx)
}

// sanitizeNodeLabel converts a node name into a safe Prometheus label value
func sanitizeNodeLabel(node string) string {
	const maxLen = 63
	safe := strings.Builder{}
	safe.Grow(len(node))

	for _, r := range strings.ToLower(node) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			safe.WriteRune(r)
		} else {
			safe.WriteRune('_')
		}
	}

	out := safe.String()
	if len(out) > maxLen {
		out = out[:maxLen]
	}
	if out == "" {
		out = "unknown"
	}

	return out
}

func (m *ProxyMetrics) recordLimiterReject(reason, peer string) {
	if m == nil {
		return
	}
	m.rateLimitHits.Inc()
	m.limiterRejects.WithLabelValues(reason, peer).Inc()
}

func (m *ProxyMetrics) recordNodeValidationFailure(reason string) {
	if m == nil {
		return
	}
	m.nodeValidationFailures.WithLabelValues(reason).Inc()
}

func (m *ProxyMetrics) recordReadTimeout() {
	if m == nil {
		return
	}
	m.readTimeouts.Inc()
}

func (m *ProxyMetrics) recordWriteTimeout() {
	if m == nil {
		return
	}
	m.writeTimeouts.Inc()
}

func (m *ProxyMetrics) recordSSHOutputOversized(node string) {
	if m == nil {
		return
	}
	if node == "" {
		node = "unknown"
	}
	m.sshOutputOversized.WithLabelValues(sanitizeNodeLabel(node)).Inc()
}

func (m *ProxyMetrics) recordHostKeyChange(node string) {
	if m == nil {
		return
	}
	if node == "" {
		node = "unknown"
	}
	m.hostKeyChanges.WithLabelValues(sanitizeNodeLabel(node)).Inc()
}

func (m *ProxyMetrics) incGlobalConcurrency() {
	if m == nil {
		return
	}
	m.globalConcurrency.Inc()
}

func (m *ProxyMetrics) decGlobalConcurrency() {
	if m == nil {
		return
	}
	m.globalConcurrency.Dec()
}

func (m *ProxyMetrics) recordPenalty(reason, peer string) {
	if m == nil {
		return
	}
	m.limiterPenalties.WithLabelValues(reason, peer).Inc()
}

func (m *ProxyMetrics) setLimiterPeers(count int) {
	if m == nil {
		return
	}
	m.limiterPeers.Set(float64(count))
}
