package api

import (
	"context"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/api/chartapi"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
)

// This file exercises four previously-uncovered pure builders/mutators in the
// api package. Every test function is prefixed with TestBranchcov0724pm so the
// run can be scoped with -run "^TestBranchcov0724pm".
//
// Targets:
//   - restoreAgentExecMetadata                        (agent_exec_token_binding.go)
//   - buildAlertConnectionSnapshotsWithRuntimeSources (connections_alerts.go)
//   - mockProtectionPostures                          (recovery_handlers.go)
//   - chartapi.BuildMockWorkloadMetricHistorySeries            (router.go)

// ---------------------------------------------------------------------------
// restoreAgentExecMetadata
// ---------------------------------------------------------------------------

func TestBranchcov0724pmRestoreAgentExecMetadata(t *testing.T) {
	t.Run("restores_present_key_to_its_snapshotted_value", func(t *testing.T) {
		metadata := map[string]string{
			"bound_agent_id": "agent-NEW",
			"unrelated":      "keep",
		}
		snapshot := map[string]agentExecMetadataValue{
			"bound_agent_id": {value: "agent-OLD", present: true},
		}
		restoreAgentExecMetadata(metadata, snapshot)
		if metadata["bound_agent_id"] != "agent-OLD" {
			t.Fatalf("bound_agent_id = %q, want %q", metadata["bound_agent_id"], "agent-OLD")
		}
		if metadata["unrelated"] != "keep" {
			t.Fatalf("unrelated key should be untouched, got %q", metadata["unrelated"])
		}
	})

	t.Run("deletes_key_that_was_absent_in_snapshot", func(t *testing.T) {
		metadata := map[string]string{
			"bound_hostname": "host-1",
			"keep":           "yes",
		}
		snapshot := map[string]agentExecMetadataValue{
			"bound_hostname": {value: "", present: false},
		}
		restoreAgentExecMetadata(metadata, snapshot)
		if _, stillPresent := metadata["bound_hostname"]; stillPresent {
			t.Fatalf("bound_hostname should have been deleted but is still present")
		}
		if metadata["keep"] != "yes" {
			t.Fatalf("keep key should be untouched, got %q", metadata["keep"])
		}
	})

	t.Run("empty_snapshot_leaves_metadata_unchanged", func(t *testing.T) {
		metadata := map[string]string{"a": "1", "b": "2"}

		restoreAgentExecMetadata(metadata, nil)
		if len(metadata) != 2 || metadata["a"] != "1" || metadata["b"] != "2" {
			t.Fatalf("nil snapshot mutated metadata: %+v", metadata)
		}

		restoreAgentExecMetadata(metadata, map[string]agentExecMetadataValue{})
		if len(metadata) != 2 || metadata["a"] != "1" || metadata["b"] != "2" {
			t.Fatalf("empty non-nil snapshot mutated metadata: %+v", metadata)
		}
	})

	t.Run("empty_metadata_map_gains_present_keys", func(t *testing.T) {
		metadata := map[string]string{}
		snapshot := map[string]agentExecMetadataValue{
			"bound_agent_id": {value: "agent-1", present: true},
			"bound_hostname": {value: "", present: false},
		}
		restoreAgentExecMetadata(metadata, snapshot)
		if len(metadata) != 1 || metadata["bound_agent_id"] != "agent-1" {
			t.Fatalf("expected only bound_agent_id=agent-1, got %+v", metadata)
		}
	})

	t.Run("mixed_snapshot_restores_and_deletes_in_one_pass", func(t *testing.T) {
		metadata := map[string]string{
			"keep":       "unchanged",
			"restore_me": "current-wrong",
			"delete_me":  "should-vanish",
		}
		snapshot := map[string]agentExecMetadataValue{
			"restore_me": {value: "correct-value", present: true},
			"delete_me":  {value: "", present: false},
			"absent_key": {value: "new", present: true},
		}
		restoreAgentExecMetadata(metadata, snapshot)
		if metadata["keep"] != "unchanged" {
			t.Fatalf("keep = %q, want %q", metadata["keep"], "unchanged")
		}
		if metadata["restore_me"] != "correct-value" {
			t.Fatalf("restore_me = %q, want %q", metadata["restore_me"], "correct-value")
		}
		if _, ok := metadata["delete_me"]; ok {
			t.Fatalf("delete_me should have been removed")
		}
		if metadata["absent_key"] != "new" {
			t.Fatalf("absent_key = %q, want %q", metadata["absent_key"], "new")
		}
		if len(metadata) != 3 {
			t.Fatalf("expected 3 keys after mixed restore/delete, got %d: %+v", len(metadata), metadata)
		}
	})
}

// ---------------------------------------------------------------------------
// buildAlertConnectionSnapshotsWithRuntimeSources
// ---------------------------------------------------------------------------

func TestBranchcov0724pmBuildAlertConnectionSnapshotsWithRuntimeSources(t *testing.T) {
	previousMock := mock.IsMockEnabled()
	if err := mock.SetEnabled(false); err != nil {
		t.Fatalf("failed to disable mock mode: %v", err)
	}
	t.Cleanup(func() { _ = mock.SetEnabled(previousMock) })

	ctx := context.Background()

	t.Run("empty_runtime_sources_yields_platform_snapshots_from_config", func(t *testing.T) {
		cfg := &config.Config{
			PVEInstances: []config.PVEInstance{{Name: "node-1", Host: "https://n1.lan:8006"}},
			PBSInstances: []config.PBSInstance{{Name: "backup-1", Host: "https://b1.lan:8007"}},
		}
		got := buildAlertConnectionSnapshotsWithRuntimeSources(ctx, cfg, nil, nil, aggregatorRuntimeSources{})
		if len(got) != 2 {
			t.Fatalf("expected 2 snapshots (PVE+PBS), got %d: %+v", len(got), got)
		}
		seen := map[alerts.ConnectionType]bool{}
		for _, snap := range got {
			seen[snap.Type] = true
			if snap.Name == "" {
				t.Fatalf("snapshot name should not be empty: %+v", snap)
			}
		}
		if !seen[alerts.ConnectionTypePVE] {
			t.Fatalf("expected a PVE snapshot, got types %+v", seen)
		}
		if !seen[alerts.ConnectionTypePBS] {
			t.Fatalf("expected a PBS snapshot, got types %+v", seen)
		}
	})

	t.Run("source_with_no_matching_connection_is_dropped", func(t *testing.T) {
		cfg := &config.Config{
			PVEInstances: []config.PVEInstance{{Name: "solo-pve", Host: "https://s.lan:8006"}},
		}
		runtime := aggregatorRuntimeSources{
			vmwarePoller:  &monitoring.VMwarePoller{},
			truenasPoller: &monitoring.TrueNASPoller{},
		}
		got := buildAlertConnectionSnapshotsWithRuntimeSources(ctx, cfg, nil, nil, runtime)
		if len(got) != 1 {
			t.Fatalf("expected exactly 1 snapshot, got %d: %+v", len(got), got)
		}
		if got[0].Type != alerts.ConnectionTypePVE {
			t.Fatalf("expected only PVE snapshot, got type %q", got[0].Type)
		}
	})

	t.Run("duplicate_config_sources_produce_duplicate_snapshots", func(t *testing.T) {
		cfg := &config.Config{
			PVEInstances: []config.PVEInstance{
				{Name: "dup", Host: "https://d1.lan:8006"},
				{Name: "dup", Host: "https://d2.lan:8006"},
			},
		}
		got := buildAlertConnectionSnapshotsWithRuntimeSources(ctx, cfg, nil, nil, aggregatorRuntimeSources{})
		if len(got) != 2 {
			t.Fatalf("expected 2 snapshots for duplicate sources, got %d: %+v", len(got), got)
		}
		for _, snap := range got {
			if snap.ID != "pve:dup" {
				t.Fatalf("expected snapshot ID pve:dup, got %q", snap.ID)
			}
			if snap.Type != alerts.ConnectionTypePVE {
				t.Fatalf("expected PVE type, got %q", snap.Type)
			}
		}
	})

	t.Run("nil_config_and_empty_runtime_produce_no_snapshots", func(t *testing.T) {
		got := buildAlertConnectionSnapshotsWithRuntimeSources(ctx, nil, nil, nil, aggregatorRuntimeSources{})
		if len(got) != 0 {
			t.Fatalf("expected 0 snapshots with nil config and mock disabled, got %d: %+v", len(got), got)
		}
	})
}

// ---------------------------------------------------------------------------
// mockProtectionPostures
// ---------------------------------------------------------------------------

func TestBranchcov0724pmMockProtectionPostures(t *testing.T) {
	now := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)

	successBackupPoint := func(resourceID string) recovery.RecoveryPoint {
		completed := now
		return recovery.RecoveryPoint{
			ID:                "pt-" + resourceID,
			Provider:          recovery.ProviderProxmoxPVE,
			Kind:              recovery.KindBackup,
			Mode:              recovery.ModeSnapshot,
			Outcome:           recovery.OutcomeSuccess,
			CompletedAt:       &completed,
			SubjectResourceID: resourceID,
			ProviderScope:     "scope-" + resourceID,
		}
	}

	t.Run("empty_points_yield_empty_result", func(t *testing.T) {
		postures, total := mockProtectionPostures(nil, recovery.ProtectionPostureQuery{}, now)
		if total != 0 || len(postures) != 0 {
			t.Fatalf("expected empty result, got %d postures, total %d", len(postures), total)
		}
	})

	t.Run("blank_resource_id_is_dropped", func(t *testing.T) {
		pt := successBackupPoint("res-1")
		pt.SubjectResourceID = "   "
		postures, total := mockProtectionPostures([]recovery.RecoveryPoint{pt}, recovery.ProtectionPostureQuery{}, now)
		if total != 0 || len(postures) != 0 {
			t.Fatalf("expected empty result after dropping blank resource ID, got %d postures", len(postures))
		}
	})

	t.Run("distinct_resources_produce_sorted_postures_with_shape", func(t *testing.T) {
		points := []recovery.RecoveryPoint{
			successBackupPoint("res-b"),
			successBackupPoint("res-a"),
		}
		postures, total := mockProtectionPostures(points, recovery.ProtectionPostureQuery{}, now)
		if total != 2 {
			t.Fatalf("expected total 2, got %d", total)
		}
		if len(postures) != 2 {
			t.Fatalf("expected 2 postures, got %d", len(postures))
		}
		if postures[0].SubjectResourceID != "res-a" || postures[1].SubjectResourceID != "res-b" {
			t.Fatalf("postures not sorted by resource ID: %q then %q",
				postures[0].SubjectResourceID, postures[1].SubjectResourceID)
		}
		for _, p := range postures {
			if p.State != recovery.ProtectionStateProtected {
				t.Fatalf("expected protected state for %q, got %q", p.SubjectResourceID, p.State)
			}
			if p.EvaluatedAt != now {
				t.Fatalf("EvaluatedAt for %q = %v, want %v", p.SubjectResourceID, p.EvaluatedAt, now)
			}
			if p.Explanation == "" {
				t.Fatalf("Explanation must be non-empty for %q", p.SubjectResourceID)
			}
			if len(p.ProviderStates) == 0 {
				t.Fatalf("ProviderStates must be non-empty for %q", p.SubjectResourceID)
			}
		}
	})

	t.Run("state_filter_excludes_non_matching_postures", func(t *testing.T) {
		failedPoint := successBackupPoint("res-bad")
		failedPoint.Outcome = recovery.OutcomeFailed
		failedPoint.ID = "pt-failed"

		points := []recovery.RecoveryPoint{
			successBackupPoint("res-good"),
			failedPoint,
		}
		query := recovery.ProtectionPostureQuery{State: recovery.ProtectionStateProtected}
		postures, total := mockProtectionPostures(points, query, now)
		if total != 1 {
			t.Fatalf("expected total 1 after filtering to Protected, got %d", total)
		}
		if len(postures) != 1 || postures[0].SubjectResourceID != "res-good" {
			t.Fatalf("expected only res-good, got %+v", postures)
		}
		if postures[0].State != recovery.ProtectionStateProtected {
			t.Fatalf("expected protected state, got %q", postures[0].State)
		}
	})

	t.Run("failed_backup_with_complete_history_is_unprotected", func(t *testing.T) {
		failedPoint := successBackupPoint("res-fail")
		failedPoint.Outcome = recovery.OutcomeFailed
		failedPoint.ID = "pt-failed"

		postures, total := mockProtectionPostures(
			[]recovery.RecoveryPoint{failedPoint},
			recovery.ProtectionPostureQuery{},
			now,
		)
		if total != 1 {
			t.Fatalf("expected total 1, got %d", total)
		}
		if postures[0].State != recovery.ProtectionStateUnprotected {
			t.Fatalf("expected unprotected state for failed backup, got %q", postures[0].State)
		}
	})

	t.Run("duplicate_provider_scope_observations_are_deduplicated", func(t *testing.T) {
		pt1 := successBackupPoint("res-1")
		pt2 := successBackupPoint("res-1")
		pt2.ID = "pt-2"
		points := []recovery.RecoveryPoint{pt1, pt2}
		postures, total := mockProtectionPostures(points, recovery.ProtectionPostureQuery{}, now)
		if total != 1 {
			t.Fatalf("expected total 1, got %d", total)
		}
		if len(postures[0].ProviderStates) != 1 {
			t.Fatalf("expected 1 provider state after observation dedup, got %d: %+v",
				len(postures[0].ProviderStates), postures[0].ProviderStates)
		}
	})

	t.Run("explicit_resource_ids_skip_pagination", func(t *testing.T) {
		points := []recovery.RecoveryPoint{
			successBackupPoint("res-1"),
			successBackupPoint("res-2"),
			successBackupPoint("res-3"),
		}
		query := recovery.ProtectionPostureQuery{
			SubjectResourceIDs: []string{"res-2", "res-3"},
			Page:               1,
			Limit:              1,
		}
		postures, total := mockProtectionPostures(points, query, now)
		if total != 2 {
			t.Fatalf("expected total 2, got %d", total)
		}
		if len(postures) != 2 {
			t.Fatalf("expected 2 postures (pagination bypassed), got %d", len(postures))
		}
		ids := []string{postures[0].SubjectResourceID, postures[1].SubjectResourceID}
		if ids[0] != "res-2" || ids[1] != "res-3" {
			t.Fatalf("unexpected posture order: %+v", ids)
		}
	})

	t.Run("pagination_applies_without_explicit_resource_ids", func(t *testing.T) {
		points := []recovery.RecoveryPoint{
			successBackupPoint("res-1"),
			successBackupPoint("res-2"),
			successBackupPoint("res-3"),
		}
		page1, total := mockProtectionPostures(points, recovery.ProtectionPostureQuery{Page: 1, Limit: 2}, now)
		if total != 3 {
			t.Fatalf("expected total 3, got %d", total)
		}
		if len(page1) != 2 {
			t.Fatalf("expected 2 postures on page 1, got %d", len(page1))
		}
		if page1[0].SubjectResourceID != "res-1" || page1[1].SubjectResourceID != "res-2" {
			t.Fatalf("page 1 order wrong: %q, %q", page1[0].SubjectResourceID, page1[1].SubjectResourceID)
		}

		page2, total2 := mockProtectionPostures(points, recovery.ProtectionPostureQuery{Page: 2, Limit: 2}, now)
		if total2 != 3 {
			t.Fatalf("expected total 3, got %d", total2)
		}
		if len(page2) != 1 || page2[0].SubjectResourceID != "res-3" {
			t.Fatalf("page 2 expected only res-3, got %+v", page2)
		}
	})

	t.Run("deterministic_output_for_same_inputs", func(t *testing.T) {
		points := []recovery.RecoveryPoint{
			successBackupPoint("res-1"),
			successBackupPoint("res-2"),
		}
		query := recovery.ProtectionPostureQuery{}
		first, total1 := mockProtectionPostures(points, query, now)
		second, total2 := mockProtectionPostures(points, query, now)
		if total1 != total2 {
			t.Fatalf("total differs across runs: %d vs %d", total1, total2)
		}
		if !reflect.DeepEqual(first, second) {
			t.Fatalf("postures not deterministic across runs")
		}
	})
}

// ---------------------------------------------------------------------------
// chartapi.BuildMockWorkloadMetricHistorySeries
// ---------------------------------------------------------------------------

func TestBranchcov0724pmBuildMockWorkloadMetricHistorySeries(t *testing.T) {
	now := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)

	t.Run("unsupported_metric_type_returns_nil", func(t *testing.T) {
		for _, metricType := range []string{"smart_temp", "bogus", ""} {
			got := chartapi.BuildMockWorkloadMetricHistorySeries(now, time.Hour, 100, "vm", "res-1", metricType, 50.0)
			if got != nil {
				t.Fatalf("metricType %q: expected nil, got %d points", metricType, len(got))
			}
		}
	})

	t.Run("supported_metric_types_produce_well_formed_series", func(t *testing.T) {
		supported := []string{"cpu", "memory", "disk", "diskread", "diskwrite", "netin", "netout"}
		duration := 2 * time.Hour
		maxPoints := 0
		expectedLen := chartapi.TargetMockSeriesPoints(duration, maxPoints)

		for _, metricType := range supported {
			t.Run(metricType, func(t *testing.T) {
				got := chartapi.BuildMockWorkloadMetricHistorySeries(now, duration, maxPoints, "vm", "res-1", metricType, 50.0)
				if len(got) != expectedLen {
					t.Fatalf("expected %d points, got %d", expectedLen, len(got))
				}
				wantStart := now.Add(-duration)
				if !got[0].Timestamp.Equal(wantStart) {
					t.Fatalf("first timestamp = %v, want %v", got[0].Timestamp, wantStart)
				}
				if got[len(got)-1].Timestamp.After(now) {
					t.Fatalf("last timestamp %v exceeds now %v", got[len(got)-1].Timestamp, now)
				}
				for i := 0; i < len(got); i++ {
					if math.IsNaN(got[i].Value) || math.IsInf(got[i].Value, 0) {
						t.Fatalf("value at index %d is not finite: %v", i, got[i].Value)
					}
					if i > 0 && !got[i].Timestamp.After(got[i-1].Timestamp) {
						t.Fatalf("timestamps not strictly ascending at index %d: %v <= %v",
							i, got[i].Timestamp, got[i-1].Timestamp)
					}
				}
			})
		}
	})

	t.Run("max_points_caps_series_length", func(t *testing.T) {
		duration := 24 * time.Hour
		maxPoints := 50
		got := chartapi.BuildMockWorkloadMetricHistorySeries(now, duration, maxPoints, "vm", "res-1", "cpu", 50.0)
		if len(got) != maxPoints {
			t.Fatalf("expected %d points (capped by maxPoints), got %d", maxPoints, len(got))
		}
	})

	t.Run("deterministic_for_same_inputs", func(t *testing.T) {
		first := chartapi.BuildMockWorkloadMetricHistorySeries(now, 2*time.Hour, 0, "vm", "res-1", "cpu", 50.0)
		second := chartapi.BuildMockWorkloadMetricHistorySeries(now, 2*time.Hour, 0, "vm", "res-1", "cpu", 50.0)
		if !reflect.DeepEqual(first, second) {
			t.Fatalf("series not deterministic for identical inputs")
		}
	})

	t.Run("different_resource_ids_produce_different_series", func(t *testing.T) {
		a := chartapi.BuildMockWorkloadMetricHistorySeries(now, 2*time.Hour, 0, "vm", "res-1", "cpu", 50.0)
		b := chartapi.BuildMockWorkloadMetricHistorySeries(now, 2*time.Hour, 0, "vm", "res-2", "cpu", 50.0)
		if reflect.DeepEqual(a, b) {
			t.Fatalf("expected different series for different resource IDs")
		}
	})
}
