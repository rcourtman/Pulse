package ai

// Tests for the blocked-run runtime finding and provider-init retry. A Patrol
// that is enabled but cannot use a provider must surface one deduped,
// self-resolving finding on the shared findings surfaces instead of skipping
// runs with no evidence outside the Patrol page: field telemetry showed
// installs recording weeks of empty error runs (provider init failed at boot
// and was never retried) before anyone noticed.

import (
	"context"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func patrolRuntimeFindingIDForTest() string {
	return generateFindingID(patrolRuntimeResourceID, "reliability", patrolRuntimeFindingKey)
}

func blockedRunStateProvider() *mockStateProvider {
	return &mockStateProvider{
		state: models.StateSnapshot{
			Nodes: []models.Node{
				{ID: "node1", Name: "node1", Status: "online"},
			},
		},
	}
}

func TestPatrol_BlockedScheduledRun_RaisesDedupedRuntimeFinding(t *testing.T) {
	ps := NewPatrolService(nil, blockedRunStateProvider())

	notified := 0
	ps.SetFindingNotifyCallback(func(f *Finding) { notified++ })

	ps.runPatrol(context.Background())
	ps.runPatrol(context.Background())

	stored := ps.findings.Get(patrolRuntimeFindingIDForTest())
	if stored == nil {
		t.Fatal("expected blocked scheduled run to raise the runtime finding")
	}
	if stored.IsResolved() {
		t.Fatal("runtime finding should stay active while runs remain blocked")
	}
	if stored.FailureCause != string(PatrolFailureCauseProviderNotConfigured) {
		t.Fatalf("failure cause = %q, want %q", stored.FailureCause, PatrolFailureCauseProviderNotConfigured)
	}
	if notified != 1 {
		t.Fatalf("finding notified %d times across two blocked runs, want exactly once", notified)
	}
}

func TestPatrol_ReadinessBlockedRun_RaisesRuntimeFindingWithoutRunRecord(t *testing.T) {
	ps := NewPatrolService(nil, blockedRunStateProvider())

	cfg := DefaultPatrolConfig()
	cfg.RuntimeBlockedReason = "No AI provider is configured yet. Add a provider API key or an Ollama server on the Provider & Models settings page."
	cfg.RuntimeBlockedCause = PatrolFailureCauseProviderNotConfigured
	ps.SetConfig(cfg)

	ps.runPatrol(context.Background())

	if got := len(ps.GetRunHistory(10)); got != 0 {
		t.Fatalf("readiness-blocked scheduled run recorded %d run(s), want none", got)
	}
	stored := ps.findings.Get(patrolRuntimeFindingIDForTest())
	if stored == nil {
		t.Fatal("expected readiness-blocked run to raise the runtime finding")
	}
	if !strings.Contains(stored.Description, "No AI provider is configured yet") {
		t.Fatalf("finding description %q should carry the readiness reason", stored.Description)
	}
}

func TestPatrol_ProviderInitFailure_UsesHonestBlockedReason(t *testing.T) {
	svc := NewService(nil, nil)
	// Configured (Ollama base URL) but the explicit model routes to a provider
	// with no credentials, so provider init fails deterministically offline.
	svc.cfg = &config.AIConfig{
		Enabled:       true,
		OllamaBaseURL: "http://127.0.0.1:1",
		Model:         "anthropic:claude-model",
	}

	ps := NewPatrolService(svc, blockedRunStateProvider())
	ps.runPatrol(context.Background())

	status := ps.GetStatus()
	if !strings.Contains(status.BlockedReason, "failed to initialize") {
		t.Fatalf("blocked reason %q should name the provider init failure, not claim no provider is configured", status.BlockedReason)
	}
	if status.BlockedCause != PatrolFailureCauseProviderConnection {
		t.Fatalf("blocked cause = %q, want %q", status.BlockedCause, PatrolFailureCauseProviderConnection)
	}
	stored := ps.findings.Get(patrolRuntimeFindingIDForTest())
	if stored == nil {
		t.Fatal("expected provider init failure to raise the runtime finding")
	}
	if !strings.Contains(stored.Description, "failed to initialize") {
		t.Fatalf("finding description %q should name the provider init failure", stored.Description)
	}
}

func TestService_RetryProviderInit(t *testing.T) {
	ctx := context.Background()

	unconfigured := NewService(nil, nil)
	unconfigured.cfg = &config.AIConfig{}
	if unconfigured.RetryProviderInit(ctx) {
		t.Fatal("retry must fail while the config has no enabled, configured provider")
	}

	// A keyless OpenAI-compatible custom endpoint constructs offline.
	recoverable := NewService(nil, nil)
	recoverable.cfg = &config.AIConfig{
		Enabled:       true,
		OpenAIBaseURL: "http://127.0.0.1:9/v1",
		Model:         "openai:test-model",
	}
	if !recoverable.RetryProviderInit(ctx) {
		t.Fatal("retry should build a provider for a constructible config")
	}
	if !recoverable.IsEnabled() {
		t.Fatal("service should report enabled after a successful retry")
	}
	if got := recoverable.ProviderInitError(); got != "" {
		t.Fatalf("provider init error = %q after successful retry, want empty", got)
	}

	failing := NewService(nil, nil)
	failing.cfg = &config.AIConfig{
		Enabled:       true,
		OllamaBaseURL: "http://127.0.0.1:1",
		Model:         "anthropic:claude-model",
	}
	if failing.RetryProviderInit(ctx) {
		t.Fatal("retry must fail when the selected model's provider has no credentials")
	}
	if failing.ProviderInitError() == "" {
		t.Fatal("failed retry should record the provider init error")
	}
}

func TestPatrol_DisablingPatrolResolvesRuntimeFinding(t *testing.T) {
	ps := NewPatrolService(nil, blockedRunStateProvider())

	ps.runPatrol(context.Background())
	if stored := ps.findings.Get(patrolRuntimeFindingIDForTest()); stored == nil || stored.IsResolved() {
		t.Fatal("expected an active runtime finding before disabling Patrol")
	}

	cfg := DefaultPatrolConfig()
	cfg.Enabled = false
	ps.SetConfig(cfg)

	stored := ps.findings.Get(patrolRuntimeFindingIDForTest())
	if stored == nil || !stored.IsResolved() {
		t.Fatal("disabling Patrol should resolve the runtime finding instead of leaving it to nag")
	}
}
