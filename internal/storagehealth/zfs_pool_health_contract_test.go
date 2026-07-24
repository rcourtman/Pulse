package storagehealth

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestAssessZFSPoolCanonicalStateAndOperationContract(t *testing.T) {
	tests := []struct {
		name      string
		pool      models.ZFSPool
		wantLevel RiskLevel
		wantCode  string
	}{
		{
			name:      "online",
			pool:      models.ZFSPool{Name: "tank", State: "ONLINE"},
			wantLevel: RiskHealthy,
		},
		{
			name:      "degraded",
			pool:      models.ZFSPool{Name: "tank", State: "DEGRADED"},
			wantLevel: RiskWarning,
			wantCode:  "zfs_pool_state",
		},
		{
			name:      "faulted",
			pool:      models.ZFSPool{Name: "tank", State: "FAULTED"},
			wantLevel: RiskCritical,
			wantCode:  "zfs_pool_state",
		},
		{
			name:      "offline",
			pool:      models.ZFSPool{Name: "tank", State: "OFFLINE"},
			wantLevel: RiskCritical,
			wantCode:  "zfs_pool_state",
		},
		{
			name:      "unavailable",
			pool:      models.ZFSPool{Name: "tank", State: "UNAVAIL"},
			wantLevel: RiskCritical,
			wantCode:  "zfs_pool_state",
		},
		{
			name: "resilver",
			pool: models.ZFSPool{
				Name:  "tank",
				State: "ONLINE",
				ScanDetails: &models.ZFSScan{
					Function:   "RESILVER",
					State:      "SCANNING",
					Percentage: 25,
				},
			},
			wantLevel: RiskWarning,
			wantCode:  "zfs_resilver_active",
		},
		{
			name: "scrub monitor only",
			pool: models.ZFSPool{
				Name:  "tank",
				State: "ONLINE",
				ScanDetails: &models.ZFSScan{
					Function: "SCRUB",
					State:    "SCANNING",
				},
			},
			wantLevel: RiskMonitor,
			wantCode:  "zfs_scrub_active",
		},
		{
			name: "scrub errors",
			pool: models.ZFSPool{
				Name:  "tank",
				State: "ONLINE",
				ScanDetails: &models.ZFSScan{
					Function: "SCRUB",
					State:    "FINISHED",
					Errors:   2,
				},
			},
			wantLevel: RiskCritical,
			wantCode:  "zfs_scan_errors",
		},
		{
			name: "spare available is healthy",
			pool: models.ZFSPool{
				Name:    "tank",
				State:   "ONLINE",
				Devices: []models.ZFSDevice{{Name: "sdc", Role: "spare", State: "AVAIL"}},
			},
			wantLevel: RiskHealthy,
		},
		{
			name: "native missing member",
			pool: models.ZFSPool{
				Name:    "tank",
				State:   "DEGRADED",
				Devices: []models.ZFSDevice{{Name: "sdb", State: "UNAVAIL", Missing: true}},
			},
			wantLevel: RiskCritical,
			wantCode:  "zfs_device_missing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assessment := AssessZFSPool(tt.pool)
			if assessment.Level != tt.wantLevel {
				t.Fatalf("level = %q, want %q (%+v)", assessment.Level, tt.wantLevel, assessment.Reasons)
			}
			if tt.wantCode == "" {
				if len(assessment.Reasons) != 0 {
					t.Fatalf("unexpected reasons: %+v", assessment.Reasons)
				}
				return
			}
			found := false
			for _, reason := range assessment.Reasons {
				if reason.Code == tt.wantCode {
					found = true
				}
			}
			if !found {
				t.Fatalf("missing %q in %+v", tt.wantCode, assessment.Reasons)
			}
		})
	}
}

func TestAssessZFSPoolUnknownDeviceStateDoesNotInventFailure(t *testing.T) {
	assessment := AssessZFSPool(models.ZFSPool{
		Name:    "tank",
		State:   "ONLINE",
		Devices: []models.ZFSDevice{{Name: "sda", State: "NOT_REPORTED"}},
	})
	if assessment.Level != RiskHealthy || len(assessment.Reasons) != 0 {
		t.Fatalf("unknown native state must remain evidence-only, got %+v", assessment)
	}
}
