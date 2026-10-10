package hostagent

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestAvailabilityModuleICMPPermissionDiagnosticSurvivesReport(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("synthetic ping fixture requires a POSIX shell; no real ping is used")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ping"), []byte("#!/bin/sh\nprintf '%s' 'ping: socket: Operation not permitted' >&2\nexit 1\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	module := testProbeModule(t, nil) // Exercise the shared core, not the test callback.
	target := config.NormalizeAvailabilityTarget(config.AvailabilityTarget{
		ID: "remote-icmp", Address: "192.0.2.10", Protocol: config.AvailabilityProbeICMP, Enabled: true, ConfigRevision: 7,
	})
	module.check(context.Background(), target)
	results := module.snapshotForReport()
	if len(results) != 1 {
		t.Fatalf("report has %d observations, want one", len(results))
	}
	result := results[0]
	if result.TargetID != target.ID || result.ConfigRevision != target.ConfigRevision || result.ObservationID == "" || result.CheckedAt.IsZero() {
		t.Fatalf("report lost observation identity: %+v", result)
	}
	if result.Outcome != "unreachable" || result.TransportOutcome != "unreachable" {
		t.Fatalf("report changed existing ICMP failure classification: %+v", result)
	}
	for _, required := range []string{"local permissions", "selected observation host", "server or agent", "docs/CONFIGURATION.md#icmp-probe-privileges", "NoNewPrivileges", "intentional capability/container restrictions."} {
		if !strings.Contains(result.Error, required) {
			t.Errorf("agent diagnostic %q lost %q", result.Error, required)
		}
	}
	if len(result.Error) > availabilityErrorLimit || strings.Contains(result.Error, "Pulse service unit") || strings.Contains(result.Error, "installer") {
		t.Fatalf("agent diagnostic is unbounded or blames the server: %q", result.Error)
	}
}
