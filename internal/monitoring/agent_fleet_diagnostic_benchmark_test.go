package monitoring

import (
	"fmt"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

var agentFleetBenchmarkNow = time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)
var agentFleetBenchmarkSink AgentFleetDiagnostics

// Synthetic read-only reports: the caller never polls providers or commands a
// guest. Mixed telemetry deliberately uses separate host/Docker identities.
func agentFleetBenchmarkMonitor(size int, scenario string) *Monitor {
	m := &Monitor{state: models.NewState(), config: &config.Config{}}
	for i := 0; i < size; i++ {
		name := fmt.Sprintf("node-%06d", i)
		m.state.Hosts = append(m.state.Hosts, models.Host{
			ID: "host-" + name, Hostname: name, MachineID: "machine-" + name,
			Platform: "linux", AgentVersion: "6.7.0", Status: "online",
			LastSeen: agentFleetBenchmarkNow, IntervalSeconds: 30,
		})
		if scenario == "mixed-split" {
			m.state.DockerHosts = append(m.state.DockerHosts, models.DockerHost{
				ID: "docker-" + name, AgentID: "docker-agent-" + name, Hostname: name,
				OS: "linux", AgentVersion: "6.7.0", Status: "online", LastSeen: agentFleetBenchmarkNow,
			})
		}
	}
	return m
}

func TestAgentFleetDiagnosticBenchmarkFixture(t *testing.T) {
	for _, scenario := range []string{"host-only", "mixed-split"} {
		m := agentFleetBenchmarkMonitor(10, scenario)
		result := m.GetAgentFleetDiagnosticsForTarget("6.7.0", "6.7.0", agentFleetBenchmarkNow)
		total := 10
		if scenario == "mixed-split" {
			total = 20
		}
		if len(result.Agents) != total || result.Summary.Total != total {
			t.Fatalf("%s fixture lost subjects: %+v", scenario, result.Summary)
		}
		if scenario == "mixed-split" && result.Summary.Warning != total {
			t.Fatalf("split fixture must retain all identity warnings: %+v", result.Summary)
		}
		if scenario == "host-only" && result.Summary.Healthy != total {
			t.Fatalf("ordinary fixture must stay healthy: %+v", result.Summary)
		}
	}
}

func BenchmarkAgentFleetDiagnosticRead(b *testing.B) {
	for _, scenario := range []string{"host-only", "mixed-split"} {
		for _, size := range []int{10, 100, 1000} {
			b.Run(fmt.Sprintf("%s/records-%d", scenario, size), func(b *testing.B) {
				m := agentFleetBenchmarkMonitor(size, scenario)
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					agentFleetBenchmarkSink = m.GetAgentFleetDiagnosticsForTarget("6.7.0", "6.7.0", agentFleetBenchmarkNow)
				}
			})
		}
	}
}
