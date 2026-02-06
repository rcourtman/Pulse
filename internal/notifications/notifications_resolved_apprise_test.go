package notifications

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
)

func TestSendResolvedApprise_NoAlerts(t *testing.T) {
	nm := NewNotificationManager("")
	defer nm.Stop()

	err := nm.sendResolvedApprise(AppriseConfig{Enabled: true}, nil, time.Now())
	if err == nil {
		t.Fatalf("expected error for empty alerts list")
	}
}

func TestSendResolvedApprise_DisabledConfig(t *testing.T) {
	nm := NewNotificationManager("")
	defer nm.Stop()

	alertList := []*alerts.Alert{{ID: "a1", ResourceName: "db"}}
	err := nm.sendResolvedApprise(AppriseConfig{Enabled: false}, alertList, time.Now())
	if err == nil {
		t.Fatalf("expected error for disabled config")
	}
}

func TestSendResolvedApprise_InvalidPayload(t *testing.T) {
	nm := NewNotificationManager("")
	defer nm.Stop()

	alertList := []*alerts.Alert{nil}
	err := nm.sendResolvedApprise(AppriseConfig{Enabled: true, Targets: []string{"discord://token"}}, alertList, time.Now())
	if err == nil {
		t.Fatalf("expected error for invalid payload")
	}
}

func TestSendResolvedApprise_HTTP(t *testing.T) {
	nm := NewNotificationManager("")
	defer nm.Stop()

	server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	if err := nm.UpdateAllowedPrivateCIDRs("127.0.0.1/32"); err != nil {
		t.Fatalf("allowlist: %v", err)
	}

	alertList := []*alerts.Alert{{
		ID:           "a1",
		ResourceName: "db-1",
		Level:        alerts.AlertLevelWarning,
		Message:      "ok",
	}}

	err := nm.sendResolvedApprise(AppriseConfig{
		Enabled:        true,
		Mode:           AppriseModeHTTP,
		ServerURL:      server.URL,
		TimeoutSeconds: 2,
	}, alertList, time.Now())
	if err != nil {
		t.Fatalf("expected resolved apprise HTTP to succeed, got %v", err)
	}
}

func TestSendResolvedApprise_CLI(t *testing.T) {
	nm := NewNotificationManager("")
	defer nm.Stop()

	nm.appriseExec = func(ctx context.Context, path string, args []string) ([]byte, error) {
		return nil, nil
	}

	alertList := []*alerts.Alert{{
		ID:           "a1",
		ResourceName: "db-1",
		Level:        alerts.AlertLevelWarning,
		Message:      "ok",
	}}

	err := nm.sendResolvedApprise(AppriseConfig{
		Enabled:        true,
		Mode:           AppriseModeCLI,
		Targets:        []string{"discord://token"},
		TimeoutSeconds: 2,
	}, alertList, time.Now())
	if err != nil {
		t.Fatalf("expected resolved apprise CLI to succeed, got %v", err)
	}
}
