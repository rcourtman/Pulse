package api

import (
	"reflect"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
)

// This file adds branch coverage for buildPMGConnection
// (internal/api/connections_aggregator.go), which previously had 0.0% test
// coverage. Every top-level function is prefixed with TestBranchcov0722 so the
// run can be scoped with -run "^TestBranchcov0722".
//
// buildPMGConnection derives a Connection row from a config.PMGInstance plus a
// health map keyed "pmg::<name>". The arms exercised below are:
//   - the Disabled -> Enabled flip,
//   - each per-surface monitor scope flag (mailStats, queues, quarantine,
//     domainStats) toggled on and off,
//   - a health map that contains an entry for this instance and one that does
//     not,
//   - the status/last-seen derivation off a frozen `now` for every state arm
//     of deriveConnectionState reachable through the builder (paused, pending,
//     unauthorized, unreachable via first failure, unreachable via breaker,
//     stale, active),
//   - the Proxmox credential-kind derivation (token / password / unknown /
//     whitespace) surfaced through Fleet.CredentialHealth,
//   - the static identity fields (id, type, address, host aliases, source,
//     surfaces, capabilities).

// frozenNow is the fixed reference time every subtest derives time-derived
// fields against, so LastSeen / stale reasons / credential timestamps are
// reproducible. It is UTC to avoid host-timezone bleed into formatted reasons.
var frozenNow = time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC)

// pmgHealthEntry builds an InstanceHealth with deterministic timestamps. The
// shared healthEntry helper stamps errors with time.Now(), which would make
// LastError.At (and the derived LastFailedAt) non-deterministic; this variant
// takes an explicit error timestamp so derived values can be asserted exactly.
func pmgHealthEntry(lastSuccess *time.Time, errAt *time.Time, errMessage, errCategory, breakerState string) monitoring.InstanceHealth {
	ps := monitoring.InstancePollStatus{LastSuccess: lastSuccess}
	if errMessage != "" {
		detail := &monitoring.ErrorDetail{Message: errMessage, Category: errCategory}
		if errAt != nil {
			detail.At = *errAt
		}
		ps.LastError = detail
	}
	return monitoring.InstanceHealth{
		PollStatus: ps,
		Breaker:    monitoring.InstanceBreaker{State: breakerState},
	}
}

// wantPMGSurfaces is the always-on surface list the builder emits regardless of
// the per-surface scope flags; asserting it stays constant distinguishes
// "surfaces" (what can be collected) from "scope" (what is enabled).
var wantPMGSurfaces = []string{"mailStats", "queues", "quarantine", "domainStats"}

// TestBranchcov0722PMGConnectionDisabledFlip exercises the !inst.Disabled ->
// Enabled flip and the fleet-governance consequences of pausing a PMG source.
func TestBranchcov0722PMGConnectionDisabledFlip(t *testing.T) {
	t.Run("disabled_instance_is_paused_and_not_enabled", func(t *testing.T) {
		inst := config.PMGInstance{Name: "mail-relay", Host: "mail.lan", Disabled: true}
		conn := buildPMGConnection(inst, nil, frozenNow)

		if conn.Enabled {
			t.Fatalf("Enabled = true, want false for Disabled instance")
		}
		if conn.State != ConnectionStatePaused {
			t.Fatalf("State = %q, want %q", conn.State, ConnectionStatePaused)
		}
		if conn.StateReason != "paused by user" {
			t.Fatalf("StateReason = %q, want %q", conn.StateReason, "paused by user")
		}
		fleet := conn.Fleet
		if fleet.EnrollmentState != fleetStatePaused {
			t.Fatalf("EnrollmentState = %q, want %q", fleet.EnrollmentState, fleetStatePaused)
		}
		if fleet.LivenessState != string(ConnectionStatePaused) {
			t.Fatalf("LivenessState = %q, want %q", fleet.LivenessState, ConnectionStatePaused)
		}
		if fleet.AdapterHealth != fleetStatePaused {
			t.Fatalf("AdapterHealth = %q, want %q", fleet.AdapterHealth, fleetStatePaused)
		}
		if fleet.ConfigRollout != fleetStatePaused {
			t.Fatalf("ConfigRollout = %q, want %q", fleet.ConfigRollout, fleetStatePaused)
		}
		if fleet.CredentialStatus != fleetStatePaused {
			t.Fatalf("CredentialStatus = %q, want %q", fleet.CredentialStatus, fleetStatePaused)
		}
		if fleet.ConfigDrift == nil || fleet.ConfigDrift.Status != fleetStatePaused {
			t.Fatalf("ConfigDrift = %+v, want paused", fleet.ConfigDrift)
		}
		if fleet.Rollout == nil || fleet.Rollout.Status != fleetStatePaused || fleet.Rollout.Stage != fleetRolloutStagePaused {
			t.Fatalf("Rollout = %+v, want status=%q stage=%q", fleet.Rollout, fleetStatePaused, fleetRolloutStagePaused)
		}
		if fleet.CredentialHealth == nil || fleet.CredentialHealth.Status != fleetStatePaused {
			t.Fatalf("CredentialHealth = %+v, want status %q", fleet.CredentialHealth, fleetStatePaused)
		}
	})

	t.Run("enabled_instance_is_not_paused", func(t *testing.T) {
		inst := config.PMGInstance{Name: "mail-relay", Host: "mail.lan"}
		conn := buildPMGConnection(inst, nil, frozenNow)

		if !conn.Enabled {
			t.Fatalf("Enabled = false, want true for non-Disabled instance")
		}
		// No health entry yet -> pending, not paused.
		if conn.State != ConnectionStatePending {
			t.Fatalf("State = %q, want %q", conn.State, ConnectionStatePending)
		}
		if conn.Fleet.EnrollmentState != fleetStateConfigured {
			t.Fatalf("EnrollmentState = %q, want %q", conn.Fleet.EnrollmentState, fleetStateConfigured)
		}
	})
}

// TestBranchcov0722PMGConnectionScopeFlags exercises each per-surface monitor
// scope flag toggled on and off. The surface list must stay constant; only the
// scope map values follow the Monitor* flags.
func TestBranchcov0722PMGConnectionScopeFlags(t *testing.T) {
	cases := []struct {
		name   string
		inst   config.PMGInstance
		wantOn map[string]bool
	}{
		{
			name: "all_flags_off",
			inst: config.PMGInstance{Name: "m", Host: "h.lan"},
			wantOn: map[string]bool{
				"mailStats": false, "queues": false, "quarantine": false, "domainStats": false,
			},
		},
		{
			name: "all_flags_on",
			inst: config.PMGInstance{
				Name: "m", Host: "h.lan",
				MonitorMailStats: true, MonitorQueues: true, MonitorQuarantine: true, MonitorDomainStats: true,
			},
			wantOn: map[string]bool{
				"mailStats": true, "queues": true, "quarantine": true, "domainStats": true,
			},
		},
		{
			name: "only_mail_stats",
			inst: config.PMGInstance{Name: "m", Host: "h.lan", MonitorMailStats: true},
			wantOn: map[string]bool{
				"mailStats": true, "queues": false, "quarantine": false, "domainStats": false,
			},
		},
		{
			name: "only_queues",
			inst: config.PMGInstance{Name: "m", Host: "h.lan", MonitorQueues: true},
			wantOn: map[string]bool{
				"mailStats": false, "queues": true, "quarantine": false, "domainStats": false,
			},
		},
		{
			name: "only_quarantine",
			inst: config.PMGInstance{Name: "m", Host: "h.lan", MonitorQuarantine: true},
			wantOn: map[string]bool{
				"mailStats": false, "queues": false, "quarantine": true, "domainStats": false,
			},
		},
		{
			name: "only_domain_stats",
			inst: config.PMGInstance{Name: "m", Host: "h.lan", MonitorDomainStats: true},
			wantOn: map[string]bool{
				"mailStats": false, "queues": false, "quarantine": false, "domainStats": true,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			conn := buildPMGConnection(tc.inst, nil, frozenNow)

			// Surfaces are always the full, ordered list regardless of scope.
			if !reflect.DeepEqual(conn.Surfaces, wantPMGSurfaces) {
				t.Fatalf("Surfaces = %+v, want %+v", conn.Surfaces, wantPMGSurfaces)
			}
			// The scope map must contain exactly the four PMG surfaces.
			if len(conn.Scope) != len(wantPMGSurfaces) {
				t.Fatalf("Scope has %d entries, want %d (%+v)", len(conn.Scope), len(wantPMGSurfaces), conn.Scope)
			}
			for _, surface := range wantPMGSurfaces {
				got, ok := conn.Scope[surface]
				if !ok {
					t.Fatalf("Scope missing required surface %q (got %+v)", surface, conn.Scope)
				}
				if got != tc.wantOn[surface] {
					t.Fatalf("Scope[%q] = %v, want %v", surface, got, tc.wantOn[surface])
				}
			}
			// Scope is the only field these flags influence for PMG.
			if !reflect.DeepEqual(conn.Scope, tc.wantOn) {
				t.Fatalf("Scope = %+v, want %+v", conn.Scope, tc.wantOn)
			}
		})
	}
}

// TestBranchcov0722PMGConnectionHealthLookup exercises the "pmg::<name>" health
// map lookup: an entry for this instance drives state/last-seen, while a missing
// entry (or an entry keyed for a different instance) leaves the connection
// pending with no last-seen.
func TestBranchcov0722PMGConnectionHealthLookup(t *testing.T) {
	inst := config.PMGInstance{Name: "mail-relay", Host: "mail.lan"}
	recentSuccess := frozenNow.Add(-30 * time.Second)

	t.Run("health_entry_present_drives_active_state", func(t *testing.T) {
		health := map[string]monitoring.InstanceHealth{
			"pmg::mail-relay": pmgHealthEntry(&recentSuccess, nil, "", "", "closed"),
		}
		conn := buildPMGConnection(inst, health, frozenNow)
		if conn.State != ConnectionStateActive {
			t.Fatalf("State = %q, want %q", conn.State, ConnectionStateActive)
		}
		if conn.LastSeen == nil || !conn.LastSeen.Equal(recentSuccess) {
			t.Fatalf("LastSeen = %+v, want %s", conn.LastSeen, recentSuccess)
		}
		if conn.LastError != nil {
			t.Fatalf("LastError = %+v, want nil for healthy entry", conn.LastError)
		}
	})

	t.Run("health_entry_missing_leaves_pending", func(t *testing.T) {
		// Empty map, nil map, and a wrong-key map must all behave the same:
		// no health resolved -> awaiting first poll.
		for label, health := range map[string]map[string]monitoring.InstanceHealth{
			"nil_map":   nil,
			"empty_map": {},
			"wrong_key": {"pmg::other-relay": pmgHealthEntry(&recentSuccess, nil, "", "", "closed")},
		} {
			conn := buildPMGConnection(inst, health, frozenNow)
			if conn.State != ConnectionStatePending {
				t.Fatalf("%s: State = %q, want %q", label, conn.State, ConnectionStatePending)
			}
			if conn.StateReason != "awaiting first poll" {
				t.Fatalf("%s: StateReason = %q, want %q", label, conn.StateReason, "awaiting first poll")
			}
			if conn.LastSeen != nil {
				t.Fatalf("%s: LastSeen = %+v, want nil", label, conn.LastSeen)
			}
			if conn.LastError != nil {
				t.Fatalf("%s: LastError = %+v, want nil", label, conn.LastError)
			}
		}
	})
}

// TestBranchcov0722PMGConnectionStateDerivation drives every state arm of
// deriveConnectionState that is reachable through buildPMGConnection, asserting
// the exact state, reason, last-seen and last-error derived from the frozen
// `now`. Health is always keyed under "pmg::mail-relay".
func TestBranchcov0722PMGConnectionStateDerivation(t *testing.T) {
	name := "mail-relay"
	healthKey := "pmg::" + name
	base := config.PMGInstance{Name: name, Host: "mail.lan"}

	t.Run("paused_when_disabled", func(t *testing.T) {
		// Disabled short-circuits before health is consulted; a present error
		// must not promote a disabled source out of "paused".
		errAt := frozenNow.Add(-1 * time.Minute)
		inst := base
		inst.Disabled = true
		health := map[string]monitoring.InstanceHealth{
			healthKey: pmgHealthEntry(ptrTime(frozenNow.Add(-30*time.Second)), &errAt, "401 Unauthorized", "auth", "closed"),
		}
		conn := buildPMGConnection(inst, health, frozenNow)
		if conn.State != ConnectionStatePaused {
			t.Fatalf("State = %q, want %q", conn.State, ConnectionStatePaused)
		}
		if conn.StateReason != "paused by user" {
			t.Fatalf("StateReason = %q, want %q", conn.StateReason, "paused by user")
		}
	})

	t.Run("pending_with_no_poll_history", func(t *testing.T) {
		conn := buildPMGConnection(base, map[string]monitoring.InstanceHealth{
			healthKey: pmgHealthEntry(nil, nil, "", "", "closed"),
		}, frozenNow)
		if conn.State != ConnectionStatePending {
			t.Fatalf("State = %q, want %q", conn.State, ConnectionStatePending)
		}
		if conn.StateReason != "awaiting first poll" {
			t.Fatalf("StateReason = %q, want %q", conn.StateReason, "awaiting first poll")
		}
		if conn.LastSeen != nil || conn.LastError != nil {
			t.Fatalf("LastSeen/LastError = %+v/%+v, want nil/nil", conn.LastSeen, conn.LastError)
		}
	})

	t.Run("unauthorized_on_auth_error", func(t *testing.T) {
		errAt := frozenNow.Add(-2 * time.Minute)
		health := map[string]monitoring.InstanceHealth{
			healthKey: pmgHealthEntry(nil, &errAt, "403 Forbidden: token lacks scope", "auth", "closed"),
		}
		conn := buildPMGConnection(base, health, frozenNow)
		if conn.State != ConnectionStateUnauthorized {
			t.Fatalf("State = %q, want %q", conn.State, ConnectionStateUnauthorized)
		}
		if conn.StateReason != "403 Forbidden: token lacks scope" {
			t.Fatalf("StateReason = %q, want the auth error message", conn.StateReason)
		}
		if conn.LastSeen != nil {
			t.Fatalf("LastSeen = %+v, want nil when no successful poll recorded", conn.LastSeen)
		}
		if conn.LastError == nil ||
			conn.LastError.Message != "403 Forbidden: token lacks scope" ||
			conn.LastError.Category != "auth" ||
			!conn.LastError.At.Equal(errAt) {
			t.Fatalf("LastError = %+v, want message/category/At derived from health", conn.LastError)
		}
		// Fleet credential health records the failure timestamp for invalid creds.
		if conn.Fleet.CredentialHealth == nil ||
			conn.Fleet.CredentialHealth.LastFailedAt == nil ||
			!conn.Fleet.CredentialHealth.LastFailedAt.Equal(errAt) {
			t.Fatalf("CredentialHealth.LastFailedAt = %+v, want %s", conn.Fleet.CredentialHealth, errAt)
		}
	})

	t.Run("unreachable_on_first_failure_without_success", func(t *testing.T) {
		errAt := frozenNow.Add(-90 * time.Second)
		health := map[string]monitoring.InstanceHealth{
			healthKey: pmgHealthEntry(nil, &errAt, "connection refused", "network", "closed"),
		}
		conn := buildPMGConnection(base, health, frozenNow)
		if conn.State != ConnectionStateUnreachable {
			t.Fatalf("State = %q, want %q", conn.State, ConnectionStateUnreachable)
		}
		if conn.StateReason != "connection refused" {
			t.Fatalf("StateReason = %q, want %q", conn.StateReason, "connection refused")
		}
		if conn.LastError == nil || conn.LastError.Message != "connection refused" {
			t.Fatalf("LastError = %+v, want connection refused", conn.LastError)
		}
	})

	t.Run("unreachable_when_breaker_open", func(t *testing.T) {
		// A prior success exists (so it is not the first-failure arm) but the
		// breaker has tripped open; no error message means the generic reason.
		success := frozenNow.Add(-30 * time.Second)
		health := map[string]monitoring.InstanceHealth{
			healthKey: pmgHealthEntry(&success, nil, "", "", "open"),
		}
		conn := buildPMGConnection(base, health, frozenNow)
		if conn.State != ConnectionStateUnreachable {
			t.Fatalf("State = %q, want %q", conn.State, ConnectionStateUnreachable)
		}
		if conn.StateReason != "circuit breaker open" {
			t.Fatalf("StateReason = %q, want %q", conn.StateReason, "circuit breaker open")
		}
		if conn.LastSeen == nil || !conn.LastSeen.Equal(success) {
			t.Fatalf("LastSeen = %+v, want %s", conn.LastSeen, success)
		}
	})

	t.Run("stale_when_last_success_beyond_threshold", func(t *testing.T) {
		// connectionStaleThreshold is 2 minutes; 5 minutes ago must be stale.
		staleSuccess := frozenNow.Add(-5 * time.Minute)
		health := map[string]monitoring.InstanceHealth{
			healthKey: pmgHealthEntry(&staleSuccess, nil, "", "", "closed"),
		}
		conn := buildPMGConnection(base, health, frozenNow)
		if conn.State != ConnectionStateStale {
			t.Fatalf("State = %q, want %q", conn.State, ConnectionStateStale)
		}
		wantReason := "no successful poll in 5m0s"
		if conn.StateReason != wantReason {
			t.Fatalf("StateReason = %q, want %q (derived from frozen now)", conn.StateReason, wantReason)
		}
		if conn.LastSeen == nil || !conn.LastSeen.Equal(staleSuccess) {
			t.Fatalf("LastSeen = %+v, want %s", conn.LastSeen, staleSuccess)
		}
	})

	t.Run("active_when_recent_success", func(t *testing.T) {
		recentSuccess := frozenNow.Add(-30 * time.Second)
		health := map[string]monitoring.InstanceHealth{
			healthKey: pmgHealthEntry(&recentSuccess, nil, "", "", "closed"),
		}
		conn := buildPMGConnection(base, health, frozenNow)
		if conn.State != ConnectionStateActive {
			t.Fatalf("State = %q, want %q", conn.State, ConnectionStateActive)
		}
		if conn.StateReason != "" {
			t.Fatalf("StateReason = %q, want empty for active connection", conn.StateReason)
		}
		if conn.LastSeen == nil || !conn.LastSeen.Equal(recentSuccess) {
			t.Fatalf("LastSeen = %+v, want %s", conn.LastSeen, recentSuccess)
		}
		// Active platform connections record the last successful poll as the
		// credential last-verified timestamp.
		if conn.Fleet.CredentialHealth == nil ||
			conn.Fleet.CredentialHealth.LastVerifiedAt == nil ||
			!conn.Fleet.CredentialHealth.LastVerifiedAt.Equal(recentSuccess) {
			t.Fatalf("CredentialHealth.LastVerifiedAt = %+v, want %s", conn.Fleet.CredentialHealth, recentSuccess)
		}
	})
}

// TestBranchcov0722PMGConnectionCredentialKind exercises the
// connectionProxmoxCredentialKind derivation surfaced through
// Fleet.CredentialHealth.Kind: token credentials win over password, password
// wins over none, whitespace-only credentials count as none, and the kind does
// not change the rotation status when there is no expiry.
func TestBranchcov0722PMGConnectionCredentialKind(t *testing.T) {
	cases := []struct {
		name     string
		inst     config.PMGInstance
		wantKind string
	}{
		{name: "token_value", inst: config.PMGInstance{Name: "m", Host: "h.lan", TokenValue: "secret"}, wantKind: fleetCredentialKindToken},
		{name: "token_name", inst: config.PMGInstance{Name: "m", Host: "h.lan", TokenName: "api"}, wantKind: fleetCredentialKindToken},
		{name: "token_over_password", inst: config.PMGInstance{Name: "m", Host: "h.lan", User: "root@pam", Password: "hunter2", TokenName: "api"}, wantKind: fleetCredentialKindToken},
		{name: "password_user", inst: config.PMGInstance{Name: "m", Host: "h.lan", User: "root@pam"}, wantKind: fleetCredentialKindPassword},
		{name: "password_value", inst: config.PMGInstance{Name: "m", Host: "h.lan", Password: "hunter2"}, wantKind: fleetCredentialKindPassword},
		{name: "no_credentials", inst: config.PMGInstance{Name: "m", Host: "h.lan"}, wantKind: fleetStateUnknown},
		{name: "whitespace_only_token", inst: config.PMGInstance{Name: "m", Host: "h.lan", TokenName: "   ", TokenValue: "\t"}, wantKind: fleetStateUnknown},
		{name: "whitespace_only_password", inst: config.PMGInstance{Name: "m", Host: "h.lan", User: " ", Password: " "}, wantKind: fleetStateUnknown},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			conn := buildPMGConnection(tc.inst, nil, frozenNow)
			ch := conn.Fleet.CredentialHealth
			if ch == nil {
				t.Fatalf("CredentialHealth is nil")
			}
			if ch.Kind != tc.wantKind {
				t.Fatalf("Kind = %q, want %q", ch.Kind, tc.wantKind)
			}
			// No expiry is ever supplied for PMG credentials, so rotation stays
			// healthy regardless of the derived kind.
			if ch.Rotation != fleetCredentialRotationHealthy {
				t.Fatalf("Rotation = %q, want %q (no expiry supplied)", ch.Rotation, fleetCredentialRotationHealthy)
			}
			if ch.ExpiresAt != nil {
				t.Fatalf("ExpiresAt = %+v, want nil", ch.ExpiresAt)
			}
		})
	}
}

// TestBranchcov0722PMGConnectionStaticFields asserts the identity fields the
// builder hard-codes for PMG sources, independent of health/scope: the id
// prefix, the type, the passthrough name/address, the normalised host aliases,
// the manual source, the always-on capabilities, and that the fleet governance
// for a non-agent platform type marks agent-only dimensions not-applicable.
func TestBranchcov0722PMGConnectionStaticFields(t *testing.T) {
	inst := config.PMGInstance{
		Name: "mail-relay",
		Host: "https://mail.lan:8006",
	}
	conn := buildPMGConnection(inst, nil, frozenNow)

	if conn.ID != "pmg:mail-relay" {
		t.Fatalf("ID = %q, want %q", conn.ID, "pmg:mail-relay")
	}
	if conn.Type != ConnectionTypePMG {
		t.Fatalf("Type = %q, want %q", conn.Type, ConnectionTypePMG)
	}
	if conn.Name != "mail-relay" {
		t.Fatalf("Name = %q, want %q", conn.Name, "mail-relay")
	}
	// Address is the raw Host, not URL-normalised.
	if conn.Address != "https://mail.lan:8006" {
		t.Fatalf("Address = %q, want the raw Host", conn.Address)
	}
	// Host aliases normalise the name (lowercased) and the URL (hostname only).
	wantAliases := []string{"mail-relay", "mail.lan"}
	if !reflect.DeepEqual(conn.HostAliases, wantAliases) {
		t.Fatalf("HostAliases = %+v, want %+v", conn.HostAliases, wantAliases)
	}
	if conn.Source != ConnectionSourceManual {
		t.Fatalf("Source = %q, want %q (PMG source is always manual)", conn.Source, ConnectionSourceManual)
	}
	if !reflect.DeepEqual(conn.Surfaces, wantPMGSurfaces) {
		t.Fatalf("Surfaces = %+v, want %+v", conn.Surfaces, wantPMGSurfaces)
	}
	if !conn.Capabilities.SupportsPause || !conn.Capabilities.SupportsScope || !conn.Capabilities.SupportsTest {
		t.Fatalf("Capabilities = %+v, want all three supports flags true", conn.Capabilities)
	}

	// Agent-only fleet dimensions are not applicable to a platform source.
	fleet := conn.Fleet
	if fleet.VersionDrift != fleetStateNotApplicable {
		t.Fatalf("VersionDrift = %q, want %q", fleet.VersionDrift, fleetStateNotApplicable)
	}
	if fleet.UpdateStatus != fleetStateNotApplicable {
		t.Fatalf("UpdateStatus = %q, want %q", fleet.UpdateStatus, fleetStateNotApplicable)
	}
	if fleet.RemoteControl != fleetStateNotApplicable {
		t.Fatalf("RemoteControl = %q, want %q", fleet.RemoteControl, fleetStateNotApplicable)
	}
	if fleet.CommandPolicy == nil ||
		fleet.CommandPolicy.Status != fleetStateNotApplicable ||
		fleet.CommandPolicy.Desired != fleetCommandPolicyNotApplicable ||
		fleet.CommandPolicy.Applied != fleetCommandPolicyNotApplicable ||
		fleet.CommandPolicy.Enforcement != fleetCommandPolicyNotApplicable {
		t.Fatalf("CommandPolicy = %+v, want all dimensions not-applicable", fleet.CommandPolicy)
	}
}
