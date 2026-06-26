package securityutil

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func TestRestrictedOutboundHTTPClient_BlocksMetadataServiceEvenViaProxy(t *testing.T) {
	client := NewRestrictedOutboundHTTPClient(0, RestrictedOutboundHTTPOptions{
		AllowedSchemes:  []string{"http", "https"},
		AllowPrivateIPs: true,
		AllowLoopback:   true,
	})

	req, err := http.NewRequestWithContext(context.Background(), "GET", "http://169.254.169.254/api/version", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	resp, err := client.Do(req)
	if resp != nil {
		resp.Body.Close()
	}
	if err == nil {
		t.Fatal("expected error for metadata service address, got nil")
	}
	if !strings.Contains(err.Error(), "metadata service address is not allowed") {
		t.Fatalf("expected 'metadata service address is not allowed', got %v", err)
	}
}

func TestRestrictedOutboundHTTPClient_BlocksLinkLocalEvenViaProxy(t *testing.T) {
	client := NewRestrictedOutboundHTTPClient(0, RestrictedOutboundHTTPOptions{
		AllowedSchemes:  []string{"http", "https"},
		AllowPrivateIPs: true,
		AllowLoopback:   true,
	})

	req, err := http.NewRequestWithContext(context.Background(), "GET", "http://169.254.10.20/test", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	resp, err := client.Do(req)
	if resp != nil {
		resp.Body.Close()
	}
	if err == nil {
		t.Fatal("expected error for link-local address, got nil")
	}
	if !strings.Contains(err.Error(), "link-local addresses are not allowed") {
		t.Fatalf("expected 'link-local addresses are not allowed', got %v", err)
	}
}
