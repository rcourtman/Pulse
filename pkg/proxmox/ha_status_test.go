package proxmox

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHAStatusEntryHelpers(t *testing.T) {
	tests := []struct {
		name            string
		entry           HAStatusEntry
		wantNode        string
		wantMaintenance bool
		wantState       string
	}{
		{
			name: "lrm with structured maintenance state",
			entry: HAStatusEntry{
				ID:     "lrm:pve1",
				Type:   "lrm",
				Node:   "pve1",
				State:  "maintenance",
				Status: "pve1 (maintenance mode, Wed Nov 25 20:00:00 2026)",
			},
			wantNode:        "pve1",
			wantMaintenance: true,
			wantState:       "maintenance",
		},
		{
			name: "lrm with status-only maintenance mode and id fallback",
			entry: HAStatusEntry{
				ID:     "lrm:pve2",
				Type:   "lrm",
				Status: "pve2 (maintenance mode, Wed Nov 25 20:00:00 2026)",
			},
			wantNode:        "pve2",
			wantMaintenance: true,
			wantState:       "maintenance",
		},
		{
			name: "lrm active node",
			entry: HAStatusEntry{
				ID:     "lrm:pve3",
				Type:   "lrm",
				Node:   "pve3",
				Status: "pve3 (active, Wed Nov 25 20:00:00 2026)",
			},
			wantNode:        "pve3",
			wantMaintenance: false,
			wantState:       "active",
		},
		{
			name: "service entry never treated as node maintenance",
			entry: HAStatusEntry{
				ID:     "service:vm:101",
				Type:   "service",
				Node:   "pve1",
				State:  "started",
				Status: "maintenance",
			},
			wantNode:        "pve1",
			wantMaintenance: false,
			wantState:       "started",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.entry.NodeName(); got != tt.wantNode {
				t.Errorf("NodeName() = %q, want %q", got, tt.wantNode)
			}
			if got := tt.entry.IsLRMMaintenance(); got != tt.wantMaintenance {
				t.Errorf("IsLRMMaintenance() = %v, want %v", got, tt.wantMaintenance)
			}
			if got := tt.entry.NormalizedLRMState(); got != tt.wantState {
				t.Errorf("NormalizedLRMState() = %q, want %q", got, tt.wantState)
			}
		})
	}
}

func TestGetClusterHAStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api2/json/cluster/ha/status/current" {
			t.Errorf("unexpected path %q", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":[
			{"id":"quorum","type":"quorum","status":"OK"},
			{"id":"master","type":"master","node":"pve1","status":"pve1 (active, Wed Nov 25 20:00:00 2026)"},
			{"id":"lrm:pve1","type":"lrm","node":"pve1","status":"pve1 (active, Wed Nov 25 20:00:00 2026)"},
			{"id":"lrm:pve2","type":"lrm","node":"pve2","state":"maintenance","status":"pve2 (maintenance mode, Wed Nov 25 20:00:00 2026)"}
		]}`)
	}))
	defer srv.Close()

	client, err := NewClient(ClientConfig{
		Host:       srv.URL,
		TokenName:  "pulse-monitor@pve!token",
		TokenValue: "secret",
		VerifySSL:  false,
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	entries, err := client.GetClusterHAStatus(context.Background())
	if err != nil {
		t.Fatalf("GetClusterHAStatus() error = %v", err)
	}
	if len(entries) != 4 {
		t.Fatalf("len(entries) = %d, want 4", len(entries))
	}
	if entries[2].IsLRMMaintenance() {
		t.Fatalf("expected pve1 active LRM not to be in maintenance")
	}
	if !entries[3].IsLRMMaintenance() || entries[3].NormalizedLRMState() != "maintenance" {
		t.Fatalf("expected pve2 LRM to be in maintenance, got %+v", entries[3])
	}
}
