package alerts

import (
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/rs/zerolog"
)

// Check both directions independently: a missing membership must not leave a
// node unmonitored, and an orphan must not suppress a node after an agent leaves.
func hostNodeMembershipErrorNoLock(m *Manager) error {
	for agentID, link := range m.hostAgentNodeLinks {
		if link.agentID != agentID || link.nodeID == "" || !link.ownsAny() {
			return fmt.Errorf("invalid primary link %q: %+v", agentID, link)
		}
		if _, ok := m.hostAgentsByNode[link.nodeID][agentID]; !ok {
			return fmt.Errorf("missing node membership for %q: %+v", agentID, link)
		}
	}
	for nodeID, agents := range m.hostAgentsByNode {
		if len(agents) == 0 {
			return fmt.Errorf("empty membership bucket for %q", nodeID)
		}
		for agentID := range agents {
			if link, ok := m.hostAgentNodeLinks[agentID]; !ok || link.nodeID != nodeID {
				return fmt.Errorf("orphan membership %q -> %q", nodeID, agentID)
			}
		}
	}
	return nil
}

// The oracle reads only primary records, sorted by agent ID. Unlike the
// production readers it neither uses the node index nor selects owners during
// map iteration. Compare complete records, not just the presence of an owner.
func assertHostNodeMembership(t *testing.T, m *Manager, nodes []string) {
	t.Helper()
	m.mu.RLock()
	if err := hostNodeMembershipErrorNoLock(m); err != nil {
		m.mu.RUnlock()
		t.Fatal(err)
	}
	links := make([]hostAgentNodeLink, 0, len(m.hostAgentNodeLinks))
	for _, link := range m.hostAgentNodeLinks {
		links = append(links, link)
	}
	m.mu.RUnlock()
	sort.Slice(links, func(i, j int) bool { return links[i].agentID < links[j].agentID })
	for _, nodeID := range nodes {
		var owners hostAgentUsageOwners
		var combined hostAgentNodeLink
		found := false
		for _, link := range links {
			if link.nodeID != strings.TrimSpace(nodeID) {
				continue
			}
			if !found {
				combined.agentID, combined.agentName, combined.nodeID = link.agentID, link.agentName, link.nodeID
			}
			found = true
			combined.cpu = combined.cpu || link.cpu
			combined.memory = combined.memory || link.memory
			combined.disk = combined.disk || link.disk
			if link.cpu && owners.cpu.agentID == "" {
				owners.cpu = link
			}
			if link.memory && owners.memory.agentID == "" {
				owners.memory = link
			}
			if link.disk && owners.disk.agentID == "" {
				owners.disk = link
			}
		}
		if got := m.linkedHostAgentsForNode(nodeID); got != owners {
			t.Fatalf("node %q metric owners = %+v, want %+v", nodeID, got, owners)
		}
		if got, ok := m.linkedHostAgentForNode(nodeID); ok != found || got != combined {
			t.Fatalf("node %q combined owner = %+v, %v; want %+v, %v", nodeID, got, ok, combined, found)
		}
		if got := m.hasHostAgentForNode(nodeID); got != found {
			t.Fatalf("node %q has agent = %v, want %v", nodeID, got, found)
		}
	}
}

func newHostNodeMembershipManager(t *testing.T) *Manager {
	t.Helper()
	m := newTestManager(t)
	m.mu.Lock()
	m.config.Enabled = true
	m.config.AgentDefaults.CPU = &HysteresisThreshold{Trigger: 80, Clear: 75}
	m.config.AgentDefaults.Memory = &HysteresisThreshold{Trigger: 85, Clear: 80}
	m.config.AgentDefaults.Disk = &HysteresisThreshold{Trigger: 90, Clear: 85}
	m.mu.Unlock()
	return m
}

func TestHostAgentNodeMembershipTransitions(t *testing.T) {
	m := newHostNodeMembershipManager(t)
	nodes := []string{"site-a/pve", "site-b/pve", "missing", "", " site-a/pve "}
	apply := func(link hostAgentNodeLink) {
		t.Helper()
		m.applyHostAgentNodeLink(link, m.beginHostAgentReport(link.agentID))
		assertHostNodeMembership(t, m, nodes)
	}

	apply(hostAgentNodeLink{agentID: "z-agent", agentName: "pve", nodeID: nodes[0], cpu: true, memory: true})
	apply(hostAgentNodeLink{agentID: "a-agent", agentName: "pve", nodeID: nodes[0], memory: true, disk: true, summaryDiskResourceID: "agent:a-agent/disk:root"})
	apply(hostAgentNodeLink{agentID: "other-site", agentName: "pve", nodeID: nodes[1], cpu: true, memory: true, disk: true})
	owners := m.linkedHostAgentsForNode(nodes[0])
	if owners.cpu.agentID != "z-agent" || owners.memory.agentID != "a-agent" || owners.disk.agentID != "a-agent" {
		t.Fatalf("split metric ownership lost: %+v", owners)
	}

	// Same-node updates must expose the new name, coverage and summary disk,
	// not cache either the prior record or the composite node owner.
	apply(hostAgentNodeLink{agentID: "a-agent", agentName: "renamed", nodeID: nodes[0], cpu: true, disk: true, summaryDiskResourceID: "agent:a-agent/disk:data"})
	older := m.beginHostAgentReport("a-agent")
	apply(hostAgentNodeLink{agentID: " a-agent ", agentName: "moved", nodeID: " site-b/pve ", cpu: true})
	m.applyHostAgentNodeLink(hostAgentNodeLink{agentID: "a-agent", nodeID: nodes[0], memory: true}, older)
	assertHostNodeMembership(t, m, nodes)
	if got := m.linkedHostAgentsForNode(nodes[1]).cpu; got.agentID != "a-agent" || got.agentName != "moved" {
		t.Fatalf("stale report changed the moved owner: %+v", got)
	}

	// Removing the last covered metric prunes its bucket, but not another
	// node with the same display name or a co-owner on the destination node.
	apply(hostAgentNodeLink{agentID: "z-agent", nodeID: nodes[0]})
	beforeOffline := m.beginHostAgentReport("a-agent")
	m.unregisterHostAgentNodeLink(" a-agent ")
	m.applyHostAgentNodeLink(hostAgentNodeLink{agentID: "a-agent", nodeID: nodes[0], cpu: true}, beforeOffline)
	assertHostNodeMembership(t, m, nodes)
	if got := m.linkedHostAgentsForNode(nodes[1]).cpu.agentID; got != "other-site" {
		t.Fatalf("offline handback lost the surviving owner: %q", got)
	}
	apply(hostAgentNodeLink{agentID: "a-agent", nodeID: nodes[0], cpu: true, memory: true})
	apply(hostAgentNodeLink{agentID: "a-agent", cpu: true})             // explicit unlink
	apply(hostAgentNodeLink{agentID: " ", nodeID: nodes[0], cpu: true}) // rejected identity

	// A real config update revokes partial coverage before the next report,
	// and global disablement removes every membership without another report.
	config := m.GetConfig()
	config.AgentDefaults.CPU = &HysteresisThreshold{}
	m.UpdateConfig(config)
	assertHostNodeMembership(t, m, nodes)
	if got := m.linkedHostAgentsForNode(nodes[1]); got.cpu.agentID != "" || got.memory.agentID != "other-site" {
		t.Fatalf("config did not revoke just CPU: %+v", got)
	}
	config = m.GetConfig()
	config.DisableAllAgents = true
	m.UpdateConfig(config)
	assertHostNodeMembership(t, m, nodes)
	if len(m.hostAgentsByNode) != 0 || len(m.hostAgentNodeLinks) != 0 {
		t.Fatal("global agent disablement retained node membership")
	}
}

func TestHostAgentNodeMembershipMatchesForwardLinks(t *testing.T) {
	originalLevel := zerolog.GlobalLevel()
	zerolog.SetGlobalLevel(zerolog.Disabled)
	defer zerolog.SetGlobalLevel(originalLevel)
	m := newHostNodeMembershipManager(t)
	random := rand.New(rand.NewSource(20261108))
	nodes := []string{"", "missing"}
	for i := 0; i < 12; i++ {
		nodes = append(nodes, fmt.Sprintf("site-%02d/pve", i))
	}
	linkFor := func(agentID string) hostAgentNodeLink {
		mask := random.Intn(8)
		return hostAgentNodeLink{
			agentID: agentID, agentName: fmt.Sprintf("pve-%d", random.Intn(3)),
			nodeID: nodes[2+random.Intn(12)], cpu: mask&1 != 0, memory: mask&2 != 0, disk: mask&4 != 0,
			summaryDiskResourceID: "agent:" + agentID + "/disk:root",
		}
	}
	for step := 0; step < 500; step++ {
		agentID := fmt.Sprintf("agent-%02d", random.Intn(24))
		switch random.Intn(5) {
		case 0, 1:
			m.applyHostAgentNodeLink(linkFor(agentID), m.beginHostAgentReport(agentID))
		case 2:
			older := m.beginHostAgentReport(agentID)
			m.applyHostAgentNodeLink(linkFor(agentID), m.beginHostAgentReport(agentID))
			m.applyHostAgentNodeLink(linkFor(agentID), older)
		case 3:
			older := m.beginHostAgentReport(agentID)
			m.unregisterHostAgentNodeLink(agentID)
			m.applyHostAgentNodeLink(linkFor(agentID), older)
		case 4:
			m.mu.Lock()
			m.config.DisableAllAgents = random.Intn(4) == 0
			m.config.AgentDefaults.CPU.Trigger = float64(random.Intn(2) * 80)
			m.reconcileHostAgentNodeLinksNoLock()
			m.mu.Unlock()
		}
		assertHostNodeMembership(t, m, nodes)
	}
}

func TestHostAgentNodeMembershipConcurrentTransitions(t *testing.T) {
	originalLevel := zerolog.GlobalLevel()
	zerolog.SetGlobalLevel(zerolog.Disabled)
	defer zerolog.SetGlobalLevel(originalLevel)
	m := newHostNodeMembershipManager(t)
	start := make(chan struct{})
	var workers sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		workers.Add(1)
		go func(worker int) {
			defer workers.Done()
			<-start
			agentID := fmt.Sprintf("agent-%02d", worker)
			for step := 0; step < 80; step++ {
				link := hostAgentNodeLink{agentID: agentID, nodeID: fmt.Sprintf("site-%d/pve", step%3), cpu: true, memory: step%2 == 0}
				older := m.beginHostAgentReport(agentID)
				if step%3 == 0 {
					m.unregisterHostAgentNodeLink(agentID)
				} else {
					m.applyHostAgentNodeLink(link, m.beginHostAgentReport(agentID))
				}
				m.applyHostAgentNodeLink(link, older)
			}
		}(worker)
	}
	workers.Add(1)
	go func() {
		defer workers.Done()
		<-start
		for step := 0; step < 80; step++ {
			m.mu.Lock()
			m.config.DisableAllAgents = step%2 == 0
			m.reconcileHostAgentNodeLinksNoLock()
			m.mu.Unlock()
		}
	}()
	for reader := 0; reader < 4; reader++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			for step := 0; step < 500; step++ {
				nodeID := fmt.Sprintf("site-%d/pve", step%4) // includes an absent node
				m.linkedHostAgentsForNode(nodeID)
				m.linkedHostAgentForNode(nodeID)
				m.hasHostAgentForNode(nodeID)
				m.mu.RLock()
				err := hostNodeMembershipErrorNoLock(m)
				m.mu.RUnlock()
				if err != nil {
					t.Error(err)
					return
				}
			}
		}()
	}
	close(start)
	workers.Wait()
	m.mu.Lock()
	m.config.DisableAllAgents = true
	m.reconcileHostAgentNodeLinksNoLock()
	m.mu.Unlock()
	assertHostNodeMembership(t, m, []string{"site-0/pve", "site-1/pve", "site-2/pve", "site-3/pve"})
	if len(m.hostAgentsByNode) != 0 {
		t.Fatal("final revocation left node membership behind")
	}
}
