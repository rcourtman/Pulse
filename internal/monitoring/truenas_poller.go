package monitoring

import (
	"context"
	"errors"
	"fmt"
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

const defaultTrueNASHistoryReadTimeout = 10 * time.Second

// TrueNASConnectionPollError captures the last runtime poll error for one
// configured TrueNAS connection.
type TrueNASConnectionPollError struct {
	At       *time.Time `json:"at,omitempty"`
	Message  string     `json:"message,omitempty"`
	Category string     `json:"category,omitempty"`
}

// TrueNASConnectionPollStatus summarizes the runtime polling state for one
// configured TrueNAS connection.
type TrueNASConnectionPollStatus struct {
	IntervalSeconds     int                         `json:"intervalSeconds"`
	LastAttemptAt       *time.Time                  `json:"lastAttemptAt,omitempty"`
	LastSuccessAt       *time.Time                  `json:"lastSuccessAt,omitempty"`
	ConsecutiveFailures int                         `json:"consecutiveFailures,omitempty"`
	LastError           *TrueNASConnectionPollError `json:"lastError,omitempty"`
}

// TrueNASConnectionObservedSummary summarizes the canonical resources the most
// recent successful poll contributed for one configured TrueNAS connection.
type TrueNASConnectionObservedSummary struct {
	Host              string     `json:"host,omitempty"`
	ResourceID        string     `json:"resourceId,omitempty"`
	CollectedAt       *time.Time `json:"collectedAt,omitempty"`
	Systems           int        `json:"systems"`
	StoragePools      int        `json:"storagePools"`
	Datasets          int        `json:"datasets"`
	Apps              int        `json:"apps"`
	Disks             int        `json:"disks"`
	RecoveryArtifacts int        `json:"recoveryArtifacts"`
}

// TrueNASConnectionSummary merges poll health with the most recent discovered
// platform contribution for one configured TrueNAS connection.
type TrueNASConnectionSummary struct {
	Poll     *TrueNASConnectionPollStatus      `json:"poll,omitempty"`
	Observed *TrueNASConnectionObservedSummary `json:"observed,omitempty"`
}

type trueNASConnectionRuntimeStatus struct {
	lastAttemptAt       time.Time
	lastSuccessAt       time.Time
	lastError           *TrueNASConnectionPollError
	consecutiveFailures int
	nextPollAt          time.Time
	observed            *TrueNASConnectionObservedSummary
}

// TrueNASPoller manages periodic polling of configured TrueNAS connections.
type TrueNASPoller struct {
	multiTenant     *config.MultiTenantPersistence
	recoveryManager *recoverymanager.Manager
	mu              sync.Mutex
	// providersByOrg is keyed by orgID, then connection ID.
	providersByOrg map[string]map[string]*truenas.Provider
	// configsByOrg is keyed by orgID, then connection ID.
	configsByOrg map[string]map[string]config.TrueNASInstance
	// statusByOrg is keyed by orgID, then connection ID.
	statusByOrg map[string]map[string]*trueNASConnectionRuntimeStatus
	// cachedRecordsByOrg is keyed by orgID, then connection ID.
	cachedRecordsByOrg map[string]map[string][]unifiedresources.IngestRecord
	cancel             context.CancelFunc
	stopped            chan struct{}
	interval           time.Duration
	explicitInterval   bool
}

type trueNASPollerProviderEntry struct {
	connectionID string
	provider     *truenas.Provider
}

// NewTrueNASPoller builds a new TrueNAS poller with the provided poll interval.
func NewTrueNASPoller(multiTenant *config.MultiTenantPersistence, interval time.Duration, recoveryManager *recoverymanager.Manager) *TrueNASPoller {
	explicitInterval := interval > 0
	if interval <= 0 {
		interval = defaultTrueNASPollInterval
	}

	stopped := make(chan struct{})
	close(stopped)

	return &TrueNASPoller{
		multiTenant:        multiTenant,
		recoveryManager:    recoveryManager,
		providersByOrg:     make(map[string]map[string]*truenas.Provider),
		configsByOrg:       make(map[string]map[string]config.TrueNASInstance),
		statusByOrg:        make(map[string]map[string]*trueNASConnectionRuntimeStatus),
		cachedRecordsByOrg: make(map[string]map[string][]unifiedresources.IngestRecord),
		stopped:            stopped,
		interval:           interval,
		explicitInterval:   explicitInterval,
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
		for {
			wait := p.nextWaitDuration(time.Now())
			timer := time.NewTimer(wait)
			select {
			case <-runCtx.Done():
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				return
			case <-timer.C:
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
	configured := make(map[string]map[string]config.TrueNASInstance, len(orgs))
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
			if id == "" {
				continue
			}
			instance.ApplyDefaults()
			if configured[orgID] == nil {
				configured[orgID] = make(map[string]config.TrueNASInstance)
			}
			configured[orgID][id] = instance
			if !instance.Enabled {
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
		if p.configsByOrg[orgID] == nil {
			p.configsByOrg[orgID] = make(map[string]config.TrueNASInstance)
		}
		if p.statusByOrg[orgID] == nil {
			p.statusByOrg[orgID] = make(map[string]*trueNASConnectionRuntimeStatus)
		}
		if p.cachedRecordsByOrg[orgID] == nil {
			p.cachedRecordsByOrg[orgID] = make(map[string][]unifiedresources.IngestRecord)
		}

		for connID, instance := range byConn {
			previousConfig := p.configsByOrg[orgID][connID]
			existingProvider, exists := p.providersByOrg[orgID][connID]
			p.configsByOrg[orgID][connID] = instance
			p.alignConnectionNextPollLocked(orgID, connID, previousConfig, instance)
			if exists && !trueNASProviderConfigChanged(previousConfig, instance) {
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
				now := time.Now().UTC()
				p.recordConnectionFailureLocked(orgID, connID, instance, err, now)
				log.Warn().
					Str("component", "truenas_poller").
					Str("action", "initialize_client").
					Str("org_id", orgID).
					Str("connection_id", connID).
					Err(err).
					Msg("TrueNAS poller failed to initialize client")
				continue
			}

			if existingProvider != nil {
				existingProvider.Close()
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
				if p.configsByOrg[orgID] != nil {
					delete(p.configsByOrg[orgID], connID)
				}
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
				if p.configsByOrg[orgID] != nil {
					delete(p.configsByOrg[orgID], connID)
				}
				if p.cachedRecordsByOrg[orgID] != nil {
					delete(p.cachedRecordsByOrg[orgID], connID)
				}
			}
		}

		if len(providers) == 0 {
			delete(p.providersByOrg, orgID)
			delete(p.configsByOrg, orgID)
			delete(p.cachedRecordsByOrg, orgID)
		}
	}

	// Prune runtime status only when the connection is truly gone from config.
	for orgID, byConn := range p.statusByOrg {
		configuredByConn := configured[orgID]
		for connID := range byConn {
			if configuredByConn == nil {
				delete(byConn, connID)
				continue
			}
			if _, ok := configuredByConn[connID]; !ok {
				delete(byConn, connID)
			}
		}
		if len(byConn) == 0 {
			delete(p.statusByOrg, orgID)
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

// ConnectionSummaries returns per-connection runtime health and discovered
// contribution summaries for the supplied TrueNAS settings records.
func (p *TrueNASPoller) ConnectionSummaries(orgID string, instances []config.TrueNASInstance) map[string]TrueNASConnectionSummary {
	if len(instances) == 0 {
		return nil
	}

	orgID = normalizeTrueNASOrgID(orgID)
	summaries := make(map[string]TrueNASConnectionSummary, len(instances))

	for i := range instances {
		instance := instances[i]
		instance.ApplyDefaults()
		connID := strings.TrimSpace(instance.ID)
		if connID == "" {
			continue
		}

		summary := TrueNASConnectionSummary{
			Poll: &TrueNASConnectionPollStatus{
				IntervalSeconds: instance.EffectivePollIntervalSecs(),
			},
		}

		if p != nil {
			p.mu.Lock()
			status := p.cloneRuntimeStatusLocked(orgID, connID)
			p.mu.Unlock()
			if status != nil {
				summary.Poll.LastAttemptAt = cloneTimePointer(status.LastAttemptAt)
				summary.Poll.LastSuccessAt = cloneTimePointer(status.LastSuccessAt)
				summary.Poll.ConsecutiveFailures = status.ConsecutiveFailures
				summary.Poll.LastError = cloneTrueNASConnectionPollError(status.LastError)
				summary.Observed = cloneTrueNASObservedSummary(status.Observed)
			}
		}

		summaries[connID] = summary
	}

	return summaries
}

func (p *TrueNASPoller) nextWaitDuration(now time.Time) time.Duration {
	if p == nil {
		return defaultTrueNASPollInterval
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	wait := p.interval
	for orgID, byConn := range p.configsByOrg {
		for connID, instance := range byConn {
			next := p.nextPollAtLocked(orgID, connID, instance)
			if next.IsZero() || !next.After(now) {
				return 0
			}
			until := next.Sub(now)
			if until < wait {
				wait = until
			}
		}
	}

	if wait < 0 {
		return 0
	}
	return wait
}

func (p *TrueNASPoller) pollAll(ctx context.Context) {
	if p == nil {
		return
	}
	p.mu.Lock()
	type providerEntry struct {
		orgID    string
		id       string
		config   config.TrueNASInstance
		provider *truenas.Provider
	}
	entries := make([]providerEntry, 0)
	now := time.Now().UTC()
	for orgID, providers := range p.providersByOrg {
		for id, provider := range providers {
			instance, ok := p.configsByOrg[orgID][id]
			if !ok {
				continue
			}
			if !p.connectionPollDueLocked(orgID, id, instance, now) {
				continue
			}
			entries = append(entries, providerEntry{
				orgID:    orgID,
				id:       id,
				config:   instance,
				provider: provider,
			})
		}
	}
	p.mu.Unlock()

	pm := getPollMetrics()

	for _, entry := range entries {
		if entry.provider == nil {
			continue
		}

		start := time.Now().UTC()
		err := entry.provider.Refresh(ctx)
		end := time.Now().UTC()
		if err != nil {
			pm.RecordResult(PollResult{
				InstanceName: entry.id,
				InstanceType: "truenas",
				Success:      false,
				Error:        classifyTrueNASError(err, entry.id),
				StartTime:    start,
				EndTime:      end,
			})
			p.mu.Lock()
			p.recordConnectionFailureLocked(entry.orgID, entry.id, entry.config, err, end)
			p.mu.Unlock()
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

		snapshot := entry.provider.Snapshot()
		p.mu.Lock()
		p.recordConnectionSuccessLocked(entry.orgID, entry.id, entry.config, end, snapshot)
		p.mu.Unlock()

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

type trueNASConnectionRuntimeSnapshot struct {
	LastAttemptAt       *time.Time
	LastSuccessAt       *time.Time
	ConsecutiveFailures int
	LastError           *TrueNASConnectionPollError
	Observed            *TrueNASConnectionObservedSummary
}

func (p *TrueNASPoller) cloneRuntimeStatusLocked(orgID string, connID string) *trueNASConnectionRuntimeSnapshot {
	if p == nil {
		return nil
	}
	byConn := p.statusByOrg[orgID]
	if byConn == nil {
		return nil
	}
	status := byConn[connID]
	if status == nil {
		return nil
	}
	return &trueNASConnectionRuntimeSnapshot{
		LastAttemptAt:       timePointerIfSet(status.lastAttemptAt),
		LastSuccessAt:       timePointerIfSet(status.lastSuccessAt),
		ConsecutiveFailures: status.consecutiveFailures,
		LastError:           cloneTrueNASConnectionPollError(status.lastError),
		Observed:            cloneTrueNASObservedSummary(status.observed),
	}
}

func (p *TrueNASPoller) ensureConnectionRuntimeStatusLocked(orgID, connID string) *trueNASConnectionRuntimeStatus {
	if p.statusByOrg[orgID] == nil {
		p.statusByOrg[orgID] = make(map[string]*trueNASConnectionRuntimeStatus)
	}
	if p.statusByOrg[orgID][connID] == nil {
		p.statusByOrg[orgID][connID] = &trueNASConnectionRuntimeStatus{}
	}
	return p.statusByOrg[orgID][connID]
}

func (p *TrueNASPoller) alignConnectionNextPollLocked(
	orgID string,
	connID string,
	previous config.TrueNASInstance,
	next config.TrueNASInstance,
) {
	status := p.ensureConnectionRuntimeStatusLocked(orgID, connID)
	if status == nil || status.lastAttemptAt.IsZero() {
		return
	}
	previousInterval := p.effectiveRuntimePollInterval(previous)
	nextInterval := p.effectiveRuntimePollInterval(next)
	if previousInterval == nextInterval {
		return
	}
	status.nextPollAt = status.lastAttemptAt.Add(nextInterval)
}

func (p *TrueNASPoller) connectionPollDueLocked(
	orgID string,
	connID string,
	instance config.TrueNASInstance,
	now time.Time,
) bool {
	next := p.nextPollAtLocked(orgID, connID, instance)
	return next.IsZero() || !next.After(now)
}

func (p *TrueNASPoller) nextPollAtLocked(orgID string, connID string, instance config.TrueNASInstance) time.Time {
	status := p.ensureConnectionRuntimeStatusLocked(orgID, connID)
	if status == nil {
		return time.Time{}
	}
	if status.nextPollAt.IsZero() && !status.lastAttemptAt.IsZero() {
		status.nextPollAt = status.lastAttemptAt.Add(p.effectiveRuntimePollInterval(instance))
	}
	return status.nextPollAt
}

func (p *TrueNASPoller) effectiveRuntimePollInterval(instance config.TrueNASInstance) time.Duration {
	base := time.Duration(instance.EffectivePollIntervalSecs()) * time.Second
	if p != nil && p.explicitInterval && p.interval > 0 && p.interval < base {
		return p.interval
	}
	return base
}

func (p *TrueNASPoller) recordConnectionSuccessLocked(
	orgID string,
	connID string,
	instance config.TrueNASInstance,
	at time.Time,
	snapshot *truenas.FixtureSnapshot,
) {
	status := p.ensureConnectionRuntimeStatusLocked(orgID, connID)
	status.lastAttemptAt = at
	status.lastSuccessAt = at
	status.lastError = nil
	status.consecutiveFailures = 0
	status.nextPollAt = at.Add(p.effectiveRuntimePollInterval(instance))
	if snapshot != nil {
		status.observed = buildTrueNASObservedSummary(snapshot)
	}
}

func (p *TrueNASPoller) recordConnectionFailureLocked(
	orgID string,
	connID string,
	instance config.TrueNASInstance,
	err error,
	at time.Time,
) {
	status := p.ensureConnectionRuntimeStatusLocked(orgID, connID)
	status.lastAttemptAt = at
	status.consecutiveFailures++
	status.nextPollAt = at.Add(p.effectiveRuntimePollInterval(instance))

	category := ""
	if monitorErr := classifyTrueNASError(err, connID); monitorErr != nil {
		category = string(monitorErr.Type)
	}
	status.lastError = &TrueNASConnectionPollError{
		At:       timePointerIfSet(at),
		Message:  strings.TrimSpace(err.Error()),
		Category: category,
	}
}

// RecordConnectionTestSuccess updates one saved TrueNAS connection summary after
// a manual row-level test without clearing the last observed contribution
// summary.
func (p *TrueNASPoller) RecordConnectionTestSuccess(
	orgID string,
	connID string,
	instance config.TrueNASInstance,
	at time.Time,
) {
	if p == nil {
		return
	}
	connID = strings.TrimSpace(connID)
	if connID == "" {
		return
	}
	orgID = normalizeTrueNASOrgID(orgID)
	instance.ApplyDefaults()

	p.mu.Lock()
	defer p.mu.Unlock()
	p.recordConnectionSuccessLocked(orgID, connID, instance, at, nil)
}

// RecordConnectionTestFailure updates one saved TrueNAS connection summary after
// a manual row-level test failure while preserving any previously observed
// resource contribution summary.
func (p *TrueNASPoller) RecordConnectionTestFailure(
	orgID string,
	connID string,
	instance config.TrueNASInstance,
	err error,
	at time.Time,
) {
	if p == nil || err == nil {
		return
	}
	connID = strings.TrimSpace(connID)
	if connID == "" {
		return
	}
	orgID = normalizeTrueNASOrgID(orgID)
	instance.ApplyDefaults()

	p.mu.Lock()
	defer p.mu.Unlock()
	p.recordConnectionFailureLocked(orgID, connID, instance, err, at)
}

func buildTrueNASObservedSummary(snapshot *truenas.FixtureSnapshot) *TrueNASConnectionObservedSummary {
	if snapshot == nil {
		return nil
	}

	host := strings.TrimSpace(snapshot.System.Hostname)
	resourceID := host
	collectedAt := snapshot.CollectedAt
	if collectedAt.IsZero() {
		collectedAt = snapshot.System.CollectedAt
	}

	summary := &TrueNASConnectionObservedSummary{
		Host:              host,
		ResourceID:        resourceID,
		Systems:           0,
		StoragePools:      len(snapshot.Pools),
		Datasets:          len(snapshot.Datasets),
		Apps:              len(snapshot.Apps),
		Disks:             len(snapshot.Disks),
		RecoveryArtifacts: len(snapshot.ZFSSnapshots) + len(snapshot.ReplicationTasks),
	}
	if host != "" || resourceID != "" {
		summary.Systems = 1
	}
	summary.CollectedAt = timePointerIfSet(collectedAt)
	return summary
}

func normalizeTrueNASOrgID(orgID string) string {
	orgID = strings.TrimSpace(orgID)
	if orgID == "" {
		return "default"
	}
	return orgID
}

func timePointerIfSet(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	copied := value
	return &copied
}

func cloneTimePointer(value *time.Time) *time.Time {
	if value == nil || value.IsZero() {
		return nil
	}
	copied := *value
	return &copied
}

func cloneTrueNASConnectionPollError(value *TrueNASConnectionPollError) *TrueNASConnectionPollError {
	if value == nil {
		return nil
	}
	return &TrueNASConnectionPollError{
		At:       cloneTimePointer(value.At),
		Message:  strings.TrimSpace(value.Message),
		Category: strings.TrimSpace(value.Category),
	}
}

func cloneTrueNASObservedSummary(value *TrueNASConnectionObservedSummary) *TrueNASConnectionObservedSummary {
	if value == nil {
		return nil
	}
	return &TrueNASConnectionObservedSummary{
		Host:              strings.TrimSpace(value.Host),
		ResourceID:        strings.TrimSpace(value.ResourceID),
		CollectedAt:       cloneTimePointer(value.CollectedAt),
		Systems:           value.Systems,
		StoragePools:      value.StoragePools,
		Datasets:          value.Datasets,
		Apps:              value.Apps,
		Disks:             value.Disks,
		RecoveryArtifacts: value.RecoveryArtifacts,
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

// PhysicalDiskTemperatureHistory exposes native TrueNAS disk temperature
// history through the canonical monitoring chart boundary.
func (p *TrueNASPoller) PhysicalDiskTemperatureHistory(_ *Monitor, orgID string, duration time.Duration) map[string][]MetricPoint {
	if p == nil || !truenas.IsFeatureEnabled() {
		return nil
	}

	orgID = strings.TrimSpace(orgID)
	if orgID == "" {
		orgID = "default"
	}

	entries := p.providerEntriesForOrg(orgID)
	if len(entries) == 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultTrueNASHistoryReadTimeout)
	defer cancel()

	history := make(map[string][]MetricPoint)
	for _, entry := range entries {
		nativeHistory, err := entry.provider.PhysicalDiskTemperatureHistory(ctx, duration)
		if err != nil {
			log.Warn().
				Str("component", "truenas_poller").
				Str("action", "disk_temperature_history").
				Str("org_id", orgID).
				Str("connection_id", strings.TrimSpace(entry.connectionID)).
				Err(err).
				Msg("TrueNAS poller failed to read native disk temperature history")
			continue
		}
		for resourceID, points := range nativeHistory {
			if strings.TrimSpace(resourceID) == "" || len(points) == 0 {
				continue
			}
			converted := make([]MetricPoint, len(points))
			for i, point := range points {
				converted[i] = MetricPoint{
					Timestamp: point.Timestamp,
					Value:     point.Value,
				}
			}
			history[resourceID] = converted
		}
	}
	if len(history) == 0 {
		return nil
	}
	return history
}

// GuestMetricHistory exposes native TrueNAS host history through the canonical
// guest-chart boundary when local Pulse history is shallow.
func (p *TrueNASPoller) GuestMetricHistory(_ *Monitor, orgID string, resourceType string, duration time.Duration) map[string]map[string][]MetricPoint {
	if p == nil || !truenas.IsFeatureEnabled() || strings.TrimSpace(resourceType) != "agent" {
		return nil
	}

	orgID = strings.TrimSpace(orgID)
	if orgID == "" {
		orgID = "default"
	}

	entries := p.providerEntriesForOrg(orgID)
	if len(entries) == 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultTrueNASHistoryReadTimeout)
	defer cancel()

	history := make(map[string]map[string][]MetricPoint)
	for _, entry := range entries {
		resourceID, nativeHistory, err := entry.provider.SystemMetricHistory(ctx, duration)
		if err != nil {
			log.Warn().
				Str("component", "truenas_poller").
				Str("action", "guest_metric_history").
				Str("resource_type", resourceType).
				Str("org_id", orgID).
				Str("connection_id", strings.TrimSpace(entry.connectionID)).
				Err(err).
				Msg("TrueNAS poller failed to read native guest history")
			continue
		}
		resourceID = strings.TrimSpace(resourceID)
		if resourceID == "" || len(nativeHistory) == 0 {
			continue
		}

		converted := make(map[string][]MetricPoint, len(nativeHistory))
		for metricType, points := range nativeHistory {
			if strings.TrimSpace(metricType) == "" || len(points) == 0 {
				continue
			}
			series := make([]MetricPoint, len(points))
			for i, point := range points {
				series[i] = MetricPoint{
					Timestamp: point.Timestamp,
					Value:     point.Value,
				}
			}
			converted[metricType] = series
		}
		if len(converted) == 0 {
			continue
		}
		history[resourceID] = converted
	}
	if len(history) == 0 {
		return nil
	}
	return history
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

// SupplementalInventoryReadyAt reports when the current org-scoped TrueNAS
// inventory has reached a settled initial baseline that has to be reflected in
// the canonical monitor store before billing can consume monitored-system
// counts.
func (p *TrueNASPoller) SupplementalInventoryReadyAt(_ *Monitor, orgID string) (time.Time, bool) {
	if p == nil {
		return time.Time{}, true
	}

	orgID = normalizeTrueNASOrgID(orgID)
	configs := p.activeConnectionConfigsForOrg(orgID)
	if len(configs) == 0 {
		return time.Time{}, true
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	var watermark time.Time
	statusByConn := p.statusByOrg[orgID]
	for connID := range configs {
		status := statusByConn[connID]
		if status == nil || status.lastAttemptAt.IsZero() {
			return time.Time{}, false
		}
		if status.lastAttemptAt.After(watermark) {
			watermark = status.lastAttemptAt
		}
	}

	return watermark, true
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

func (p *TrueNASPoller) activeConnectionConfigsForOrg(orgID string) map[string]config.TrueNASInstance {
	orgID = normalizeTrueNASOrgID(orgID)
	if p == nil {
		return nil
	}

	p.mu.Lock()
	cached := cloneTrueNASConnectionConfigMap(p.configsByOrg[orgID])
	p.mu.Unlock()
	if len(cached) > 0 || p.multiTenant == nil {
		return cached
	}

	persistence, err := p.multiTenant.GetPersistence(orgID)
	if err != nil || persistence == nil {
		return cached
	}
	instances, err := persistence.LoadTrueNASConfig()
	if err != nil {
		return cached
	}

	active := make(map[string]config.TrueNASInstance)
	for i := range instances {
		instance := instances[i]
		instance.ApplyDefaults()
		if !instance.Enabled {
			continue
		}
		connID := strings.TrimSpace(instance.ID)
		if connID == "" {
			continue
		}
		active[connID] = instance
	}
	return active
}

func cloneTrueNASConnectionConfigMap(
	in map[string]config.TrueNASInstance,
) map[string]config.TrueNASInstance {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]config.TrueNASInstance, len(in))
	for connID, instance := range in {
		out[connID] = instance
	}
	return out
}

// ControlApp executes a canonical TrueNAS app action for one tenant-scoped
// API-backed app-container resource and refreshes the cached records afterward.
func (p *TrueNASPoller) ControlApp(ctx context.Context, orgID, host, appID, action string) (*truenas.App, error) {
	if p == nil {
		return nil, fmt.Errorf("truenas poller is nil")
	}
	if !truenas.IsFeatureEnabled() {
		return nil, fmt.Errorf("truenas integration is disabled")
	}

	orgID = strings.TrimSpace(orgID)
	if orgID == "" {
		orgID = "default"
	}
	host = strings.TrimSpace(host)
	appID = strings.TrimSpace(appID)
	action = strings.ToLower(strings.TrimSpace(action))
	if appID == "" {
		return nil, fmt.Errorf("truenas app id is required")
	}

	entry, currentApp, err := p.findProviderEntryForApp(orgID, host, appID)
	if err != nil {
		return nil, err
	}

	var nextSnapshot *truenas.FixtureSnapshot
	switch action {
	case "start", "stop":
		nextSnapshot, err = entry.provider.ControlApp(ctx, trueNASAppCanonicalID(*currentApp), action)
	case "restart":
		if trueNASAppRunning(currentApp) {
			if _, err = entry.provider.ControlApp(ctx, trueNASAppCanonicalID(*currentApp), "stop"); err != nil {
				return nil, err
			}
		}
		nextSnapshot, err = entry.provider.ControlApp(ctx, trueNASAppCanonicalID(*currentApp), "start")
	default:
		return nil, fmt.Errorf("unsupported truenas app action %q", action)
	}
	if err != nil {
		return nil, err
	}

	records := entry.provider.Records()
	p.mu.Lock()
	if p.cachedRecordsByOrg[orgID] == nil {
		p.cachedRecordsByOrg[orgID] = make(map[string][]unifiedresources.IngestRecord)
	}
	p.cachedRecordsByOrg[orgID][entry.connectionID] = cloneIngestRecords(records)
	p.mu.Unlock()
	p.ingestRecoveryPoints(ctx, orgID, entry.connectionID, entry.provider)

	if updatedApp, ok := findTrueNASAppSnapshot(nextSnapshot, host, appID); ok {
		appCopy := *updatedApp
		return &appCopy, nil
	}
	appCopy := *currentApp
	return &appCopy, nil
}

// ReadAppLogs executes a canonical bounded log read for one tenant-scoped
// TrueNAS-managed app-container resource.
func (p *TrueNASPoller) ReadAppLogs(ctx context.Context, orgID, host, appID, containerRef string, tailLines int) (*truenas.AppLogResult, error) {
	if p == nil {
		return nil, fmt.Errorf("truenas poller is nil")
	}
	if !truenas.IsFeatureEnabled() {
		return nil, fmt.Errorf("truenas integration is disabled")
	}

	orgID = strings.TrimSpace(orgID)
	if orgID == "" {
		orgID = "default"
	}
	host = strings.TrimSpace(host)
	appID = strings.TrimSpace(appID)
	if appID == "" {
		return nil, fmt.Errorf("truenas app id is required")
	}

	entry, _, err := p.findProviderEntryForApp(orgID, host, appID)
	if err != nil {
		return nil, err
	}
	return entry.provider.ReadAppLogs(ctx, appID, containerRef, tailLines)
}

// GetAppConfig reads the current TrueNAS app configuration/runtime shape for
// one tenant-scoped API-backed app-container resource.
func (p *TrueNASPoller) GetAppConfig(ctx context.Context, orgID, host, appID string) (*truenas.AppConfigResult, error) {
	if p == nil {
		return nil, fmt.Errorf("truenas poller is nil")
	}
	if !truenas.IsFeatureEnabled() {
		return nil, fmt.Errorf("truenas integration is disabled")
	}

	orgID = strings.TrimSpace(orgID)
	if orgID == "" {
		orgID = "default"
	}
	host = strings.TrimSpace(host)
	appID = strings.TrimSpace(appID)
	if appID == "" {
		return nil, fmt.Errorf("truenas app id is required")
	}

	entry, currentApp, err := p.findProviderEntryForApp(orgID, host, appID)
	if err != nil {
		return nil, err
	}

	result, err := entry.provider.GetAppConfig(ctx, trueNASAppCanonicalID(*currentApp))
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, fmt.Errorf("truenas app %q configuration is unavailable", appID)
	}
	return result, nil
}

func cloneIngestRecords(records []unifiedresources.IngestRecord) []unifiedresources.IngestRecord {
	if len(records) == 0 {
		return nil
	}
	cloned := make([]unifiedresources.IngestRecord, len(records))
	copy(cloned, records)
	return cloned
}

func (p *TrueNASPoller) providerEntriesForOrg(orgID string) []trueNASPollerProviderEntry {
	if p == nil {
		return nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	entries := make([]trueNASPollerProviderEntry, 0, len(p.providersByOrg[orgID]))
	for connectionID, provider := range p.providersByOrg[orgID] {
		if provider == nil {
			continue
		}
		entries = append(entries, trueNASPollerProviderEntry{
			connectionID: connectionID,
			provider:     provider,
		})
	}
	return entries
}

func (p *TrueNASPoller) findProviderEntryForApp(orgID, host, appID string) (trueNASPollerProviderEntry, *truenas.App, error) {
	for _, entry := range p.providerEntriesForOrg(orgID) {
		currentSnapshot := entry.provider.Snapshot()
		currentApp, ok := findTrueNASAppSnapshot(currentSnapshot, host, appID)
		if ok {
			return entry, currentApp, nil
		}
	}
	return trueNASPollerProviderEntry{}, nil, fmt.Errorf("truenas app %q was not found for org %q", appID, orgID)
}

func findTrueNASAppSnapshot(snapshot *truenas.FixtureSnapshot, host, appID string) (*truenas.App, bool) {
	if snapshot == nil {
		return nil, false
	}
	if host != "" && !strings.EqualFold(strings.TrimSpace(snapshot.System.Hostname), host) {
		return nil, false
	}
	for i := range snapshot.Apps {
		app := &snapshot.Apps[i]
		if strings.EqualFold(trueNASAppCanonicalID(*app), appID) || strings.EqualFold(strings.TrimSpace(app.Name), appID) {
			return app, true
		}
	}
	return nil, false
}

func trueNASAppCanonicalID(app truenas.App) string {
	if id := strings.TrimSpace(app.ID); id != "" {
		return id
	}
	return strings.TrimSpace(app.Name)
}

func trueNASAppRunning(app *truenas.App) bool {
	if app == nil {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(app.State), "RUNNING") {
		return true
	}
	for _, container := range app.Containers {
		if strings.EqualFold(strings.TrimSpace(container.State), "running") {
			return true
		}
	}
	return false
}

func trueNASProviderConfigChanged(previous, next config.TrueNASInstance) bool {
	return strings.TrimSpace(previous.Host) != strings.TrimSpace(next.Host) ||
		previous.Port != next.Port ||
		strings.TrimSpace(previous.APIKey) != strings.TrimSpace(next.APIKey) ||
		strings.TrimSpace(previous.Username) != strings.TrimSpace(next.Username) ||
		strings.TrimSpace(previous.Password) != strings.TrimSpace(next.Password) ||
		previous.UseHTTPS != next.UseHTTPS ||
		previous.InsecureSkipVerify != next.InsecureSkipVerify ||
		strings.TrimSpace(previous.Fingerprint) != strings.TrimSpace(next.Fingerprint)
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
