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
	ID          string
	Name        string
	Type        ConnectionType
	State       ConnectionState
	StateReason string
	Enabled     bool
	LastSeen    *time.Time
	LastError   *ConnectionErrorSnapshot
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
	if !snap.Enabled || snap.State == ConnectionStatePaused {
		m.clearConnectionDegradedAlert(snap)
		return
	}

	switch snap.State {
	case ConnectionStateActive:
		m.clearConnectionDegradedAlert(snap)
		return
	case ConnectionStateStale, ConnectionStateUnreachable, ConnectionStateUnauthorized:
		// fall through and fire
	default:
		// pending/unknown — no alert, but reset the consecutive count so a
		// later degraded run starts from zero instead of accumulating across
		// a transient pending blip.
		m.mu.Lock()
		delete(m.connectionDegradedCount, snap.ID)
		m.mu.Unlock()
		return
	}

	severity := AlertLevelWarning
	if snap.State == ConnectionStateUnreachable || snap.State == ConnectionStateUnauthorized {
		severity = AlertLevelCritical
	}

	// Another degraded observation invalidates any in-flight recovery
	// confirmations — recovery must build back up from zero.
	alertID := canonicalDiscreteStateStateID(snap.ID, connectionDegradedStateKey)
	m.mu.Lock()
	delete(m.offlineRecoveryConfirmations, alertID)
	m.mu.Unlock()

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
		Tracking:      m.connectionDegradedCount,
		TrackingKey:   snap.ID,
		AlertID:       alertID,
		AlertType:     connectionDegradedAlertType,
		ResourceID:    snap.ID,
		ResourceName:  snap.Name,
		Instance:      snap.Name,
		Message:       message,
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

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.connectionDegradedCount[snap.ID] > 0 {
		log.Debug().
			Str("connection", snap.Name).
			Int("previousCount", m.connectionDegradedCount[snap.ID]).
			Msg("Connection healthy, resetting degraded count")
		delete(m.connectionDegradedCount, snap.ID)
	}

	alert, exists := m.getActiveAlertNoLock(alertID)
	if !exists {
		delete(m.offlineRecoveryConfirmations, alertID)
		return
	}

	recoveryCount, confirmed := m.confirmOfflineRecoveryNoLock(alertID, offlineRecoveryConfirmationsDefault)
	if !confirmed {
		log.Debug().
			Str("connection", snap.Name).
			Int("confirmations", recoveryCount).
			Int("required", offlineRecoveryConfirmationsDefault).
			Msg("Connection appears healthy, waiting for recovery confirmation")
		return
	}

	m.removeActiveAlertNoLock(alertID)

	resolvedAlert := m.newResolvedAlert(alert, time.Now(), nil)
	m.addRecentlyResolvedWithPrimaryLock(resolvedAlert)
	m.safeCallResolvedAlertCallback(alert, alertID, true)

	log.Info().
		Str("connection", snap.Name).
		Str("connectionID", snap.ID).
		Dur("downtime", time.Since(alert.StartTime)).
		Msg("Connection back to active")
}
