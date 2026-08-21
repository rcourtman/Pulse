package configapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// Issue #1644: the Settings > Infrastructure installer mints generic host
// install tokens, while install.sh auto-detects Proxmox on the target machine
// and presents type "pve"/"pbs" to /api/auto-register. The bootstrap grant
// must accept a host-issued install token for a canonical Proxmox type, while
// staying one-shot per type and hostname-bound.
//
// A host with both PVE and PBS installed is an officially supported deployment
// and the agent's RunAll registers each product in turn from the one install
// token, so the grant is consumed per canonical type: one PVE create and one
// PBS create, each still one-shot, both under the token's single mint-age TTL
// and its single first-use hostname binding.

// assertProxmoxGrantConsumedTypes fails unless the token's consumed-type ledger
// is exactly want.
func assertProxmoxGrantConsumedTypes(t *testing.T, record *config.APITokenRecord, want ...string) {
	t.Helper()
	consumed, tracked := proxmoxInstallRegistrationConsumedTypeSet(record)
	if !tracked && len(want) > 0 {
		t.Fatalf("token has no per-type consumption ledger (%s=%q); want %v",
			proxmoxInstallRegistrationConsumedTypesKey,
			record.Metadata[proxmoxInstallRegistrationConsumedTypesKey],
			want)
	}
	if len(consumed) != len(want) {
		t.Fatalf("consumed types = %v, want %v", consumed, want)
	}
	for _, nodeType := range want {
		if _, ok := consumed[nodeType]; !ok {
			t.Fatalf("consumed types = %v, want %v", consumed, want)
		}
	}
}

func mintHostInstallToken(t *testing.T, handler *ConfigHandlers) string {
	t.Helper()
	installReq := httptest.NewRequest(
		http.MethodPost,
		"/api/agent-install-command",
		strings.NewReader(`{"type":"host","name":"issue-1644-host"}`),
	)
	installReq.Host = "pulse.example:7655"
	installRec := httptest.NewRecorder()
	handler.HandleAgentInstallCommand(installRec, installReq)
	if installRec.Code != http.StatusOK {
		t.Fatalf("host install token mint status = %d, body=%s", installRec.Code, installRec.Body.String())
	}
	var install AgentInstallCommandResponse
	if err := json.Unmarshal(installRec.Body.Bytes(), &install); err != nil {
		t.Fatalf("decode host install token response: %v", err)
	}
	if strings.TrimSpace(install.Token) == "" {
		t.Fatal("host install token mint omitted runtime token")
	}
	return install.Token
}

func TestIssue1644HostInstallTokenBootstrapsDetectedProxmoxSource(t *testing.T) {
	stubAutoRegisterNetworkDeps(t)

	cfg := &config.Config{
		DataPath: t.TempDir(),
		AuthUser: "admin",
		AuthPass: "hashed-password",
	}
	handler := newTestConfigHandlers(t, cfg)
	rawToken := mintHostInstallToken(t, handler)
	if len(cfg.APITokens) != 1 {
		t.Fatalf("API tokens = %d, want 1", len(cfg.APITokens))
	}
	if got := cfg.APITokens[0].Metadata["install_type"]; got != agentInstallTypeHost {
		t.Fatalf("minted install_type = %q, want %q", got, agentInstallTypeHost)
	}

	// The agent's pre-registration check must report canRegister=true so
	// runForType proceeds instead of aborting with a blocked registration.
	checkRec := runAgentAutoRegister(t, handler, rawToken, AutoRegisterRequest{
		Type:              "pve",
		Host:              "https://pve-host.local:8006",
		ServerName:        "pve-host",
		Source:            "agent",
		CheckRegistration: true,
	})
	if checkRec.Code != http.StatusOK {
		t.Fatalf("registration check status = %d, body=%s", checkRec.Code, checkRec.Body.String())
	}
	var check autoRegisterCheckResponse
	if err := json.Unmarshal(checkRec.Body.Bytes(), &check); err != nil {
		t.Fatalf("decode registration check: %v", err)
	}
	if !check.CanRegister {
		t.Fatalf("host install token presenting pve reported canRegister=%v, want true", check.CanRegister)
	}

	registerRec := runAgentAutoRegister(t, handler, rawToken, AutoRegisterRequest{
		Type:       "pve",
		Host:       "https://pve-host.local:8006",
		TokenID:    "pulse-monitor@pve!pulse-pve-host",
		TokenValue: "proxmox-secret",
		ServerName: "pve-host",
		Source:     "agent",
	})
	if registerRec.Code != http.StatusOK {
		t.Fatalf("host-token pve registration status = %d, body=%s", registerRec.Code, registerRec.Body.String())
	}
	if len(cfg.PVEInstances) != 1 {
		t.Fatalf("PVE instances = %d, want 1", len(cfg.PVEInstances))
	}
	if !strings.EqualFold(cfg.APITokens[0].Metadata[proxmoxInstallRegistrationCompletedKey], "true") {
		t.Fatal("host-token pve registration did not consume the install grant")
	}
	assertProxmoxGrantConsumedTypes(t, &cfg.APITokens[0], "pve")

	// The pve grant is spent: the same token cannot create a second pve source.
	reuseRec := runAgentAutoRegister(t, handler, rawToken, AutoRegisterRequest{
		Type:       "pve",
		Host:       "https://pve-second.local:8006",
		TokenID:    "pulse-monitor@pve!pulse-pve-second",
		TokenValue: "other-proxmox-secret",
		ServerName: "pve-host",
		Source:     "agent",
	})
	if reuseRec.Code != http.StatusForbidden {
		t.Fatalf("consumed pve grant reuse status = %d, want 403; body=%s", reuseRec.Code, reuseRec.Body.String())
	}
	if len(cfg.PVEInstances) != 1 {
		t.Fatalf("PVE instances = %d, want 1 after the consumed pve grant was refused", len(cfg.PVEInstances))
	}
}

// TestIssue1644CombinedProxmoxHostRegistersBothTypes covers the deployment the
// docs call officially supported: PVE and PBS on one machine, discovered by the
// agent's RunAll and registered in sequence from a single host install token.
// Consumption used to be recorded per token, so the PBS leg was refused and the
// installer printed an ERROR banner over a perfectly good PVE registration.
func TestIssue1644CombinedProxmoxHostRegistersBothTypes(t *testing.T) {
	stubAutoRegisterNetworkDeps(t)

	cfg := &config.Config{
		DataPath: t.TempDir(),
		AuthUser: "admin",
		AuthPass: "hashed-password",
	}
	handler := newTestConfigHandlers(t, cfg)
	rawToken := mintHostInstallToken(t, handler)

	pveRec := runAgentAutoRegister(t, handler, rawToken, AutoRegisterRequest{
		Type:       "pve",
		Host:       "https://combined.local:8006",
		TokenID:    "pulse-monitor@pve!pulse-combined",
		TokenValue: "pve-secret",
		ServerName: "combined",
		Source:     "agent",
	})
	if pveRec.Code != http.StatusOK {
		t.Fatalf("combined-host pve registration status = %d, body=%s", pveRec.Code, pveRec.Body.String())
	}
	assertProxmoxGrantConsumedTypes(t, &cfg.APITokens[0], "pve")

	// The agent's second RunAll leg checks first, exactly as runForType does.
	checkRec := runAgentAutoRegister(t, handler, rawToken, AutoRegisterRequest{
		Type:              "pbs",
		Host:              "https://combined.local:8007",
		ServerName:        "combined",
		Source:            "agent",
		CheckRegistration: true,
	})
	if checkRec.Code != http.StatusOK {
		t.Fatalf("combined-host pbs check status = %d, body=%s", checkRec.Code, checkRec.Body.String())
	}
	var check autoRegisterCheckResponse
	if err := json.Unmarshal(checkRec.Body.Bytes(), &check); err != nil {
		t.Fatalf("decode pbs registration check: %v", err)
	}
	if !check.CanRegister {
		t.Fatal("pbs leg of a combined host reported canRegister=false after the pve leg consumed its grant")
	}

	pbsRec := runAgentAutoRegister(t, handler, rawToken, AutoRegisterRequest{
		Type:       "pbs",
		Host:       "https://combined.local:8007",
		TokenID:    "pulse-monitor@pbs!pulse-combined",
		TokenValue: "pbs-secret",
		ServerName: "combined",
		Source:     "agent",
	})
	if pbsRec.Code != http.StatusOK {
		t.Fatalf("combined-host pbs registration status = %d, body=%s", pbsRec.Code, pbsRec.Body.String())
	}
	if len(cfg.PVEInstances) != 1 || len(cfg.PBSInstances) != 1 {
		t.Fatalf("combined host sources = %d PVE / %d PBS, want 1 each", len(cfg.PVEInstances), len(cfg.PBSInstances))
	}
	assertProxmoxGrantConsumedTypes(t, &cfg.APITokens[0], "pve", "pbs")

	// Both per-type completion blocks survive; neither leg overwrote the other.
	metadata := cfg.APITokens[0].Metadata
	for nodeType, wantHost := range map[string]string{
		"pve": "https://combined.local:8006",
		"pbs": "https://combined.local:8007",
	} {
		key := proxmoxInstallRegistrationTypedKey(proxmoxInstallRegistrationHostKey, nodeType)
		if got := metadata[key]; got != wantHost {
			t.Fatalf("%s = %q, want %q", key, got, wantHost)
		}
	}

	// Every grant is now spent: neither type can create a second source.
	for _, spent := range []AutoRegisterRequest{
		{
			Type: "pve", Host: "https://combined-2.local:8006",
			TokenID: "pulse-monitor@pve!pulse-combined-2", TokenValue: "pve-secret-2",
			ServerName: "combined", Source: "agent",
		},
		{
			Type: "pbs", Host: "https://combined-2.local:8007",
			TokenID: "pulse-monitor@pbs!pulse-combined-2", TokenValue: "pbs-secret-2",
			ServerName: "combined", Source: "agent",
		},
	} {
		rec := runAgentAutoRegister(t, handler, rawToken, spent)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("second %s create status = %d, want 403; body=%s", spent.Type, rec.Code, rec.Body.String())
		}
	}
	if len(cfg.PVEInstances) != 1 || len(cfg.PBSInstances) != 1 {
		t.Fatalf("spent grants created extra sources: %d PVE / %d PBS", len(cfg.PVEInstances), len(cfg.PBSInstances))
	}
}

// TestIssue1644PerTypeGrantKeepsSingleHostnameBinding pins that per-type
// consumption did not turn one hostname binding into two. The first type to
// register pins the hostname for the whole token, so the second type cannot be
// redirected at a different machine.
func TestIssue1644PerTypeGrantKeepsSingleHostnameBinding(t *testing.T) {
	stubAutoRegisterNetworkDeps(t)

	cfg := &config.Config{
		DataPath: t.TempDir(),
		AuthUser: "admin",
		AuthPass: "hashed-password",
	}
	handler := newTestConfigHandlers(t, cfg)
	rawToken := mintHostInstallToken(t, handler)

	pveRec := runAgentAutoRegister(t, handler, rawToken, AutoRegisterRequest{
		Type:       "pve",
		Host:       "https://bound-combined.local:8006",
		TokenID:    "pulse-monitor@pve!pulse-bound-combined",
		TokenValue: "pve-secret",
		ServerName: "bound-combined",
		Source:     "agent",
	})
	if pveRec.Code != http.StatusOK {
		t.Fatalf("pve registration status = %d, body=%s", pveRec.Code, pveRec.Body.String())
	}
	if got := cfg.APITokens[0].Metadata["bound_hostname"]; got != "bound-combined" {
		t.Fatalf("bound hostname = %q, want %q", got, "bound-combined")
	}

	otherHostRec := runAgentAutoRegister(t, handler, rawToken, AutoRegisterRequest{
		Type:       "pbs",
		Host:       "https://elsewhere.local:8007",
		TokenID:    "pulse-monitor@pbs!pulse-elsewhere",
		TokenValue: "pbs-secret",
		ServerName: "elsewhere",
		Source:     "agent",
	})
	if otherHostRec.Code != http.StatusForbidden {
		t.Fatalf("pbs grant on a different hostname status = %d, want 403; body=%s", otherHostRec.Code, otherHostRec.Body.String())
	}
	if len(cfg.PBSInstances) != 0 {
		t.Fatalf("PBS instances = %d, want 0 after a cross-hostname refusal", len(cfg.PBSInstances))
	}
	if got := cfg.APITokens[0].Metadata["bound_hostname"]; got != "bound-combined" {
		t.Fatalf("bound hostname after refusal = %q, want %q", got, "bound-combined")
	}

	// The pbs grant is live, just pinned to the bound host — proving the
	// refusal above came from the binding and not from a spent grant.
	boundRec := runAgentAutoRegister(t, handler, rawToken, AutoRegisterRequest{
		Type:       "pbs",
		Host:       "https://bound-combined.local:8007",
		TokenID:    "pulse-monitor@pbs!pulse-bound-combined",
		TokenValue: "pbs-secret",
		ServerName: "bound-combined",
		Source:     "agent",
	})
	if boundRec.Code != http.StatusOK {
		t.Fatalf("pbs registration on the bound host status = %d, body=%s", boundRec.Code, boundRec.Body.String())
	}
	assertProxmoxGrantConsumedTypes(t, &cfg.APITokens[0], "pve", "pbs")
}

// TestIssue1644LegacyCompletionMarkerConsumesEveryType covers upgrade: a token
// spent under the old per-token semantics carries only
// proxmox_registration_completed=true. Reading that as "pve is spent, pbs is
// free" would hand every already-used install token in the field a fresh
// create, so the bare marker still means the whole token is spent.
func TestIssue1644LegacyCompletionMarkerConsumesEveryType(t *testing.T) {
	stubAutoRegisterNetworkDeps(t)

	rawToken := "issue-1644-legacy-consumed.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeAgentReport}, map[string]string{
		"install_type":                         agentInstallTypeHost,
		"issued_via":                           agentInstallIssuedViaConfig,
		"bound_hostname":                       "legacy-host",
		proxmoxInstallRegistrationCompletedKey: "true",
		proxmoxInstallRegistrationTypeKey:      "pve",
		proxmoxInstallRegistrationHostnameKey:  "legacy-host",
	})
	if got := record.Metadata[proxmoxInstallRegistrationConsumedTypesKey]; got != "" {
		t.Fatalf("legacy fixture carries a per-type ledger (%q); the upgrade assertion would be vacuous", got)
	}

	for _, nodeType := range []string{"pve", "pbs"} {
		req := &AutoRegisterRequest{
			Type:       nodeType,
			Host:       "https://legacy-host.local:8006",
			ServerName: "legacy-host",
			Source:     "agent",
		}
		if canBootstrapProxmoxInstallRegistration(&record, req) {
			t.Fatalf("legacy completed token still authorized a %s bootstrap", nodeType)
		}
	}

	cfg := &config.Config{
		DataPath:  t.TempDir(),
		APITokens: []config.APITokenRecord{record},
	}
	handler := newTestConfigHandlers(t, cfg)
	rec := runAgentAutoRegister(t, handler, rawToken, AutoRegisterRequest{
		Type:       "pbs",
		Host:       "https://legacy-host.local:8007",
		TokenID:    "pulse-monitor@pbs!pulse-legacy-host",
		TokenValue: "pbs-secret",
		ServerName: "legacy-host",
		Source:     "agent",
	})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("legacy consumed token pbs registration status = %d, want 403; body=%s", rec.Code, rec.Body.String())
	}
	if len(cfg.PBSInstances) != 0 {
		t.Fatalf("PBS instances = %d, want 0 for a legacy-consumed token", len(cfg.PBSInstances))
	}
}

// TestIssue1644FailedSourceSaveRollsBackOnlyTheConsumedType pins that the
// consume-before-persist rollback is scoped to the type being consumed. A PBS
// save failure must put the PBS grant back without resurrecting the PVE grant
// that already produced a persisted source.
func TestIssue1644FailedSourceSaveRollsBackOnlyTheConsumedType(t *testing.T) {
	stubAutoRegisterNetworkDeps(t)

	dataPath := t.TempDir()
	cfg := &config.Config{
		DataPath: dataPath,
		AuthUser: "admin",
		AuthPass: "hashed-password",
	}
	handler := newTestConfigHandlers(t, cfg)
	rawToken := mintHostInstallToken(t, handler)

	pveRec := runAgentAutoRegister(t, handler, rawToken, AutoRegisterRequest{
		Type:       "pve",
		Host:       "https://split-rollback.local:8006",
		TokenID:    "pulse-monitor@pve!pulse-split-rollback",
		TokenValue: "pve-secret",
		ServerName: "split-rollback",
		Source:     "agent",
	})
	if pveRec.Code != http.StatusOK {
		t.Fatalf("pve registration status = %d, body=%s", pveRec.Code, pveRec.Body.String())
	}
	assertProxmoxGrantConsumedTypes(t, &cfg.APITokens[0], "pve")

	// Block only the nodes store so the pbs source cannot be persisted.
	nodesFile := filepath.Join(dataPath, "nodes.enc")
	if err := os.RemoveAll(nodesFile); err != nil {
		t.Fatalf("clear nodes config: %v", err)
	}
	if err := os.MkdirAll(nodesFile, 0o755); err != nil {
		t.Fatalf("block nodes config persistence: %v", err)
	}

	pbsRec := runAgentAutoRegister(t, handler, rawToken, AutoRegisterRequest{
		Type:       "pbs",
		Host:       "https://split-rollback.local:8007",
		TokenID:    "pulse-monitor@pbs!pulse-split-rollback",
		TokenValue: "pbs-secret",
		ServerName: "split-rollback",
		Source:     "agent",
	})
	if pbsRec.Code != http.StatusInternalServerError {
		t.Fatalf("blocked pbs source save status = %d, want 500; body=%s", pbsRec.Code, pbsRec.Body.String())
	}

	// The pbs grant came back; the pve grant stayed spent.
	assertProxmoxGrantConsumedTypes(t, &cfg.APITokens[0], "pve")
	if got := cfg.APITokens[0].Metadata[proxmoxInstallRegistrationTypedKey(proxmoxInstallRegistrationHostKey, "pbs")]; got != "" {
		t.Fatalf("rolled-back pbs consumption left completion detail %q behind", got)
	}
	if got := cfg.APITokens[0].Metadata[proxmoxInstallRegistrationTypedKey(proxmoxInstallRegistrationHostKey, "pve")]; got != "https://split-rollback.local:8006" {
		t.Fatalf("pbs rollback disturbed the pve completion detail: %q", got)
	}
	if got := cfg.APITokens[0].Metadata[proxmoxInstallRegistrationTypeKey]; got != "pve" {
		t.Fatalf("pbs rollback left the latest-completion block on %q, want the restored pve block", got)
	}

	persisted, err := config.NewConfigPersistence(dataPath).LoadAPITokens()
	if err != nil {
		t.Fatalf("load persisted API tokens: %v", err)
	}
	if len(persisted) != 1 {
		t.Fatalf("persisted API tokens = %d, want 1", len(persisted))
	}
	assertProxmoxGrantConsumedTypes(t, &persisted[0], "pve")

	// A second pve create is still refused: the rollback restored one type.
	replayRec := runAgentAutoRegister(t, handler, rawToken, AutoRegisterRequest{
		Type:       "pve",
		Host:       "https://split-rollback-2.local:8006",
		TokenID:    "pulse-monitor@pve!pulse-split-rollback-2",
		TokenValue: "pve-secret-2",
		ServerName: "split-rollback",
		Source:     "agent",
	})
	if replayRec.Code != http.StatusForbidden {
		t.Fatalf("pve replay after a pbs rollback status = %d, want 403; body=%s", replayRec.Code, replayRec.Body.String())
	}

	// With the nodes store working again the restored pbs grant completes once.
	if err := os.Remove(nodesFile); err != nil {
		t.Fatalf("unblock nodes config persistence: %v", err)
	}
	retryRec := runAgentAutoRegister(t, handler, rawToken, AutoRegisterRequest{
		Type:       "pbs",
		Host:       "https://split-rollback.local:8007",
		TokenID:    "pulse-monitor@pbs!pulse-split-rollback",
		TokenValue: "pbs-secret",
		ServerName: "split-rollback",
		Source:     "agent",
	})
	if retryRec.Code != http.StatusOK {
		t.Fatalf("pbs retry after restored persistence status = %d, body=%s", retryRec.Code, retryRec.Body.String())
	}
	assertProxmoxGrantConsumedTypes(t, &cfg.APITokens[0], "pve", "pbs")
}

func TestIssue1644HostInstallTokenGrantStaysHostnameBound(t *testing.T) {
	stubAutoRegisterNetworkDeps(t)

	rawToken := "issue-1644-host-bound.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeAgentReport}, map[string]string{
		"install_type": agentInstallTypeHost,
		"issued_via":   agentInstallIssuedViaConfig,
	})
	cfg := &config.Config{
		DataPath:  t.TempDir(),
		APITokens: []config.APITokenRecord{record},
	}
	handler := newTestConfigHandlers(t, cfg)

	// Bind the grant WITHOUT consuming it: the agent's pre-registration check
	// binds the first presenting hostname and leaves the grant available for
	// the completion that follows. Completing a registration first would make
	// the cross-host assertion below pass on the spent-grant path instead of
	// the hostname binding, which is the thing under test.
	checkRec := runAgentAutoRegister(t, handler, rawToken, AutoRegisterRequest{
		Type:              "pve",
		Host:              "https://pve-bound.local:8006",
		ServerName:        "pve-bound",
		Source:            "agent",
		CheckRegistration: true,
	})
	if checkRec.Code != http.StatusOK {
		t.Fatalf("binding registration check status = %d, body=%s", checkRec.Code, checkRec.Body.String())
	}
	if got := cfg.APITokens[0].Metadata["bound_hostname"]; got != "pve-bound" {
		t.Fatalf("bound hostname = %q, want %q", got, "pve-bound")
	}
	if got := cfg.APITokens[0].Metadata[proxmoxInstallRegistrationCompletedKey]; strings.EqualFold(got, "true") {
		t.Fatalf("registration check consumed the grant (%s=%q); the cross-host assertion below would be vacuous", proxmoxInstallRegistrationCompletedKey, got)
	}

	otherHostRec := runAgentAutoRegister(t, handler, rawToken, AutoRegisterRequest{
		Type:       "pve",
		Host:       "https://pve-other.local:8006",
		TokenID:    "pulse-monitor@pve!pulse-pve-other",
		TokenValue: "proxmox-secret",
		ServerName: "pve-other",
		Source:     "agent",
	})
	if otherHostRec.Code != http.StatusForbidden {
		t.Fatalf("cross-host host-token registration status = %d, want 403; body=%s", otherHostRec.Code, otherHostRec.Body.String())
	}
	if len(cfg.PVEInstances) != 0 {
		t.Fatalf("PVE instances = %d, want 0 after cross-host rejection of an unconsumed grant", len(cfg.PVEInstances))
	}
	if got := cfg.APITokens[0].Metadata["bound_hostname"]; got != "pve-bound" {
		t.Fatalf("bound hostname after rejection = %q, want %q", got, "pve-bound")
	}

	// The grant itself is still live: the bound host completes normally. This
	// proves the rejection above came from the hostname binding and not from a
	// consumed or otherwise unusable grant.
	boundHostRec := runAgentAutoRegister(t, handler, rawToken, AutoRegisterRequest{
		Type:       "pve",
		Host:       "https://pve-bound.local:8006",
		TokenID:    "pulse-monitor@pve!pulse-pve-bound",
		TokenValue: "proxmox-secret",
		ServerName: "pve-bound",
		Source:     "agent",
	})
	if boundHostRec.Code != http.StatusOK {
		t.Fatalf("bound-host registration status = %d, body=%s", boundHostRec.Code, boundHostRec.Body.String())
	}
	if len(cfg.PVEInstances) != 1 {
		t.Fatalf("PVE instances = %d, want 1 after the bound host completes", len(cfg.PVEInstances))
	}
}

func TestIssue1644HostInstallTokenBootstrapGrantExpiresAfterTTL(t *testing.T) {
	stubAutoRegisterNetworkDeps(t)

	rawToken := "issue-1644-host-expired.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeAgentReport}, map[string]string{
		"install_type": agentInstallTypeHost,
		"issued_via":   agentInstallIssuedViaConfig,
		agentInstallTokenIssuedAtKey: time.Now().UTC().
			Add(-proxmoxInstallBootstrapGrantTTL - time.Hour).
			Format(time.RFC3339),
	})
	cfg := &config.Config{
		DataPath:  t.TempDir(),
		APITokens: []config.APITokenRecord{record},
	}
	handler := newTestConfigHandlers(t, cfg)

	req := &AutoRegisterRequest{
		Type:       "pve",
		Host:       "https://pve-stale.local:8006",
		ServerName: "pve-stale",
		Source:     "agent",
	}
	if !proxmoxInstallGrantEligible(&record, req) {
		t.Fatal("expired-grant fixture is not otherwise eligible; the TTL assertion would be vacuous")
	}
	if !proxmoxInstallGrantExpiredForRequest(&record, req) {
		t.Fatal("grant minted beyond the TTL was not reported as expired")
	}
	if canBootstrapProxmoxInstallRegistration(&record, req) {
		t.Fatal("grant minted beyond the TTL still authorized a bootstrap")
	}

	checkRec := runAgentAutoRegister(t, handler, rawToken, AutoRegisterRequest{
		Type:              "pve",
		Host:              "https://pve-stale.local:8006",
		ServerName:        "pve-stale",
		Source:            "agent",
		CheckRegistration: true,
	})
	if checkRec.Code != http.StatusOK {
		t.Fatalf("expired-grant registration check status = %d, body=%s", checkRec.Code, checkRec.Body.String())
	}
	var check autoRegisterCheckResponse
	if err := json.Unmarshal(checkRec.Body.Bytes(), &check); err != nil {
		t.Fatalf("decode registration check: %v", err)
	}
	if check.CanRegister {
		t.Fatal("expired grant reported canRegister=true")
	}
	if got := cfg.APITokens[0].Metadata["bound_hostname"]; got != "" {
		t.Fatalf("expired grant bound a hostname (%q); it must not take the binding path", got)
	}

	registerRec := runAgentAutoRegister(t, handler, rawToken, AutoRegisterRequest{
		Type:       "pve",
		Host:       "https://pve-stale.local:8006",
		TokenID:    "pulse-monitor@pve!pulse-pve-stale",
		TokenValue: "proxmox-secret",
		ServerName: "pve-stale",
		Source:     "agent",
	})
	if registerRec.Code != http.StatusForbidden {
		t.Fatalf("expired-grant registration status = %d, want 403; body=%s", registerRec.Code, registerRec.Body.String())
	}
	if len(cfg.PVEInstances) != 0 {
		t.Fatalf("PVE instances = %d, want 0 for an expired grant", len(cfg.PVEInstances))
	}
}

func TestIssue1644HostInstallTokenBootstrapGrantSurvivesInsideTTL(t *testing.T) {
	stubAutoRegisterNetworkDeps(t)

	rawToken := "issue-1644-host-fresh.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeAgentReport}, map[string]string{
		"install_type": agentInstallTypeHost,
		"issued_via":   agentInstallIssuedViaConfig,
		agentInstallTokenIssuedAtKey: time.Now().UTC().
			Add(-proxmoxInstallBootstrapGrantTTL + time.Hour).
			Format(time.RFC3339),
	})
	cfg := &config.Config{
		DataPath:  t.TempDir(),
		APITokens: []config.APITokenRecord{record},
	}
	handler := newTestConfigHandlers(t, cfg)

	registerRec := runAgentAutoRegister(t, handler, rawToken, AutoRegisterRequest{
		Type:       "pve",
		Host:       "https://pve-fresh.local:8006",
		TokenID:    "pulse-monitor@pve!pulse-pve-fresh",
		TokenValue: "proxmox-secret",
		ServerName: "pve-fresh",
		Source:     "agent",
	})
	if registerRec.Code != http.StatusOK {
		t.Fatalf("in-TTL registration status = %d, body=%s", registerRec.Code, registerRec.Body.String())
	}
	if len(cfg.PVEInstances) != 1 {
		t.Fatalf("PVE instances = %d, want 1 for a grant inside the TTL", len(cfg.PVEInstances))
	}
}

func TestIssue1644InstallTokenMintStampsGrantIssuedAt(t *testing.T) {
	cfg := &config.Config{
		DataPath: t.TempDir(),
		AuthUser: "admin",
		AuthPass: "hashed-password",
	}
	handler := newTestConfigHandlers(t, cfg)
	mintHostInstallToken(t, handler)

	if len(cfg.APITokens) != 1 {
		t.Fatalf("API tokens = %d, want 1", len(cfg.APITokens))
	}
	raw := strings.TrimSpace(cfg.APITokens[0].Metadata[agentInstallTokenIssuedAtKey])
	if raw == "" {
		t.Fatalf("minted install token is missing %q; the bootstrap grant would have no expiry clock", agentInstallTokenIssuedAtKey)
	}
	issuedAt, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		t.Fatalf("parse %s=%q: %v", agentInstallTokenIssuedAtKey, raw, err)
	}
	if delta := time.Since(issuedAt); delta < -time.Minute || delta > time.Minute {
		t.Fatalf("%s = %s is not the mint time (delta %s)", agentInstallTokenIssuedAtKey, raw, delta)
	}
}

// Issue #1644 follow-up: SaveNodesConfig ran before the grant was consumed, so
// a persistently failing token-store write left a created source on disk next
// to a live grant — a repeatable create-a-source primitive. The grant is now
// consumed first and rolled back when the source fails to persist, so the
// failure is atomic: no source, and the grant is still exactly one use.
func TestIssue1644FailedSourceSaveRollsBackGrantConsumption(t *testing.T) {
	stubAutoRegisterNetworkDeps(t)

	dataPath := t.TempDir()
	cfg := &config.Config{
		DataPath: dataPath,
		AuthUser: "admin",
		AuthPass: "hashed-password",
	}
	handler := newTestConfigHandlers(t, cfg)
	rawToken := mintHostInstallToken(t, handler)

	// Block only the nodes store: a directory where the nodes file belongs
	// fails the rename, while API token writes keep working.
	if err := os.MkdirAll(filepath.Join(dataPath, "nodes.enc"), 0o755); err != nil {
		t.Fatalf("block nodes config persistence: %v", err)
	}

	registerRec := runAgentAutoRegister(t, handler, rawToken, AutoRegisterRequest{
		Type:       "pve",
		Host:       "https://pve-rollback.local:8006",
		TokenID:    "pulse-monitor@pve!pulse-pve-rollback",
		TokenValue: "proxmox-secret",
		ServerName: "pve-rollback",
		Source:     "agent",
	})
	if registerRec.Code != http.StatusInternalServerError {
		t.Fatalf("blocked source save status = %d, want 500; body=%s", registerRec.Code, registerRec.Body.String())
	}

	if got := cfg.APITokens[0].Metadata[proxmoxInstallRegistrationCompletedKey]; strings.EqualFold(strings.TrimSpace(got), "true") {
		t.Fatalf("grant stayed consumed (%s=%q) after the source failed to persist", proxmoxInstallRegistrationCompletedKey, got)
	}

	persisted, err := config.NewConfigPersistence(dataPath).LoadAPITokens()
	if err != nil {
		t.Fatalf("load persisted API tokens: %v", err)
	}
	if len(persisted) != 1 {
		t.Fatalf("persisted API tokens = %d, want 1", len(persisted))
	}
	if got := persisted[0].Metadata[proxmoxInstallRegistrationCompletedKey]; strings.EqualFold(strings.TrimSpace(got), "true") {
		t.Fatalf("persisted grant stayed consumed (%s=%q) after the source failed to persist", proxmoxInstallRegistrationCompletedKey, got)
	}

	// With the nodes store working again the same install completes exactly
	// once, and the restored grant is spent by that completion.
	if err := os.Remove(filepath.Join(dataPath, "nodes.enc")); err != nil {
		t.Fatalf("unblock nodes config persistence: %v", err)
	}
	retryRec := runAgentAutoRegister(t, handler, rawToken, AutoRegisterRequest{
		Type:       "pve",
		Host:       "https://pve-rollback.local:8006",
		TokenID:    "pulse-monitor@pve!pulse-pve-rollback",
		TokenValue: "proxmox-secret",
		ServerName: "pve-rollback",
		Source:     "agent",
	})
	if retryRec.Code != http.StatusOK {
		t.Fatalf("retry after restored persistence status = %d, body=%s", retryRec.Code, retryRec.Body.String())
	}
	if !strings.EqualFold(cfg.APITokens[0].Metadata[proxmoxInstallRegistrationCompletedKey], "true") {
		t.Fatal("retry did not consume the restored grant")
	}
}

// Issue #1644 follow-up: with SaveNodesConfig running first, a persistently
// failing token-store write left the source on disk while the grant stayed
// unconsumed — replay the install and every new host URL under the bound
// hostname creates another source. Consuming first means a token-store failure
// aborts before anything is persisted.
func TestIssue1644FailedGrantConsumptionLeavesNoPersistedSource(t *testing.T) {
	stubAutoRegisterNetworkDeps(t)

	dataPath := t.TempDir()
	cfg := &config.Config{
		DataPath: dataPath,
		AuthUser: "admin",
		AuthPass: "hashed-password",
	}
	handler := newTestConfigHandlers(t, cfg)
	rawToken := mintHostInstallToken(t, handler)

	// Bind the hostname while the token store still works, so the failure
	// under test is the grant consumption and not the first-use binding.
	checkRec := runAgentAutoRegister(t, handler, rawToken, AutoRegisterRequest{
		Type:              "pve",
		Host:              "https://pve-replay.local:8006",
		ServerName:        "pve-replay",
		Source:            "agent",
		CheckRegistration: true,
	})
	if checkRec.Code != http.StatusOK {
		t.Fatalf("binding registration check status = %d, body=%s", checkRec.Code, checkRec.Body.String())
	}

	// Block only the token store: a directory where the API token file belongs
	// fails the rename, while the nodes config would still save happily.
	apiTokensFile := filepath.Join(dataPath, "api_tokens.json")
	if err := os.Remove(apiTokensFile); err != nil {
		t.Fatalf("clear API token file: %v", err)
	}
	if err := os.MkdirAll(apiTokensFile, 0o755); err != nil {
		t.Fatalf("block API token persistence: %v", err)
	}

	registerRec := runAgentAutoRegister(t, handler, rawToken, AutoRegisterRequest{
		Type:       "pve",
		Host:       "https://pve-replay.local:8006",
		TokenID:    "pulse-monitor@pve!pulse-pve-replay",
		TokenValue: "proxmox-secret",
		ServerName: "pve-replay",
		Source:     "agent",
	})
	if registerRec.Code != http.StatusInternalServerError {
		t.Fatalf("blocked grant consumption status = %d, want 500; body=%s", registerRec.Code, registerRec.Body.String())
	}
	if _, err := os.Stat(filepath.Join(dataPath, "nodes.enc")); !os.IsNotExist(err) {
		t.Fatalf("source was persisted despite an unconsumable grant (stat nodes.enc err = %v); the install is replayable", err)
	}
	if strings.EqualFold(strings.TrimSpace(cfg.APITokens[0].Metadata[proxmoxInstallRegistrationCompletedKey]), "true") {
		t.Fatal("failed grant consumption left the completion marker behind")
	}
}

func TestIssue1644HostInstallTokenRejectsNonCanonicalType(t *testing.T) {
	rawToken := "issue-1644-host-bad-type.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeAgentReport}, map[string]string{
		"install_type": agentInstallTypeHost,
		"issued_via":   agentInstallIssuedViaConfig,
	})
	req := &AutoRegisterRequest{
		Type:       "host",
		ServerName: "some-host",
		Source:     "agent",
	}
	if canBootstrapProxmoxInstallRegistration(&record, req) {
		t.Fatal("host install token must not hold a bootstrap grant for non-Proxmox types")
	}
}

func runIssue1644AutoRegisterRaw(t *testing.T, handler *ConfigHandlers, rawToken string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/auto-register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Token", rawToken)
	rec := httptest.NewRecorder()
	handler.HandleAutoRegister(rec, req)
	return rec
}

func TestIssue1644TypedInstallTokenStaysPinnedToItsType(t *testing.T) {
	rawToken := "issue-1644-typed-pinned.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeAgentReport}, map[string]string{
		"install_type": "pve",
		"issued_via":   agentInstallIssuedViaConfig,
	})
	cfg := &config.Config{
		DataPath:  t.TempDir(),
		APITokens: []config.APITokenRecord{record},
	}
	handler := newTestConfigHandlers(t, cfg)

	payload, err := json.Marshal(AutoRegisterRequest{
		Type:       "pbs",
		Host:       "https://pbs-pinned.local:8007",
		TokenID:    "pulse-monitor@pbs!pulse-pbs-pinned",
		TokenValue: "proxmox-secret",
		ServerName: "pbs-pinned",
		Source:     "agent",
	})
	if err != nil {
		t.Fatalf("marshal pinned-type payload: %v", err)
	}
	rec := runIssue1644AutoRegisterRaw(t, handler, rawToken, payload)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("pve-typed token presenting pbs status = %d, want 403; body=%s", rec.Code, rec.Body.String())
	}
}
