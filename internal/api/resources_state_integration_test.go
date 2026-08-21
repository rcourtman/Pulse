package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	authpkg "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

type stateSupplementalRecordsProvider struct {
	records []unified.IngestRecord
}

func (p stateSupplementalRecordsProvider) GetCurrentRecords() []unified.IngestRecord {
	return append([]unified.IngestRecord(nil), p.records...)
}

func (p stateSupplementalRecordsProvider) SupplementalRecords(*monitoring.Monitor, string) []unified.IngestRecord {
	return p.GetCurrentRecords()
}

func TestStateEndpointDerivesProxmoxWorkloadParentFromSupplementalRecords(t *testing.T) {
	now := time.Date(2026, 5, 14, 10, 0, 0, 0, time.UTC)
	hashedPassword, err := authpkg.HashPassword("password")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	dataPath := t.TempDir()
	InitSessionStore(dataPath)
	InitCSRFStore(dataPath)
	cfg := &config.Config{DataPath: dataPath, AuthUser: "admin", AuthPass: hashedPassword}
	monitor, err := monitoring.New(cfg)
	if err != nil {
		t.Fatalf("new monitor: %v", err)
	}
	t.Cleanup(func() { monitor.Stop() })

	monitor.SetResourceStore(unified.NewMonitorAdapter(nil))
	monitor.SetSupplementalRecordsProvider(unified.SourceProxmox, stateSupplementalRecordsProvider{records: []unified.IngestRecord{
		{
			SourceID: "homelab-delly",
			Resource: unified.Resource{
				Type: unified.ResourceTypeAgent, Name: "delly", Status: unified.StatusOnline, LastSeen: now,
				Proxmox: &unified.ProxmoxData{SourceID: "homelab-delly", NodeName: "delly", ClusterName: "homelab", Instance: "delly"},
			},
			Identity: unified.ResourceIdentity{MachineID: "machine-delly", Hostnames: []string{"delly"}},
		},
		{
			SourceID: "delly:delly:104",
			Resource: unified.Resource{
				Type: unified.ResourceTypeSystemContainer, Name: "cloudflared", Status: unified.StatusOnline, LastSeen: now,
				Proxmox: &unified.ProxmoxData{SourceID: "delly:delly:104", NodeName: "delly", ClusterName: "homelab", Instance: "delly", VMID: 104},
			},
			Identity: unified.ResourceIdentity{Hostnames: []string{"cloudflared"}},
		},
	}})

	router := &Router{config: cfg, monitor: monitor}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/state", nil)
	req.SetBasicAuth("admin", "password")
	router.handleState(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("/api/state status = %d, body=%s", rec.Code, rec.Body.String())
	}

	var state models.StateFrontend
	if err := json.NewDecoder(rec.Body).Decode(&state); err != nil {
		t.Fatalf("decode /api/state: %v", err)
	}
	dellyCount, dellyID, cloudflaredParentID := 0, "", ""
	for _, resource := range state.Resources {
		switch {
		case resource.Type == string(unified.ResourceTypeAgent) && resource.Name == "delly":
			dellyCount++
			dellyID = resource.ID
		case resource.Type == string(unified.ResourceTypeSystemContainer) && resource.Name == "cloudflared":
			cloudflaredParentID = resource.ParentID
		}
	}
	if dellyCount != 1 || dellyID == "" || cloudflaredParentID != dellyID {
		t.Fatalf("unexpected /api/state resource projection: dellyCount=%d dellyID=%q cloudflaredParentID=%q resources=%#v", dellyCount, dellyID, cloudflaredParentID, state.Resources)
	}
}

// Closing must be safe to repeat and safe on a nil compatibility facade: the
// shutdown path runs before every dependency is guaranteed to be wired.
func TestResourceHandlers_CloseIsIdempotentAndNilSafe(t *testing.T) {
	var nilHandlers *ResourceHandlers
	if err := nilHandlers.CloseStores(); err != nil {
		t.Fatalf("nil handler CloseStores: %v", err)
	}
	if err := nilHandlers.CloseTenantStore("client-a"); err != nil {
		t.Fatalf("nil handler CloseTenantStore: %v", err)
	}

	handlers := NewResourceHandlers(&config.Config{DataPath: t.TempDir()})
	if _, err := handlers.getStore("client-a"); err != nil {
		t.Fatalf("getStore: %v", err)
	}
	if err := handlers.CloseStores(); err != nil {
		t.Fatalf("first CloseStores: %v", err)
	}
	if err := handlers.CloseStores(); err != nil {
		t.Fatalf("second CloseStores must be a no-op: %v", err)
	}
	if err := handlers.CloseTenantStore("client-a"); err != nil {
		t.Fatalf("CloseTenantStore on an evicted org must be a no-op: %v", err)
	}
}
