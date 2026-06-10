package monitoring

import (
	"context"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring/errors"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
	"github.com/rs/zerolog/log"
)

func (m *Monitor) pollVMsWithNodes(ctx context.Context, instanceName string, clusterName string, isCluster bool, client PVEClientInterface, nodes []proxmox.Node, nodeEffectiveStatus map[string]string) {
	startTime := time.Now()

	type nodeResult struct {
		node             string
		vms              []models.VM
		templateSubjects map[string]struct{}
		err              error
	}

	resultChan := make(chan nodeResult, len(nodes))
	var wg sync.WaitGroup

	onlineNodes := 0
	for _, node := range nodes {
		if nodeEffectiveStatus[node.Node] == "online" {
			onlineNodes++
		}
	}

	prevGuests := m.previousGuestContextForInstance(instanceName)
	prevVMByID := prevGuests.vmsByID
	vmIDToHostAgent := prevGuests.hostAgentsByVMID

	log.Debug().
		Str("instance", instanceName).
		Int("totalNodes", len(nodes)).
		Int("onlineNodes", onlineNodes).
		Msg("Starting parallel VM polling")

	for _, node := range nodes {
		if nodeEffectiveStatus[node.Node] != "online" {
			log.Debug().
				Str("node", node.Node).
				Str("status", node.Status).
				Msg("Skipping offline node for VM polling")
			continue
		}

		wg.Add(1)
		go func(n proxmox.Node) {
			defer wg.Done()

			nodeStart := time.Now()
			vms, err := client.GetVMs(ctx, n.Node)
			if err != nil {
				monErr := errors.NewMonitorError(errors.ErrorTypeAPI, "get_vms", instanceName, err).WithNode(n.Node)
				log.Error().Err(monErr).Str("node", n.Node).Msg("failed to get VMs; deferring node poll until next cycle")
				resultChan <- nodeResult{node: n.Node, err: err}
				return
			}

			nodeVMs, nodeTemplateSubjects := m.pollNodeVMsWithClusterResourceBuilder(ctx, instanceName, n.Node, vms, client, prevVMByID, vmIDToHostAgent)
			nodeDuration := time.Since(nodeStart)
			log.Debug().
				Str("node", n.Node).
				Int("vms", len(nodeVMs)).
				Dur("duration", nodeDuration).
				Msg("Node VM polling completed")

			resultChan <- nodeResult{node: n.Node, vms: nodeVMs, templateSubjects: nodeTemplateSubjects}
		}(node)
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	var allVMs []models.VM
	qemuTemplateSubjects := make(map[string]struct{})
	successfulNodes := 0
	failedNodes := 0

	for result := range resultChan {
		if result.err != nil {
			failedNodes++
			continue
		}
		successfulNodes++
		allVMs = append(allVMs, result.vms...)
		for key := range result.templateSubjects {
			qemuTemplateSubjects[key] = struct{}{}
		}
	}
	if failedNodes == 0 && successfulNodes > 0 {
		m.updatePVEBackupTemplateSubjectsForType(instanceName, "qemu", qemuTemplateSubjects)
	}

	if len(allVMs) == 0 && len(nodes) > 0 {
		allVMs = append(allVMs, prevGuests.vms...)
		prevVMCount := len(prevGuests.vms)
		if prevVMCount > 0 {
			log.Warn().
				Str("instance", instanceName).
				Int("prevVMs", prevVMCount).
				Int("successfulNodes", successfulNodes).
				Int("totalNodes", len(nodes)).
				Msg("Traditional polling returned zero VMs but had VMs before - preserving previous VMs")
		}
	}

	m.state.UpdateVMsForInstance(instanceName, allVMs)

	if !shouldSkipNativeMockStateMetricWrites() {
		now := time.Now()
		for _, vm := range allVMs {
			if vm.Status != "running" {
				continue
			}
			// IO/network series are not recorded on the traditional polling
			// path (parity with the historical inline writes).
			m.recordGuestMetric("vm", vm.ID, vm.CPU*100, vm.Memory.Usage, vm.Disk.Usage, -1, -1, -1, -1, now)
		}
	}

	duration := time.Since(startTime)
	log.Debug().
		Str("instance", instanceName).
		Int("totalVMs", len(allVMs)).
		Int("successfulNodes", successfulNodes).
		Int("failedNodes", failedNodes).
		Dur("duration", duration).
		Msg("Parallel VM polling completed")
}
