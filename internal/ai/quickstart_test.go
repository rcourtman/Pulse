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

func TestPersistentQuickstartCreditManager_EnsureBootstrapRequiresInstallationToken(t *testing.T) {
	dir := t.TempDir()
	persistence := config.NewConfigPersistence(dir)

	client := &stubQuickstartBootstrapClient{
		resp: &pkglicensing.QuickstartBootstrapResponse{
			QuickstartToken:  "qst_should_not_be_used",
			CreditsRemaining: 17,
			CreditsTotal:     25,
		},
	}
	mgr := NewPersistentQuickstartCreditManagerWithClient(persistence, "default", func() *config.AIConfig { return &config.AIConfig{Enabled: true} }, client)

	err := mgr.EnsureBootstrap(context.Background())
	if err == nil {
		t.Fatal("expected activation-required error")
	}
	if QuickstartBlockedReasonForError(err) != patrolQuickstartActivationRequiredReason {
		t.Fatalf("blocked reason = %q, want %q", QuickstartBlockedReasonForError(err), patrolQuickstartActivationRequiredReason)
	}
	if client.calls != 0 {
		t.Fatalf("BootstrapQuickstart calls = %d, want 0", client.calls)
	}
	if mgr.CreditsRemaining() != 0 || mgr.CreditsTotal() != 0 {
		t.Fatalf("credits = %d/%d, want 0/0", mgr.CreditsRemaining(), mgr.CreditsTotal())
	}
}

func TestPersistentQuickstartCreditManager_LoadsPersistedTokenWithoutBootstrap(t *testing.T) {
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

	expiresAt := time.Now().Add(30 * time.Minute).UTC().Unix()
	lastSyncedAt := time.Now().UTC().Unix()
	if err := persistence.SaveQuickstartState(config.QuickstartState{
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

func TestPersistentQuickstartCreditManager_ExpiredTokenRebootstrapsWithInstallationToken(t *testing.T) {
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

	expiredAt := time.Now().Add(-5 * time.Minute).UTC().Unix()
	lastSyncedAt := time.Now().Add(-10 * time.Minute).UTC().Unix()
	if err := persistence.SaveQuickstartState(config.QuickstartState{
		QuickstartToken:            "qst_live_expired",
		QuickstartTokenExpiresAt:   &expiredAt,
		QuickstartCreditsRemaining: 11,
		QuickstartCreditsTotal:     25,
		LastSyncedAt:               &lastSyncedAt,
	}); err != nil {
		t.Fatalf("SaveQuickstartState(): %v", err)
	}

	client := &stubQuickstartBootstrapClient{
		resp: &pkglicensing.QuickstartBootstrapResponse{
			QuickstartToken:          "qst_live_rotated",
			QuickstartTokenExpiresAt: time.Now().Add(30 * time.Minute).UTC().Format(time.RFC3339),
			CreditsRemaining:         9,
			CreditsTotal:             25,
		},
	}
	mgr := NewPersistentQuickstartCreditManagerWithClient(persistence, "default", func() *config.AIConfig { return &config.AIConfig{Enabled: true} }, client)

	if err := mgr.EnsureBootstrap(context.Background()); err != nil {
		t.Fatalf("EnsureBootstrap(): %v", err)
	}
	if client.calls != 1 {
		t.Fatalf("BootstrapQuickstart calls = %d, want 1", client.calls)
	}
	if client.bearer != "pit_live_test" {
		t.Fatalf("bearer = %q, want pit_live_test", client.bearer)
	}
	if client.req.InstanceFingerprint != "fp-live-test" {
		t.Fatalf("InstanceFingerprint = %q, want fp-live-test", client.req.InstanceFingerprint)
	}

	loaded, err := persistence.LoadQuickstartState()
	if err != nil {
		t.Fatalf("LoadQuickstartState(): %v", err)
	}
	if loaded.QuickstartToken != "qst_live_rotated" {
		t.Fatalf("QuickstartToken = %q, want qst_live_rotated", loaded.QuickstartToken)
	}
	if loaded.QuickstartCreditsRemaining != 9 {
		t.Fatalf("QuickstartCreditsRemaining = %d, want 9", loaded.QuickstartCreditsRemaining)
	}
}

func TestPersistentQuickstartCreditManager_TenantUsesSharedInstallationActivationState(t *testing.T) {
	dir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(dir)

	defaultPersistence, err := mtp.GetPersistence("default")
	if err != nil {
		t.Fatalf("GetPersistence(default): %v", err)
	}
	tenantPersistence, err := mtp.GetPersistence("t-tenant")
	if err != nil {
		t.Fatalf("GetPersistence(t-tenant): %v", err)
	}

	licensePersistence, err := pkglicensing.NewPersistence(defaultPersistence.SharedInstallationDataDir())
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
			QuickstartToken:          "qst_live_tenant",
			QuickstartTokenExpiresAt: time.Now().Add(30 * time.Minute).UTC().Format(time.RFC3339),
			CreditsRemaining:         14,
			CreditsTotal:             25,
		},
	}
	mgr := NewPersistentQuickstartCreditManagerWithClient(tenantPersistence, "t-tenant", func() *config.AIConfig { return &config.AIConfig{Enabled: true} }, client)

	if err := mgr.EnsureBootstrap(context.Background()); err != nil {
		t.Fatalf("EnsureBootstrap(): %v", err)
	}
	if client.calls != 1 {
		t.Fatalf("BootstrapQuickstart calls = %d, want 1", client.calls)
	}
	if client.bearer != "pit_live_test" {
		t.Fatalf("bearer = %q, want pit_live_test", client.bearer)
	}
	if mgr.CreditsRemaining() != 14 {
		t.Fatalf("CreditsRemaining() = %d, want 14", mgr.CreditsRemaining())
	}
}
