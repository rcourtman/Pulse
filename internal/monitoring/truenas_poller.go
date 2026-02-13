package monitoring

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	internalerrors "github.com/rcourtman/pulse-go-rewrite/internal/monitoring/errors"
	"github.com/rcourtman/pulse-go-rewrite/internal/truenas"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

const defaultTrueNASPollInterval = 60 * time.Second

// TrueNASPoller manages periodic polling of configured TrueNAS connections.
type TrueNASPoller struct {
	registry              *unifiedresources.ResourceRegistry
	persistence           *config.ConfigPersistence
	mu                    sync.Mutex
	providers             map[string]*truenas.Provider // keyed by connection ID
	cachedRecordsByConnID map[string][]unifiedresources.IngestRecord
	cancel                context.CancelFunc
	stopped               chan struct{}
	interval              time.Duration
}

// NewTrueNASPoller builds a new TrueNAS poller with the provided poll interval.
func NewTrueNASPoller(registry *unifiedresources.ResourceRegistry, persistence *config.ConfigPersistence, interval time.Duration) *TrueNASPoller {
	if interval <= 0 {
		interval = defaultTrueNASPollInterval
	}

	stopped := make(chan struct{})
	close(stopped)

	return &TrueNASPoller{
		registry:              registry,
		persistence:           persistence,
		providers:             make(map[string]*truenas.Provider),
		cachedRecordsByConnID: make(map[string][]unifiedresources.IngestRecord),
		stopped:               stopped,
		interval:              interval,
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

	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()

	select {
	case <-stopped:
		p.closeAllProviders()
	case <-timer.C:
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
			if provider := p.providers[id]; provider != nil {
				provider.Close()
			}
			delete(p.providers, id)
			delete(p.cachedRecordsByConnID, id)
		}
	}
}

func (p *TrueNASPoller) closeAllProviders() {
	if p == nil {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	for _, provider := range p.providers {
		if provider != nil {
			provider.Close()
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
			p.mu.Lock()
			p.cachedRecordsByConnID[entry.id] = nil
			p.mu.Unlock()
			continue
		}
		p.registry.IngestRecords(unifiedresources.SourceTrueNAS, records)
		p.mu.Lock()
		p.cachedRecordsByConnID[entry.id] = cloneIngestRecords(records)
		p.mu.Unlock()
	}
}

// GetCurrentRecords returns the latest known TrueNAS records across active connections.
func (p *TrueNASPoller) GetCurrentRecords() []unifiedresources.IngestRecord {
	if p == nil {
		return nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.cachedRecordsByConnID) == 0 {
		return nil
	}

	connectionIDs := make([]string, 0, len(p.cachedRecordsByConnID))
	for id := range p.cachedRecordsByConnID {
		connectionIDs = append(connectionIDs, id)
	}
	sort.Strings(connectionIDs)

	total := 0
	for _, id := range connectionIDs {
		total += len(p.cachedRecordsByConnID[id])
	}
	if total == 0 {
		return nil
	}

	records := make([]unifiedresources.IngestRecord, 0, total)
	for _, id := range connectionIDs {
		records = append(records, cloneIngestRecords(p.cachedRecordsByConnID[id])...)
	}
	return records
}

func cloneIngestRecords(records []unifiedresources.IngestRecord) []unifiedresources.IngestRecord {
	if len(records) == 0 {
		return nil
	}
	cloned := make([]unifiedresources.IngestRecord, len(records))
	copy(cloned, records)
	return cloned
}

// classifyTrueNASError wraps a TrueNAS API error in MonitorError for metrics classification.
func classifyTrueNASError(err error, connectionID string) *internalerrors.MonitorError {
	if err == nil {
		return nil
	}

	errType := internalerrors.ErrorTypeAPI
	retryable := true

	var apiErr *truenas.APIError
	if errors.As(err, &apiErr) {
		switch {
		case apiErr.StatusCode == http.StatusUnauthorized || apiErr.StatusCode == http.StatusForbidden:
			errType = internalerrors.ErrorTypeAuth
			retryable = false
		case apiErr.StatusCode == http.StatusRequestTimeout || apiErr.StatusCode == http.StatusGatewayTimeout:
			errType = internalerrors.ErrorTypeTimeout
		default:
			errType = internalerrors.ErrorTypeAPI
		}
	} else {
		// Transport-level errors: timeout takes precedence over generic connection failures.
		var urlErr *url.Error
		if (errors.As(err, &urlErr) && urlErr.Timeout()) || errors.Is(err, context.DeadlineExceeded) {
			errType = internalerrors.ErrorTypeTimeout
		} else {
			var netOpErr *net.OpError
			if errors.As(err, &netOpErr) {
				errType = internalerrors.ErrorTypeConnection
			}
		}
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
