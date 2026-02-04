package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	pulsews "github.com/rcourtman/pulse-go-rewrite/internal/websocket"
)

func newWebSocketRouter(t *testing.T, allowedOrigins []string, tokenRecord config.APITokenRecord) (*httptest.Server, func()) {
	t.Helper()

	cfg := newTestConfigWithTokens(t, tokenRecord)

	hub := pulsews.NewHub(nil)
	hub.SetAllowedOrigins(allowedOrigins)
	go hub.Run()

	router := NewRouter(cfg, nil, nil, hub, nil, "1.0.0")
	server := httptest.NewServer(router.Handler())

	cleanup := func() {
		server.Close()
		hub.Stop()
	}

	return server, cleanup
}

func TestWebSocketOriginRejectedWhenNotAllowed(t *testing.T) {
	rawToken := "ws-origin-reject-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeMonitoringRead}, nil)

	server, cleanup := newWebSocketRouter(t, []string{"https://allowed.example.com"}, record)
	defer cleanup()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	headers := http.Header{}
	headers.Set("X-API-Token", rawToken)
	headers.Set("Origin", "https://evil.example.com")

	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, headers)
	if err == nil {
		conn.Close()
		t.Fatalf("expected websocket origin rejection")
	}
	if resp == nil {
		t.Fatalf("expected HTTP response for rejected origin")
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, resp.StatusCode)
	}
}

func TestWebSocketOriginAllowedWhenConfigured(t *testing.T) {
	rawToken := "ws-origin-allow-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeMonitoringRead}, nil)

	server, cleanup := newWebSocketRouter(t, []string{"https://allowed.example.com"}, record)
	defer cleanup()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	headers := http.Header{}
	headers.Set("X-API-Token", rawToken)
	headers.Set("Origin", "https://allowed.example.com")

	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, headers)
	if err != nil {
		t.Fatalf("expected websocket connection, got %v", err)
	}
	if resp == nil || resp.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("expected 101 switching protocols, got %v", resp)
	}
	conn.Close()
}

func TestSocketIOWebSocketOriginRejected(t *testing.T) {
	rawToken := "socket-origin-reject-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeMonitoringRead}, nil)

	server, cleanup := newWebSocketRouter(t, []string{"https://allowed.example.com"}, record)
	defer cleanup()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/socket.io/?transport=websocket"
	headers := http.Header{}
	headers.Set("X-API-Token", rawToken)
	headers.Set("Origin", "https://evil.example.com")

	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, headers)
	if err == nil {
		conn.Close()
		t.Fatalf("expected websocket origin rejection for socket.io")
	}
	if resp == nil {
		t.Fatalf("expected HTTP response for rejected socket.io origin")
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, resp.StatusCode)
	}
}

func TestWebSocketOriginRejectedWhenNoAllowedOriginsAndPublicOrigin(t *testing.T) {
	rawToken := "ws-origin-default-reject-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeMonitoringRead}, nil)

	server, cleanup := newWebSocketRouter(t, []string{}, record)
	defer cleanup()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	headers := http.Header{}
	headers.Set("X-API-Token", rawToken)
	headers.Set("Origin", "https://evil.example.com")

	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, headers)
	if err == nil {
		conn.Close()
		t.Fatalf("expected websocket origin rejection with empty allowed origins")
	}
	if resp == nil {
		t.Fatalf("expected HTTP response for rejected origin")
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, resp.StatusCode)
	}
}

func TestWebSocketOriginAllowsPrivateWhenNoAllowedOrigins(t *testing.T) {
	rawToken := "ws-origin-default-allow-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeMonitoringRead}, nil)

	server, cleanup := newWebSocketRouter(t, []string{}, record)
	defer cleanup()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	headers := http.Header{}
	headers.Set("X-API-Token", rawToken)
	headers.Set("Origin", "http://localhost:3000")

	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, headers)
	if err != nil {
		t.Fatalf("expected websocket connection, got %v", err)
	}
	if resp == nil || resp.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("expected 101 switching protocols, got %v", resp)
	}
	conn.Close()
}
