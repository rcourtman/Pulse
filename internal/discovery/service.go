package discovery

import (
	"context"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/websocket"
	"github.com/rcourtman/pulse-go-rewrite/pkg/discovery"
	"github.com/rs/zerolog/log"
)

// Service handles background network discovery
type Service struct {
	scanner        *discovery.Scanner
	wsHub          *websocket.Hub
	cache          *DiscoveryCache
	interval       time.Duration
	subnet         string
	mu             sync.RWMutex
	lastScan       time.Time
	isScanning     bool
	stopChan       chan struct{}
	ctx            context.Context
}

// DiscoveryCache stores the latest discovery results
type DiscoveryCache struct {
	mu      sync.RWMutex
	result  *discovery.DiscoveryResult
	updated time.Time
}

// NewService creates a new discovery service
func NewService(wsHub *websocket.Hub, interval time.Duration, subnet string) *Service {
	if interval == 0 {
		interval = 5 * time.Minute // Default to 5 minutes
	}
	if subnet == "" {
		subnet = "auto"
	}

	return &Service{
		scanner:  discovery.NewScanner(),
		wsHub:    wsHub,
		cache:    &DiscoveryCache{},
		interval: interval,
		subnet:   subnet,
		stopChan: make(chan struct{}),
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

	defer func() {
		s.mu.Lock()
		s.isScanning = false
		s.lastScan = time.Now()
		s.mu.Unlock()
	}()

	log.Info().Str("subnet", s.subnet).Msg("Starting background discovery scan")

	// Create a context with timeout for the scan
	scanCtx, cancel := context.WithTimeout(s.ctx, 30*time.Second)
	defer cancel()

	// Perform the scan
	result, err := s.scanner.DiscoverServers(scanCtx, s.subnet)
	if err != nil {
		log.Error().Err(err).Msg("Background discovery scan failed")
		return
	}

	// Update cache
	s.cache.mu.Lock()
	s.cache.result = result
	s.cache.updated = time.Now()
	s.cache.mu.Unlock()

	log.Info().
		Int("servers", len(result.Servers)).
		Int("errors", len(result.Errors)).
		Msg("Background discovery scan completed")

	// Send update via WebSocket
	if s.wsHub != nil {
		s.wsHub.Broadcast(websocket.Message{
			Type: "discovery_update",
			Data: map[string]interface{}{
				"servers":   result.Servers,
				"errors":    result.Errors,
				"timestamp": time.Now().Unix(),
			},
		})
	}
}

// GetCachedResult returns the cached discovery result
func (s *Service) GetCachedResult() (*discovery.DiscoveryResult, time.Time) {
	s.cache.mu.RLock()
	defer s.cache.mu.RUnlock()
	
	if s.cache.result == nil {
		return &discovery.DiscoveryResult{
			Servers: []discovery.DiscoveredServer{},
			Errors:  []string{},
		}, time.Time{}
	}
	
	return s.cache.result, s.cache.updated
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