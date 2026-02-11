package agentupdate

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
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

func TestUpdater_getServerVersion_RejectsRedirects(t *testing.T) {
	var redirectedHits int32
	redirectTarget := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&redirectedHits, 1)
		_ = json.NewEncoder(w).Encode(map[string]string{"version": "9.9.9"})
	}))
	defer redirectTarget.Close()

	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, redirectTarget.URL+r.URL.Path, http.StatusFound)
	}))
	defer redirector.Close()

	u := New(Config{
		PulseURL:       redirector.URL,
		APIToken:       "token",
		CurrentVersion: "1.0.0",
	})

	_, err := u.getServerVersion(context.Background())
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "redirect") {
		t.Fatalf("expected redirect rejection error, got: %v", err)
	}
	if atomic.LoadInt32(&redirectedHits) != 0 {
		t.Fatalf("expected no redirected request to be sent")
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

func TestUpdater_performUpdateWithExecPath_RejectsRedirects(t *testing.T) {
	var redirectedHits int32
	redirectTarget := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&redirectedHits, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer redirectTarget.Close()

	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, redirectTarget.URL+r.URL.RequestURI(), http.StatusFound)
	}))
	defer redirector.Close()

	_, execPath := writeTempExec(t)
	u := New(Config{
		PulseURL:       redirector.URL,
		APIToken:       "token",
		AgentName:      "pulse-agent",
		CurrentVersion: "1.0.0",
	})

	origRestart := restartProcessFn
	t.Cleanup(func() { restartProcessFn = origRestart })
	restartProcessFn = func(string) error { return nil }

	err := u.performUpdateWithExecPath(context.Background(), execPath)
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "redirect") {
		t.Fatalf("expected redirect rejection error, got: %v", err)
	}
	if atomic.LoadInt32(&redirectedHits) != 0 {
		t.Fatalf("expected no redirected download request to be sent")
	}
}
