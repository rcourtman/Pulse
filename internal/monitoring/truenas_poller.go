package monitoring

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	internalerrors "github.com/rcourtman/pulse-go-rewrite/internal/monitoring/errors"
	recoverymanager "github.com/rcourtman/pulse-go-rewrite/internal/recovery/manager"
	truenasmapper "github.com/rcourtman/pulse-go-rewrite/internal/recovery/mapper/truenas"
	"github.com/rcourtman/pulse-go-rewrite/internal/truenas"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rs/zerolog/log"
)

const defaultTrueNASPollInterval = 60 * time.Second

// TrueNASPoller manages periodic polling of configured TrueNAS connections.
type TrueNASPoller struct {
	multiTenant     *config.MultiTenantPersistence
	recoveryManager *recoverymanager.Manager
	mu              sync.Mutex
	// providersByOrg is keyed by orgID, then connection ID.
	providersByOrg map[string]map[string]*truenas.Provider
	// cachedRecordsByOrg is keyed by orgID, then connection ID.
	cachedRecordsByOrg map[string]map[string][]unifiedresources.IngestRecord
	cancel             context.CancelFunc
	stopped            chan struct{}
	interval           time.Duration
}

// NewTrueNASPoller builds a new TrueNAS poller with the provided poll interval.
func NewTrueNASPoller(multiTenant *config.MultiTenantPersistence, interval time.Duration, recoveryManager *recoverymanager.Manager) *TrueNASPoller {
	if interval <= 0 {
		interval = defaultTrueNASPollInterval
	}

	stopped := make(chan struct{})
	close(stopped)

	return &TrueNASPoller{
		multiTenant:        multiTenant,
		recoveryManager:    recoveryManager,
		providersByOrg:     make(map[string]map[string]*truenas.Provider),
		cachedRecordsByOrg: make(map[string]map[string][]unifiedresources.IngestRecord),
		stopped:            stopped,
		interval:           interval,
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
	case <-time.After(5 * time.Second):
		log.Warn().
			Str("component", "truenas_poller").
			Str("action", "stop").
			Dur("timeout", 5*time.Second).
			Msg("TrueNAS poller stop timed out waiting for shutdown")
	}
}

func (p *TrueNASPoller) syncConnections() {
	if p == nil {
		return
	}
	if p.multiTenant == nil {
		log.Warn().
			Str("component", "truenas_poller").
			Str("action", "sync_connections").
			Msg("TrueNAS poller cannot sync connections because multi-tenant persistence is nil")
		return
	}

	orgs, err := p.multiTenant.ListOrganizations()
	if err != nil {
		log.Warn().
			Str("component", "truenas_poller").
			Str("action", "list_organizations").
			Err(err).
			Msg("TrueNAS poller failed to list organizations")
		return
	}

	// orgID -> connID -> instance
	active := make(map[string]map[string]config.TrueNASInstance, len(orgs))

	for _, org := range orgs {
		if org == nil {
			continue
		}
		orgID := strings.TrimSpace(org.ID)
		if orgID == "" {
			continue
		}
		persistence, err := p.multiTenant.GetPersistence(orgID)
		if err != nil || persistence == nil {
			log.Warn().
				Str("component", "truenas_poller").
				Str("action", "get_persistence").
				Str("org_id", orgID).
				Err(err).
				Msg("TrueNAS poller failed to open tenant persistence")
			continue
		}

		instances, err := persistence.LoadTrueNASConfig()
		if err != nil {
			log.Warn().
				Str("component", "truenas_poller").
				Str("action", "load_truenas_config").
				Str("org_id", orgID).
				Err(err).
				Msg("TrueNAS poller failed to load TrueNAS config")
			continue
		}

		for i := range instances {
			instance := instances[i]
			id := strings.TrimSpace(instance.ID)
			if id == "" || !instance.Enabled {
				continue
			}
			if active[orgID] == nil {
				active[orgID] = make(map[string]config.TrueNASInstance)
			}
			active[orgID][id] = instance
		}
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// Ensure providers exist for active connections.
	for orgID, byConn := range active {
		if len(byConn) == 0 {
			continue
		}
		if p.providersByOrg[orgID] == nil {
			p.providersByOrg[orgID] = make(map[string]*truenas.Provider)
		}
		if p.cachedRecordsByOrg[orgID] == nil {
			p.cachedRecordsByOrg[orgID] = make(map[string][]unifiedresources.IngestRecord)
		}

		for connID, instance := range byConn {
			if _, exists := p.providersByOrg[orgID][connID]; exists {
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
				log.Warn().
					Str("component", "truenas_poller").
					Str("action", "initialize_client").
					Str("org_id", orgID).
					Str("connection_id", connID).
					Err(err).
					Msg("TrueNAS poller failed to initialize client")
				continue
			}

			p.providersByOrg[orgID][connID] = truenas.NewLiveProvider(&truenas.APIFetcher{Client: client})
		}
	}

	// Prune providers for removed/disabled connections and removed orgs.
	for orgID, providers := range p.providersByOrg {
		activeByConn := active[orgID]
		for connID, provider := range providers {
			if activeByConn == nil {
				// Org no longer exists or has no active TrueNAS connections.
				if provider != nil {
					provider.Close()
				}
				delete(providers, connID)
				if p.cachedRecordsByOrg[orgID] != nil {
					delete(p.cachedRecordsByOrg[orgID], connID)
				}
				continue
			}
			if _, ok := activeByConn[connID]; !ok {
				if provider != nil {
					provider.Close()
				}
				delete(providers, connID)
				if p.cachedRecordsByOrg[orgID] != nil {
					delete(p.cachedRecordsByOrg[orgID], connID)
				}
			}
		}

		if len(providers) == 0 {
			delete(p.providersByOrg, orgID)
			delete(p.cachedRecordsByOrg, orgID)
		}
	}
}

func (p *TrueNASPoller) closeAllProviders() {
	if p == nil {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	for _, providers := range p.providersByOrg {
		for _, provider := range providers {
			if provider != nil {
				provider.Close()
			}
		}
	}
}

func (p *TrueNASPoller) pollAll(ctx context.Context) {
	if p == nil {
		return
	}
	p.mu.Lock()
	type providerEntry struct {
		orgID    string
		id       string
		provider *truenas.Provider
	}
	entries := make([]providerEntry, 0)
	for orgID, providers := range p.providersByOrg {
		for id, provider := range providers {
			entries = append(entries, providerEntry{orgID: orgID, id: id, provider: provider})
		}
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
			log.Warn().
				Str("component", "truenas_poller").
				Str("action", "refresh_connection").
				Str("connection_id", entry.id).
				Err(err).
				Msg("TrueNAS poller refresh failed")
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
			if p.cachedRecordsByOrg[entry.orgID] == nil {
				p.cachedRecordsByOrg[entry.orgID] = make(map[string][]unifiedresources.IngestRecord)
			}
			p.cachedRecordsByOrg[entry.orgID][entry.id] = nil
			p.mu.Unlock()
			continue
		}
		p.mu.Lock()
		if p.cachedRecordsByOrg[entry.orgID] == nil {
			p.cachedRecordsByOrg[entry.orgID] = make(map[string][]unifiedresources.IngestRecord)
		}
		p.cachedRecordsByOrg[entry.orgID][entry.id] = cloneIngestRecords(records)
		p.mu.Unlock()

		p.ingestRecoveryPoints(ctx, entry.orgID, entry.id, entry.provider)
	}
}

func (p *TrueNASPoller) ingestRecoveryPoints(ctx context.Context, orgID string, connectionID string, provider *truenas.Provider) {
	if p == nil || p.recoveryManager == nil || provider == nil {
		return
	}

	store, err := p.recoveryManager.StoreForOrg(orgID)
	if err != nil {
		log.Warn().
			Str("component", "truenas_poller").
			Str("action", "recovery_store").
			Err(err).
			Msg("TrueNAS poller failed to open recovery store")
		return
	}

	snapshot := provider.Snapshot()
	points := truenasmapper.FromTrueNASSnapshot(connectionID, snapshot)
	if len(points) == 0 {
		return
	}

	// Best-effort cap in case a system has an enormous snapshot history.
	const maxPointsPerPoll = 2000
	if len(points) > maxPointsPerPoll {
		points = points[:maxPointsPerPoll]
	}

	if err := store.UpsertPoints(ctx, points); err != nil {
		log.Warn().
			Str("component", "truenas_poller").
			Str("action", "ingest_recovery_points").
			Str("connection_id", strings.TrimSpace(connectionID)).
			Err(err).
			Msg("TrueNAS poller failed to ingest recovery points")
	}
}

// GetCurrentRecords returns the latest known TrueNAS records across active connections.
func (p *TrueNASPoller) GetCurrentRecords() []unifiedresources.IngestRecord {
	return p.GetCurrentRecordsForOrg("default")
}

// SupplementalRecords adapts TrueNAS records to the monitor supplemental
// provider contract.
func (p *TrueNASPoller) SupplementalRecords(_ *Monitor, orgID string) []unifiedresources.IngestRecord {
	return p.GetCurrentRecordsForOrg(orgID)
}

// SnapshotOwnedSources declares source-native ingest ownership for legacy
// snapshot suppression (default/global call path).
func (p *TrueNASPoller) SnapshotOwnedSources() []unifiedresources.DataSource {
	return []unifiedresources.DataSource{unifiedresources.SourceTrueNAS}
}

// SnapshotOwnedSourcesForOrg declares source-native ingest ownership for a
// specific tenant.
func (p *TrueNASPoller) SnapshotOwnedSourcesForOrg(string) []unifiedresources.DataSource {
	return []unifiedresources.DataSource{unifiedresources.SourceTrueNAS}
}

// GetCurrentRecordsForOrg returns the latest known TrueNAS records for the specified org.
func (p *TrueNASPoller) GetCurrentRecordsForOrg(orgID string) []unifiedresources.IngestRecord {
	if p == nil {
		return nil
	}

	orgID = strings.TrimSpace(orgID)
	if orgID == "" {
		orgID = "default"
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	recordsByConn := p.cachedRecordsByOrg[orgID]
	if len(recordsByConn) == 0 {
		return nil
	}

	connectionIDs := make([]string, 0, len(recordsByConn))
	for id := range recordsByConn {
		connectionIDs = append(connectionIDs, id)
	}
	sort.Strings(connectionIDs)

	total := 0
	for _, id := range connectionIDs {
		total += len(recordsByConn[id])
	}
	if total == 0 {
		return nil
	}

	records := make([]unifiedresources.IngestRecord, 0, total)
	for _, id := range connectionIDs {
		records = append(records, cloneIngestRecords(recordsByConn[id])...)
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
