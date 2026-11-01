package api

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	gorillaws "github.com/gorilla/websocket"
	"github.com/rcourtman/pulse-go-rewrite/internal/logging"
	pulsews "github.com/rcourtman/pulse-go-rewrite/internal/websocket"
)

// This real registered HTTP/WS journey is also a safe, optional wire-evidence
// exporter for the existing System Logs and WebSocket browser consumers. The
// UI fixture itself is evidence only, never a repository test or product route.
func sessionReadBrowserEvidence(t *testing.T) {
	t.Helper()
	_, cfg, server, hub := newMonitoringWebSocketRouter(t)
	cfg.AuthUser = "operator"
	const raw = "synthetic-browser-session-read"
	const before = "synthetic permitted line before session logout"
	const forbidden = "synthetic forbidden line after session logout"
	GetSessionStore().CreateSession(raw, time.Hour, "fixture", "127.0.0.1", "operator")
	t.Cleanup(func() { GetSessionStore().DeleteSession(raw) })
	socket := openMonitoringWebSocket(t, server, raw, "session")
	logging.GetBroadcaster().Write([]byte(before))
	req, _ := http.NewRequest(http.MethodGet, server.URL+"/api/logs/stream", nil)
	req.AddCookie(&http.Cookie{Name: cookieNameSession, Value: raw})
	response, err := server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("log admission = %d", response.StatusCode)
	}
	reader := bufio.NewReader(response.Body)
	readLogStreamMarker(t, reader, before)
	logoutReadSession(t, server.URL, raw)
	logging.GetBroadcaster().Write([]byte(forbidden))
	wantLogStreamClosed(t, reader, forbidden)
	hub.BroadcastMessageToTenant("default", pulsews.Message{Type: "notification", Data: forbidden})
	_ = socket.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, body, err := socket.ReadMessage()
	var closed *gorillaws.CloseError
	if !errors.As(err, &closed) || closed.Code != gorillaws.ClosePolicyViolation || closed.Text != "Monitoring access ended" {
		t.Fatalf("session socket disclosed or closed unexpectedly: %s %v", body, err)
	}
	fresh, _ := http.NewRequest(http.MethodGet, server.URL+"/api/logs/stream", nil)
	fresh.AddCookie(&http.Cookie{Name: cookieNameSession, Value: raw})
	denied, err := server.Client().Do(fresh)
	if err != nil {
		t.Fatal(err)
	}
	defer denied.Body.Close()
	deniedBody, err := io.ReadAll(denied.Body)
	if err != nil || denied.StatusCode != http.StatusUnauthorized {
		t.Fatalf("reconnect = %d %s %v", denied.StatusCode, deniedBody, err)
	}
	securityResponse, err := server.Client().Get(server.URL + "/api/security/status")
	if err != nil {
		t.Fatal(err)
	}
	defer securityResponse.Body.Close()
	var securityStatus map[string]any
	if err := json.NewDecoder(securityResponse.Body).Decode(&securityStatus); err != nil || securityResponse.StatusCode != http.StatusOK || securityStatus["hasAuthentication"] != true {
		t.Fatalf("post-logout security status: %v %v", securityStatus, err)
	}
	evidence := map[string]any{
		"scope":     "Synthetic local full-Router HTTP/WebSocket producer and persisted logout, not installed/native-IdP acceptance",
		"logout":    map[string]any{"status": http.StatusOK, "persisted_session_absent": true},
		"logs":      map[string]any{"admission_status": response.StatusCode, "content_type": response.Header.Get("Content-Type"), "permitted_line": before, "after_logout_eof": true, "forbidden_line": forbidden, "reconnect_status": denied.StatusCode, "reconnect_body": string(deniedBody)},
		"websocket": map[string]any{"close_code": closed.Code, "close_reason": closed.Text},
	}
	if directory := os.Getenv("PULSE_SESSION_READ_EVIDENCE"); directory != "" {
		if err := os.MkdirAll(directory, 0700); err != nil {
			t.Fatal(err)
		}
		data, err := json.MarshalIndent(evidence, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(directory+"/session-read.json", append(data, '\n'), 0600); err != nil {
			t.Fatal(err)
		}
	}
}
