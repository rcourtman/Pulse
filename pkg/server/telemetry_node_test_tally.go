package server

import (
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/telemetry"
)

// applyNodeTestTelemetrySnapshot adds content-free node connection test counts
// from the local tally.
//
// ConfiguredConnections reports only connections that were saved, so without
// these an install that tried to reach a node and could not is indistinguishable
// from one that never opened the add-node dialog: both report zero connections
// and stall at the "secured" activation stage.
//
// The window is the shared install-ID rotation window, the same one the
// Pulse Intelligence counters use, so no counter outlives the pseudonymous
// identifier it is reported against.
func applyNodeTestTelemetrySnapshot(snap *telemetry.Snapshot, persistence *config.ConfigPersistence, now time.Time) {
	if snap == nil || persistence == nil {
		return
	}
	if now.IsZero() {
		now = time.Now()
	}
	tally, err := persistence.LoadNodeTestTally()
	if err != nil || tally == nil {
		return
	}
	since := now.UTC().Add(-telemetry.PulseIntelligenceTelemetryWindow)
	snap.NodeTestAttempts30d = tally.AttemptsSince(since)
	snap.NodeTestFailures30d = tally.FailuresSince(since)
}
