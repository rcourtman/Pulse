package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/reporting"
)

func TestReportingRuntimeSnapshotUsesMonitorUnifiedResources(t *testing.T) {
	total := int64(1000)
	used := int64(250)

	state := models.NewState()
	state.Nodes = []models.Node{{ID: "node-1", Name: "node-a"}}

	monitor := newReportingMonitorForTest(t, state, []unifiedresources.Resource{
		{
			ID:         "storage-1",
			Type:       unifiedresources.ResourceTypeStorage,
			Name:       "tank",
			Status:     unifiedresources.StatusOnline,
			ParentName: "node-a",
			Identity: unifiedresources.ResourceIdentity{
				Hostnames: []string{"node-a"},
			},
			Metrics: &unifiedresources.ResourceMetrics{
				Disk: &unifiedresources.MetricValue{
					Total: &total,
					Used:  &used,
				},
			},
			Storage: &unifiedresources.StorageMeta{
				Type:    "zfs",
				Content: "images",
			},
		},
		{
			ID:         "disk-1",
			Type:       unifiedresources.ResourceTypePhysicalDisk,
			Name:       "sda",
			ParentName: "node-a",
			Identity: unifiedresources.ResourceIdentity{
				Hostnames: []string{"node-a"},
			},
			PhysicalDisk: &unifiedresources.PhysicalDiskMeta{
				DevPath:     "/dev/sda",
				Model:       "Seagate",
				Serial:      "SER123",
				DiskType:    "hdd",
				SizeBytes:   1000,
				Health:      "PASSED",
				Temperature: 31,
				Wearout:     2,
			},
		},
	})

	handlers := NewReportingHandlers(newReportingMTMForTest(t, monitor), nil)
	snapshot, ok := handlers.getRuntimeStateSnapshot(context.Background(), "default")
	if !ok {
		t.Fatal("expected runtime snapshot to be available")
	}
	if len(snapshot.Storage) != 1 {
		t.Fatalf("expected 1 storage snapshot, got %d", len(snapshot.Storage))
	}
	if len(snapshot.Disks) != 1 {
		t.Fatalf("expected 1 disk snapshot, got %d", len(snapshot.Disks))
	}

	storage := snapshot.Storage[0]
	if storage.Name != "tank" || storage.Node != "node-a" || storage.Type != "zfs" {
		t.Fatalf("unexpected storage snapshot: %#v", storage)
	}
	if storage.Total != 1000 || storage.Used != 250 || storage.Available != 750 || storage.UsagePerc != 25 {
		t.Fatalf("unexpected storage metrics: %#v", storage)
	}

	disk := snapshot.Disks[0]
	if disk.Node != "node-a" || disk.Device != "/dev/sda" || disk.Serial != "SER123" {
		t.Fatalf("unexpected disk snapshot: %#v", disk)
	}
}

func TestReportingGenerateReportEnrichesNodeFromMonitorUnifiedResources(t *testing.T) {
	engine := &stubReportingEngine{data: []byte("report"), contentType: "application/pdf"}
	original := reporting.GetEngine()
	reporting.SetEngine(engine)
	t.Cleanup(func() { reporting.SetEngine(original) })

	total := int64(2048)
	used := int64(1024)

	state := models.NewState()
	state.Nodes = []models.Node{{ID: "node-1", Name: "node-a", Status: "online"}}

	monitor := newReportingMonitorForTest(t, state, []unifiedresources.Resource{
		{
			ID:         "storage-1",
			Type:       unifiedresources.ResourceTypeStorage,
			Name:       "pool-a",
			Status:     unifiedresources.StatusOnline,
			ParentName: "node-a",
			Identity: unifiedresources.ResourceIdentity{
				Hostnames: []string{"node-a"},
			},
			Metrics: &unifiedresources.ResourceMetrics{
				Disk: &unifiedresources.MetricValue{
					Total: &total,
					Used:  &used,
				},
			},
		},
		{
			ID:         "disk-1",
			Type:       unifiedresources.ResourceTypePhysicalDisk,
			Name:       "nvme0n1",
			ParentName: "node-a",
			Identity: unifiedresources.ResourceIdentity{
				Hostnames: []string{"node-a"},
			},
			PhysicalDisk: &unifiedresources.PhysicalDiskMeta{
				DevPath:   "/dev/nvme0n1",
				DiskType:  "nvme",
				SizeBytes: 2048,
				Health:    "PASSED",
			},
		},
	})

	handler := NewReportingHandlers(newReportingMTMForTest(t, monitor), nil)
	req := httptest.NewRequest(http.MethodGet, "/api/reporting?format=pdf&resourceType=node&resourceId=node-1", nil)
	rr := httptest.NewRecorder()

	handler.HandleGenerateReport(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
	if len(engine.lastReq.Storage) != 1 {
		t.Fatalf("expected node report storage enrichment, got %#v", engine.lastReq.Storage)
	}
	if len(engine.lastReq.Disks) != 1 {
		t.Fatalf("expected node report disk enrichment, got %#v", engine.lastReq.Disks)
	}
}

func newReportingMonitorForTest(t *testing.T, state *models.State, resources []unifiedresources.Resource) *monitoring.Monitor {
	t.Helper()

	registry := unifiedresources.NewRegistry(nil)
	records := make([]unifiedresources.IngestRecord, 0, len(resources))
	for i, resource := range resources {
		sourceID := resource.ID
		if sourceID == "" {
			sourceID = "resource"
		}
		records = append(records, unifiedresources.IngestRecord{
			SourceID: sourceID + "-" + string(rune('a'+i)),
			Resource: resource,
			Identity: resource.Identity,
		})
	}
	registry.IngestRecords(unifiedresources.SourceTrueNAS, records)

	monitor := &monitoring.Monitor{}
	setUnexportedField(t, monitor, "state", state)
	monitor.SetResourceStore(unifiedresources.NewMonitorAdapter(registry))
	return monitor
}

func newReportingMTMForTest(t *testing.T, monitor *monitoring.Monitor) *monitoring.MultiTenantMonitor {
	t.Helper()

	mtm := monitoring.NewMultiTenantMonitor(&config.Config{}, nil, nil)
	setUnexportedField(t, mtm, "monitors", map[string]*monitoring.Monitor{
		"default": monitor,
	})
	return mtm
}
