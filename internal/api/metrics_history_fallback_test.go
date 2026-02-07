package api

import (
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
)

type metricsHistoryResponse struct {
	ResourceType string `json:"resourceType"`
	ResourceId   string `json:"resourceId"`
	Metric       string `json:"metric"`
	Range        string `json:"range"`
	Source       string `json:"source"`
	Points       []struct {
		Timestamp int64   `json:"timestamp"`
		Value     float64 `json:"value"`
	} `json:"points"`
}

func TestMetricsHistoryFallbackUsesLivePoint(t *testing.T) {
	state := models.NewState()
	vm := models.VM{
		ID:       "pve1:node1:101",
		VMID:     101,
		Name:     "vm-101",
		Node:     "node1",
		Instance: "pve1",
		Status:   "running",
		Type:     "qemu",
		CPU:      0.42,
		Memory: models.Memory{
			Usage: 55.0,
		},
		Disk: models.Disk{
			Usage: 33.0,
		},
	}
	state.UpdateVMsForInstance("pve1", []models.VM{vm})

	monitor := &monitoring.Monitor{}
	setUnexportedField(t, monitor, "state", state)
	setUnexportedField(t, monitor, "metricsHistory", monitoring.NewMetricsHistory(10, time.Hour))

	tempDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(tempDir)
	if _, err := mtp.GetPersistence("default"); err != nil {
		t.Fatalf("failed to init persistence: %v", err)
	}

	router := &Router{
		monitor:         monitor,
		licenseHandlers: NewLicenseHandlers(mtp),
	}

	req := httptest.NewRequest(http.MethodGet, "/api/metrics-store/history?resourceType=vm&resourceId=pve1:node1:101&metric=cpu&range=24h", nil)
	rec := httptest.NewRecorder()
	router.handleMetricsHistory(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var resp metricsHistoryResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Source != "live" {
		t.Fatalf("expected source live, got %q", resp.Source)
	}
	if len(resp.Points) != 1 {
		t.Fatalf("expected 1 point, got %d", len(resp.Points))
	}
	if math.Abs(resp.Points[0].Value-42.0) > 0.001 {
		t.Fatalf("expected value 42.0, got %f", resp.Points[0].Value)
	}
}

func TestMetricsHistoryFallbackMockDiskSynthesizesSeries(t *testing.T) {
	mock.SetEnabled(true)
	t.Cleanup(func() { mock.SetEnabled(false) })

	state := mock.GetMockState()
	var disk models.PhysicalDisk
	found := false
	for _, candidate := range state.PhysicalDisks {
		if candidate.Serial != "" && candidate.Temperature > 0 {
			disk = candidate
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected at least one mock physical disk with serial and temperature")
	}

	monitor := &monitoring.Monitor{}
	router := &Router{monitor: monitor}

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/metrics-store/history?resourceType=disk&resourceId="+disk.Serial+"&metric=smart_temp&range=1h",
		nil,
	)
	rec := httptest.NewRecorder()
	router.handleMetricsHistory(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var resp metricsHistoryResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Source != "mock_synthetic" {
		t.Fatalf("expected source mock_synthetic, got %q", resp.Source)
	}
	if len(resp.Points) < 24 {
		t.Fatalf("expected dense synthetic history points, got %d", len(resp.Points))
	}
	if math.Abs(resp.Points[len(resp.Points)-1].Value-float64(disk.Temperature)) > 0.001 {
		t.Fatalf("expected last point %d, got %f", disk.Temperature, resp.Points[len(resp.Points)-1].Value)
	}
}
