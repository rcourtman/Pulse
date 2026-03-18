package agentupdate

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const testPEMCertificate = `-----BEGIN CERTIFICATE-----
MIICuDCCAaACCQDptFpSdDdFNjANBgkqhkiG9w0BAQsFADAeMRwwGgYDVQQDDBNw
dWxzZS10ZXN0LWNhLmxvY2FsMB4XDTI2MDMxNDA4NTgyN1oXDTI3MDMxNDA4NTgy
N1owHjEcMBoGA1UEAwwTcHVsc2UtdGVzdC1jYS5sb2NhbDCCASIwDQYJKoZIhvcN
AQEBBQADggEPADCCAQoCggEBANWmj5xXF1pDWKqbScN6VtU1PX3e9DuyDnegnAuR
UA7QIqgyQ7gfPZtAABr0kaV993mZZw92XkdXeF+9eClRBnVoJmISdwiBpB6oE8w/
H6tfnG34JUjvXN39/B66mAeuBd/erAxj4fXuH+ohA3AWZcotCYS2anOAbyRPo8BU
DGm79VBp5/s/uZ8bGe5LiSPxFXOp7kBk2sDWI77Y0UNwuc/wzO+GrE0GGXnbxcRW
9ICRPq7pked0BO2oBaeMRmvo7npAn9+w+0EDVi1qqw5xoYposYgsR76uLSYhQgaL
5ZgUYlCW7Vvp5ve/tmxPXuae8y3OIrOT7WFWfm8GAa9ZneMCAwEAATANBgkqhkiG
9w0BAQsFAAOCAQEAdpFuEiVPhYcJe/kkfPuHwv68Dx+/5jFXMkLQFIZnnC5Umkph
ubtFPrce9BLqLQBGdhQ4IkaEA9QDSZDTUbzZLtw3G6tHgl63H4kuB5ZbXgEVPmNT
07i8Obt4uUgIhfx/EzyCaZpfoQnXHmHm2xxg6QiP4v2TUQdBkLpD5mzVTwYOw9GF
w8AuCKd92UTs4/0ikTMdK0M4zwhF0JAhibyMNBRXfg1c96KyCFYSSNeERQFy5Fqo
TREsx8ScXgne7V+lLwLa8CTjUAcvCVq6SIqKbjSEZ1V5UpzvwBh52/cWCa6Rafd5
ARKc3gwyVxyCX3h21kFcEU2rt7C7/RcXBCyWzQ==
-----END CERTIFICATE-----
`

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
		_ = json.NewEncoder(w).Encode(serverVersionResponse{Version: "1.2.3"})
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

func TestNew_UsesCustomCABundleForHTTPTransport(t *testing.T) {
	certPath := filepath.Join(t.TempDir(), "pulse-ca.pem")
	if err := os.WriteFile(certPath, []byte(testPEMCertificate), 0o600); err != nil {
		t.Fatalf("write cert: %v", err)
	}

	u := New(Config{
		PulseURL:       "https://pulse.example.com",
		CurrentVersion: "1.0.0",
		CACertPath:     certPath,
	})

	transport, ok := u.client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("transport type = %T, want *http.Transport", u.client.Transport)
	}
	if transport.TLSClientConfig == nil || transport.TLSClientConfig.RootCAs == nil {
		t.Fatalf("expected RootCAs to be populated when CACertPath is configured")
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
				_ = json.NewEncoder(w).Encode(serverVersionResponse{Version: tc.server})
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
