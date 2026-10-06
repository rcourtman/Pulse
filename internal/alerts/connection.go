package alerts

import (
	"fmt"
	"strings"
	"time"

	alertspecs "github.com/rcourtman/pulse-go-rewrite/internal/alerts/specs"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rs/zerolog/log"
)

// ConnectionState mirrors api.ConnectionState as a free-standing string so the
// alerts package can react to derived connection state without importing the
// api package (which would create a cycle through api → monitoring → alerts).
// Values stay in lockstep with api.ConnectionState by convention; the
// connections_aggregator translator is the single producer.
type ConnectionState string

const (
	ConnectionStateActive       ConnectionState = "active"
	ConnectionStateStale        ConnectionState = "stale"
	ConnectionStateUnreachable  ConnectionState = "unreachable"
	ConnectionStateUnauthorized ConnectionState = "unauthorized"
	ConnectionStatePaused       ConnectionState = "paused"
	ConnectionStatePending      ConnectionState = "pending"
)

// ConnectionType narrows the set of connections that participate in the
// connection-degraded alert. Agent, availability, docker, and kubernetes
// connections have their own dedicated alert lifecycles, so the aggregator
// must omit them when handing snapshots to CheckConnection.
type ConnectionType string

const (
	ConnectionTypePVE     ConnectionType = "pve"
	ConnectionTypePBS     ConnectionType = "pbs"
	ConnectionTypePMG     ConnectionType = "pmg"
	ConnectionTypeVMware  ConnectionType = "vmware"
	ConnectionTypeTrueNAS ConnectionType = "truenas"
)

// platformType returns the canonical platform id of the connection's source,
// so a connection alert links to and names the platform it belongs to.
func (t ConnectionType) platformType() string {
	switch t {
	case ConnectionTypePVE:
		return "proxmox-pve"
	case ConnectionTypePBS:
		return "proxmox-pbs"
	case ConnectionTypePMG:
		return "proxmox-pmg"
	case ConnectionTypeVMware:
		return "vmware-vsphere"
	case ConnectionTypeTrueNAS:
		return "truenas"
	default:
		return ""
	}
}

// ConnectionErrorSnapshot mirrors the api.ConnectionError shape that the
// aggregator computes per target. Only the fields used in alert metadata are
// carried.
type ConnectionErrorSnapshot struct {
	At       time.Time
	Message  string
	Category string
}

// ConnectionSnapshot is the alerts-side view of one unified connection row.
// The api package's aggregator translates each platform api.Connection into
// this shape before invoking CheckConnection so alerts does not depend on
// api.
type ConnectionSnapshot struct {
	ID               string
	PolicyResourceID string
	Name             string
	Type             ConnectionType
	State            ConnectionState
	StateReason      string
	Enabled          bool
	LastSeen         *time.Time
	LastError        *ConnectionErrorSnapshot
}

// connectionDegradedAlertType is the alert.Type emitted for connection-degraded
// alerts. Surfacing this as a constant keeps notification routing and history
// indexing in lockstep with the producer.
const connectionDegradedAlertType = "connection-degraded"

// connectionDegradedStateKey is the DiscreteState spec stateKey for the
// connection-degraded canonical alert. The full state-ID derived by
// canonicalDiscreteStateStateID becomes "<connectionID>-<connectionDegradedStateKey>".
const connectionDegradedStateKey = "connection-degraded"

func isPlatformConnectionType(t ConnectionType) bool {
	switch t {
	case ConnectionTypePVE, ConnectionTypePBS, ConnectionTypePMG, ConnectionTypeVMware, ConnectionTypeTrueNAS:
		return true
	default:
		return false
	}
}

// connectionDegradedPolicyDisabledNoLock applies the alert policy owned by
// the platform resource to its canonical connection-degraded lifecycle. A
// platform connection is another observation of that resource's
// availability, not an independent alert-policy surface. Callers must hold at
// least m.mu.RLock.
func (m *Manager) connectionDegradedPolicyDisabledNoLock(resourceID, policyResourceID string, connectionType ConnectionType) bool {
	if !m.config.Enabled {
		return true
	}

	thresholdType := ""
	switch connectionType {
	case ConnectionTypePVE:
		thresholdType = "node"
	case ConnectionTypePBS:
		thresholdType = "pbs"
	case ConnectionTypePMG:
		thresholdType = "pmg"
	case ConnectionTypeVMware:
		thresholdType = "vmware-host"
	case ConnectionTypeTrueNAS:
		thresholdType = "truenas-system"
	default:
		return true
	}
	if allDisabled, offlineDisabled := m.alertPolicyTypeSwitchesNoLock(thresholdType); allDisabled || offlineDisabled {
		return true
	}

	policyResourceID = strings.TrimSpace(policyResourceID)
	if policyResourceID == "" {
		policyResourceID = resourceID
	}
	thresholds := m.resolveResourceThresholds(thresholdType, policyResourceID)
	return thresholds.Disabled || thresholds.DisableConnectivity
}

func connectionTypeFromAlert(alert *Alert) (ConnectionType, bool) {
	if alert == nil || alert.Type != connectionDegradedAlertType {
		return "", false
	}
	connectionType := ConnectionType(strings.TrimSpace(metadataStringValue(alert.Metadata, "connectionType")))
	return connectionType, isPlatformConnectionType(connectionType)
}

// suppressConnectionDegradedAlert immediately removes detector tracking and
// any active alert when the connection or its availability policy is disabled.
// Policy changes are authoritative and do not need healthy-poll confirmation.
func (m *Manager) suppressConnectionDegradedAlert(snap ConnectionSnapshot) {
	alertID := canonicalDiscreteStateStateID(snap.ID, connectionDegradedStateKey)

	m.mu.Lock()
	m.interruptConnectionDegradedRunNoLock(snap.ID)
	m.mu.Unlock()

	m.clearAlert(alertID)
}

// interruptConnectionDegradedRunNoLock ends confirmation and intent runs for
// this connection only. Unknown evidence is neither degradation nor recovery:
// keep any firing occurrence, but require fresh consecutive healthy polls.
// Callers hold m.mu.
func (m *Manager) interruptConnectionDegradedRunNoLock(resourceID string) {
	specKey := canonicalDiscreteStateSpecID(resourceID, connectionDegradedStateKey)
	for _, state := range m.mirrorStatesNoLock() {
		state.InterruptDiscreteRun(resourceID, specKey)
	}
	m.clearConnectionDegradedIntentNoLock(resourceID)
}

func (m *Manager) clearConnectionDegradedIntentNoLock(resourceID string) {
	trackingKey := canonicalDiscreteStateStateID(resourceID, connectionDegradedStateKey)
	if m.clearIntentPendingNoLock(trackingKey) {
		m.saveActiveAlertsAsync("connection confirmation interrupted")
	}
}

// CheckConnection raises or clears the connection-degraded alert for one
// platform connection. Severity scales with observed state: stale → warning,
// unreachable / unauthorized → critical. State=active runs through the
// recovery-confirmation gate before resolving. Paused, disabled, and
// non-platform connections are skipped.
func (m *Manager) CheckConnection(snap ConnectionSnapshot) {
	if !isPlatformConnectionType(snap.Type) {
		return
	}
	if strings.TrimSpace(snap.ID) == "" {
		return
	}
	m.mu.RLock()
	policyDisabled := m.connectionDegradedPolicyDisabledNoLock(snap.ID, snap.PolicyResourceID, snap.Type)
	m.mu.RUnlock()
	if !snap.Enabled || snap.State == ConnectionStatePaused || policyDisabled {
		m.suppressConnectionDegradedAlert(snap)
		return
	}

	switch snap.State {
	case ConnectionStateActive:
		m.clearConnectionDegradedAlert(snap)
		return
	case ConnectionStateStale, ConnectionStateUnreachable, ConnectionStateUnauthorized:
		// fall through and fire
	default:
		// Pending/unknown breaks both confirmation runs without resolving an
		// existing outage or carrying activation grace across the gap.
		m.mu.Lock()
		m.interruptConnectionDegradedRunNoLock(snap.ID)
		m.mu.Unlock()
		return
	}

	severity := AlertLevelWarning
	if snap.State == ConnectionStateUnreachable || snap.State == ConnectionStateUnauthorized {
		severity = AlertLevelCritical
	}

	// The reducer resets recovery confirmations on degraded observations.
	alertID := canonicalDiscreteStateStateID(snap.ID, connectionDegradedStateKey)

	spec, err := buildCanonicalDiscreteStateSpec(
		snap.ID,
		snap.Name,
		unifiedresources.ResourceType("connection"),
		severity,
		3,
		false,
		connectionDegradedStateKey,
		[]string{
			string(ConnectionStateStale),
			string(ConnectionStateUnreachable),
			string(ConnectionStateUnauthorized),
		},
	)
	if err != nil {
		log.Warn().
			Err(err).
			Str("connection", snap.Name).
			Str("connectionID", snap.ID).
			Msg("Skipping invalid canonical connection-degraded spec")
		return
	}

	reason := strings.TrimSpace(snap.StateReason)
	message := fmt.Sprintf("Connection '%s' is %s", snap.Name, snap.State)
	if reason != "" {
		message = fmt.Sprintf("%s: %s", message, reason)
	}

	metadata := map[string]interface{}{
		"resourceType":   "connection",
		"connectionType": string(snap.Type),
		"state":          string(snap.State),
	}
	if platformType := snap.Type.platformType(); platformType != "" {
		metadata[alertPlatformTypeKey] = platformType
	}
	var correlation *AlertCorrelation
	if snap.Type == ConnectionTypePVE {
		correlation = NewSharedSystemAlertCorrelation(
			snap.ID,
			AlertCorrelationRolePrimary,
			"proxmox-connection-ownership",
		)
	}
	if policyResourceID := strings.TrimSpace(snap.PolicyResourceID); policyResourceID != "" {
		metadata["policyResourceID"] = policyResourceID
	}
	if reason != "" {
		metadata["stateReason"] = reason
	}
	if snap.LastSeen != nil {
		metadata["lastSeen"] = snap.LastSeen.UTC()
	}
	if snap.LastError != nil && strings.TrimSpace(snap.LastError.Message) != "" {
		metadata["lastError"] = snap.LastError.Message
		if !snap.LastError.At.IsZero() {
			metadata["lastErrorAt"] = snap.LastError.At.UTC()
		}
		if snap.LastError.Category != "" {
			metadata["lastErrorCategory"] = snap.LastError.Category
		}
	}

	_, _ = m.evaluateCanonicalLifecycleAlert(canonicalLifecycleAlertParams{
		Spec: spec,
		Evidence: alertspecs.AlertEvidence{
			ObservedAt: time.Now(),
			DiscreteState: &alertspecs.DiscreteStateEvidence{
				StateKey: connectionDegradedStateKey,
				Observed: string(snap.State),
			},
		},
		IntentSignal: string(AlertIntentSignalOffline),
		PolicyDisabledNoLock: func() bool {
			return m.connectionDegradedPolicyDisabledNoLock(snap.ID, snap.PolicyResourceID, snap.Type)
		},
		AlertID:       alertID,
		AlertType:     connectionDegradedAlertType,
		ResourceID:    snap.ID,
		ResourceName:  snap.Name,
		Instance:      snap.Name,
		Message:       message,
		Correlation:   correlation,
		Metadata:      metadata,
		AddToRecent:   true,
		AddToHistory:  true,
		RateLimit:     true,
		DispatchAsync: false,
	})
}

// clearConnectionDegradedAlert resolves an active connection-degraded alert
// after enough consecutive healthy observations to confirm recovery. Mirrors
// clearNodeOfflineAlert so a single flap back to active does not silently
// resolve a real outage. Callers hold no manager locks.
func (m *Manager) clearConnectionDegradedAlert(snap ConnectionSnapshot) {
	alertID := canonicalDiscreteStateStateID(snap.ID, connectionDegradedStateKey)
	specKey := canonicalDiscreteStateSpecID(snap.ID, connectionDegradedStateKey)

	m.mu.Lock()
	defer m.mu.Unlock()
	defer m.shadowObserveRecoveryNoLock(snap.ID, specKey, alertID, offlineRecoveryConfirmationsDefault)

	// Healthy evidence ends pending activation grace as well as the reducer's
	// activation count. A firing incident still uses the recovery gate below.
	m.clearConnectionDegradedIntentNoLock(snap.ID)
	m.resolveDiscreteRecoveryNoLock(snap.ID, specKey, alertID, offlineRecoveryConfirmationsDefault, "Connection", snap.Name, snap.ID)
}
