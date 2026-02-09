package monitoring

import (
	"context"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	internalerrors "github.com/rcourtman/pulse-go-rewrite/internal/errors"
	"github.com/rcourtman/pulse-go-rewrite/internal/truenas"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

const defaultTrueNASPollInterval = 60 * time.Second

// TrueNASPoller manages periodic polling of configured TrueNAS connections.
type TrueNASPoller struct {
	registry    *unifiedresources.ResourceRegistry
	persistence *config.ConfigPersistence
	mu          sync.Mutex
	providers   map[string]*truenas.Provider // keyed by connection ID
	cancel      context.CancelFunc
	stopped     chan struct{}
	interval    time.Duration
}

// NewTrueNASPoller builds a new TrueNAS poller with the provided poll interval.
func NewTrueNASPoller(registry *unifiedresources.ResourceRegistry, persistence *config.ConfigPersistence, interval time.Duration) *TrueNASPoller {
	if interval <= 0 {
		interval = defaultTrueNASPollInterval
	}

	stopped := make(chan struct{})
	close(stopped)

	return &TrueNASPoller{
		registry:    registry,
		persistence: persistence,
		providers:   make(map[string]*truenas.Provider),
		stopped:     stopped,
		interval:    interval,
	}
}

// Start begins periodic TrueNAS polling if the feature flag is enabled.
func (p *TrueNASPoller) Start(ctx context.Context) {
	if p == nil || !truenas.IsFeatureEnabled() {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}

	p.mu.Lock()
	if p.cancel != nil {
		p.mu.Unlock()
		return
	}

	runCtx, cancel := context.WithCancel(ctx)
	p.cancel = cancel
	p.stopped = make(chan struct{})
	stopped := p.stopped
	p.mu.Unlock()

	go func() {
		defer close(stopped)
		defer func() {
			p.mu.Lock()
			if p.stopped == stopped {
				p.cancel = nil
			}
			p.mu.Unlock()
		}()

		p.syncConnections()
		p.pollAll(runCtx)

		ticker := time.NewTicker(p.interval)
		defer ticker.Stop()

		for {
			select {
			case <-runCtx.Done():
				return
			case <-ticker.C:
				p.syncConnections()
				p.pollAll(runCtx)
			}
		}
	}()
}

// Stop requests poller shutdown and waits up to five seconds for exit.
func (p *TrueNASPoller) Stop() {
	if p == nil {
		return
	}

	p.mu.Lock()
	cancel := p.cancel
	stopped := p.stopped
	p.mu.Unlock()

	if cancel != nil {
		cancel()
	}

	if stopped == nil {
		return
	}

	select {
	case <-stopped:
	case <-time.After(5 * time.Second):
		log.Printf("[TrueNASPoller] Stop timed out waiting for shutdown")
	}
}

func (p *TrueNASPoller) syncConnections() {
	if p == nil {
		return
	}
	if p.persistence == nil {
		log.Printf("[TrueNASPoller] Unable to sync connections: persistence is nil")
		return
	}

	instances, err := p.persistence.LoadTrueNASConfig()
	if err != nil {
		log.Printf("[TrueNASPoller] Failed to load TrueNAS config: %v", err)
		return
	}

	activeIDs := make(map[string]struct{}, len(instances))

	p.mu.Lock()
	defer p.mu.Unlock()

	for i := range instances {
		instance := instances[i]
		id := strings.TrimSpace(instance.ID)
		if id == "" || !instance.Enabled {
			continue
		}
		activeIDs[id] = struct{}{}

		if _, exists := p.providers[id]; exists {
			continue
		}

		client, err := truenas.NewClient(truenas.ClientConfig{
			Host:               instance.Host,
			Port:               instance.Port,
			APIKey:             instance.APIKey,
			Username:           instance.Username,
			Password:           instance.Password,
			UseHTTPS:           instance.UseHTTPS,
			InsecureSkipVerify: instance.InsecureSkipVerify,
			Fingerprint:        instance.Fingerprint,
		})
		if err != nil {
			log.Printf("[TrueNASPoller] Failed to initialize client for connection %q: %v", id, err)
			continue
		}

		p.providers[id] = truenas.NewLiveProvider(&truenas.APIFetcher{Client: client})
	}

	for id := range p.providers {
		if _, ok := activeIDs[id]; !ok {
			delete(p.providers, id)
		}
	}
}

func (p *TrueNASPoller) pollAll(ctx context.Context) {
	if p == nil {
		return
	}
	if p.registry == nil {
		log.Printf("[TrueNASPoller] Skipping poll: registry is nil")
		return
	}

	p.mu.Lock()
	type providerEntry struct {
		id       string
		provider *truenas.Provider
	}
	entries := make([]providerEntry, 0, len(p.providers))
	for id, provider := range p.providers {
		entries = append(entries, providerEntry{id: id, provider: provider})
	}
	p.mu.Unlock()

	pm := getPollMetrics()

	for _, entry := range entries {
		if entry.provider == nil {
			continue
		}

		start := time.Now()
		err := entry.provider.Refresh(ctx)
		end := time.Now()
		if err != nil {
			pm.RecordResult(PollResult{
				InstanceName: entry.id,
				InstanceType: "truenas",
				Success:      false,
				Error:        classifyTrueNASError(err, entry.id),
				StartTime:    start,
				EndTime:      end,
			})
			log.Printf("[TrueNASPoller] Refresh failed for %s: %v", entry.id, err)
			continue
		}

		pm.RecordResult(PollResult{
			InstanceName: entry.id,
			InstanceType: "truenas",
			Success:      true,
			StartTime:    start,
			EndTime:      end,
		})

		records := entry.provider.Records()
		if len(records) == 0 {
			continue
		}
		p.registry.IngestRecords(unifiedresources.SourceTrueNAS, records)
	}
}

// classifyTrueNASError wraps a TrueNAS API error in MonitorError for metrics classification.
func classifyTrueNASError(err error, connectionID string) *internalerrors.MonitorError {
	if err == nil {
		return nil
	}

	errType := internalerrors.ErrorTypeAPI
	retryable := true

	errStr := strings.ToLower(err.Error())
	switch {
	case strings.Contains(errStr, "timeout") || strings.Contains(errStr, "deadline exceeded"):
		errType = internalerrors.ErrorTypeTimeout
	case strings.Contains(errStr, "401") || strings.Contains(errStr, "403") || strings.Contains(errStr, "unauthorized") || strings.Contains(errStr, "forbidden"):
		errType = internalerrors.ErrorTypeAuth
		retryable = false
	case strings.Contains(errStr, "connection refused") || strings.Contains(errStr, "no such host") || strings.Contains(errStr, "connection reset"):
		errType = internalerrors.ErrorTypeConnection
	}

	return &internalerrors.MonitorError{
		Type:      errType,
		Op:        "truenas_poll",
		Instance:  connectionID,
		Err:       err,
		Timestamp: time.Now(),
		Retryable: retryable,
	}
}
