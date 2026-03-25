package providers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestQuickstartProxyURL_Default(t *testing.T) {
	t.Setenv("PULSE_AI_QUICKSTART_PROXY_URL", "")
	if got := quickstartProxyURL(); got != defaultQuickstartProxyURL {
		t.Fatalf("quickstartProxyURL()=%q want %q", got, defaultQuickstartProxyURL)
	}
}

func TestQuickstartClientChat_UsesOverrideProxyURL(t *testing.T) {
	var seenLicenseID string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req quickstartRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		seenLicenseID = req.LicenseID
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(quickstartResponse{
			Content:    "hello",
			Model:      quickstartModel,
			StopReason: "end_turn",
		}); err != nil {
			t.Fatalf("encode response: %v", err)
		}
	}))
	defer server.Close()

	t.Setenv("PULSE_AI_QUICKSTART_PROXY_URL", server.URL)

	client := NewQuickstartClient("lic_test")
	resp, err := client.Chat(context.Background(), ChatRequest{
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("Chat(): %v", err)
	}
	if seenLicenseID != "lic_test" {
		t.Fatalf("license_id=%q want lic_test", seenLicenseID)
	}
	if resp.Content != "hello" {
		t.Fatalf("content=%q want hello", resp.Content)
	}
}
