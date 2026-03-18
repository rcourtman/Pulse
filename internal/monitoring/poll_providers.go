package monitoring

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// PollProvider defines how a monitoring backend participates in scheduling and
// task execution.
type PollProvider interface {
	Type() InstanceType
	ListInstances(m *Monitor) []string
	BaseInterval(m *Monitor) time.Duration
	BuildPollTask(m *Monitor, instanceName string) (PollTask, error)
}

// PollProviderInstanceInfo describes a provider instance for health/status APIs.
type PollProviderInstanceInfo struct {
	Name        string
	DisplayName string
	Connection  string
	Metadata    map[string]string
}

// InstanceInfoPollProvider is an optional PollProvider extension for providers
// that expose instance metadata for scheduler health responses.
type InstanceInfoPollProvider interface {
	DescribeInstances(m *Monitor) []PollProviderInstanceInfo
}

// ConnectionStatusPollProvider is an optional PollProvider extension that
// publishes connection status entries keyed by external node ID (for example:
// "pve-node-a", "pbs-backup-1", "xcp-cluster-1").
type ConnectionStatusPollProvider interface {
	ConnectionStatuses(m *Monitor) map[string]bool
}

// ConnectionHealthKeyPollProvider is an optional PollProvider extension for
// provider-owned connection health key normalization in monitor state.
type ConnectionHealthKeyPollProvider interface {
	ConnectionHealthKey(m *Monitor, instanceName string) string
}

// SupplementalRecordsPollProvider is an optional PollProvider extension for
// providers that can emit source-native unified ingest records.
type SupplementalRecordsPollProvider interface {
	SupplementalSource() unifiedresources.DataSource
	SupplementalRecords(m *Monitor, orgID string) []unifiedresources.IngestRecord
}

// SnapshotOwnedSourcesPollProvider is an optional SupplementalRecordsPollProvider
// extension for providers that fully own source-native resource ingest.
// When set, legacy snapshot slices for these sources are suppressed.
type SnapshotOwnedSourcesPollProvider interface {
	SnapshotOwnedSources(m *Monitor) []unifiedresources.DataSource
}

type pollProviderAdapter struct {
	instanceType      InstanceType
	listInstances     func(*Monitor) []string
	describeInstances func(*Monitor) []PollProviderInstanceInfo
	connectionStatus  func(*Monitor) map[string]bool
	connectionKey     func(*Monitor, string) string
	baseInterval      func(*Monitor) time.Duration
	buildPollTask     func(*Monitor, string) (PollTask, error)
}

func (p pollProviderAdapter) Type() InstanceType {
	return p.instanceType
}

func (p pollProviderAdapter) ListInstances(m *Monitor) []string {
	if p.listInstances == nil {
		return nil
	}
	return p.listInstances(m)
}

func (p pollProviderAdapter) DescribeInstances(m *Monitor) []PollProviderInstanceInfo {
	if p.describeInstances == nil {
		return nil
	}
	return p.describeInstances(m)
}

func (p pollProviderAdapter) ConnectionStatuses(m *Monitor) map[string]bool {
	if p.connectionStatus == nil {
		return nil
	}
	return p.connectionStatus(m)
}

func (p pollProviderAdapter) ConnectionHealthKey(m *Monitor, instanceName string) string {
	if p.connectionKey == nil {
		return ""
	}
	return p.connectionKey(m, instanceName)
}

func (p pollProviderAdapter) BaseInterval(m *Monitor) time.Duration {
	if p.baseInterval == nil {
		return 0
	}
	return p.baseInterval(m)
}

func (p pollProviderAdapter) BuildPollTask(m *Monitor, instanceName string) (PollTask, error) {
	if p.buildPollTask == nil {
		return PollTask{}, fmt.Errorf("provider %q does not support poll task construction", p.instanceType)
	}
	return p.buildPollTask(m, instanceName)
}

func newPVEPollProvider() PollProvider {
	return pollProviderAdapter{
		instanceType: InstanceTypePVE,
		listInstances: func(m *Monitor) []string {
			if m == nil {
				return nil
			}
			m.mu.RLock()
			names := make([]string, 0, len(m.pveClients))
			for name := range m.pveClients {
				names = append(names, name)
			}
			m.mu.RUnlock()
			sort.Strings(names)
			return names
		},
		describeInstances: func(m *Monitor) []PollProviderInstanceInfo {
			if m == nil {
				return nil
			}

			byName := make(map[string]PollProviderInstanceInfo)
			m.mu.RLock()
			if m.config != nil {
				for _, inst := range m.config.PVEInstances {
					name := strings.TrimSpace(inst.Name)
					if name == "" {
						name = strings.TrimSpace(inst.Host)
					}
					if name == "" {
						name = "pve-instance"
					}

					display := strings.TrimSpace(inst.Name)
					if display == "" {
						display = name
					}
					connection := strings.TrimSpace(inst.Host)
					byName[name] = PollProviderInstanceInfo{
						Name:        name,
						DisplayName: display,
						Connection:  connection,
					}
				}
			}
			for name := range m.pveClients {
				trimmed := strings.TrimSpace(name)
				if trimmed == "" {
					continue
				}
				if _, exists := byName[trimmed]; exists {
					continue
				}
				byName[trimmed] = PollProviderInstanceInfo{Name: trimmed, DisplayName: trimmed}
			}
			m.mu.RUnlock()

			if len(byName) == 0 {
				return nil
			}
			names := make([]string, 0, len(byName))
			for name := range byName {
				names = append(names, name)
			}
			sort.Strings(names)
			infos := make([]PollProviderInstanceInfo, 0, len(names))
			for _, name := range names {
				infos = append(infos, byName[name])
			}
			return infos
		},
		connectionStatus: func(m *Monitor) map[string]bool {
			if m == nil {
				return nil
			}

			statuses := make(map[string]bool)
			m.mu.RLock()
			if m.config != nil {
				for _, inst := range m.config.PVEInstances {
					name := strings.TrimSpace(inst.Name)
					if name == "" {
						name = strings.TrimSpace(inst.Host)
					}
					if name == "" {
						continue
					}
					key := "pve-" + name
					connected := false
					if client, exists := m.pveClients[name]; exists && client != nil {
						if m.state != nil && m.state.ConnectionHealth != nil {
							stateKey := m.connectionHealthStateKey(InstanceTypePVE, name)
							connected = m.state.ConnectionHealth[stateKey]
						} else {
							connected = true
						}
					}
					statuses[key] = connected
				}
			}
			m.mu.RUnlock()
			if len(statuses) == 0 {
				return nil
			}
			return statuses
		},
		connectionKey: func(_ *Monitor, instanceName string) string {
			return strings.TrimSpace(instanceName)
		},
		baseInterval: func(m *Monitor) time.Duration {
			if m == nil {
				return 0
			}
			return m.effectivePVEPollingInterval()
		},
		buildPollTask: func(m *Monitor, instanceName string) (PollTask, error) {
			if m == nil {
				return PollTask{}, fmt.Errorf("monitor is nil")
			}
			client, ok := m.getPVEClient(instanceName)
			if !ok || client == nil {
				return PollTask{}, fmt.Errorf("PVE client missing for scheduled task")
			}
			return PollTask{
				InstanceName: instanceName,
				InstanceType: string(InstanceTypePVE),
				PVEClient:    client,
			}, nil
		},
	}
}

func newPBSPollProvider() PollProvider {
	return pollProviderAdapter{
		instanceType: InstanceTypePBS,
		listInstances: func(m *Monitor) []string {
			if m == nil {
				return nil
			}
			m.mu.RLock()
			names := make([]string, 0, len(m.pbsClients))
			for name := range m.pbsClients {
				names = append(names, name)
			}
			m.mu.RUnlock()
			sort.Strings(names)
			return names
		},
		describeInstances: func(m *Monitor) []PollProviderInstanceInfo {
			if m == nil {
				return nil
			}

			byName := make(map[string]PollProviderInstanceInfo)
			m.mu.RLock()
			if m.config != nil {
				for _, inst := range m.config.PBSInstances {
					name := strings.TrimSpace(inst.Name)
					if name == "" {
						name = strings.TrimSpace(inst.Host)
					}
					if name == "" {
						name = "pbs-instance"
					}

					display := strings.TrimSpace(inst.Name)
					if display == "" {
						display = name
					}
					connection := strings.TrimSpace(inst.Host)
					byName[name] = PollProviderInstanceInfo{
						Name:        name,
						DisplayName: display,
						Connection:  connection,
					}
				}
			}
			for name := range m.pbsClients {
				trimmed := strings.TrimSpace(name)
				if trimmed == "" {
					continue
				}
				if _, exists := byName[trimmed]; exists {
					continue
				}
				byName[trimmed] = PollProviderInstanceInfo{Name: trimmed, DisplayName: trimmed}
			}
			m.mu.RUnlock()

			if len(byName) == 0 {
				return nil
			}
			names := make([]string, 0, len(byName))
			for name := range byName {
				names = append(names, name)
			}
			sort.Strings(names)
			infos := make([]PollProviderInstanceInfo, 0, len(names))
			for _, name := range names {
				infos = append(infos, byName[name])
			}
			return infos
		},
		connectionStatus: func(m *Monitor) map[string]bool {
			if m == nil {
				return nil
			}

			statuses := make(map[string]bool)
			m.mu.RLock()
			if m.config != nil {
				for _, inst := range m.config.PBSInstances {
					name := strings.TrimSpace(inst.Name)
					if name == "" {
						name = strings.TrimSpace(inst.Host)
					}
					if name == "" {
						continue
					}
					key := "pbs-" + name
					connected := false
					if client, exists := m.pbsClients[name]; exists && client != nil {
						if m.state != nil && m.state.ConnectionHealth != nil {
							stateKey := m.connectionHealthStateKey(InstanceTypePBS, name)
							connected = m.state.ConnectionHealth[stateKey]
						} else {
							connected = true
						}
					}
					statuses[key] = connected
				}
			}
			m.mu.RUnlock()
			if len(statuses) == 0 {
				return nil
			}
			return statuses
		},
		connectionKey: func(_ *Monitor, instanceName string) string {
			trimmed := strings.TrimSpace(instanceName)
			if trimmed == "" {
				return ""
			}
			return "pbs-" + trimmed
		},
		baseInterval: func(m *Monitor) time.Duration {
			if m == nil || m.config == nil {
				return 0
			}
			return clampInterval(m.config.PBSPollingInterval, 10*time.Second, time.Hour)
		},
		buildPollTask: func(m *Monitor, instanceName string) (PollTask, error) {
			if m == nil {
				return PollTask{}, fmt.Errorf("monitor is nil")
			}
			client, ok := m.getPBSClient(instanceName)
			if !ok || client == nil {
				return PollTask{}, fmt.Errorf("PBS client missing for scheduled task")
			}
			return PollTask{
				InstanceName: instanceName,
				InstanceType: string(InstanceTypePBS),
				PBSClient:    client,
			}, nil
		},
	}
}

func newPMGPollProvider() PollProvider {
	return pollProviderAdapter{
		instanceType: InstanceTypePMG,
		listInstances: func(m *Monitor) []string {
			if m == nil {
				return nil
			}
			m.mu.RLock()
			names := make([]string, 0, len(m.pmgClients))
			for name := range m.pmgClients {
				names = append(names, name)
			}
			m.mu.RUnlock()
			sort.Strings(names)
			return names
		},
		describeInstances: func(m *Monitor) []PollProviderInstanceInfo {
			if m == nil {
				return nil
			}

			byName := make(map[string]PollProviderInstanceInfo)
			m.mu.RLock()
			if m.config != nil {
				for _, inst := range m.config.PMGInstances {
					name := strings.TrimSpace(inst.Name)
					if name == "" {
						name = strings.TrimSpace(inst.Host)
					}
					if name == "" {
						name = "pmg-instance"
					}

					display := strings.TrimSpace(inst.Name)
					if display == "" {
						display = name
					}
					connection := strings.TrimSpace(inst.Host)
					byName[name] = PollProviderInstanceInfo{
						Name:        name,
						DisplayName: display,
						Connection:  connection,
					}
				}
			}
			for name := range m.pmgClients {
				trimmed := strings.TrimSpace(name)
				if trimmed == "" {
					continue
				}
				if _, exists := byName[trimmed]; exists {
					continue
				}
				byName[trimmed] = PollProviderInstanceInfo{Name: trimmed, DisplayName: trimmed}
			}
			m.mu.RUnlock()

			if len(byName) == 0 {
				return nil
			}
			names := make([]string, 0, len(byName))
			for name := range byName {
				names = append(names, name)
			}
			sort.Strings(names)
			infos := make([]PollProviderInstanceInfo, 0, len(names))
			for _, name := range names {
				infos = append(infos, byName[name])
			}
			return infos
		},
		connectionStatus: func(m *Monitor) map[string]bool {
			if m == nil {
				return nil
			}

			statuses := make(map[string]bool)
			m.mu.RLock()
			if m.config != nil {
				for _, inst := range m.config.PMGInstances {
					name := strings.TrimSpace(inst.Name)
					if name == "" {
						name = strings.TrimSpace(inst.Host)
					}
					if name == "" {
						continue
					}
					key := "pmg-" + name
					connected := false
					if client, exists := m.pmgClients[name]; exists && client != nil {
						if m.state != nil && m.state.ConnectionHealth != nil {
							stateKey := m.connectionHealthStateKey(InstanceTypePMG, name)
							connected = m.state.ConnectionHealth[stateKey]
						} else {
							connected = true
						}
					}
					statuses[key] = connected
				}
			}
			m.mu.RUnlock()
			if len(statuses) == 0 {
				return nil
			}
			return statuses
		},
		connectionKey: func(_ *Monitor, instanceName string) string {
			trimmed := strings.TrimSpace(instanceName)
			if trimmed == "" {
				return ""
			}
			return "pmg-" + trimmed
		},
		baseInterval: func(m *Monitor) time.Duration {
			if m == nil || m.config == nil {
				return 0
			}
			return clampInterval(m.config.PMGPollingInterval, 10*time.Second, time.Hour)
		},
		buildPollTask: func(m *Monitor, instanceName string) (PollTask, error) {
			if m == nil {
				return PollTask{}, fmt.Errorf("monitor is nil")
			}
			client, ok := m.getPMGClient(instanceName)
			if !ok || client == nil {
				return PollTask{}, fmt.Errorf("PMG client missing for scheduled task")
			}
			return PollTask{
				InstanceName: instanceName,
				InstanceType: string(InstanceTypePMG),
				PMGClient:    client,
			}, nil
		},
	}
}

// RegisterPollProvider registers or replaces a polling provider for an
// instance type.
func (m *Monitor) RegisterPollProvider(provider PollProvider) error {
	if m == nil {
		return fmt.Errorf("monitor is nil")
	}
	if provider == nil {
		return fmt.Errorf("poll provider is nil")
	}

	providerType := InstanceType(strings.TrimSpace(string(provider.Type())))
	if providerType == "" {
		return fmt.Errorf("poll provider type is required")
	}

	m.mu.Lock()
	if m.pollProviders == nil {
		m.pollProviders = make(map[InstanceType]PollProvider)
	}
	m.pollProviders[providerType] = provider
	m.mu.Unlock()

	m.refreshInstanceInfoCacheFromProviders()
	return nil
}

func (m *Monitor) registerBuiltInPollProviders() {
	_ = m.RegisterPollProvider(newPVEPollProvider())
	_ = m.RegisterPollProvider(newPBSPollProvider())
	_ = m.RegisterPollProvider(newPMGPollProvider())
}

func (m *Monitor) pollProviderSnapshot() []PollProvider {
	if m == nil {
		return nil
	}

	m.mu.RLock()
	if len(m.pollProviders) == 0 {
		m.mu.RUnlock()
		return nil
	}
	byType := make(map[InstanceType]PollProvider, len(m.pollProviders))
	for providerType, provider := range m.pollProviders {
		if provider == nil {
			continue
		}
		if providerType == "" {
			providerType = provider.Type()
		}
		if providerType == "" {
			continue
		}
		byType[providerType] = provider
	}
	m.mu.RUnlock()

	if len(byType) == 0 {
		return nil
	}

	types := make([]string, 0, len(byType))
	for providerType := range byType {
		types = append(types, string(providerType))
	}
	sort.Strings(types)

	providers := make([]PollProvider, 0, len(types))
	for _, providerType := range types {
		providers = append(providers, byType[InstanceType(providerType)])
	}
	return providers
}

func (m *Monitor) pollProviderSnapshotWithBuiltins() []PollProvider {
	byType := make(map[InstanceType]PollProvider)

	for _, provider := range m.pollProviderSnapshot() {
		if provider == nil {
			continue
		}
		providerType := provider.Type()
		if providerType == "" {
			continue
		}
		byType[providerType] = provider
	}

	builtins := []PollProvider{
		newPVEPollProvider(),
		newPBSPollProvider(),
		newPMGPollProvider(),
	}
	for _, provider := range builtins {
		if provider == nil {
			continue
		}
		providerType := provider.Type()
		if providerType == "" {
			continue
		}
		if _, exists := byType[providerType]; !exists {
			byType[providerType] = provider
		}
	}

	if len(byType) == 0 {
		return nil
	}

	types := make([]string, 0, len(byType))
	for providerType := range byType {
		types = append(types, string(providerType))
	}
	sort.Strings(types)

	providers := make([]PollProvider, 0, len(types))
	for _, providerType := range types {
		providers = append(providers, byType[InstanceType(providerType)])
	}

	return providers
}

func (m *Monitor) getPollProvider(instanceType InstanceType) PollProvider {
	if m == nil {
		return nil
	}

	m.mu.RLock()
	provider := m.pollProviders[instanceType]
	m.mu.RUnlock()
	if provider != nil {
		return provider
	}

	switch instanceType {
	case InstanceTypePVE:
		return newPVEPollProvider()
	case InstanceTypePBS:
		return newPBSPollProvider()
	case InstanceTypePMG:
		return newPMGPollProvider()
	default:
		return nil
	}
}

func (m *Monitor) activeSchedulerKeys() map[string]struct{} {
	activeKeys := make(map[string]struct{})
	for _, provider := range m.pollProviderSnapshotWithBuiltins() {
		if provider == nil {
			continue
		}
		instanceType := provider.Type()
		for _, instanceName := range provider.ListInstances(m) {
			name := strings.TrimSpace(instanceName)
			if name == "" {
				continue
			}
			activeKeys[schedulerKey(instanceType, name)] = struct{}{}
		}
	}
	return activeKeys
}

func cloneProviderMetadata(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func (m *Monitor) providerInstanceInfos() map[string]*instanceInfo {
	if m == nil {
		return nil
	}

	infos := make(map[string]*instanceInfo)
	for _, provider := range m.pollProviderSnapshotWithBuiltins() {
		if provider == nil {
			continue
		}

		providerType := provider.Type()
		if strings.TrimSpace(string(providerType)) == "" {
			continue
		}

		addInfo := func(name, display, connection string, metadata map[string]string) {
			name = strings.TrimSpace(name)
			if name == "" {
				return
			}
			key := schedulerKey(providerType, name)
			display = strings.TrimSpace(display)
			connection = strings.TrimSpace(connection)
			if display == "" {
				display = name
			}

			existing, ok := infos[key]
			if !ok || existing == nil {
				infos[key] = &instanceInfo{
					Key:         key,
					Type:        providerType,
					DisplayName: display,
					Connection:  connection,
					Metadata:    cloneProviderMetadata(metadata),
				}
				return
			}

			if existing.Type == "" {
				existing.Type = providerType
			}
			if existing.DisplayName == "" && display != "" {
				existing.DisplayName = display
			}
			if existing.Connection == "" && connection != "" {
				existing.Connection = connection
			}
			if len(metadata) == 0 {
				return
			}
			if existing.Metadata == nil {
				existing.Metadata = make(map[string]string, len(metadata))
			}
			for mk, mv := range metadata {
				existing.Metadata[mk] = mv
			}
		}

		if describer, ok := provider.(InstanceInfoPollProvider); ok {
			for _, info := range describer.DescribeInstances(m) {
				addInfo(info.Name, info.DisplayName, info.Connection, info.Metadata)
			}
		}

		for _, name := range provider.ListInstances(m) {
			addInfo(name, name, "", nil)
		}
	}

	if len(infos) == 0 {
		return nil
	}
	return infos
}

func (m *Monitor) refreshInstanceInfoCacheFromProviders() {
	if m == nil {
		return
	}

	infos := m.providerInstanceInfos()

	m.mu.Lock()
	if infos == nil {
		m.instanceInfoCache = make(map[string]*instanceInfo)
	} else {
		m.instanceInfoCache = infos
	}
	m.mu.Unlock()
}

func (m *Monitor) connectionHealthStateKey(instanceType InstanceType, instanceName string) string {
	trimmedName := strings.TrimSpace(instanceName)
	if trimmedName == "" {
		return ""
	}

	if provider := m.getPollProvider(instanceType); provider != nil {
		if keyProvider, ok := provider.(ConnectionHealthKeyPollProvider); ok {
			if key := strings.TrimSpace(keyProvider.ConnectionHealthKey(m, trimmedName)); key != "" {
				return key
			}
		}
	}

	trimmedType := strings.TrimSpace(string(instanceType))
	if trimmedType == "" {
		return trimmedName
	}
	return trimmedType + "-" + trimmedName
}

func (m *Monitor) setProviderConnectionHealth(instanceType InstanceType, instanceName string, healthy bool) {
	if m == nil || m.state == nil {
		return
	}
	key := m.connectionHealthStateKey(instanceType, instanceName)
	if key == "" {
		return
	}
	m.state.SetConnectionHealth(key, healthy)
}

func (m *Monitor) removeProviderConnectionHealth(instanceType InstanceType, instanceName string) {
	if m == nil || m.state == nil {
		return
	}
	key := m.connectionHealthStateKey(instanceType, instanceName)
	if key == "" {
		return
	}
	m.state.RemoveConnectionHealth(key)
}

func (m *Monitor) providerConnectionStatuses(provider PollProvider) map[string]bool {
	if m == nil || provider == nil {
		return nil
	}

	if statusProvider, ok := provider.(ConnectionStatusPollProvider); ok {
		statuses := statusProvider.ConnectionStatuses(m)
		out := make(map[string]bool, len(statuses))
		for key, connected := range statuses {
			trimmed := strings.TrimSpace(key)
			if trimmed == "" {
				continue
			}
			out[trimmed] = connected
		}
		if len(out) > 0 {
			return out
		}
	}

	instanceType := strings.TrimSpace(string(provider.Type()))
	if instanceType == "" {
		return nil
	}

	names := provider.ListInstances(m)
	if len(names) == 0 {
		return nil
	}

	connectionHealth := make(map[string]bool)
	m.mu.RLock()
	if m.state != nil && m.state.ConnectionHealth != nil {
		connectionHealth = make(map[string]bool, len(m.state.ConnectionHealth))
		for key, healthy := range m.state.ConnectionHealth {
			connectionHealth[key] = healthy
		}
	}
	m.mu.RUnlock()

	statuses := make(map[string]bool, len(names))
	for _, name := range names {
		trimmedName := strings.TrimSpace(name)
		if trimmedName == "" {
			continue
		}

		key := instanceType + "-" + trimmedName
		stateKey := m.connectionHealthStateKey(InstanceType(instanceType), trimmedName)
		if stateKey == "" {
			stateKey = trimmedName
		}
		connected := false
		if healthy, exists := connectionHealth[stateKey]; exists {
			connected = healthy
		} else if healthy, exists := connectionHealth[key]; exists {
			connected = healthy
		} else if healthy, exists := connectionHealth[trimmedName]; exists {
			connected = healthy
		}
		statuses[key] = connected
	}

	if len(statuses) == 0 {
		return nil
	}
	return statuses
}

func (m *Monitor) providerOwnedSnapshotSources() []unifiedresources.DataSource {
	owned := make(map[string]unifiedresources.DataSource)

	providers := m.pollProviderSnapshotWithBuiltins()
	for _, provider := range providers {
		if provider == nil {
			continue
		}

		// Snapshot source ownership is only meaningful for providers that can
		// emit source-native records.
		if _, ok := provider.(SupplementalRecordsPollProvider); !ok {
			continue
		}
		owner, ok := provider.(SnapshotOwnedSourcesPollProvider)
		if !ok {
			continue
		}

		for _, source := range owner.SnapshotOwnedSources(m) {
			key := strings.ToLower(strings.TrimSpace(string(source)))
			if key == "" {
				continue
			}
			owned[key] = unifiedresources.DataSource(key)
		}
	}

	orgID := "default"
	if m != nil {
		if trimmed := strings.TrimSpace(m.orgID); trimmed != "" {
			orgID = trimmed
		}
	}
	for _, provider := range m.supplementalProviderSnapshot() {
		if provider == nil {
			continue
		}

		var sources []unifiedresources.DataSource
		if tenantOwner, ok := provider.(interface {
			SnapshotOwnedSourcesForOrg(string) []unifiedresources.DataSource
		}); ok {
			sources = tenantOwner.SnapshotOwnedSourcesForOrg(orgID)
		} else if owner, ok := provider.(interface {
			SnapshotOwnedSources() []unifiedresources.DataSource
		}); ok {
			sources = owner.SnapshotOwnedSources()
		}

		for _, source := range sources {
			key := strings.ToLower(strings.TrimSpace(string(source)))
			if key == "" {
				continue
			}
			owned[key] = unifiedresources.DataSource(key)
		}
	}

	if len(owned) == 0 {
		return nil
	}

	keys := make([]string, 0, len(owned))
	for key := range owned {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	sources := make([]unifiedresources.DataSource, 0, len(keys))
	for _, key := range keys {
		sources = append(sources, owned[key])
	}
	return sources
}

func (m *Monitor) supplementalProviderSnapshot() map[unifiedresources.DataSource]MonitorSupplementalRecordsProvider {
	if m == nil {
		return nil
	}

	m.mu.RLock()
	if len(m.supplementalProviders) == 0 {
		m.mu.RUnlock()
		return nil
	}
	snapshot := make(map[unifiedresources.DataSource]MonitorSupplementalRecordsProvider, len(m.supplementalProviders))
	for source, provider := range m.supplementalProviders {
		if provider == nil {
			continue
		}
		normalized := unifiedresources.DataSource(strings.ToLower(strings.TrimSpace(string(source))))
		if normalized == "" {
			continue
		}
		snapshot[normalized] = provider
	}
	m.mu.RUnlock()

	if len(snapshot) == 0 {
		return nil
	}
	return snapshot
}

func (m *Monitor) collectSupplementalRecordsBySource() map[unifiedresources.DataSource][]unifiedresources.IngestRecord {
	providers := m.pollProviderSnapshotWithBuiltins()
	manualProviders := m.supplementalProviderSnapshot()
	if len(providers) == 0 && len(manualProviders) == 0 {
		return nil
	}

	orgID := "default"
	if m != nil {
		if trimmed := strings.TrimSpace(m.orgID); trimmed != "" {
			orgID = trimmed
		}
	}

	recordsBySource := make(map[unifiedresources.DataSource][]unifiedresources.IngestRecord)
	for _, provider := range providers {
		supplementalProvider, ok := provider.(SupplementalRecordsPollProvider)
		if !ok {
			continue
		}
		source := supplementalProvider.SupplementalSource()
		if strings.TrimSpace(string(source)) == "" {
			continue
		}
		records := supplementalProvider.SupplementalRecords(m, orgID)
		if len(records) == 0 {
			continue
		}
		recordsBySource[source] = append(recordsBySource[source], records...)
	}

	if len(recordsBySource) == 0 {
		recordsBySource = nil
	}

	if len(manualProviders) > 0 {
		sources := make([]string, 0, len(manualProviders))
		for source := range manualProviders {
			sources = append(sources, string(source))
		}
		sort.Strings(sources)

		if recordsBySource == nil {
			recordsBySource = make(map[unifiedresources.DataSource][]unifiedresources.IngestRecord, len(sources))
		}
		for _, sourceName := range sources {
			source := unifiedresources.DataSource(sourceName)
			provider := manualProviders[source]
			if provider == nil {
				continue
			}
			records := provider.SupplementalRecords(m, orgID)
			if len(records) == 0 {
				continue
			}
			recordsBySource[source] = append(recordsBySource[source], records...)
		}
	}

	if len(recordsBySource) == 0 {
		return nil
	}
	return recordsBySource
}
