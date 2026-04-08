package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/internal/vmware"
)

type fakeVMwareClient struct {
	testConnection func(context.Context) (*vmware.InventorySummary, error)
}

func (c *fakeVMwareClient) TestConnection(ctx context.Context) (*vmware.InventorySummary, error) {
	if c == nil || c.testConnection == nil {
		return &vmware.InventorySummary{}, nil
	}
	return c.testConnection(ctx)
}

func (c *fakeVMwareClient) Close() {}

func TestVMwareHandlers_HandleAdd_Success(t *testing.T) {
	setVMwareFeatureForTest(t, true)
	setMockModeForVMwareTest(t, false)

	handler, persistence := newVMwareHandlersForTest(t)

	body := marshalVMwareRequest(t, map[string]any{
		"name":     "lab-vcenter",
		"host":     "vcsa.lab.local",
		"port":     443,
		"username": "administrator@vsphere.local",
		"password": "super-secret",
		"enabled":  true,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/vmware/connections", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleAdd(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var created config.VMwareVCenterInstance
	if err := json.NewDecoder(rec.Body).Decode(&created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if created.ID == "" {
		t.Fatalf("expected generated ID, got empty")
	}
	if created.Password != "********" {
		t.Fatalf("expected password redacted, got %q", created.Password)
	}

	stored, err := persistence.LoadVMwareConfig()
	if err != nil {
		t.Fatalf("load saved config: %v", err)
	}
	if len(stored) != 1 {
		t.Fatalf("expected 1 saved instance, got %d", len(stored))
	}
	if stored[0].Password != "super-secret" {
		t.Fatalf("expected unredacted password persisted, got %q", stored[0].Password)
	}
}

func TestVMwareHandlers_HandleAdd_ValidationAndFeatureGate(t *testing.T) {
	t.Run("missing host", func(t *testing.T) {
		setVMwareFeatureForTest(t, true)
		setMockModeForVMwareTest(t, false)
		handler, _ := newVMwareHandlersForTest(t)

		body := marshalVMwareRequest(t, map[string]any{
			"username": "administrator@vsphere.local",
			"password": "super-secret",
		})
		req := httptest.NewRequest(http.MethodPost, "/api/vmware/connections", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		handler.HandleAdd(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("feature disabled", func(t *testing.T) {
		setVMwareFeatureForTest(t, false)
		setMockModeForVMwareTest(t, false)
		handler, _ := newVMwareHandlersForTest(t)

		body := marshalVMwareRequest(t, map[string]any{
			"host":     "vcsa.lab.local",
			"username": "administrator@vsphere.local",
			"password": "super-secret",
		})
		req := httptest.NewRequest(http.MethodPost, "/api/vmware/connections", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		handler.HandleAdd(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "explicitly disabled") {
			t.Fatalf("expected explicit disable message, got %s", rec.Body.String())
		}
	})
}

func TestVMwareHandlers_HandleAdd_BlocksProjectedNetNewSystemsAtLimit(t *testing.T) {
	setVMwareFeatureForTest(t, true)
	setMockModeForVMwareTest(t, false)
	setMaxMonitoredSystemsLicenseForTests(t, 1)

	handler, persistence := newVMwareHandlersForTest(t)
	registry := unifiedresources.NewRegistry(nil)
	registry.IngestRecords(unifiedresources.SourceAgent, []unifiedresources.IngestRecord{
		{
			SourceID: "host-1",
			Resource: unifiedresources.Resource{
				ID:     "host-1",
				Type:   unifiedresources.ResourceTypeAgent,
				Name:   "existing.local",
				Status: unifiedresources.StatusOnline,
				Agent: &unifiedresources.AgentData{
					AgentID:   "agent-1",
					Hostname:  "existing.local",
					MachineID: "machine-1",
				},
				Identity: unifiedresources.ResourceIdentity{
					MachineID: "machine-1",
					Hostnames: []string{"existing.local"},
				},
			},
		},
	})
	monitor := &monitoring.Monitor{}
	monitor.SetResourceStore(unifiedresources.NewMonitorAdapter(registry))
	handler.getMonitor = func(context.Context) *monitoring.Monitor { return monitor }
	handler.previewRecords = func(context.Context, config.VMwareVCenterInstance) ([]unifiedresources.IngestRecord, error) {
		return []unifiedresources.IngestRecord{
			{
				SourceID: "vc-1-host-1",
				Resource: unifiedresources.Resource{
					Type:   unifiedresources.ResourceTypeAgent,
					Name:   "esxi-02.lab.local",
					Status: unifiedresources.StatusOnline,
					VMware: &unifiedresources.VMwareData{
						ConnectionID:    "vc-1",
						ManagedObjectID: "host-1",
						EntityType:      "host",
						HostUUID:        "vmware-host-2",
					},
				},
				Identity: unifiedresources.ResourceIdentity{
					DMIUUID:   "vmware-host-2",
					Hostnames: []string{"esxi-02.lab.local"},
				},
			},
		}, nil
	}

	body := marshalVMwareRequest(t, map[string]any{
		"id":       "vc-1",
		"name":     "lab-vcenter",
		"host":     "vcsa.lab.local",
		"port":     443,
		"username": "administrator@vsphere.local",
		"password": "super-secret",
		"enabled":  true,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/vmware/connections", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleAdd(rec, req)

	if rec.Code != http.StatusPaymentRequired {
		t.Fatalf("expected 402 once projected VMware inventory exceeds the cap, got %d: %s", rec.Code, rec.Body.String())
	}
	payload := decodeMonitoredSystemLimitBlockedPayload(t, rec.Body.Bytes())
	if payload.Error != "license_required" {
		t.Fatalf("error=%q, want license_required", payload.Error)
	}
	if payload.Feature != maxMonitoredSystemsLicenseGateKey {
		t.Fatalf("feature=%q, want %q", payload.Feature, maxMonitoredSystemsLicenseGateKey)
	}
	if !payload.MonitoredSystemPreview.WouldExceedLimit {
		t.Fatalf("expected monitored_system_preview.would_exceed_limit=true, got %+v", payload.MonitoredSystemPreview)
	}
	if payload.MonitoredSystemPreview.Effect != "creates_new" {
		t.Fatalf("effect=%q, want creates_new", payload.MonitoredSystemPreview.Effect)
	}
	if payload.MonitoredSystemPreview.AdditionalCount != 1 {
		t.Fatalf("additional_count=%d, want 1", payload.MonitoredSystemPreview.AdditionalCount)
	}
	if len(payload.MonitoredSystemPreview.ProjectedSystems) != 1 {
		t.Fatalf("len(projected_systems)=%d, want 1", len(payload.MonitoredSystemPreview.ProjectedSystems))
	}

	stored, err := persistence.LoadVMwareConfig()
	if err != nil {
		t.Fatalf("load vmware config: %v", err)
	}
	if len(stored) != 0 {
		t.Fatalf("expected blocked VMware add not to persist, got %d connections", len(stored))
	}
}

func TestVMwareHandlers_HandleAdd_AllowsCanonicalOverlapAtLimit(t *testing.T) {
	setVMwareFeatureForTest(t, true)
	setMockModeForVMwareTest(t, false)
	setMaxMonitoredSystemsLicenseForTests(t, 1)

	handler, persistence := newVMwareHandlersForTest(t)
	registry := unifiedresources.NewRegistry(nil)
	registry.IngestRecords(unifiedresources.SourceAgent, []unifiedresources.IngestRecord{
		{
			SourceID: "host-1",
			Resource: unifiedresources.Resource{
				ID:     "host-1",
				Type:   unifiedresources.ResourceTypeAgent,
				Name:   "esxi-01.lab.local",
				Status: unifiedresources.StatusOnline,
				Agent: &unifiedresources.AgentData{
					AgentID:   "agent-1",
					Hostname:  "esxi-01.lab.local",
					MachineID: "machine-1",
				},
				Identity: unifiedresources.ResourceIdentity{
					MachineID: "machine-1",
					Hostnames: []string{"esxi-01.lab.local"},
				},
			},
		},
	})
	monitor := &monitoring.Monitor{}
	monitor.SetResourceStore(unifiedresources.NewMonitorAdapter(registry))
	handler.getMonitor = func(context.Context) *monitoring.Monitor { return monitor }
	handler.previewRecords = func(context.Context, config.VMwareVCenterInstance) ([]unifiedresources.IngestRecord, error) {
		return []unifiedresources.IngestRecord{
			{
				SourceID: "vc-1-host-1",
				Resource: unifiedresources.Resource{
					Type:   unifiedresources.ResourceTypeAgent,
					Name:   "esxi-01.lab.local",
					Status: unifiedresources.StatusOnline,
					VMware: &unifiedresources.VMwareData{
						ConnectionID:    "vc-1",
						ManagedObjectID: "host-1",
						EntityType:      "host",
						HostUUID:        "vmware-host-1",
					},
				},
				Identity: unifiedresources.ResourceIdentity{
					DMIUUID:   "vmware-host-1",
					Hostnames: []string{"esxi-01.lab.local"},
				},
			},
		}, nil
	}

	body := marshalVMwareRequest(t, map[string]any{
		"id":       "vc-1",
		"name":     "lab-vcenter",
		"host":     "vcsa.lab.local",
		"port":     443,
		"username": "administrator@vsphere.local",
		"password": "super-secret",
		"enabled":  true,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/vmware/connections", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleAdd(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected overlapping VMware add to be allowed at limit, got %d: %s", rec.Code, rec.Body.String())
	}

	stored, err := persistence.LoadVMwareConfig()
	if err != nil {
		t.Fatalf("load vmware config: %v", err)
	}
	if len(stored) != 1 {
		t.Fatalf("expected allowed VMware add to persist, got %d connections", len(stored))
	}
}

func TestVMwareHandlers_HandleList_RedactsSensitiveFieldsAndIncludesRuntimeSummary(t *testing.T) {
	setVMwareFeatureForTest(t, true)

	handler, persistence := newVMwareHandlersForTest(t)
	poller := monitoring.NewVMwarePoller(nil, time.Minute)
	handler.getPoller = func(context.Context) *monitoring.VMwarePoller { return poller }
	if err := persistence.SaveVMwareConfig([]config.VMwareVCenterInstance{
		{
			ID:                 "vc-1",
			Name:               "lab-a",
			Host:               "vcsa-a.lab.local",
			Port:               443,
			Username:           "administrator@vsphere.local",
			Password:           "secret-a",
			InsecureSkipVerify: false,
			Enabled:            true,
		},
	}); err != nil {
		t.Fatalf("seed vmware config: %v", err)
	}

	recordedAt := time.Date(2026, 3, 30, 10, 11, 12, 0, time.UTC)
	poller.RecordConnectionTestSuccess("default", "vc-1", &vmware.InventorySummary{
		Hosts:      3,
		VMs:        42,
		Datastores: 6,
		VIRelease:  "8.0.3",
	}, recordedAt)

	req := httptest.NewRequest(http.MethodGet, "/api/vmware/connections", nil)
	rec := httptest.NewRecorder()
	handler.HandleList(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var listed []vmwareConnectionResponse
	if err := json.NewDecoder(rec.Body).Decode(&listed); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("expected 1 listed instance, got %d", len(listed))
	}
	if listed[0].Password != "********" {
		t.Fatalf("expected password to be redacted, got %q", listed[0].Password)
	}
	if listed[0].Poll == nil || listed[0].Poll.LastSuccessAt == nil {
		t.Fatalf("expected saved test runtime summary, got %+v", listed[0].Poll)
	}
	if listed[0].Observed == nil {
		t.Fatalf("expected observed summary, got nil")
	}
	if listed[0].Observed.Hosts != 3 || listed[0].Observed.VMs != 42 || listed[0].Observed.Datastores != 6 {
		t.Fatalf("unexpected observed counts: %+v", listed[0].Observed)
	}
	if listed[0].Observed.VIRelease != "8.0.3" {
		t.Fatalf("unexpected VI release: %+v", listed[0].Observed)
	}
}

func TestVMwareHandlers_HandleList_ReturnsMockConnectionsInMockMode(t *testing.T) {
	setVMwareFeatureForTest(t, true)
	setMockModeForVMwareTest(t, true)

	handler, _ := newVMwareHandlersForTest(t)

	req := httptest.NewRequest(http.MethodGet, "/api/vmware/connections", nil)
	rec := httptest.NewRecorder()
	handler.HandleList(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var listed []vmwareConnectionResponse
	if err := json.NewDecoder(rec.Body).Decode(&listed); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("expected 1 mock VMware connection, got %d", len(listed))
	}
	if listed[0].Host != "vcsa.lab.local" {
		t.Fatalf("expected mock VMware host vcsa.lab.local, got %q", listed[0].Host)
	}
	if listed[0].Password != "********" {
		t.Fatalf("expected redacted mock VMware password, got %q", listed[0].Password)
	}
	if listed[0].Poll == nil || listed[0].Poll.LastSuccessAt == nil {
		t.Fatalf("expected mock VMware poll summary, got %+v", listed[0].Poll)
	}
	if listed[0].Observed == nil || listed[0].Observed.Hosts == 0 || listed[0].Observed.VMs == 0 {
		t.Fatalf("expected populated mock VMware observed summary, got %+v", listed[0].Observed)
	}
}

func TestVMwareHandlers_HandleDelete_RemovesAndClearsRuntimeSummary(t *testing.T) {
	setVMwareFeatureForTest(t, true)
	setMockModeForVMwareTest(t, false)

	handler, persistence := newVMwareHandlersForTest(t)
	if err := persistence.SaveVMwareConfig([]config.VMwareVCenterInstance{
		{ID: "alpha", Host: "vcsa-a.lab.local", Username: "admin", Password: "a"},
		{ID: "beta", Host: "vcsa-b.lab.local", Username: "admin", Password: "b"},
	}); err != nil {
		t.Fatalf("seed vmware config: %v", err)
	}
	handler.recordTestSuccess("alpha", &vmware.InventorySummary{Hosts: 1}, time.Now().UTC())

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/vmware/connections/alpha", nil)
	deleteRec := httptest.NewRecorder()
	handler.HandleDelete(deleteRec, deleteReq)

	if deleteRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", deleteRec.Code, deleteRec.Body.String())
	}

	stored, err := persistence.LoadVMwareConfig()
	if err != nil {
		t.Fatalf("load persisted config: %v", err)
	}
	if len(stored) != 1 || stored[0].ID != "beta" {
		t.Fatalf("expected only beta to remain, got %+v", stored)
	}
	if status := handler.runtimeStatus("alpha"); status.Poll != nil || status.Observed != nil {
		t.Fatalf("expected runtime summary to be cleared, got %+v", status)
	}
}

func TestVMwareHandlers_HandleList_CarriesDegradedObservedSummary(t *testing.T) {
	setVMwareFeatureForTest(t, true)

	handler, persistence := newVMwareHandlersForTest(t)
	if err := persistence.SaveVMwareConfig([]config.VMwareVCenterInstance{
		{
			ID:       "vc-1",
			Name:     "lab-a",
			Host:     "vcsa-a.lab.local",
			Port:     443,
			Username: "administrator@vsphere.local",
			Password: "secret-a",
			Enabled:  true,
		},
	}); err != nil {
		t.Fatalf("seed vmware config: %v", err)
	}

	recordedAt := time.Date(2026, 3, 31, 10, 11, 12, 0, time.UTC)
	handler.statusMu.Lock()
	handler.statuses = map[string]vmwareConnectionRuntimeStatus{
		"vc-1": {
			Poll: &monitoring.VMwareConnectionPollStatus{
				IntervalSeconds: 60,
				LastSuccessAt:   &recordedAt,
			},
			Observed: &monitoring.VMwareConnectionObservedSummary{
				CollectedAt: &recordedAt,
				Hosts:       3,
				VMs:         42,
				Datastores:  6,
				VIRelease:   "8.0.3",
				Degraded:    true,
				IssueCount:  2,
				Issues: []monitoring.VMwareConnectionObservedIssue{
					{Stage: "signals", Category: "permission", Message: "VMware permissions are insufficient for host overall status", Occurrences: 2},
				},
			},
		},
	}
	handler.statusMu.Unlock()

	req := httptest.NewRequest(http.MethodGet, "/api/vmware/connections", nil)
	rec := httptest.NewRecorder()
	handler.HandleList(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var listed []vmwareConnectionResponse
	if err := json.NewDecoder(rec.Body).Decode(&listed); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(listed) != 1 || listed[0].Observed == nil {
		t.Fatalf("expected degraded observed summary, got %+v", listed)
	}
	if !listed[0].Observed.Degraded || listed[0].Observed.IssueCount != 2 {
		t.Fatalf("unexpected degraded observed summary: %+v", listed[0].Observed)
	}
	if len(listed[0].Observed.Issues) != 1 || listed[0].Observed.Issues[0].Occurrences != 2 {
		t.Fatalf("unexpected observed issues: %+v", listed[0].Observed.Issues)
	}
}

func TestVMwareHandlers_HandleUpdate_PreservesMaskedSecretsAndReplacesFields(t *testing.T) {
	setVMwareFeatureForTest(t, true)
	setMockModeForVMwareTest(t, false)

	handler, persistence := newVMwareHandlersForTest(t)
	if err := persistence.SaveVMwareConfig([]config.VMwareVCenterInstance{
		{
			ID:                 "alpha",
			Name:               "old-name",
			Host:               "old.lab.local",
			Port:               443,
			Username:           "administrator@vsphere.local",
			Password:           "super-secret",
			InsecureSkipVerify: false,
			Enabled:            true,
		},
	}); err != nil {
		t.Fatalf("seed vmware config: %v", err)
	}

	body := marshalVMwareRequest(t, map[string]any{
		"id":                 "ignored-id",
		"name":               "new-name",
		"host":               "new.lab.local",
		"port":               8443,
		"username":           "operator@vsphere.local",
		"password":           "********",
		"insecureSkipVerify": true,
		"enabled":            true,
	})

	req := httptest.NewRequest(http.MethodPut, "/api/vmware/connections/alpha", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleUpdate(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var updated config.VMwareVCenterInstance
	if err := json.NewDecoder(rec.Body).Decode(&updated); err != nil {
		t.Fatalf("decode update response: %v", err)
	}
	if updated.ID != "alpha" {
		t.Fatalf("expected path ID to win, got %q", updated.ID)
	}
	if updated.Password != "********" {
		t.Fatalf("expected password to remain redacted, got %q", updated.Password)
	}

	stored, err := persistence.LoadVMwareConfig()
	if err != nil {
		t.Fatalf("load persisted config: %v", err)
	}
	if len(stored) != 1 {
		t.Fatalf("expected 1 stored instance, got %d", len(stored))
	}
	if stored[0].Host != "new.lab.local" || stored[0].Port != 8443 {
		t.Fatalf("expected updated endpoint to persist, got %+v", stored[0])
	}
	if stored[0].Password != "super-secret" {
		t.Fatalf("expected masked password to preserve stored secret, got %q", stored[0].Password)
	}
	if !stored[0].InsecureSkipVerify {
		t.Fatalf("expected insecureSkipVerify update to persist")
	}
}

func TestVMwareHandlers_HandleUpdate_BlocksProjectedNetNewSystemsAtLimit(t *testing.T) {
	setVMwareFeatureForTest(t, true)
	setMockModeForVMwareTest(t, false)
	setMaxMonitoredSystemsLicenseForTests(t, 1)

	handler, persistence := newVMwareHandlersForTest(t)
	if err := persistence.SaveVMwareConfig([]config.VMwareVCenterInstance{{
		ID:                 "alpha",
		Name:               "vc-a",
		Host:               "vc-a.lab.local",
		Port:               443,
		Username:           "administrator@vsphere.local",
		Password:           "super-secret",
		InsecureSkipVerify: true,
		Enabled:            true,
	}}); err != nil {
		t.Fatalf("seed vmware config: %v", err)
	}

	monitor := &monitoring.Monitor{}
	registry := unifiedresources.NewRegistry(nil)
	registry.IngestRecords(unifiedresources.SourceAgent, []unifiedresources.IngestRecord{
		{
			SourceID: "host-1",
			Resource: unifiedresources.Resource{
				ID:     "host-1",
				Type:   unifiedresources.ResourceTypeAgent,
				Name:   "archive.local",
				Status: unifiedresources.StatusOnline,
				Agent: &unifiedresources.AgentData{
					AgentID:   "agent-1",
					Hostname:  "archive.local",
					MachineID: "machine-1",
				},
				Identity: unifiedresources.ResourceIdentity{
					MachineID: "machine-1",
					Hostnames: []string{"archive.local"},
				},
			},
		},
	})
	registry.IngestRecords(unifiedresources.SourceVMware, []unifiedresources.IngestRecord{
		{
			SourceID: "vmware:alpha:host-1",
			Resource: unifiedresources.Resource{
				ID:     "vmware-host-1",
				Type:   unifiedresources.ResourceTypeAgent,
				Name:   "archive.local",
				Status: unifiedresources.StatusOnline,
				VMware: &unifiedresources.VMwareData{
					ConnectionID:    "alpha",
					ConnectionName:  "vc-a",
					VCenterHost:     "vc-a.lab.local",
					ManagedObjectID: "host-1",
					EntityType:      "host",
					HostUUID:        "vmware-host-1",
				},
			},
			Identity: unifiedresources.ResourceIdentity{
				MachineID: "machine-1",
				Hostnames: []string{"archive.local"},
			},
		},
	})
	monitor.SetResourceStore(unifiedresources.NewMonitorAdapter(registry))
	handler.getMonitor = func(context.Context) *monitoring.Monitor { return monitor }
	handler.previewRecords = func(context.Context, config.VMwareVCenterInstance) ([]unifiedresources.IngestRecord, error) {
		return []unifiedresources.IngestRecord{
			{
				SourceID: "vmware:alpha:host-2",
				Resource: unifiedresources.Resource{
					ID:     "vmware-host-2",
					Type:   unifiedresources.ResourceTypeAgent,
					Name:   "backup.local",
					Status: unifiedresources.StatusOnline,
					VMware: &unifiedresources.VMwareData{
						ConnectionID:    "alpha",
						ConnectionName:  "vc-a",
						VCenterHost:     "vc-b.lab.local",
						ManagedObjectID: "host-2",
						EntityType:      "host",
						HostUUID:        "vmware-host-2",
					},
				},
				Identity: unifiedresources.ResourceIdentity{
					Hostnames: []string{"backup.local"},
				},
			},
		}, nil
	}

	body := marshalVMwareRequest(t, map[string]any{
		"name":               "vc-b",
		"host":               "vc-b.lab.local",
		"port":               443,
		"username":           "administrator@vsphere.local",
		"password":           "********",
		"insecureSkipVerify": true,
		"enabled":            true,
	})
	req := httptest.NewRequest(http.MethodPut, "/api/vmware/connections/alpha", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleUpdate(rec, req)

	if rec.Code != http.StatusPaymentRequired {
		t.Fatalf("expected 402 when update would add a new monitored system, got %d: %s", rec.Code, rec.Body.String())
	}
	payload := decodeMonitoredSystemLimitBlockedPayload(t, rec.Body.Bytes())
	if !payload.MonitoredSystemPreview.WouldExceedLimit {
		t.Fatalf("expected monitored_system_preview.would_exceed_limit=true, got %+v", payload.MonitoredSystemPreview)
	}
	if payload.MonitoredSystemPreview.Effect != "splits_existing" {
		t.Fatalf("effect=%q, want splits_existing", payload.MonitoredSystemPreview.Effect)
	}
	if payload.MonitoredSystemPreview.AdditionalCount != 1 {
		t.Fatalf("additional_count=%d, want 1", payload.MonitoredSystemPreview.AdditionalCount)
	}
	if len(payload.MonitoredSystemPreview.CurrentSystems) != 1 {
		t.Fatalf("len(current_systems)=%d, want 1", len(payload.MonitoredSystemPreview.CurrentSystems))
	}
	if len(payload.MonitoredSystemPreview.ProjectedSystems) != 1 {
		t.Fatalf("len(projected_systems)=%d, want 1", len(payload.MonitoredSystemPreview.ProjectedSystems))
	}

	stored, err := persistence.LoadVMwareConfig()
	if err != nil {
		t.Fatalf("load persisted config: %v", err)
	}
	if stored[0].Host != "vc-a.lab.local" {
		t.Fatalf("expected blocked update to preserve original host, got %+v", stored[0])
	}
}

func TestVMwareHandlers_HandleTestConnection_SuccessAndFailure(t *testing.T) {
	setVMwareFeatureForTest(t, true)

	handler, _ := newVMwareHandlersForTest(t)
	var gotConfig vmware.ClientConfig
	handler.newClient = func(cfg vmware.ClientConfig) (vmwareClient, error) {
		gotConfig = cfg
		return &fakeVMwareClient{
			testConnection: func(context.Context) (*vmware.InventorySummary, error) {
				return &vmware.InventorySummary{Hosts: 1, VMs: 2, Datastores: 3, VIRelease: "8.0.3"}, nil
			},
		}, nil
	}

	successBody := marshalVMwareRequest(t, map[string]any{
		"host":     "vcsa.lab.local",
		"port":     8443,
		"username": "administrator@vsphere.local",
		"password": "super-secret",
	})
	successReq := httptest.NewRequest(http.MethodPost, "/api/vmware/connections/test", bytes.NewReader(successBody))
	successRec := httptest.NewRecorder()
	handler.HandleTestConnection(successRec, successReq)

	if successRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", successRec.Code, successRec.Body.String())
	}
	if gotConfig.Host != "vcsa.lab.local" || gotConfig.Port != 8443 {
		t.Fatalf("unexpected client config: %+v", gotConfig)
	}

	handler.newClient = nil
	failureBody := marshalVMwareRequest(t, map[string]any{
		"host":     "http://127.0.0.1/path",
		"username": "administrator@vsphere.local",
		"password": "super-secret",
	})
	failureReq := httptest.NewRequest(http.MethodPost, "/api/vmware/connections/test", bytes.NewReader(failureBody))
	failureRec := httptest.NewRecorder()
	handler.HandleTestConnection(failureRec, failureReq)

	if failureRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for bad host, got %d: %s", failureRec.Code, failureRec.Body.String())
	}

	handler.newClient = func(cfg vmware.ClientConfig) (vmwareClient, error) {
		return &fakeVMwareClient{
			testConnection: func(context.Context) (*vmware.InventorySummary, error) {
				return nil, errors.New("boom")
			},
		}, nil
	}
	errorBody := marshalVMwareRequest(t, map[string]any{
		"host":     "vcsa.lab.local",
		"username": "administrator@vsphere.local",
		"password": "super-secret",
	})
	errorReq := httptest.NewRequest(http.MethodPost, "/api/vmware/connections/test", bytes.NewReader(errorBody))
	errorRec := httptest.NewRecorder()
	handler.HandleTestConnection(errorRec, errorReq)

	if errorRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for failing connection, got %d: %s", errorRec.Code, errorRec.Body.String())
	}
}

func TestVMwareHandlers_HandleTestConnection_PreservesUnsupportedVersionCategory(t *testing.T) {
	setVMwareFeatureForTest(t, true)

	handler, _ := newVMwareHandlersForTest(t)
	handler.newClient = func(cfg vmware.ClientConfig) (vmwareClient, error) {
		return &fakeVMwareClient{
			testConnection: func(context.Context) (*vmware.InventorySummary, error) {
				return nil, &vmware.ConnectionError{
					Category: "unsupported_version",
					Message:  "VMware vCenter version is outside the implemented VI JSON probe floor; Pulse currently probes 9.0.0.0, 8.0.3, 8.0.2.0, 8.0.1.0",
				}
			},
		}, nil
	}

	body := marshalVMwareRequest(t, map[string]any{
		"host":     "vcsa.lab.local",
		"username": "administrator@vsphere.local",
		"password": "super-secret",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/vmware/connections/test", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleTestConnection(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}

	var payload struct {
		Code    string            `json:"code"`
		Message string            `json:"message"`
		Details map[string]string `json:"details"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if payload.Code != "vmware_connection_failed" {
		t.Fatalf("unexpected code: %+v", payload)
	}
	if payload.Details["category"] != "unsupported_version" {
		t.Fatalf("expected unsupported_version category, got %+v", payload.Details)
	}
}

func TestVMwareHandlers_HandlePreviewConnection_ReturnsCanonicalMultiSystemImpact(t *testing.T) {
	setVMwareFeatureForTest(t, true)
	setMaxMonitoredSystemsLicenseForTests(t, 1)

	handler, _ := newVMwareHandlersForTest(t)
	registry := unifiedresources.NewRegistry(nil)
	registry.IngestRecords(unifiedresources.SourceAgent, []unifiedresources.IngestRecord{
		{
			SourceID: "agent-host-1",
			Resource: unifiedresources.Resource{
				Type:   unifiedresources.ResourceTypeAgent,
				Name:   "esxi-01.lab.local",
				Status: unifiedresources.StatusOnline,
			},
			Identity: unifiedresources.ResourceIdentity{
				DMIUUID:   "uuid-host-1",
				Hostnames: []string{"esxi-01.lab.local"},
			},
		},
	})
	monitor := &monitoring.Monitor{}
	monitor.SetResourceStore(unifiedresources.NewMonitorAdapter(registry))
	handler.getMonitor = func(context.Context) *monitoring.Monitor { return monitor }
	handler.previewRecords = func(context.Context, config.VMwareVCenterInstance) ([]unifiedresources.IngestRecord, error) {
		return []unifiedresources.IngestRecord{
			{
				SourceID: "vc-1:host:host-101",
				Resource: unifiedresources.Resource{
					Type:   unifiedresources.ResourceTypeAgent,
					Name:   "esxi-01.lab.local",
					Status: unifiedresources.StatusOnline,
					VMware: &unifiedresources.VMwareData{
						ConnectionID:    "vc-1",
						ConnectionName:  "Lab VC",
						ManagedObjectID: "host-101",
						EntityType:      "host",
						HostUUID:        "uuid-host-1",
					},
				},
				Identity: unifiedresources.ResourceIdentity{
					DMIUUID:   "uuid-host-1",
					Hostnames: []string{"esxi-01.lab.local"},
				},
			},
			{
				SourceID: "vc-1:host:host-102",
				Resource: unifiedresources.Resource{
					Type:   unifiedresources.ResourceTypeAgent,
					Name:   "esxi-02.lab.local",
					Status: unifiedresources.StatusOnline,
					VMware: &unifiedresources.VMwareData{
						ConnectionID:    "vc-1",
						ConnectionName:  "Lab VC",
						ManagedObjectID: "host-102",
						EntityType:      "host",
						HostUUID:        "uuid-host-2",
					},
				},
				Identity: unifiedresources.ResourceIdentity{
					DMIUUID:   "uuid-host-2",
					Hostnames: []string{"esxi-02.lab.local"},
				},
			},
		}, nil
	}

	body := marshalVMwareRequest(t, map[string]any{
		"host":     "vcsa.lab.local",
		"username": "administrator@vsphere.local",
		"password": "super-secret",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/vmware/connections/preview", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandlePreviewConnection(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var preview MonitoredSystemLedgerPreviewResponse
	if err := json.NewDecoder(rec.Body).Decode(&preview); err != nil {
		t.Fatalf("decode preview response: %v", err)
	}
	if preview.CurrentCount != 1 || preview.ProjectedCount != 2 || preview.AdditionalCount != 1 {
		t.Fatalf("unexpected preview counts: %+v", preview)
	}
	if !preview.WouldExceedLimit {
		t.Fatalf("expected preview to report limit overrun, got %+v", preview)
	}
	if len(preview.CurrentSystems) != 1 {
		t.Fatalf("len(CurrentSystems) = %d, want 1", len(preview.CurrentSystems))
	}
	if len(preview.ProjectedSystems) != 2 {
		t.Fatalf("len(ProjectedSystems) = %d, want 2", len(preview.ProjectedSystems))
	}
}

func TestVMwareHandlers_HandlePreviewSavedConnection_PreservesStoredSecrets(t *testing.T) {
	setVMwareFeatureForTest(t, true)

	handler, persistence := newVMwareHandlersForTest(t)
	monitor, _, _ := newTestMonitor(t)
	handler.getMonitor = func(context.Context) *monitoring.Monitor { return monitor }
	if err := persistence.SaveVMwareConfig([]config.VMwareVCenterInstance{
		{
			ID:       "conn-1",
			Name:     "lab-vcenter",
			Host:     "vcsa.lab.local",
			Username: "administrator@vsphere.local",
			Password: "super-secret",
			Enabled:  true,
		},
	}); err != nil {
		t.Fatalf("seed vmware config: %v", err)
	}

	var previewed config.VMwareVCenterInstance
	handler.previewRecords = func(_ context.Context, instance config.VMwareVCenterInstance) ([]unifiedresources.IngestRecord, error) {
		previewed = instance
		return []unifiedresources.IngestRecord{
			{
				SourceID: "vc-1:host:host-101",
				Resource: unifiedresources.Resource{
					Type:   unifiedresources.ResourceTypeAgent,
					Name:   "esxi-01.lab.local",
					Status: unifiedresources.StatusOnline,
					VMware: &unifiedresources.VMwareData{
						ConnectionID:    "conn-1",
						ConnectionName:  "lab-vcenter",
						ManagedObjectID: "host-101",
						EntityType:      "host",
						HostUUID:        "uuid-host-1",
					},
				},
				Identity: unifiedresources.ResourceIdentity{
					DMIUUID:   "uuid-host-1",
					Hostnames: []string{"esxi-01.lab.local"},
				},
			},
		}, nil
	}

	body := marshalVMwareRequest(t, map[string]any{
		"host":               "edited.lab.local",
		"username":           "operator@vsphere.local",
		"password":           "********",
		"insecureSkipVerify": true,
		"enabled":            true,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/vmware/connections/conn-1/preview", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandlePreviewSavedConnection(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if previewed.Password != "super-secret" {
		t.Fatalf("expected stored password to be reused, got %+v", previewed)
	}
	if previewed.Host != "edited.lab.local" {
		t.Fatalf("expected edited host to be previewed, got %+v", previewed)
	}
}

func TestVMwareHandlers_HandleTestSavedConnection_UsesStoredSecretsAndUpdatesRuntimeSummary(t *testing.T) {
	setVMwareFeatureForTest(t, true)

	connection := config.VMwareVCenterInstance{
		ID:                 "conn-1",
		Name:               "lab-vcenter",
		Host:               "vcsa.lab.local",
		Port:               443,
		Username:           "administrator@vsphere.local",
		Password:           "super-secret",
		InsecureSkipVerify: false,
		Enabled:            true,
	}
	handler, persistence := newVMwareHandlersForTest(t)
	poller := monitoring.NewVMwarePoller(nil, time.Minute)
	handler.getPoller = func(context.Context) *monitoring.VMwarePoller { return poller }
	if err := persistence.SaveVMwareConfig([]config.VMwareVCenterInstance{connection}); err != nil {
		t.Fatalf("seed vmware config: %v", err)
	}

	var gotConfig vmware.ClientConfig
	handler.newClient = func(cfg vmware.ClientConfig) (vmwareClient, error) {
		gotConfig = cfg
		return &fakeVMwareClient{
			testConnection: func(context.Context) (*vmware.InventorySummary, error) {
				return &vmware.InventorySummary{Hosts: 4, VMs: 25, Datastores: 5, VIRelease: "8.0.3"}, nil
			},
		}, nil
	}

	req := httptest.NewRequest(http.MethodPost, "/api/vmware/connections/conn-1/test", nil)
	rec := httptest.NewRecorder()
	handler.HandleTestSavedConnection(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if gotConfig.Host != "vcsa.lab.local" || gotConfig.Password != "super-secret" {
		t.Fatalf("unexpected saved client config: %+v", gotConfig)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/vmware/connections", nil)
	listRec := httptest.NewRecorder()
	handler.HandleList(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list expected 200, got %d: %s", listRec.Code, listRec.Body.String())
	}

	var listed []vmwareConnectionResponse
	if err := json.NewDecoder(listRec.Body).Decode(&listed); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(listed) != 1 || listed[0].Poll == nil || listed[0].Poll.LastSuccessAt == nil {
		t.Fatalf("expected saved retest to update runtime status, got %+v", listed)
	}
	if listed[0].Observed == nil || listed[0].Observed.VMs != 25 {
		t.Fatalf("expected saved retest to update observed summary, got %+v", listed[0].Observed)
	}
}

func TestVMwareHandlers_HandleTestSavedConnection_UpdatesRuntimeSummaryFailure(t *testing.T) {
	setVMwareFeatureForTest(t, true)

	connection := config.VMwareVCenterInstance{
		ID:       "conn-1",
		Host:     "vcsa.lab.local",
		Username: "administrator@vsphere.local",
		Password: "super-secret",
		Enabled:  true,
	}
	handler, persistence := newVMwareHandlersForTest(t)
	poller := monitoring.NewVMwarePoller(nil, time.Minute)
	handler.getPoller = func(context.Context) *monitoring.VMwarePoller { return poller }
	if err := persistence.SaveVMwareConfig([]config.VMwareVCenterInstance{connection}); err != nil {
		t.Fatalf("seed vmware config: %v", err)
	}
	handler.newClient = func(cfg vmware.ClientConfig) (vmwareClient, error) {
		return &fakeVMwareClient{
			testConnection: func(context.Context) (*vmware.InventorySummary, error) {
				return nil, &vmware.ConnectionError{Category: "auth", Message: "authentication failed"}
			},
		}, nil
	}

	req := httptest.NewRequest(http.MethodPost, "/api/vmware/connections/conn-1/test", nil)
	rec := httptest.NewRecorder()
	handler.HandleTestSavedConnection(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}

	summary := poller.ConnectionSummaries("default", []config.VMwareVCenterInstance{connection})[connection.ID]
	if summary.Poll == nil || summary.Poll.LastError == nil {
		t.Fatalf("expected saved retest failure to update runtime summary, got %+v", summary.Poll)
	}
	if summary.Poll.LastError.Message != "authentication failed" || summary.Poll.LastError.Category != "auth" {
		t.Fatalf("unexpected failure summary: %+v", summary.Poll.LastError)
	}
}

func TestVMwareHandlers_HandleTestSavedConnection_MergesEditedPayloadWithoutOverwritingRuntimeSummary(t *testing.T) {
	setVMwareFeatureForTest(t, true)

	handler, persistence := newVMwareHandlersForTest(t)
	poller := monitoring.NewVMwarePoller(nil, time.Minute)
	handler.getPoller = func(context.Context) *monitoring.VMwarePoller { return poller }
	if err := persistence.SaveVMwareConfig([]config.VMwareVCenterInstance{
		{
			ID:                 "conn-1",
			Name:               "lab-vcenter",
			Host:               "vcsa.lab.local",
			Port:               443,
			Username:           "administrator@vsphere.local",
			Password:           "super-secret",
			InsecureSkipVerify: false,
			Enabled:            true,
		},
	}); err != nil {
		t.Fatalf("seed vmware config: %v", err)
	}

	previousAt := time.Date(2026, 3, 30, 9, 0, 0, 0, time.UTC)
	poller.RecordConnectionTestSuccess("default", "conn-1", &vmware.InventorySummary{Hosts: 1, VMs: 2, Datastores: 3, VIRelease: "8.0.2.0"}, previousAt)

	var gotConfig vmware.ClientConfig
	handler.newClient = func(cfg vmware.ClientConfig) (vmwareClient, error) {
		gotConfig = cfg
		return &fakeVMwareClient{
			testConnection: func(context.Context) (*vmware.InventorySummary, error) {
				return &vmware.InventorySummary{Hosts: 9, VMs: 99, Datastores: 12, VIRelease: "8.0.3"}, nil
			},
		}, nil
	}

	body := marshalVMwareRequest(t, map[string]any{
		"name":               "edited-vcenter",
		"host":               "edited.lab.local",
		"port":               8443,
		"username":           "operator@vsphere.local",
		"password":           "********",
		"insecureSkipVerify": true,
		"enabled":            true,
	})
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/vmware/connections/conn-1/test",
		bytes.NewReader(body),
	)
	rec := httptest.NewRecorder()
	handler.HandleTestSavedConnection(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if gotConfig.Host != "edited.lab.local" || gotConfig.Port != 8443 {
		t.Fatalf("expected edited endpoint, got %+v", gotConfig)
	}
	if gotConfig.Password != "super-secret" {
		t.Fatalf("expected stored password to be reused, got %+v", gotConfig)
	}

	summary := poller.ConnectionSummaries("default", []config.VMwareVCenterInstance{{
		ID:                 "conn-1",
		Name:               "lab-vcenter",
		Host:               "vcsa.lab.local",
		Port:               443,
		Username:           "administrator@vsphere.local",
		Password:           "super-secret",
		InsecureSkipVerify: false,
		Enabled:            true,
	}})["conn-1"]
	if summary.Observed == nil || summary.Observed.VMs != 2 || summary.Observed.VIRelease != "8.0.2.0" {
		t.Fatalf("expected existing runtime summary to remain unchanged, got %+v", summary.Observed)
	}
	if summary.Poll == nil || summary.Poll.LastSuccessAt == nil || !summary.Poll.LastSuccessAt.Equal(previousAt) {
		t.Fatalf("expected existing last success timestamp to remain unchanged, got %+v", summary.Poll)
	}
}

func newVMwareHandlersForTest(t *testing.T) (*VMwareHandlers, *config.ConfigPersistence) {
	t.Helper()

	persistence := config.NewConfigPersistence(t.TempDir())
	monitor, _, _ := newTestMonitor(t)
	handler := &VMwareHandlers{
		getPersistence: func(context.Context) *config.ConfigPersistence { return persistence },
		getMonitor:     func(context.Context) *monitoring.Monitor { return monitor },
		previewRecords: func(context.Context, config.VMwareVCenterInstance) ([]unifiedresources.IngestRecord, error) {
			return nil, nil
		},
	}

	return handler, persistence
}

func setVMwareFeatureForTest(t *testing.T, enabled bool) {
	t.Helper()
	previous := vmware.IsFeatureEnabled()
	vmware.SetFeatureEnabled(enabled)
	t.Cleanup(func() {
		vmware.SetFeatureEnabled(previous)
	})
}

func setMockModeForVMwareTest(t *testing.T, enabled bool) {
	t.Helper()
	previous := mock.IsMockEnabled()
	mock.SetEnabled(enabled)
	t.Cleanup(func() {
		mock.SetEnabled(previous)
	})
}

func marshalVMwareRequest(t *testing.T, payload map[string]any) []byte {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal request body: %v", err)
	}
	return body
}
