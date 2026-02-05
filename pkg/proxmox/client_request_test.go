package proxmox

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClientRequest_403TokenPermissionHint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte("forbidden"))
	}))
	defer server.Close()

	client, err := NewClient(ClientConfig{
		Host:       server.URL,
		TokenName:  "user@pve!token",
		TokenValue: "secret",
		VerifySSL:  false,
	})
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	_, err = client.get(context.Background(), "/nodes/node1/status")
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "authentication error") {
		t.Fatalf("expected authentication error, got %q", msg)
	}
	if !strings.Contains(msg, "does not have sufficient permissions") {
		t.Fatalf("expected permission hint, got %q", msg)
	}
	if !strings.Contains(msg, "user@pve") {
		t.Fatalf("expected user in error message, got %q", msg)
	}
}

func TestClientRequest_595NodeSpecific(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(595)
		w.Write([]byte("no ticket"))
	}))
	defer server.Close()

	client, err := NewClient(ClientConfig{
		Host:       server.URL,
		TokenName:  "user@pve!token",
		TokenValue: "secret",
		VerifySSL:  false,
	})
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	_, err = client.get(context.Background(), "/nodes/node1/status")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "Cannot access node resource") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClientRequest_595Auth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(595)
		w.Write([]byte("no ticket"))
	}))
	defer server.Close()

	client, err := NewClient(ClientConfig{
		Host:       server.URL,
		TokenName:  "user@pve!token",
		TokenValue: "secret",
		VerifySSL:  false,
	})
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	_, err = client.get(context.Background(), "/cluster/status")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "Authentication failed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClientRequest_401Unauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("unauthorized"))
	}))
	defer server.Close()

	client, err := NewClient(ClientConfig{
		Host:       server.URL,
		TokenName:  "user@pve!token",
		TokenValue: "secret",
		VerifySSL:  false,
	})
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	_, err = client.get(context.Background(), "/nodes")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "API error 401") {
		t.Fatalf("unexpected error: %v", err)
	}
}
