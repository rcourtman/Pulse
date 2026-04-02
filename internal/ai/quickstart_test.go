package ai

import (
	"context"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
)

type stubQuickstartBootstrapClient struct {
	calls  int
	bearer string
	req    pkglicensing.QuickstartBootstrapRequest
	resp   *pkglicensing.QuickstartBootstrapResponse
	err    error
}

func (s *stubQuickstartBootstrapClient) BootstrapQuickstart(ctx context.Context, bearerToken string, req pkglicensing.QuickstartBootstrapRequest) (*pkglicensing.QuickstartBootstrapResponse, error) {
	s.calls++
	s.bearer = bearerToken
	s.req = req
	if s.err != nil {
		return nil, s.err
	}
	return s.resp, nil
}

func TestPersistentQuickstartCreditManager_EnsureBootstrapUsesActivationIdentity(t *testing.T) {
	dir := t.TempDir()
	persistence := config.NewConfigPersistence(dir)

	licensePersistence, err := pkglicensing.NewPersistence(persistence.GetConfigDir())
	if err != nil {
		t.Fatalf("NewPersistence(): %v", err)
	}
	if err := licensePersistence.SaveActivationState(&pkglicensing.ActivationState{
		InstallationID:      "inst_live_test",
		InstallationToken:   "pit_live_test",
		InstanceFingerprint: "fp-live-test",
	}); err != nil {
		t.Fatalf("SaveActivationState(): %v", err)
	}

	client := &stubQuickstartBootstrapClient{
		resp: &pkglicensing.QuickstartBootstrapResponse{
			QuickstartToken:          "qst_live_activation",
			QuickstartTokenExpiresAt: time.Now().Add(30 * time.Minute).UTC().Format(time.RFC3339),
			CreditsRemaining:         19,
			CreditsTotal:             25,
		},
	}
	mgr := NewPersistentQuickstartCreditManagerWithClient(persistence, "default", func() *config.AIConfig { return &config.AIConfig{Enabled: true} }, client)
	mgr.hostname = func() (string, error) { return "pulse-test", nil }

	if err := mgr.EnsureBootstrap(context.Background()); err != nil {
		t.Fatalf("EnsureBootstrap(): %v", err)
	}
	if client.calls != 1 {
		t.Fatalf("BootstrapQuickstart calls = %d, want 1", client.calls)
	}
	if client.bearer != "pit_live_test" {
		t.Fatalf("bearer = %q, want pit_live_test", client.bearer)
	}
	if client.req.ClientInstallationID != "" {
		t.Fatalf("ClientInstallationID = %q, want empty", client.req.ClientInstallationID)
	}
	if client.req.InstanceFingerprint != "fp-live-test" {
		t.Fatalf("InstanceFingerprint = %q, want fp-live-test", client.req.InstanceFingerprint)
	}
	if mgr.CreditsRemaining() != 19 {
		t.Fatalf("CreditsRemaining() = %d, want 19", mgr.CreditsRemaining())
	}
	if mgr.CreditsTotal() != 25 {
		t.Fatalf("CreditsTotal() = %d, want 25", mgr.CreditsTotal())
	}
	if mgr.GetProvider() == nil {
		t.Fatal("expected quickstart provider after activation bootstrap")
	}
}

func TestPersistentQuickstartCreditManager_EnsureBootstrapPersistsCommunityInstallationID(t *testing.T) {
	dir := t.TempDir()
	persistence := config.NewConfigPersistence(dir)

	client := &stubQuickstartBootstrapClient{
		resp: &pkglicensing.QuickstartBootstrapResponse{
			QuickstartToken:          "qst_live_community",
			QuickstartTokenExpiresAt: time.Now().Add(30 * time.Minute).UTC().Format(time.RFC3339),
			CreditsRemaining:         17,
			CreditsTotal:             25,
		},
	}
	mgr := NewPersistentQuickstartCreditManagerWithClient(persistence, "default", func() *config.AIConfig { return &config.AIConfig{Enabled: true} }, client)
	mgr.hostname = func() (string, error) { return "pulse-community", nil }
	mgr.newID = func() string { return "community-install-1" }

	if err := mgr.EnsureBootstrap(context.Background()); err != nil {
		t.Fatalf("EnsureBootstrap(): %v", err)
	}
	if client.calls != 1 {
		t.Fatalf("BootstrapQuickstart calls = %d, want 1", client.calls)
	}
	if client.bearer != "" {
		t.Fatalf("bearer = %q, want empty", client.bearer)
	}
	if client.req.ClientInstallationID != "community-install-1" {
		t.Fatalf("ClientInstallationID = %q, want community-install-1", client.req.ClientInstallationID)
	}
	if client.req.InstanceFingerprint != "community-install-1" {
		t.Fatalf("InstanceFingerprint = %q, want community-install-1", client.req.InstanceFingerprint)
	}

	loaded, err := persistence.LoadQuickstartState()
	if err != nil {
		t.Fatalf("LoadQuickstartState(): %v", err)
	}
	if loaded.ClientInstallationID != "community-install-1" {
		t.Fatalf("persisted ClientInstallationID = %q, want community-install-1", loaded.ClientInstallationID)
	}
	if loaded.QuickstartToken != "qst_live_community" {
		t.Fatalf("persisted QuickstartToken = %q, want qst_live_community", loaded.QuickstartToken)
	}
}

func TestPersistentQuickstartCreditManager_LoadsPersistedTokenWithoutBootstrap(t *testing.T) {
	dir := t.TempDir()
	persistence := config.NewConfigPersistence(dir)
	expiresAt := time.Now().Add(30 * time.Minute).UTC().Unix()
	lastSyncedAt := time.Now().UTC().Unix()
	if err := persistence.SaveQuickstartState(config.QuickstartState{
		ClientInstallationID:       "community-install-1",
		QuickstartToken:            "qst_live_cached",
		QuickstartTokenExpiresAt:   &expiresAt,
		QuickstartCreditsRemaining: 11,
		QuickstartCreditsTotal:     25,
		LastSyncedAt:               &lastSyncedAt,
	}); err != nil {
		t.Fatalf("SaveQuickstartState(): %v", err)
	}

	client := &stubQuickstartBootstrapClient{
		resp: &pkglicensing.QuickstartBootstrapResponse{
			QuickstartToken:  "qst_should_not_be_used",
			CreditsRemaining: 1,
			CreditsTotal:     25,
		},
	}
	mgr := NewPersistentQuickstartCreditManagerWithClient(persistence, "default", func() *config.AIConfig { return &config.AIConfig{Enabled: true} }, client)

	if err := mgr.EnsureBootstrap(context.Background()); err != nil {
		t.Fatalf("EnsureBootstrap(): %v", err)
	}
	if client.calls != 0 {
		t.Fatalf("BootstrapQuickstart calls = %d, want 0 for fresh cached state", client.calls)
	}
	if mgr.CreditsRemaining() != 11 {
		t.Fatalf("CreditsRemaining() = %d, want 11", mgr.CreditsRemaining())
	}
	if mgr.GetProvider() == nil {
		t.Fatal("expected provider from cached quickstart token")
	}
}
