package unifiedresources

import (
	"testing"
	"time"
)

func TestPresentationKeepsTrueNASConnectionHostsDistinct(t *testing.T) {
	native := func(id, connection string) Resource {
		return Resource{ID: id, Type: ResourceTypeAgent, Name: "truenas", Sources: []DataSource{SourceTrueNAS},
			Identity:      ResourceIdentity{Hostnames: []string{"truenas"}},
			TrueNAS:       &TrueNASData{Hostname: "truenas"},
			Agent:         &AgentData{AgentID: connection, Hostname: "truenas", Platform: "truenas", MachineID: "shared-vendor-serial"},
			MetricsTarget: &MetricsTarget{ResourceType: "agent", ResourceID: connection},
			LastSeen:      time.Now(), Status: StatusOnline}
	}
	for _, reverse := range []bool{false, true} {
		a, b := native("native-a", "connection-a"), native("native-b", "connection-b")
		rows := []Resource{a, b}
		if reverse {
			rows = []Resource{b, a}
		}
		parentA, parentB := a.ID, b.ID
		rows = append(rows, Resource{ID: "pool-a", Type: ResourceTypeStorage, ParentID: &parentA}, Resource{ID: "pool-b", Type: ResourceTypeStorage, ParentID: &parentB})
		got := CoalescePresentationHostResources(rows)
		if len(got) != 4 {
			t.Fatalf("distinct native connections collapsed (reverse=%t): got %d rows, want 4", reverse, len(got))
		}
		for _, r := range got {
			if r.ID == "pool-a" && (r.ParentID == nil || *r.ParentID != parentA) || r.ID == "pool-b" && (r.ParentID == nil || *r.ParentID != parentB) {
				t.Fatalf("pool borrowed another connection: %+v", r)
			}
			if r.Type == ResourceTypeAgent && (r.MetricsTarget == nil || r.MetricsTarget.ResourceID != r.Agent.AgentID) {
				t.Fatalf("host borrowed history: %+v", r)
			}
		}
	}
	caseA, caseB := native("case-a", "connection-a"), native("case-b", "Connection-A")
	if got := CoalescePresentationHostResources([]Resource{caseA, caseB}); len(got) != 2 {
		t.Fatal("case-sensitive connection IDs were treated as one")
	}
	// The guard is specific to two native TrueNAS connection identities. Keep
	// an ordinary agent enrichment and duplicate views of one connection working.
	nativeRow := native("native-a", "connection-a")
	same := native("native-duplicate", "connection-a")
	if got := CoalescePresentationHostResources([]Resource{nativeRow, same}); len(got) != 1 {
		t.Fatal("one connection's views no longer coalesce")
	}
	agent := Resource{ID: "agent-a", Type: ResourceTypeAgent, Name: "truenas", Sources: []DataSource{SourceAgent}, Identity: ResourceIdentity{Hostnames: []string{"truenas"}}, Agent: &AgentData{AgentID: "real-agent", Hostname: "truenas", MachineID: "shared-vendor-serial"}, Status: StatusOnline, LastSeen: time.Now()}
	for _, rows := range [][]Resource{{nativeRow, agent}, {agent, nativeRow}} {
		if got := CoalescePresentationHostResources(rows); len(got) != 1 {
			t.Fatal("ordinary single-connection agent enrichment stopped coalescing")
		}
	}
}
