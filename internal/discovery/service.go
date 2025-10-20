package discovery

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/websocket"
	pkgdiscovery "github.com/rcourtman/pulse-go-rewrite/pkg/discovery"
	"github.com/rs/zerolog/log"
)

// Service handles background network discovery
type Service struct {
	scanner    *pkgdiscovery.Scanner
	wsHub      *websocket.Hub
	cache      *DiscoveryCache
	interval   time.Duration
	subnet     string
	mu         sync.RWMutex
	lastScan   time.Time
	isScanning bool
	stopChan   chan struct{}
	ctx        context.Context
	cfgProvider func() config.DiscoveryConfig
}

// DiscoveryCache stores the latest discovery results
type DiscoveryCache struct {
	mu      sync.RWMutex
	result  *pkgdiscovery.DiscoveryResult
	updated time.Time
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
		scanner:  pkgdiscovery.NewScanner(),
		wsHub:    wsHub,
		cache:    &DiscoveryCache{},
		interval: interval,
		subnet:   subnet,
		stopChan: make(chan struct{}),
		cfgProvider: cfgProvider,
	}
}

// Start begins the background discovery service
func (s *Service) Start(ctx context.Context) {
	s.ctx = ctx
	log.Info().
		Dur("interval", s.interval).
		Str("subnet", s.subnet).
		Msg("Starting background discovery service")

	// Do initial scan immediately
	go s.performScan()

	// Start background scanning loop
	go s.scanLoop()
}

// Stop stops the background discovery service
func (s *Service) Stop() {
	close(s.stopChan)
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

// performScan executes a network scan
func (s *Service) performScan() {
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
		s.mu.Lock()
		s.isScanning = false
		s.lastScan = time.Now()
		s.mu.Unlock()

		// Send scan complete notification
		if s.wsHub != nil {
			data := map[string]interface{}{
				"scanning":  false,
				"timestamp": time.Now().Unix(),
			}
			if result != nil && result.Environment != nil {
				data["environment"] = result.Environment
			}
			s.wsHub.Broadcast(websocket.Message{
				Type: "discovery_complete",
				Data: data,
			})
		}
	}()

	log.Info().Str("subnet", s.subnet).Msg("Starting background discovery scan")

	// Send scan started notification
	if s.wsHub != nil {
		s.wsHub.Broadcast(websocket.Message{
			Type: "discovery_started",
			Data: map[string]interface{}{
				"scanning":  true,
				"subnet":    s.subnet,
				"timestamp": time.Now().Unix(),
			},
		})
	}

	// Create a context with timeout for the scan
	// Scanning multiple subnets takes longer, allow 2 minutes
	scanCtx, cancel := context.WithTimeout(s.ctx, 2*time.Minute)
	defer cancel()

	cfg := config.NormalizeDiscoveryConfig(config.DefaultDiscoveryConfig())
	if s.cfgProvider != nil {
		cfg = config.NormalizeDiscoveryConfig(config.CloneDiscoveryConfig(s.cfgProvider()))
	}

	newScanner, err := BuildScanner(cfg)
	if err != nil {
		log.Warn().Err(err).Msg("Environment detection failed during discovery; falling back to default scanner configuration")
		newScanner = pkgdiscovery.NewScanner()
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
				Data: map[string]interface{}{
					"server":    server,
					"phase":     phase,
					"timestamp": time.Now().Unix(),
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
				Data: map[string]interface{}{
					"progress":  progress,
					"timestamp": time.Now().Unix(),
				},
			})
		}
	}

	result, err = newScanner.DiscoverServersWithCallbacks(scanCtx, s.subnet, serverCallback, progressCallback)
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
			data := map[string]interface{}{
				"servers":           result.Servers,
				"errors":            result.Errors,            // Legacy format (deprecated)
				"structured_errors": result.StructuredErrors, // New structured format
				"scanning":          false,
				"timestamp":         time.Now().Unix(),
			}
			if result.Environment != nil {
				data["environment"] = result.Environment
			}
			s.wsHub.Broadcast(websocket.Message{
				Type: "discovery_update",
				Data: data,
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

// IsScanning returns whether a scan is currently in progress
func (s *Service) IsScanning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.isScanning
}

// ForceRefresh triggers an immediate scan
func (s *Service) ForceRefresh() {
	go s.performScan()
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
	defer s.mu.Unlock()
	s.subnet = subnet
	log.Info().Str("subnet", subnet).Msg("Updated discovery subnet")

	// Trigger immediate rescan with new subnet
	go s.performScan()
}

// GetStatus returns the current service status
func (s *Service) GetStatus() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return map[string]interface{}{
		"is_scanning": s.isScanning,
		"last_scan":   s.lastScan,
		"interval":    s.interval.String(),
		"subnet":      s.subnet,
	}
}
