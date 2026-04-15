package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestHandleCheckoutStart_ReopensOwnedBillingWhenPulseAccountHandoffIsUnavailable(t *testing.T) {
	handler := createTestHandler(t)
	handler.SetConfig(&config.Config{PublicURL: "https://pulse.example.com"})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/checkout/portal-handoff" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		http.Error(w, "portal handoff unavailable", http.StatusBadGateway)
	}))
	defer server.Close()
	t.Setenv("PULSE_LICENSE_SERVER_URL", server.URL)

	req := httptest.NewRequest(
		http.MethodGet,
		"https://pulse.example.com/auth/license-purchase-start?feature=max_monitored_systems",
		nil,
	)
	rec := httptest.NewRecorder()

	handler.HandleCheckoutStart(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d (body=%q)", rec.Code, http.StatusServiceUnavailable, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Pulse Account unavailable") {
		t.Fatalf("body = %q, want unavailable title", body)
	}
	if !strings.Contains(body, "window.opener.location.assign") {
		t.Fatalf("body = %q, want opener billing redirect", body)
	}
	if !strings.Contains(body, "window.location.replace(redirectPath)") {
		t.Fatalf("body = %q, want same-tab billing fallback", body)
	}
	if !strings.Contains(body, "window.close()") {
		t.Fatalf("body = %q, want popup close bridge", body)
	}
	if !strings.Contains(body, "/settings/system/billing/plan?intent=max_monitored_systems&purchase=unavailable") {
		t.Fatalf("body = %q, want owned billing unavailable route", body)
	}
}
