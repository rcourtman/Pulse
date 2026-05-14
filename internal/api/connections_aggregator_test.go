package api

import (
	"reflect"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/remoteconfig"
)

func ptrTime(t time.Time) *time.Time { return &t }

func ptrBool(v bool) *bool { return &v }

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

func desiredAgentConfigFingerprint(t *testing.T, commandsEnabled *bool, settings map[string]interface{}) ConnectionFleetConfigFingerprint {
	t.Helper()
	desired := desiredAgentConfig(t, commandsEnabled, settings)
	if desired.Fingerprint == nil {
		t.Fatal("expected desired config fingerprint")
	}
	return *desired.Fingerprint
}

func desiredAgentConfig(t *testing.T, commandsEnabled *bool, settings map[string]interface{}) connectionAgentDesiredConfig {
	t.Helper()
	metadata, err := remoteconfig.BuildDesiredConfigMetadata(commandsEnabled, settings)
	if err != nil {
		t.Fatalf("BuildDesiredConfigMetadata: %v", err)
	}
	fingerprint := ConnectionFleetConfigFingerprint{
		Version: metadata.Version,
		Hash:    metadata.Hash,
	}
	return connectionAgentDesiredConfig{
		Fingerprint:     &fingerprint,
		CommandsEnabled: cloneBoolPtr(commandsEnabled),
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

func TestBuildConnections_AvailabilityTargetStateFromProbeStatus(t *testing.T) {
	checkedAt := time.Date(2026, 5, 6, 10, 0, 0, 0, time.UTC)
	in := aggregatorInputs{
		availabilityTargets: []config.AvailabilityTarget{
			{
				ID:               "sensor-1",
				Name:             "Energy monitor",
				Address:          "192.0.2.10",
				Protocol:         config.AvailabilityProbeICMP,
				Enabled:          true,
				FailureThreshold: 2,
			},
		},
		availabilityStatuses: map[string]monitoring.AvailabilityProbeStatus{
			"sensor-1": {
				TargetID:            "sensor-1",
				Available:           false,
				LastChecked:         checkedAt,
				ConsecutiveFailures: 1,
				LastError:           "timeout",
			},
		},
		now: checkedAt.Add(5 * time.Second),
	}

	got := buildConnections(in)
	if len(got) != 1 {
		t.Fatalf("expected 1 connection, got %d", len(got))
	}
	connection := got[0]
	if connection.ID != "availability:sensor-1" {
		t.Fatalf("ID = %q, want availability:sensor-1", connection.ID)
	}
	if connection.Type != ConnectionTypeAvailability {
		t.Fatalf("Type = %q, want availability", connection.Type)
	}
	if connection.State != ConnectionStateUnreachable {
		t.Fatalf("State = %q, want unreachable", connection.State)
	}
	if connection.LastError == nil || connection.LastError.Category != "availability" {
		t.Fatalf("LastError = %+v, want availability category", connection.LastError)
	}
	if !reflect.DeepEqual(connection.Surfaces, []string{"availability"}) {
		t.Fatalf("Surfaces = %+v, want [availability]", connection.Surfaces)
	}
	if !connection.Capabilities.SupportsPause || connection.Capabilities.SupportsScope || !connection.Capabilities.SupportsTest {
		t.Fatalf("Capabilities = %+v, want pause/test without scope", connection.Capabilities)
	}
	if !reflect.DeepEqual(connection.HostAliases, []string{"192.0.2.10"}) {
		t.Fatalf("HostAliases = %+v, want endpoint address", connection.HostAliases)
	}
}

func TestBuildConnections_AgentHostAliasesIncludeReportedIdentityHints(t *testing.T) {
	now := time.Now()
	in := aggregatorInputs{
		hosts: []models.Host{
			{
				ID:              "pi",
				Hostname:        "pi",
				Platform:        "slackware",
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
			Platform:        "linux",
			HostProfile:     "unraid",
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

func TestBuildConnections_AgentHostProfileFromUnraidStorageFacts(t *testing.T) {
	now := time.Now()
	in := aggregatorInputs{
		hosts: []models.Host{
			{
				ID:              "tower",
				Hostname:        "tower",
				Platform:        "linux",
				Unraid:          &models.HostUnraidStorage{ArrayStarted: true},
				LastSeen:        now,
				AgentVersion:    "6.12.10",
				CommandsEnabled: false,
			},
		},
		now: now,
	}

	got := buildConnections(in)
	if len(got) != 1 {
		t.Fatalf("expected 1 connection, got %d", len(got))
	}
	identity := got[0].AgentIdentity
	if identity == nil {
		t.Fatal("expected agent identity metadata")
	}
	if identity.HostProfile != "unraid" {
		t.Fatalf("host profile = %q, want %q", identity.HostProfile, "unraid")
	}
	if identity.Platform != "linux" {
		t.Fatalf("platform = %q, want %q", identity.Platform, "linux")
	}
	if identity.OSName != "" {
		t.Fatalf("os name = %q, want empty when source identity is absent", identity.OSName)
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
	currentDesiredConfig := desiredAgentConfig(t, ptrBool(true), nil)
	currentDesired := *currentDesiredConfig.Fingerprint
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
		agentDesiredConfigs: map[string]connectionAgentDesiredConfig{
			"current": currentDesiredConfig,
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
	if current.ConfigDrift == nil ||
		current.ConfigDrift.Status != fleetStatePending ||
		current.ConfigDrift.Desired == nil ||
		current.ConfigDrift.Applied != nil ||
		current.ConfigDrift.Desired.Version != connectionAgentConfigFingerprintVersion ||
		current.ConfigDrift.Desired.Hash != currentDesired.Hash {
		t.Fatalf("current agent config drift = %+v", current.ConfigDrift)
	}
	if current.Rollout == nil || current.Rollout.Status != fleetStatePending || current.Rollout.Stage != fleetRolloutStagePending {
		t.Fatalf("current agent rollout = %+v", current.Rollout)
	}
	if current.CommandPolicy == nil ||
		current.CommandPolicy.Status != fleetStateEnabled ||
		current.CommandPolicy.Desired != fleetStateEnabled ||
		current.CommandPolicy.Applied != fleetStateEnabled ||
		current.CommandPolicy.Enforcement != fleetCommandPolicyInSync {
		t.Fatalf("current agent command policy = %+v", current.CommandPolicy)
	}

	outdated := byID["agent:outdated"].Fleet
	if outdated.VersionDrift != fleetStateBehind ||
		outdated.UpdateStatus != fleetStateUpdateAvailable ||
		outdated.RemoteControl != fleetStateDisabled {
		t.Fatalf("outdated agent fleet governance = %+v", outdated)
	}
	if outdated.CommandPolicy == nil ||
		outdated.CommandPolicy.Status != fleetStateDisabled ||
		outdated.CommandPolicy.Desired != fleetStateUnknown ||
		outdated.CommandPolicy.Applied != fleetStateDisabled ||
		outdated.CommandPolicy.Enforcement != fleetCommandPolicyNotApplicable {
		t.Fatalf("outdated agent command policy = %+v", outdated.CommandPolicy)
	}

	pending := byID["agent:pending"].Fleet
	if pending.EnrollmentState != fleetStatePending ||
		pending.LivenessState != string(ConnectionStatePending) ||
		pending.AdapterHealth != fleetStateDegraded ||
		pending.ConfigRollout != fleetStateUnknown ||
		pending.CredentialStatus != fleetStateUnknown {
		t.Fatalf("pending agent fleet governance = %+v", pending)
	}
	if pending.ConfigDrift == nil || pending.ConfigDrift.Status != fleetStateUnknown {
		t.Fatalf("pending agent config drift = %+v", pending.ConfigDrift)
	}
	if pending.Rollout == nil || pending.Rollout.Status != fleetStatePending {
		t.Fatalf("pending agent rollout = %+v", pending.Rollout)
	}
	if pending.CommandPolicy == nil ||
		pending.CommandPolicy.Desired != fleetStateUnknown ||
		pending.CommandPolicy.Applied != fleetStateUnknown ||
		pending.CommandPolicy.Enforcement != fleetStatePending {
		t.Fatalf("pending agent command policy = %+v", pending.CommandPolicy)
	}
}

func TestBuildConnections_AgentWithoutManagedDesiredConfigDoesNotReportRolloutPending(t *testing.T) {
	now := time.Now()
	in := aggregatorInputs{
		hosts: []models.Host{
			{
				ID:              "default-agent",
				Hostname:        "default-agent",
				LastSeen:        now,
				CommandsEnabled: false,
			},
		},
		agentDesiredConfigs: map[string]connectionAgentDesiredConfig{
			"default-agent": {},
		},
		now: now,
	}

	got := buildConnections(in)
	if len(got) != 1 {
		t.Fatalf("expected 1 connection, got %d", len(got))
	}

	fleet := got[0].Fleet
	if fleet.ConfigDrift == nil ||
		fleet.ConfigDrift.Status != fleetConfigDriftNotApplicable ||
		fleet.ConfigDrift.Desired != nil ||
		fleet.ConfigDrift.Applied != nil {
		t.Fatalf("default agent config drift = %+v", fleet.ConfigDrift)
	}
	if fleet.Rollout == nil ||
		fleet.Rollout.Status != fleetStateCurrent ||
		fleet.Rollout.Stage != fleetRolloutStageApplied {
		t.Fatalf("default agent rollout = %+v", fleet.Rollout)
	}
}

func TestBuildConnections_AgentCommandPolicyDesiredAppliedConvergence(t *testing.T) {
	now := time.Now()
	desiredDisabled := desiredAgentConfig(t, ptrBool(false), nil)
	desiredEnabled := desiredAgentConfig(t, ptrBool(true), nil)
	in := aggregatorInputs{
		hosts: []models.Host{
			{
				ID:              "desired-disabled-applied-enabled",
				Hostname:        "desired-disabled-applied-enabled",
				LastSeen:        now,
				CommandsEnabled: true,
			},
			{
				ID:              "desired-enabled-applied-disabled",
				Hostname:        "desired-enabled-applied-disabled",
				LastSeen:        now,
				CommandsEnabled: false,
			},
			{
				ID:       "desired-disabled-unreported",
				Hostname: "desired-disabled-unreported",
			},
		},
		agentDesiredConfigs: map[string]connectionAgentDesiredConfig{
			"desired-disabled-applied-enabled": desiredDisabled,
			"desired-enabled-applied-disabled": desiredEnabled,
			"desired-disabled-unreported":      desiredDisabled,
		},
		now: now,
	}

	got := buildConnections(in)
	byID := map[string]Connection{}
	for _, connection := range got {
		byID[connection.ID] = connection
	}

	disabledDesired := byID["agent:desired-disabled-applied-enabled"].Fleet
	if disabledDesired.RemoteControl != fleetStateEnabled {
		t.Fatalf("remote control for desired-disabled/applied-enabled = %q, want applied enabled", disabledDesired.RemoteControl)
	}
	if disabledDesired.CommandPolicy == nil ||
		disabledDesired.CommandPolicy.Status != fleetStateEnabled ||
		disabledDesired.CommandPolicy.Desired != fleetStateDisabled ||
		disabledDesired.CommandPolicy.Applied != fleetStateEnabled ||
		disabledDesired.CommandPolicy.Enforcement != fleetCommandPolicyDrifted {
		t.Fatalf("desired-disabled/applied-enabled command policy = %+v", disabledDesired.CommandPolicy)
	}

	enabledDesired := byID["agent:desired-enabled-applied-disabled"].Fleet
	if enabledDesired.RemoteControl != fleetStateDisabled {
		t.Fatalf("remote control for desired-enabled/applied-disabled = %q, want applied disabled", enabledDesired.RemoteControl)
	}
	if enabledDesired.CommandPolicy == nil ||
		enabledDesired.CommandPolicy.Status != fleetStateDisabled ||
		enabledDesired.CommandPolicy.Desired != fleetStateEnabled ||
		enabledDesired.CommandPolicy.Applied != fleetStateDisabled ||
		enabledDesired.CommandPolicy.Enforcement != fleetCommandPolicyDrifted {
		t.Fatalf("desired-enabled/applied-disabled command policy = %+v", enabledDesired.CommandPolicy)
	}

	unreported := byID["agent:desired-disabled-unreported"].Fleet
	if unreported.RemoteControl != fleetStateUnknown {
		t.Fatalf("remote control for unreported agent = %q, want unknown", unreported.RemoteControl)
	}
	if unreported.CommandPolicy == nil ||
		unreported.CommandPolicy.Status != fleetStateUnknown ||
		unreported.CommandPolicy.Desired != fleetStateDisabled ||
		unreported.CommandPolicy.Applied != fleetStateUnknown ||
		unreported.CommandPolicy.Enforcement != fleetStatePending {
		t.Fatalf("unreported command policy = %+v", unreported.CommandPolicy)
	}
}

func TestBuildConnections_AgentConfigDriftUsesCanonicalDesiredMetadataWithoutSelfComparing(t *testing.T) {
	now := time.Now()
	desiredConfig := desiredAgentConfig(t, ptrBool(true), map[string]interface{}{
		"interval": "10s",
	})
	desired := *desiredConfig.Fingerprint
	in := aggregatorInputs{
		hosts: []models.Host{
			{
				ID:              "agent-1",
				Hostname:        "agent-1",
				LastSeen:        now,
				CommandsEnabled: false,
				DiskExclude:     []string{"/dev/loop*"},
			},
		},
		agentDesiredConfigs: map[string]connectionAgentDesiredConfig{
			"agent-1": desiredConfig,
		},
		now: now,
	}

	got := buildConnections(in)
	if len(got) != 1 {
		t.Fatalf("expected 1 connection, got %d", len(got))
	}
	drift := got[0].Fleet.ConfigDrift
	if drift == nil {
		t.Fatal("expected agent config drift metadata")
	}
	if drift.Status != fleetStatePending {
		t.Fatalf("config drift status = %q, want pending", drift.Status)
	}
	if drift.Desired == nil || *drift.Desired != desired {
		t.Fatalf("desired config drift fingerprint = %+v, want %+v", drift.Desired, desired)
	}
	if drift.Applied != nil {
		t.Fatalf("applied config fingerprint should be absent until agent reports a comparable fingerprint, got %+v", drift.Applied)
	}

	selfCompared := connectionConfigFingerprint(connectionAgentConfigFingerprintVersion, map[string]any{
		"commandsEnabled": false,
		"diskExclude":     []string{"/dev/loop*"},
	})
	if selfCompared == nil {
		t.Fatal("expected local self-comparison fingerprint to be derivable")
	}
	if drift.Desired.Hash == selfCompared.Hash {
		t.Fatalf("desired config hash reused report-field fingerprint %q", drift.Desired.Hash)
	}
	if got[0].Fleet.Rollout == nil || got[0].Fleet.Rollout.Status == fleetStateCurrent {
		t.Fatalf("rollout should not claim current without an applied config comparison, got %+v", got[0].Fleet.Rollout)
	}
}

func TestConnectionAgentDesiredConfigFingerprintsSkipsEmptyDefaultConfig(t *testing.T) {
	monitor, err := monitoring.New(&config.Config{DataPath: t.TempDir()})
	if err != nil {
		t.Fatalf("monitoring.New: %v", err)
	}
	t.Cleanup(func() { monitor.Stop() })

	hostID := "default-agent"
	got := connectionAgentDesiredConfigFingerprints(monitor, []models.Host{{ID: hostID}})
	desired, ok := got[hostID]
	if !ok {
		t.Fatalf("expected resolved desired config entry for %q, got %+v", hostID, got)
	}
	if desired.Fingerprint != nil {
		t.Fatalf("empty default config should not create rollout fingerprint, got %+v", desired.Fingerprint)
	}
	if desired.CommandsEnabled != nil {
		t.Fatalf("empty default config should not set command override, got %+v", desired.CommandsEnabled)
	}

	commandsEnabled := true
	if err := monitor.UpdateHostAgentConfig(hostID, &commandsEnabled); err != nil {
		t.Fatalf("UpdateHostAgentConfig: %v", err)
	}
	got = connectionAgentDesiredConfigFingerprints(monitor, []models.Host{{ID: hostID}})
	desired = got[hostID]
	if desired.Fingerprint == nil {
		t.Fatalf("managed command override should create desired config fingerprint")
	}
	if desired.CommandsEnabled == nil || !*desired.CommandsEnabled {
		t.Fatalf("managed command override = %+v, want true", desired.CommandsEnabled)
	}
}

func TestConnectionFleetAgentConfigDriftComparesAppliedFingerprintsWhenAvailable(t *testing.T) {
	now := time.Now()
	conn := Connection{
		Type:     ConnectionTypeAgent,
		State:    ConnectionStateActive,
		Enabled:  true,
		LastSeen: &now,
	}
	desired := &ConnectionFleetConfigFingerprint{Version: connectionAgentConfigFingerprintVersion, Hash: "sha256:desired"}
	applied := &ConnectionFleetConfigFingerprint{Version: connectionAgentConfigFingerprintVersion, Hash: "sha256:applied"}

	drifted := connectionFleetAgentConfigDriftForFingerprints(conn, desired, applied)
	if drifted.Status != fleetConfigDriftDrifted ||
		drifted.Desired != desired ||
		drifted.Applied != applied ||
		drifted.LastObservedAt == nil {
		t.Fatalf("drifted config comparison = %+v", drifted)
	}

	matchingApplied := &ConnectionFleetConfigFingerprint{Version: desired.Version, Hash: desired.Hash}
	current := connectionFleetAgentConfigDriftForFingerprints(conn, desired, matchingApplied)
	if current.Status != fleetConfigDriftCurrent ||
		current.Desired != desired ||
		current.Applied != matchingApplied ||
		current.LastObservedAt == nil {
		t.Fatalf("current config comparison = %+v", current)
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
	if healthy.ConfigDrift == nil ||
		healthy.ConfigDrift.Status != fleetConfigDriftCurrent ||
		healthy.ConfigDrift.Desired == nil ||
		healthy.ConfigDrift.Applied == nil ||
		healthy.ConfigDrift.Desired.Hash != healthy.ConfigDrift.Applied.Hash {
		t.Fatalf("healthy platform config drift = %+v", healthy.ConfigDrift)
	}
	if healthy.Rollout == nil || healthy.Rollout.Status != fleetStateCurrent {
		t.Fatalf("healthy platform rollout = %+v", healthy.Rollout)
	}
	if healthy.CredentialHealth == nil || healthy.CredentialHealth.Status != fleetStateVerified {
		t.Fatalf("healthy platform credential health = %+v", healthy.CredentialHealth)
	}

	badToken := byID["pve:bad-token"].Fleet
	if badToken.AdapterHealth != fleetStateBlocked ||
		badToken.CredentialStatus != fleetStateInvalid ||
		badToken.LivenessState != string(ConnectionStateUnauthorized) {
		t.Fatalf("bad token fleet governance = %+v", badToken)
	}
	if badToken.CredentialHealth == nil ||
		badToken.CredentialHealth.Status != fleetStateInvalid ||
		badToken.CredentialHealth.LastFailedAt == nil {
		t.Fatalf("bad token credential health = %+v", badToken.CredentialHealth)
	}
	if badToken.Rollout == nil || badToken.Rollout.Status != fleetRolloutBlocked {
		t.Fatalf("bad token rollout = %+v", badToken.Rollout)
	}

	paused := byID["pve:paused"].Fleet
	if paused.EnrollmentState != fleetStatePaused ||
		paused.AdapterHealth != fleetStatePaused ||
		paused.ConfigRollout != fleetStatePaused ||
		paused.CredentialStatus != fleetStatePaused {
		t.Fatalf("paused platform fleet governance = %+v", paused)
	}
	if paused.ConfigDrift == nil || paused.ConfigDrift.Status != fleetStatePaused {
		t.Fatalf("paused config drift = %+v", paused.ConfigDrift)
	}
	if paused.Rollout == nil || paused.Rollout.Status != fleetStatePaused {
		t.Fatalf("paused rollout = %+v", paused.Rollout)
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
