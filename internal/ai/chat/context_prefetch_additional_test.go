package chat

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func newTestReadState(snapshot models.StateSnapshot) unifiedresources.ReadState {
	rr := unifiedresources.NewRegistry(nil)
	rr.IngestSnapshot(snapshot)
	return rr
}

func TestContextPrefetcherCanonicalGuestMentionsKeepIndependentInstancesDistinct(t *testing.T) {
	rr := unifiedresources.NewRegistry(nil)
	rr.IngestSnapshot(models.StateSnapshot{VMs: []models.VM{
		{ID: "cluster-a:node1:101", Name: "duplicate", Instance: "cluster-a", Node: "node1", VMID: 101, Type: "qemu"},
		{ID: "cluster-b:node1:101", Name: "duplicate", Instance: "cluster-b", Node: "node1", VMID: 101, Type: "qemu"},
	}})
	prefetcher := NewContextPrefetcher(rr)
	for _, instance := range []string{"cluster-a", "cluster-b"} {
		id := unifiedresources.ProxmoxGuestCanonicalID(unifiedresources.ResourceTypeVM, instance, 101)
		if _, found := rr.Get(id); !found {
			t.Fatalf("fixture did not publish canonical guest %s", id)
		}
		mentions := prefetcher.resolveStructuredMentions([]StructuredMention{{
			ID: id, Name: "duplicate", Type: "vm", Node: "node1",
		}})
		if len(mentions) != 1 || mentions[0].UnifiedResourceID != id {
			t.Errorf("explicit canonical guest selection crossed instance boundaries: selected=%s mentions=%+v", id, mentions)
		}
	}
}

func TestContextPrefetcherCanonicalGuestMentionUsesCurrentCoordinates(t *testing.T) {
	rr := unifiedresources.NewRegistry(nil)
	id := unifiedresources.ProxmoxGuestCanonicalID(unifiedresources.ResourceTypeVM, "cluster-a", 101)
	rr.IngestSnapshot(models.StateSnapshot{VMs: []models.VM{{
		ID: "cluster-a:old-node:101", Name: "old-name", Instance: "cluster-a", Node: "old-node", VMID: 101, Type: "qemu",
	}}})
	rr.IngestSnapshot(models.StateSnapshot{VMs: []models.VM{{
		ID: "cluster-a:new-node:101", Name: "current-name", Instance: "cluster-a", Node: "new-node", VMID: 101, Type: "qemu",
	}}})
	prefetcher := NewContextPrefetcher(rr)
	mentions := prefetcher.resolveStructuredMentions([]StructuredMention{{
		ID: id, Name: "old-name", Type: "vm", Node: "old-node",
	}})
	if len(mentions) != 1 || mentions[0].UnifiedResourceID != id ||
		mentions[0].ResourceID != "101" || mentions[0].TargetID != "new-node" ||
		mentions[0].Name != "current-name" {
		t.Fatalf("canonical mention trusted stale client labels or coordinates: %+v", mentions)
	}
}

func TestContextPrefetcherUnknownCanonicalGuestCannotAttachByDisplayName(t *testing.T) {
	prefetcher := NewContextPrefetcher(newTestReadState(models.StateSnapshot{VMs: []models.VM{{
		ID: "cluster-a:node1:101", Name: "duplicate", Instance: "cluster-a", Node: "node1", VMID: 101, Type: "qemu",
	}}}))
	mentions := prefetcher.resolveStructuredMentions([]StructuredMention{{
		ID: "vm-0000000000000000", Name: "duplicate", Type: "vm", Node: "node1",
	}})
	if len(mentions) != 0 {
		t.Fatalf("unknown explicit resource ID selected a different same-name guest: %+v", mentions)
	}
}

func TestContextPrefetcher_NoReadState(t *testing.T) {
	prefetcher := NewContextPrefetcher(nil)
	if ctx := prefetcher.Prefetch(context.Background(), "check @missing", nil); ctx != nil {
		t.Fatalf("expected nil context when ReadState missing")
	}
}

func TestContextPrefetcher_UnresolvedPlainTextMentionDoesNotInjectContext(t *testing.T) {
	state := models.StateSnapshot{}
	prefetcher := NewContextPrefetcher(newTestReadState(state))
	ctx := prefetcher.Prefetch(context.Background(), "check @missing", nil)
	if ctx != nil {
		t.Fatalf("expected plain-text mentions to stay model-owned, got %#v", ctx)
	}
}

func TestContextPrefetcher_PlainTextResourceNamesDoNotPrefetch(t *testing.T) {
	state := models.StateSnapshot{
		Nodes:      []models.Node{{ID: "node1", Name: "node1"}},
		VMs:        []models.VM{{ID: "vm-1", Name: "alpha", VMID: 101, Node: "node1"}},
		Containers: []models.Container{{ID: "lxc-1", Name: "beta", VMID: 201, Node: "node1", Type: "lxc"}},
		DockerHosts: []models.DockerHost{{
			ID:       "dock1",
			Hostname: "dock1",
			Containers: []models.DockerContainer{{
				ID:    "cid1",
				Name:  "homepage",
				State: "running",
				Mounts: []models.DockerContainerMount{{
					Source:      "/data",
					Destination: "/config",
				}},
			}},
		}},
		Hosts: []models.Host{{ID: "host1", Hostname: "host1"}},
		KubernetesClusters: []models.KubernetesCluster{{
			ID:      "k8s1",
			Name:    "k8s",
			AgentID: "agent-1",
			Pods: []models.KubernetesPod{{
				Name:      "pod1",
				Namespace: "default",
			}},
			Deployments: []models.KubernetesDeployment{{
				Name:      "dep1",
				Namespace: "default",
			}},
		}},
	}

	rs := newTestReadState(state)
	prefetcher := NewContextPrefetcher(rs)
	ctx := prefetcher.Prefetch(context.Background(), "alpha beta homepage node1 host1 pod1 dep1", nil)
	if ctx != nil {
		t.Fatalf("expected plain resource names to stay model-owned, got %#v", ctx)
	}
}

func TestContextPrefetcher_ResolveStructuredMentions(t *testing.T) {
	state := models.StateSnapshot{
		Containers: []models.Container{{ID: "lab:node1:201", Name: "beta", Instance: "lab", Node: "node1", VMID: 201, Type: "lxc"}},
		DockerHosts: []models.DockerHost{{
			ID:       "dock1",
			Hostname: "dock1",
			Containers: []models.DockerContainer{{
				ID:   "cid:part",
				Name: "homepage",
				Mounts: []models.DockerContainerMount{{
					Source:      "/data",
					Destination: "/config",
				}},
			}},
		}},
		Hosts: []models.Host{{ID: "host1", Hostname: "host1"}},
	}

	structured := []StructuredMention{
		{ID: "docker:dock1:cid:part", Name: "homepage", Type: "app-container"},
		{ID: "app-container:dock1:cid:part", Name: "homepage", Type: "app-container"},
		{ID: "agent:host1", Name: "host1", Type: "agent"},
		{ID: "system-container:node1:201", Name: "beta", Type: "system-container", Node: "node1"},
		{ID: "docker:resource:docker:abc:cid-simple", Name: "homepage2", Type: "app-container"},
	}
	state.DockerHosts = append(state.DockerHosts, models.DockerHost{
		ID:       "resource:docker:abc",
		Hostname: "dock2",
		Containers: []models.DockerContainer{{
			ID:   "cid-simple",
			Name: "homepage2",
		}},
	})

	// Build ReadState from the full state (after appending second docker host)
	rs := newTestReadState(state)
	prefetcher := NewContextPrefetcher(rs)

	mentions := prefetcher.resolveStructuredMentions(structured)
	if len(mentions) != 5 {
		t.Fatalf("expected 5 mentions, got %d", len(mentions))
	}

	if mentions[0].ResourceID != "cid:part" {
		t.Fatalf("expected container ID with colons preserved, got %q", mentions[0].ResourceID)
	}
	if len(mentions[0].BindMounts) == 0 {
		t.Fatalf("expected bind mounts on docker mention")
	}
	if mentions[1].TargetID != "dock1" || mentions[1].ResourceID != "cid:part" {
		t.Fatalf("expected canonical app-container mention to resolve through docker host, got %+v", mentions[1])
	}
	if mentions[2].ResourceType != "agent" {
		t.Fatalf("expected agent mention, got %q", mentions[2].ResourceType)
	}
	if mentions[3].ResourceType != "system-container" {
		t.Fatalf("expected system-container mention type, got %q", mentions[3].ResourceType)
	}
	if mentions[4].TargetID != "resource:docker:abc" {
		t.Fatalf("expected docker target ID with colons preserved, got %q", mentions[4].TargetID)
	}
	if mentions[4].ResourceID != "cid-simple" {
		t.Fatalf("expected docker container ID parsed correctly, got %q", mentions[4].ResourceID)
	}

	unknown := prefetcher.resolveStructuredMentions([]StructuredMention{{ID: "weird:1", Name: "mystery", Type: "weird"}})
	if len(unknown) != 0 {
		t.Fatalf("unknown resource reference must not fabricate monitoring context")
	}

	legacyHost := prefetcher.resolveStructuredMentions([]StructuredMention{{ID: "host:host1", Name: "host1", Type: "host"}})
	if len(legacyHost) != 0 {
		t.Fatalf("expected legacy host mention to be ignored, got %#v", legacyHost)
	}
}

func TestContextPrefetcher_ResolveStructuredMentions_TrueNASCanonicalAppContainer(t *testing.T) {
	rr := unifiedresources.NewRegistry(nil)
	now := time.Now().UTC()
	hostID := "agent:truenas-main"
	hostSourceID := "truenas-system:truenas-main"
	rr.IngestRecords(unifiedresources.SourceTrueNAS, []unifiedresources.IngestRecord{
		{
			SourceID: hostSourceID,
			Resource: unifiedresources.Resource{
				ID:        hostID,
				Type:      unifiedresources.ResourceTypeAgent,
				Name:      "truenas-main",
				Status:    unifiedresources.StatusOnline,
				LastSeen:  now,
				UpdatedAt: now,
				Agent: &unifiedresources.AgentData{
					Hostname: "truenas-main",
					Platform: "truenas",
				},
				TrueNAS: &unifiedresources.TrueNASData{Hostname: "truenas-main"},
			},
			Identity: unifiedresources.ResourceIdentity{Hostnames: []string{"truenas-main"}},
		},
		{
			SourceID:       "app:nextcloud",
			ParentSourceID: hostSourceID,
			Resource: unifiedresources.Resource{
				ID:         "app-container:truenas-main:nextcloud",
				Type:       unifiedresources.ResourceTypeAppContainer,
				Name:       "Nextcloud",
				Status:     unifiedresources.StatusOnline,
				LastSeen:   now,
				UpdatedAt:  now,
				ParentID:   &hostID,
				ParentName: "truenas-main",
				TrueNAS:    &unifiedresources.TrueNASData{Hostname: "truenas-main"},
				Canonical: &unifiedresources.CanonicalIdentity{
					DisplayName: "Nextcloud",
					Hostname:    "truenas-main",
					PrimaryID:   "nextcloud",
					Aliases:     []string{"Nextcloud", "nextcloud"},
				},
				Tags: []string{"truenas", "app"},
			},
			Identity: unifiedresources.ResourceIdentity{Hostnames: []string{"truenas-main"}},
		},
	})

	appContainers := rr.ListByType(unifiedresources.ResourceTypeAppContainer)
	if len(appContainers) != 1 {
		t.Fatalf("expected one TrueNAS app resource, got %d", len(appContainers))
	}
	appResourceID := appContainers[0].ID

	prefetcher := NewContextPrefetcher(rr)
	mentions := prefetcher.resolveStructuredMentions([]StructuredMention{{
		ID:   appResourceID,
		Name: "Nextcloud",
		Type: "app-container",
	}})
	if len(mentions) != 1 {
		t.Fatalf("expected one mention, got %#v", mentions)
	}
	if mentions[0].ResourceType != "app-container" {
		t.Fatalf("expected app-container mention type, got %q", mentions[0].ResourceType)
	}
	if mentions[0].TargetID != "truenas-main" || mentions[0].TargetHost != "truenas-main" {
		t.Fatalf("expected TrueNAS host routing, got %+v", mentions[0])
	}
	if mentions[0].ResourceID != appResourceID {
		t.Fatalf("expected canonical app mention id to be preserved, got %+v", mentions[0])
	}
	if mentions[0].UnifiedResourceID != appResourceID {
		t.Fatalf("expected unified resource id to be preserved, got %+v", mentions[0])
	}
}

func TestContextPrefetcher_ResolveStructuredMentions_VMwareCanonicalIDs(t *testing.T) {
	now := time.Now().UTC()
	rr := unifiedresources.NewRegistry(nil)
	rr.IngestRecords(unifiedresources.SourceVMware, []unifiedresources.IngestRecord{
		{
			SourceID: "vc-1:host:host-101",
			Resource: unifiedresources.Resource{
				Type:      unifiedresources.ResourceTypeAgent,
				Name:      "esxi-01.lab.local",
				Status:    unifiedresources.StatusOnline,
				LastSeen:  now,
				UpdatedAt: now,
				Agent: &unifiedresources.AgentData{
					AgentID:         "vc-1:host:host-101",
					Hostname:        "esxi-01.lab.local",
					Platform:        "VMware ESXi",
					CommandsEnabled: false,
				},
				VMware: &unifiedresources.VMwareData{
					ConnectionID:    "vc-1",
					ConnectionName:  "Lab VC",
					ManagedObjectID: "host-101",
					EntityType:      "host",
				},
			},
			Identity: unifiedresources.ResourceIdentity{Hostnames: []string{"esxi-01.lab.local"}},
		},
		{
			SourceID: "vc-1:vm:vm-201",
			Resource: unifiedresources.Resource{
				ID:         "vmware-vm-1",
				Type:       unifiedresources.ResourceTypeVM,
				Name:       "app-01",
				Status:     unifiedresources.StatusOnline,
				LastSeen:   now,
				UpdatedAt:  now,
				ParentName: "esxi-01.lab.local",
				VMware: &unifiedresources.VMwareData{
					ConnectionID:    "vc-1",
					ConnectionName:  "Lab VC",
					ManagedObjectID: "vm-201",
					EntityType:      "vm",
					RuntimeHostName: "esxi-01.lab.local",
				},
			},
			Identity: unifiedresources.ResourceIdentity{Hostnames: []string{"app-01"}},
		},
		{
			SourceID: "vc-1:datastore:datastore-11",
			Resource: unifiedresources.Resource{
				ID:         "vmware-datastore-1",
				Type:       unifiedresources.ResourceTypeStorage,
				Name:       "nvme-primary",
				Status:     unifiedresources.StatusOnline,
				LastSeen:   now,
				UpdatedAt:  now,
				ParentName: "Lab VC",
				VMware: &unifiedresources.VMwareData{
					ConnectionID:    "vc-1",
					ConnectionName:  "Lab VC",
					ManagedObjectID: "datastore-11",
					EntityType:      "datastore",
				},
				Storage: &unifiedresources.StorageMeta{Type: "vmfs"},
			},
		},
	})

	vmResources := rr.ListByType(unifiedresources.ResourceTypeVM)
	if len(vmResources) != 1 {
		t.Fatalf("expected one VMware VM resource, got %d", len(vmResources))
	}
	agentResources := rr.ListByType(unifiedresources.ResourceTypeAgent)
	if len(agentResources) != 1 {
		t.Fatalf("expected one VMware agent resource, got %d", len(agentResources))
	}
	storageResources := rr.ListByType(unifiedresources.ResourceTypeStorage)
	if len(storageResources) != 1 {
		t.Fatalf("expected one VMware storage resource, got %d", len(storageResources))
	}
	vmResourceID := vmResources[0].ID
	storageResourceID := storageResources[0].ID

	prefetcher := NewContextPrefetcher(rr)
	mentions := prefetcher.resolveStructuredMentions([]StructuredMention{
		{ID: "agent:vc-1:host:host-101", Name: "ESXi 01", Type: "agent"},
		{ID: vmResourceID, Name: "app-01", Type: "vm"},
		{ID: storageResourceID, Name: "nvme-primary", Type: "storage"},
	})
	if len(mentions) != 3 {
		t.Fatalf("expected three mentions, got %#v", mentions)
	}
	if mentions[0].ResourceType != "agent" || mentions[0].ResourceID != "vc-1:host:host-101" || mentions[0].TargetID != "vc-1:host:host-101" {
		t.Fatalf("expected VMware host routing coordinates, got %+v", mentions[0])
	}
	if mentions[0].SupportsControl {
		t.Fatalf("expected VMware host mention to stay read-only, got %+v", mentions[0])
	}
	if mentions[0].UnifiedResourceID != rr.Hosts()[0].ID() {
		t.Fatalf("expected VMware host unified resource id, got %+v", mentions[0])
	}
	if mentions[1].ResourceType != "vm" || mentions[1].ResourceID != "vm-201" || mentions[1].TargetID != "esxi-01.lab.local" {
		t.Fatalf("expected VMware VM routing coordinates, got %+v", mentions[1])
	}
	if mentions[1].SupportsControl {
		t.Fatalf("expected VMware VM mention to stay read-only, got %+v", mentions[1])
	}
	if mentions[1].UnifiedResourceID != vmResourceID {
		t.Fatalf("expected VMware VM unified resource id, got %+v", mentions[1])
	}
	if mentions[2].ResourceType != "storage" || mentions[2].ResourceID != storageResourceID || mentions[2].TargetID != "Lab VC" {
		t.Fatalf("expected VMware datastore shared coordinates, got %+v", mentions[2])
	}
	if mentions[2].UnifiedResourceID != storageResourceID {
		t.Fatalf("expected VMware datastore unified resource id, got %+v", mentions[2])
	}
}

func TestContextPrefetcher_ResolveStructuredMentions_IgnoresLegacyContainerAliases(t *testing.T) {
	rs := newTestReadState(models.StateSnapshot{})
	prefetcher := NewContextPrefetcher(rs)

	mentions := prefetcher.resolveStructuredMentions([]StructuredMention{
		{
			ID:   "lxc:node1:201",
			Name: "beta",
			Type: "lxc",
			Node: "node1",
		},
		{
			ID:   "container:node1:202",
			Name: "gamma",
			Type: "container",
			Node: "node1",
		},
		{
			ID:   "docker:host1:abc123",
			Name: "homepage",
			Type: "docker",
		},
		{
			ID:   "docker:host1:abc123",
			Name: "homepage",
			Type: "docker-container",
		},
	})
	if len(mentions) != 0 {
		t.Fatalf("expected legacy container aliases to be ignored, got %#v", mentions)
	}
}

func TestContextPrefetcher_FormatContextSummary(t *testing.T) {
	prefetcher := NewContextPrefetcher(newTestReadState(models.StateSnapshot{}))

	mentions := []ResourceMention{
		{
			Name:           "homepage",
			ResourceType:   "app-container",
			ResourceID:     "cid1",
			TargetID:       "dock1",
			DockerHostName: "dock1",
			DockerHostType: "standalone",
			TargetHost:     "dock1",
			BindMounts:     []MountInfo{{Source: "/data", Destination: "/config"}},
		},
		{
			Name:         "beta",
			ResourceType: "lxc",
			ResourceID:   "201",
			TargetID:     "node1",
		},
	}

	summary := prefetcher.formatContextSummary(mentions)
	if !strings.Contains(summary, "Docker container") {
		t.Fatalf("expected app-container docker context in summary")
	}
	if !strings.Contains(summary, "target_host") {
		t.Fatalf("expected target_host in summary")
	}
	if !strings.Contains(summary, "VMID") && !strings.Contains(summary, "Type: lxc") {
		t.Fatalf("expected LXC mention details in summary")
	}
}

func TestContextPrefetcher_FormatContextSummary_VMwareGuestStaysReadOnly(t *testing.T) {
	prefetcher := NewContextPrefetcher(newTestReadState(models.StateSnapshot{}))

	summary := prefetcher.formatContextSummary([]ResourceMention{{
		Name:              "app-01",
		ResourceType:      "vm",
		ResourceID:        "vm-201",
		TargetID:          "esxi-01.lab.local",
		TargetHost:        "esxi-01.lab.local",
		Adapter:           "vmware-vsphere",
		UnifiedResourceID: "vmware-vm-1",
		SupportsControl:   false,
	}})

	if !strings.Contains(summary, "read-only in Pulse") {
		t.Fatalf("expected VMware read-only note, got %q", summary)
	}
	if strings.Contains(summary, "To control this guest, use: pulse_control") {
		t.Fatalf("expected VMware summary to avoid guest-control instructions, got %q", summary)
	}
	if strings.Contains(summary, "target_host:") {
		t.Fatalf("expected VMware guest summary to avoid target_host routing, got %q", summary)
	}
	if !strings.Contains(summary, "resource_id: \"vmware-vm-1\"") {
		t.Fatalf("expected VMware guest summary to expose canonical resource_id routing, got %q", summary)
	}
	if !strings.Contains(summary, "Status, alerts, recent activity, and metrics are available through resource_id=\"vmware-vm-1\"") {
		t.Fatalf("expected VMware summary to expose resource_id read context, got %q", summary)
	}
}

func TestContextPrefetcher_FormatContextSummary_VMwareHostAndStorageStayReadOnly(t *testing.T) {
	prefetcher := NewContextPrefetcher(newTestReadState(models.StateSnapshot{}))

	summary := prefetcher.formatContextSummary([]ResourceMention{
		{
			Name:              "esxi-01.lab.local",
			ResourceType:      "agent",
			ResourceID:        "vc-1:host:host-101",
			TargetID:          "vc-1:host:host-101",
			Adapter:           "vmware-vsphere",
			UnifiedResourceID: "vmware-host-1",
			SupportsControl:   false,
		},
		{
			Name:              "nvme-primary",
			ResourceType:      "storage",
			ResourceID:        "vmware-datastore-1",
			TargetID:          "Lab VC",
			Adapter:           "vmware-vsphere",
			UnifiedResourceID: "vmware-datastore-1",
			SupportsControl:   false,
		},
	})

	if strings.Contains(summary, "Proceed directly with pulse_control") {
		t.Fatalf("expected VMware host and datastore summary to avoid control instructions, got %q", summary)
	}
	if strings.Contains(summary, "target_host:") {
		t.Fatalf("expected VMware host and datastore summary to avoid target_host routing, got %q", summary)
	}
	if got := strings.Count(summary, "Status, alerts, recent activity, and metrics are available through resource_id="); got != 2 {
		t.Fatalf("expected resource_id-based read context for VMware host and datastore, got count=%d summary=%q", got, summary)
	}
}

func TestContextPrefetcher_FormatContextSummary_TrueNASAppUsesNativeResourceReadPath(t *testing.T) {
	prefetcher := NewContextPrefetcher(newTestReadState(models.StateSnapshot{}))

	summary := prefetcher.formatContextSummary([]ResourceMention{{
		Name:              "Nextcloud",
		ResourceType:      "app-container",
		ResourceID:        "app-container:truenas-main:nextcloud",
		TargetID:          "truenas-main",
		TargetHost:        "truenas-main",
		Adapter:           "truenas",
		UnifiedResourceID: "app-container:truenas-main:nextcloud",
		SupportsControl:   true,
	}})

	if strings.Contains(summary, "Docker container") {
		t.Fatalf("expected native TrueNAS app summary to avoid Docker wording, got %q", summary)
	}
	if strings.Contains(summary, "pulse_docker") {
		t.Fatalf("expected native TrueNAS app summary to avoid pulse_docker guidance, got %q", summary)
	}
	if strings.Contains(summary, "target_host:") {
		t.Fatalf("expected native TrueNAS app summary to avoid target_host routing, got %q", summary)
	}
	if !strings.Contains(summary, "resource_id: \"app-container:truenas-main:nextcloud\"") {
		t.Fatalf("expected native TrueNAS app summary to expose resource_id routing, got %q", summary)
	}
	if !strings.Contains(summary, "Shared reads address this resource by resource_id=\"app-container:truenas-main:nextcloud\"") {
		t.Fatalf("expected native TrueNAS app summary to expose native resource context, got %q", summary)
	}
}

func TestContextPrefetcher_PrefetchStructuredMentions(t *testing.T) {
	state := models.StateSnapshot{
		VMs: []models.VM{{ID: "vm-1", Name: "alpha", VMID: 101, Node: "node1", Tags: []string{"database"}}},
	}
	prefetcher := NewContextPrefetcher(newTestReadState(state))
	ctx := prefetcher.Prefetch(context.Background(), "@alpha", []StructuredMention{
		{ID: "vm:node1:101", Name: "alpha", Type: "vm", Node: "node1"},
	})
	if ctx == nil || len(ctx.Mentions) != 1 {
		t.Fatalf("expected structured mention to be resolved")
	}
	if !strings.Contains(ctx.Summary, "Governed resource") {
		t.Fatalf("expected governed summary heading, got %q", ctx.Summary)
	}
	if strings.Contains(ctx.Summary, "alpha") {
		t.Fatalf("expected governed summary to avoid raw resource name, got %q", ctx.Summary)
	}
	if !strings.Contains(ctx.Summary, "virtual machine resource") {
		t.Fatalf("expected aiSafeSummary in governed output, got %q", ctx.Summary)
	}
}

func TestContextPrefetcher_PrefetchRestrictedMentionWithholdsPaths(t *testing.T) {
	state := models.StateSnapshot{
		Containers: []models.Container{{
			ID:   "lxc-1",
			Name: "customer-db",
			VMID: 201,
			Node: "node1",
			Type: "lxc",
			Tags: []string{"customer-data"},
		}},
	}
	prefetcher := NewContextPrefetcher(newTestReadState(state))

	ctx := prefetcher.Prefetch(context.Background(), "@customer-db", []StructuredMention{
		{ID: "system-container:node1:201", Name: "customer-db", Type: "system-container", Node: "node1"},
	})
	if ctx == nil || len(ctx.Mentions) != 1 {
		t.Fatalf("expected structured mention to be resolved")
	}
	if !strings.Contains(ctx.Summary, "local-only context") {
		t.Fatalf("expected aiSafeSummary local-only wording, got %q", ctx.Summary)
	}
	if !strings.Contains(ctx.Summary, unifiedresources.ResourcePolicyGovernedSummaryPreamble()) {
		t.Fatalf("expected governed summary preamble, got %q", ctx.Summary)
	}
	if !strings.Contains(ctx.Summary, "routing=Local Only") {
		t.Fatalf("expected policy routing in summary, got %q", ctx.Summary)
	}
	if strings.Contains(ctx.Summary, "customer-db") {
		t.Fatalf("expected restricted summary to avoid raw resource name, got %q", ctx.Summary)
	}
	if strings.Contains(ctx.Summary, "target_host") {
		t.Fatalf("expected restricted summary to withhold target_host, got %q", ctx.Summary)
	}
}

func TestContextPrefetcher_FormatContextSummary_GovernedMention(t *testing.T) {
	prefetcher := NewContextPrefetcher(newTestReadState(models.StateSnapshot{}))

	summary := prefetcher.formatContextSummary([]ResourceMention{{
		Name:          "customer-db",
		ResourceType:  "system-container",
		ResourceID:    "201",
		TargetID:      "node1",
		AISafeSummary: "system container resource; status online; local-only context",
		Policy: &unifiedresources.ResourcePolicy{
			Sensitivity: unifiedresources.ResourceSensitivityRestricted,
			Routing: unifiedresources.ResourceRoutingPolicy{
				Scope: unifiedresources.ResourceRoutingScopeLocalOnly,
				Redact: []unifiedresources.ResourceRedactionHint{
					unifiedresources.ResourceRedactionAlias,
					unifiedresources.ResourceRedactionHostname,
					unifiedresources.ResourceRedactionPath,
				},
			},
		},
	}})

	if !strings.Contains(summary, "Governed resource") {
		t.Fatalf("expected governed heading, got %q", summary)
	}
	if !strings.Contains(summary, "Policy: sensitivity=Restricted, routing=Local Only") {
		t.Fatalf("expected canonical policy line, got %q", summary)
	}
	if !strings.Contains(summary, "Redactions: Hostname, Alias, Path") {
		t.Fatalf("expected canonical redaction list, got %q", summary)
	}
	if !strings.Contains(summary, unifiedresources.ResourcePolicyGovernedSummaryFooter()) {
		t.Fatalf("expected governed summary footer, got %q", summary)
	}
	if strings.Contains(summary, "customer-db") {
		t.Fatalf("expected governed formatter to avoid raw name, got %q", summary)
	}
	if strings.Contains(summary, "target_host") {
		t.Fatalf("expected governed formatter to withhold target_host, got %q", summary)
	}
}

func TestContextPrefetcher_FormatContextSummary_UsesSharedGovernedBlockFormatter(t *testing.T) {
	prefetcher := NewContextPrefetcher(newTestReadState(models.StateSnapshot{}))

	summary := prefetcher.formatContextSummary([]ResourceMention{{
		Name:          "customer-db",
		ResourceType:  "system-container",
		ResourceID:    "201",
		TargetID:      "node1",
		AISafeSummary: "system container resource; status online; local-only context",
		Policy: &unifiedresources.ResourcePolicy{
			Sensitivity: unifiedresources.ResourceSensitivityRestricted,
			Routing: unifiedresources.ResourceRoutingPolicy{
				Scope: unifiedresources.ResourceRoutingScopeLocalOnly,
				Redact: []unifiedresources.ResourceRedactionHint{
					unifiedresources.ResourceRedactionAlias,
					unifiedresources.ResourceRedactionHostname,
				},
			},
		},
	}})

	want := unifiedresources.FormatResourcePolicyGovernedSummary(
		"system container resource; status online; local-only context",
		&unifiedresources.ResourcePolicy{
			Sensitivity: unifiedresources.ResourceSensitivityRestricted,
			Routing: unifiedresources.ResourceRoutingPolicy{
				Scope: unifiedresources.ResourceRoutingScopeLocalOnly,
				Redact: []unifiedresources.ResourceRedactionHint{
					unifiedresources.ResourceRedactionAlias,
					unifiedresources.ResourceRedactionHostname,
				},
			},
		},
	)
	if !strings.Contains(summary, want) {
		t.Fatalf("expected governed summary block to match shared formatter, got %q", summary)
	}
}
