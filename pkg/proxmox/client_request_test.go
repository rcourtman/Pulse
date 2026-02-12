package proxmox

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
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

func TestClientRequest_401PasswordAuthReauthAndRetry(t *testing.T) {
	var authCalls int32
	var nodeCalls int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api2/json/access/ticket":
			call := atomic.AddInt32(&authCalls, 1)
			fmt.Fprintf(w, `{"data":{"ticket":"ticket-%d","CSRFPreventionToken":"csrf-%d"}}`, call, call)
		case "/api2/json/nodes":
			call := atomic.AddInt32(&nodeCalls, 1)
			cookie := r.Header.Get("Cookie")
			if call == 1 {
				if !strings.Contains(cookie, "ticket-1") {
					t.Fatalf("first request missing initial ticket, got %q", cookie)
				}
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte("ticket expired"))
				return
			}
			if !strings.Contains(cookie, "ticket-2") {
				t.Fatalf("retry request missing refreshed ticket, got %q", cookie)
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"data":[]}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client, err := NewClient(ClientConfig{
		Host:      server.URL,
		User:      "user@pam",
		Password:  "secret",
		VerifySSL: false,
	})
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	resp, err := client.get(context.Background(), "/nodes")
	if err != nil {
		t.Fatalf("expected retry to succeed, got %v", err)
	}
	defer resp.Body.Close()
	_, _ = io.ReadAll(resp.Body)

	if got := atomic.LoadInt32(&authCalls); got != 2 {
		t.Fatalf("expected 2 auth calls (initial + refresh), got %d", got)
	}
	if got := atomic.LoadInt32(&nodeCalls); got != 2 {
		t.Fatalf("expected 2 node calls (initial + retry), got %d", got)
	}
}

func TestClientRequest_401PasswordAuthReauthFailure(t *testing.T) {
	var authCalls int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api2/json/access/ticket":
			call := atomic.AddInt32(&authCalls, 1)
			if call == 1 {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"data":{"ticket":"ticket-1","CSRFPreventionToken":"csrf-1"}}`))
				return
			}
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte("bad password"))
		case "/api2/json/nodes":
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte("ticket invalid"))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client, err := NewClient(ClientConfig{
		Host:      server.URL,
		User:      "user@pam",
		Password:  "secret",
		VerifySSL: false,
	})
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	_, err = client.get(context.Background(), "/nodes")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "re-authentication failed after 401") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClientRequest_500NonAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("boom"))
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
	msg := err.Error()
	if !strings.Contains(msg, "API error 500") {
		t.Fatalf("expected api error 500, got %q", msg)
	}
	if strings.Contains(strings.ToLower(msg), "authentication error") {
		t.Fatalf("did not expect authentication error for 500, got %q", msg)
	}
}
