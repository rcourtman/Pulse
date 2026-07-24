package unifiedresources

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/storagehealth"
)

func TestCephUsesNativeClusterEvidenceThroughPoolHealthContract(t *testing.T) {
	observedAt := time.Date(2026, 7, 24, 10, 0, 0, 0, time.UTC)
	resource, identity := resourceFromCephCluster(models.CephCluster{
		ID:            "pve-a-fsid",
		Instance:      "pve-a",
		Source:        models.CephClusterSourceProxmoxAPI,
		Name:          "Ceph",
		FSID:          "fsid-1",
		Health:        "HEALTH_WARN",
		HealthMessage: "1 osd down; 42 pgs degraded",
		HealthChecks: []models.CephHealthCheck{
			{Code: "OSD_DOWN", Severity: "HEALTH_WARN", Summary: "1 osd down"},
			{Code: "PG_DEGRADED", Severity: "HEALTH_WARN", Summary: "42 pgs degraded"},
		},
		LastUpdated: observedAt,
	})

	if identity.MachineID != "fsid-1" {
		t.Fatalf("identity = %+v", identity)
	}
	if resource.Type != ResourceTypeCeph || resource.Status != StatusWarning || resource.Ceph == nil || resource.Ceph.PoolHealth == nil {
		t.Fatalf("ceph resource = %+v", resource)
	}
	health := resource.Ceph.PoolHealth
	if health.Scope != "cluster" || health.CanonicalState != "DEGRADED" || health.NativeState != "HEALTH_WARN" || health.Severity != storagehealth.RiskWarning {
		t.Fatalf("pool health = %+v", health)
	}
	if len(health.EvidenceCodes) != 2 || len(resource.Ceph.HealthChecks) != 2 {
		t.Fatalf("native evidence was not preserved: health=%+v ceph=%+v", health, resource.Ceph)
	}
	if len(resource.Incidents) != 1 || resource.Incidents[0].Code != "ceph_cluster_health" || resource.Incidents[0].NativeID != "fsid-1" {
		t.Fatalf("cluster incident = %+v", resource.Incidents)
	}
	if resource.Incidents[0].ConfirmationsRequired != 2 || resource.Incidents[0].RecoveryConfirmationsRequired != 2 {
		t.Fatalf("cluster lifecycle = %+v", resource.Incidents[0])
	}
	if resource.PhysicalDisk != nil || resource.Storage != nil {
		t.Fatal("cluster health must not invent a failed disk or ZFS storage resource")
	}
}

func TestCephUnknownHealthDoesNotInventIncident(t *testing.T) {
	resource, _ := resourceFromCephCluster(models.CephCluster{
		ID:          "cluster-without-health",
		Instance:    "pve-a",
		Health:      "NOT_REPORTED",
		LastUpdated: time.Date(2026, 7, 24, 10, 0, 0, 0, time.UTC),
	})
	if resource.Status != StatusUnknown || len(resource.Incidents) != 0 {
		t.Fatalf("unknown health must remain unknown: %+v", resource)
	}
	if resource.Ceph == nil || resource.Ceph.PoolHealth == nil || resource.Ceph.PoolHealth.CanonicalState != "UNKNOWN" {
		t.Fatalf("unknown pool-health envelope missing: %+v", resource.Ceph)
	}
}
