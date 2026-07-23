package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func runAgentAutoRegister(t *testing.T, handler *ConfigHandlers, rawToken string, payload AutoRegisterRequest) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal auto-register payload: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/auto-register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Token", rawToken)
	rec := httptest.NewRecorder()
	handler.HandleAutoRegister(rec, req)
	return rec
}

func TestAgentInstallCommandFirstRunRegistersProxmoxSource(t *testing.T) {
	stubAutoRegisterNetworkDeps(t)

	cfg := &config.Config{
		DataPath: t.TempDir(),
		AuthUser: "admin",
		AuthPass: "hashed-password",
	}
	handler := newTestConfigHandlers(t, cfg)
	installReq := httptest.NewRequest(
		http.MethodPost,
		"/api/agent-install-command",
		strings.NewReader(`{"type":"pve"}`),
	)
	installReq.Host = "pulse.example:7655"
	installRec := httptest.NewRecorder()
	handler.HandleAgentInstallCommand(installRec, installReq)
	if installRec.Code != http.StatusOK {
		t.Fatalf("agent install command status = %d, body=%s", installRec.Code, installRec.Body.String())
	}
	var install AgentInstallCommandResponse
	if err := json.Unmarshal(installRec.Body.Bytes(), &install); err != nil {
		t.Fatalf("decode agent install command: %v", err)
	}
	if strings.TrimSpace(install.Token) == "" {
		t.Fatal("agent install command omitted runtime token")
	}
	if len(cfg.APITokens) != 1 {
		t.Fatalf("API tokens = %d, want 1", len(cfg.APITokens))
	}

	registerRec := runAgentAutoRegister(t, handler, install.Token, AutoRegisterRequest{
		Type:       "pve",
		Host:       "https://pve-first.local:8006",
		TokenID:    "pulse-monitor@pve!pulse-pve-first",
		TokenValue: "proxmox-secret",
		ServerName: "pve-first",
		Source:     "agent",
	})
	if registerRec.Code != http.StatusOK {
		t.Fatalf("first-run registration status = %d, body=%s", registerRec.Code, registerRec.Body.String())
	}
	if len(cfg.PVEInstances) != 1 {
		t.Fatalf("PVE instances = %d, want 1", len(cfg.PVEInstances))
	}
	if !strings.EqualFold(cfg.APITokens[0].Metadata[proxmoxInstallRegistrationCompletedKey], "true") {
		t.Fatal("first-run registration did not consume the install grant")
	}

	persistedTokens, err := config.NewConfigPersistence(cfg.DataPath).LoadAPITokens()
	if err != nil {
		t.Fatalf("load persisted install token after registration: %v", err)
	}
	restartedCfg := &config.Config{
		DataPath:  cfg.DataPath,
		APITokens: persistedTokens,
	}
	restartedHandler := newTestConfigHandlers(t, restartedCfg)
	reuseRec := runAgentAutoRegister(t, restartedHandler, install.Token, AutoRegisterRequest{
		Type:       "pve",
		Host:       "https://pve-after-restart.local:8006",
		TokenID:    "pulse-monitor@pve!pulse-pve-after-restart",
		TokenValue: "different-proxmox-secret",
		ServerName: "pve-after-restart",
		Source:     "agent",
	})
	if reuseRec.Code != http.StatusForbidden {
		t.Fatalf("persisted install grant reuse status = %d, want 403; body=%s", reuseRec.Code, reuseRec.Body.String())
	}
}

func TestProxmoxInstallTokenExistingSourceCheckConsumesGrant(t *testing.T) {
	rawToken := "install-existing-pve.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeAgentReport}, map[string]string{
		"install_type": "pve",
		"issued_via":   agentInstallIssuedViaConfig,
	})
	cfg := &config.Config{
		DataPath:  t.TempDir(),
		APITokens: []config.APITokenRecord{record},
		PVEInstances: []config.PVEInstance{{
			Name: "existing-pve",
			Host: "https://existing-pve.local:8006",
		}},
	}
	handler := newTestConfigHandlers(t, cfg)

	rec := runAgentAutoRegister(t, handler, rawToken, AutoRegisterRequest{
		Type:              "pve",
		Host:              "https://existing-pve.local:8006",
		ServerName:        "existing-pve",
		Source:            "agent",
		CheckRegistration: true,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("existing registration check status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var response autoRegisterCheckResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode registration check: %v", err)
	}
	if !response.Registered {
		t.Fatal("existing source check returned registered=false")
	}
	if !strings.EqualFold(cfg.APITokens[0].Metadata[proxmoxInstallRegistrationCompletedKey], "true") {
		t.Fatal("existing source check left the fresh-install grant dormant")
	}
}

func TestOrdinaryAgentRegistrationCheckCannotCreateMissingSource(t *testing.T) {
	rawToken := "ordinary-agent-registration-check.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeAgentReport}, nil)
	cfg := &config.Config{
		DataPath:  t.TempDir(),
		APITokens: []config.APITokenRecord{record},
	}
	handler := newTestConfigHandlers(t, cfg)

	rec := runAgentAutoRegister(t, handler, rawToken, AutoRegisterRequest{
		Type:              "pve",
		Host:              "https://missing-pve.local:8006",
		ServerName:        "missing-pve",
		Source:            "agent",
		CheckRegistration: true,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("ordinary agent check status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var response autoRegisterCheckResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode ordinary agent check: %v", err)
	}
	if response.Registered || response.SourceExists || response.CanRegister {
		t.Fatalf("ordinary agent missing-source response = %#v, want all false", response)
	}
}

func TestProxmoxInstallTokenBootstrapsDeclaredSourceOnce(t *testing.T) {
	stubAutoRegisterNetworkDeps(t)

	for _, nodeType := range []string{"pve", "pbs"} {
		t.Run(nodeType, func(t *testing.T) {
			rawToken := "install-" + nodeType + "-token.12345678"
			record := newTokenRecord(t, rawToken, []string{config.ScopeAgentReport}, map[string]string{
				"install_type": nodeType,
				"issued_via":   agentInstallIssuedViaConfig,
			})
			cfg := &config.Config{
				DataPath:  t.TempDir(),
				APITokens: []config.APITokenRecord{record},
			}
			handler := newTestConfigHandlers(t, cfg)
			host := fmt.Sprintf("https://%s-node.local:%d", nodeType, map[string]int{"pve": 8006, "pbs": 8007}[nodeType])
			payload := AutoRegisterRequest{
				Type:       nodeType,
				Host:       host,
				TokenID:    fmt.Sprintf("pulse-monitor@%s!pulse-%s-node", nodeType, nodeType),
				TokenValue: "proxmox-secret",
				ServerName: nodeType + "-node",
				Source:     "agent",
			}

			checkPayload := payload
			checkPayload.TokenID = ""
			checkPayload.TokenValue = ""
			checkPayload.CheckRegistration = true
			checkRec := runAgentAutoRegister(t, handler, rawToken, checkPayload)
			if checkRec.Code != http.StatusOK {
				t.Fatalf("registration check status = %d, body=%s", checkRec.Code, checkRec.Body.String())
			}
			if strings.EqualFold(cfg.APITokens[0].Metadata[proxmoxInstallRegistrationCompletedKey], "true") {
				t.Fatal("read-only registration check consumed the install grant")
			}
			if got := cfg.APITokens[0].Metadata["bound_hostname"]; got != payload.ServerName {
				t.Fatalf("install token bound hostname = %q, want %q", got, payload.ServerName)
			}

			registerRec := runAgentAutoRegister(t, handler, rawToken, payload)
			if registerRec.Code != http.StatusOK {
				t.Fatalf("initial registration status = %d, body=%s", registerRec.Code, registerRec.Body.String())
			}
			if !strings.EqualFold(cfg.APITokens[0].Metadata[proxmoxInstallRegistrationCompletedKey], "true") {
				t.Fatal("successful initial registration did not consume the install grant")
			}
			if got := cfg.APITokens[0].Metadata[proxmoxInstallRegistrationTypeKey]; got != nodeType {
				t.Fatalf("registration type metadata = %q, want %q", got, nodeType)
			}

			switch nodeType {
			case "pve":
				if len(cfg.PVEInstances) != 1 {
					t.Fatalf("PVE instances = %d, want 1", len(cfg.PVEInstances))
				}
				cfg.PVEInstances = nil
			case "pbs":
				if len(cfg.PBSInstances) != 1 {
					t.Fatalf("PBS instances = %d, want 1", len(cfg.PBSInstances))
				}
				cfg.PBSInstances = nil
			}

			reuseRec := runAgentAutoRegister(t, handler, rawToken, payload)
			if reuseRec.Code != http.StatusForbidden {
				t.Fatalf("reused install grant status = %d, want 403; body=%s", reuseRec.Code, reuseRec.Body.String())
			}
		})
	}
}

func TestProxmoxInstallTokenCannotBootstrapDifferentType(t *testing.T) {
	stubAutoRegisterNetworkDeps(t)

	rawToken := "install-pve-wrong-type.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeAgentReport}, map[string]string{
		"install_type": "pve",
		"issued_via":   agentInstallIssuedViaConfig,
	})
	cfg := &config.Config{
		DataPath:  t.TempDir(),
		APITokens: []config.APITokenRecord{record},
	}
	handler := newTestConfigHandlers(t, cfg)
	rec := runAgentAutoRegister(t, handler, rawToken, AutoRegisterRequest{
		Type:       "pbs",
		Host:       "https://pbs-node.local:8007",
		TokenID:    "pulse-monitor@pbs!pulse-pbs-node",
		TokenValue: "proxmox-secret",
		ServerName: "pbs-node",
		Source:     "agent",
	})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("cross-type registration status = %d, want 403; body=%s", rec.Code, rec.Body.String())
	}
}

func TestProxmoxInstallTokenMalformedRequestDoesNotBindGrant(t *testing.T) {
	rawToken := "install-pve-malformed.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeAgentReport}, map[string]string{
		"install_type": "pve",
		"issued_via":   agentInstallIssuedViaConfig,
	})
	cfg := &config.Config{
		DataPath:  t.TempDir(),
		APITokens: []config.APITokenRecord{record},
	}
	handler := newTestConfigHandlers(t, cfg)

	rec := runAgentAutoRegister(t, handler, rawToken, AutoRegisterRequest{
		Type:       "pve",
		Host:       "https://malformed.local:8006",
		TokenID:    "not-a-canonical-token-id",
		TokenValue: "proxmox-secret",
		ServerName: "malformed-host",
		Source:     "agent",
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("malformed registration status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
	if got := cfg.APITokens[0].Metadata["bound_hostname"]; got != "" {
		t.Fatalf("malformed request bound install token to %q", got)
	}
}

func TestProxmoxInstallTokenConcurrentBootstrapConsumesOneGrant(t *testing.T) {
	stubAutoRegisterNetworkDeps(t)

	rawToken := "install-pve-race.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeAgentReport}, map[string]string{
		"install_type": "pve",
		"issued_via":   agentInstallIssuedViaConfig,
	})
	cfg := &config.Config{
		DataPath:  t.TempDir(),
		APITokens: []config.APITokenRecord{record},
	}
	handler := newTestConfigHandlers(t, cfg)

	start := make(chan struct{})
	statuses := make(chan int, 2)
	var wg sync.WaitGroup
	for i := 1; i <= 2; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			rec := runAgentAutoRegister(t, handler, rawToken, AutoRegisterRequest{
				Type:       "pve",
				Host:       fmt.Sprintf("https://pve-%d.local:8006", i),
				TokenID:    fmt.Sprintf("pulse-monitor@pve!pulse-pve-%d", i),
				TokenValue: fmt.Sprintf("proxmox-secret-%d", i),
				ServerName: fmt.Sprintf("pve-%d", i),
				Source:     "agent",
			})
			statuses <- rec.Code
		}()
	}
	close(start)
	wg.Wait()
	close(statuses)

	counts := map[int]int{}
	for status := range statuses {
		counts[status]++
	}
	if counts[http.StatusOK] != 1 || counts[http.StatusForbidden] != 1 {
		t.Fatalf("concurrent statuses = %#v, want one 200 and one 403", counts)
	}
	if len(cfg.PVEInstances) != 1 {
		t.Fatalf("PVE instances = %d, want one successful bootstrap", len(cfg.PVEInstances))
	}
}
