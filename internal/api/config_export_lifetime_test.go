package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/api/configapi"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

const configExportPassphrase = "synthetic-backup-passphrase"

// Hold the real exporter after it has loaded and encrypted only temporary
// configuration, at the existing audit dependency, before any transport write.
type configExportHold struct {
	armed            atomic.Bool
	entered, release chan struct{}
	once             sync.Once
}

func (h *configExportHold) unblock() { h.once.Do(func() { close(h.release) }) }

func newHeldConfigExportRouter(t *testing.T) (*Router, *config.Config, *httptest.Server, *configExportHold) {
	t.Helper()
	cfg := newTestConfigWithTokens(t,
		newTokenRecord(t, logStreamToken, []string{config.ScopeSettingsRead}, nil),
		newTokenRecord(t, logStreamControl, []string{config.ScopeWildcard}, nil))
	cfg.AuthUser = "operator"
	t.Setenv("PULSE_DATA_DIR", cfg.DataPath)
	router := NewRouter(cfg, nil, nil, nil, nil, "test")
	cleanupTestRouter(t, router)
	if err := router.persistence.SaveNodesConfig([]config.PVEInstance{{Name: "private-backup-fixture", Host: "https://pve.invalid:8006", User: "fixture@pve", TokenName: "fixture", TokenValue: "synthetic-backup-secret", VerifySSL: true}}, nil, nil); err != nil {
		t.Fatal(err)
	}
	if err := router.persistence.SaveAPITokens(cfg.APITokens); err != nil {
		t.Fatal(err)
	}
	hold := &configExportHold{entered: make(chan struct{}), release: make(chan struct{})}
	router.configHandlers.SetRuntimeDependencies(configapi.RuntimeDependencies{AuditEvent: func(_, event, _, _, path string, success bool, _ string) {
		if event == "config_exported" && path == "/api/config/export" && success && hold.armed.CompareAndSwap(true, false) {
			close(hold.entered)
			<-hold.release
		}
	}})
	server := httptest.NewServer(router.Handler())
	t.Cleanup(func() { hold.unblock(); server.Close() })
	return router, cfg, server, hold
}

func configExportRequest(t *testing.T, ctx context.Context, server *httptest.Server, raw, transport string) *http.Request {
	t.Helper()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, server.URL+"/api/config/export", strings.NewReader(`{"passphrase":"`+configExportPassphrase+`"}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	switch transport {
	case "bearer":
		req.Header.Set("Authorization", "Bearer "+raw)
	case "session", "sso_session":
		req.AddCookie(&http.Cookie{Name: cookieNameSession, Value: raw})
		req.Header.Set("X-CSRF-Token", generateCSRFToken(raw))
	default:
		req.Header.Set("X-API-Token", raw)
	}
	return req
}

type configExportWire struct {
	Status int
	Header http.Header
	Body   string
}

func readConfigExportWire(t *testing.T, response *http.Response) configExportWire {
	t.Helper()
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	return configExportWire{response.StatusCode, response.Header.Clone(), string(body)}
}

func assertConfigExportArchive(t *testing.T, wire configExportWire) {
	t.Helper()
	var envelope struct{ Status, Data string }
	if wire.Status != http.StatusOK || json.Unmarshal([]byte(wire.Body), &envelope) != nil || envelope.Status != "success" || envelope.Data == "" || wire.Header.Get("Content-Type") != "application/json" {
		t.Fatal("authorised export lost its JSON archive")
	}
	// Use the existing importer only in a separate temporary persistence scope:
	// an opaque ciphertext string is not proof that the backup still restores.
	restored := config.NewConfigPersistence(t.TempDir())
	if err := restored.ImportConfig(envelope.Data, configExportPassphrase); err != nil {
		t.Fatalf("synthetic backup no longer restores: %v", err)
	}
	nodes, err := restored.LoadNodesConfig()
	if err != nil || len(nodes.PVEInstances) != 1 || nodes.PVEInstances[0].TokenValue != "synthetic-backup-secret" || !nodes.PVEInstances[0].VerifySSL {
		t.Fatal("synthetic backup changed its connection or credential")
	}
}

func configExportNativeWithdrawal(t *testing.T, transport string) (configExportWire, configExportWire) {
	t.Helper()
	router, cfg, server, hold := newHeldConfigExportRouter(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	raw := logStreamToken
	if transport == "session" || transport == "sso_session" {
		raw = "synthetic-config-export-session"
		username := cfg.AuthUser
		if transport == "sso_session" {
			username = "sso:fixture@example.invalid"
			manager := installTestRBACManager(t)
			if err := manager.UpdateUserRoles(username, []string{internalauth.RoleAdmin}); err != nil {
				t.Fatal(err)
			}
		}
		GetSessionStore().CreateSession(raw, time.Hour, "fixture", "127.0.0.1", username)
		t.Cleanup(func() { GetSessionStore().DeleteSession(raw) })
	}
	request := configExportRequest(t, ctx, server, raw, transport)
	hold.armed.Store(true)
	type result struct {
		response *http.Response
		err      error
	}
	done := make(chan result, 1)
	go func() { r, e := server.Client().Do(request); done <- result{r, e} }()
	select {
	case <-hold.entered:
	case r := <-done:
		t.Fatalf("export did not reach encrypted-result hold: %v %v", r.response, r.err)
	case <-ctx.Done():
		t.Fatal("export never reached encrypted-result hold")
	}
	// A separately admitted reader must finish while this export remains held.
	response, err := server.Client().Do(configExportRequest(t, ctx, server, logStreamControl, "header"))
	if err != nil {
		t.Fatal(err)
	}
	authorised := readConfigExportWire(t, response)
	assertConfigExportArchive(t, authorised)
	if transport == "session" || transport == "sso_session" {
		logoutReadSession(t, server.URL, raw)
	} else {
		config.Mu.RLock()
		id := cfg.APITokens[0].ID
		config.Mu.RUnlock()
		deletion, err := http.NewRequestWithContext(ctx, http.MethodDelete, server.URL+"/api/security/tokens/"+id, nil)
		if err != nil {
			t.Fatal(err)
		}
		deletion.Header.Set("X-API-Token", logStreamControl)
		response, err := server.Client().Do(deletion)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		persisted, err := router.persistence.LoadAPITokens()
		if response.StatusCode != http.StatusNoContent || err != nil || len(persisted) != 1 || persisted[0].ID == id {
			t.Fatal("DELETE did not persist exact credential withdrawal")
		}
	}
	fresh, err := server.Client().Do(configExportRequest(t, ctx, server, raw, transport))
	if err != nil {
		t.Fatal(err)
	}
	freshWire := readConfigExportWire(t, fresh)
	if freshWire.Status != http.StatusUnauthorized {
		t.Fatalf("fresh admission=%d, want 401", freshWire.Status)
	}
	hold.unblock()
	var completed result
	select {
	case completed = <-done:
	case <-ctx.Done():
		t.Fatal("encrypted export failed to finish after release")
	}
	if completed.err != nil {
		t.Fatal(completed.err)
	}
	denied := readConfigExportWire(t, completed.response)
	if denied.Status != http.StatusUnauthorized || denied.Body != "Authentication required\n" || denied.Header.Get("Content-Disposition") != "" {
		// Parent witnesses establish that the escaped result is a decryptable
		// configuration backup, not a success flag or a harmless empty response.
		if denied.Status == http.StatusOK {
			assertConfigExportArchive(t, denied)
		}
		t.Errorf("held export status=%d, encrypted archive escaped=%v after persisted withdrawal; want generic 401", denied.Status, strings.Contains(denied.Body, `"data"`))
	}
	nodes, err := router.persistence.LoadNodesConfig()
	if err != nil || len(nodes.PVEInstances) != 1 || nodes.PVEInstances[0].TokenValue != "synthetic-backup-secret" {
		t.Fatal("withdrawal changed source configuration")
	}
	return authorised, denied
}

func TestConfigExportNativeWithdrawal(t *testing.T) {
	for _, transport := range []string{"header", "bearer", "session", "sso_session"} {
		t.Run(transport, func(t *testing.T) { configExportNativeWithdrawal(t, transport) })
	}
}
