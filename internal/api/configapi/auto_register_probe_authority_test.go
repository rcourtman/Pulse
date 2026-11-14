package configapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/logging"
)

// recordTLSFingerprintFetches replaces the fingerprint fetch with one that
// records each address Pulse contacts and answers with answer(address).
func recordTLSFingerprintFetches(t *testing.T, answer func(string) (string, error)) func() []string {
	t.Helper()
	var mu sync.Mutex
	var contacted []string
	original := fetchTLSFingerprint
	fetchTLSFingerprint = func(address string) (string, error) {
		mu.Lock()
		contacted = append(contacted, address)
		mu.Unlock()
		return answer(address)
	}
	t.Cleanup(func() { fetchTLSFingerprint = original })
	return func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), contacted...)
	}
}

// Pulse contacts a candidate address for its TLS certificate only once the
// caller is known to be allowed to change that address. An agent token with
// no install grant may change only its recorded connection, so no other
// address is contacted, and a refused request names none of the candidates.
func TestAgentTokenAutoRegisterContactsOnlyAddressesItMayChange(t *testing.T) {
	probe := AutoRegisterRequest{
		Type: "pve", Host: "https://10.20.30.40:8006",
		CandidateHosts: []string{"https://10.20.30.41:8443", "https://pve-real.local:8006"},
		ServerName:     "probe", Source: "agent",
		TokenID: "pulse-monitor@pve!pulse-probe", TokenValue: "probe-secret",
	}
	unowned := map[string]map[string]string{
		"unregistered":   nil,
		"bound hostname": {"bound_hostname": "probe", "bound_agent_id": "agent-1"},
		"owns pbs only":  {proxmoxInstallRegistrationTypedKey(proxmoxInstallRegistrationHostKey, "pbs"): "https://pbs-real.local:8007"},
	}
	for name, metadata := range unowned {
		rawToken := strings.ReplaceAll(name, " ", "-") + "-probe.12345678"
		handler, cfg := agentAuthorityFixture(t, rawToken, metadata)
		contacted := recordTLSFingerprintFetches(t, func(string) (string, error) { return "AA:AA", nil })
		pve, pbs := snapshotAutoRegisterSources(cfg)

		rec := runAgentAutoRegister(t, handler, rawToken, probe)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("%s: status = %d, want 403; body=%s", name, rec.Code, rec.Body.String())
		}
		if got := contacted(); len(got) != 0 {
			t.Fatalf("%s: Pulse contacted %v for a caller that may change none of them", name, got)
		}
		for _, address := range append([]string{probe.Host}, probe.CandidateHosts...) {
			host := strings.TrimPrefix(address, "https://")
			if strings.Contains(rec.Body.String(), host) {
				t.Fatalf("%s: refusal names candidate %q: %s", name, host, rec.Body.String())
			}
		}
		assertAutoRegisterSourcesUnchanged(t, name, cfg, pve, pbs)
	}

	// The token that owns pve-real is still refreshed and re-pinned, but only
	// its own address is contacted, wherever it sits in the candidate list.
	rawToken := "owns-pve-real-probe.12345678"
	handler, cfg := agentAuthorityFixture(t, rawToken, map[string]string{
		proxmoxInstallRegistrationTypedKey(proxmoxInstallRegistrationHostKey, "pve"): "https://pve-real.local:8006",
	})
	contacted := recordTLSFingerprintFetches(t, func(string) (string, error) { return "AA:AA", nil })
	rec := runAgentAutoRegister(t, handler, rawToken, probe)
	if rec.Code != http.StatusOK {
		t.Fatalf("owned connection status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if got := contacted(); len(got) != 1 || got[0] != "https://pve-real.local:8006" {
		t.Fatalf("contacted %v, want only the recorded connection address", got)
	}
	var response AutoRegisterResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Host != "https://pve-real.local:8006" {
		t.Fatalf("response host = %q, want the recorded connection address", response.Host)
	}
	got := cfg.PVEInstances[0]
	if got.Host != "https://pve-real.local:8006" || got.TokenValue != "probe-secret" || got.Fingerprint != "AA:AA" {
		t.Fatalf("owned connection = host %q token %q pin %q", got.Host, got.TokenValue, got.Fingerprint)
	}
}

// The probe filter compares addresses as spelled, never by resolving names:
// a name that resolves to the recorded address when checked could dial
// another address when Pulse connects.
func TestAgentTokenAutoRegisterProbeIgnoresNameAliases(t *testing.T) {
	stubAutoRegisterNetworkDeps(t)
	rawToken := "loopback-owner.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeAgentReport}, map[string]string{
		proxmoxInstallRegistrationTypedKey(proxmoxInstallRegistrationHostKey, "pve"): "https://127.0.0.1:8006",
	})
	cfg := &config.Config{
		DataPath:  t.TempDir(),
		APITokens: []config.APITokenRecord{record},
		PVEInstances: []config.PVEInstance{{
			Name:       "pve-loopback",
			Host:       "https://127.0.0.1:8006",
			TokenName:  "pulse-monitor@pve!pulse-loopback",
			TokenValue: "loopback-secret",
		}},
	}
	handler := newTestConfigHandlers(t, cfg)
	contacted := recordTLSFingerprintFetches(t, func(string) (string, error) { return "AA:AA", nil })

	rec := runAgentAutoRegister(t, handler, rawToken, AutoRegisterRequest{
		Type: "pve", Host: "https://localhost:8006", CandidateHosts: []string{"https://127.0.0.2:8006"},
		ServerName: "pve-loopback", Source: "agent",
		TokenID: "pulse-monitor@pve!pulse-loopback", TokenValue: "rotated-secret",
	})
	if got := contacted(); len(got) != 0 {
		t.Fatalf("contacted %v through name or loopback aliases of the recorded address", got)
	}
	if got := cfg.PVEInstances[0]; got.Host != "https://127.0.0.1:8006" || got.Fingerprint != "" {
		t.Fatalf("alias request moved or re-pinned the connection: host %q pin %q (status %d)", got.Host, got.Fingerprint, rec.Code)
	}

	rec = runAgentAutoRegister(t, handler, rawToken, AutoRegisterRequest{
		Type: "pve", Host: "https://localhost:8006", CandidateHosts: []string{"https://127.0.0.1:8006"},
		ServerName: "pve-loopback", Source: "agent",
		TokenID: "pulse-monitor@pve!pulse-loopback", TokenValue: "rotated-secret",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("recorded address status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if got := contacted(); len(got) != 1 || got[0] != "https://127.0.0.1:8006" {
		t.Fatalf("contacted %v, want only the recorded address as spelled", got)
	}
	if got := cfg.PVEInstances[0]; got.Fingerprint != "AA:AA" || got.TokenValue != "rotated-secret" {
		t.Fatalf("recorded address heal: pin %q token %q", got.Fingerprint, got.TokenValue)
	}
}

// An install grant may create a source at any address the agent offers, so
// the reachable-fallback selection still tries every candidate for it.
func TestInstallGrantAutoRegisterStillSelectsReachableFallback(t *testing.T) {
	stubAutoRegisterNetworkDeps(t)
	rawToken := "install-fallback-probe.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeAgentReport}, map[string]string{
		"install_type": "pve",
		"issued_via":   agentInstallIssuedViaConfig,
	})
	cfg := &config.Config{DataPath: t.TempDir(), APITokens: []config.APITokenRecord{record}}
	handler := newTestConfigHandlers(t, cfg)
	contacted := recordTLSFingerprintFetches(t, func(address string) (string, error) {
		if address == "https://10.0.0.2:8006" {
			return "CC:CC", nil
		}
		return "", errors.New("unreachable")
	})

	rec := runAgentAutoRegister(t, handler, rawToken, AutoRegisterRequest{
		Type: "pve", Host: "https://10.0.0.1:8006", CandidateHosts: []string{"https://10.0.0.2:8006"},
		ServerName: "fresh-node", Source: "agent",
		TokenID: "pulse-monitor@pve!pulse-fresh", TokenValue: "fresh-secret",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("install grant status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if got := contacted(); len(got) != 2 {
		t.Fatalf("contacted %v, want both candidates tried in order", got)
	}
	if len(cfg.PVEInstances) != 1 || cfg.PVEInstances[0].Host != "https://10.0.0.2:8006" || cfg.PVEInstances[0].Fingerprint != "CC:CC" {
		t.Fatalf("install grant source = %+v, want the reachable fallback pinned", cfg.PVEInstances)
	}
	var response AutoRegisterResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Host != "https://10.0.0.2:8006" {
		t.Fatalf("response host = %q, want the reachable fallback", response.Host)
	}
	recordedKey := proxmoxInstallRegistrationTypedKey(proxmoxInstallRegistrationHostKey, "pve")
	if got := cfg.APITokens[0].Metadata[recordedKey]; got != "https://10.0.0.2:8006" {
		t.Fatalf("grant recorded %q, want the registered address", got)
	}
}

// A malformed body is logged by size only: it may carry a setup token.
func TestAutoRegisterParseFailuresDoNotLogTheBody(t *testing.T) {
	const secret = "setup-token-must-not-be-logged-0123456789"
	body := `{"authToken":"` + secret + `","type":"pve","host":`
	for _, tc := range []struct {
		path    string
		message string
		handle  func(*ConfigHandlers, http.ResponseWriter, *http.Request)
	}{
		{"/api/auto-register", "Failed to parse auto-register request", (*ConfigHandlers).HandleAutoRegister},
		{"/api/auto-unregister", "Failed to parse auto-unregister request", (*ConfigHandlers).HandleAutoUnregister},
	} {
		t.Run(strings.TrimPrefix(tc.path, "/api/"), func(t *testing.T) {
			id, lines, _ := logging.GetBroadcaster().Subscribe()
			defer logging.GetBroadcaster().Unsubscribe(id)
			handler := newTestConfigHandlers(t, &config.Config{DataPath: t.TempDir()})
			req := httptest.NewRequest(http.MethodPost, tc.path, bytes.NewReader([]byte(body)))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			tc.handle(handler, rec, req)

			var logs strings.Builder
		drain:
			for {
				select {
				case line := <-lines:
					logs.WriteString(line)
				default:
					break drain
				}
			}

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("%s status = %d, want 400", tc.path, rec.Code)
			}
			output := logs.String()
			if !strings.Contains(output, tc.message) {
				t.Fatalf("%s did not log the parse failure; logs: %s", tc.path, output)
			}
			if strings.Contains(output, secret) || strings.Contains(rec.Body.String(), secret) {
				t.Fatalf("%s leaked the request body; logs: %s", tc.path, output)
			}
		})
	}
}
