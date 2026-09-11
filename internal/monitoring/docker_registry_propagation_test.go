package monitoring

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
)

// Consumes the real collector's synthetic registry response qualification output.
func TestDockerRegistryReportQualification(t *testing.T) {
	path := os.Getenv("PULSE_REGISTRY_REPORT_PROOF")
	if path == "" {
		t.Skip("set PULSE_REGISTRY_REPORT_PROOF to collector qualification output")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var reports []agentsdocker.Report
	if err := json.Unmarshal(data, &reports); err != nil {
		t.Fatal(err)
	}
	if len(reports) != 2 {
		t.Fatal("expected failure and recovery reports")
	}
	previous := mock.IsMockEnabled()
	mustSetMockEnabled(t, false)
	t.Cleanup(func() { mustSetMockEnabled(t, previous) })
	m := newTestMonitor(t)
	var snapshots []models.StateSnapshot
	for _, report := range reports {
		host, err := m.ApplyDockerReport(report, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(host.Containers) != len(report.Containers) {
			t.Fatal("container inventory changed")
		}
		for i, c := range host.Containers {
			want := report.Containers[i].UpdateStatus
			got := c.UpdateStatus
			if got == nil || got.CurrentDigest != want.CurrentDigest || got.LatestDigest != want.LatestDigest || got.Error != want.Error || got.UpdateAvailable != want.UpdateAvailable {
				t.Fatalf("ingest altered status: %+v vs %+v", got, want)
			}
		}
		snapshots = append(snapshots, models.StateSnapshot{DockerHosts: []models.DockerHost{host}, LastUpdate: report.Timestamp})
	}
	if path := os.Getenv("PULSE_REGISTRY_SNAPSHOT_PROOF"); path != "" {
		data, err := json.MarshalIndent(snapshots, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
}
