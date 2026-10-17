package configapi

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	gorillaws "github.com/gorilla/websocket"
	"github.com/rcourtman/pulse-go-rewrite/internal/api/apicontext"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/discovery"
	"github.com/rcourtman/pulse-go-rewrite/internal/websocket"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
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

func TestAutoRegistrationEventsStayWithRequestOrganization(t *testing.T) {
	for _, orgID := range []string{"org-a", "default", ""} {
		t.Run("org="+orgID, func(t *testing.T) {
			hub := websocket.NewHub(nil)
			owner, other := tenantEventConnections(t, hub, orgID)
			h := NewConfigHandlers(nil, nil, nil, hub, nil, nil, false)
			monitor, _, _ := newTestMonitor(t)
			service := discovery.NewService(nil, orgID, time.Hour, "auto", nil)
			service.Stop()
			setUnexportedField(t, monitor, "discoveryService", service)
			h.defaultMonitor = monitor
			ctx := context.WithValue(context.Background(), apicontext.OrgIDContextKey, orgID)
			req := &AutoRegisterRequest{Type: "pve", Source: "script"}
			h.notifyAutoRegistrationSuccess(ctx, req, "192.0.2.10", "node-a", "token-id", true)
			h.notifyAutoRegistrationSuccess(ctx, req, "192.0.2.10", "node-a", "token-id", false)
			h.notifyAutoUnregisterSuccess(ctx, &AutoUnregisterRequest{Type: "pve"})
			assertTenantEvents(t, owner, other,
				"node_auto_registered", "discovery_update", "nodes_changed", "discovery_update", "node_deleted", "discovery_update")
		})
	}
}

func TestNodeMutationEventsStayWithRequestOrganization(t *testing.T) {
	for _, orgID := range []string{"org-a", "default"} {
		t.Run(orgID, func(t *testing.T) {
			hub := websocket.NewHub(nil)
			owner, other := tenantEventConnections(t, hub, orgID)
			h := newTestConfigHandlers(t, &config.Config{DataPath: t.TempDir(), PVEInstances: []config.PVEInstance{
				{Name: "node-a", Host: "https://192.0.2.10:8006", TokenValue: "synthetic"},
			}})
			h.wsHub = hub
			original := detectPVECluster
			t.Cleanup(func() { detectPVECluster = original })
			detectPVECluster = func(proxmox.ClientConfig, string, []config.ClusterEndpoint) (bool, string, []config.ClusterEndpoint) {
				return true, "cluster-a", []config.ClusterEndpoint{{NodeName: "node-a", Host: "https://192.0.2.10:8006"}}
			}
			ctx := context.WithValue(context.Background(), apicontext.OrgIDContextKey, orgID)
			refresh := httptest.NewRequest(http.MethodPost, "/api/config/nodes/pve-0/refresh-cluster", nil).WithContext(ctx)
			rec := httptest.NewRecorder()
			h.HandleRefreshClusterNodes(rec, refresh)
			if rec.Code != http.StatusOK {
				t.Fatalf("refresh: %d %s", rec.Code, rec.Body.String())
			}
			remove := httptest.NewRequest(http.MethodDelete, "/api/config/nodes/pve-0", nil).WithContext(ctx)
			rec = httptest.NewRecorder()
			h.HandleDeleteNode(rec, remove)
			if rec.Code != http.StatusOK {
				t.Fatalf("delete: %d %s", rec.Code, rec.Body.String())
			}
			assertTenantEvents(t, owner, other, "nodes_updated", "node_deleted")
		})
	}
}
