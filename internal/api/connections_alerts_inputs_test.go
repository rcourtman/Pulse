package api

import (
	"context"
	"reflect"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
)

func TestPlatformAlertInputsDoNotLoadAvailabilityChecks(t *testing.T) {
	t.Setenv("PULSE_DEV", "true")
	t.Setenv("PULSE_MOCK_MODE", "false")
	t.Setenv("PULSE_MOCK_KEEP_REAL_POLLING", "false")
	previousMock := mock.IsMockEnabled()
	t.Cleanup(func() {
		if err := mock.SetEnabled(previousMock); err != nil {
			t.Error(err)
		}
	})

	for _, tc := range []struct {
		name string
		demo bool
	}{
		{name: "saved checks"},
		{name: "demo checks", demo: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := mock.SetEnabled(tc.demo); err != nil {
				t.Fatal(err)
			}
			cfg := &config.Config{DataPath: t.TempDir()}
			// Give the real monitor no remote clients; its ordinary availability
			// snapshot still reads the same persisted checks as the ledger.
			monitor, err := monitoring.New(cfg)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(monitor.Stop)

			persistence := config.NewConfigPersistence(cfg.DataPath)
			if err := persistence.SaveAvailabilityTargets([]config.AvailabilityTarget{{
				ID: "synthetic-check", Name: "Synthetic check", Address: "checks.example.test",
				Protocol: config.AvailabilityProbeTCP, Port: 443, Enabled: true,
			}}); err != nil {
				t.Fatal(err)
			}
			sources := &config.Config{
				PVEInstances: []config.PVEInstance{{Name: "pve-lab", Host: "https://pve.example.test"}},
				PBSInstances: []config.PBSInstance{{Name: "pbs-lab", Host: "https://pbs.example.test"}},
				PMGInstances: []config.PMGInstance{{Name: "pmg-lab", Host: "https://pmg.example.test"}},
			}

			ctx := context.Background()
			runtime := aggregatorRuntimeSources{orgID: "default"}
			full := buildAggregatorInputsWithRuntimeSources(ctx, sources, persistence, monitor, runtime)
			fullRows := buildConnections(full)
			targets := 0
			for _, row := range fullRows {
				if row.Type == ConnectionTypeAvailability {
					targets++
				}
			}
			if targets == 0 || full.availabilityStatuses == nil {
				t.Fatal("the ordinary connections ledger must still read and project availability checks")
			}
			if !tc.demo && (len(full.availabilityTargets) != 1 || full.availabilityTargets[0].ID != "synthetic-check") {
				t.Fatal("the ordinary ledger did not read the persisted check")
			}
			runtime.platformRowsOnly = true
			lean := buildAggregatorInputsWithRuntimeSources(ctx, sources, persistence, monitor, runtime)
			lean.now = full.now
			want := snapshotConnectionsForAlerts(fullRows)
			if len(want) == 0 || (!tc.demo && len(want) != 3) {
				t.Fatalf("wrong platform fixture: got %d alert snapshots", len(want))
			}
			// The mock metrics ticker may advance between the two acquisitions.
			// Compare mock eligibility on the same snapshot used by the ledger.
			comparison := lean
			if tc.demo {
				comparison = full
				comparison.availabilityTargets, comparison.availabilityStatuses = lean.availabilityTargets, lean.availabilityStatuses
			}
			if got := snapshotConnectionsForAlerts(buildConnections(comparison)); !reflect.DeepEqual(got, want) {
				t.Fatalf("platform alert inputs changed: got %+v, want %+v", got, want)
			}
			if len(lean.availabilityTargets) != 0 || lean.availabilityStatuses != nil {
				t.Fatalf("platform alert eval acquired %d availability targets and %d statuses it never uses",
					len(lean.availabilityTargets), len(lean.availabilityStatuses))
			}
		})
	}
}
