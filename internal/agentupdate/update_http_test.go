package agentupdate

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestUpdater_getServerVersion_SetsAuthHeaders(t *testing.T) {
	var sawAuth bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/agent/version" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("X-API-Token") != "token" {
			t.Fatalf("X-API-Token = %q", r.Header.Get("X-API-Token"))
		}
		if r.Header.Get("Authorization") != "Bearer token" {
			t.Fatalf("Authorization = %q", r.Header.Get("Authorization"))
		}
		sawAuth = true
		_ = json.NewEncoder(w).Encode(map[string]string{"version": "1.2.3"})
	}))
	defer srv.Close()

	u := New(Config{
		PulseURL:       srv.URL,
		APIToken:       "token",
		CurrentVersion: "1.0.0",
		CheckInterval:  time.Minute,
	})

	v, err := u.getServerVersion(context.Background())
	if err != nil {
		t.Fatalf("getServerVersion: %v", err)
	}
	if v != "1.2.3" || !sawAuth {
		t.Fatalf("unexpected version=%q sawAuth=%v", v, sawAuth)
	}
}

func TestUpdater_getServerVersion_BadStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	u := New(Config{PulseURL: srv.URL, CurrentVersion: "1.0.0"})
	_, err := u.getServerVersion(context.Background())
	if err == nil || !strings.Contains(err.Error(), "status") {
		t.Fatalf("expected status error, got: %v", err)
	}
}

func TestUpdater_getServerVersion_BadJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("{not-json"))
	}))
	defer srv.Close()

	u := New(Config{PulseURL: srv.URL, CurrentVersion: "1.0.0"})
	_, err := u.getServerVersion(context.Background())
	if err == nil || !strings.Contains(err.Error(), "decode") {
		t.Fatalf("expected decode error, got: %v", err)
	}
}

func TestUpdater_CheckAndUpdate_EarlyReturns(t *testing.T) {
	u := New(Config{Disabled: true})
	u.performUpdateFn = func(ctx context.Context) error {
		t.Fatalf("performUpdate should not be called")
		return nil
	}
	u.CheckAndUpdate(context.Background())

	u = New(Config{CurrentVersion: "dev"})
	u.performUpdateFn = func(ctx context.Context) error {
		t.Fatalf("performUpdate should not be called")
		return nil
	}
	u.CheckAndUpdate(context.Background())

	u = New(Config{CurrentVersion: "1.0.0", PulseURL: ""})
	u.performUpdateFn = func(ctx context.Context) error {
		t.Fatalf("performUpdate should not be called")
		return nil
	}
	u.CheckAndUpdate(context.Background())
}

func TestUpdater_CheckAndUpdate_VersionComparePaths(t *testing.T) {
	tests := []struct {
		name          string
		current       string
		server        string
		expectUpdate  bool
		expectNoError bool
	}{
		{"up-to-date", "1.0.0", "1.0.0", false, true},
		{"server-older", "1.0.1", "1.0.0", false, true},
		{"server-dev", "1.0.0", "dev", false, true},
		{"server-newer", "1.0.0", "1.0.1", true, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var called bool

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_ = json.NewEncoder(w).Encode(map[string]string{"version": tc.server})
			}))
			defer srv.Close()

			u := New(Config{
				PulseURL:       srv.URL,
				AgentName:      "pulse-agent",
				CurrentVersion: tc.current,
				CheckInterval:  time.Minute,
			})
			u.performUpdateFn = func(ctx context.Context) error {
				called = true
				return nil
			}

			u.CheckAndUpdate(context.Background())

			if called != tc.expectUpdate {
				t.Fatalf("performUpdate called=%v, want %v", called, tc.expectUpdate)
			}
		})
	}
}
