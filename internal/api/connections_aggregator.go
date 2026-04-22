package api

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	pulseutils "github.com/rcourtman/pulse-go-rewrite/internal/utils"
)

// connectionStaleThreshold is the baseline "haven't heard from this connection
// recently" cutoff used to transition `active` → `stale`. Per-type poll
// intervals vary (PVE 5-10s, TrueNAS 60s, agents 30-60s); 2 minutes sits
// comfortably above 2× the slowest default so we don't flap on a single
// dropped tick. Refined later once per-type intervals become first-class.
const connectionStaleThreshold = 2 * time.Minute

// connectionAuthErrorPattern matches the error strings pollers surface when a
// credential is wrong or the token lacks scope. Kept centrally so
// state-derivation stays the same across types.
var connectionAuthErrorPattern = regexp.MustCompile(`(?i)401|403|unauthori[sz]ed|forbidden|authentication|permission denied|invalid (credentials|token|api key)`)

// aggregatorInputs bundles everything the aggregator reads. Separating inputs
// from the handler makes the aggregator unit-testable without spinning up a
// monitor or persistence layer.
type aggregatorInputs struct {
	pveInstances         []config.PVEInstance
	pbsInstances         []config.PBSInstance
	pmgInstances         []config.PMGInstance
	vmwareInstances      []config.VMwareVCenterInstance
	truenasInstances     []config.TrueNASInstance
	hosts                []models.Host
	instanceHealth       map[string]monitoring.InstanceHealth
	expectedAgentVersion string
	now                  time.Time
}

// buildConnections produces a stable, sorted list of connection rows across
// every supported infrastructure type. The function is pure — it does not
// perform any I/O and does not mutate its inputs.
func buildConnections(in aggregatorInputs) []Connection {
	now := in.now
	if now.IsZero() {
		now = time.Now()
	}

	out := make([]Connection, 0,
		len(in.pveInstances)+len(in.pbsInstances)+len(in.pmgInstances)+
			len(in.vmwareInstances)+len(in.truenasInstances)+len(in.hosts))

	for _, pve := range in.pveInstances {
		out = append(out, buildPVEConnection(pve, in.instanceHealth, now))
	}
	for _, pbs := range in.pbsInstances {
		out = append(out, buildPBSConnection(pbs, in.instanceHealth, now))
	}
	for _, pmg := range in.pmgInstances {
		out = append(out, buildPMGConnection(pmg, in.instanceHealth, now))
	}
	for _, vmw := range in.vmwareInstances {
		out = append(out, buildVMwareConnection(vmw, in.instanceHealth, now))
	}
	for _, tn := range in.truenasInstances {
		out = append(out, buildTrueNASConnection(tn, in.instanceHealth, now))
	}
	for _, host := range in.hosts {
		out = append(out, buildAgentConnection(host, in.expectedAgentVersion, now))
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Type != out[j].Type {
			return out[i].Type < out[j].Type
		}
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})

	return out
}

func buildPVEConnection(inst config.PVEInstance, health map[string]monitoring.InstanceHealth, now time.Time) Connection {
	enabled := !inst.Disabled
	surfaces := []string{"vms", "containers", "storage", "backups"}
	scope := map[string]bool{
		"vms":        inst.MonitorVMs,
		"containers": inst.MonitorContainers,
		"storage":    inst.MonitorStorage,
		"backups":    inst.MonitorBackups,
	}
	h := health["pve::"+inst.Name]
	state, reason, lastSeen, lastError := deriveConnectionState(enabled, h, now)
	return Connection{
		ID:           "pve:" + inst.Name,
		Type:         ConnectionTypePVE,
		Name:         inst.Name,
		Address:      inst.Host,
		State:        state,
		StateReason:  reason,
		Enabled:      enabled,
		Surfaces:     surfaces,
		Scope:        scope,
		LastSeen:     lastSeen,
		LastError:    lastError,
		Source:       sourceFromString(inst.Source),
		Capabilities: ConnectionCapabilities{SupportsPause: true, SupportsScope: true, SupportsTest: true},
	}
}

func buildPBSConnection(inst config.PBSInstance, health map[string]monitoring.InstanceHealth, now time.Time) Connection {
	enabled := !inst.Disabled
	surfaces := []string{"backups", "datastores", "syncJobs", "verifyJobs", "pruneJobs", "garbageJobs"}
	scope := map[string]bool{
		"backups":     inst.MonitorBackups,
		"datastores":  inst.MonitorDatastores,
		"syncJobs":    inst.MonitorSyncJobs,
		"verifyJobs":  inst.MonitorVerifyJobs,
		"pruneJobs":   inst.MonitorPruneJobs,
		"garbageJobs": inst.MonitorGarbageJobs,
	}
	h := health["pbs::"+inst.Name]
	state, reason, lastSeen, lastError := deriveConnectionState(enabled, h, now)
	return Connection{
		ID:           "pbs:" + inst.Name,
		Type:         ConnectionTypePBS,
		Name:         inst.Name,
		Address:      inst.Host,
		State:        state,
		StateReason:  reason,
		Enabled:      enabled,
		Surfaces:     surfaces,
		Scope:        scope,
		LastSeen:     lastSeen,
		LastError:    lastError,
		Source:       sourceFromString(inst.Source),
		Capabilities: ConnectionCapabilities{SupportsPause: true, SupportsScope: true, SupportsTest: true},
	}
}

func buildPMGConnection(inst config.PMGInstance, health map[string]monitoring.InstanceHealth, now time.Time) Connection {
	enabled := !inst.Disabled
	surfaces := []string{"mailStats", "queues", "quarantine", "domainStats"}
	scope := map[string]bool{
		"mailStats":   inst.MonitorMailStats,
		"queues":      inst.MonitorQueues,
		"quarantine":  inst.MonitorQuarantine,
		"domainStats": inst.MonitorDomainStats,
	}
	h := health["pmg::"+inst.Name]
	state, reason, lastSeen, lastError := deriveConnectionState(enabled, h, now)
	return Connection{
		ID:           "pmg:" + inst.Name,
		Type:         ConnectionTypePMG,
		Name:         inst.Name,
		Address:      inst.Host,
		State:        state,
		StateReason:  reason,
		Enabled:      enabled,
		Surfaces:     surfaces,
		Scope:        scope,
		LastSeen:     lastSeen,
		LastError:    lastError,
		Source:       ConnectionSourceManual,
		Capabilities: ConnectionCapabilities{SupportsPause: true, SupportsScope: true, SupportsTest: true},
	}
}

func buildVMwareConnection(inst config.VMwareVCenterInstance, health map[string]monitoring.InstanceHealth, now time.Time) Connection {
	enabled := inst.Enabled
	surfaces := []string{"vms", "hosts", "datastores"}
	scope := map[string]bool{
		"vms":        inst.MonitorVMs,
		"hosts":      inst.MonitorHosts,
		"datastores": inst.MonitorDatastores,
	}
	h := health["vmware::"+inst.ID]
	state, reason, lastSeen, lastError := deriveConnectionState(enabled, h, now)
	port := inst.Port
	if port == 0 {
		port = 443
	}
	return Connection{
		ID:           "vmware:" + inst.ID,
		Type:         ConnectionTypeVMware,
		Name:         inst.Name,
		Address:      fmt.Sprintf("https://%s:%d", inst.Host, port),
		State:        state,
		StateReason:  reason,
		Enabled:      enabled,
		Surfaces:     surfaces,
		Scope:        scope,
		LastSeen:     lastSeen,
		LastError:    lastError,
		Source:       ConnectionSourceManual,
		Capabilities: ConnectionCapabilities{SupportsPause: true, SupportsScope: true, SupportsTest: true},
	}
}

func buildTrueNASConnection(inst config.TrueNASInstance, health map[string]monitoring.InstanceHealth, now time.Time) Connection {
	enabled := inst.Enabled
	surfaces := []string{"datasets", "pools", "replication"}
	scope := map[string]bool{
		"datasets":    inst.MonitorDatasets,
		"pools":       inst.MonitorPools,
		"replication": inst.MonitorReplication,
	}
	h := health["truenas::"+inst.ID]
	state, reason, lastSeen, lastError := deriveConnectionState(enabled, h, now)
	scheme := "https"
	if !inst.UseHTTPS {
		scheme = "http"
	}
	port := inst.Port
	if port == 0 {
		if inst.UseHTTPS {
			port = 443
		} else {
			port = 80
		}
	}
	return Connection{
		ID:           "truenas:" + inst.ID,
		Type:         ConnectionTypeTrueNAS,
		Name:         inst.Name,
		Address:      fmt.Sprintf("%s://%s:%d", scheme, inst.Host, port),
		State:        state,
		StateReason:  reason,
		Enabled:      enabled,
		Surfaces:     surfaces,
		Scope:        scope,
		LastSeen:     lastSeen,
		LastError:    lastError,
		Source:       ConnectionSourceManual,
		Capabilities: ConnectionCapabilities{SupportsPause: true, SupportsScope: true, SupportsTest: true},
	}
}

// buildAgentConnection derives a connection row from an agent Host record.
// Agents have no pause toggle and no scope — reports are all-or-nothing —
// so capability flags are off.
func buildAgentConnection(host models.Host, expectedAgentVersion string, now time.Time) Connection {
	name := host.DisplayName
	if strings.TrimSpace(name) == "" {
		name = host.Hostname
	}
	if strings.TrimSpace(name) == "" {
		name = host.ID
	}
	address := host.Hostname
	if strings.TrimSpace(address) == "" {
		address = host.ReportIP
	}

	var lastSeen *time.Time
	if !host.LastSeen.IsZero() {
		t := host.LastSeen
		lastSeen = &t
	}

	state := ConnectionStatePending
	reason := ""
	currentAgentVersion := strings.TrimSpace(host.AgentVersion)
	expectedAgentVersion = strings.TrimSpace(expectedAgentVersion)
	updateAvailable := false
	if currentAgentVersion != "" && expectedAgentVersion != "" {
		updateAvailable = pulseutils.CompareVersions(currentAgentVersion, expectedAgentVersion) < 0
	}
	switch {
	case lastSeen == nil:
		state = ConnectionStatePending
	case now.Sub(*lastSeen) > connectionStaleThreshold:
		state = ConnectionStateStale
		reason = fmt.Sprintf("no heartbeat in %s", now.Sub(*lastSeen).Round(time.Second))
	default:
		state = ConnectionStateActive
	}

	return Connection{
		ID:                   "agent:" + host.ID,
		Type:                 ConnectionTypeAgent,
		Name:                 name,
		Address:              address,
		State:                state,
		StateReason:          reason,
		Enabled:              true,
		Surfaces:             []string{"host"},
		Scope:                map[string]bool{"host": true},
		LastSeen:             lastSeen,
		LastError:            nil,
		Source:               ConnectionSourceAgent,
		AgentVersion:         currentAgentVersion,
		ExpectedAgentVersion: expectedAgentVersion,
		AgentUpdateAvailable: updateAvailable,
		Capabilities:         ConnectionCapabilities{SupportsPause: false, SupportsScope: false, SupportsTest: false},
	}
}

// deriveConnectionState maps (Enabled, InstanceHealth) onto the unified state
// vocabulary. No new state is persisted — the inputs come from the existing
// monitoring scheduler.
func deriveConnectionState(enabled bool, h monitoring.InstanceHealth, now time.Time) (ConnectionState, string, *time.Time, *ConnectionError) {
	var lastSeen *time.Time
	if h.PollStatus.LastSuccess != nil && !h.PollStatus.LastSuccess.IsZero() {
		t := *h.PollStatus.LastSuccess
		lastSeen = &t
	}

	var lastError *ConnectionError
	if h.PollStatus.LastError != nil && h.PollStatus.LastError.Message != "" {
		lastError = &ConnectionError{
			At:       h.PollStatus.LastError.At,
			Message:  h.PollStatus.LastError.Message,
			Category: h.PollStatus.LastError.Category,
		}
	}

	if !enabled {
		return ConnectionStatePaused, "paused by user", lastSeen, lastError
	}

	if lastSeen == nil && lastError == nil {
		return ConnectionStatePending, "awaiting first poll", nil, nil
	}

	if lastError != nil && connectionAuthErrorPattern.MatchString(lastError.Message) {
		return ConnectionStateUnauthorized, lastError.Message, lastSeen, lastError
	}

	if strings.EqualFold(h.Breaker.State, "open") {
		reason := "circuit breaker open"
		if lastError != nil {
			reason = lastError.Message
		}
		return ConnectionStateUnreachable, reason, lastSeen, lastError
	}

	if lastSeen != nil && now.Sub(*lastSeen) > connectionStaleThreshold {
		return ConnectionStateStale, fmt.Sprintf("no successful poll in %s", now.Sub(*lastSeen).Round(time.Second)), lastSeen, lastError
	}

	return ConnectionStateActive, "", lastSeen, lastError
}

func sourceFromString(s string) ConnectionSource {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "agent":
		return ConnectionSourceAgent
	case "script":
		return ConnectionSourceScript
	default:
		return ConnectionSourceManual
	}
}

// instanceHealthByKey flattens SchedulerHealthResponse.Instances into a
// lookup map keyed by schedulerKey ("pve::instance-name"). The aggregator
// consults this to pick up LastSuccess and LastError without re-polling.
func instanceHealthByKey(resp monitoring.SchedulerHealthResponse) map[string]monitoring.InstanceHealth {
	out := make(map[string]monitoring.InstanceHealth, len(resp.Instances))
	for _, inst := range resp.Instances {
		if inst.Key == "" {
			continue
		}
		out[inst.Key] = inst
	}
	return out
}
