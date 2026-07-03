package models

import (
	"math"
	"testing"
)

func TestNormalizeDockerContainerCPUCapacityPercent(t *testing.T) {
	tests := []struct {
		name     string
		raw      float64
		hostCPUs int
		want     float64
	}{
		{name: "single core keeps raw percent", raw: 75, hostCPUs: 1, want: 75},
		{name: "multi core normalizes Docker core percent", raw: 240, hostCPUs: 4, want: 60},
		{name: "capacity percent clamps at 100", raw: 450, hostCPUs: 4, want: 100},
		{name: "unknown core count clamps raw to capacity ceiling", raw: 150, hostCPUs: 0, want: 100},
		{name: "negative raw percent returns zero", raw: -10, hostCPUs: 4, want: 0},
		{name: "nan raw percent returns zero", raw: math.NaN(), hostCPUs: 4, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NormalizeDockerContainerCPUCapacityPercent(tt.raw, tt.hostCPUs); got != tt.want {
				t.Fatalf("NormalizeDockerContainerCPUCapacityPercent(%v, %d) = %v, want %v", tt.raw, tt.hostCPUs, got, tt.want)
			}
		})
	}
}

func TestDockerContainerCPUCapacityPercentPrefersPersistedCapacity(t *testing.T) {
	container := DockerContainer{
		CPUPercent:         240,
		CPUCapacityPercent: 62.5,
	}

	if got := DockerContainerCPUCapacityPercent(container, 4); got != 62.5 {
		t.Fatalf("DockerContainerCPUCapacityPercent() = %v, want persisted capacity percent", got)
	}
}
