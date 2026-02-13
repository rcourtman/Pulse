package discovery

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/websocket"
	pkgdiscovery "github.com/rcourtman/pulse-go-rewrite/pkg/discovery"
	"github.com/rs/zerolog/log"
)

// Service handles background network discovery
type Service struct {
	scanner        discoveryScanner
	wsHub          *websocket.Hub
	cache          *DiscoveryCache
	interval       time.Duration
	subnet         string
	mu             sync.RWMutex
	lastScan       time.Time
	isScanning     bool
	stopChan       chan struct{}
	ctx            context.Context
	cfgProvider    func() config.DiscoveryConfig
	history        []historyEntry
	historyLimit   int
	scannerFactory scannerFactory
	stopOnce       sync.Once
}

// DiscoveryCache stores the latest discovery results
type DiscoveryCache struct {
	mu      sync.RWMutex
	result  *pkgdiscovery.DiscoveryResult
	updated time.Time
}

type historyEntry struct {
	startedAt       time.Time
	completedAt     time.Time
	subnet          string
	serverCount     int
	errorCount      int
	duration        time.Duration
	blocklistLength int
	status          scanResultStatus
}

func (h historyEntry) StartedAt() time.Time {
	return h.startedAt
}

func (h historyEntry) CompletedAt() time.Time {
	return h.completedAt
}

func (h historyEntry) Subnet() string {
	return h.subnet
}

func (h historyEntry) ServerCount() int {
	return h.serverCount
}

func (h historyEntry) ErrorCount() int {
	return h.errorCount
}

func (h historyEntry) Duration() time.Duration {
	return h.duration
}

func (h historyEntry) BlocklistLength() int {
	return h.blocklistLength
}

func (h historyEntry) Status() string {
	return string(h.status)
}

const defaultHistoryLimit = 20

type scanResultStatus string

const (
	scanResultStatusSuccess scanResultStatus = "success"
	scanResultStatusFailure scanResultStatus = "failure"
	scanResultStatusPartial scanResultStatus = "partial"
)

type discoveryStartedPayload struct {
	Scanning  bool   `json:"scanning"`
	Subnet    string `json:"subnet"`
	Timestamp int64  `json:"timestamp"`
}

type discoveryServerFoundPayload struct {
	Server    pkgdiscovery.DiscoveredServer `json:"server"`
	Phase     string                        `json:"phase"`
	Timestamp int64                         `json:"timestamp"`
}

type discoveryProgressPayload struct {
	Progress  pkgdiscovery.ScanProgress `json:"progress"`
	Timestamp int64                     `json:"timestamp"`
}

type discoveryCompletePayload struct {
	Scanning    bool                          `json:"scanning"`
	Timestamp   int64                         `json:"timestamp"`
	Environment *pkgdiscovery.EnvironmentInfo `json:"environment,omitempty"`
}

type discoveryUpdatePayload struct {
	Servers          []pkgdiscovery.DiscoveredServer `json:"servers"`
	Errors           []string                        `json:"errors"`
	StructuredErrors []pkgdiscovery.DiscoveryError   `json:"structured_errors"`
	Scanning         bool                            `json:"scanning"`
	Timestamp        int64                           `json:"timestamp"`
	Environment      *pkgdiscovery.EnvironmentInfo   `json:"environment,omitempty"`
}

type ServiceStatus struct {
	IsScanning bool
	LastScan   time.Time
	Interval   time.Duration
	Subnet     string
}

func (s ServiceStatus) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"is_scanning": s.IsScanning,
		"last_scan":   s.LastScan,
		"interval":    s.Interval.String(),
		"subnet":      s.Subnet,
	}
}

type discoveryScanner interface {
	DiscoverServersWithCallbacks(ctx context.Context, subnet string, serverCallback pkgdiscovery.ServerCallback, progressCallback pkgdiscovery.ProgressCallback) (*pkgdiscovery.DiscoveryResult, error)
}

type scannerFactory func(config.DiscoveryConfig) (discoveryScanner, error)

var (
	newScannerFn = func() discoveryScanner {
		return pkgdiscovery.NewScanner()
	}
	discoveryScanResults = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "pulse",
			Subsystem: "discovery",
			Name:      "scans_total",
			Help:      "Total number of discovery scans by result status.",
		},
		[]string{"result"},
	)
	discoveryScanDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: "pulse",
			Subsystem: "discovery",
			Name:      "scan_duration_seconds",
			Help:      "Duration of discovery scans in seconds.",
			Buckets:   []float64{5, 10, 20, 30, 45, 60, 90, 120, 180},
		},
	)
	discoveryScanServers = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "pulse",
			Subsystem: "discovery",
			Name:      "last_scan_servers",
			Help:      "Number of servers found in the most recent discovery scan.",
		},
	)
	discoveryScanErrors = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "pulse",
			Subsystem: "discovery",
			Name:      "last_scan_errors",
			Help:      "Number of errors encountered in the most recent discovery scan.",
		},
	)
)

func init() {
	prometheus.MustRegister(discoveryScanResults, discoveryScanDuration, discoveryScanServers, discoveryScanErrors)
}

// NewService creates a new discovery service
func NewService(wsHub *websocket.Hub, interval time.Duration, subnet string, cfgProvider func() config.DiscoveryConfig) *Service {
	if interval == 0 {
		interval = 5 * time.Minute // Default to 5 minutes
	}
	if subnet == "" {
		subnet = "auto"
	}

	if cfgProvider == nil {
		cfgProvider = func() config.DiscoveryConfig { return config.DefaultDiscoveryConfig() }
	}

	return &Service{
		scanner:      newScannerFn(),
		wsHub:        wsHub,
		cache:        &DiscoveryCache{},
		interval:     interval,
		subnet:       subnet,
		stopChan:     make(chan struct{}),
		cfgProvider:  cfgProvider,
		history:      make([]historyEntry, 0, defaultHistoryLimit),
		historyLimit: defaultHistoryLimit,
		scannerFactory: func(cfg config.DiscoveryConfig) (discoveryScanner, error) {
			return BuildScanner(cfg)
		},
	}
}

// goRecover launches fn in a goroutine with panic recovery logging.
func goRecover(label string, fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Error().
					Interface("panic", r).
					Stack().
					Msgf("Recovered from panic in %s", label)
			}
		}()
		fn()
	}()
}

// Start begins the background discovery service
func (s *Service) Start(ctx context.Context) {
	s.ctx = ctx
	log.Info().
		Dur("interval", s.interval).
		Str("subnet", s.subnet).
		Msg("Starting background discovery service")

	// Do initial scan immediately
	goRecover("initial discovery scan", s.performScan)

	// Start background scanning loop
	goRecover("discovery scan loop", s.scanLoop)
}

// Stop stops the background discovery service
func (s *Service) Stop() {
	s.stopOnce.Do(func() {
		close(s.stopChan)
	})
}

// scanLoop runs periodic scans
func (s *Service) scanLoop() {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.performScan()
		case <-s.stopChan:
			log.Info().Msg("Stopping background discovery service")
			return
		case <-s.ctx.Done():
			log.Info().Msg("Background discovery service context cancelled")
			return
		}
	}
}

func (s *Service) appendHistory(entry historyEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.historyLimit <= 0 {
		s.historyLimit = defaultHistoryLimit
	}

	s.history = append(s.history, entry)
	if len(s.history) > s.historyLimit {
		offset := len(s.history) - s.historyLimit
		s.history = append([]historyEntry(nil), s.history[offset:]...)
	}
}

// GetHistory returns up to limit recent discovery history entries (most recent first).
func (s *Service) GetHistory(limit int) []historyEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 || limit > len(s.history) {
		limit = len(s.history)
	}
	if limit == 0 {
		return nil
	}

	result := make([]historyEntry, 0, limit)
	for i := len(s.history) - 1; i >= 0 && len(result) < limit; i-- {
		result = append(result, s.history[i])
	}
	return result
}

// performScan executes a network scan
func (s *Service) performScan() {
	startTime := time.Now()
	var scanErr error
	var blocklistLength int

	s.mu.Lock()
	if s.isScanning {
		s.mu.Unlock()
		log.Debug().Msg("Discovery scan already in progress, skipping")
		return
	}
	s.isScanning = true
	s.mu.Unlock()

	var result *pkgdiscovery.DiscoveryResult

	defer func() {
		duration := time.Since(startTime)
		completedAt := time.Now()
		serverCount := 0
		errorCount := 0
		status := scanResultStatusSuccess
		if result != nil {
			serverCount = len(result.Servers)
			if len(result.StructuredErrors) > 0 {
				errorCount = len(result.StructuredErrors)
			} else {
				errorCount = len(result.Errors)
			}
		}

		if scanErr != nil {
			if result == nil || serverCount == 0 {
				status = scanResultStatusFailure
			} else {
				status = scanResultStatusPartial
			}
		}

		discoveryScanDuration.Observe(duration.Seconds())
		discoveryScanServers.Set(float64(serverCount))
		discoveryScanErrors.Set(float64(errorCount))
		discoveryScanResults.WithLabelValues(string(status)).Inc()

		s.appendHistory(historyEntry{
			startedAt:       startTime,
			completedAt:     completedAt,
			subnet:          s.subnet,
			serverCount:     serverCount,
			errorCount:      errorCount,
			duration:        duration,
			blocklistLength: blocklistLength,
			status:          status,
		})

		s.mu.Lock()
		s.isScanning = false
		s.lastScan = completedAt
		s.mu.Unlock()

		// Send scan complete notification
		if s.wsHub != nil {
			payload := discoveryCompletePayload{
				Scanning:  false,
				Timestamp: completedAt.Unix(),
			}
			if result != nil && result.Environment != nil {
				payload.Environment = result.Environment
			}
			s.wsHub.Broadcast(websocket.Message{
				Type: "discovery_complete",
				Data: payload,
			})
		}
	}()

	log.Info().Str("subnet", s.subnet).Msg("Starting background discovery scan")

	// Send scan started notification
	if s.wsHub != nil {
		s.wsHub.Broadcast(websocket.Message{
			Type: "discovery_started",
			Data: discoveryStartedPayload{
				Scanning:  true,
				Subnet:    s.subnet,
				Timestamp: time.Now().Unix(),
			},
		})
	}

	// Create a context with timeout for the scan
	// Scanning multiple subnets takes longer, allow 2 minutes
	scanParentCtx := s.ctx
	if scanParentCtx == nil {
		scanParentCtx = context.Background()
	}
	scanCtx, cancel := context.WithTimeout(scanParentCtx, 2*time.Minute)
	defer cancel()

	cfg := config.NormalizeDiscoveryConfig(config.DefaultDiscoveryConfig())
	if s.cfgProvider != nil {
		cfg = config.NormalizeDiscoveryConfig(config.CloneDiscoveryConfig(s.cfgProvider()))
	}
	blocklistLength = len(cfg.SubnetBlocklist)

	var (
		newScanner discoveryScanner
		err        error
	)
	if s.scannerFactory != nil {
		newScanner, err = s.scannerFactory(cfg)
	} else {
		newScanner, err = BuildScanner(cfg)
	}
	if err != nil {
		log.Warn().Err(err).Msg("Environment detection failed during discovery; falling back to default scanner configuration")
		newScanner = newScannerFn()
	}
	if newScanner == nil {
		log.Warn().Msg("Discovery scanner factory returned nil; using default scanner configuration")
		newScanner = newScannerFn()
	}
	s.mu.Lock()
	s.scanner = newScanner
	s.mu.Unlock()

	// Perform the scan with real-time callbacks
	serverCallback := func(server pkgdiscovery.DiscoveredServer, phase string) {
		// Send immediate update for each discovered server
		if s.wsHub != nil {
			s.wsHub.Broadcast(websocket.Message{
				Type: "discovery_server_found",
				Data: discoveryServerFoundPayload{
					Server:    server,
					Phase:     phase,
					Timestamp: time.Now().Unix(),
				},
			})
			log.Debug().
				Str("phase", phase).
				Str("ip", server.IP).
				Str("type", server.Type).
				Msg("Broadcasting discovered server to clients")
		}
	}

	progressCallback := func(progress pkgdiscovery.ScanProgress) {
		// Send progress update via WebSocket
		if s.wsHub != nil {
			s.wsHub.Broadcast(websocket.Message{
				Type: "discovery_progress",
				Data: discoveryProgressPayload{
					Progress:  progress,
					Timestamp: time.Now().Unix(),
				},
			})
		}
	}

	result, err = newScanner.DiscoverServersWithCallbacks(scanCtx, s.subnet, serverCallback, progressCallback)
	scanErr = err
	if err != nil {
		// Even if scan timed out, we might have partial results
		if result == nil || (len(result.Servers) == 0 && !errors.Is(err, context.DeadlineExceeded)) {
			log.Error().Err(err).Msg("Background discovery scan failed")
			return
		}
		log.Warn().
			Err(err).
			Int("servers_found", len(result.Servers)).
			Msg("Discovery scan incomplete but found some servers")
	}

	// Always update cache with results (even if empty) to prevent stale data
	if result != nil {
		s.cache.mu.Lock()
		s.cache.result = result
		s.cache.updated = time.Now()
		s.cache.mu.Unlock()

		if result.Environment != nil {
			log.Info().
				Str("environment", result.Environment.Type).
				Float64("confidence", result.Environment.Confidence).
				Int("phases", len(result.Environment.Phases)).
				Msg("Environment detection summary")
		}

		log.Info().
			Int("servers", len(result.Servers)).
			Int("errors", len(result.Errors)).
			Msg("Background discovery scan completed")

		// Send final update via WebSocket with all servers
		if s.wsHub != nil {
			payload := discoveryUpdatePayload{
				Servers:          result.Servers,
				Errors:           result.Errors, // Legacy format (deprecated)
				StructuredErrors: result.StructuredErrors,
				Scanning:         false,
				Timestamp:        time.Now().Unix(),
				Environment:      result.Environment,
			}
			s.wsHub.Broadcast(websocket.Message{
				Type: "discovery_update",
				Data: payload,
			})
		}
	}
}

// GetCachedResult returns the cached discovery result
func (s *Service) GetCachedResult() (*pkgdiscovery.DiscoveryResult, time.Time) {
	s.cache.mu.RLock()
	defer s.cache.mu.RUnlock()

	if s.cache.result == nil {
		return &pkgdiscovery.DiscoveryResult{
			Servers: []pkgdiscovery.DiscoveredServer{},
			Errors:  []string{},
		}, time.Time{}
	}

	return s.cache.result, s.cache.updated
}

// ForceRefresh triggers an immediate scan
func (s *Service) ForceRefresh() {
	// Check if scan is already in progress to prevent goroutine leak
	s.mu.RLock()
	if s.isScanning {
		s.mu.RUnlock()
		log.Debug().Msg("Scan already in progress, skipping ForceRefresh")
		return
	}
	s.mu.RUnlock()

	goRecover("ForceRefresh scan", s.performScan)
}

// SetInterval updates the scan interval
func (s *Service) SetInterval(interval time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.interval = interval
	log.Info().Dur("interval", interval).Msg("Updated discovery scan interval")
}

// SetSubnet updates the subnet to scan
func (s *Service) SetSubnet(subnet string) {
	s.mu.Lock()
	s.subnet = subnet
	// Check if scan is already in progress to prevent goroutine leak
	alreadyScanning := s.isScanning
	s.mu.Unlock()

	log.Info().Str("subnet", subnet).Msg("Updated discovery subnet")

	// Trigger immediate rescan with new subnet if not already scanning
	if !alreadyScanning {
		goRecover("SetSubnet scan", s.performScan)
	} else {
		log.Debug().Msg("Scan already in progress, new subnet will be used in next scan")
	}
}

// GetStatus returns the current service status
func (s *Service) GetStatus() map[string]interface{} {
	return s.GetStatusSnapshot().ToMap()
}

// GetStatusSnapshot returns the current service status using strong typing.
func (s *Service) GetStatusSnapshot() ServiceStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return ServiceStatus{
		IsScanning: s.isScanning,
		LastScan:   s.lastScan,
		Interval:   s.interval,
		Subnet:     s.subnet,
	}
}
