package websocket

import (
	"fmt"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// benchmarkFrontendState isolates once-per-broadcast snapshot/delta work at
// the reported fleet scale. It is not a network or installed CPU benchmark.
func benchmarkFrontendState(count int) models.StateFrontend {
	state := models.EmptyStateFrontend()
	state.Resources = make([]models.ResourceFrontend, count)
	for i := range state.Resources {
		name := fmt.Sprintf("vm-%d", i)
		state.Resources[i] = models.ResourceFrontend{
			ID:           fmt.Sprintf("proxmox:lab:%d", i+100),
			Type:         "vm",
			Name:         name,
			DisplayName:  name,
			PlatformType: "proxmox",
			SourceType:   "proxmox",
			Status:       "online",
			CPU:          &models.ResourceMetricFrontend{Current: 25},
			Memory:       &models.ResourceMetricFrontend{Current: 60},
			Disk:         &models.ResourceMetricFrontend{Current: 45},
		}
	}
	return state
}

func BenchmarkBuildClientStateSnapshot1000Resources(b *testing.B) {
	state := benchmarkFrontendState(1000)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := buildClientStateSnapshot(state); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkBuildClientStateDelta1000Resources(b *testing.B) {
	previousState := benchmarkFrontendState(1000)
	currentState := previousState
	currentState.Resources = append([]models.ResourceFrontend(nil), previousState.Resources...)
	currentState.Resources[0].CPU = &models.ResourceMetricFrontend{Current: 26}
	previous, err := buildClientStateSnapshot(previousState)
	if err != nil {
		b.Fatal(err)
	}
	current, err := buildClientStateSnapshot(currentState)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := buildClientStateDelta(previous, current); err != nil {
			b.Fatal(err)
		}
	}
}
