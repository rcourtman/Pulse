package chat

import (
	"context"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

type mockPrefetchStateProvider struct {
	state models.StateSnapshot
}

func (m mockPrefetchStateProvider) GetState() models.StateSnapshot {
	return m.state
}

type mockDiscoveryProvider struct {
	existing     map[string]*tools.ResourceDiscoveryInfo
	triggeredKey []string
	triggerErr   error
}

func (m *mockDiscoveryProvider) key(resourceType, hostID, resourceID string) string {
	return resourceType + ":" + hostID + ":" + resourceID
}

func (m *mockDiscoveryProvider) GetDiscovery(id string) (*tools.ResourceDiscoveryInfo, error) {
	return nil, nil
}

func (m *mockDiscoveryProvider) GetDiscoveryByResource(resourceType, hostID, resourceID string) (*tools.ResourceDiscoveryInfo, error) {
	if m.existing == nil {
		return nil, nil
	}
	return m.existing[m.key(resourceType, hostID, resourceID)], nil
}

func (m *mockDiscoveryProvider) ListDiscoveries() ([]*tools.ResourceDiscoveryInfo, error) {
	return nil, nil
}

func (m *mockDiscoveryProvider) ListDiscoveriesByType(resourceType string) ([]*tools.ResourceDiscoveryInfo, error) {
	return nil, nil
}

func (m *mockDiscoveryProvider) ListDiscoveriesByHost(hostID string) ([]*tools.ResourceDiscoveryInfo, error) {
	return nil, nil
}

func (m *mockDiscoveryProvider) FormatForAIContext(discoveries []*tools.ResourceDiscoveryInfo) string {
	return ""
}

func (m *mockDiscoveryProvider) TriggerDiscovery(ctx context.Context, resourceType, hostID, resourceID string) (*tools.ResourceDiscoveryInfo, error) {
	m.triggeredKey = append(m.triggeredKey, m.key(resourceType, hostID, resourceID))
	if m.triggerErr != nil {
		return nil, m.triggerErr
	}
	return &tools.ResourceDiscoveryInfo{
		ResourceType: resourceType,
		HostID:       hostID,
		ResourceID:   resourceID,
		Hostname:     hostID,
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

func TestContextPrefetcher_NoStateProvider(t *testing.T) {
	prefetcher := NewContextPrefetcher(nil, nil, nil)
	if ctx := prefetcher.Prefetch(context.Background(), "check @missing", nil); ctx != nil {
		t.Fatalf("expected nil context when state provider missing")
	}
}

func TestContextPrefetcher_UnresolvedMention(t *testing.T) {
	state := models.StateSnapshot{}
	prefetcher := NewContextPrefetcher(mockPrefetchStateProvider{state: state}, nil, nil)
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
	prefetcher := NewContextPrefetcher(mockPrefetchStateProvider{state: state}, rs, nil)
	mentions := prefetcher.extractResourceMentions("alpha beta homepage node1 host1 pod1 dep1", state, rs)
	if len(mentions) == 0 {
		t.Fatalf("expected mentions to be detected")
	}

	foundDocker := false
	for _, m := range mentions {
		if m.ResourceType == "docker" && m.Name == "homepage" {
			foundDocker = true
			if m.TargetHost == "" {
				t.Fatalf("expected docker mention to have target host")
			}
			if len(m.BindMounts) == 0 {
				t.Fatalf("expected docker bind mounts to be captured")
			}
		}
	}
	if !foundDocker {
		t.Fatalf("expected docker mention")
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

	prefetcher := NewContextPrefetcher(mockPrefetchStateProvider{state: state}, nil, nil)
	structured := []StructuredMention{
		{ID: "docker:dock1:cid:part", Name: "homepage", Type: "docker"},
		{ID: "host:host1", Name: "host1", Type: "host"},
		{ID: "container:node1:201", Name: "beta", Type: "container", Node: "node1"},
		{ID: "docker:resource:docker:abc:cid-simple", Name: "homepage2", Type: "docker"},
	}
	state.DockerHosts = append(state.DockerHosts, models.DockerHost{
		ID:       "resource:docker:abc",
		Hostname: "dock2",
		Containers: []models.DockerContainer{{
			ID:   "cid-simple",
			Name: "homepage2",
		}},
	})

	mentions := prefetcher.resolveStructuredMentions(structured, state)
	if len(mentions) != 4 {
		t.Fatalf("expected 4 mentions, got %d", len(mentions))
	}

	if mentions[0].ResourceID != "cid:part" {
		t.Fatalf("expected container ID with colons preserved, got %q", mentions[0].ResourceID)
	}
	if len(mentions[0].BindMounts) == 0 {
		t.Fatalf("expected bind mounts on docker mention")
	}
	if mentions[1].ResourceType != "host" {
		t.Fatalf("expected host mention, got %q", mentions[1].ResourceType)
	}
	if mentions[2].ResourceType != "system-container" {
		t.Fatalf("expected container type normalized to system-container, got %q", mentions[2].ResourceType)
	}
	if mentions[3].HostID != "resource:docker:abc" {
		t.Fatalf("expected docker host ID with colons preserved, got %q", mentions[3].HostID)
	}
	if mentions[3].ResourceID != "cid-simple" {
		t.Fatalf("expected docker container ID parsed correctly, got %q", mentions[3].ResourceID)
	}

	unknown := prefetcher.resolveStructuredMentions([]StructuredMention{{ID: "weird:1", Name: "mystery", Type: "weird"}}, state)
	if len(unknown) != 1 || unknown[0].ResourceType != "weird" {
		t.Fatalf("expected unknown type to be preserved")
	}
}

func TestContextPrefetcher_GetOrTriggerDiscovery(t *testing.T) {
	provider := &mockDiscoveryProvider{existing: map[string]*tools.ResourceDiscoveryInfo{}}
	prefetcher := NewContextPrefetcher(mockPrefetchStateProvider{}, nil, provider)

	provider.existing[provider.key("vm", "node1", "101")] = &tools.ResourceDiscoveryInfo{
		ResourceType: "vm",
		HostID:       "node1",
		ResourceID:   "101",
	}

	mention := ResourceMention{ResourceType: "vm", HostID: "node1", ResourceID: "101", Name: "alpha"}
	res, err := prefetcher.getOrTriggerDiscovery(context.Background(), mention)
	if err != nil || res == nil {
		t.Fatalf("expected cached discovery, got err=%v res=%v", err, res)
	}
	if len(provider.triggeredKey) != 0 {
		t.Fatalf("expected no trigger when cached discovery exists")
	}

	mention2 := ResourceMention{ResourceType: "docker", HostID: "dock1", ResourceID: "cid1", Name: "homepage"}
	res, err = prefetcher.getOrTriggerDiscovery(context.Background(), mention2)
	if err != nil || res == nil {
		t.Fatalf("expected discovery trigger to succeed")
	}
	if len(provider.triggeredKey) != 1 {
		t.Fatalf("expected trigger to be called once")
	}

	mention3 := ResourceMention{ResourceType: "host", HostID: "host1", ResourceID: "host1", Name: "host1"}
	res, err = prefetcher.getOrTriggerDiscovery(context.Background(), mention3)
	if err != nil || res != nil {
		t.Fatalf("expected no discovery for host type")
	}
}

func TestContextPrefetcher_FormatContextSummary(t *testing.T) {
	prefetcher := NewContextPrefetcher(mockPrefetchStateProvider{}, nil, nil)

	mentions := []ResourceMention{
		{
			Name:           "homepage",
			ResourceType:   "docker",
			ResourceID:     "cid1",
			HostID:         "dock1",
			DockerHostName: "dock1",
			DockerHostType: "standalone",
			TargetHost:     "dock1",
			BindMounts:     []MountInfo{{Source: "/data", Destination: "/config"}},
		},
		{
			Name:         "beta",
			ResourceType: "lxc",
			ResourceID:   "201",
			HostID:       "node1",
		},
	}

	discoveries := []*tools.ResourceDiscoveryInfo{{
		ResourceType: "docker",
		HostID:       "dock1",
		ResourceID:   "cid1",
		Hostname:     "dock1",
		ConfigPaths:  []string{"/etc/homepage/config"},
		LogPaths:     []string{"/var/log/homepage.log"},
	}, {
		ResourceType: "lxc",
		HostID:       "node1",
		ResourceID:   "201",
		Hostname:     "node1",
		LogPaths:     []string{"journalctl -u service"},
		DataPaths:    []string{"/var/lib/service"},
	}}

	summary := prefetcher.formatContextSummary(mentions, discoveries)
	if !strings.Contains(summary, "Docker container") {
		t.Fatalf("expected docker context in summary")
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
			HostID:       "node1",
			ResourceID:   "101",
			Hostname:     "node1",
			ServiceType:  "nginx",
		},
	}}

	prefetcher := NewContextPrefetcher(mockPrefetchStateProvider{state: state}, nil, provider)
	ctx := prefetcher.Prefetch(context.Background(), "@alpha", []StructuredMention{
		{ID: "vm:node1:101", Name: "alpha", Type: "vm", Node: "node1"},
	})
	if ctx == nil || len(ctx.Mentions) != 1 {
		t.Fatalf("expected structured mention to be resolved")
	}
	if !strings.Contains(ctx.Summary, "alpha") {
		t.Fatalf("expected summary to include resource name")
	}
}
