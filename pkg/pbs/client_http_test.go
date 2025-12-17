package pbs

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewClient_TokenAuth_SetsAuthorizationHeader(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api2/json/version" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "PBSAPIToken=root@pam!pulse-token:secret" {
			t.Fatalf("Authorization = %q", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": Version{Version: "3.0", Release: "1", Repoid: "abc"},
		})
	}))
	defer server.Close()

	client, err := NewClient(ClientConfig{
		Host:       server.URL,
		TokenName:  "root@pam!pulse-token",
		TokenValue: "secret",
		Timeout:    2 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	v, err := client.GetVersion(context.Background())
	if err != nil {
		t.Fatalf("GetVersion: %v", err)
	}
	if v.Version != "3.0" {
		t.Fatalf("unexpected version: %+v", v)
	}
}

func TestNewClient_PasswordAuth_FallsBackToFormOnUnsupportedMediaType(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api2/json/access/ticket" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		switch atomic.AddInt32(&calls, 1) {
		case 1:
			if ct := r.Header.Get("Content-Type"); !strings.Contains(ct, "application/json") {
				t.Fatalf("expected json content-type, got %q", ct)
			}
			w.WriteHeader(http.StatusUnsupportedMediaType)
			_, _ = w.Write([]byte("unsupported"))
		case 2:
			if ct := r.Header.Get("Content-Type"); !strings.Contains(ct, "application/x-www-form-urlencoded") {
				t.Fatalf("expected form content-type, got %q", ct)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"ticket":              "ticket123",
					"CSRFPreventionToken": "csrf456",
				},
			})
		default:
			t.Fatalf("unexpected call count")
		}
	}))
	defer server.Close()

	client, err := NewClient(ClientConfig{
		Host:     server.URL,
		User:     "root@pam",
		Password: "password",
		Timeout:  2 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if client.auth.ticket != "ticket123" || client.auth.csrfToken != "csrf456" {
		t.Fatalf("expected auth fields to be set, got ticket=%q csrf=%q", client.auth.ticket, client.auth.csrfToken)
	}
}

func TestClient_request_SendsTicketAndCSRFToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api2/json/test" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Cookie"); got != "PBSAuthCookie=ticket123" {
			t.Fatalf("Cookie = %q", got)
		}
		if got := r.Header.Get("CSRFPreventionToken"); got != "csrf456" {
			t.Fatalf("CSRFPreventionToken = %q", got)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := NewClient(ClientConfig{
		Host:     server.URL,
		User:     "root@pam",
		Timeout:  2 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	client.auth.ticket = "ticket123"
	client.auth.csrfToken = "csrf456"
	client.auth.expiresAt = time.Now().Add(time.Hour)

	resp, err := client.request(context.Background(), http.MethodPost, "/test", url.Values{"a": {"b"}})
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	_ = resp.Body.Close()
}

func TestClient_GetNodeStatus_PermissionDeniedReturnsNil(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api2/json/nodes/localhost/status" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("permission denied"))
	}))
	defer server.Close()

	client, err := NewClient(ClientConfig{
		Host:       server.URL,
		TokenName:  "root@pam!pulse-token",
		TokenValue: "secret",
		Timeout:    2 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	status, err := client.GetNodeStatus(context.Background())
	if err != nil {
		t.Fatalf("GetNodeStatus: %v", err)
	}
	if status != nil {
		t.Fatalf("expected nil status on permission error, got: %+v", status)
	}
}

func TestClient_GetDatastores_HTMLResponseOnHTTP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api2/json/admin/datastore" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<html><body>error</body></html>"))
	}))
	defer server.Close()

	client, err := NewClient(ClientConfig{
		Host:       server.URL, // http://...
		TokenName:  "root@pam!pulse-token",
		TokenValue: "secret",
		Timeout:    2 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	_, err = client.GetDatastores(context.Background())
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "Try changing your URL") {
		t.Fatalf("unexpected error: %v", err)
	}
}
