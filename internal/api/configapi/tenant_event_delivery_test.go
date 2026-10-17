package configapi

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	gorillaws "github.com/gorilla/websocket"
	"github.com/rcourtman/pulse-go-rewrite/internal/websocket"
)

func tenantEventConnections(t *testing.T, hub *websocket.Hub, orgID string) (*gorillaws.Conn, *gorillaws.Conn) {
	t.Helper()
	go hub.Run()
	t.Cleanup(hub.Stop)
	server := httptest.NewServer(http.HandlerFunc(hub.HandleWebSocket))
	t.Cleanup(server.Close)
	connect := func(org string) *gorillaws.Conn {
		headers := http.Header{"Origin": {server.URL}}
		conn, _, err := gorillaws.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"?org_id="+org, headers)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { conn.Close() })
		deadline := time.Now().Add(time.Second)
		for hub.GetTenantClientCount(org) == 0 {
			if time.Now().After(deadline) {
				t.Fatalf("client for %s did not register", org)
			}
			time.Sleep(time.Millisecond)
		}
		return conn
	}
	if orgID == "" {
		orgID = "default"
	}
	return connect(orgID), connect("org-b")
}

func assertTenantEvents(t *testing.T, owner, other *gorillaws.Conn, types ...string) {
	t.Helper()
	owner.SetReadDeadline(time.Now().Add(3 * time.Second))
	for _, want := range types {
		var msg websocket.Message
		if err := owner.ReadJSON(&msg); err != nil {
			t.Fatalf("read %s: %v", want, err)
		}
		if msg.Type != want {
			t.Fatalf("got %q, want %q", msg.Type, want)
		}
	}
	other.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	if _, data, err := other.ReadMessage(); err == nil {
		t.Fatalf("other organization received %s", data)
	} else if netErr, ok := err.(net.Error); !ok || !netErr.Timeout() {
		t.Fatalf("expected open idle connection, got %v", err)
	}
}
