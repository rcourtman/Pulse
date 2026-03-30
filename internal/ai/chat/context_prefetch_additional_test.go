package chat

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

type mockDiscoveryProvider struct {
	existing     map[string]*tools.ResourceDiscoveryInfo
	triggeredKey []string
	triggerErr   error
}

func (m *mockDiscoveryProvider) key(resourceType, targetID, resourceID string) string {
	return resourceType + ":" + targetID + ":" + resourceID
}

func (m *mockDiscoveryProvider) GetDiscovery(id string) (*tools.ResourceDiscoveryInfo, error) {
	return nil, nil
}

func (m *mockDiscoveryProvider) GetDiscoveryByResource(resourceType, targetID, resourceID string) (*tools.ResourceDiscoveryInfo, error) {
	if m.existing == nil {
		return nil, nil
	}
	return m.existing[m.key(resourceType, targetID, resourceID)], nil
}

func (m *mockDiscoveryProvider) ListDiscoveries() ([]*tools.ResourceDiscoveryInfo, error) {
	return nil, nil
}

func (m *mockDiscoveryProvider) ListDiscoveriesByType(resourceType string) ([]*tools.ResourceDiscoveryInfo, error) {
	return nil, nil
}

func (m *mockDiscoveryProvider) ListDiscoveriesByTarget(targetID string) ([]*tools.ResourceDiscoveryInfo, error) {
	return nil, nil
}

func (m *mockDiscoveryProvider) FormatForAIContext(discoveries []*tools.ResourceDiscoveryInfo) string {
	return ""
}

func (m *mockDiscoveryProvider) TriggerDiscovery(ctx context.Context, resourceType, targetID, resourceID string) (*tools.ResourceDiscoveryInfo, error) {
	m.triggeredKey = append(m.triggeredKey, m.key(resourceType, targetID, resourceID))
	if m.triggerErr != nil {
		return nil, m.triggerErr
	}
	return &tools.ResourceDiscoveryInfo{
		ResourceType: resourceType,
		TargetID:     targetID,
		ResourceID:   resourceID,
		Hostname:     targetID,
		ServiceType:  "nginx",
		ServiceName:  "nginx",
		ConfigPaths:  []string{"/etc/nginx/nginx.conf"},
		LogPaths:     []string{"/var/log/nginx/error.log"},
	}, nil
}

func newTestReadState(snapshot models.StateSnapshot) unifiedresources.ReadState {
	rr := unifiedresources.NewRegistry(nil)
	rr.IngestSnapshot(snapshot)
	return rr
}

func TestContextPrefetcher_NoReadState(t *testing.T) {
	prefetcher := NewContextPrefetcher(nil, nil)
	if ctx := prefetcher.Prefetch(context.Background(), "check @missing", nil); ctx != nil {
		t.Fatalf("expected nil context when ReadState missing")
	}
}

func TestContextPrefetcher_UnresolvedMention(t *testing.T) {
	state := models.StateSnapshot{}
	prefetcher := NewContextPrefetcher(newTestReadState(state), nil)
	ctx := prefetcher.Prefetch(context.Background(), "check @missing", nil)
	if ctx == nil || !strings.Contains(ctx.Summary, "NOT found") {
		t.Fatalf("expected unresolved mention summary, got %#v", ctx)
	}
}

func TestContextPrefetcher_ExtractResourceMentions(t *testing.T) {
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
	prefetcher := NewContextPrefetcher(rs, nil)
	mentions := prefetcher.extractResourceMentions("alpha beta homepage node1 host1 pod1 dep1")
	if len(mentions) == 0 {
		t.Fatalf("expected mentions to be detected")
	}

	foundAppContainer := false
	for _, m := range mentions {
		if m.ResourceType == "app-container" && m.Name == "homepage" {
			foundAppContainer = true
			if m.TargetHost == "" {
				t.Fatalf("expected app-container mention to have target host")
			}
			if len(m.BindMounts) == 0 {
				t.Fatalf("expected app-container bind mounts to be captured")
			}
		}
	}
	if !foundAppContainer {
		t.Fatalf("expected app-container mention")
	}
}

func TestContextPrefetcher_ResolveStructuredMentions(t *testing.T) {
	state := models.StateSnapshot{
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
	prefetcher := NewContextPrefetcher(rs, nil)

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
	if len(unknown) != 1 || unknown[0].ResourceType != "weird" {
		t.Fatalf("expected unknown type to be preserved")
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

	prefetcher := NewContextPrefetcher(rr, nil)
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

	prefetcher := NewContextPrefetcher(rr, nil)
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
	if mentions[0].UnifiedResourceID != "agent:vc-1:host:host-101" {
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
	prefetcher := NewContextPrefetcher(rs, nil)

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

func TestContextPrefetcher_GetOrTriggerDiscovery(t *testing.T) {
	provider := &mockDiscoveryProvider{existing: map[string]*tools.ResourceDiscoveryInfo{}}
	prefetcher := NewContextPrefetcher(newTestReadState(models.StateSnapshot{}), provider)

	provider.existing[provider.key("vm", "node1", "101")] = &tools.ResourceDiscoveryInfo{
		ResourceType: "vm",
		TargetID:     "node1",
		ResourceID:   "101",
	}

	mention := ResourceMention{ResourceType: "vm", TargetID: "node1", ResourceID: "101", Name: "alpha"}
	res, err := prefetcher.getOrTriggerDiscovery(context.Background(), mention)
	if err != nil || res == nil {
		t.Fatalf("expected cached discovery, got err=%v res=%v", err, res)
	}
	if len(provider.triggeredKey) != 0 {
		t.Fatalf("expected no trigger when cached discovery exists")
	}

	mention2 := ResourceMention{ResourceType: "app-container", TargetID: "dock1", ResourceID: "cid1", Name: "homepage"}
	res, err = prefetcher.getOrTriggerDiscovery(context.Background(), mention2)
	if err != nil || res == nil {
		t.Fatalf("expected discovery trigger to succeed")
	}
	if len(provider.triggeredKey) != 1 {
		t.Fatalf("expected trigger to be called once")
	}

	mention3 := ResourceMention{ResourceType: "agent", TargetID: "host1", ResourceID: "host1", Name: "host1"}
	res, err = prefetcher.getOrTriggerDiscovery(context.Background(), mention3)
	if err != nil || res != nil {
		t.Fatalf("expected no discovery for agent type")
	}
}

func TestContextPrefetcher_FormatContextSummary(t *testing.T) {
	prefetcher := NewContextPrefetcher(newTestReadState(models.StateSnapshot{}), nil)

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

	discoveries := []*tools.ResourceDiscoveryInfo{{
		ResourceType: "docker",
		TargetID:     "dock1",
		ResourceID:   "cid1",
		Hostname:     "dock1",
		ConfigPaths:  []string{"/etc/homepage/config"},
		LogPaths:     []string{"/var/log/homepage.log"},
	}, {
		ResourceType: "lxc",
		TargetID:     "node1",
		ResourceID:   "201",
		Hostname:     "node1",
		LogPaths:     []string{"journalctl -u service"},
		DataPaths:    []string{"/var/lib/service"},
	}}

	summary := prefetcher.formatContextSummary(mentions, discoveries)
	if !strings.Contains(summary, "Docker container") {
		t.Fatalf("expected app-container docker context in summary")
	}
	if !strings.Contains(summary, "target_host") {
		t.Fatalf("expected target_host in summary")
	}
	if !strings.Contains(summary, "VMID") && !strings.Contains(summary, "Type: lxc") {
		t.Fatalf("expected LXC mention details in summary")
	}
	if !strings.Contains(summary, "Log commands") {
		t.Fatalf("expected log command formatting in summary")
	}
}

func TestContextPrefetcher_FormatContextSummary_VMwareGuestStaysReadOnly(t *testing.T) {
	prefetcher := NewContextPrefetcher(newTestReadState(models.StateSnapshot{}), nil)

	summary := prefetcher.formatContextSummary([]ResourceMention{{
		Name:              "app-01",
		ResourceType:      "vm",
		ResourceID:        "vm-201",
		TargetID:          "esxi-01.lab.local",
		TargetHost:        "esxi-01.lab.local",
		UnifiedResourceID: "vmware-vm-1",
		SupportsControl:   false,
	}}, nil)

	if !strings.Contains(summary, "read-only in Pulse") {
		t.Fatalf("expected VMware read-only note, got %q", summary)
	}
	if strings.Contains(summary, "To control this guest, use: pulse_control") {
		t.Fatalf("expected VMware summary to avoid guest-control instructions, got %q", summary)
	}
	if !strings.Contains(summary, "Use pulse_query or pulse_read only") {
		t.Fatalf("expected VMware summary to direct the assistant to read-only tools, got %q", summary)
	}
}

func TestContextPrefetcher_FormatContextSummary_VMwareHostAndStorageStayReadOnly(t *testing.T) {
	prefetcher := NewContextPrefetcher(newTestReadState(models.StateSnapshot{}), nil)

	summary := prefetcher.formatContextSummary([]ResourceMention{
		{
			Name:              "esxi-01.lab.local",
			ResourceType:      "agent",
			ResourceID:        "vc-1:host:host-101",
			TargetID:          "vc-1:host:host-101",
			UnifiedResourceID: "vmware-host-1",
			SupportsControl:   false,
		},
		{
			Name:              "nvme-primary",
			ResourceType:      "storage",
			ResourceID:        "vmware-datastore-1",
			TargetID:          "Lab VC",
			UnifiedResourceID: "vmware-datastore-1",
			SupportsControl:   false,
		},
	}, nil)

	if strings.Contains(summary, "Proceed directly with pulse_control") {
		t.Fatalf("expected VMware host and datastore summary to avoid control instructions, got %q", summary)
	}
	if got := strings.Count(summary, "Use pulse_query or pulse_read only"); got != 2 {
		t.Fatalf("expected read-only guidance for VMware host and datastore, got count=%d summary=%q", got, summary)
	}
}

func TestContextPrefetcher_ExtractHelpers(t *testing.T) {
	words := extractWords("hello-123 world")
	if len(words) != 3 {
		t.Fatalf("expected 3 words, got %d", len(words))
	}

	mentions := extractExplicitAtMentions("ping @alpha and @beta")
	if len(mentions) != 2 {
		t.Fatalf("expected 2 mentions, got %d", len(mentions))
	}

	if !matchesResource("homepage docker", []string{"homepage"}, "homepage-docker") {
		t.Fatalf("expected fuzzy match to succeed")
	}
}

func TestContextPrefetcher_PrefetchStructuredMentions(t *testing.T) {
	state := models.StateSnapshot{
		VMs: []models.VM{{ID: "vm-1", Name: "alpha", VMID: 101, Node: "node1"}},
	}
	provider := &mockDiscoveryProvider{existing: map[string]*tools.ResourceDiscoveryInfo{
		"vm:node1:101": {
			ResourceType: "vm",
			TargetID:     "node1",
			ResourceID:   "101",
			Hostname:     "node1",
			ServiceType:  "nginx",
		},
	}}

	prefetcher := NewContextPrefetcher(newTestReadState(state), provider)
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

func TestContextPrefetcher_PrefetchRestrictedMentionSkipsDiscoveryAndPaths(t *testing.T) {
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
	provider := &mockDiscoveryProvider{existing: map[string]*tools.ResourceDiscoveryInfo{}}
	prefetcher := NewContextPrefetcher(newTestReadState(state), provider)

	ctx := prefetcher.Prefetch(context.Background(), "@customer-db", []StructuredMention{
		{ID: "system-container:node1:201", Name: "customer-db", Type: "system-container", Node: "node1"},
	})
	if ctx == nil || len(ctx.Mentions) != 1 {
		t.Fatalf("expected structured mention to be resolved")
	}
	if len(provider.triggeredKey) != 0 {
		t.Fatalf("expected governed mention to skip discovery trigger, got %#v", provider.triggeredKey)
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
	prefetcher := NewContextPrefetcher(newTestReadState(models.StateSnapshot{}), nil)

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
	}}, nil)

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
	prefetcher := NewContextPrefetcher(newTestReadState(models.StateSnapshot{}), nil)

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
	}}, nil)

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
