package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
	"github.com/rcourtman/pulse-go-rewrite/pkg/extensions"
)

// Use ordinary prose: chat intentionally withholds long unspaced provider
// fragments as possible compacted tool-call preludes. Do not weaken that guard.
const assistantPrivateResult = "The synthetic private assistant result is ready."
const assistantControlToken = "assistant-control-token.12345678"

// Only text from a local fake Ollama, never model tools, agents or commands.
// Separate gates/counters exclude startup probes and distinguish two real
// requests: withdrawing one reader must not affect the other reader's result.
type assistantProviderGate struct {
	entered, release, finished chan struct{}
	enterOnce, releaseOnce     sync.Once
	calls                      atomic.Int32
	cancelled                  atomic.Bool
	result                     string // Set before the request; empty retains the ordinary fixture text.
}

func newAssistantProviderGate() *assistantProviderGate {
	return &assistantProviderGate{entered: make(chan struct{}), release: make(chan struct{}), finished: make(chan struct{})}
}
func (g *assistantProviderGate) unblock() { g.releaseOnce.Do(func() { close(g.release) }) }

// The test binder preserves the extension seam and its licence refusal while
// executing the actual core handler. It is not private-binary/licence proof.
type assistantAnalysisFixture struct {
	extensions.AIAlertAnalysisEndpoints
	runtime  extensions.AIAlertAnalysisRuntime
	licensed *atomic.Bool
}

func (f assistantAnalysisFixture) HandleInvestigateAlert(w http.ResponseWriter, r *http.Request) {
	if !f.licensed.Load() {
		f.runtime.WriteLicenseRequired(w, featureAIAlertsKey, "Alert investigation requires Pulse Pro")
		return
	}
	f.runtime.CoreHandlers.HandleInvestigateAlert(w, r)
}

type assistantStreamFixture struct {
	router     *Router
	server     *httptest.Server
	token      string
	kind       string
	primary    *assistantProviderGate
	control    *assistantProviderGate
	licensed   atomic.Bool
	otherCalls atomic.Int32
}

func newAssistantStreamFixture(t *testing.T, kind string, scopes []string, writeTimeout ...time.Duration) *assistantStreamFixture {
	t.Helper()
	f := &assistantStreamFixture{kind: kind, primary: newAssistantProviderGate(), control: newAssistantProviderGate()}
	f.licensed.Store(true)
	provider := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" {
			mockOllamaForExecute().ServeHTTP(w, r)
			return
		}
		var request struct {
			Messages []struct{ Role, Content string } `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, "invalid fixture request", http.StatusBadRequest)
			return
		}
		var gate *assistantProviderGate
		for _, message := range request.Messages {
			if message.Role != "user" || strings.HasPrefix(message.Content, "Title this Pulse Assistant session.") {
				continue
			}
			if strings.Contains(message.Content, "stream-fixture-primary") {
				gate = f.primary
			} else if strings.Contains(message.Content, "stream-fixture-control") {
				gate = f.control
			}
		}
		content := "synthetic provider ready"
		if gate == nil {
			f.otherCalls.Add(1)
		} else {
			if gate.calls.Add(1) != 1 {
				http.Error(w, "unexpected repeat fixture request", http.StatusInternalServerError)
				return
			}
			defer close(gate.finished)
			gate.enterOnce.Do(func() { close(gate.entered) })
			select {
			case <-gate.release:
			case <-r.Context().Done():
				gate.cancelled.Store(true)
				return
			}
			if r.Context().Err() != nil {
				gate.cancelled.Store(true)
				return
			}
			content = assistantPrivateResult
			if gate.result != "" {
				content = gate.result
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"model": "llama3", "message": map[string]any{"role": "assistant", "content": content},
			"done": true, "done_reason": "stop", "prompt_eval_count": 3, "eval_count": 5,
		})
	}))
	t.Cleanup(provider.Close)

	// NewRouter resolves the extension while binding routes. Restore the
	// previous binder immediately; tests never run parallel with global setup.
	aiAlertAnalysisBindMu.RLock()
	previous := aiAlertAnalysisEndpointsBinder
	aiAlertAnalysisBindMu.RUnlock()
	if kind == "investigate-alert" {
		SetAIAlertAnalysisEndpointsBinder(func(defaults extensions.AIAlertAnalysisEndpoints, runtime extensions.AIAlertAnalysisRuntime) extensions.AIAlertAnalysisEndpoints {
			return assistantAnalysisFixture{AIAlertAnalysisEndpoints: defaults, runtime: runtime, licensed: &f.licensed}
		})
		defer SetAIAlertAnalysisEndpointsBinder(previous)
	}
	f.router, f.token = setupExecuteRouterForScopes(t, provider.URL, scopes, true)
	f.router.config.APITokens = append(f.router.config.APITokens, newTokenRecord(t, assistantControlToken, []string{config.ScopeWildcard}, nil))
	f.router.config.AuthUser = "assistant-fixture-admin"
	f.router.config.ProxyAuthSecret = "synthetic-assistant-proxy-secret"
	f.router.config.ProxyAuthUserHeader = "X-Remote-User"
	if kind == "chat" {
		if err := f.router.aiHandler.Start(context.Background(), nil); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = f.router.aiHandler.Stop(context.Background()) })
	}
	if len(writeTimeout) == 0 {
		f.server = newIPv4HTTPServer(t, f.router.Handler())
	} else {
		// Set before Start: an initial server deadline must really be cleared
		// through the complete Router/middleware/guard chain, not a test writer.
		server := httptest.NewUnstartedServer(f.router.Handler())
		_ = server.Listener.Close()
		listener, err := net.Listen("tcp4", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		server.Listener = listener
		server.Config.WriteTimeout = writeTimeout[0]
		server.Start()
		f.server = server
	}
	t.Cleanup(f.server.Close)
	t.Cleanup(func() { f.primary.unblock(); f.control.unblock() })
	return f
}

func (f *assistantStreamFixture) request(t *testing.T, label, raw, transport string) *http.Request {
	t.Helper()
	path := "/api/ai/" + f.kind
	if f.kind == "execute" {
		path += "/stream"
	}
	body := fmt.Sprintf(`{"prompt":"Return stream-fixture-%s text without tools."}`, label)
	if f.kind == "investigate-alert" {
		body = fmt.Sprintf(`{"alertIdentifier":"fixture-%s","resource_name":"stream-fixture-%s","resource_type":"node","resource_id":"synthetic-node","alert_type":"cpu"}`, label, label)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	t.Cleanup(cancel)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, f.server.URL+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Pulse-Org-ID", "default")
	switch transport {
	case "bearer":
		req.Header.Set("Authorization", "Bearer "+raw)
	case "session":
		req.AddCookie(&http.Cookie{Name: cookieNameSession, Value: raw})
		req.Header.Set("X-CSRF-Token", generateCSRFToken(raw))
	case "proxy":
		req.Header.Set("X-Proxy-Secret", f.router.config.ProxyAuthSecret)
		req.Header.Set("X-Remote-User", "assistant-fixture-admin")
	default:
		if raw != "" {
			req.Header.Set("X-API-Token", raw)
		}
	}
	return req
}

func (f *assistantStreamFixture) open(t *testing.T, label, raw, transport string) *http.Response {
	t.Helper()
	resp, err := f.server.Client().Do(f.request(t, label, raw, transport))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	if resp.StatusCode != http.StatusOK || !strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
		data, _ := io.ReadAll(resp.Body)
		t.Fatalf("admission = %d: %s", resp.StatusCode, data)
	}
	gate := f.primary
	if label == "control" {
		gate = f.control
	}
	select {
	case <-gate.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("execution did not reach its exact prompt-bound provider gate")
	}
	return resp
}

func readAssistantResult(t *testing.T, response *http.Response, visible bool) {
	t.Helper()
	data, err := io.ReadAll(response.Body)
	response.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Contains(string(data), assistantPrivateResult); got != visible {
		t.Fatalf("private result visible = %t, want %t: %s", got, visible, data)
	}
	if !visible && (strings.Contains(string(data), `"type":"complete"`) || strings.Contains(string(data), `"type":"done"`) || strings.Contains(string(data), "Authentication required")) {
		t.Fatalf("withdrawn stream appended terminal success or HTTP error: %s", data)
	}
}

func assistantScope(kind string) string {
	if kind == "chat" {
		return config.ScopeAIChat
	}
	return config.ScopeAIExecute
}

func TestAssistantStreamsWithdrawAuthority(t *testing.T) {
	for _, kind := range []string{"execute", "investigate-alert", "chat"} {
		for _, transport := range []string{"header", "bearer"} {
			for _, change := range []string{"revoked", "expired", "scope_removed", "org_removed", "credential_replaced", "principal_replaced", "invalid_role"} {
				t.Run(kind+"/"+transport+"/"+change, func(t *testing.T) {
					f := newAssistantStreamFixture(t, kind, []string{assistantScope(kind)})
					control := f.open(t, "control", assistantControlToken, "header")
					primary := f.open(t, "primary", f.token, transport)
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
						record.ID = "replacement-credential"
					case "principal_replaced":
						record.Metadata = map[string]string{apiTokenMetadataOwnerUserID: "replacement-principal"}
					case "invalid_role":
						record.Metadata = map[string]string{internalauth.RuntimeRoleMetadataKey: internalauth.RuntimeRoleActionRunner}
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
							t.Fatalf("fresh status/calls = %d/%d, want %d/1", fresh.StatusCode, f.primary.calls.Load(), want)
						}
					}
					f.primary.unblock()
					f.control.unblock()
					readAssistantResult(t, primary, false)
					readAssistantResult(t, control, true)
					if kind != "chat" && f.primary.cancelled.Load() {
						t.Fatal("read withdrawal cancelled independent legacy execution")
					}
				})
			}
		}
	}
}

func TestAssistantStreamsPreserveScopeAlternativesAndNonTokenReaders(t *testing.T) {
	for _, kind := range []string{"execute", "investigate-alert", "chat"} {
		for _, transport := range []string{"header", "session", "proxy"} {
			t.Run(kind+"/"+transport, func(t *testing.T) {
				f := newAssistantStreamFixture(t, kind, []string{assistantScope(kind), config.ScopeSettingsRead})
				raw := f.token
				if transport == "session" {
					raw = "synthetic-assistant-session"
					GetSessionStore().CreateSession(raw, time.Hour, "fixture", "127.0.0.1", f.router.config.AuthUser)
					t.Cleanup(func() { GetSessionStore().DeleteSession(raw) })
				}
				response := f.open(t, "primary", raw, transport)
				config.Mu.Lock()
				if transport == "header" {
					// Losing an unrelated scope is not losing this route's read.
					f.router.config.APITokens[0].Scopes = []string{assistantScope(kind)}
				} else {
					f.router.config.RemoveAPIToken(f.router.config.APITokens[0].ID)
				}
				config.Mu.Unlock()
				f.primary.unblock()
				readAssistantResult(t, response, true)
			})
		}
	}
	for _, scopes := range [][]string{{config.ScopeRelayMobileAccess}, {config.ScopeAIChat}, {config.ScopeAIChat, config.ScopeRelayMobileAccess}} {
		t.Run(strings.Join(scopes, "+"), func(t *testing.T) {
			f := newAssistantStreamFixture(t, "chat", scopes)
			response := f.open(t, "primary", f.token, "header")
			config.Mu.Lock()
			f.router.config.APITokens[0].Scopes = []string{config.ScopeRelayMobileAccess}
			config.Mu.Unlock()
			f.primary.unblock()
			readAssistantResult(t, response, true)
		})
	}
}

func TestAssistantStreamsRetainAdminAndLicenseGates(t *testing.T) {
	for _, kind := range []string{"execute", "investigate-alert"} {
		t.Run(kind, func(t *testing.T) {
			f := newAssistantStreamFixture(t, kind, []string{config.ScopeAIExecute})
			response := f.open(t, "primary", f.token, "header")
			config.Mu.Lock()
			// Still has selected-org access, but no longer instance authority.
			f.router.config.APITokens[0].OrgIDs = []string{"default", "another-org"}
			config.Mu.Unlock()
			fresh, err := f.server.Client().Do(f.request(t, "primary", f.token, "header"))
			if err != nil {
				t.Fatal(err)
			}
			fresh.Body.Close()
			if fresh.StatusCode != http.StatusForbidden {
				t.Fatalf("lost instance authority = %d", fresh.StatusCode)
			}
			f.primary.unblock()
			readAssistantResult(t, response, false)
		})
	}
	t.Run("extension-license", func(t *testing.T) {
		f := newAssistantStreamFixture(t, "investigate-alert", []string{config.ScopeAIExecute})
		f.licensed.Store(false)
		response, err := f.server.Client().Do(f.request(t, "primary", f.token, "header"))
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != http.StatusPaymentRequired || f.primary.calls.Load() != 0 {
			t.Fatalf("license refusal = %d, calls = %d", response.StatusCode, f.primary.calls.Load())
		}
	})
}

// Withdrawal is a read decision, not a new policy for stopping work. Legacy
// handlers deliberately run independently of the browser; chat deliberately
// cancels with its request. Exercise the real idle writer, not a new short timer.
func TestAssistantStreamsPreserveExecutionLifetimes(t *testing.T) {
	for _, kind := range []string{"execute", "investigate-alert", "chat"} {
		t.Run(kind, func(t *testing.T) {
			f := newAssistantStreamFixture(t, kind, []string{assistantScope(kind)})
			response := f.open(t, "primary", f.token, "header")
			config.Mu.Lock()
			f.router.config.RemoveAPIToken(f.router.config.APITokens[0].ID)
			config.Mu.Unlock()
			if kind == "chat" {
				select {
				case <-f.primary.finished:
					if !f.primary.cancelled.Load() {
						t.Fatal("chat provider completed instead of observing request cancellation")
					}
				case <-time.After(8 * time.Second):
					t.Fatal("chat idle writer did not cancel request-bound work")
				}
			} else {
				select {
				case <-f.primary.finished:
					t.Fatal("idle read withdrawal ended independent legacy provider work")
				case <-time.After(3 * time.Second):
				}
				f.primary.unblock()
			}
			readAssistantResult(t, response, false)
		})
	}
}

func TestAssistantStreamsClearNativeServerDeadline(t *testing.T) {
	for _, kind := range []string{"execute", "investigate-alert", "chat"} {
		t.Run(kind, func(t *testing.T) {
			f := newAssistantStreamFixture(t, kind, []string{assistantScope(kind)}, 100*time.Millisecond)
			response := f.open(t, "primary", f.token, "header")
			// The initial HTTP deadline has passed, but this stream was
			// deliberately admitted with a longer execution/write budget.
			time.Sleep(200 * time.Millisecond)
			f.primary.unblock()
			readAssistantResult(t, response, true)
		})
	}
}
