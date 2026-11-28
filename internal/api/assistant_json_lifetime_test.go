package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
	"github.com/rcourtman/pulse-go-rewrite/pkg/extensions"
)

// Exercise the actual public analysis handler through the extension boundary,
// not a stand-in result writer. This is not private-binary/licence proof.
type assistantJSONAnalysisFixture struct {
	extensions.AIAlertAnalysisEndpoints
	runtime  extensions.AIAlertAnalysisRuntime
	licensed *atomic.Bool
}

func (f assistantJSONAnalysisFixture) HandleAnalyzeKubernetesCluster(w http.ResponseWriter, r *http.Request) {
	if !f.licensed.Load() {
		f.runtime.WriteLicenseRequired(w, featureKubernetesAIKey, "Kubernetes analysis requires Pulse Pro")
		return
	}
	f.runtime.CoreHandlers.HandleAnalyzeKubernetesCluster(w, r)
}

type assistantJSONFixture struct {
	*assistantStreamFixture
	kind     string
	licensed *atomic.Bool
}

func newAssistantJSONFixture(t *testing.T, kind string, scopes []string) *assistantJSONFixture {
	t.Helper()
	licensed := &atomic.Bool{}
	licensed.Store(true)
	if kind == "kubernetes" {
		aiAlertAnalysisBindMu.RLock()
		previous := aiAlertAnalysisEndpointsBinder
		aiAlertAnalysisBindMu.RUnlock()
		SetAIAlertAnalysisEndpointsBinder(func(defaults extensions.AIAlertAnalysisEndpoints, runtime extensions.AIAlertAnalysisRuntime) extensions.AIAlertAnalysisEndpoints {
			return assistantJSONAnalysisFixture{defaults, runtime, licensed}
		})
		defer SetAIAlertAnalysisEndpointsBinder(previous)
	}
	// Share the held text-only Ollama and native full-Router setup. The JSON
	// requests below never open an SSE reader or execute a tool/agent/command.
	f := &assistantJSONFixture{newAssistantStreamFixture(t, "json-"+kind, scopes), kind, licensed}
	if kind == "kubernetes" {
		registry := unifiedresources.NewRegistry(nil)
		registry.IngestSnapshot(models.StateSnapshot{KubernetesClusters: []models.KubernetesCluster{
			{ID: "json-primary", Name: "stream-fixture-primary"},
			{ID: "json-control", Name: "stream-fixture-control"},
		}})
		f.router.aiSettingsHandler.defaultAIService.SetReadState(registry)
	}
	return f
}

func (f *assistantJSONFixture) request(t *testing.T, label, raw, transport string) *http.Request {
	t.Helper()
	req := f.assistantStreamFixture.request(t, label, raw, transport)
	req.URL.Path = "/api/ai/execute"
	if f.kind == "kubernetes" {
		req.URL.Path = "/api/ai/kubernetes/analyze"
		body := fmt.Sprintf(`{"cluster_id":"json-%s"}`, label)
		req.Body = io.NopCloser(strings.NewReader(body))
		req.ContentLength = int64(len(body))
		req.GetBody = nil
	}
	return req
}

type assistantJSONResult struct {
	response *http.Response
	err      error
}

func (f *assistantJSONFixture) begin(t *testing.T, label, raw, transport string) <-chan assistantJSONResult {
	t.Helper()
	req := f.request(t, label, raw, transport)
	result := make(chan assistantJSONResult, 1)
	go func() {
		response, err := f.server.Client().Do(req)
		result <- assistantJSONResult{response, err}
	}()
	gate := f.primary
	if label == "control" {
		gate = f.control
	}
	select {
	case <-gate.entered:
	case got := <-result:
		if got.response != nil {
			defer got.response.Body.Close()
			body, _ := io.ReadAll(got.response.Body)
			t.Fatalf("JSON request returned before its provider gate: %d %s", got.response.StatusCode, body)
		}
		t.Fatalf("JSON request failed before its provider gate: %v", got.err)
	case <-time.After(5 * time.Second):
		t.Fatal("JSON request did not reach its prompt-bound provider gate")
	}
	return result
}

func readAssistantJSONResult(t *testing.T, result <-chan assistantJSONResult, visible bool) {
	t.Helper()
	var got assistantJSONResult
	select {
	case got = <-result:
	case <-time.After(5 * time.Second):
		t.Fatal("JSON request did not finish after its provider released")
	}
	if got.err != nil {
		t.Fatal(got.err)
	}
	defer got.response.Body.Close()
	body, err := io.ReadAll(got.response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if visible {
		var response AIExecuteResponse
		if got.response.StatusCode != http.StatusOK || !strings.Contains(got.response.Header.Get("Content-Type"), "application/json") || json.Unmarshal(body, &response) != nil || response.Content != assistantPrivateResult {
			t.Fatalf("authorised JSON response changed: %d %s", got.response.StatusCode, body)
		}
		return
	}
	if got.response.StatusCode != http.StatusUnauthorized || strings.Contains(string(body), assistantPrivateResult) || !strings.Contains(string(body), "Authentication required") || strings.Contains(got.response.Header.Get("Content-Type"), "application/json") {
		t.Fatalf("withdrawn JSON reader received result/success instead of auth refusal: %d %s", got.response.StatusCode, body)
	}
}

func TestAssistantJSONWithdrawAuthority(t *testing.T) {
	for _, kind := range []string{"execute", "kubernetes"} {
		for _, transport := range []string{"header", "bearer"} {
			for _, change := range []string{"revoked", "expired", "scope_removed", "org_removed", "credential_replaced", "principal_replaced", "invalid_role", "instance_removed"} {
				t.Run(kind+"/"+transport+"/"+change, func(t *testing.T) {
					f := newAssistantJSONFixture(t, kind, []string{config.ScopeAIExecute})
					control := f.begin(t, "control", assistantControlToken, "header")
					primary := f.begin(t, "primary", f.token, transport)
					config.Mu.Lock()
					record := &f.router.config.APITokens[0]
					switch change {
					case "revoked":
						f.router.config.RemoveAPIToken(record.ID)
					case "expired":
						past := time.Now().Add(-time.Second)
						record.ExpiresAt = &past
					case "scope_removed":
						record.Scopes = []string{config.ScopeMonitoringRead}
					case "org_removed":
						record.OrgIDs = []string{"other-org"}
					case "credential_replaced":
						record.ID = "replacement-json-credential"
					case "principal_replaced":
						record.Metadata = map[string]string{apiTokenMetadataOwnerUserID: "replacement-json-principal"}
					case "invalid_role":
						record.Metadata = map[string]string{internalauth.RuntimeRoleMetadataKey: internalauth.RuntimeRoleActionRunner}
					case "instance_removed":
						record.OrgIDs = []string{"default", "another-org"}
					}
					config.Mu.Unlock()
					if change != "credential_replaced" && change != "principal_replaced" {
						fresh, err := f.server.Client().Do(f.request(t, "primary", f.token, transport))
						if err != nil {
							t.Fatal(err)
						}
						fresh.Body.Close()
						want := http.StatusForbidden
						if change == "revoked" || change == "expired" || change == "invalid_role" {
							want = http.StatusUnauthorized
						}
						if fresh.StatusCode != want || f.primary.calls.Load() != 1 {
							t.Fatalf("fresh JSON status/calls = %d/%d, want %d/1", fresh.StatusCode, f.primary.calls.Load(), want)
						}
					}
					f.primary.unblock()
					f.control.unblock()
					readAssistantJSONResult(t, control, true)
					readAssistantJSONResult(t, primary, false)
					if f.primary.cancelled.Load() {
						t.Fatal("result refusal changed admitted execution lifetime")
					}
				})
			}
		}
	}
}

func TestAssistantJSONPreserveReaders(t *testing.T) {
	for _, kind := range []string{"execute", "kubernetes"} {
		for _, transport := range []string{"header", "session", "proxy"} {
			t.Run(kind+"/"+transport, func(t *testing.T) {
				f := newAssistantJSONFixture(t, kind, []string{config.ScopeAIExecute, config.ScopeSettingsRead})
				raw := f.token
				if transport == "session" {
					raw = "synthetic-assistant-json-session"
					GetSessionStore().CreateSession(raw, time.Hour, "fixture", "127.0.0.1", f.router.config.AuthUser)
					t.Cleanup(func() { GetSessionStore().DeleteSession(raw) })
				}
				result := f.begin(t, "primary", raw, transport)
				config.Mu.Lock()
				if transport == "header" {
					f.router.config.APITokens[0].Scopes = []string{config.ScopeAIExecute}
				} else {
					f.router.config.RemoveAPIToken(f.router.config.APITokens[0].ID)
				}
				config.Mu.Unlock()
				f.primary.unblock()
				readAssistantJSONResult(t, result, true)
			})
		}
	}
}

func TestAssistantJSONPreserveRequestCancellation(t *testing.T) {
	for _, kind := range []string{"execute", "kubernetes"} {
		t.Run(kind, func(t *testing.T) {
			f := newAssistantJSONFixture(t, kind, []string{config.ScopeAIExecute})
			req := f.request(t, "primary", f.token, "header")
			ctx, cancel := context.WithCancel(req.Context())
			defer cancel()
			result := make(chan assistantJSONResult, 1)
			go func() {
				response, err := f.server.Client().Do(req.WithContext(ctx))
				result <- assistantJSONResult{response, err}
			}()
			select {
			case <-f.primary.entered:
			case <-time.After(5 * time.Second):
				t.Fatal("JSON execution did not reach provider")
			}
			cancel()
			select {
			case <-f.primary.finished:
				if !f.primary.cancelled.Load() {
					t.Fatal("JSON execution detached from request cancellation")
				}
			case <-time.After(5 * time.Second):
				t.Fatal("JSON provider did not observe request cancellation")
			}
			got := <-result
			if got.response != nil {
				got.response.Body.Close()
			}
			if !errors.Is(got.err, context.Canceled) {
				t.Fatalf("cancelled JSON client error = %v", got.err)
			}
		})
	}
}

func TestAssistantJSONRetainLicenseGate(t *testing.T) {
	f := newAssistantJSONFixture(t, "kubernetes", []string{config.ScopeAIExecute})
	f.licensed.Store(false)
	response, err := f.server.Client().Do(f.request(t, "primary", f.token, "header"))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusPaymentRequired || f.primary.calls.Load() != 0 {
		t.Fatalf("JSON extension licence refusal = %d; provider calls = %d", response.StatusCode, f.primary.calls.Load())
	}
}

func assistantJSONWithdrawAfterPersistedTokenDelete(t *testing.T) {
	for _, kind := range []string{"execute", "kubernetes"} {
		t.Run(kind, func(t *testing.T) {
			f := newAssistantJSONFixture(t, kind, []string{config.ScopeAIExecute})
			primary := f.begin(t, "primary", f.token, "header")
			control := f.begin(t, "control", assistantControlToken, "bearer")
			id := f.router.config.APITokens[0].ID
			req, err := http.NewRequest(http.MethodDelete, f.server.URL+"/api/security/tokens/"+id, nil)
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("X-API-Token", assistantControlToken)
			deleted, err := f.server.Client().Do(req)
			if err != nil {
				t.Fatal(err)
			}
			deleted.Body.Close()
			if deleted.StatusCode != http.StatusNoContent {
				t.Fatalf("JSON persisted DELETE = %d", deleted.StatusCode)
			}
			records, err := f.router.persistence.LoadAPITokens()
			if err != nil || len(records) != 1 || records[0].ID == id {
				t.Fatalf("deleted JSON token remains durable: records=%d error=%v", len(records), err)
			}
			f.primary.unblock()
			f.control.unblock()
			readAssistantJSONResult(t, control, true)
			readAssistantJSONResult(t, primary, false)
		})
	}
}
