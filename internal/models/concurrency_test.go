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
