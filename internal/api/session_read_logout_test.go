package api

import (
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	pulsews "github.com/rcourtman/pulse-go-rewrite/internal/websocket"
)

// Use the actual CSRF-protected logout and persisted session store, not just
// an in-memory fixture deletion. This must not invalidate another session.
func logoutReadSession(t *testing.T, serverURL, raw string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, serverURL+"/api/logout", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.AddCookie(&http.Cookie{Name: cookieNameSession, Value: raw})
	req.Header.Set("X-CSRF-Token", generateCSRFToken(raw))
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil || response.StatusCode != http.StatusOK || ValidateSession(raw) {
		t.Fatalf("logout failed: %d %s %v", response.StatusCode, body, err)
	}
	persisted := NewSessionStore(GetSessionStore().dataPath)
	defer persisted.Shutdown()
	if persisted.GetSession(raw) != nil {
		t.Fatal("logout left persisted session authority")
	}
}

func TestSessionReadLogoutWebSocket(t *testing.T) {
	_, _, server, hub := newMonitoringWebSocketRouter(t)
	const primaryRaw, controlRaw = "synthetic-logout-socket", "synthetic-logout-socket-control"
	for _, raw := range []string{primaryRaw, controlRaw} {
		GetSessionStore().CreateSession(raw, time.Hour, "fixture", "127.0.0.1", "operator")
		t.Cleanup(func() { GetSessionStore().DeleteSession(raw) })
	}
	primary := openMonitoringWebSocket(t, server, primaryRaw, "session")
	control := openMonitoringWebSocket(t, server, controlRaw, "session")
	logoutReadSession(t, server.URL, primaryRaw)
	hub.BroadcastMessageToTenant("default", pulsews.Message{Type: "notification", Data: "private-after-logout"})
	if got := nextMonitoringWebSocketFrame(t, control); got.Data != "private-after-logout" {
		t.Fatalf("independent reader: %+v", got)
	}
	wantMonitoringWebSocketClosed(t, primary)
}

func TestSessionReadLogoutJSON(t *testing.T) {
	for _, kind := range []string{"execute", "kubernetes"} {
		t.Run(kind, func(t *testing.T) {
			f := newAssistantJSONFixture(t, kind, []string{config.ScopeAIExecute})
			const primaryRaw, controlRaw = "synthetic-logout-json", "synthetic-logout-json-control"
			for _, raw := range []string{primaryRaw, controlRaw} {
				GetSessionStore().CreateSession(raw, time.Hour, "fixture", "127.0.0.1", f.router.config.AuthUser)
				t.Cleanup(func() { GetSessionStore().DeleteSession(raw) })
			}
			primary := f.begin(t, "primary", primaryRaw, "session")
			control := f.begin(t, "control", controlRaw, "session")
			logoutReadSession(t, f.server.URL, primaryRaw)
			fresh, err := f.server.Client().Do(f.request(t, "primary", primaryRaw, "session"))
			if err != nil {
				t.Fatal(err)
			}
			fresh.Body.Close()
			if fresh.StatusCode != http.StatusUnauthorized || f.primary.calls.Load() != 1 {
				t.Fatalf("fresh logout admission = %d calls=%d", fresh.StatusCode, f.primary.calls.Load())
			}
			f.primary.unblock()
			f.control.unblock()
			readAssistantJSONResult(t, control, true)
			readAssistantJSONResult(t, primary, false)
			if f.primary.cancelled.Load() {
				t.Fatal("result refusal cancelled admitted work")
			}
		})
	}
}
