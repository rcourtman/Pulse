package agentupdate

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/updatesignature"
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
		if got := r.URL.Query().Get("agentVersion"); got != "1.0.0" {
			t.Fatalf("agentVersion query = %q, want %q", got, "1.0.0")
		}
		if got := r.URL.Query().Get("check"); got == "" {
			t.Fatal("expected a cache-unique version check query")
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

func TestNew_UsesPinnedServerFingerprintForHTTPTransport(t *testing.T) {
	u := New(Config{
		PulseURL:          "https://pulse.example.com",
		CurrentVersion:    "1.0.0",
		ServerFingerprint: "aabbccddaabbccddaabbccddaabbccddaabbccddaabbccddaabbccddaabbccdd",
	})

	transport, ok := u.client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("transport type = %T, want *http.Transport", u.client.Transport)
	}
	if transport.TLSClientConfig == nil {
		t.Fatal("expected TLS config to be configured")
	}
	if !transport.TLSClientConfig.InsecureSkipVerify {
		t.Fatal("expected fingerprint pinning to use explicit peer verification")
	}
	if transport.TLSClientConfig.VerifyPeerCertificate == nil {
		t.Fatal("expected VerifyPeerCertificate to be configured for fingerprint pinning")
	}
}

func TestNew_InvalidCustomCABundleFailsClosed(t *testing.T) {
	certPath := filepath.Join(t.TempDir(), "invalid.pem")
	if err := os.WriteFile(certPath, []byte("not-a-cert"), 0o600); err != nil {
		t.Fatalf("write cert: %v", err)
	}

	u := New(Config{
		PulseURL:       "https://pulse.example.com",
		CurrentVersion: "1.0.0",
		CACertPath:     certPath,
	})

	if u.client != nil {
		t.Fatal("expected updater HTTP client to remain disabled when the custom CA bundle is invalid")
	}
	if u.configErr == nil {
		t.Fatal("expected invalid CA bundle to populate configErr")
	}
	if _, err := u.getServerVersion(context.Background()); err == nil || !strings.Contains(err.Error(), "invalid TLS configuration") {
		t.Fatalf("expected invalid CA bundle error, got %v", err)
	}
}

func configureTrustedUpdateSigningKey(t *testing.T) ed25519.PrivateKey {
	t.Helper()

	original := updatesignature.EmbeddedTrustedPublicKeys
	t.Cleanup(func() { updatesignature.EmbeddedTrustedPublicKeys = original })

	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate signing key: %v", err)
	}
	updatesignature.EmbeddedTrustedPublicKeys = httpHeaderPublicKey(t, publicKey)
	return privateKey
}

func httpHeaderPublicKey(t *testing.T, publicKey ed25519.PublicKey) string {
	t.Helper()
	return base64.StdEncoding.EncodeToString(publicKey)
}

func signedUpdateHeader(t *testing.T, data []byte, privateKey ed25519.PrivateKey) string {
	t.Helper()
	signature, err := updatesignature.SignBytes(data, privateKey)
	if err != nil {
		t.Fatalf("sign update: %v", err)
	}
	return signature
}

func TestUpdater_performUpdateWithExecPath_RequiresSignatureWhenTrustedKeysConfigured(t *testing.T) {
	privateKey := configureTrustedUpdateSigningKey(t)
	data := testBinary()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("serverVersion"); got != "6.0.0" {
			t.Fatalf("serverVersion query = %q, want %q", got, "6.0.0")
		}
		w.Header().Set(checksumSHA256Header, checksum(data))
		w.Header().Set(signatureHeader, signedUpdateHeader(t, data, privateKey))
		_, _ = w.Write(data)
	}))
	defer server.Close()

	_, execPath := writeTempExec(t)
	u := newUpdaterForTest(server.URL)
	u.client = server.Client()

	origRestart := restartProcessFn
	t.Cleanup(func() { restartProcessFn = origRestart })
	restartProcessFn = func(string) error { return nil }

	if err := u.performUpdateWithExecPathForVersion(context.Background(), execPath, "6.0.0"); err != nil {
		t.Fatalf("performUpdateWithExecPathForVersion: %v", err)
	}
}

func TestUpdater_performUpdateWithExecPath_RejectsMissingSignatureWhenTrustedKeysConfigured(t *testing.T) {
	_ = configureTrustedUpdateSigningKey(t)
	data := testBinary()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(checksumSHA256Header, checksum(data))
		_, _ = w.Write(data)
	}))
	defer server.Close()

	_, execPath := writeTempExec(t)
	u := newUpdaterForTest(server.URL)
	u.client = server.Client()

	if err := u.performUpdateWithExecPath(context.Background(), execPath); err == nil || !strings.Contains(err.Error(), signatureHeader) {
		t.Fatalf("expected missing signature error, got %v", err)
	}
}

func TestUpdater_CheckAndUpdate_EarlyReturns(t *testing.T) {
	u := New(Config{Disabled: true})
	u.performUpdateFn = func(ctx context.Context, _ string) error {
		t.Fatalf("performUpdate should not be called")
		return nil
	}
	u.CheckAndUpdate(context.Background())

	u = New(Config{CurrentVersion: "dev"})
	u.performUpdateFn = func(ctx context.Context, _ string) error {
		t.Fatalf("performUpdate should not be called")
		return nil
	}
	u.CheckAndUpdate(context.Background())

	u = New(Config{CurrentVersion: "1.0.0", PulseURL: ""})
	u.performUpdateFn = func(ctx context.Context, _ string) error {
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
		{"release-candidate-to-release-candidate", "6.0.0-rc.1", "6.0.0-rc.6", true, true},
		{"release-candidate-to-stable", "6.0.0-rc.6", "6.0.0", true, true},
		{"stable-to-stable", "6.0.0", "6.1.1", true, true},
		{"stable-does-not-downgrade-to-release-candidate", "6.0.0", "6.0.0-rc.7", false, true},
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
			u.performUpdateFn = func(ctx context.Context, targetVersion string) error {
				called = true
				if targetVersion != tc.server {
					t.Fatalf("target version = %q, want %q", targetVersion, tc.server)
				}
				return nil
			}

			u.CheckAndUpdate(context.Background())

			if called != tc.expectUpdate {
				t.Fatalf("performUpdate called=%v, want %v", called, tc.expectUpdate)
			}
		})
	}
}

func TestUpdater_CheckAndUpdate_ReconcilesVersionAndBinaryAcrossCachingProxy(t *testing.T) {
	originalKeys := updatesignature.EmbeddedTrustedPublicKeys
	updatesignature.EmbeddedTrustedPublicKeys = ""
	t.Cleanup(func() { updatesignature.EmbeddedTrustedPublicKeys = originalKeys })

	targetVersion := "6.0.0"
	staleBinary := append(testBinary(), []byte("-stale-rc.1")...)
	targetBinary := append(testBinary(), []byte("-stable-6.0.0")...)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/agent/version":
			if r.URL.Query().Get("check") == "" {
				_ = json.NewEncoder(w).Encode(serverVersionResponse{Version: "6.0.0-rc.1"})
				return
			}
			if got := r.URL.Query().Get("agentVersion"); got != "6.0.0-rc.1" {
				t.Fatalf("agentVersion query = %q, want %q", got, "6.0.0-rc.1")
			}
			_ = json.NewEncoder(w).Encode(serverVersionResponse{Version: targetVersion})
		case "/download/pulse-agent":
			payload := staleBinary
			if r.URL.Query().Get("serverVersion") == targetVersion {
				payload = targetBinary
			}
			w.Header().Set(checksumSHA256Header, checksum(payload))
			_, _ = w.Write(payload)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	_, execPath := writeTempExec(t)
	u := New(Config{
		PulseURL:       server.URL,
		AgentName:      "pulse-agent",
		CurrentVersion: "6.0.0-rc.1",
	})
	u.client = server.Client()
	u.selfTestFn = func(context.Context, string) error { return nil }
	u.performUpdateFn = func(ctx context.Context, version string) error {
		return u.performUpdateWithExecPathForVersion(ctx, execPath, version)
	}

	originalRestart := restartProcessFn
	restartProcessFn = func(string) error { return nil }
	t.Cleanup(func() { restartProcessFn = originalRestart })

	u.CheckAndUpdate(context.Background())

	got, err := os.ReadFile(execPath)
	if err != nil {
		t.Fatalf("read updated binary: %v", err)
	}
	if string(got) != string(targetBinary) {
		t.Fatalf("updated binary came from stale cache: got %q want %q", got, targetBinary)
	}
	status := u.Snapshot()
	if status.State != UpdateStateIdle || status.LastSuccessAt == nil || status.LastError != "" {
		t.Fatalf("reconciled update status = %+v", status)
	}
}

func TestUpdater_CheckAndUpdate_RecoversAfterOfflineAndPartialFailure(t *testing.T) {
	var versionChecks int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&versionChecks, 1) <= updateRequestMaxAttempts {
			http.Error(w, "offline", http.StatusServiceUnavailable)
			return
		}
		_ = json.NewEncoder(w).Encode(serverVersionResponse{Version: "6.1.1"})
	}))
	defer server.Close()

	originalSleep := retrySleepFn
	retrySleepFn = func(context.Context, time.Duration) error { return nil }
	t.Cleanup(func() { retrySleepFn = originalSleep })

	u := New(Config{PulseURL: server.URL, CurrentVersion: "6.1.0"})
	var updateAttempts int32
	u.performUpdateFn = func(context.Context, string) error {
		if atomic.AddInt32(&updateAttempts, 1) == 1 {
			return errors.New("partial replacement rejected")
		}
		return nil
	}

	u.CheckAndUpdate(context.Background())
	if status := u.Snapshot(); status.State != UpdateStateError || status.LastCheckedAt == nil {
		t.Fatalf("offline status = %+v", status)
	}

	u.CheckAndUpdate(context.Background())
	if status := u.Snapshot(); status.State != UpdateStateError || status.LastError != "partial replacement rejected" {
		t.Fatalf("partial-failure status = %+v", status)
	}

	u.CheckAndUpdate(context.Background())
	status := u.Snapshot()
	if status.State != UpdateStateIdle || status.LastSuccessAt == nil || status.LastError != "" {
		t.Fatalf("recovered status = %+v", status)
	}
	if got := atomic.LoadInt32(&updateAttempts); got != 2 {
		t.Fatalf("update attempts = %d, want 2", got)
	}
}

func TestUpdater_CheckAndUpdate_RecordsLifecycleStatus(t *testing.T) {
	t.Run("current", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode(serverVersionResponse{Version: "1.0.0"})
		}))
		defer srv.Close()

		u := New(Config{PulseURL: srv.URL, CurrentVersion: "1.0.0"})
		u.CheckAndUpdate(context.Background())

		status := u.Snapshot()
		if status.State != UpdateStateIdle || !status.AutoUpdate || status.LastCheckedAt == nil {
			t.Fatalf("current status = %+v", status)
		}
		if status.AvailableVersion != "" || status.LastError != "" {
			t.Fatalf("current status retained transient fields: %+v", status)
		}
	})

	t.Run("failed update", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode(serverVersionResponse{Version: "1.1.0"})
		}))
		defer srv.Close()

		u := New(Config{PulseURL: srv.URL, CurrentVersion: "1.0.0"})
		u.performUpdateFn = func(context.Context, string) error { return errors.New("replacement rejected") }
		u.CheckAndUpdate(context.Background())

		status := u.Snapshot()
		if status.State != UpdateStateError || status.AvailableVersion != "1.1.0" {
			t.Fatalf("failed update status = %+v", status)
		}
		if status.LastCheckedAt == nil || status.LastAttemptAt == nil || status.LastError != "replacement rejected" {
			t.Fatalf("failed update evidence = %+v", status)
		}
	})

	t.Run("disabled", func(t *testing.T) {
		status := New(Config{Disabled: true}).Snapshot()
		if status.State != UpdateStateDisabled || status.AutoUpdate {
			t.Fatalf("disabled status = %+v", status)
		}
	})
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

func drainNudge(u *Updater) bool {
	select {
	case <-u.nudgeCh:
		return true
	default:
		return false
	}
}

func TestNudgeVersionQueuesOnlyForNewerVersions(t *testing.T) {
	t.Parallel()

	u := New(Config{CurrentVersion: "6.2.0", PulseURL: "http://127.0.0.1:7655"})

	for _, version := range []string{"", "6.2.0", "6.1.9", "v6.2.0", "not-a-version"} {
		u.NudgeVersion(version)
		if drainNudge(u) {
			t.Fatalf("NudgeVersion(%q) queued a nudge, want none", version)
		}
	}

	u.NudgeVersion("6.2.1")
	if !drainNudge(u) {
		t.Fatal("NudgeVersion(6.2.1) queued no nudge, want one")
	}
}

func TestNudgeVersionComparesPrereleaseIdentifiers(t *testing.T) {
	t.Parallel()

	u := New(Config{CurrentVersion: "6.2.0-rc.8", PulseURL: "http://127.0.0.1:7655"})

	u.NudgeVersion("6.2.0-rc.8")
	if drainNudge(u) {
		t.Fatal("equal prerelease queued a nudge, want none")
	}

	u.NudgeVersion("v6.2.0-rc.9")
	if !drainNudge(u) {
		t.Fatal("newer prerelease queued no nudge, want one")
	}

	// A stable release outranks its own release candidates.
	u.NudgeVersion("6.2.0")
	if !drainNudge(u) {
		t.Fatal("stable release above rc queued no nudge, want one")
	}
}

func TestNudgeVersionNudgesEachDistinctVersionOnce(t *testing.T) {
	t.Parallel()

	u := New(Config{CurrentVersion: "6.2.0", PulseURL: "http://127.0.0.1:7655"})

	u.NudgeVersion("6.2.1")
	if !drainNudge(u) {
		t.Fatal("first nudge for 6.2.1 not queued")
	}

	// Repeats of the same server version — one per report cycle in production —
	// must not re-wake the loop even after the first nudge was consumed.
	u.NudgeVersion("6.2.1")
	if drainNudge(u) {
		t.Fatal("repeated nudge for 6.2.1 queued, want dedupe")
	}

	u.NudgeVersion("6.2.2")
	if !drainNudge(u) {
		t.Fatal("nudge for distinct newer version 6.2.2 not queued")
	}
}

func TestNudgeVersionRespectsDisabledAndDevelopmentGates(t *testing.T) {
	t.Parallel()

	disabled := New(Config{CurrentVersion: "6.2.0", PulseURL: "http://127.0.0.1:7655", Disabled: true})
	disabled.NudgeVersion("6.2.1")
	if drainNudge(disabled) {
		t.Fatal("disabled updater queued a nudge, want none")
	}

	dev := New(Config{CurrentVersion: developmentVersion, PulseURL: "http://127.0.0.1:7655"})
	dev.NudgeVersion("6.2.1")
	if drainNudge(dev) {
		t.Fatal("development-mode updater queued a nudge, want none")
	}

	current := New(Config{CurrentVersion: "6.2.0", PulseURL: "http://127.0.0.1:7655"})
	current.NudgeVersion(developmentVersion)
	if drainNudge(current) {
		t.Fatal("development server version queued a nudge, want none")
	}
}

func TestRunLoopRunsCheckOnNudge(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(serverVersionResponse{Version: "6.2.1"})
	}))
	defer srv.Close()

	u := New(Config{CurrentVersion: "6.2.0", PulseURL: srv.URL})
	// Keep the initial check and the hourly ticker out of the way so the only
	// thing that can trigger a check is the nudge.
	u.initialDelay = time.Hour
	updated := make(chan string, 1)
	u.performUpdateFn = func(_ context.Context, targetVersion string) error {
		updated <- targetVersion
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		u.RunLoop(ctx)
	}()

	u.NudgeVersion("6.2.1")

	select {
	case targetVersion := <-updated:
		if targetVersion != "6.2.1" {
			t.Fatalf("performUpdate target = %q, want %q", targetVersion, "6.2.1")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("nudge did not trigger an update check")
	}

	cancel()
	<-done
}
