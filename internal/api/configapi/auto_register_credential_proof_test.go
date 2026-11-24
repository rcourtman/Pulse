package configapi

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

// acceptAutoRegisterCredentials replaces the credential check with Proxmox
// APIs that accept, per address, only the token value listed for it. It
// returns the addresses Pulse presented a token to.
func acceptAutoRegisterCredentials(t *testing.T, accepted map[string]string) func() []proxmox.ClientConfig {
	t.Helper()
	var mu sync.Mutex
	var presented []proxmox.ClientConfig
	original := verifyAutoRegisterCredential
	verifyAutoRegisterCredential = func(_ context.Context, _ string, clientConfig proxmox.ClientConfig) error {
		mu.Lock()
		presented = append(presented, clientConfig)
		mu.Unlock()
		if value, ok := accepted[clientConfig.Host]; ok && value == clientConfig.TokenValue {
			return nil
		}
		return errors.New("authentication failed")
	}
	t.Cleanup(func() { verifyAutoRegisterCredential = original })
	return func() []proxmox.ClientConfig {
		mu.Lock()
		defer mu.Unlock()
		return append([]proxmox.ClientConfig(nil), presented...)
	}
}

// hostInstallGrant is the metadata of a fresh host install token.
func hostInstallGrant() map[string]string {
	return map[string]string{
		"install_type": agentInstallTypeHost,
		"issued_via":   agentInstallIssuedViaConfig,
	}
}

// An unspent install grant sits on every host install token, Docker-only
// hosts included, and which saved connection a request matches is the caller's
// own word about its name or address. The grant therefore changes an existing
// connection only to a token that connection accepts, asked at the connection's
// saved address and never at one the caller chose.
func TestInstallGrantChangesAnExistingConnectionOnlyToATokenItAccepts(t *testing.T) {
	const attacker = "http://192.0.2.66:8006"
	for name, tc := range map[string]struct {
		request   AutoRegisterRequest
		wantAsked string
	}{
		"pve by name and token id": {
			request: AutoRegisterRequest{
				Type: "pve", Host: attacker, ServerName: "pve-real", Source: "agent",
				TokenID: "pulse-monitor@pve!pulse-real", TokenValue: "made-up",
			},
			wantAsked: "https://pve-real.local:8006",
		},
		"pve by address": {
			request: AutoRegisterRequest{
				Type: "pve", Host: "https://pve-real.local:8006", ServerName: "web1", Source: "agent",
				TokenID: "pulse-monitor@pve!pulse-web1", TokenValue: "made-up",
			},
			wantAsked: "https://pve-real.local:8006",
		},
		"pve cluster member by name": {
			request: AutoRegisterRequest{
				Type: "pve", Host: attacker, ServerName: "pve2", Source: "agent",
				TokenID: "pulse-monitor@pve!pulse-other", TokenValue: "made-up",
			},
			wantAsked: "https://pve1.local:8006",
		},
		"pbs by name and token id": {
			request: AutoRegisterRequest{
				Type: "pbs", Host: "http://192.0.2.66:8007", ServerName: "pbs-real", Source: "agent",
				TokenID: "pulse-monitor@pbs!pulse-real", TokenValue: "made-up",
			},
			wantAsked: "https://pbs-real.local:8007",
		},
		"pbs by address": {
			request: AutoRegisterRequest{
				Type: "pbs", Host: "https://pbs-real.local:8007", ServerName: "web1", Source: "agent",
				TokenID: "pulse-monitor@pbs!pulse-web1", TokenValue: "made-up",
			},
			wantAsked: "https://pbs-real.local:8007",
		},
	} {
		t.Run(name, func(t *testing.T) {
			rawToken := "docker-host-install.12345678"
			handler, cfg := agentAuthorityFixture(t, rawToken, hostInstallGrant())
			asked := acceptAutoRegisterCredentials(t, map[string]string{
				attacker:                 "made-up",
				"http://192.0.2.66:8007": "made-up",
			})
			pve, pbs := snapshotAutoRegisterSources(cfg)

			rec := runAgentAutoRegister(t, handler, rawToken, tc.request)
			if rec.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want 403, body=%s", rec.Code, rec.Body.String())
			}
			assertAutoRegisterSourcesUnchanged(t, name, cfg, pve, pbs)
			got := asked()
			if len(got) != 1 || got[0].Host != tc.wantAsked || got[0].TokenValue != "made-up" {
				t.Fatalf("token presented to %+v, want only the saved connection %q", got, tc.wantAsked)
			}
		})
	}
}

// A grant still adopts the connection of the host its agent runs on: that
// host accepts the token the agent minted there. Without a certificate pinned
// for the connection nothing shows that another address is the same host, so
// the saved address stays.
func TestInstallGrantAdoptsAConnectionThatAcceptsItsToken(t *testing.T) {
	rawToken := "pve-real-install.12345678"
	handler, cfg := agentAuthorityFixture(t, rawToken, hostInstallGrant())
	acceptAutoRegisterCredentials(t, map[string]string{"https://pve-real.local:8006": "fresh-secret"})
	fetchTLSFingerprint = func(string) (string, error) { return "AA:AA", nil }

	rec := runAgentAutoRegister(t, handler, rawToken, AutoRegisterRequest{
		Type: "pve", Host: "https://192.0.2.50:8006", ServerName: "pve-real", Source: "agent",
		TokenID: "pulse-monitor@pve!pulse-real", TokenValue: "fresh-secret",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	got := cfg.PVEInstances[0]
	if got.TokenValue != "fresh-secret" {
		t.Fatalf("adopted connection token = %q, want the accepted token", got.TokenValue)
	}
	if got.Host != "https://pve-real.local:8006" || got.Fingerprint != "" {
		t.Fatalf("unpinned connection moved or re-pinned: host %q pin %q", got.Host, got.Fingerprint)
	}

	key := proxmoxInstallRegistrationTypedKey(proxmoxInstallRegistrationHostKey, "pve")
	if got := cfg.APITokens[0].Metadata[key]; got != "https://pve-real.local:8006" {
		t.Fatalf("%s = %q, want the adopted connection's saved address", key, got)
	}
	acceptAutoRegisterCredentials(t, map[string]string{"https://pve-real.local:8006": "rotated-secret"})
	rec = runAgentAutoRegister(t, handler, rawToken, AutoRegisterRequest{
		Type: "pve", Host: "https://192.0.2.50:8006", ServerName: "pve-real", Source: "agent",
		TokenID: "pulse-monitor@pve!pulse-real", TokenValue: "rotated-secret",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("rotation after the grant was spent: status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if got := cfg.PVEInstances[0]; got.TokenValue != "rotated-secret" || got.Host != "https://pve-real.local:8006" {
		t.Fatalf("rotation saved token %q at %q", got.TokenValue, got.Host)
	}
}

// A node that changed address keeps its certificate. A grant follows it there
// when the new address presents the pinned certificate and accepts the token.
func TestInstallGrantFollowsThePinnedCertificateToANewAddress(t *testing.T) {
	const moved = "https://192.0.2.50:8006"
	for name, tc := range map[string]struct {
		accepted string
		wantCode int
		wantHost string
	}{
		"token accepted": {accepted: "fresh-secret", wantCode: http.StatusOK, wantHost: moved},
		"token refused":  {accepted: "another-secret", wantCode: http.StatusForbidden, wantHost: "https://pve-real.local:8006"},
	} {
		t.Run(name, func(t *testing.T) {
			rawToken := "pve-real-install.12345678"
			handler, cfg := agentAuthorityFixture(t, rawToken, hostInstallGrant())
			cfg.PVEInstances[0].Fingerprint = "aa:aa"
			cfg.PVEInstances[0].VerifySSL = true
			asked := acceptAutoRegisterCredentials(t, map[string]string{moved: tc.accepted})
			fetchTLSFingerprint = func(string) (string, error) { return "AA:AA", nil }

			rec := runAgentAutoRegister(t, handler, rawToken, AutoRegisterRequest{
				Type: "pve", Host: moved, ServerName: "pve-real", Source: "agent",
				TokenID: "pulse-monitor@pve!pulse-real", TokenValue: "fresh-secret",
			})
			if rec.Code != tc.wantCode {
				t.Fatalf("status = %d, want %d, body=%s", rec.Code, tc.wantCode, rec.Body.String())
			}
			if got := cfg.PVEInstances[0].Host; got != tc.wantHost {
				t.Fatalf("connection address = %q, want %q", got, tc.wantHost)
			}
			got := asked()
			if len(got) != 1 || got[0].Host != moved || !config.ProxmoxFingerprintsMatch(got[0].Fingerprint, "aa:aa") || !got[0].VerifySSL {
				t.Fatalf("token presented to %+v, want %q behind the pinned certificate", got, moved)
			}
		})
	}
}

// The registration check spends a grant on an existing source and records its
// address on the token without seeing any Proxmox credential. The recorded
// address admits later rotations, which still have to be accepted by the
// connection.
func TestRecordedAddressRotatesOnlyToATokenTheConnectionAccepts(t *testing.T) {
	rawToken := "docker-host-install.12345678"
	handler, cfg := agentAuthorityFixture(t, rawToken, hostInstallGrant())
	acceptAutoRegisterCredentials(t, map[string]string{"https://pve-real.local:8006": "fresh-secret"})

	rec := runAgentAutoRegister(t, handler, rawToken, AutoRegisterRequest{
		Type: "pve", Host: "https://pve-real.local:8006", ServerName: "web1", Source: "agent", CheckRegistration: true,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("check status = %d, body=%s", rec.Code, rec.Body.String())
	}
	key := proxmoxInstallRegistrationTypedKey(proxmoxInstallRegistrationHostKey, "pve")
	if got := cfg.APITokens[0].Metadata[key]; got != "https://pve-real.local:8006" {
		t.Fatalf("%s = %q, want the matched source address", key, got)
	}
	pve, pbs := snapshotAutoRegisterSources(cfg)

	completion := AutoRegisterRequest{
		Type: "pve", Host: "https://pve-real.local:8006", ServerName: "web1", Source: "agent",
		TokenID: "pulse-monitor@pve!pulse-web1", TokenValue: "made-up",
	}
	rec = runAgentAutoRegister(t, handler, rawToken, completion)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("made-up token status = %d, want 403, body=%s", rec.Code, rec.Body.String())
	}
	assertAutoRegisterSourcesUnchanged(t, "made-up token", cfg, pve, pbs)

	completion.TokenValue = "fresh-secret"
	rec = runAgentAutoRegister(t, handler, rawToken, completion)
	if rec.Code != http.StatusOK {
		t.Fatalf("accepted token status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if got := cfg.PVEInstances[0].TokenValue; got != "fresh-secret" {
		t.Fatalf("accepted token not saved: %q", got)
	}
}

// Re-locating the target slot after credential verification prevents a
// concurrent node deletion from dropping the rotated token or writing it onto
// a shifted neighbor.
func TestAgentTokenAutoRegisterRelocatesSlotAfterConcurrentDeletion(t *testing.T) {
	rawToken := "pve-second-agent.12345678"
	handler, cfg := agentAuthorityFixture(t, rawToken, map[string]string{
		proxmoxInstallRegistrationTypedKey(proxmoxInstallRegistrationHostKey, "pve"): "https://pve-second.local:8006",
	})
	cfg.PVEInstances = append(cfg.PVEInstances, config.PVEInstance{
		Name:       "pve-second",
		Host:       "https://pve-second.local:8006",
		TokenName:  "pulse-monitor@pve!pulse-second",
		TokenValue: "old-secret",
	})

	original := verifyAutoRegisterCredential
	verifyAutoRegisterCredential = func(_ context.Context, _ string, _ proxmox.ClientConfig) error {
		// Simulate an administrator removing an earlier connection while the
		// credential probe is in flight.
		cfg.PVEInstances = append([]config.PVEInstance(nil), cfg.PVEInstances[1:]...)
		return nil
	}
	t.Cleanup(func() { verifyAutoRegisterCredential = original })

	rec := runAgentAutoRegister(t, handler, rawToken, AutoRegisterRequest{
		Type: "pve", Host: "https://pve-second.local:8006", ServerName: "pve-second", Source: "agent",
		TokenID: "pulse-monitor@pve!pulse-second", TokenValue: "rotated-secret",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	var found bool
	for _, instance := range cfg.PVEInstances {
		if instance.Name == "pve-second" {
			found = true
			if instance.TokenValue != "rotated-secret" {
				t.Fatalf("pve-second token = %q, want rotated-secret", instance.TokenValue)
			}
		}
	}
	if !found {
		t.Fatal("pve-second missing after concurrent removal of earlier instance")
	}
}
