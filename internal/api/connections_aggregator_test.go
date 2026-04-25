package api

import (
	"reflect"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
)

func ptrTime(t time.Time) *time.Time { return &t }

func healthEntry(lastSuccess *time.Time, errMessage, errCategory string, breakerState string) monitoring.InstanceHealth {
	ps := monitoring.InstancePollStatus{LastSuccess: lastSuccess}
	if errMessage != "" {
		ps.LastError = &monitoring.ErrorDetail{
			At:       time.Now(),
			Message:  errMessage,
			Category: errCategory,
		}
	}
	return monitoring.InstanceHealth{
		PollStatus: ps,
		Breaker:    monitoring.InstanceBreaker{State: breakerState},
	}
}

func TestDeriveConnectionState_Paused(t *testing.T) {
	state, reason, _, _ := deriveConnectionState(false, monitoring.InstanceHealth{}, time.Now())
	if state != ConnectionStatePaused {
		t.Fatalf("got %q, want %q", state, ConnectionStatePaused)
	}
	if reason == "" {
		t.Fatal("expected non-empty reason for paused state")
	}
}

func TestDeriveConnectionState_Pending(t *testing.T) {
	state, _, lastSeen, lastError := deriveConnectionState(true, monitoring.InstanceHealth{}, time.Now())
	if state != ConnectionStatePending {
		t.Fatalf("got %q, want %q", state, ConnectionStatePending)
	}
	if lastSeen != nil || lastError != nil {
		t.Fatal("expected nil lastSeen and lastError in pending state")
	}
}

func TestDeriveConnectionState_Unauthorized(t *testing.T) {
	now := time.Now()
	h := healthEntry(ptrTime(now.Add(-30*time.Second)), "401 Unauthorized: token invalid", "auth", "closed")
	state, reason, _, err := deriveConnectionState(true, h, now)
	if state != ConnectionStateUnauthorized {
		t.Fatalf("got %q, want %q", state, ConnectionStateUnauthorized)
	}
	if reason == "" {
		t.Fatal("expected reason to include error message")
	}
	if err == nil || err.Message != "401 Unauthorized: token invalid" {
		t.Fatalf("unexpected lastError: %+v", err)
	}
}

func TestDeriveConnectionState_Unreachable(t *testing.T) {
	now := time.Now()
	h := healthEntry(ptrTime(now.Add(-30*time.Second)), "connection refused", "network", "open")
	state, _, _, _ := deriveConnectionState(true, h, now)
	if state != ConnectionStateUnreachable {
		t.Fatalf("got %q, want %q", state, ConnectionStateUnreachable)
	}
}

func TestDeriveConnectionState_Stale(t *testing.T) {
	now := time.Now()
	stale := now.Add(-5 * time.Minute)
	h := healthEntry(ptrTime(stale), "", "", "closed")
	state, reason, _, _ := deriveConnectionState(true, h, now)
	if state != ConnectionStateStale {
		t.Fatalf("got %q, want %q", state, ConnectionStateStale)
	}
	if reason == "" {
		t.Fatal("expected reason to describe staleness")
	}
}

func TestDeriveConnectionState_Active(t *testing.T) {
	now := time.Now()
	h := healthEntry(ptrTime(now.Add(-10*time.Second)), "", "", "closed")
	state, _, _, _ := deriveConnectionState(true, h, now)
	if state != ConnectionStateActive {
		t.Fatalf("got %q, want %q", state, ConnectionStateActive)
	}
}

func TestBuildConnections_SortsByTypeThenName(t *testing.T) {
	now := time.Now()
	in := aggregatorInputs{
		pveInstances: []config.PVEInstance{
			{Name: "beta", Host: "https://b.lan:8006", MonitorVMs: true},
			{Name: "alpha", Host: "https://a.lan:8006", MonitorVMs: true},
		},
		pbsInstances: []config.PBSInstance{
			{Name: "backups", Host: "https://bkp.lan:8007"},
		},
		now: now,
	}
	got := buildConnections(in)
	if len(got) != 3 {
		t.Fatalf("expected 3 connections, got %d", len(got))
	}
	if got[0].Type != ConnectionTypePBS {
		t.Fatalf("expected PBS first, got %q", got[0].Type)
	}
	if got[1].Name != "alpha" || got[2].Name != "beta" {
		t.Fatalf("PVE order wrong: %q, %q", got[1].Name, got[2].Name)
	}
}

func TestBuildConnections_PVEPausedRespectsDisabled(t *testing.T) {
	in := aggregatorInputs{
		pveInstances: []config.PVEInstance{{Name: "pve1", Host: "https://pve1.lan:8006", Disabled: true}},
		now:          time.Now(),
	}
	got := buildConnections(in)
	if len(got) != 1 {
		t.Fatalf("expected 1 connection, got %d", len(got))
	}
	if got[0].State != ConnectionStatePaused {
		t.Fatalf("expected paused, got %q", got[0].State)
	}
	if got[0].Enabled {
		t.Fatal("expected Enabled=false for Disabled PVE instance")
	}
}

func TestBuildConnections_AgentStateFromLastSeen(t *testing.T) {
	now := time.Now()
	in := aggregatorInputs{
		hosts: []models.Host{
			{ID: "fresh", Hostname: "h1", LastSeen: now.Add(-10 * time.Second)},
			{ID: "stale", Hostname: "h2", LastSeen: now.Add(-5 * time.Minute)},
			{ID: "never", Hostname: "h3"},
		},
		now: now,
	}
	got := buildConnections(in)
	byID := map[string]Connection{}
	for _, c := range got {
		byID[c.ID] = c
	}
	if byID["agent:fresh"].State != ConnectionStateActive {
		t.Fatalf("fresh agent: got %q, want active", byID["agent:fresh"].State)
	}
	if byID["agent:stale"].State != ConnectionStateStale {
		t.Fatalf("stale agent: got %q, want stale", byID["agent:stale"].State)
	}
	if byID["agent:never"].State != ConnectionStatePending {
		t.Fatalf("never-reported agent: got %q, want pending", byID["agent:never"].State)
	}
	for _, c := range got {
		if c.Capabilities.SupportsPause || c.Capabilities.SupportsScope {
			t.Fatalf("agents must not advertise pause/scope capabilities: %+v", c)
		}
	}
}

func TestBuildConnections_AgentHostAliasesIncludeReportedIdentityHints(t *testing.T) {
	now := time.Now()
	in := aggregatorInputs{
		hosts: []models.Host{
			{
				ID:              "pi",
				Hostname:        "pi",
				Platform:        "unraid",
				OSName:          "Unraid",
				OSVersion:       "7.1.0",
				KernelVersion:   "6.12.0",
				Architecture:    "x86_64",
				ReportIP:        "192.168.0.2",
				LastSeen:        now,
				AgentVersion:    "6.0.0",
				CommandsEnabled: true,
				NetworkInterfaces: []models.HostNetworkInterface{
					{Name: "eth0", Addresses: []string{"192.168.0.2/24", "fe80::1/64"}},
				},
			},
		},
		expectedAgentVersion: "6.0.1",
		now:                  now,
	}

	got := buildConnections(in)
	if len(got) != 1 {
		t.Fatalf("expected 1 connection, got %d", len(got))
	}
	if !reflect.DeepEqual(got[0].HostAliases, []string{"pi", "192.168.0.2", "fe80::1"}) {
		t.Fatalf("agent host aliases = %+v, want %+v", got[0].HostAliases, []string{"pi", "192.168.0.2", "fe80::1"})
	}
	if got[0].AgentIdentity == nil {
		t.Fatal("expected agent identity metadata")
	}
	if !reflect.DeepEqual(
		got[0].AgentIdentity,
		&ConnectionAgentIdentity{
			Hostname:        "pi",
			Platform:        "unraid",
			OSName:          "Unraid",
			OSVersion:       "7.1.0",
			KernelVersion:   "6.12.0",
			Architecture:    "x86_64",
			ReportIP:        "192.168.0.2",
			CommandsEnabled: true,
		},
	) {
		t.Fatalf("agent identity = %+v", got[0].AgentIdentity)
	}
}

func TestBuildConnections_AgentVersionUpdateAvailability(t *testing.T) {
	now := time.Now()
	in := aggregatorInputs{
		hosts: []models.Host{
			{ID: "current", Hostname: "current", LastSeen: now, AgentVersion: "6.0.2"},
			{ID: "outdated", Hostname: "outdated", LastSeen: now, AgentVersion: "6.0.0"},
			{ID: "unknown", Hostname: "unknown", LastSeen: now},
		},
		expectedAgentVersion: "6.0.2",
		now:                  now,
	}

	got := buildConnections(in)
	byID := map[string]Connection{}
	for _, connection := range got {
		byID[connection.ID] = connection
	}

	if byID["agent:current"].AgentVersion != "6.0.2" {
		t.Fatalf("current agent version = %q, want %q", byID["agent:current"].AgentVersion, "6.0.2")
	}
	if byID["agent:current"].ExpectedAgentVersion != "6.0.2" {
		t.Fatalf(
			"current expected agent version = %q, want %q",
			byID["agent:current"].ExpectedAgentVersion,
			"6.0.2",
		)
	}
	if byID["agent:current"].AgentUpdateAvailable {
		t.Fatal("current agent should not report an update available")
	}

	if byID["agent:outdated"].AgentVersion != "6.0.0" {
		t.Fatalf(
			"outdated agent version = %q, want %q",
			byID["agent:outdated"].AgentVersion,
			"6.0.0",
		)
	}
	if !byID["agent:outdated"].AgentUpdateAvailable {
		t.Fatal("outdated agent should report an update available")
	}

	if byID["agent:unknown"].AgentVersion != "" {
		t.Fatalf("unknown agent version = %q, want empty", byID["agent:unknown"].AgentVersion)
	}
	if byID["agent:unknown"].AgentUpdateAvailable {
		t.Fatal("agent without a reported version should not raise an update flag")
	}
}

func TestBuildConnections_AgentFleetGovernance(t *testing.T) {
	now := time.Now()
	in := aggregatorInputs{
		hosts: []models.Host{
			{
				ID:              "current",
				Hostname:        "current",
				LastSeen:        now,
				AgentVersion:    "6.0.2",
				CommandsEnabled: true,
			},
			{
				ID:           "outdated",
				Hostname:     "outdated",
				LastSeen:     now,
				AgentVersion: "6.0.0",
			},
			{
				ID:       "pending",
				Hostname: "pending",
			},
		},
		expectedAgentVersion: "6.0.2",
		now:                  now,
	}

	got := buildConnections(in)
	byID := map[string]Connection{}
	for _, connection := range got {
		byID[connection.ID] = connection
	}

	current := byID["agent:current"].Fleet
	if current.EnrollmentState != fleetStateEnrolled ||
		current.LivenessState != fleetStateActive ||
		current.VersionDrift != fleetStateCurrent ||
		current.AdapterHealth != fleetStateHealthy ||
		current.ConfigRollout != fleetStateReported ||
		current.CredentialStatus != fleetStateVerified ||
		current.UpdateStatus != fleetStateCurrent ||
		current.RemoteControl != fleetStateEnabled {
		t.Fatalf("current agent fleet governance = %+v", current)
	}

	outdated := byID["agent:outdated"].Fleet
	if outdated.VersionDrift != fleetStateBehind ||
		outdated.UpdateStatus != fleetStateUpdateAvailable ||
		outdated.RemoteControl != fleetStateDisabled {
		t.Fatalf("outdated agent fleet governance = %+v", outdated)
	}

	pending := byID["agent:pending"].Fleet
	if pending.EnrollmentState != fleetStatePending ||
		pending.LivenessState != string(ConnectionStatePending) ||
		pending.AdapterHealth != fleetStateDegraded ||
		pending.ConfigRollout != fleetStateUnknown ||
		pending.CredentialStatus != fleetStateUnknown {
		t.Fatalf("pending agent fleet governance = %+v", pending)
	}
}

func TestBuildConnections_PlatformFleetGovernance(t *testing.T) {
	now := time.Now()
	lastSuccess := now.Add(-30 * time.Second)
	in := aggregatorInputs{
		pveInstances: []config.PVEInstance{
			{Name: "healthy", Host: "https://healthy.lan:8006"},
			{Name: "bad-token", Host: "https://bad-token.lan:8006"},
			{Name: "paused", Host: "https://paused.lan:8006", Disabled: true},
		},
		instanceHealth: map[string]monitoring.InstanceHealth{
			"pve::healthy":   healthEntry(&lastSuccess, "", "", "closed"),
			"pve::bad-token": healthEntry(&lastSuccess, "403 forbidden", "auth", "closed"),
		},
		now: now,
	}

	got := buildConnections(in)
	byID := map[string]Connection{}
	for _, connection := range got {
		byID[connection.ID] = connection
	}

	healthy := byID["pve:healthy"].Fleet
	if healthy.EnrollmentState != fleetStateConfigured ||
		healthy.LivenessState != fleetStateActive ||
		healthy.VersionDrift != fleetStateNotApplicable ||
		healthy.AdapterHealth != fleetStateHealthy ||
		healthy.ConfigRollout != fleetStateConfigured ||
		healthy.CredentialStatus != fleetStateVerified ||
		healthy.UpdateStatus != fleetStateNotApplicable ||
		healthy.RemoteControl != fleetStateNotApplicable {
		t.Fatalf("healthy platform fleet governance = %+v", healthy)
	}

	badToken := byID["pve:bad-token"].Fleet
	if badToken.AdapterHealth != fleetStateBlocked ||
		badToken.CredentialStatus != fleetStateInvalid ||
		badToken.LivenessState != string(ConnectionStateUnauthorized) {
		t.Fatalf("bad token fleet governance = %+v", badToken)
	}

	paused := byID["pve:paused"].Fleet
	if paused.EnrollmentState != fleetStatePaused ||
		paused.AdapterHealth != fleetStatePaused ||
		paused.ConfigRollout != fleetStatePaused ||
		paused.CredentialStatus != fleetStatePaused {
		t.Fatalf("paused platform fleet governance = %+v", paused)
	}
}

func TestBuildConnections_VMwareAndTrueNASEnabledFlag(t *testing.T) {
	in := aggregatorInputs{
		vmwareInstances: []config.VMwareVCenterInstance{{
			ID: "vc1", Name: "vc", Host: "vc.lan", Enabled: false,
			MonitorVMs: true, MonitorHosts: true, MonitorDatastores: false,
		}},
		truenasInstances: []config.TrueNASInstance{{
			ID: "tn1", Name: "tn", Host: "tn.lan", Enabled: true, UseHTTPS: true,
			MonitorDatasets: true, MonitorPools: false, MonitorReplication: true,
		}},
		now: time.Now(),
	}
	got := buildConnections(in)
	var vmw, tn Connection
	for _, c := range got {
		switch c.Type {
		case ConnectionTypeVMware:
			vmw = c
		case ConnectionTypeTrueNAS:
			tn = c
		}
	}
	if vmw.State != ConnectionStatePaused || vmw.Enabled {
		t.Fatalf("vmware with Enabled=false should be paused, got state=%q enabled=%v", vmw.State, vmw.Enabled)
	}
	if !vmw.Capabilities.SupportsScope {
		t.Fatal("vmware connections must advertise scope capability")
	}
	if vmw.Scope["datastores"] || !vmw.Scope["vms"] || !vmw.Scope["hosts"] {
		t.Fatalf("vmware scope map should reflect Monitor* fields, got %+v", vmw.Scope)
	}
	if tn.State != ConnectionStatePending {
		t.Fatalf("truenas with no health yet should be pending, got %q", tn.State)
	}
	if !tn.Enabled {
		t.Fatal("truenas with Enabled=true should surface enabled=true")
	}
	if !tn.Capabilities.SupportsScope {
		t.Fatal("truenas connections must advertise scope capability")
	}
	if tn.Scope["pools"] || !tn.Scope["datasets"] || !tn.Scope["replication"] {
		t.Fatalf("truenas scope map should reflect Monitor* fields, got %+v", tn.Scope)
	}
}

func TestBuildConnections_UsesHealthLookup(t *testing.T) {
	now := time.Now()
	ls := now.Add(-15 * time.Second)
	in := aggregatorInputs{
		pveInstances: []config.PVEInstance{{Name: "pve1", Host: "https://pve1.lan:8006"}},
		instanceHealth: map[string]monitoring.InstanceHealth{
			"pve::pve1": healthEntry(&ls, "", "", "closed"),
		},
		now: now,
	}
	got := buildConnections(in)
	if got[0].State != ConnectionStateActive {
		t.Fatalf("expected active, got %q", got[0].State)
	}
	if got[0].LastSeen == nil || !got[0].LastSeen.Equal(ls) {
		t.Fatalf("lastSeen not propagated: %+v", got[0].LastSeen)
	}
}
