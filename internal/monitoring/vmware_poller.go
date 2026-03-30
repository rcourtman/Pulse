package monitoring

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/internal/vmware"
	"github.com/rs/zerolog/log"
)

const defaultVMwarePollInterval = 60 * time.Second

type vmwarePollerProvider interface {
	Refresh(ctx context.Context) error
	Records() []unifiedresources.IngestRecord
	ActivityChanges() []unifiedresources.ResourceChange
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
		}

		if len(providers) == 0 {
			delete(p.providersByOrg, orgID)
			delete(p.configsByOrg, orgID)
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

	if err := entry.provider.Refresh(pollCtx); err != nil {
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

	p.mu.Lock()
	if p.cachedRecordsByOrg[entry.orgID] == nil {
		p.cachedRecordsByOrg[entry.orgID] = make(map[string][]unifiedresources.IngestRecord)
	}
	if p.cachedChangesByOrg[entry.orgID] == nil {
		p.cachedChangesByOrg[entry.orgID] = make(map[string][]unifiedresources.ResourceChange)
	}
	p.cachedRecordsByOrg[entry.orgID][entry.connectionID] = records
	p.cachedChangesByOrg[entry.orgID][entry.connectionID] = changes
	p.mu.Unlock()
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
