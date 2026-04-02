package ai

import (
	"context"
	"net/http"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
)

type stubServiceQuickstartManager struct {
	ensureErr error
	remaining int
	total     int
	provider  providers.Provider
	byok      bool
}

func (m *stubServiceQuickstartManager) EnsureBootstrap(context.Context) error { return m.ensureErr }
func (m *stubServiceQuickstartManager) HasCredits() bool                      { return m.remaining > 0 }
func (m *stubServiceQuickstartManager) CreditsRemaining() int                 { return m.remaining }
func (m *stubServiceQuickstartManager) CreditsTotal() int {
	if m.total > 0 {
		return m.total
	}
	return pkglicensing.QuickstartCreditsTotal
}
func (m *stubServiceQuickstartManager) HasBYOK() bool                   { return m.byok }
func (m *stubServiceQuickstartManager) GetProvider() providers.Provider { return m.provider }

func TestServiceLoadConfig_PrefersBYOKProviderOverQuickstart(t *testing.T) {
	dir := t.TempDir()
	persistence := config.NewConfigPersistence(dir)
	cfg := config.NewDefaultAIConfig()
	cfg.Enabled = true
	cfg.Model = "ollama:llama3"
	cfg.OllamaBaseURL = "http://127.0.0.1:11434"
	if err := persistence.SaveAIConfig(*cfg); err != nil {
		t.Fatalf("SaveAIConfig(): %v", err)
	}

	svc := NewService(persistence, nil)
	svc.SetQuickstartCredits(&stubServiceQuickstartManager{
		ensureErr: &pkglicensing.LicenseServerError{StatusCode: http.StatusServiceUnavailable, Code: "temporarily_unavailable", Message: "down", Retryable: true},
		remaining: 25,
		total:     25,
		provider:  providers.NewQuickstartClientWithToken("qst_live_test", nil, nil),
	})

	if err := svc.LoadConfig(); err != nil {
		t.Fatalf("LoadConfig(): %v", err)
	}
	if svc.IsUsingQuickstart() {
		t.Fatal("expected BYOK provider to win over quickstart")
	}
	if svc.QuickstartBlockedReason() != "" {
		t.Fatalf("QuickstartBlockedReason() = %q, want empty when BYOK is configured", svc.QuickstartBlockedReason())
	}
	if !svc.IsEnabled() {
		t.Fatal("expected BYOK AI service to remain enabled")
	}
}

func TestServiceLoadConfig_TracksQuickstartBootstrapFailure(t *testing.T) {
	dir := t.TempDir()
	persistence := config.NewConfigPersistence(dir)
	cfg := config.NewDefaultAIConfig()
	cfg.Enabled = true
	if err := persistence.SaveAIConfig(*cfg); err != nil {
		t.Fatalf("SaveAIConfig(): %v", err)
	}

	svc := NewService(persistence, nil)
	svc.SetQuickstartCredits(&stubServiceQuickstartManager{
		ensureErr: &pkglicensing.LicenseServerError{StatusCode: http.StatusServiceUnavailable, Code: "temporarily_unavailable", Message: "down", Retryable: true},
	})

	if err := svc.LoadConfig(); err != nil {
		t.Fatalf("LoadConfig(): %v", err)
	}
	if svc.QuickstartBlockedReason() != patrolQuickstartUnavailableReason {
		t.Fatalf("QuickstartBlockedReason() = %q, want %q", svc.QuickstartBlockedReason(), patrolQuickstartUnavailableReason)
	}
	if svc.IsEnabled() {
		t.Fatal("expected AI service to remain disabled when bootstrap failed and no BYOK exists")
	}
}
