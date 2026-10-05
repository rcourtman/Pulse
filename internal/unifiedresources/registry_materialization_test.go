package unifiedresources

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"
)

// This is the pre-materialization List implementation, retained independently
// of the optimized copy. It always derives metadata from the live resource.
func freshRegistryListForMetadataTest(rr *ResourceRegistry) []Resource {
	rr.mu.RLock()
	defer rr.mu.RUnlock()
	out := make([]Resource, 0, len(rr.resources))
	for _, r := range rr.resources {
		out = append(out, cloneResource(r))
	}
	sortResourcesByName(out)
	return out
}

func assertRegistryMetadataOracle(t *testing.T, rr *ResourceRegistry) []Resource {
	t.Helper()
	actual := rr.List()
	expected := freshRegistryListForMetadataTest(rr)
	if !reflect.DeepEqual(actual, expected) {
		for i := range expected {
			if i >= len(actual) || !reflect.DeepEqual(actual[i], expected[i]) {
				t.Fatalf("List differs from fresh clone at resource %q", expected[i].ID)
			}
		}
		t.Fatal("List changed inventory")
	}
	actualJSON, err := json.Marshal(actual)
	if err != nil {
		t.Fatal(err)
	}
	expectedJSON, err := json.Marshal(expected)
	if err != nil {
		t.Fatal(err)
	}
	if string(actualJSON) != string(expectedJSON) {
		t.Fatal("List changed canonical wire content")
	}
	return actual
}

func metadataOracleFixture() []Resource {
	now := time.Now().UTC()
	return []Resource{
		{ID: "agent:lab", Type: ResourceTypeAgent, Name: "lab", LastSeen: now, Sources: []DataSource{SourceAgent, SourceProxmox}, Agent: &AgentData{AgentID: "lab", Hostname: "lab.example.test"}, Proxmox: &ProxmoxData{NodeName: "lab"}, Tags: []string{"PII"}, Identity: ResourceIdentity{IPAddresses: []string{"192.0.2.1"}}, SupersededCanonicalIDs: []string{"agent:old-lab"}},
		{ID: "vm:lab", Type: ResourceTypeVM, Name: "guest", LastSeen: now, Sources: []DataSource{SourceProxmox}, Proxmox: &ProxmoxData{VMID: 100, NodeName: "lab"}, Metrics: &ResourceMetrics{CPU: &MetricValue{Percent: 1.25, Source: SourceProxmox}}},
		{ID: "container:lab", Type: ResourceTypeSystemContainer, Name: "ct", LastSeen: now, Sources: []DataSource{SourceProxmox}, Proxmox: &ProxmoxData{VMID: 101, NodeName: "lab"}},
		{ID: "docker:lab", Type: ResourceTypeAgent, Name: "docker", LastSeen: now, Sources: []DataSource{SourceDocker}, Docker: &DockerData{Hostname: "docker.example.test", HostSourceID: "runtime"}},
		{ID: "app:lab", Type: ResourceTypeAppContainer, Name: "app", LastSeen: now, Sources: []DataSource{SourceDocker}, Docker: &DockerData{ContainerID: "container-id", HostSourceID: "runtime"}},
		{ID: "storage:lab", Type: ResourceTypeStorage, Name: "storage", LastSeen: now, Sources: []DataSource{SourceProxmox}, Storage: &StorageMeta{Path: "/srv/synthetic"}},
		{ID: "pbs:lab", Type: ResourceTypePBS, Name: "backup", LastSeen: now, Sources: []DataSource{SourcePBS}, PBS: &PBSData{InstanceID: "pbs-lab", Hostname: "backup.example.test"}},
		{ID: "pmg:lab", Type: ResourceTypePMG, Name: "mail", LastSeen: now, Sources: []DataSource{SourcePMG}, PMG: &PMGData{InstanceID: "pmg-lab", Hostname: "mail.example.test"}},
		{ID: "k8s:lab", Type: ResourceTypeK8sCluster, Name: "cluster", LastSeen: now, Sources: []DataSource{SourceK8s}, Kubernetes: &K8sData{ClusterID: "cluster-lab"}},
		{ID: "k8s:node", Type: ResourceTypeK8sNode, Name: "node", LastSeen: now, Sources: []DataSource{SourceK8s}, Kubernetes: &K8sData{ClusterID: "cluster-lab"}},
		{ID: "k8s:pod", Type: ResourceTypePod, Name: "pod", LastSeen: now, Sources: []DataSource{SourceK8s}, Kubernetes: &K8sData{ClusterID: "cluster-lab"}},
		{ID: "k8s:deploy", Type: ResourceTypeK8sDeployment, Name: "deploy", LastSeen: now, Sources: []DataSource{SourceK8s}, Kubernetes: &K8sData{ClusterID: "cluster-lab"}},
		{ID: "k8s:secret", Type: ResourceTypeK8sSecret, Name: "synthetic-secret-metadata", LastSeen: now, Sources: []DataSource{SourceK8s}, Kubernetes: &K8sData{ClusterID: "cluster-lab", SecretUID: "secret-lab"}},
		{ID: "k8s:role", Type: ResourceTypeK8sRole, Name: "role", LastSeen: now, Sources: []DataSource{SourceK8s}, Kubernetes: &K8sData{ClusterID: "cluster-lab", RoleUID: "role-lab"}},
		{ID: "tn:lab", Type: ResourceTypeAgent, Name: "nas", LastSeen: now, Sources: []DataSource{SourceTrueNAS}, TrueNAS: &TrueNASData{Hostname: "nas.example.test"}},
		{ID: "vmware:lab", Type: ResourceTypeAgent, Name: "esxi", LastSeen: now, Sources: []DataSource{SourceVMware}, VMware: &VMwareData{ConnectionID: "vsphere-lab", ManagedObjectID: "host-1"}},
		{ID: "disk:lab", Type: ResourceTypePhysicalDisk, Name: "disk", LastSeen: now, Sources: []DataSource{SourceAgent}, PhysicalDisk: &PhysicalDiskMeta{}},
		{ID: "ceph:lab", Type: ResourceTypeCeph, Name: "ceph", LastSeen: now, Sources: []DataSource{SourceProxmox}, Ceph: &CephMeta{}},
	}
}

func TestRegistryMaterializedMetadataMatchesFreshClones(t *testing.T) {
	rr := NewRegistry(nil)
	rr.IngestResources(metadataOracleFixture())
	original := assertRegistryMetadataOracle(t, rr)
	if len(original) != 18 {
		t.Fatalf("fixture lost resources: %d", len(original))
	}
	// Low-level record ingestion is observable before its batch epilogue. It
	// must invalidate metadata even when the typed cache was previously clean.
	rr.Workloads()
	id := rr.ingestRecord(SourceAgent, "lab", Resource{Type: ResourceTypeAgent, Name: "renamed", Status: StatusOnline, LastSeen: time.Now().UTC(), Agent: &AgentData{AgentID: "lab", Hostname: "renamed.example.test"}, Tags: []string{"public"}}, ResourceIdentity{}, false)
	assertRegistryMetadataOracle(t, rr)
	rr.retainSupersededCanonicalIDs(id, []string{"agent:new-retired-era"})
	assertRegistryMetadataOracle(t, rr)
	rr.IngestRecords(SourceAgent, []IngestRecord{{SourceID: "lab", Resource: Resource{Type: ResourceTypeAgent, Name: "restricted", Status: StatusOffline, LastSeen: time.Now().UTC(), Tags: []string{"customer-data"}, Agent: &AgentData{AgentID: "lab", Hostname: "renamed.example.test"}}}})
	assertRegistryMetadataOracle(t, rr)
	// Seeding normalized input must not trust caller-supplied policy or scopes.
	rr.IngestResources([]Resource{{ID: "storage:lab", Type: ResourceTypeStorage, Name: "moved", Status: StatusOnline, LastSeen: time.Now().UTC(), Sources: []DataSource{SourceTrueNAS}, TrueNAS: &TrueNASData{Hostname: "new-nas.example.test"}, Tags: []string{"public"}, PlatformScopes: []string{"invented"}, AISafeSummary: "stale"}})
	assertRegistryMetadataOracle(t, rr)
	rr.MarkStale(time.Now().UTC().Add(24*time.Hour), nil)
	stale := assertRegistryMetadataOracle(t, rr)
	if reflect.DeepEqual(stale, original) {
		t.Fatal("mutation fixture did not change any output")
	}
}

func TestRegistryMaterializedMetadataDetachesDerivedFields(t *testing.T) {
	rr := NewRegistry(nil)
	rr.IngestResources(metadataOracleFixture())
	before := assertRegistryMetadataOracle(t, rr)
	actual := rr.List()
	for i := range actual {
		r := &actual[i]
		if r.Canonical != nil {
			r.Canonical.DisplayName = "changed"
			if len(r.Canonical.Aliases) > 0 {
				r.Canonical.Aliases[0] = "changed"
			}
			if len(r.Canonical.SupersededIDs) > 0 {
				r.Canonical.SupersededIDs[0] = "changed"
			}
		}
		if r.Policy != nil {
			r.Policy.Sensitivity = ResourceSensitivityPublic
			if len(r.Policy.Routing.Redact) > 0 {
				r.Policy.Routing.Redact[0] = "changed"
			}
		}
		if len(r.PlatformScopes) > 0 {
			r.PlatformScopes[0] = "changed"
		}
	}
	if got := assertRegistryMetadataOracle(t, rr); !reflect.DeepEqual(got, before) {
		t.Fatal("List result mutated stored canonical metadata")
	}
}

func TestRegistryMaterializedMetadataConcurrentReads(t *testing.T) {
	rr := NewRegistry(nil)
	rr.IngestResources(metadataOracleFixture())
	var wg sync.WaitGroup
	for reader := 0; reader < 4; reader++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 40; i++ {
				for _, r := range rr.List() {
					fresh := cloneResource(&r)
					if !reflect.DeepEqual(fresh, r) {
						t.Error("concurrent List retained stale derived metadata")
						return
					}
				}
				rr.Workloads()
			}
		}()
	}
	for i := 0; i < 40; i++ {
		rr.IngestRecords(SourceAgent, []IngestRecord{{SourceID: "live", Resource: Resource{Type: ResourceTypeAgent, Name: fmt.Sprintf("host-%d", i), Status: StatusOnline, LastSeen: time.Now().UTC(), Tags: []string{"customer-data"}, Agent: &AgentData{AgentID: "live"}}}})
	}
	wg.Wait()
	assertRegistryMetadataOracle(t, rr)
}

var materializedMetadataAllocationSink []Resource

func TestRegistryMaterializedMetadataAllocationBudget(t *testing.T) {
	rr := NewRegistry(nil)
	rr.IngestSnapshot(benchmarkVMState(1000))
	assertRegistryMetadataOracle(t, rr)
	before := testing.AllocsPerRun(3, func() { materializedMetadataAllocationSink = freshRegistryListForMetadataTest(rr) })
	after := testing.AllocsPerRun(3, func() { materializedMetadataAllocationSink = rr.List() })
	t.Logf("1000-resource bulk read allocations: always-refresh=%.0f materialized=%.0f", before, after)
	if after >= before*0.7 {
		t.Fatalf("bulk reads still repeat canonical derivation: %.0f >= 70%% of %.0f", after, before)
	}
}

// Paired same-process measurements are source costs, not fleet CPU/RSS. The
// immutable fresh-clone oracle and current implementation use the same input.
func BenchmarkRegistryMetadataBulkReads(b *testing.B) {
	for _, count := range []int{1, 1000} {
		rr := NewRegistry(nil)
		rr.IngestSnapshot(benchmarkVMState(count))
		rr.List()
		for _, read := range []struct {
			name string
			fn   func() []Resource
		}{{"fresh-clone", func() []Resource { return freshRegistryListForMetadataTest(rr) }}, {"materialized", rr.List}} {
			b.Run(fmt.Sprintf("resources-%d/%s", count, read.name), func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					materializedMetadataAllocationSink = read.fn()
				}
			})
		}
	}
}

// FreshRegistryMetadataListForTest exposes the independent clone oracle only
// to external-package connected fixture tests, never production consumers.
func FreshRegistryMetadataListForTest(rr *ResourceRegistry) []Resource {
	return freshRegistryListForMetadataTest(rr)
}

func TestRegistryMaterializedMetadataViewsKeepFreshness(t *testing.T) {
	rr := NewRegistry(nil)
	rr.IngestSnapshot(benchmarkVMState(1))
	before := rr.VMs()[0]
	if before.Status() != StatusOnline {
		t.Fatalf("fresh VM status=%s", before.Status())
	}
	rr.MarkStale(time.Now().UTC().Add(24*time.Hour), nil)
	after := rr.VMs()[0]
	if after.r.SourceStatus[SourceProxmox].Status != "stale" || after.Status() == StatusOnline {
		t.Fatal("stale marking left the typed view online")
	}
	if before.Status() != StatusOnline {
		t.Fatal("stale marking mutated a retained detached view")
	}
	// A no-change stale pass must not throw away the already fresh view.
	rr.MarkStale(time.Now().UTC().Add(24*time.Hour), nil)
	if rr.VMs()[0] != after {
		t.Fatal("unchanged staleness rebuilt the typed view")
	}
	rr.ingestRecord(SourceProxmox, "lab:node-1:100", Resource{Type: ResourceTypeVM, Name: "renamed-vm", Status: StatusOnline, LastSeen: time.Now().UTC(), Proxmox: &ProxmoxData{VMID: 100, NodeName: "node-1"}}, ResourceIdentity{}, false)
	// No batch epilogue has run here; source mapping and typed view must already
	// reflect the resource visible to List and point reads.
	current := rr.VMs()[0]
	if current.Status() != StatusOnline || current.Name() != "renamed-vm" {
		t.Fatal("mid-batch record retained the previous typed view")
	}
	assertRegistryMetadataOracle(t, rr)
}

func TestRegistryMaterializedMetadataListKeepsViewsLazy(t *testing.T) {
	rr := NewRegistry(nil)
	rr.IngestSnapshot(benchmarkVMState(1000))
	rr.List()
	if rr.cachedVMs != nil || !rr.viewsDirty {
		t.Fatal("a List-only read built unnecessary typed copies")
	}
	rr.VMs()
	if rr.canonicalMetadataDirty || rr.viewsDirty {
		t.Fatal("views were not materialized from current canonical metadata")
	}
	before := rr.cachedVMs[0]
	rr.List()
	if rr.cachedVMs[0] != before {
		t.Fatal("clean bulk read discarded detached typed views")
	}
}
