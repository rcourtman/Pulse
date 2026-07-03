package models

import "math"

// NormalizeDockerContainerCPUCapacityPercent converts Docker's per-core CPU
// convention into host-capacity percentage. Docker reports 100% per CPU, so a
// container using all of a 4-vCPU host can report 400% raw CPU.
func NormalizeDockerContainerCPUCapacityPercent(rawPercent float64, hostCPUs int) float64 {
	if math.IsNaN(rawPercent) || math.IsInf(rawPercent, 0) || rawPercent <= 0 {
		return 0
	}
	if hostCPUs > 1 {
		rawPercent = rawPercent / float64(hostCPUs)
	}
	if rawPercent > 100 {
		return 100
	}
	return rawPercent
}

func DockerContainerCPUCapacityPercent(container DockerContainer, hostCPUs int) float64 {
	if container.CPUCapacityPercent > 0 {
		return NormalizeDockerContainerCPUCapacityPercent(container.CPUCapacityPercent, 1)
	}
	return NormalizeDockerContainerCPUCapacityPercent(container.CPUPercent, hostCPUs)
}
