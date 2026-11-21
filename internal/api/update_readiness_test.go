package api

import (
	"context"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/updates"
)

func TestRouterUpdateReadinessConfigSnapshotUsesCanonicalRuntimeTokens(t *testing.T) {
	token, err := config.NewAPITokenRecord(
		"0123456789abcdef0123456789abcdef",
		"agent",
		[]string{config.ScopeAgentReport},
	)
	if err != nil {
		t.Fatalf("NewAPITokenRecord() error = %v", err)
	}

	configHandlers := &ConfigHandlers{}
	configHandlers.SetConfig(&config.Config{})
	r := &Router{
		config:         &config.Config{APITokens: []config.APITokenRecord{*token}},
		configHandlers: configHandlers,
	}

	snapshot := r.updateReadinessConfigSnapshot(context.Background())
	if snapshot == nil || len(snapshot.APITokens) != 1 {
		t.Fatalf("snapshot APITokens = %#v, want canonical runtime token", snapshot)
	}

	r.config.APITokens = nil
	if len(snapshot.APITokens) != 1 {
		t.Fatal("expected update readiness snapshot to be independent of later runtime mutations")
	}
}

func TestBuildUpdateReadiness_ActiveV5AgentWarnsForFirstHopTransport(t *testing.T) {
	now := time.Date(2026, 5, 28, 12, 0, 0, 0, time.UTC)
	record, err := config.NewAPITokenRecord("abcdef1234567890abcdef1234567890", "agent", []string{config.ScopeAgentReport})
	if err != nil {
		t.Fatalf("NewAPITokenRecord: %v", err)
	}

	readiness := buildUpdateReadiness(updateReadinessInputs{
		cfg: &config.Config{APITokens: []config.APITokenRecord{*record}},
		hosts: []models.Host{{
			ID:           "host-1",
			Hostname:     "host-1",
			LastSeen:     now.Add(-30 * time.Second),
			AgentVersion: "5.1.23",
			IsLegacy:     true,
		}},
		targetVersion: "v6.0.0",
		plan: updates.UpdatePlan{
			CanAutoUpdate:   true,
			RollbackSupport: true,
		},
		now: now,
	})

	if readiness.Status != updateReadinessAttention {
		t.Fatalf("readiness status = %q, want %q: %#v", readiness.Status, updateReadinessAttention, readiness)
	}
	if got := readiness.Checks[2].ID; got != "agent-migration-security" {
		t.Fatalf("check[2] id = %q, want agent-migration-security", got)
	}
	if got := readiness.Checks[2].Status; got != updateReadinessCheckWarning {
		t.Fatalf("migration security status = %q, want warning", got)
	}
	if got := readiness.Checks[2].Summary; got != "v5 agents can auto-update to v6, but the first hop depends on trusted transport." {
		t.Fatalf("migration security summary = %q", got)
	}
	wantDetail := "Use HTTPS, or keep the Pulse-to-agent migration path on a trusted local network; v5 checksum validation alone does not protect plain HTTP from an on-path attacker."
	if !containsExactString(readiness.Checks[2].Details, wantDetail) {
		t.Fatalf("migration security details missing %q: %#v", wantDetail, readiness.Checks[2].Details)
	}
}

func TestBuildUpdateReadiness_BlocksWhenAgentsHaveNoReportingToken(t *testing.T) {
	now := time.Date(2026, 5, 28, 12, 0, 0, 0, time.UTC)
	record, err := config.NewAPITokenRecord("abcdef1234567890abcdef1234567890", "settings", []string{config.ScopeSettingsRead})
	if err != nil {
		t.Fatalf("NewAPITokenRecord: %v", err)
	}

	readiness := buildUpdateReadiness(updateReadinessInputs{
		cfg: &config.Config{APITokens: []config.APITokenRecord{*record}},
		hosts: []models.Host{{
			ID:           "host-1",
			Hostname:     "host-1",
			LastSeen:     now.Add(-30 * time.Second),
			AgentVersion: "6.0.0-rc.6",
		}},
		targetVersion: "v6.0.0",
		plan: updates.UpdatePlan{
			CanAutoUpdate:   true,
			RollbackSupport: true,
		},
		now: now,
	})

	if readiness.Status != updateReadinessBlocked {
		t.Fatalf("readiness status = %q, want %q: %#v", readiness.Status, updateReadinessBlocked, readiness)
	}
	if got := readiness.Checks[3].Status; got != updateReadinessCheckBlocked {
		t.Fatalf("agent token check status = %q, want blocked", got)
	}
}

func TestBuildUpdateReadiness_WarnsOnEphemeralDockerDataDir(t *testing.T) {
	now := time.Date(2026, 11, 21, 11, 0, 0, 0, time.UTC)

	readiness := buildUpdateReadiness(updateReadinessInputs{
		cfg:           &config.Config{DataPath: "/data"},
		targetVersion: "v6.10.0",
		plan: updates.UpdatePlan{
			CanAutoUpdate:   false,
			RollbackSupport: true,
		},
		now:                    now,
		dockerDataDirEphemeral: true,
		dataDir:                "/data",
	})

	if readiness.Status != updateReadinessAttention {
		t.Fatalf("readiness status = %q, want %q: %#v", readiness.Status, updateReadinessAttention, readiness)
	}
	if len(readiness.Checks) != 5 {
		t.Fatalf("len(checks) = %d, want 5: %#v", len(readiness.Checks), readiness.Checks)
	}
	check := readiness.Checks[4]
	if check.ID != "container-data-persistence" || check.Status != updateReadinessCheckWarning {
		t.Fatalf("unexpected persistence check: %#v", check)
	}
	wantDetail := "Mount a persistent volume at /data (for example: -v pulse-data:/data) before recreating or updating the container."
	if !containsExactString(check.Details, wantDetail) {
		t.Fatalf("persistence check details missing %q: %#v", wantDetail, check.Details)
	}

	persistent := buildUpdateReadiness(updateReadinessInputs{
		cfg:           &config.Config{DataPath: "/data"},
		targetVersion: "v6.10.0",
		plan: updates.UpdatePlan{
			CanAutoUpdate:   false,
			RollbackSupport: true,
		},
		now:                    now,
		dockerDataDirEphemeral: false,
		dataDir:                "/data",
	})
	if len(persistent.Checks) != 4 {
		t.Fatalf("len(checks) when persistent = %d, want 4", len(persistent.Checks))
	}
}

func containsExactString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func TestBuildUpdateReadiness_WarnsOnStaleAgent(t *testing.T) {
	now := time.Date(2026, 5, 28, 12, 0, 0, 0, time.UTC)
	record, err := config.NewAPITokenRecord("abcdef1234567890abcdef1234567890", "agent", []string{config.ScopeAgentReport})
	if err != nil {
		t.Fatalf("NewAPITokenRecord: %v", err)
	}

	readiness := buildUpdateReadiness(updateReadinessInputs{
		cfg: &config.Config{APITokens: []config.APITokenRecord{*record}},
		hosts: []models.Host{{
			ID:              "host-1",
			Hostname:        "host-1",
			LastSeen:        now.Add(-5*time.Minute - time.Second),
			IntervalSeconds: 30,
			AgentVersion:    "6.0.0-rc.6",
		}},
		targetVersion: "v6.0.0",
		plan: updates.UpdatePlan{
			CanAutoUpdate:   true,
			RollbackSupport: true,
		},
		now: now,
	})

	if readiness.Status != updateReadinessAttention {
		t.Fatalf("readiness status = %q, want %q: %#v", readiness.Status, updateReadinessAttention, readiness)
	}
	if got := readiness.Checks[1].Status; got != updateReadinessCheckWarning {
		t.Fatalf("agent continuity check status = %q, want warning", got)
	}
}
