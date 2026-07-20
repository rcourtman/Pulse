package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/fleethealth"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/platformsupport"
	"github.com/rcourtman/pulse-go-rewrite/internal/remoteconfig"
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

const (
	fleetStateActive          = "active"
	fleetStateBehind          = "behind"
	fleetStateBlocked         = "blocked"
	fleetStateConfigured      = "configured"
	fleetStateCurrent         = "current"
	fleetStateDegraded        = "degraded"
	fleetStateDisabled        = "disabled"
	fleetStateEnabled         = "enabled"
	fleetStateEnrolled        = "enrolled"
	fleetStateHealthy         = "healthy"
	fleetStateChecking        = "checking"
	fleetStateFailed          = "failed"
	fleetStateInvalid         = "invalid"
	fleetStateNotApplicable   = "not-applicable"
	fleetStatePaused          = "paused"
	fleetStatePending         = "pending"
	fleetStateReported        = "reported"
	fleetStateUnknown         = "unknown"
	fleetStateUpdateAvailable = "update-available"
	fleetStateUpdating        = "updating"
	fleetStateVerified        = "verified"

	fleetConfigDriftCurrent       = "current"
	fleetConfigDriftDrifted       = "drifted"
	fleetConfigDriftNotApplicable = "not-applicable"

	fleetRolloutBlocked       = "blocked"
	fleetRolloutNotApplicable = "not-applicable"
	fleetRolloutStageApplied  = "applied"
	fleetRolloutStageBlocked  = "blocked"
	fleetRolloutStageLocal    = "local"
	fleetRolloutStagePaused   = "paused"
	fleetRolloutStagePending  = "pending"

	fleetCredentialKindAPIKey       = "api-key"
	fleetCredentialKindAgentToken   = "agent-token"
	fleetCredentialKindNone         = "none"
	fleetCredentialKindPassword     = "password"
	fleetCredentialKindToken        = "token"
	fleetCredentialRotationExpired  = "expired"
	fleetCredentialRotationExpiring = "expiring"
	fleetCredentialRotationHealthy  = "healthy"
	fleetCredentialRotationNone     = "not-applicable"
	fleetCredentialStatusExpired    = "expired"
	fleetCredentialStatusExpiring   = "expiring"

	fleetCommandPolicyBlocked       = "blocked"
	fleetCommandPolicyDrifted       = "drifted"
	fleetCommandPolicyInSync        = "in-sync"
	fleetCommandPolicyNotApplicable = "not-applicable"

	connectionConfigFingerprintVersion      = "connection-config/v1"
	connectionAgentConfigFingerprintVersion = "host-agent-config/v1"
)

// aggregatorInputs bundles everything the aggregator reads. Separating inputs
// from the handler makes the aggregator unit-testable without spinning up a
// monitor or persistence layer.
type aggregatorInputs struct {
	pveInstances         []config.PVEInstance
	pbsInstances         []config.PBSInstance
	pmgInstances         []config.PMGInstance
	vmwareInstances      []config.VMwareVCenterInstance
	vmwareSummaries      map[string]monitoring.VMwareConnectionSummary
	truenasInstances     []config.TrueNASInstance
	truenasSummaries     map[string]monitoring.TrueNASConnectionSummary
	availabilityTargets  []config.AvailabilityTarget
	availabilityStatuses map[string]monitoring.AvailabilityProbeStatus
	hosts                []models.Host
	apiTokens            []config.APITokenRecord
	agentDesiredConfigs  map[string]connectionAgentDesiredConfig
	instanceHealth       map[string]monitoring.InstanceHealth
	expectedAgentVersion string
	now                  time.Time
}

type connectionAgentDesiredConfig struct {
	Fingerprint     *ConnectionFleetConfigFingerprint
	CommandsEnabled *bool
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
			len(in.vmwareInstances)+len(in.truenasInstances)+len(in.availabilityTargets)+len(in.hosts))

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
		out = append(out, buildVMwareConnection(vmw, in.instanceHealth, in.vmwareSummaries, now))
	}
	for _, tn := range in.truenasInstances {
		out = append(out, buildTrueNASConnection(tn, in.instanceHealth, in.truenasSummaries, now))
	}
	for _, target := range in.availabilityTargets {
		out = append(out, buildAvailabilityConnection(target, in.availabilityStatuses[target.ID], now))
	}
	for _, host := range in.hosts {
		desiredConfig := connectionAgentDesiredConfigForHost(in.agentDesiredConfigs, host.ID)
		out = append(out, buildAgentConnection(host, in.expectedAgentVersion, now, desiredConfig))
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
	conn := withFleetGovernance(Connection{
		ID:           "pve:" + inst.Name,
		Type:         ConnectionTypePVE,
		Name:         inst.Name,
		Address:      inst.Host,
		HostAliases:  appendNormalizedHosts(nil, inst.Name, inst.Host),
		State:        state,
		StateReason:  reason,
		Enabled:      enabled,
		Surfaces:     surfaces,
		Scope:        scope,
		LastSeen:     lastSeen,
		LastError:    lastError,
		Source:       sourceFromString(inst.Source),
		Capabilities: ConnectionCapabilities{SupportsPause: true, SupportsScope: true, SupportsTest: true},
	}, now)
	conn.Fleet.CredentialHealth = connectionFleetCredentialHealth(conn, connectionProxmoxCredentialKind(inst.User, inst.Password, inst.TokenName, inst.TokenValue), nil, nil, now)
	return conn
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
	conn := withFleetGovernance(Connection{
		ID:           "pbs:" + inst.Name,
		Type:         ConnectionTypePBS,
		Name:         inst.Name,
		Address:      inst.Host,
		HostAliases:  appendNormalizedHosts(nil, inst.Name, inst.Host),
		State:        state,
		StateReason:  reason,
		Enabled:      enabled,
		Surfaces:     surfaces,
		Scope:        scope,
		LastSeen:     lastSeen,
		LastError:    lastError,
		Source:       sourceFromString(inst.Source),
		Capabilities: ConnectionCapabilities{SupportsPause: true, SupportsScope: true, SupportsTest: true},
	}, now)
	conn.Fleet.CredentialHealth = connectionFleetCredentialHealth(conn, connectionProxmoxCredentialKind(inst.User, inst.Password, inst.TokenName, inst.TokenValue), nil, nil, now)
	return conn
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
	conn := withFleetGovernance(Connection{
		ID:           "pmg:" + inst.Name,
		Type:         ConnectionTypePMG,
		Name:         inst.Name,
		Address:      inst.Host,
		HostAliases:  appendNormalizedHosts(nil, inst.Name, inst.Host),
		State:        state,
		StateReason:  reason,
		Enabled:      enabled,
		Surfaces:     surfaces,
		Scope:        scope,
		LastSeen:     lastSeen,
		LastError:    lastError,
		Source:       ConnectionSourceManual,
		Capabilities: ConnectionCapabilities{SupportsPause: true, SupportsScope: true, SupportsTest: true},
	}, now)
	conn.Fleet.CredentialHealth = connectionFleetCredentialHealth(conn, connectionProxmoxCredentialKind(inst.User, inst.Password, inst.TokenName, inst.TokenValue), nil, nil, now)
	return conn
}

func buildVMwareConnection(
	inst config.VMwareVCenterInstance,
	health map[string]monitoring.InstanceHealth,
	summaries map[string]monitoring.VMwareConnectionSummary,
	now time.Time,
) Connection {
	enabled := inst.Enabled
	surfaces := []string{"vms", "hosts", "datastores"}
	scope := map[string]bool{
		"vms":        inst.MonitorVMs,
		"hosts":      inst.MonitorHosts,
		"datastores": inst.MonitorDatastores,
	}
	h := health["vmware::"+inst.ID]
	if summary, ok := summaries[strings.TrimSpace(inst.ID)]; ok {
		h = mergeVMwareSummaryHealth(h, summary)
	}
	state, reason, lastSeen, lastError := deriveConnectionState(enabled, h, now)
	port := inst.Port
	if port == 0 {
		port = 443
	}
	conn := withFleetGovernance(Connection{
		ID:           "vmware:" + inst.ID,
		Type:         ConnectionTypeVMware,
		Name:         inst.Name,
		Address:      fmt.Sprintf("https://%s:%d", inst.Host, port),
		HostAliases:  appendNormalizedHosts(nil, inst.Name, inst.Host),
		State:        state,
		StateReason:  reason,
		Enabled:      enabled,
		Surfaces:     surfaces,
		Scope:        scope,
		LastSeen:     lastSeen,
		LastError:    lastError,
		Source:       ConnectionSourceManual,
		Capabilities: ConnectionCapabilities{SupportsPause: true, SupportsScope: true, SupportsTest: true},
	}, now)
	conn.Fleet.CredentialHealth = connectionFleetCredentialHealth(conn, connectionPasswordCredentialKind(inst.Username, inst.Password), nil, nil, now)
	return conn
}

func buildTrueNASConnection(
	inst config.TrueNASInstance,
	health map[string]monitoring.InstanceHealth,
	summaries map[string]monitoring.TrueNASConnectionSummary,
	now time.Time,
) Connection {
	enabled := inst.Enabled
	surfaces := []string{"datasets", "pools", "replication"}
	scope := map[string]bool{
		"datasets":    inst.MonitorDatasets,
		"pools":       inst.MonitorPools,
		"replication": inst.MonitorReplication,
	}
	h := health["truenas::"+inst.ID]
	if summary, ok := summaries[strings.TrimSpace(inst.ID)]; ok {
		h = mergeTrueNASSummaryHealth(h, summary)
	}
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
	conn := withFleetGovernance(Connection{
		ID:           "truenas:" + inst.ID,
		Type:         ConnectionTypeTrueNAS,
		Name:         inst.Name,
		Address:      fmt.Sprintf("%s://%s:%d", scheme, inst.Host, port),
		HostAliases:  appendNormalizedHosts(nil, inst.Name, inst.Host),
		State:        state,
		StateReason:  reason,
		Enabled:      enabled,
		Surfaces:     surfaces,
		Scope:        scope,
		LastSeen:     lastSeen,
		LastError:    lastError,
		Source:       ConnectionSourceManual,
		Capabilities: ConnectionCapabilities{SupportsPause: true, SupportsScope: true, SupportsTest: true},
	}, now)
	conn.Fleet.CredentialHealth = connectionFleetCredentialHealth(conn, connectionTrueNASCredentialKind(inst), nil, nil, now)
	return conn
}

func mergeVMwareSummaryHealth(
	h monitoring.InstanceHealth,
	summary monitoring.VMwareConnectionSummary,
) monitoring.InstanceHealth {
	if summary.Poll == nil {
		return h
	}
	return mergePlatformPollHealth(
		h,
		summary.Poll.LastSuccessAt,
		vmwarePollErrorAt(summary.Poll.LastError),
		vmwarePollErrorMessage(summary.Poll.LastError),
		vmwarePollErrorCategory(summary.Poll.LastError),
		summary.Poll.ConsecutiveFailures,
	)
}

func mergeTrueNASSummaryHealth(
	h monitoring.InstanceHealth,
	summary monitoring.TrueNASConnectionSummary,
) monitoring.InstanceHealth {
	if summary.Poll == nil {
		return h
	}
	return mergePlatformPollHealth(
		h,
		summary.Poll.LastSuccessAt,
		trueNASPollErrorAt(summary.Poll.LastError),
		trueNASPollErrorMessage(summary.Poll.LastError),
		trueNASPollErrorCategory(summary.Poll.LastError),
		summary.Poll.ConsecutiveFailures,
	)
}

func mergePlatformPollHealth(
	h monitoring.InstanceHealth,
	lastSuccess *time.Time,
	lastErrorAt *time.Time,
	lastErrorMessage string,
	lastErrorCategory string,
	consecutiveFailures int,
) monitoring.InstanceHealth {
	if lastSuccess != nil && !lastSuccess.IsZero() {
		t := *lastSuccess
		h.PollStatus.LastSuccess = &t
	}
	if consecutiveFailures > h.PollStatus.ConsecutiveFailures {
		h.PollStatus.ConsecutiveFailures = consecutiveFailures
	}

	lastErrorMessage = strings.TrimSpace(lastErrorMessage)
	if lastErrorMessage != "" {
		at := time.Time{}
		if lastErrorAt != nil {
			at = *lastErrorAt
		}
		h.PollStatus.LastError = &monitoring.ErrorDetail{
			At:       at,
			Message:  lastErrorMessage,
			Category: strings.TrimSpace(lastErrorCategory),
		}
	}
	return h
}

func vmwarePollErrorAt(err *monitoring.VMwareConnectionPollError) *time.Time {
	if err == nil {
		return nil
	}
	return err.At
}

func vmwarePollErrorMessage(err *monitoring.VMwareConnectionPollError) string {
	if err == nil {
		return ""
	}
	return err.Message
}

func vmwarePollErrorCategory(err *monitoring.VMwareConnectionPollError) string {
	if err == nil {
		return ""
	}
	return err.Category
}

func trueNASPollErrorAt(err *monitoring.TrueNASConnectionPollError) *time.Time {
	if err == nil {
		return nil
	}
	return err.At
}

func trueNASPollErrorMessage(err *monitoring.TrueNASConnectionPollError) string {
	if err == nil {
		return ""
	}
	return err.Message
}

func trueNASPollErrorCategory(err *monitoring.TrueNASConnectionPollError) string {
	if err == nil {
		return ""
	}
	return err.Category
}

func buildAvailabilityConnection(target config.AvailabilityTarget, status monitoring.AvailabilityProbeStatus, now time.Time) Connection {
	target = config.NormalizeAvailabilityTarget(target)
	state, reason, lastSeen, lastError := deriveAvailabilityConnectionState(target, status, now)
	conn := withFleetGovernance(Connection{
		ID:           "availability:" + target.ID,
		Type:         ConnectionTypeAvailability,
		Name:         target.DisplayName(),
		Address:      target.Address,
		HostAliases:  appendNormalizedHosts(nil, target.Address, target.ProbeAddress()),
		State:        state,
		StateReason:  reason,
		Enabled:      target.Enabled,
		Surfaces:     []string{"availability"},
		Scope:        map[string]bool{"availability": true},
		LastSeen:     lastSeen,
		LastError:    lastError,
		Source:       ConnectionSourceManual,
		Capabilities: ConnectionCapabilities{SupportsPause: true, SupportsScope: false, SupportsTest: true},
	}, now)
	conn.Fleet.CredentialHealth = connectionFleetCredentialHealthNotApplicable()
	return conn
}

// buildAgentConnection derives a connection row from an agent Host record.
// Agents have no pause toggle and no scope — reports are all-or-nothing —
// so capability flags are off.
func buildAgentConnection(host models.Host, expectedAgentVersion string, now time.Time, desiredConfig *connectionAgentDesiredConfig) Connection {
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
	versionDrift := fleethealth.DeriveAgentVersionDrift(currentAgentVersion, expectedAgentVersion)
	updateAvailable := versionDrift == fleethealth.AgentVersionBehind
	agentIdentity := connectionAgentIdentityForHost(host)
	switch fleethealth.DeriveAgentLiveness(host.LastSeen, now, host.IntervalSeconds) {
	case fleethealth.AgentLivenessPending:
		state = ConnectionStatePending
	case fleethealth.AgentLivenessStale:
		state = ConnectionStateStale
		reason = fmt.Sprintf("no heartbeat in %s", now.Sub(*lastSeen).Round(time.Second))
	default:
		state = ConnectionStateActive
	}

	conn := withFleetGovernance(Connection{
		ID:                   fleethealth.AgentConnectionID(host.ID),
		Type:                 ConnectionTypeAgent,
		Name:                 name,
		Address:              address,
		HostAliases:          connectionHostAliasesForAgent(host, name, address),
		State:                state,
		StateReason:          reason,
		Enabled:              true,
		Surfaces:             []string{"host"},
		Scope:                map[string]bool{"host": true},
		LastSeen:             lastSeen,
		LastError:            nil,
		Source:               ConnectionSourceAgent,
		AgentIdentity:        agentIdentity,
		AgentVersion:         currentAgentVersion,
		ExpectedAgentVersion: expectedAgentVersion,
		AgentUpdateAvailable: updateAvailable,
		AgentUpdate:          connectionAgentUpdateStatus(host.AgentUpdate),
		AgentModules:         connectionAgentModuleStatuses(host.AgentModules),
		Capabilities:         ConnectionCapabilities{SupportsPause: false, SupportsScope: false, SupportsTest: false},
	}, now)
	conn.Fleet.ConfigDrift = connectionFleetAgentConfigDrift(conn, desiredConfig, host.AppliedConfig)
	conn.Fleet.CredentialHealth = connectionFleetAgentCredentialHealth(conn, host, now)
	conn.Fleet.CommandPolicy = connectionFleetAgentCommandPolicy(conn, host, desiredConfig)
	conn.Fleet.Rollout = connectionFleetRollout(conn)
	return conn
}

func connectionAgentModuleStatuses(values []models.AgentModuleStatus) []ConnectionAgentModuleStatus {
	if len(values) == 0 {
		return nil
	}
	result := make([]ConnectionAgentModuleStatus, 0, len(values))
	for _, value := range values {
		result = append(result, ConnectionAgentModuleStatus{
			Name:      strings.TrimSpace(value.Name),
			Enabled:   value.Enabled,
			State:     strings.TrimSpace(value.State),
			LastError: strings.TrimSpace(value.LastError),
			UpdatedAt: value.UpdatedAt.UTC(),
		})
	}
	return result
}

func connectionAgentUpdateStatus(value *models.AgentUpdateStatus) *ConnectionAgentUpdateStatus {
	if value == nil {
		return nil
	}
	return &ConnectionAgentUpdateStatus{
		State:            strings.TrimSpace(value.State),
		AutoUpdate:       value.AutoUpdate,
		UpdatedFrom:      strings.TrimSpace(value.UpdatedFrom),
		AvailableVersion: strings.TrimSpace(value.AvailableVersion),
		LastCheckedAt:    cloneTime(value.LastCheckedAt),
		LastAttemptAt:    cloneTime(value.LastAttemptAt),
		LastSuccessAt:    cloneTime(value.LastSuccessAt),
		LastError:        strings.TrimSpace(value.LastError),
	}
}

func cloneTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	copy := value.UTC()
	return &copy
}

func withFleetGovernance(conn Connection, now time.Time) Connection {
	conn.Fleet = deriveConnectionFleetGovernance(conn, now)
	return conn
}

func deriveConnectionFleetGovernance(conn Connection, now time.Time) ConnectionFleetGovernance {
	fleet := ConnectionFleetGovernance{
		EnrollmentState:  connectionFleetEnrollmentState(conn),
		LivenessState:    string(conn.State),
		VersionDrift:     connectionFleetVersionDrift(conn),
		AdapterHealth:    connectionFleetAdapterHealth(conn),
		ConfigRollout:    connectionFleetConfigRollout(conn),
		CredentialStatus: connectionFleetCredentialStatus(conn),
		UpdateStatus:     connectionFleetUpdateStatus(conn),
		RemoteControl:    connectionFleetRemoteControl(conn),
	}
	fleet.ConfigDrift = connectionFleetConfigDrift(conn)
	fleet.Rollout = connectionFleetRollout(conn)
	fleet.CredentialHealth = connectionFleetCredentialHealth(conn, "", nil, nil, now)
	fleet.CommandPolicy = connectionFleetCommandPolicy(conn)
	return fleet
}

func connectionFleetEnrollmentState(conn Connection) string {
	if conn.Type == ConnectionTypeAgent {
		if conn.LastSeen == nil {
			return fleetStatePending
		}
		return fleetStateEnrolled
	}
	if !conn.Enabled {
		return fleetStatePaused
	}
	return fleetStateConfigured
}

func connectionFleetVersionDrift(conn Connection) string {
	if conn.Type != ConnectionTypeAgent {
		return fleetStateNotApplicable
	}
	switch fleethealth.DeriveAgentVersionDrift(conn.AgentVersion, conn.ExpectedAgentVersion) {
	case fleethealth.AgentVersionBehind:
		return fleetStateBehind
	case fleethealth.AgentVersionCurrent:
		return fleetStateCurrent
	default:
		return fleetStateUnknown
	}
}

func connectionFleetAdapterHealth(conn Connection) string {
	if conn.Type == ConnectionTypeAgent {
		for _, module := range conn.AgentModules {
			if module.Enabled && strings.TrimSpace(module.State) != "running" {
				return fleetStateDegraded
			}
		}
	}
	switch conn.State {
	case ConnectionStateActive:
		return fleetStateHealthy
	case ConnectionStateStale, ConnectionStatePending:
		return fleetStateDegraded
	case ConnectionStateUnauthorized, ConnectionStateUnreachable:
		return fleetStateBlocked
	case ConnectionStatePaused:
		return fleetStatePaused
	default:
		return fleetStateUnknown
	}
}

func connectionFleetConfigRollout(conn Connection) string {
	if !conn.Enabled || conn.State == ConnectionStatePaused {
		return fleetStatePaused
	}
	if conn.Type != ConnectionTypeAgent {
		return fleetStateConfigured
	}
	if conn.LastSeen == nil {
		return fleetStateUnknown
	}
	return fleetStateReported
}

func connectionFleetCredentialStatus(conn Connection) string {
	switch conn.State {
	case ConnectionStateUnauthorized:
		return fleetStateInvalid
	case ConnectionStatePending:
		return fleetStateUnknown
	case ConnectionStatePaused:
		return fleetStatePaused
	default:
		return fleetStateVerified
	}
}

func connectionFleetUpdateStatus(conn Connection) string {
	if conn.Type != ConnectionTypeAgent {
		return fleetStateNotApplicable
	}
	if conn.AgentUpdate != nil {
		switch strings.TrimSpace(conn.AgentUpdate.State) {
		case "error":
			return fleetStateFailed
		case "updating":
			return fleetStateUpdating
		case "checking":
			return fleetStateChecking
		case "disabled":
			if conn.AgentUpdateAvailable {
				return fleetStateDisabled
			}
		}
	}
	if conn.AgentUpdateAvailable {
		return fleetStateUpdateAvailable
	}
	if strings.TrimSpace(conn.AgentVersion) == "" || strings.TrimSpace(conn.ExpectedAgentVersion) == "" {
		return fleetStateUnknown
	}
	return fleetStateCurrent
}

func connectionFleetRemoteControl(conn Connection) string {
	if conn.Type != ConnectionTypeAgent {
		return fleetStateNotApplicable
	}
	if conn.LastSeen == nil {
		return fleetStateUnknown
	}
	if conn.AgentIdentity != nil && conn.AgentIdentity.CommandsEnabled {
		return fleetStateEnabled
	}
	return fleetStateDisabled
}

func connectionFleetConfigDrift(conn Connection) *ConnectionFleetConfigDrift {
	if !conn.Enabled || conn.State == ConnectionStatePaused {
		return &ConnectionFleetConfigDrift{
			Status: fleetStatePaused,
			Reason: "configuration rollout is paused with this connection",
		}
	}

	fingerprint := connectionConfigFingerprint(connectionConfigFingerprintVersion, map[string]any{
		"enabled":  conn.Enabled,
		"scope":    conn.Scope,
		"surfaces": conn.Surfaces,
		"type":     conn.Type,
	})
	if fingerprint == nil {
		return &ConnectionFleetConfigDrift{
			Status: fleetStateUnknown,
			Reason: "configuration fingerprint could not be derived",
		}
	}
	return &ConnectionFleetConfigDrift{
		Status:         fleetConfigDriftCurrent,
		Desired:        fingerprint,
		Applied:        fingerprint,
		LastObservedAt: conn.LastSeen,
		Reason:         "configured collection scope matches the applied local ledger state",
	}
}

func connectionFleetAgentConfigDrift(conn Connection, desiredConfig *connectionAgentDesiredConfig, appliedConfig *models.AgentConfigFingerprint) *ConnectionFleetConfigDrift {
	if desiredConfig == nil {
		return &ConnectionFleetConfigDrift{
			Status: fleetStateUnknown,
			Reason: "Pulse has not resolved canonical desired agent configuration metadata",
		}
	}
	if desiredConfig.Fingerprint == nil {
		return &ConnectionFleetConfigDrift{
			Status:         fleetConfigDriftNotApplicable,
			LastObservedAt: conn.LastSeen,
			Reason:         "no managed agent configuration override is assigned",
		}
	}
	return connectionFleetAgentConfigDriftForFingerprints(conn, desiredConfig.Fingerprint, connectionConfigFingerprintFromAgent(appliedConfig))
}

func connectionConfigFingerprintFromAgent(value *models.AgentConfigFingerprint) *ConnectionFleetConfigFingerprint {
	if value == nil {
		return nil
	}
	return connectionConfigFingerprintFromMetadata(value.Version, value.Hash)
}

func connectionFleetAgentConfigDriftForFingerprints(conn Connection, desired, applied *ConnectionFleetConfigFingerprint) *ConnectionFleetConfigDrift {
	if !conn.Enabled || conn.State == ConnectionStatePaused {
		return &ConnectionFleetConfigDrift{
			Status: fleetStatePaused,
			Reason: "agent configuration rollout is paused",
		}
	}

	if desired == nil {
		return &ConnectionFleetConfigDrift{
			Status: fleetStateUnknown,
			Reason: "Pulse has not resolved canonical desired agent configuration metadata",
		}
	}

	if conn.LastSeen == nil {
		return &ConnectionFleetConfigDrift{
			Status:  fleetStateUnknown,
			Desired: desired,
			Reason:  "Pulse has not received an agent report to compare against desired configuration",
		}
	}

	if applied == nil {
		return &ConnectionFleetConfigDrift{
			Status:  fleetStatePending,
			Desired: desired,
			Reason:  "Pulse has not received a comparable applied agent configuration fingerprint yet",
		}
	}

	status := fleetConfigDriftCurrent
	reason := "reported applied agent configuration matches the desired fleet policy"
	if desired.Version != applied.Version || desired.Hash != applied.Hash {
		status = fleetConfigDriftDrifted
		reason = "desired agent configuration fingerprint differs from the reported applied fingerprint"
	}
	return &ConnectionFleetConfigDrift{
		Status:         status,
		Desired:        desired,
		Applied:        applied,
		LastObservedAt: conn.LastSeen,
		Reason:         reason,
	}
}

func connectionFleetRollout(conn Connection) *ConnectionFleetRolloutState {
	if conn.Type != ConnectionTypeAgent && conn.Type != ConnectionTypePVE && conn.Type != ConnectionTypePBS &&
		conn.Type != ConnectionTypePMG && conn.Type != ConnectionTypeVMware && conn.Type != ConnectionTypeTrueNAS &&
		conn.Type != ConnectionTypeAvailability {
		return &ConnectionFleetRolloutState{Status: fleetRolloutNotApplicable}
	}
	if !conn.Enabled || conn.State == ConnectionStatePaused {
		return &ConnectionFleetRolloutState{
			Status: fleetStatePaused,
			Stage:  fleetRolloutStagePaused,
			Reason: "rollout is paused while the connection is disabled",
		}
	}
	if conn.State == ConnectionStatePending {
		return &ConnectionFleetRolloutState{
			Status: fleetStatePending,
			Stage:  fleetRolloutStagePending,
			Reason: "waiting for first connection confirmation",
		}
	}
	if conn.State == ConnectionStateUnauthorized || conn.State == ConnectionStateUnreachable {
		return &ConnectionFleetRolloutState{
			Status: fleetRolloutBlocked,
			Stage:  fleetRolloutStageBlocked,
			Reason: "rollout is blocked until the connection recovers",
		}
	}
	if conn.Type == ConnectionTypeAgent && conn.LastSeen == nil {
		return &ConnectionFleetRolloutState{
			Status: fleetStatePending,
			Stage:  fleetRolloutStagePending,
			Reason: "waiting for the agent to report applied configuration",
		}
	}
	if conn.Type == ConnectionTypeAgent && conn.Fleet.ConfigDrift != nil {
		switch conn.Fleet.ConfigDrift.Status {
		case fleetConfigDriftDrifted:
			return &ConnectionFleetRolloutState{
				Status: fleetStatePending,
				Stage:  fleetRolloutStagePending,
				Reason: "desired configuration has not converged on the reported runtime",
			}
		case fleetStatePending:
			return &ConnectionFleetRolloutState{
				Status: fleetStatePending,
				Stage:  fleetRolloutStagePending,
				Reason: "waiting for the agent to report an applied configuration fingerprint",
			}
		case fleetStateUnknown:
			return &ConnectionFleetRolloutState{
				Status: fleetStateUnknown,
				Stage:  fleetRolloutStagePending,
				Reason: "rollout state cannot be confirmed without comparable desired and applied agent config fingerprints",
			}
		case fleetConfigDriftNotApplicable:
			return &ConnectionFleetRolloutState{
				Status: fleetStateCurrent,
				Stage:  fleetRolloutStageApplied,
				Reason: "no managed agent configuration rollout is assigned",
			}
		}
	}
	stage := fleetRolloutStageLocal
	if conn.Type == ConnectionTypeAgent {
		stage = fleetRolloutStageApplied
	}
	return &ConnectionFleetRolloutState{
		Status: fleetStateCurrent,
		Stage:  stage,
		Reason: "rollout state is current for this connection",
	}
}

func connectionFleetCredentialHealth(conn Connection, kind string, lastUsedAt, expiresAt *time.Time, now time.Time) *ConnectionFleetCredentialHealth {
	status := connectionFleetCredentialStatus(conn)
	if strings.TrimSpace(kind) == "" {
		kind = fleetStateUnknown
	}
	if kind == fleetCredentialKindNone {
		return connectionFleetCredentialHealthNotApplicable()
	}

	health := &ConnectionFleetCredentialHealth{
		Status:     status,
		Kind:       kind,
		Rotation:   fleetCredentialRotationHealthy,
		LastUsedAt: cloneTimePtr(lastUsedAt),
		ExpiresAt:  cloneTimePtr(expiresAt),
	}
	if conn.LastSeen != nil && (status == fleetStateVerified || status == fleetCredentialStatusExpiring) {
		health.LastVerifiedAt = cloneTimePtr(conn.LastSeen)
	}
	if conn.LastError != nil && status == fleetStateInvalid {
		health.LastFailedAt = cloneTimePtr(&conn.LastError.At)
	}
	if expiresAt == nil {
		return health
	}

	if now.IsZero() {
		now = time.Now()
	}
	switch {
	case now.After(*expiresAt):
		health.Status = fleetCredentialStatusExpired
		health.Rotation = fleetCredentialRotationExpired
	case expiresAt.Sub(now) <= 14*24*time.Hour:
		if health.Status == fleetStateVerified {
			health.Status = fleetCredentialStatusExpiring
		}
		health.Rotation = fleetCredentialRotationExpiring
	}
	return health
}

func connectionFleetCredentialHealthNotApplicable() *ConnectionFleetCredentialHealth {
	return &ConnectionFleetCredentialHealth{
		Status:   fleetStateNotApplicable,
		Kind:     fleetCredentialKindNone,
		Rotation: fleetCredentialRotationNone,
	}
}

func connectionFleetAgentCredentialHealth(conn Connection, host models.Host, now time.Time) *ConnectionFleetCredentialHealth {
	kind := fleetStateUnknown
	if strings.TrimSpace(host.TokenID) != "" || strings.TrimSpace(host.TokenName) != "" || strings.TrimSpace(host.TokenHint) != "" {
		kind = fleetCredentialKindAgentToken
	}
	return connectionFleetCredentialHealth(conn, kind, host.TokenLastUsedAt, nil, now)
}

func connectionFleetCommandPolicy(conn Connection) *ConnectionFleetCommandPolicy {
	if conn.Type != ConnectionTypeAgent {
		return &ConnectionFleetCommandPolicy{
			Status:      fleetStateNotApplicable,
			Desired:     fleetCommandPolicyNotApplicable,
			Applied:     fleetCommandPolicyNotApplicable,
			Enforcement: fleetCommandPolicyNotApplicable,
		}
	}
	if conn.LastSeen == nil {
		return &ConnectionFleetCommandPolicy{
			Status:      fleetStateUnknown,
			Desired:     fleetStateUnknown,
			Applied:     fleetStateUnknown,
			Enforcement: fleetStatePending,
			Reason:      "waiting for the agent to report command-policy state",
		}
	}
	applied := fleetStateDisabled
	if conn.AgentIdentity != nil && conn.AgentIdentity.CommandsEnabled {
		applied = fleetStateEnabled
	}
	return &ConnectionFleetCommandPolicy{
		Status:      applied,
		Desired:     fleetStateUnknown,
		Applied:     applied,
		Enforcement: fleetCommandPolicyNotApplicable,
		Reason:      "no desired command-policy override is configured; reporting the agent-applied state",
	}
}

func connectionFleetAgentCommandPolicy(conn Connection, host models.Host, desiredConfig *connectionAgentDesiredConfig) *ConnectionFleetCommandPolicy {
	desired := fleetStateUnknown
	if desiredConfig != nil && desiredConfig.CommandsEnabled != nil {
		desired = connectionFleetCommandPolicyState(*desiredConfig.CommandsEnabled)
	}

	if conn.LastSeen == nil {
		return &ConnectionFleetCommandPolicy{
			Status:      fleetStateUnknown,
			Desired:     desired,
			Applied:     fleetStateUnknown,
			Enforcement: fleetStatePending,
			Reason:      "waiting for the agent to report command-policy state",
		}
	}

	applied := connectionFleetCommandPolicyState(host.CommandsEnabled)
	policy := &ConnectionFleetCommandPolicy{
		Status:  applied,
		Desired: desired,
		Applied: applied,
	}

	if desired == fleetStateUnknown {
		policy.Enforcement = fleetCommandPolicyNotApplicable
		policy.Reason = "no desired command-policy override is configured; reporting the agent-applied state"
		return policy
	}
	if desired == applied {
		policy.Enforcement = fleetCommandPolicyInSync
		if applied == fleetStateEnabled {
			policy.Reason = "agent command execution matches the desired enabled policy"
		} else {
			policy.Reason = "agent command execution matches the desired disabled policy"
		}
		return policy
	}

	policy.Enforcement = fleetCommandPolicyDrifted
	if desired == fleetStateDisabled && applied == fleetStateEnabled {
		policy.Reason = "agent still reports command execution enabled while desired policy disables it"
	} else {
		policy.Reason = "agent reports command execution disabled while desired policy enables it"
	}
	return policy
}

func connectionFleetCommandPolicyState(enabled bool) string {
	if enabled {
		return fleetStateEnabled
	}
	return fleetStateDisabled
}

func connectionConfigFingerprint(version string, payload any) *ConnectionFleetConfigFingerprint {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil
	}
	sum := sha256.Sum256(data)
	return &ConnectionFleetConfigFingerprint{
		Version: version,
		Hash:    "sha256:" + hex.EncodeToString(sum[:]),
	}
}

func connectionAgentDesiredConfigFingerprints(monitor *monitoring.Monitor, hosts []models.Host, tokens []config.APITokenRecord) map[string]connectionAgentDesiredConfig {
	if monitor == nil || len(hosts) == 0 {
		return nil
	}

	configs := make(map[string]connectionAgentDesiredConfig, len(hosts))
	tokenByID := connectionAgentTokenRecordsByID(tokens)
	for _, host := range hosts {
		hostID := strings.TrimSpace(host.ID)
		if hostID == "" {
			continue
		}
		cfg := monitor.GetHostAgentConfig(hostID)
		cfg = effectiveConnectionAgentConfig(cfg, host, tokenByID)
		desired := connectionAgentDesiredConfig{
			CommandsEnabled: cloneBoolPtr(cfg.CommandsEnabled),
		}
		if cfg.DesiredConfig != nil && remoteconfig.HasAppliedDesiredConfig(cfg.CommandsEnabled, cfg.Settings) {
			if fp := connectionConfigFingerprintFromMetadata(cfg.DesiredConfig.Version, cfg.DesiredConfig.Hash); fp != nil {
				desired.Fingerprint = fp
			}
		}
		configs[hostID] = desired
	}
	if len(configs) == 0 {
		return nil
	}
	return configs
}

func connectionAgentTokenRecordsByID(tokens []config.APITokenRecord) map[string]*config.APITokenRecord {
	if len(tokens) == 0 {
		return nil
	}
	records := make(map[string]*config.APITokenRecord, len(tokens))
	for i := range tokens {
		id := strings.TrimSpace(tokens[i].ID)
		if id == "" {
			continue
		}
		records[id] = &tokens[i]
	}
	return records
}

func effectiveConnectionAgentConfig(cfg monitoring.HostAgentConfig, host models.Host, tokenByID map[string]*config.APITokenRecord) monitoring.HostAgentConfig {
	var record *config.APITokenRecord
	if tokenByID != nil {
		record = tokenByID[strings.TrimSpace(host.TokenID)]
	}
	if record == nil && len(tokenByID) > 0 && cfg.CommandsEnabled != nil && *cfg.CommandsEnabled {
		// API tokens exist but this host's TokenID resolves to none of them
		// (stale after a token was revoked and the agent reinstalled, or an
		// untracked binding). The runtime config served to such an agent can
		// never enable commands, so the desired side must not either; leaving
		// it enabled fabricates a command-policy mismatch the operator cannot
		// fix from the profile. The contract requires desired to be the
		// sanitized effective config, not the raw profile desire.
		disabled := false
		cfg.CommandsEnabled = &disabled
	} else {
		cfg = sanitizeHostAgentConfigForToken(cfg, record, host)
	}

	metadata, err := remoteconfig.BuildDesiredConfigMetadata(cfg.CommandsEnabled, cfg.Settings)
	if err != nil {
		cfg.DesiredConfig = nil
		return cfg
	}
	cfg.DesiredConfig = &metadata
	return cfg
}

func connectionAgentDesiredConfigForHost(configs map[string]connectionAgentDesiredConfig, hostID string) *connectionAgentDesiredConfig {
	if len(configs) == 0 {
		return nil
	}
	desired, ok := configs[strings.TrimSpace(hostID)]
	if !ok {
		return nil
	}
	return &connectionAgentDesiredConfig{
		Fingerprint:     cloneConnectionFleetConfigFingerprint(desired.Fingerprint),
		CommandsEnabled: cloneBoolPtr(desired.CommandsEnabled),
	}
}

func connectionConfigFingerprintFromMetadata(version, hash string) *ConnectionFleetConfigFingerprint {
	version = strings.TrimSpace(version)
	hash = strings.TrimSpace(hash)
	if version == "" || hash == "" {
		return nil
	}
	return &ConnectionFleetConfigFingerprint{
		Version: version,
		Hash:    hash,
	}
}

func cloneConnectionFleetConfigFingerprint(fp *ConnectionFleetConfigFingerprint) *ConnectionFleetConfigFingerprint {
	if fp == nil {
		return nil
	}
	copied := *fp
	return &copied
}

func cloneBoolPtr(v *bool) *bool {
	if v == nil {
		return nil
	}
	copied := *v
	return &copied
}

func connectionProxmoxCredentialKind(user, password, tokenName, tokenValue string) string {
	if strings.TrimSpace(tokenName) != "" || strings.TrimSpace(tokenValue) != "" {
		return fleetCredentialKindToken
	}
	return connectionPasswordCredentialKind(user, password)
}

func connectionPasswordCredentialKind(user, password string) string {
	if strings.TrimSpace(user) != "" || strings.TrimSpace(password) != "" {
		return fleetCredentialKindPassword
	}
	return fleetStateUnknown
}

func connectionTrueNASCredentialKind(inst config.TrueNASInstance) string {
	if strings.TrimSpace(inst.APIKey) != "" && !config.IsTrueNASSensitiveMask(inst.APIKey) {
		return fleetCredentialKindAPIKey
	}
	if strings.TrimSpace(inst.Username) != "" || strings.TrimSpace(inst.Password) != "" {
		return fleetCredentialKindPassword
	}
	return fleetStateUnknown
}

func cloneTimePtr(t *time.Time) *time.Time {
	if t == nil || t.IsZero() {
		return nil
	}
	copied := *t
	return &copied
}

func connectionAgentIdentityForHost(host models.Host) *ConnectionAgentIdentity {
	hostProfile := connectionAgentHostProfileForHost(host)
	identity := &ConnectionAgentIdentity{
		Hostname:        strings.TrimSpace(host.Hostname),
		Platform:        connectionAgentPlatformForHost(host, hostProfile),
		HostProfile:     hostProfile,
		OSName:          strings.TrimSpace(host.OSName),
		OSVersion:       strings.TrimSpace(host.OSVersion),
		KernelVersion:   strings.TrimSpace(host.KernelVersion),
		Architecture:    strings.TrimSpace(host.Architecture),
		ReportIP:        strings.TrimSpace(host.ReportIP),
		CommandsEnabled: host.CommandsEnabled,
	}
	if identity.Hostname == "" &&
		identity.Platform == "" &&
		identity.HostProfile == "" &&
		identity.OSName == "" &&
		identity.OSVersion == "" &&
		identity.KernelVersion == "" &&
		identity.Architecture == "" &&
		identity.ReportIP == "" &&
		!identity.CommandsEnabled {
		return nil
	}
	return identity
}

func connectionAgentPlatformForHost(host models.Host, hostProfile string) string {
	return platformsupport.NormalizeRuntimePlatformForAgentHostProfile(hostProfile, host.Platform)
}

func connectionAgentHostProfileForHost(host models.Host) string {
	if host.Unraid != nil {
		return connectionAgentHostProfileIDFromIdentity("unraid")
	}
	return connectionAgentHostProfileIDFromIdentity(host.OSName, host.Platform)
}

func connectionAgentHostProfileIDFromIdentity(values ...string) string {
	profile, ok := platformsupport.AgentHostProfileForIdentity(values...)
	if !ok {
		return ""
	}
	return profile.ID
}

func connectionHostAliasesForAgent(host models.Host, name, address string) []string {
	values := []string{name, address, host.Hostname, host.ReportIP}
	for _, iface := range host.NetworkInterfaces {
		values = append(values, iface.Addresses...)
	}
	return appendNormalizedHosts(nil, values...)
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

	if lastSeen == nil && lastError != nil {
		return ConnectionStateUnreachable, lastError.Message, lastSeen, lastError
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

func deriveAvailabilityConnectionState(target config.AvailabilityTarget, status monitoring.AvailabilityProbeStatus, now time.Time) (ConnectionState, string, *time.Time, *ConnectionError) {
	var lastSeen *time.Time
	if !status.LastSuccess.IsZero() {
		t := status.LastSuccess
		lastSeen = &t
	}

	var lastError *ConnectionError
	if strings.TrimSpace(status.LastError) != "" && !status.LastChecked.IsZero() {
		lastError = &ConnectionError{
			At:       status.LastChecked,
			Message:  status.LastError,
			Category: "availability",
		}
	}

	if !target.Enabled {
		return ConnectionStatePaused, "paused by user", lastSeen, lastError
	}
	if status.LastChecked.IsZero() {
		return ConnectionStatePending, "awaiting first probe", nil, nil
	}
	if !status.Available {
		threshold := target.EffectiveFailureThreshold()
		reason := fmt.Sprintf("probe failed %d/%d times", status.ConsecutiveFailures, threshold)
		if status.LastError != "" {
			reason = status.LastError
		}
		return ConnectionStateUnreachable, reason, lastSeen, lastError
	}
	if lastSeen != nil {
		staleThreshold := time.Duration(target.EffectivePollIntervalSecs()*2) * time.Second
		if staleThreshold < connectionStaleThreshold {
			staleThreshold = connectionStaleThreshold
		}
		if now.Sub(*lastSeen) > staleThreshold {
			return ConnectionStateStale, fmt.Sprintf("no successful probe in %s", now.Sub(*lastSeen).Round(time.Second)), lastSeen, lastError
		}
	}
	return ConnectionStateActive, "", lastSeen, nil
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
