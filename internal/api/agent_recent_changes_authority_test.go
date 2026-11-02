package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

const historyAuthorityToken = "history-authority-primary.12345678"
const historyAuthorityControl = "history-authority-control.12345678"

// Hold the normal inventory/name lookup, after the actual history store read.
// No database locks, clock sleeps, auth stubs or production hooks are needed.
type historyAuthorityProvider struct {
	entered chan struct{}
	release chan struct{}
	once    sync.Once
	calls   atomic.Int32
}

func (p *historyAuthorityProvider) ReadSnapshot() models.StateSnapshot {
	if p.calls.Add(1) <= 2 {
		p.entered <- struct{}{}
		<-p.release
	}
	return models.StateSnapshot{}
}

func (p *historyAuthorityProvider) unblock() { p.once.Do(func() { close(p.release) }) }

type historyAuthorityFixture struct {
	router   *Router
	server   *httptest.Server
	provider *historyAuthorityProvider
	store    unified.ResourceStore
	original string
}

func newHistoryAuthorityFixture(t *testing.T, scopes []string) *historyAuthorityFixture {
	t.Helper()
	t.Setenv("PULSE_AUTH_BYPASS", "false")
	cfg := newTestConfigWithTokens(t,
		newTokenRecord(t, historyAuthorityToken, scopes, nil),
		newTokenRecord(t, historyAuthorityControl, []string{config.ScopeMonitoringRead, config.ScopeAIExecute}, nil),
	)
	cfg.AuthUser, cfg.AuthPass = "operator", "fixture-nonempty-hash"
	cfg.ProxyAuthSecret, cfg.ProxyAuthUserHeader = "history-authority-proxy-secret", "X-Remote-User"
	router := NewRouter(cfg, nil, nil, nil, nil, "test")
	t.Cleanup(router.ShutdownResourceStores)
	provider := &historyAuthorityProvider{entered: make(chan struct{}, 2), release: make(chan struct{})}
	router.resourceHandlers.SetStateProvider(provider)
	store, err := router.resourceHandlers.getStore("default")
	if err != nil {
		t.Fatal(err)
	}
	for _, change := range []*unified.ResourceChange{
		unified.BuildCommandExecutionChange("vm:101", "alert-1", "operator", recentChangePrivateText, true, recentChangePrivateText, nil),
		unified.BuildRunbookExecutionChange("vm:101", "alert-1", "operator", "rb-1", recentChangePrivateText, "partial", false, recentChangePrivateText, nil),
	} {
		if err := store.RecordChange(*change); err != nil {
			t.Fatal(err)
		}
	}
	f := &historyAuthorityFixture{router: router, provider: provider, store: store}
	f.original = f.stored(t)
	f.server = httptest.NewServer(router.Handler())
	t.Cleanup(f.server.Close)
	// Release before closing the HTTP server even when a test fails early.
	t.Cleanup(provider.unblock)
	return f
}

func (f *historyAuthorityFixture) stored(t *testing.T) string {
	t.Helper()
	changes, err := f.store.GetRecentChangesFiltered("", time.Time{}, 50, unified.ResourceChangeFilters{})
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(changes)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

func (f *historyAuthorityFixture) request(t *testing.T, raw, transport string) *http.Request {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, f.server.URL+agentcapabilities.RecentChangesCapabilityPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	switch transport {
	case "header":
		req.Header.Set("X-API-Token", raw)
	case "bearer":
		req.Header.Set("Authorization", "Bearer "+raw)
	case "session":
		req.AddCookie(&http.Cookie{Name: cookieNameSession, Value: raw})
	case "proxy":
		req.Header.Set("X-Proxy-Secret", f.router.config.ProxyAuthSecret)
		req.Header.Set("X-Remote-User", "operator")
	default:
		t.Fatalf("unknown history fixture transport %q", transport)
	}
	return req
}

type historyAuthorityResult struct {
	response *http.Response
	err      error
}

func (f *historyAuthorityFixture) begin(t *testing.T, raw, transport string) <-chan historyAuthorityResult {
	t.Helper()
	req := f.request(t, raw, transport)
	result := make(chan historyAuthorityResult, 1)
	go func() {
		response, err := f.server.Client().Do(req)
		result <- historyAuthorityResult{response, err}
	}()
	select {
	case <-f.provider.entered:
	case got := <-result:
		if got.response != nil {
			got.response.Body.Close()
		}
		t.Fatalf("history returned before its inventory gate: %v", got.err)
	case <-time.After(5 * time.Second):
		t.Fatal("history did not reach its inventory gate")
	}
	return result
}

func readHistoryAuthorityResult(t *testing.T, result <-chan historyAuthorityResult, status int, private bool) {
	t.Helper()
	var got historyAuthorityResult
	select {
	case got = <-result:
	case <-time.After(5 * time.Second):
		t.Fatal("history did not finish after inventory release")
	}
	if got.err != nil {
		t.Fatal(got.err)
	}
	defer got.response.Body.Close()
	body, err := io.ReadAll(got.response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if got.response.StatusCode != status {
		t.Errorf("history status = %d, want %d (execution detail present: %t)", got.response.StatusCode, status, strings.Contains(string(body), recentChangePrivateText))
	}
	if status != http.StatusOK {
		if strings.Contains(string(body), recentChangePrivateText) || !strings.Contains(string(body), "Authentication required") || strings.Contains(got.response.Header.Get("Content-Type"), "application/json") {
			t.Error("withdrawn history disclosed data or implied successful empty JSON")
		}
		return
	}
	var history AgentRecentChanges
	if json.Unmarshal(body, &history) != nil || history.Count != 2 || len(history.Changes) != 2 || history.Truncated {
		t.Error("authorised history lost its existing JSON envelope")
	}
	if strings.Contains(string(body), recentChangePrivateText) != private {
		t.Error("history command visibility differs from admitted authority")
	}
	for _, change := range history.Changes {
		if change.CommandRedacted == private || change.Metadata[unified.MetadataAlertIdentifier] != "alert-1" {
			t.Error("history lost its redaction marker or safe attribution")
		}
	}
}

func TestAgentHistoryWithdrawnSessionCannotDisclose(t *testing.T) {
	for _, change := range []string{"logout", "expired", "replaced", "principal", "store"} {
		t.Run(change, func(t *testing.T) {
			f := newHistoryAuthorityFixture(t, []string{config.ScopeMonitoringRead})
			const raw = "history-authority-session"
			GetSessionStore().CreateSession(raw, time.Hour, "fixture", "127.0.0.1", f.router.config.AuthUser)
			t.Cleanup(func() { GetSessionStore().DeleteSession(raw) })
			control := f.begin(t, historyAuthorityControl, "header")
			primary := f.begin(t, raw, "session")
			switch change {
			case "logout":
				logoutReadSession(t, f.server.URL, raw)
			case "store":
				InitSessionStore(t.TempDir())
			default:
				mutateReadSession(raw, change)
			}
			f.provider.unblock()
			readHistoryAuthorityResult(t, primary, http.StatusUnauthorized, false)
			readHistoryAuthorityResult(t, control, http.StatusOK, true)
			if f.stored(t) != f.original {
				t.Error("session withdrawal rewrote canonical history")
			}
		})
	}
}
