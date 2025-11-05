package monitoring

import (
	"context"
	stderrors "errors"
	"sync"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

var (
	// ErrTemperatureMonitoringDisabled indicates that temperature polling is disabled globally.
	ErrTemperatureMonitoringDisabled = stderrors.New("temperature monitoring disabled")
	// ErrTemperatureCollectorUnavailable indicates the collector could not be created or is misconfigured.
	ErrTemperatureCollectorUnavailable = stderrors.New("temperature collector unavailable")
)

// TemperatureService defines the contract used by the monitor to collect temperature data.
type TemperatureService interface {
	Enabled() bool
	Collect(ctx context.Context, host, nodeName string) (*models.Temperature, error)
	Enable()
	Disable()
}

type temperatureService struct {
	mu        sync.RWMutex
	enabled   bool
	user      string
	keyPath   string
	collector *TemperatureCollector
}

func newTemperatureService(enabled bool, user, keyPath string) TemperatureService {
	return &temperatureService{
		enabled: enabled,
		user:    user,
		keyPath: keyPath,
	}
}

func (s *temperatureService) Enabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.enabled
}

func (s *temperatureService) Enable() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.enabled {
		return
	}

	s.enabled = true
	if s.collector == nil && s.user != "" && s.keyPath != "" {
		s.collector = NewTemperatureCollector(s.user, s.keyPath)
	}
}

func (s *temperatureService) Disable() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.enabled = false
	s.collector = nil
}

func (s *temperatureService) Collect(ctx context.Context, host, nodeName string) (*models.Temperature, error) {
	s.mu.RLock()
	enabled := s.enabled
	collector := s.collector
	s.mu.RUnlock()

	if !enabled {
		return nil, ErrTemperatureMonitoringDisabled
	}

	if collector == nil {
		s.mu.Lock()
		if s.enabled && s.collector == nil && s.user != "" && s.keyPath != "" {
			s.collector = NewTemperatureCollector(s.user, s.keyPath)
		}
		collector = s.collector
		enabled = s.enabled
		s.mu.Unlock()

		if !enabled {
			return nil, ErrTemperatureMonitoringDisabled
		}
	}

	if collector == nil {
		return nil, ErrTemperatureCollectorUnavailable
	}

	return collector.CollectTemperature(ctx, host, nodeName)
}
