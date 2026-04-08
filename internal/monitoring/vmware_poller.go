package monitoring

import (
	"context"
	"errors"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/internal/vmware"
	"github.com/rs/zerolog/log"
)

const defaultVMwarePollInterval = 60 * time.Second

// VMwareConnectionPollError captures the last runtime poll error for one
// configured VMware connection.
type VMwareConnectionPollError struct {
	At       *time.Time `json:"at,omitempty"`
	Message  string     `json:"message,omitempty"`
	Category string     `json:"category,omitempty"`
}

// VMwareConnectionPollStatus summarizes the runtime polling state for one
// configured VMware connection.
type VMwareConnectionPollStatus struct {
	IntervalSeconds     int                        `json:"intervalSeconds"`
	LastAttemptAt       *time.Time                 `json:"lastAttemptAt,omitempty"`
	LastSuccessAt       *time.Time                 `json:"lastSuccessAt,omitempty"`
	ConsecutiveFailures int                        `json:"consecutiveFailures,omitempty"`
	LastError           *VMwareConnectionPollError `json:"lastError,omitempty"`
}

// VMwareConnectionObservedSummary summarizes the canonical resources the most
// recent successful poll contributed for one configured VMware connection.
type VMwareConnectionObservedSummary struct {
	CollectedAt *time.Time                      `json:"collectedAt,omitempty"`
	Hosts       int                             `json:"hosts"`
	VMs         int                             `json:"vms"`
	Datastores  int                             `json:"datastores"`
	VIRelease   string                          `json:"viRelease,omitempty"`
	Degraded    bool                            `json:"degraded,omitempty"`
	IssueCount  int                             `json:"issueCount,omitempty"`
	Issues      []VMwareConnectionObservedIssue `json:"issues,omitempty"`
}

// VMwareConnectionObservedIssue summarizes one class of optional VMware reads
// that degraded an otherwise successful inventory refresh.
type VMwareConnectionObservedIssue struct {
	Stage       string `json:"stage,omitempty"`
	Category    string `json:"category,omitempty"`
	Message     string `json:"message,omitempty"`
	Occurrences int    `json:"occurrences,omitempty"`
}

// VMwareConnectionSummary merges poll health with the most recent discovered
// platform contribution for one configured VMware connection.
type VMwareConnectionSummary struct {
	Poll     *VMwareConnectionPollStatus      `json:"poll,omitempty"`
	Observed *VMwareConnectionObservedSummary `json:"observed,omitempty"`
}

type vmwareConnectionRuntimeStatus struct {
	lastAttemptAt       time.Time
	lastSuccessAt       time.Time
	lastError           *VMwareConnectionPollError
	consecutiveFailures int
	observed            *VMwareConnectionObservedSummary
	observedIssueKey    string
}

type vmwareObservedTransition struct {
	action     string
	message    string
	issueCount int
}

type vmwarePollerProvider interface {
	Refresh(ctx context.Context) error
	Records() []unifiedresources.IngestRecord
	ActivityChanges() []unifiedresources.ResourceChange
	Snapshot() *vmware.InventorySnapshot
	Close()
}

// VMwarePoller manages periodic polling of configured VMware vCenter connections.
type VMwarePoller struct {
	multiTenant *config.MultiTenantPersistence

	mu sync.Mutex
	// providersByOrg is keyed by orgID, then connection ID.
	providersByOrg map[string]map[string]vmwarePollerProvider
	// configsByOrg is keyed by orgID, then connection ID.
	configsByOrg map[string]map[string]config.VMwareVCenterInstance
	// statusByOrg is keyed by orgID, then connection ID.
	statusByOrg map[string]map[string]*vmwareConnectionRuntimeStatus
	// cachedRecordsByOrg is keyed by orgID, then connection ID.
	cachedRecordsByOrg map[string]map[string][]unifiedresources.IngestRecord
	// cachedChangesByOrg is keyed by orgID, then connection ID.
	cachedChangesByOrg map[string]map[string][]unifiedresources.ResourceChange
	newProvider        func(config.VMwareVCenterInstance) (vmwarePollerProvider, error)
	cancel             context.CancelFunc
	stopped            chan struct{}
	interval           time.Duration
	explicitInterval   bool
}

type vmwarePollerProviderEntry struct {
	orgID        string
	connectionID string
	provider     vmwarePollerProvider
}

// NewVMwarePoller builds a new VMware poller with the provided interval.
func NewVMwarePoller(multiTenant *config.MultiTenantPersistence, interval time.Duration) *VMwarePoller {
	explicitInterval := interval > 0
	if interval <= 0 {
		interval = defaultVMwarePollInterval
	}

	stopped := make(chan struct{})
	close(stopped)

	poller := &VMwarePoller{
		multiTenant:        multiTenant,
		providersByOrg:     make(map[string]map[string]vmwarePollerProvider),
		configsByOrg:       make(map[string]map[string]config.VMwareVCenterInstance),
		statusByOrg:        make(map[string]map[string]*vmwareConnectionRuntimeStatus),
		cachedRecordsByOrg: make(map[string]map[string][]unifiedresources.IngestRecord),
		cachedChangesByOrg: make(map[string]map[string][]unifiedresources.ResourceChange),
		stopped:            stopped,
		interval:           interval,
		explicitInterval:   explicitInterval,
	}
	poller.newProvider = poller.defaultProvider
	return poller
}

// Start begins periodic VMware polling if the feature flag is enabled.
func (p *VMwarePoller) Start(ctx context.Context) {
	if p == nil || !vmware.IsFeatureEnabled() {
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
func (p *VMwarePoller) Stop() {
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
	case <-timer.C:
		log.Warn().
			Str("component", "vmware_poller").
			Str("action", "stop").
			Dur("timeout", 5*time.Second).
			Msg("VMware poller stop timed out waiting for shutdown")
	}
}

func (p *VMwarePoller) syncConnections() {
	if p == nil {
		return
	}
	if p.multiTenant == nil {
		log.Warn().
			Str("component", "vmware_poller").
			Str("action", "sync_connections").
			Msg("VMware poller cannot sync connections because multi-tenant persistence is nil")
		return
	}

	orgs, err := p.multiTenant.ListOrganizations()
	if err != nil {
		log.Warn().
			Str("component", "vmware_poller").
			Str("action", "list_organizations").
			Err(err).
			Msg("VMware poller failed to list organizations")
		return
	}

	active := make(map[string]map[string]config.VMwareVCenterInstance, len(orgs))
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
				Str("component", "vmware_poller").
				Str("action", "get_persistence").
				Str("org_id", orgID).
				Err(err).
				Msg("VMware poller failed to open tenant persistence")
			continue
		}

		instances, err := persistence.LoadVMwareConfig()
		if err != nil {
			log.Warn().
				Str("component", "vmware_poller").
				Str("action", "load_vmware_config").
				Str("org_id", orgID).
				Err(err).
				Msg("VMware poller failed to load tenant VMware config")
			continue
		}

		activeByConnection := make(map[string]config.VMwareVCenterInstance)
		for _, instance := range instances {
			instance.ApplyDefaults()
			if !instance.Enabled {
				continue
			}
			connectionID := strings.TrimSpace(instance.ID)
			if connectionID == "" {
				continue
			}
			activeByConnection[connectionID] = instance
		}
		active[orgID] = activeByConnection
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	for orgID, connectionMap := range active {
		if p.providersByOrg[orgID] == nil {
			p.providersByOrg[orgID] = make(map[string]vmwarePollerProvider)
		}
		if p.configsByOrg[orgID] == nil {
			p.configsByOrg[orgID] = make(map[string]config.VMwareVCenterInstance)
		}
		if p.cachedRecordsByOrg[orgID] == nil {
			p.cachedRecordsByOrg[orgID] = make(map[string][]unifiedresources.IngestRecord)
		}
		if p.cachedChangesByOrg[orgID] == nil {
			p.cachedChangesByOrg[orgID] = make(map[string][]unifiedresources.ResourceChange)
		}
		if p.statusByOrg[orgID] == nil {
			p.statusByOrg[orgID] = make(map[string]*vmwareConnectionRuntimeStatus)
		}

		for connectionID, instance := range connectionMap {
			existingConfig, exists := p.configsByOrg[orgID][connectionID]
			if exists && existingConfig == instance && p.providersByOrg[orgID][connectionID] != nil {
				continue
			}

			if provider := p.providersByOrg[orgID][connectionID]; provider != nil {
				provider.Close()
			}

			provider, err := p.newProvider(instance)
			if err != nil {
				log.Warn().
					Str("component", "vmware_poller").
					Str("action", "build_provider").
					Str("org_id", orgID).
					Str("connection_id", connectionID).
					Err(err).
					Msg("VMware poller failed to build provider")
				delete(p.providersByOrg[orgID], connectionID)
				delete(p.cachedRecordsByOrg[orgID], connectionID)
				delete(p.cachedChangesByOrg[orgID], connectionID)
				p.configsByOrg[orgID][connectionID] = instance
				continue
			}

			p.providersByOrg[orgID][connectionID] = provider
			p.configsByOrg[orgID][connectionID] = instance
		}
	}

	for orgID, providers := range p.providersByOrg {
		activeConnections := active[orgID]
		for connectionID, provider := range providers {
			if _, ok := activeConnections[connectionID]; ok {
				continue
			}
			if provider != nil {
				provider.Close()
			}
			delete(providers, connectionID)
			if p.cachedRecordsByOrg[orgID] != nil {
				delete(p.cachedRecordsByOrg[orgID], connectionID)
			}
			if p.cachedChangesByOrg[orgID] != nil {
				delete(p.cachedChangesByOrg[orgID], connectionID)
			}
			if p.configsByOrg[orgID] != nil {
				delete(p.configsByOrg[orgID], connectionID)
			}
			if p.statusByOrg[orgID] != nil {
				delete(p.statusByOrg[orgID], connectionID)
			}
		}

		if len(providers) == 0 {
			delete(p.providersByOrg, orgID)
			delete(p.configsByOrg, orgID)
			delete(p.statusByOrg, orgID)
			delete(p.cachedRecordsByOrg, orgID)
			delete(p.cachedChangesByOrg, orgID)
		}
	}
}

func (p *VMwarePoller) pollAll(ctx context.Context) {
	for _, entry := range p.providerEntries() {
		p.pollConnection(ctx, entry)
	}
}

func (p *VMwarePoller) pollConnection(ctx context.Context, entry vmwarePollerProviderEntry) {
	if entry.provider == nil {
		return
	}

	pollCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	at := time.Now().UTC()
	if err := entry.provider.Refresh(pollCtx); err != nil {
		p.mu.Lock()
		p.recordConnectionFailureLocked(entry.orgID, entry.connectionID, err, at)
		p.mu.Unlock()
		log.Warn().
			Str("component", "vmware_poller").
			Str("action", "refresh").
			Str("org_id", entry.orgID).
			Str("connection_id", entry.connectionID).
			Err(err).
			Msg("VMware poller refresh failed")
		return
	}

	records := cloneIngestRecords(entry.provider.Records())
	changes := cloneResourceChanges(entry.provider.ActivityChanges())
	snapshot := entry.provider.Snapshot()

	p.mu.Lock()
	transition := p.recordConnectionSuccessLocked(entry.orgID, entry.connectionID, at, snapshot)
	if p.cachedRecordsByOrg[entry.orgID] == nil {
		p.cachedRecordsByOrg[entry.orgID] = make(map[string][]unifiedresources.IngestRecord)
	}
	if p.cachedChangesByOrg[entry.orgID] == nil {
		p.cachedChangesByOrg[entry.orgID] = make(map[string][]unifiedresources.ResourceChange)
	}
	p.cachedRecordsByOrg[entry.orgID][entry.connectionID] = records
	p.cachedChangesByOrg[entry.orgID][entry.connectionID] = changes
	p.mu.Unlock()

	if transition != nil {
		event := log.Info()
		if strings.HasPrefix(transition.action, "refresh_partial") {
			event = log.Warn()
		}
		event.
			Str("component", "vmware_poller").
			Str("action", transition.action).
			Str("org_id", entry.orgID).
			Str("connection_id", entry.connectionID).
			Int("issue_count", transition.issueCount).
			Msg(transition.message)
	}
}

func (p *VMwarePoller) providerEntries() []vmwarePollerProviderEntry {
	if p == nil {
		return nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	var entries []vmwarePollerProviderEntry
	for orgID, providers := range p.providersByOrg {
		for connectionID, provider := range providers {
			if provider == nil {
				continue
			}
			entries = append(entries, vmwarePollerProviderEntry{
				orgID:        orgID,
				connectionID: connectionID,
				provider:     provider,
			})
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].orgID == entries[j].orgID {
			return entries[i].connectionID < entries[j].connectionID
		}
		return entries[i].orgID < entries[j].orgID
	})
	return entries
}

func (p *VMwarePoller) defaultProvider(instance config.VMwareVCenterInstance) (vmwarePollerProvider, error) {
	client, err := vmware.NewClient(vmware.ClientConfig{
		Host:               instance.Host,
		Port:               instance.Port,
		Username:           instance.Username,
		Password:           instance.Password,
		InsecureSkipVerify: instance.InsecureSkipVerify,
		Timeout:            20 * time.Second,
	})
	if err != nil {
		return nil, err
	}
	return vmware.NewAPIProvider(vmware.ProviderMetadata{
		ConnectionID:   strings.TrimSpace(instance.ID),
		ConnectionName: strings.TrimSpace(instance.Name),
		VCenterHost:    strings.TrimSpace(instance.Host),
	}, client), nil
}

func (p *VMwarePoller) nextWaitDuration(now time.Time) time.Duration {
	if p == nil || p.interval <= 0 {
		return defaultVMwarePollInterval
	}
	return p.interval
}

// ConnectionSummaries returns per-connection runtime poll health and discovered
// contribution summaries for the supplied VMware settings records.
func (p *VMwarePoller) ConnectionSummaries(orgID string, instances []config.VMwareVCenterInstance) map[string]VMwareConnectionSummary {
	if len(instances) == 0 {
		return nil
	}

	orgID = normalizeVMwareOrgID(orgID)
	summaries := make(map[string]VMwareConnectionSummary, len(instances))
	intervalSeconds := defaultVMwareConnectionPollIntervalSeconds()
	if p != nil {
		intervalSeconds = p.connectionPollIntervalSeconds()
	}

	for i := range instances {
		instance := instances[i]
		instance.ApplyDefaults()
		connID := strings.TrimSpace(instance.ID)
		if connID == "" {
			continue
		}

		summary := VMwareConnectionSummary{
			Poll: &VMwareConnectionPollStatus{
				IntervalSeconds: int(intervalSeconds),
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
				summary.Poll.LastError = cloneVMwareConnectionPollError(status.LastError)
				summary.Observed = cloneVMwareObservedSummary(status.Observed)
			}
		}

		summaries[connID] = summary
	}

	return summaries
}

func (p *VMwarePoller) connectionPollIntervalSeconds() int {
	if p == nil {
		return defaultVMwareConnectionPollIntervalSeconds()
	}
	if p.interval < time.Second {
		return defaultVMwareConnectionPollIntervalSeconds()
	}
	return int(p.interval / time.Second)
}

func defaultVMwareConnectionPollIntervalSeconds() int {
	return int(defaultVMwarePollInterval / time.Second)
}

// GetCurrentRecords returns the latest known VMware records for the default org.
func (p *VMwarePoller) GetCurrentRecords() []unifiedresources.IngestRecord {
	return p.GetCurrentRecordsForOrg("default")
}

// SupplementalRecords implements the monitor supplemental-record provider contract.
func (p *VMwarePoller) SupplementalRecords(_ *Monitor, orgID string) []unifiedresources.IngestRecord {
	return p.GetCurrentRecordsForOrg(orgID)
}

// SupplementalChanges implements the monitor supplemental-change provider contract.
func (p *VMwarePoller) SupplementalChanges(_ *Monitor, orgID string) []unifiedresources.ResourceChange {
	return p.GetCurrentChangesForOrg(orgID)
}

// SnapshotOwnedSources declares that VMware records are ingested through the
// supplemental resource path rather than legacy monitor snapshot slices.
func (p *VMwarePoller) SnapshotOwnedSources() []unifiedresources.DataSource {
	return []unifiedresources.DataSource{unifiedresources.SourceVMware}
}

// SnapshotOwnedSourcesForOrg is the tenant-aware variant of SnapshotOwnedSources.
func (p *VMwarePoller) SnapshotOwnedSourcesForOrg(string) []unifiedresources.DataSource {
	return []unifiedresources.DataSource{unifiedresources.SourceVMware}
}

// SupplementalInventoryReadyAt reports when the current org-scoped VMware
// inventory has reached a settled initial baseline that has to be reflected in
// the canonical monitor store before billing can consume monitored-system
// counts.
func (p *VMwarePoller) SupplementalInventoryReadyAt(_ *Monitor, orgID string) (time.Time, bool) {
	if p == nil {
		return time.Time{}, true
	}

	orgID = normalizeVMwareOrgID(orgID)
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

// GetCurrentRecordsForOrg returns the latest known VMware records for the specified org.
func (p *VMwarePoller) GetCurrentRecordsForOrg(orgID string) []unifiedresources.IngestRecord {
	if p == nil {
		return nil
	}

	orgID = strings.TrimSpace(orgID)
	if orgID == "" {
		orgID = "default"
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	recordsByConnection := p.cachedRecordsByOrg[orgID]
	if len(recordsByConnection) == 0 {
		return nil
	}

	connectionIDs := make([]string, 0, len(recordsByConnection))
	for connectionID := range recordsByConnection {
		connectionIDs = append(connectionIDs, connectionID)
	}
	sort.Strings(connectionIDs)

	total := 0
	for _, connectionID := range connectionIDs {
		total += len(recordsByConnection[connectionID])
	}
	if total == 0 {
		return nil
	}

	records := make([]unifiedresources.IngestRecord, 0, total)
	for _, connectionID := range connectionIDs {
		records = append(records, cloneIngestRecords(recordsByConnection[connectionID])...)
	}
	return records
}

func (p *VMwarePoller) activeConnectionConfigsForOrg(orgID string) map[string]config.VMwareVCenterInstance {
	orgID = normalizeVMwareOrgID(orgID)
	if p == nil {
		return nil
	}

	p.mu.Lock()
	cached := cloneVMwareConnectionConfigMap(p.configsByOrg[orgID])
	p.mu.Unlock()
	if len(cached) > 0 || p.multiTenant == nil {
		return cached
	}

	persistence, err := p.multiTenant.GetPersistence(orgID)
	if err != nil || persistence == nil {
		return cached
	}
	instances, err := persistence.LoadVMwareConfig()
	if err != nil {
		return cached
	}

	active := make(map[string]config.VMwareVCenterInstance)
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

func cloneVMwareConnectionConfigMap(
	in map[string]config.VMwareVCenterInstance,
) map[string]config.VMwareVCenterInstance {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]config.VMwareVCenterInstance, len(in))
	for connID, instance := range in {
		out[connID] = instance
	}
	return out
}

// GetCurrentChangesForOrg returns the latest known VMware timeline changes for the specified org.
func (p *VMwarePoller) GetCurrentChangesForOrg(orgID string) []unifiedresources.ResourceChange {
	if p == nil {
		return nil
	}

	orgID = strings.TrimSpace(orgID)
	if orgID == "" {
		orgID = "default"
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	changesByConnection := p.cachedChangesByOrg[orgID]
	if len(changesByConnection) == 0 {
		return nil
	}

	connectionIDs := make([]string, 0, len(changesByConnection))
	for connectionID := range changesByConnection {
		connectionIDs = append(connectionIDs, connectionID)
	}
	sort.Strings(connectionIDs)

	total := 0
	for _, connectionID := range connectionIDs {
		total += len(changesByConnection[connectionID])
	}
	if total == 0 {
		return nil
	}

	changes := make([]unifiedresources.ResourceChange, 0, total)
	for _, connectionID := range connectionIDs {
		changes = append(changes, cloneResourceChanges(changesByConnection[connectionID])...)
	}
	return changes
}

func cloneResourceChanges(in []unifiedresources.ResourceChange) []unifiedresources.ResourceChange {
	if in == nil {
		return nil
	}
	out := make([]unifiedresources.ResourceChange, len(in))
	for i := range in {
		out[i] = in[i]
		if in[i].OccurredAt != nil {
			occurredAt := in[i].OccurredAt.UTC()
			out[i].OccurredAt = &occurredAt
		}
		if in[i].RelatedResources != nil {
			out[i].RelatedResources = append([]string(nil), in[i].RelatedResources...)
		}
		if in[i].Metadata != nil {
			out[i].Metadata = make(map[string]any, len(in[i].Metadata))
			for key, value := range in[i].Metadata {
				out[i].Metadata[key] = value
			}
		}
	}
	return out
}

func (p *VMwarePoller) cloneRuntimeStatusLocked(orgID string, connID string) *vmwareConnectionRuntimeSnapshot {
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
	return &vmwareConnectionRuntimeSnapshot{
		LastAttemptAt:       timePointerIfSet(status.lastAttemptAt),
		LastSuccessAt:       timePointerIfSet(status.lastSuccessAt),
		ConsecutiveFailures: status.consecutiveFailures,
		LastError:           cloneVMwareConnectionPollError(status.lastError),
		Observed:            cloneVMwareObservedSummary(status.observed),
	}
}

type vmwareConnectionRuntimeSnapshot struct {
	LastAttemptAt       *time.Time
	LastSuccessAt       *time.Time
	ConsecutiveFailures int
	LastError           *VMwareConnectionPollError
	Observed            *VMwareConnectionObservedSummary
}

func (p *VMwarePoller) ensureConnectionRuntimeStatusLocked(orgID, connID string) *vmwareConnectionRuntimeStatus {
	if p.statusByOrg[orgID] == nil {
		p.statusByOrg[orgID] = make(map[string]*vmwareConnectionRuntimeStatus)
	}
	if p.statusByOrg[orgID][connID] == nil {
		p.statusByOrg[orgID][connID] = &vmwareConnectionRuntimeStatus{}
	}
	return p.statusByOrg[orgID][connID]
}

func (p *VMwarePoller) recordConnectionSuccessLocked(orgID, connID string, at time.Time, snapshot *vmware.InventorySnapshot) *vmwareObservedTransition {
	status := p.ensureConnectionRuntimeStatusLocked(orgID, connID)
	status.lastAttemptAt = at
	status.lastSuccessAt = at
	status.lastError = nil
	status.consecutiveFailures = 0
	if snapshot != nil {
		nextObserved := buildVMwareObservedSummary(snapshot)
		nextIssueKey := summarizeVMwareObservedIssueKey(snapshot.EnrichmentIssues)
		transition := classifyVMwareObservedTransition(status.observed, status.observedIssueKey, nextObserved, nextIssueKey)
		status.observed = nextObserved
		status.observedIssueKey = nextIssueKey
		return transition
	}
	return nil
}

func (p *VMwarePoller) recordConnectionFailureLocked(orgID, connID string, err error, at time.Time) {
	if err == nil {
		return
	}
	status := p.ensureConnectionRuntimeStatusLocked(orgID, connID)
	status.lastAttemptAt = at
	status.consecutiveFailures++
	status.lastError = &VMwareConnectionPollError{
		At:       timePointerIfSet(at),
		Message:  strings.TrimSpace(err.Error()),
		Category: classifyVMwarePollError(err),
	}
}

// RecordConnectionTestSuccess updates one saved VMware connection summary after
// a manual row-level test using the same canonical runtime health owner as the
// live poller.
func (p *VMwarePoller) RecordConnectionTestSuccess(orgID, connID string, summary *vmware.InventorySummary, at time.Time) {
	if p == nil {
		return
	}
	connID = strings.TrimSpace(connID)
	if connID == "" {
		return
	}
	orgID = normalizeVMwareOrgID(orgID)

	p.mu.Lock()
	defer p.mu.Unlock()
	status := p.ensureConnectionRuntimeStatusLocked(orgID, connID)
	status.lastAttemptAt = at
	status.lastSuccessAt = at
	status.lastError = nil
	status.consecutiveFailures = 0
	if summary != nil {
		status.observed = &VMwareConnectionObservedSummary{
			CollectedAt: timePointerIfSet(at),
			Hosts:       summary.Hosts,
			VMs:         summary.VMs,
			Datastores:  summary.Datastores,
			VIRelease:   strings.TrimSpace(summary.VIRelease),
		}
		status.observedIssueKey = ""
	}
}

// RecordConnectionTestFailure updates one saved VMware connection summary after
// a manual row-level test failure while preserving previously observed
// contribution summary.
func (p *VMwarePoller) RecordConnectionTestFailure(orgID, connID string, err error, at time.Time) {
	if p == nil || err == nil {
		return
	}
	connID = strings.TrimSpace(connID)
	if connID == "" {
		return
	}
	orgID = normalizeVMwareOrgID(orgID)

	p.mu.Lock()
	defer p.mu.Unlock()
	p.recordConnectionFailureLocked(orgID, connID, err, at)
}

func buildVMwareObservedSummary(snapshot *vmware.InventorySnapshot) *VMwareConnectionObservedSummary {
	if snapshot == nil {
		return nil
	}
	collectedAt := snapshot.CollectedAt
	summary := &VMwareConnectionObservedSummary{
		CollectedAt: timePointerIfSet(collectedAt),
		Hosts:       len(snapshot.Hosts),
		VMs:         len(snapshot.VMs),
		Datastores:  len(snapshot.Datastores),
		VIRelease:   strings.TrimSpace(snapshot.VIRelease),
	}
	if len(snapshot.EnrichmentIssues) > 0 {
		summary.Degraded = true
		summary.IssueCount = len(snapshot.EnrichmentIssues)
		summary.Issues = summarizeVMwareObservedIssues(snapshot.EnrichmentIssues)
	}
	return summary
}

func classifyVMwareObservedTransition(previous *VMwareConnectionObservedSummary, previousIssueKey string, next *VMwareConnectionObservedSummary, nextIssueKey string) *vmwareObservedTransition {
	prevDegraded := previous != nil && previous.Degraded
	nextDegraded := next != nil && next.Degraded

	switch {
	case !prevDegraded && nextDegraded:
		return &vmwareObservedTransition{
			action:     "refresh_partial",
			message:    "VMware poller refreshed base inventory with degraded optional enrichment",
			issueCount: next.IssueCount,
		}
	case prevDegraded && nextDegraded && previousIssueKey != nextIssueKey:
		return &vmwareObservedTransition{
			action:     "refresh_partial_changed",
			message:    "VMware poller degraded optional enrichment changed",
			issueCount: next.IssueCount,
		}
	case prevDegraded && !nextDegraded:
		return &vmwareObservedTransition{
			action:  "refresh_recovered",
			message: "VMware poller optional enrichment recovered",
		}
	default:
		return nil
	}
}

func classifyVMwarePollError(err error) string {
	if err == nil {
		return ""
	}
	var connectionErr *vmware.ConnectionError
	if errors.As(err, &connectionErr) && connectionErr != nil {
		return strings.TrimSpace(connectionErr.Category)
	}
	return ""
}

func normalizeVMwareOrgID(orgID string) string {
	orgID = strings.TrimSpace(orgID)
	if orgID == "" {
		return "default"
	}
	return orgID
}

func cloneVMwareConnectionPollError(value *VMwareConnectionPollError) *VMwareConnectionPollError {
	if value == nil {
		return nil
	}
	return &VMwareConnectionPollError{
		At:       cloneTimePointer(value.At),
		Message:  strings.TrimSpace(value.Message),
		Category: strings.TrimSpace(value.Category),
	}
}

func cloneVMwareObservedSummary(value *VMwareConnectionObservedSummary) *VMwareConnectionObservedSummary {
	if value == nil {
		return nil
	}
	return &VMwareConnectionObservedSummary{
		CollectedAt: cloneTimePointer(value.CollectedAt),
		Hosts:       value.Hosts,
		VMs:         value.VMs,
		Datastores:  value.Datastores,
		VIRelease:   strings.TrimSpace(value.VIRelease),
		Degraded:    value.Degraded,
		IssueCount:  value.IssueCount,
		Issues:      cloneVMwareObservedIssues(value.Issues),
	}
}

func summarizeVMwareObservedIssues(issues []vmware.InventoryEnrichmentIssue) []VMwareConnectionObservedIssue {
	if len(issues) == 0 {
		return nil
	}

	type aggregate struct {
		issue VMwareConnectionObservedIssue
	}

	byKey := make(map[string]*aggregate, len(issues))
	for _, issue := range issues {
		key := strings.ToLower(strings.TrimSpace(issue.Stage)) + "\x00" +
			strings.ToLower(strings.TrimSpace(issue.Category)) + "\x00" +
			strings.ToLower(strings.TrimSpace(issue.Message))
		entry := byKey[key]
		if entry == nil {
			entry = &aggregate{
				issue: VMwareConnectionObservedIssue{
					Stage:    strings.TrimSpace(issue.Stage),
					Category: strings.TrimSpace(issue.Category),
					Message:  strings.TrimSpace(issue.Message),
				},
			}
			byKey[key] = entry
		}
		entry.issue.Occurrences++
	}

	summary := make([]VMwareConnectionObservedIssue, 0, len(byKey))
	for _, entry := range byKey {
		summary = append(summary, entry.issue)
	}
	sort.Slice(summary, func(i, j int) bool {
		if summary[i].Occurrences != summary[j].Occurrences {
			return summary[i].Occurrences > summary[j].Occurrences
		}
		if summary[i].Stage != summary[j].Stage {
			return summary[i].Stage < summary[j].Stage
		}
		if summary[i].Category != summary[j].Category {
			return summary[i].Category < summary[j].Category
		}
		return summary[i].Message < summary[j].Message
	})
	if len(summary) > 3 {
		summary = summary[:3]
	}
	return summary
}

func summarizeVMwareObservedIssueKey(issues []vmware.InventoryEnrichmentIssue) string {
	if len(issues) == 0 {
		return ""
	}

	counts := make(map[string]int, len(issues))
	for _, issue := range issues {
		key := strings.ToLower(strings.TrimSpace(issue.Stage)) + "\x00" +
			strings.ToLower(strings.TrimSpace(issue.Category)) + "\x00" +
			strings.ToLower(strings.TrimSpace(issue.Message))
		counts[key]++
	}

	parts := make([]string, 0, len(counts))
	for key, count := range counts {
		parts = append(parts, key+"\x00"+strconv.Itoa(count))
	}
	sort.Strings(parts)
	return strings.Join(parts, "\n")
}

func cloneVMwareObservedIssues(in []VMwareConnectionObservedIssue) []VMwareConnectionObservedIssue {
	if in == nil {
		return nil
	}
	out := make([]VMwareConnectionObservedIssue, len(in))
	copy(out, in)
	for i := range out {
		out[i].Stage = strings.TrimSpace(out[i].Stage)
		out[i].Category = strings.TrimSpace(out[i].Category)
		out[i].Message = strings.TrimSpace(out[i].Message)
	}
	return out
}
