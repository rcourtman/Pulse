package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/storagehealth"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestResourceListPreservesCanonicalPoolHealthEvidence(t *testing.T) {
	observedAt := time.Date(2026, 7, 24, 10, 0, 0, 0, time.UTC)
	cfg := &config.Config{DataPath: t.TempDir()}
	h := NewResourceHandlers(cfg)
	h.SetStateProvider(resourceUnifiedSeedProvider{
		snapshot: models.StateSnapshot{LastUpdate: observedAt},
		resources: []unified.Resource{{
			ID:        "truenas:nas-a:pool:pool-guid",
			Type:      unified.ResourceTypeStorage,
			Name:      "tank",
			Status:    unified.StatusWarning,
			LastSeen:  observedAt,
			UpdatedAt: observedAt,
			Sources:   []unified.DataSource{unified.SourceTrueNAS},
			Storage: &unified.StorageMeta{
				Platform:     "truenas",
				Topology:     "mirror",
				ZFSPoolState: "DEGRADED",
				PoolHealth: &unified.PoolHealth{
					Scope:          "pool",
					Provider:       "truenas",
					NativeID:       "pool-guid",
					CanonicalState: "DEGRADED",
					NativeState:    "DEGRADED",
					Severity:       storagehealth.RiskCritical,
					Summary:        "Pool tank is degraded with a missing mirror member",
					Recommendation: "Confirm the missing mirror member in TrueNAS before replacement.",
					Source:         "pool.query",
					EvidenceCodes:  []string{"zfs_pool_state", "zfs_device_missing"},
					ObservedAt:     observedAt,
				},
				ZFSPool: &models.ZFSPool{
					Name:  "tank",
					State: "DEGRADED",
					ScanDetails: &models.ZFSScan{
						Function:   "RESILVER",
						State:      "SCANNING",
						Percentage: 33.3,
					},
					ReadErrors: 1,
					Devices: []models.ZFSDevice{{
						Name:       "missing",
						Type:       "UNAVAIL_DISK",
						Role:       "data",
						Parent:     "mirror-0",
						Path:       "/dev/disk/by-partuuid/missing",
						State:      "UNAVAIL",
						ReadErrors: 1,
						Missing:    true,
					}},
				},
			},
			Incidents: []unified.ResourceIncident{{
				Provider:                      "truenas",
				NativeID:                      "pool:pool-guid:vdev:missing",
				Code:                          "zfs_device_missing",
				Severity:                      storagehealth.RiskCritical,
				Source:                        "pool.query",
				Summary:                       "Pool tank has a missing data member",
				ConfirmationsRequired:         2,
				RecoveryConfirmationsRequired: 2,
			}},
		}},
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/resources?type=storage&page=1&limit=100", nil)
	h.HandleListResources(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var response ResourcesResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Data) != 1 {
		t.Fatalf("resources = %+v", response.Data)
	}
	resource := response.Data[0]
	if resource.Storage == nil || resource.Storage.PoolHealth == nil || resource.Storage.ZFSPool == nil {
		t.Fatalf("pool-health envelope missing from API: %+v", resource.Storage)
	}
	if resource.Storage.PoolHealth.CanonicalState != "DEGRADED" ||
		resource.Storage.PoolHealth.Source != "pool.query" ||
		len(resource.Storage.PoolHealth.EvidenceCodes) != 2 {
		t.Fatalf("pool-health evidence = %+v", resource.Storage.PoolHealth)
	}
	if resource.Storage.ZFSPool.ScanDetails == nil ||
		resource.Storage.ZFSPool.ScanDetails.Function != "RESILVER" ||
		len(resource.Storage.ZFSPool.Devices) != 1 ||
		!resource.Storage.ZFSPool.Devices[0].Missing {
		t.Fatalf("native ZFS report = %+v", resource.Storage.ZFSPool)
	}
	if len(resource.Incidents) != 1 ||
		resource.Incidents[0].ConfirmationsRequired != 2 ||
		resource.Incidents[0].RecoveryConfirmationsRequired != 2 {
		t.Fatalf("incident lifecycle contract = %+v", resource.Incidents)
	}
}
