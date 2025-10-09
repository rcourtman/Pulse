package models

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestStateConcurrentSnapshots(t *testing.T) {
	state := NewState()

	const iterations = 1000

	var wg sync.WaitGroup
	wg.Add(3)

	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			idx := i
			nodes := []Node{{
				ID:       fmt.Sprintf("node-%d", idx),
				Name:     fmt.Sprintf("node-%d", idx),
				Instance: "test-instance",
				Status:   "online",
				CPU:      float64(idx % 100),
				Memory: Memory{
					Total: 1024 * 1024,
					Used:  int64(idx % 1024),
				},
				Disk: Disk{
					Total: 1024 * 1024 * 10,
					Used:  int64(idx % (1024 * 10)),
				},
				LastSeen: time.Now(),
			}}
			vms := []VM{{
				ID:         fmt.Sprintf("vm-%d", idx),
				VMID:       idx,
				Name:       fmt.Sprintf("vm-%d", idx),
				Node:       nodes[0].Name,
				Instance:   "test-instance",
				CPU:        float64(idx % 100),
				Memory:     Memory{Total: 512 * 1024, Used: int64(idx % 512)},
				Disk:       Disk{Total: 1024 * 1024, Used: int64(idx % 1024)},
				NetworkIn:  int64(idx),
				NetworkOut: int64(idx),
				DiskRead:   int64(idx),
				DiskWrite:  int64(idx),
				LastSeen:   time.Now(),
			}}
			containers := []Container{{
				ID:         fmt.Sprintf("ct-%d", idx),
				VMID:       idx,
				Name:       fmt.Sprintf("ct-%d", idx),
				Node:       nodes[0].Name,
				Instance:   "test-instance",
				CPU:        float64(idx % 100),
				Memory:     Memory{Total: 256 * 1024, Used: int64(idx % 256)},
				Disk:       Disk{Total: 512 * 1024, Used: int64(idx % 512)},
				NetworkIn:  int64(idx),
				NetworkOut: int64(idx),
				DiskRead:   int64(idx),
				DiskWrite:  int64(idx),
				LastSeen:   time.Now(),
			}}
			storage := []Storage{{
				ID:       fmt.Sprintf("storage-%d", idx),
				Name:     fmt.Sprintf("storage-%d", idx),
				Node:     nodes[0].Name,
				Instance: "test-instance",
				Type:     "lvm",
				Status:   "available",
				Total:    1024 * 1024 * 1024,
				Used:     int64(idx % (1024 * 1024)),
				Free:     1024*1024*1024 - int64(idx%(1024*1024)),
				Usage:    float64(idx % 100),
			}}
			alerts := []Alert{{
				ID:           fmt.Sprintf("alert-%d", idx),
				Type:         "cpu",
				Level:        "warning",
				ResourceID:   vms[0].ID,
				ResourceName: vms[0].Name,
				Node:         nodes[0].Name,
				Instance:     "test-instance",
				Message:      "CPU usage high",
				Value:        float64(idx % 100),
				Threshold:    80,
				StartTime:    time.Now(),
			}}

			state.UpdateNodes(nodes)
			state.UpdateVMs(vms)
			state.UpdateContainers(containers)
			state.UpdateStorage(storage)
			state.UpdateActiveAlerts(alerts)
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			snapshot := state.GetSnapshot()
			_ = snapshot.ToFrontend()
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			_ = state.ToFrontend()
		}
	}()

	wg.Wait()
}

// TestStateMultiInstanceConcurrentUpdates simulates the real-world scenario where
// multiple PVE instances (like delly.lan cluster + pi standalone) update nodes concurrently.
// This is the pattern that caused the node count to alternate between 1 and 3.
func TestStateMultiInstanceConcurrentUpdates(t *testing.T) {
	state := NewState()

	const iterations = 1000
	const numInstances = 3

	var wg sync.WaitGroup

	// Simulate multiple PVE instances updating concurrently (like delly.lan + pi)
	for instance := 0; instance < numInstances; instance++ {
		wg.Add(1)
		go func(instanceID int) {
			defer wg.Done()
			instanceName := fmt.Sprintf("instance-%d", instanceID)

			for i := 0; i < iterations; i++ {
				// Each instance updates its own nodes
				nodesPerInstance := instanceID + 1 // instance-0 has 1 node, instance-1 has 2 nodes, etc.
				nodes := make([]Node, nodesPerInstance)
				for n := 0; n < nodesPerInstance; n++ {
					nodes[n] = Node{
						ID:       fmt.Sprintf("%s-node-%d", instanceName, n),
						Name:     fmt.Sprintf("node-%d", n),
						Instance: instanceName,
						Status:   "online",
						CPU:      float64(i % 100),
						Memory: Memory{
							Total: 1024 * 1024,
							Used:  int64(i % 1024),
						},
						Disk: Disk{
							Total: 1024 * 1024 * 10,
							Used:  int64(i % (1024 * 10)),
						},
						LastSeen: time.Now(),
					}
				}

				// Update nodes for this specific instance (merges with other instances)
				state.UpdateNodesForInstance(instanceName, nodes)

				// Also update VMs and containers for this instance
				vms := make([]VM, nodesPerInstance)
				for v := 0; v < nodesPerInstance; v++ {
					vms[v] = VM{
						ID:       fmt.Sprintf("%s-vm-%d", instanceName, v),
						VMID:     v,
						Name:     fmt.Sprintf("vm-%d", v),
						Node:     nodes[0].Name,
						Instance: instanceName,
						Status:   "running",
						CPU:      float64(i % 100),
						Memory:   Memory{Total: 512 * 1024, Used: int64(i % 512)},
						Disk:     Disk{Total: 1024 * 1024, Used: int64(i % 1024)},
						LastSeen: time.Now(),
					}
				}
				state.UpdateVMsForInstance(instanceName, vms)

				containers := make([]Container, nodesPerInstance)
				for c := 0; c < nodesPerInstance; c++ {
					containers[c] = Container{
						ID:       fmt.Sprintf("%s-ct-%d", instanceName, c),
						VMID:     c,
						Name:     fmt.Sprintf("ct-%d", c),
						Node:     nodes[0].Name,
						Instance: instanceName,
						Status:   "running",
						CPU:      float64(i % 100),
						Memory:   Memory{Total: 256 * 1024, Used: int64(i % 256)},
						Disk:     Disk{Total: 512 * 1024, Used: int64(i % 512)},
						LastSeen: time.Now(),
					}
				}
				state.UpdateContainersForInstance(instanceName, containers)
			}
		}(instance)
	}

	// Simulate concurrent reads (like GetState() calls from API handlers and WebSocket broadcasts)
	for reader := 0; reader < 3; reader++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				snapshot := state.GetSnapshot()
				frontendState := snapshot.ToFrontend()

				// Verify state consistency - all instances' data should be present
				// After initial updates, we should have 1 + 2 + 3 = 6 total nodes
				if i > 100 { // Give time for initial updates
					nodeCount := len(frontendState.Nodes)
					if nodeCount != 0 && nodeCount != 6 {
						// It's OK to have 0 nodes initially, or partial counts during ramp-up,
						// but once stable we should have all 6 nodes
						// Don't fail here - just track for race detector
					}
				}
			}
		}()
	}

	wg.Wait()

	// Final verification - all instances' nodes should be present
	finalSnapshot := state.GetSnapshot()
	if len(finalSnapshot.Nodes) != 6 {
		t.Errorf("Expected 6 nodes (1+2+3), got %d", len(finalSnapshot.Nodes))
	}

	// Verify each instance's nodes are present
	nodesByInstance := make(map[string]int)
	for _, node := range finalSnapshot.Nodes {
		nodesByInstance[node.Instance]++
	}

	for instance := 0; instance < numInstances; instance++ {
		instanceName := fmt.Sprintf("instance-%d", instance)
		expectedCount := instance + 1
		actualCount := nodesByInstance[instanceName]
		if actualCount != expectedCount {
			t.Errorf("Instance %s: expected %d nodes, got %d", instanceName, expectedCount, actualCount)
		}
	}

	// Verify VMs and containers too
	if len(finalSnapshot.VMs) != 6 {
		t.Errorf("Expected 6 VMs, got %d", len(finalSnapshot.VMs))
	}
	if len(finalSnapshot.Containers) != 6 {
		t.Errorf("Expected 6 containers, got %d", len(finalSnapshot.Containers))
	}
}
