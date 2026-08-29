package alerts

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestNewSharedSystemAlertCorrelationFailsOpenOnIncompleteIdentity(t *testing.T) {
	for _, tc := range []struct {
		name   string
		key    string
		role   AlertCorrelationRole
		reason string
	}{
		{name: "missing key", role: AlertCorrelationRolePrimary, reason: "verified"},
		{name: "missing reason", key: "pve:delly", role: AlertCorrelationRolePrimary},
		{name: "unknown role", key: "pve:delly", role: AlertCorrelationRole("peer"), reason: "verified"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := NewSharedSystemAlertCorrelation(tc.key, tc.role, tc.reason); got != nil {
				t.Fatalf("expected invalid correlation to fail open, got %+v", got)
			}
		})
	}
}

func TestAlertCloneDoesNotShareCorrelation(t *testing.T) {
	original := &Alert{
		ID: "connection-alert",
		Correlation: NewSharedSystemAlertCorrelation(
			"pve:delly",
			AlertCorrelationRolePrimary,
			"proxmox-connection-ownership",
		),
	}
	clone := original.Clone()
	if clone.Correlation == nil || clone.Correlation == original.Correlation {
		t.Fatalf("expected independent correlation copy, got original=%p clone=%p", original.Correlation, clone.Correlation)
	}
	original.Correlation.Key = "pve:changed"
	if clone.Correlation.Key != "pve:delly" {
		t.Fatalf("clone correlation changed with original: %+v", clone.Correlation)
	}
}

func TestAlertCloneDropsMalformedPersistedCorrelation(t *testing.T) {
	for _, correlation := range []*AlertCorrelation{
		{Key: "pve:delly", Kind: AlertCorrelationKind("causal"), Role: AlertCorrelationRolePrimary, Reason: "legacy"},
		{Key: "pve:delly", Kind: AlertCorrelationKindSharedSystem, Role: AlertCorrelationRole("peer"), Reason: "legacy"},
		{Key: "", Kind: AlertCorrelationKindSharedSystem, Role: AlertCorrelationRolePrimary, Reason: "legacy"},
	} {
		clone := (&Alert{ID: "persisted", Correlation: correlation}).Clone()
		if clone.Correlation != nil {
			t.Fatalf("malformed persisted correlation must fail open, got %+v", clone.Correlation)
		}
	}
}

func TestConnectivityAlertsRetainIndependentLifecyclesWithSharedSystemCorrelation(t *testing.T) {
	m := newTestManager(t)
	node := models.Node{ID: "node/delly/pve-1", Name: "pve-1", Instance: "delly"}
	host := models.Host{ID: "agent-1", Hostname: "pve-1", DisplayName: "PVE Agent"}
	hostCorrelation := NewSharedSystemAlertCorrelation(
		"pve:delly",
		AlertCorrelationRoleSupporting,
		"verified-proxmox-node-agent-link",
	)

	for range 3 {
		m.checkNodeOffline(node)
		m.HandleHostOfflineWithCorrelation(host, hostCorrelation)
	}

	nodeAlert := testRequireActiveAlert(t, m, canonicalConnectivityStateID(node.ID))
	hostAlert := testRequireActiveAlert(t, m, canonicalConnectivityStateID(hostResourceID(host.ID)))
	if nodeAlert.ID == hostAlert.ID {
		t.Fatal("shared-system signals must retain separate canonical alert identities")
	}
	for _, alert := range []*Alert{nodeAlert, hostAlert} {
		if alert.Correlation == nil || alert.Correlation.Key != "pve:delly" {
			t.Fatalf("alert %q missing shared-system correlation: %+v", alert.ID, alert.Correlation)
		}
		if alert.Correlation.Role != AlertCorrelationRoleSupporting {
			t.Fatalf("alert %q role = %q, want supporting", alert.ID, alert.Correlation.Role)
		}
	}

	m.HandleHostOnline(host)
	if testHasActiveAlert(t, m, hostAlert.ID) {
		t.Fatal("host recovery should resolve only the host detector lifecycle")
	}
	if !testHasActiveAlert(t, m, nodeAlert.ID) {
		t.Fatal("host recovery must not resolve the still-firing node detector")
	}
}

func TestPVEConnectionAlertIsSharedSystemPrimary(t *testing.T) {
	m := newTestManager(t)
	snap := platformConnectionSnapshot("pve:delly", "Delly", ConnectionStateUnreachable)
	for range 3 {
		m.CheckConnection(snap)
	}

	alert := testRequireActiveAlert(t, m, canonicalDiscreteStateStateID(snap.ID, connectionDegradedStateKey))
	if alert.Correlation == nil {
		t.Fatal("expected PVE connection correlation")
	}
	if alert.Correlation.Key != snap.ID || alert.Correlation.Role != AlertCorrelationRolePrimary {
		t.Fatalf("unexpected PVE connection correlation: %+v", alert.Correlation)
	}
}
