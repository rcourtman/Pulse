package unifiedresources

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/storagehealth"
)

// --- summarizePBSAffectedDatastores ---

func TestSummarizePBSAffectedDatastores_Empty(t *testing.T) {
	got := summarizePBSAffectedDatastores(nil)
	if got != "" {
		t.Errorf("expected empty for nil, got %q", got)
	}
}

func TestSummarizePBSAffectedDatastores_Single(t *testing.T) {
	got := summarizePBSAffectedDatastores([]string{"ds1"})
	expected := "Affects 1 backup datastore: ds1"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestSummarizePBSAffectedDatastores_Multiple(t *testing.T) {
	got := summarizePBSAffectedDatastores([]string{"ds1", "ds2", "ds3"})
	expected := "Affects 3 backup datastores: ds1, ds2, ds3"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

// --- summarizePBSProtectedWorkloads ---

func TestSummarizePBSProtectedWorkloads_ZeroCount(t *testing.T) {
	got := summarizePBSProtectedWorkloads(0, nil)
	if got != "" {
		t.Errorf("expected empty for 0 count, got %q", got)
	}
}

func TestSummarizePBSProtectedWorkloads_NoNames(t *testing.T) {
	got := summarizePBSProtectedWorkloads(3, nil)
	expected := "Puts backups for 3 protected workloads at risk"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestSummarizePBSProtectedWorkloads_SingleWorkload(t *testing.T) {
	got := summarizePBSProtectedWorkloads(1, []string{"web-server"})
	expected := "Puts backups for 1 protected workload at risk: web-server"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestSummarizePBSProtectedWorkloads_WithRemaining(t *testing.T) {
	got := summarizePBSProtectedWorkloads(10, []string{"vm-1", "vm-2", "vm-3"})
	expected := "Puts backups for 10 protected workloads at risk: vm-1, vm-2, vm-3, and 7 more"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestSummarizePBSProtectedWorkloads_ExactMatch(t *testing.T) {
	got := summarizePBSProtectedWorkloads(2, []string{"vm-1", "vm-2"})
	expected := "Puts backups for 2 protected workloads at risk: vm-1, vm-2"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

// --- summarizePBSPosture ---

func TestSummarizePBSPosture_NilPBS(t *testing.T) {
	got := summarizePBSPosture(nil)
	if got != "" {
		t.Errorf("expected empty for nil PBS, got %q", got)
	}
}

func TestSummarizePBSPosture_NeitherAffected(t *testing.T) {
	pbs := &PBSData{}
	got := summarizePBSPosture(pbs)
	if got != "" {
		t.Errorf("expected empty when no affected datastores or workloads, got %q", got)
	}
}

func TestSummarizePBSPosture_OnlyDatastores(t *testing.T) {
	pbs := &PBSData{
		AffectedDatastoreCount: 1,
		AffectedDatastores:     []string{"ds1"},
	}
	got := summarizePBSPosture(pbs)
	expected := "Affects 1 backup datastore: ds1"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestSummarizePBSPosture_OnlyWorkloads(t *testing.T) {
	pbs := &PBSData{
		ProtectedWorkloadCount: 5,
		ProtectedWorkloadNames: []string{"vm-1", "vm-2"},
	}
	got := summarizePBSPosture(pbs)
	expected := "Puts backups for 5 protected workloads at risk: vm-1, vm-2, and 3 more"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestSummarizePBSPosture_CombinedDatastoresAndWorkloads(t *testing.T) {
	pbs := &PBSData{
		AffectedDatastoreCount: 2,
		AffectedDatastores:     []string{"ds1", "ds2"},
		ProtectedWorkloadCount: 3,
		ProtectedWorkloadNames: []string{"web", "db", "api"},
	}
	got := summarizePBSPosture(pbs)
	expected := "Affects 2 backup datastores: ds1, ds2. Puts backups for 3 protected workloads at risk: web, db, api"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

// --- pbsDatastoreAffectsPosture ---

func TestPbsDatastoreAffectsPosture_Nil(t *testing.T) {
	if pbsDatastoreAffectsPosture(nil) {
		t.Error("nil resource should not affect posture")
	}
}

func TestPbsDatastoreAffectsPosture_NoRiskNoIncidents(t *testing.T) {
	resource := &Resource{ID: "r-1", Storage: &StorageMeta{}}
	if pbsDatastoreAffectsPosture(resource) {
		t.Error("resource with no risk and no incidents should not affect posture")
	}
}

func TestPbsDatastoreAffectsPosture_WithRisk(t *testing.T) {
	resource := &Resource{
		ID: "r-1",
		Storage: &StorageMeta{
			Risk: &StorageRisk{
				Level:   storagehealth.RiskWarning,
				Reasons: []StorageRiskReason{{Code: "test", Summary: "test"}},
			},
		},
	}
	if !pbsDatastoreAffectsPosture(resource) {
		t.Error("resource with risk reasons should affect posture")
	}
}

func TestPbsDatastoreAffectsPosture_WithIncidents(t *testing.T) {
	resource := &Resource{
		ID: "r-1",
		Incidents: []ResourceIncident{
			{Code: "test", Summary: "test"},
		},
	}
	if !pbsDatastoreAffectsPosture(resource) {
		t.Error("resource with incidents should affect posture")
	}
}

func TestPbsDatastoreAffectsPosture_EmptyRiskReasons(t *testing.T) {
	resource := &Resource{
		ID: "r-1",
		Storage: &StorageMeta{
			Risk: &StorageRisk{
				Level:   storagehealth.RiskHealthy,
				Reasons: nil,
			},
		},
	}
	if pbsDatastoreAffectsPosture(resource) {
		t.Error("resource with risk but empty reasons should not affect posture")
	}
}

// --- isPBSDatastoreResource ---

func TestIsPBSDatastoreResource_Valid(t *testing.T) {
	resource := &Resource{
		Type: ResourceTypeStorage,
		Storage: &StorageMeta{
			Platform: "pbs",
			Topology: "datastore",
		},
	}
	if !isPBSDatastoreResource(resource) {
		t.Error("should identify as PBS datastore resource")
	}
}

func TestIsPBSDatastoreResource_CaseInsensitive(t *testing.T) {
	resource := &Resource{
		Type: ResourceTypeStorage,
		Storage: &StorageMeta{
			Platform: " PBS ",
			Topology: " Datastore ",
		},
	}
	if !isPBSDatastoreResource(resource) {
		t.Error("should be case-insensitive and trim spaces")
	}
}

func TestIsPBSDatastoreResource_WrongType(t *testing.T) {
	resource := &Resource{
		Type: ResourceTypeVM,
		Storage: &StorageMeta{
			Platform: "pbs",
			Topology: "datastore",
		},
	}
	if isPBSDatastoreResource(resource) {
		t.Error("non-storage type should not identify as PBS datastore")
	}
}

func TestIsPBSDatastoreResource_Nil(t *testing.T) {
	if isPBSDatastoreResource(nil) {
		t.Error("nil should not be PBS datastore")
	}
}

func TestIsPBSDatastoreResource_NilStorage(t *testing.T) {
	resource := &Resource{Type: ResourceTypeStorage}
	if isPBSDatastoreResource(resource) {
		t.Error("nil storage should not be PBS datastore")
	}
}

// --- normalizePBSLookupName ---

func TestNormalizePBSLookupName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"PBS-Server", "pbs-server"},
		{"  MyHost  ", "myhost"},
		{"", ""},
		{"already-lower", "already-lower"},
	}
	for _, tt := range tests {
		got := normalizePBSLookupName(tt.input)
		if got != tt.expected {
			t.Errorf("normalizePBSLookupName(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

// --- selectPBSInstanceResource ---

func TestSelectPBSInstanceResource_EmptyKey(t *testing.T) {
	index := map[string][]*Resource{
		"test": {{ID: "r-1"}},
	}
	if selectPBSInstanceResource(index, "") != nil {
		t.Error("empty instance should return nil")
	}
}

func TestSelectPBSInstanceResource_SingleMatch(t *testing.T) {
	r := &Resource{ID: "pbs-1"}
	index := map[string][]*Resource{
		"pbs-server": {r},
	}
	got := selectPBSInstanceResource(index, "PBS-Server")
	if got != r {
		t.Error("should return the single matching resource")
	}
}

func TestSelectPBSInstanceResource_AmbiguousMultipleMatches(t *testing.T) {
	index := map[string][]*Resource{
		"pbs-server": {{ID: "r-1"}, {ID: "r-2"}},
	}
	if selectPBSInstanceResource(index, "pbs-server") != nil {
		t.Error("multiple candidates should return nil (ambiguous)")
	}
}

func TestSelectPBSInstanceResource_NoMatch(t *testing.T) {
	index := map[string][]*Resource{
		"pbs-server": {{ID: "r-1"}},
	}
	if selectPBSInstanceResource(index, "other-server") != nil {
		t.Error("no match should return nil")
	}
}
