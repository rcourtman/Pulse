package monitoring

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
)

// These collections cannot contribute a host facet. Workloads, Kubernetes
// nodes and all non-Kubernetes sources deliberately remain in the lookup.
var agentLookupMetadataCollections = []string{
	"Namespaces", "Services", "Ingresses", "EndpointSlices", "NetworkPolicies",
	"PersistentVolumes", "PersistentVolumeClaims", "StorageClasses", "ConfigMaps",
	"Secrets", "ServiceAccounts", "Roles", "ClusterRoles", "RoleBindings",
	"ClusterRoleBindings", "ResourceQuotas", "LimitRanges", "PodDisruptionBudgets",
	"HorizontalPodAutoscalers", "Events",
}

func agentLookupFixture(metadataCount int) *Monitor {
	now := time.Now().UTC()
	state := models.NewState()
	state.UpsertHost(models.Host{
		ID: "host", Hostname: "worker", MachineID: "machine", Status: "online", LastSeen: now,
		TokenID: "host-token", Tags: []string{"host-tag"},
		NetworkInterfaces: []models.HostNetworkInterface{{Name: "eth0", Addresses: []string{"192.0.2.1"}}},
	})
	state.UpsertDockerHost(models.DockerHost{
		ID: "docker-host", AgentID: "host", Hostname: "worker", MachineID: "machine",
		Status: "online", LastSeen: now, TokenID: "docker-token", CPUs: 4,
		Containers: []models.DockerContainer{{ID: "app", Name: "app", State: "running", Labels: map[string]string{"app": "web"}}},
	})
	state.UpdateNodes([]models.Node{{ID: "pve:worker", Name: "worker", Instance: "pve", LinkedAgentID: "host", Status: "online", LastSeen: now}})
	state.UpdateVMs([]models.VM{{ID: "pve:worker:100", Name: "vm", VMID: 100, Node: "worker", Instance: "pve", Status: "running", LastSeen: now}})
	state.UpdateContainers([]models.Container{{ID: "pve:worker:101", Name: "lxc", VMID: 101, Node: "worker", Instance: "pve", Status: "running", LastSeen: now}})
	cluster := models.KubernetesCluster{
		ID: "cluster", AgentID: "host", Name: "cluster", Status: "online", LastSeen: now,
		Nodes:       []models.KubernetesNode{{UID: "node", Name: "worker", MachineID: "machine", Ready: true}},
		Pods:        []models.KubernetesPod{{UID: "pod", Name: "pod", NodeName: "worker", Namespace: "ns", Phase: "Running"}},
		Deployments: []models.KubernetesDeployment{{UID: "deployment", Name: "deployment", Namespace: "ns"}},
	}
	value := reflect.ValueOf(&cluster).Elem()
	for i := 0; i < metadataCount; i++ {
		field := value.FieldByName(agentLookupMetadataCollections[i%len(agentLookupMetadataCollections)])
		entry := reflect.New(field.Type().Elem()).Elem()
		// Deliberately overlap metadata names/UIDs with host identities. Type,
		// not a friendly naming convention, must keep these out of admission.
		for key, text := range map[string]string{"UID": fmt.Sprintf("host-%d", i), "Name": "worker", "Namespace": "ns"} {
			if f := entry.FieldByName(key); f.IsValid() && f.Kind() == reflect.String {
				f.SetString(text)
			}
		}
		field.Set(reflect.Append(field, entry))
	}
	state.UpsertKubernetesCluster(cluster)
	return &Monitor{state: state, config: &config.Config{}}
}

func fullAgentLookupReference(m *Monitor) unifiedresources.ReadState {
	registry := unifiedresources.NewRegistry(nil)
	thresholds := m.resourceStaleThresholds()
	registry.IngestSnapshotWithStaleThresholds(m.state.GetSnapshot(), thresholds)
	return unifiedresources.NewMonitorAdapterWithStaleThresholds(registry, thresholds)
}

// Adapters timestamp a newly built registry with the wall clock. Compare
// every canonical host field except that construction timestamp, retaining
// source LastSeen, metric/source observation times and all identity payloads.
func agentLookupViewResources(t *testing.T, read unifiedresources.ReadState) [][]unifiedresources.Resource {
	t.Helper()
	adapter, ok := read.(*unifiedresources.MonitorAdapter)
	if !ok {
		t.Fatal("lookup must return the real monitor adapter")
	}
	byID := make(map[string]unifiedresources.Resource)
	for _, resource := range adapter.GetAll() {
		if resource.UpdatedAt.IsZero() {
			t.Fatal("registry construction timestamp is missing")
		}
		resource.UpdatedAt = time.Time{}
		byID[resource.ID] = resource
	}
	result := make([][]unifiedresources.Resource, 2)
	for _, host := range read.Hosts() {
		resource, found := byID[host.ID()]
		if !found {
			t.Fatalf("host view %q is absent from the registry", host.ID())
		}
		result[0] = append(result[0], resource)
	}
	for _, host := range read.DockerHosts() {
		resource, found := byID[host.ID()]
		if !found {
			t.Fatalf("Docker host view %q is absent from the registry", host.ID())
		}
		result[1] = append(result[1], resource)
	}
	return result
}

func TestAgentLookupMatchesWholeSnapshotHostViews(t *testing.T) {
	for _, count := range []int{0, 20, 1000} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			m := agentLookupFixture(count)
			before := m.state.GetSnapshot()
			want := fullAgentLookupReference(m)
			got := m.snapshotBackedAgentLookupReadState()
			if len(want.Hosts()) == 0 || len(want.DockerHosts()) == 0 {
				t.Fatal("fixture must exercise both host facets")
			}
			if !reflect.DeepEqual(agentLookupViewResources(t, got), agentLookupViewResources(t, want)) {
				t.Fatal("agent admission changed a canonical host or Docker host view")
			}
			if !reflect.DeepEqual(before, m.state.GetSnapshot()) {
				t.Fatal("agent lookup mutated the full inventory")
			}
			// A second report must see current source data without a cache or
			// freshness grace period, including explicit clears and removal.
			m.state.UpsertHost(models.Host{ID: "host", Hostname: "renamed", MachineID: "machine", Status: "online", LastSeen: time.Now().UTC()})
			m.state.RemoveDockerHost("docker-host")
			want = fullAgentLookupReference(m)
			got = m.snapshotBackedAgentLookupReadState()
			if !reflect.DeepEqual(agentLookupViewResources(t, got), agentLookupViewResources(t, want)) {
				t.Fatal("agent lookup retained a removed host or old metadata")
			}
		})
	}
}

func TestAgentLookupAllocationIgnoresUnrelatedKubernetesMetadata(t *testing.T) {
	measure := func(count int) float64 {
		m := agentLookupFixture(count)
		return testing.AllocsPerRun(3, func() {
			read := m.snapshotBackedAgentLookupReadState()
			if len(read.Hosts()) == 0 || len(read.DockerHosts()) == 0 {
				panic("missing fixture host")
			}
		})
	}
	small, large := measure(0), measure(1000)
	t.Logf("agent identity lookup allocations: no metadata=%.0f, 1000 metadata objects=%.0f", small, large)
	if large > small+10 {
		t.Fatalf("unrelated Kubernetes metadata still adds identity lookup allocations: %.0f -> %.0f", small, large)
	}
}

func TestAgentPointLookupAllocationIgnoresOtherHosts(t *testing.T) {
	measure := func(count int) float64 {
		m := &Monitor{state: models.NewState()}
		for i := 0; i < count; i++ {
			m.state.UpsertHost(models.Host{ID: fmt.Sprint(i), Hostname: fmt.Sprint(i), Tags: []string{"tag"}, Sensors: models.HostSensorSummary{TemperatureCelsius: map[string]float64{"cpu": 40}}})
		}
		return testing.AllocsPerRun(10, func() {
			if _, ok := m.hostByID("0"); !ok {
				panic("missing fixture host")
			}
		})
	}
	small, large := measure(1), measure(17)
	t.Logf("native host point lookup allocations: one host=%.0f, 17 hosts=%.0f", small, large)
	if large > small+1 {
		t.Fatalf("point lookup still clones unrelated hosts: %.0f -> %.0f", small, large)
	}
}

func TestAgentLookupReportsKeepCompleteKubernetesInventory(t *testing.T) {
	m := newTestMonitor(t)
	m.state = agentLookupFixture(1000).state
	before := m.state.GetSnapshot().KubernetesClusters
	registry := unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(nil))
	m.SetResourceStore(registry)

	// Both serialized report paths use the new admission lookup, but their
	// normal accepted-report publication must retain the complete inventory.
	hostReport := agentshost.Report{
		Agent:     agentshost.AgentInfo{ID: "host", Version: "test", IntervalSeconds: 30},
		Host:      agentshost.HostInfo{ID: "host", MachineID: "machine", Hostname: "worker", Platform: "linux"},
		Timestamp: time.Now().UTC(),
	}
	payload, err := json.Marshal(hostReport)
	if err != nil || json.Unmarshal(payload, &hostReport) != nil {
		t.Fatal("host report round trip failed")
	}
	if host, err := m.ApplyHostReport(hostReport, nil); err != nil || host.ID != "host" {
		t.Fatalf("host report identity changed: %q, %v", host.ID, err)
	}
	dockerReport := agentsdocker.Report{
		Agent:      agentsdocker.AgentInfo{ID: "host", Version: "test", IntervalSeconds: 30},
		Host:       agentsdocker.HostInfo{MachineID: "machine", Hostname: "worker", Runtime: "podman", TotalCPU: 4},
		Containers: []agentsdocker.Container{{ID: "app", Name: "app", State: "running", CPUPercent: 9.4}},
		Timestamp:  time.Now().UTC(),
	}
	payload, err = json.Marshal(dockerReport)
	if err != nil || json.Unmarshal(payload, &dockerReport) != nil {
		t.Fatal("Docker report round trip failed")
	}
	if host, err := m.ApplyDockerReport(dockerReport, nil); err != nil || host.ID != "docker-host" {
		t.Fatalf("Docker report identity changed: %q, %v", host.ID, err)
	}
	if !reflect.DeepEqual(before, m.GetState().KubernetesClusters) {
		t.Fatal("report lookup discarded or mutated published Kubernetes metadata")
	}
	// Compare all canonical resources as well as the native source snapshot.
	want := unifiedresources.NewRegistry(nil)
	want.IngestSnapshotWithStaleThresholds(m.GetState(), m.resourceStaleThresholds())
	got, expected := registry.GetAll(), want.List()
	for i := range got {
		got[i].UpdatedAt = time.Time{}
	}
	for i := range expected {
		expected[i].UpdatedAt = time.Time{}
	}
	if !reflect.DeepEqual(got, expected) {
		t.Fatal("report publication no longer contains the full canonical inventory")
	}
	for _, host := range m.GetUnifiedReadState().DockerHosts() {
		if host == nil {
			continue
		}
		if got, found := m.GetDockerHost(host.ID()); !found || got.ID != "docker-host" {
			t.Fatal("point lookup bypassed canonical Docker host identity resolution")
		}
	}
}

func BenchmarkAgentIdentityLookup(b *testing.B) {
	for _, count := range []int{0, 1000} {
		m := agentLookupFixture(count)
		for _, reference := range []bool{true, false} {
			name := "scoped"
			if reference {
				name = "whole-snapshot-reference"
			}
			b.Run(fmt.Sprintf("%s/metadata-%d", name, count), func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					var read unifiedresources.ReadState
					if reference {
						read = fullAgentLookupReference(m)
					} else {
						read = m.snapshotBackedAgentLookupReadState()
					}
					if len(read.Hosts()) == 0 || len(read.DockerHosts()) == 0 {
						b.Fatal("missing fixture host")
					}
				}
			})
		}
	}
}
