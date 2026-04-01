package securityutil

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestValidateOutboundFetchURLRejectsLoopbackByDefault(t *testing.T) {
	if _, err := ValidateOutboundFetchURL(context.Background(), "http://127.0.0.1:8443/metadata", RestrictedOutboundHTTPOptions{
		AllowedSchemes:  []string{"http", "https"},
		AllowPrivateIPs: true,
	}); err == nil || !strings.Contains(err.Error(), "loopback") {
		t.Fatalf("expected loopback rejection, got %v", err)
	}
}

func TestValidateOutboundFetchURLAllowsLoopbackWhenRequested(t *testing.T) {
	target, err := ValidateOutboundFetchURL(context.Background(), "http://127.0.0.1:8443/metadata", RestrictedOutboundHTTPOptions{
		AllowedSchemes:  []string{"http", "https"},
		AllowPrivateIPs: true,
		AllowLoopback:   true,
	})
	if err != nil {
		t.Fatalf("ValidateOutboundFetchURL() error = %v", err)
	}
	if target.Host != "127.0.0.1:8443" {
		t.Fatalf("unexpected host %q", target.Host)
	}
}

func TestValidateOutboundFetchURLRejectsFragment(t *testing.T) {
	if _, err := ValidateOutboundFetchURL(context.Background(), "https://idp.example.com/metadata#fragment", RestrictedOutboundHTTPOptions{
		AllowedSchemes:  []string{"http", "https"},
		AllowPrivateIPs: true,
	}); err == nil || !strings.Contains(err.Error(), "fragments") {
		t.Fatalf("expected fragment rejection, got %v", err)
	}
}

func TestNewRestrictedOutboundHTTPClientBlocksCrossOriginRedirect(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()

	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+r.URL.Path, http.StatusFound)
	}))
	defer origin.Close()

	client := NewRestrictedOutboundHTTPClient(0, RestrictedOutboundHTTPOptions{
		AllowedSchemes:  []string{"http", "https"},
		AllowPrivateIPs: true,
		AllowLoopback:   true,
	})

	resp, err := client.Get(origin.URL + "/metadata")
	if resp != nil {
		resp.Body.Close()
	}
	if err == nil || !strings.Contains(err.Error(), "same origin") {
		t.Fatalf("expected same-origin redirect rejection, got %v", err)
	}
}
