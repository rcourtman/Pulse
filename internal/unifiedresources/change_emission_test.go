package unifiedresources

import (
	"testing"
	"time"
)

func TestBuildResourceChange_ReturnsNilWhenUnchanged(t *testing.T) {
	before := Resource{
		ID:     "vm:1",
		Type:   ResourceTypeVM,
		Name:   "vm-1",
		Status: StatusOnline,
	}

	if change := buildResourceChange(before, true, before, true, time.Now().UTC(), nil, SourcePulseDiff, ""); change != nil {
		t.Fatalf("expected nil change for identical resources, got %+v", change)
	}
}

func TestBuildResourceChange_ClassifiesStateTransition(t *testing.T) {
	before := Resource{
		ID:     "vm:1",
		Type:   ResourceTypeVM,
		Name:   "vm-1",
		Status: StatusOnline,
	}
	after := before
	after.Status = StatusWarning

	change := buildResourceChange(before, true, after, true, time.Now().UTC(), nil, SourcePlatformEvent, "")
	if change == nil {
		t.Fatal("expected state transition change, got nil")
	}
	if change.Kind != ChangeStateTransition {
		t.Fatalf("Kind = %q, want %q", change.Kind, ChangeStateTransition)
	}
	if change.From != "online" || change.To != "warning" {
		t.Fatalf("From/To = %q/%q, want online/warning", change.From, change.To)
	}
	if !sameStringSet(mustChangedFields(t, change), []string{"status"}) {
		t.Fatalf("changedFields = %+v, want status", mustChangedFields(t, change))
	}
}

func TestBuildResourceChange_ClassifiesRelationshipChangeAndIncludesEndpoints(t *testing.T) {
	before := Resource{
		ID:     "vm:1",
		Type:   ResourceTypeVM,
		Name:   "vm-1",
		Status: StatusOnline,
		Relationships: []ResourceRelationship{
			{SourceID: "vm:1", TargetID: "db:1", Type: RelDependsOn, Confidence: 1},
		},
	}
	after := before
	after.Relationships = []ResourceRelationship{
		{SourceID: "vm:1", TargetID: "db:2", Type: RelDependsOn, Confidence: 1},
	}

	change := buildResourceChange(before, true, after, true, time.Now().UTC(), nil, SourcePulseDiff, "")
	if change == nil {
		t.Fatal("expected relationship change, got nil")
	}
	if change.Kind != ChangeRelationship {
		t.Fatalf("Kind = %q, want %q", change.Kind, ChangeRelationship)
	}
	if change.From != "vm:1->db:1[depends_on]" || change.To != "vm:1->db:2[depends_on]" {
		t.Fatalf("From/To = %q/%q, want vm:1->db:1[depends_on]/vm:1->db:2[depends_on]", change.From, change.To)
	}
	if !sameStringSet(change.RelatedResources, []string{"db:1", "db:2"}) {
		t.Fatalf("RelatedResources = %+v, want db:1 and db:2", change.RelatedResources)
	}
	for _, id := range change.RelatedResources {
		if id == change.ResourceID {
			t.Fatalf("RelatedResources unexpectedly included primary resource ID %q", id)
		}
	}
	if !sameStringSet(mustChangedFields(t, change), []string{"relationships"}) {
		t.Fatalf("changedFields = %+v, want relationships", mustChangedFields(t, change))
	}
}

func TestBuildResourceChange_ClassifiesCapabilityChange(t *testing.T) {
	before := Resource{
		ID:     "vm:1",
		Type:   ResourceTypeVM,
		Name:   "vm-1",
		Status: StatusOnline,
	}
	after := before
	after.Capabilities = []ResourceCapability{{Name: "restart", Type: CapabilityTypeCommon}}

	change := buildResourceChange(before, true, after, true, time.Now().UTC(), nil, SourcePulseDiff, "")
	if change == nil {
		t.Fatal("expected capability change, got nil")
	}
	if change.Kind != ChangeCapability {
		t.Fatalf("Kind = %q, want %q", change.Kind, ChangeCapability)
	}
	if change.From != "0" || change.To != "1" {
		t.Fatalf("From/To = %q/%q, want 0/1", change.From, change.To)
	}
	if !sameStringSet(mustChangedFields(t, change), []string{"capabilities"}) {
		t.Fatalf("changedFields = %+v, want capabilities", mustChangedFields(t, change))
	}
}

func TestBuildResourceChange_ClassifiesDockerRestartChange(t *testing.T) {
	before := Resource{
		ID:     "container:1",
		Type:   ResourceTypeAppContainer,
		Name:   "container-1",
		Status: StatusOnline,
		Docker: &DockerData{
			RestartCount:  1,
			UptimeSeconds: 3600,
			Image:         "example/app:1",
			UpdateStatus:  &DockerUpdateStatusMeta{UpdateAvailable: false},
		},
	}
	after := before
	after.Docker = &DockerData{
		RestartCount:  2,
		UptimeSeconds: 120,
		Image:         "example/app:1",
		UpdateStatus:  &DockerUpdateStatusMeta{UpdateAvailable: false},
	}

	change := buildResourceChange(before, true, after, true, time.Now().UTC(), nil, SourcePlatformEvent, "")
	if change == nil {
		t.Fatal("expected docker restart change, got nil")
	}
	if change.Kind != ChangeRestart {
		t.Fatalf("Kind = %q, want %q", change.Kind, ChangeRestart)
	}
	if change.From != "online|docker.restartCount=1|docker.uptimeSeconds=3600" || change.To != "online|docker.restartCount=2|docker.uptimeSeconds=120" {
		t.Fatalf("From/To = %q/%q, want docker restart summaries", change.From, change.To)
	}
	if !sameStringSet(mustChangedFields(t, change), []string{"docker.restartCount", "docker.uptimeSeconds"}) {
		t.Fatalf("changedFields = %+v, want docker restart counters", mustChangedFields(t, change))
	}
}

func TestBuildResourceChange_ClassifiesKubernetesRestartChange(t *testing.T) {
	before := Resource{
		ID:     "pod:1",
		Type:   ResourceTypePod,
		Name:   "pod-1",
		Status: StatusOnline,
		Kubernetes: &K8sData{
			PodPhase:                "Running",
			Restarts:                1,
			UptimeSeconds:           1800,
			Namespace:               "default",
			PodUID:                  "pod-uid-1",
			PodReason:               "Running",
			PodMessage:              "",
			OwnerKind:               "Deployment",
			OwnerName:               "app",
			ContainerRuntimeVersion: "containerd://1.7.0",
		},
	}
	after := before
	after.Kubernetes = &K8sData{
		PodPhase:                "Running",
		Restarts:                2,
		UptimeSeconds:           75,
		Namespace:               "default",
		PodUID:                  "pod-uid-1",
		PodReason:               "Running",
		PodMessage:              "",
		OwnerKind:               "Deployment",
		OwnerName:               "app",
		ContainerRuntimeVersion: "containerd://1.7.0",
	}

	change := buildResourceChange(before, true, after, true, time.Now().UTC(), nil, SourcePlatformEvent, "")
	if change == nil {
		t.Fatal("expected kubernetes restart change, got nil")
	}
	if change.Kind != ChangeRestart {
		t.Fatalf("Kind = %q, want %q", change.Kind, ChangeRestart)
	}
	if change.From != "online|kubernetes.restarts=1|kubernetes.uptimeSeconds=1800" || change.To != "online|kubernetes.restarts=2|kubernetes.uptimeSeconds=75" {
		t.Fatalf("From/To = %q/%q, want kubernetes restart summaries", change.From, change.To)
	}
	if !sameStringSet(mustChangedFields(t, change), []string{"kubernetes.restarts", "kubernetes.uptimeSeconds"}) {
		t.Fatalf("changedFields = %+v, want kubernetes restart counters", mustChangedFields(t, change))
	}
}

func TestBuildResourceChange_ClassifiesIncidentAnomalyChange(t *testing.T) {
	before := Resource{
		ID:     "storage:1",
		Type:   ResourceTypeStorage,
		Name:   "storage-1",
		Status: StatusOnline,
	}
	after := before
	after.Incidents = []ResourceIncident{{
		Provider: "pbs",
		NativeID: "datastore:capacity_runway_low",
		Code:     "capacity_runway_low",
		Severity: "warning",
		Source:   "pbs",
		Summary:  "PBS datastore archive is READ_ONLY",
	}}
	refreshResourceIncidentRollup(&after)

	change := buildResourceChange(before, true, after, true, time.Now().UTC(), nil, SourcePulseDiff, "")
	if change == nil {
		t.Fatal("expected incident anomaly change, got nil")
	}
	if change.Kind != ChangeAnomaly {
		t.Fatalf("Kind = %q, want %q", change.Kind, ChangeAnomaly)
	}
	if change.From != "none" || change.To != "capacity_runway_low[warning]:PBS datastore archive is READ_ONLY" {
		t.Fatalf("From/To = %q/%q, want incident summaries", change.From, change.To)
	}
	if !sameStringSet(mustChangedFields(t, change), []string{"incidents"}) {
		t.Fatalf("changedFields = %+v, want incidents", mustChangedFields(t, change))
	}
}

func TestBuildResourceChange_ClassifiesConfigUpdate(t *testing.T) {
	before := Resource{
		ID:     "vm:1",
		Type:   ResourceTypeVM,
		Name:   "vm-1",
		Status: StatusOnline,
	}
	after := before
	after.Name = "vm-1-renamed"
	after.CustomURL = "https://example.invalid/vm-1"

	change := buildResourceChange(before, true, after, true, time.Now().UTC(), nil, SourcePulseDiff, "")
	if change == nil {
		t.Fatal("expected config update, got nil")
	}
	if change.Kind != ChangeConfigUpdate {
		t.Fatalf("Kind = %q, want %q", change.Kind, ChangeConfigUpdate)
	}
	if change.From == change.To {
		t.Fatalf("expected distinct config summaries, got %q", change.From)
	}
	if !sameStringSet(mustChangedFields(t, change), []string{"name", "customUrl"}) {
		t.Fatalf("changedFields = %+v, want name and customUrl", mustChangedFields(t, change))
	}
}

func mustChangedFields(t *testing.T, change *ResourceChange) []string {
	t.Helper()
	raw, ok := change.Metadata["changedFields"]
	if !ok {
		t.Fatal("expected changedFields metadata")
	}
	fields, ok := raw.([]string)
	if !ok {
		t.Fatalf("changedFields type = %T, want []string", raw)
	}
	return fields
}
