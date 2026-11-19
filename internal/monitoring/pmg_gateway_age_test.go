package monitoring

import (
	"reflect"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
)

const pmgGatewayAgeAlertID = "pmg-gateway::pmg-gateway-oldest-message"

type pmgGatewayAgeStep struct {
	name    string
	nodes   []pmgNodeQueuePhase
	level   alerts.AlertLevel
	minutes float64
	retain  bool
	drained bool
}

func pmgGatewayAgeSteps(multi bool) []pmgGatewayAgeStep {
	deferred := func(bucket string, lowerSeconds int64) pmgNodeQueuePhase {
		return pmgNodeQueuePhase{deferred: 1, hold: 3, deferredBucket: bucket, wantAgeSeconds: lowerSeconds}
	}
	heldOnly := pmgNodeQueuePhase{hold: 3} // The fixture puts held mail in 1280m+.
	critical, warning := deferred("160m", 80*60), deferred("40m", 20*60)
	steps := []pmgGatewayAgeStep{
		{"cold-held-only", []pmgNodeQueuePhase{heldOnly}, "", 0, false, false},
		{"critical", []pmgNodeQueuePhase{critical}, alerts.AlertLevelCritical, 80, false, false},
		{"ambiguous-critical", []pmgNodeQueuePhase{deferred("80m", 40*60)}, alerts.AlertLevelCritical, 80, true, false},
		{"missing-delivery-total", []pmgNodeQueuePhase{{hold: 3, missingTotal: "deferred"}}, alerts.AlertLevelCritical, 80, true, false},
		{"null-delivery-total", []pmgNodeQueuePhase{{hold: 3, nullTotal: "deferred"}}, alerts.AlertLevelCritical, 80, true, false},
		{"unread-node", []pmgNodeQueuePhase{{unread: true}}, alerts.AlertLevelCritical, 80, true, false},
		{"proven-warning", []pmgNodeQueuePhase{warning}, alerts.AlertLevelWarning, 20, false, false},
		{"ambiguous-warning", []pmgNodeQueuePhase{deferred("20m", 10*60)}, alerts.AlertLevelWarning, 20, true, false},
		{"proven-held-only", []pmgNodeQueuePhase{heldOnly}, "", 0, false, true},
		{"observed-empty", []pmgNodeQueuePhase{{}}, "", 0, false, true},
	}
	if multi {
		// A genuinely lower reading from a peer does not make another
		// node's missing/ambiguous age exact. The incident's oldest
		// demonstrated age also moves between nodes during recovery.
		peers := []pmgNodeQueuePhase{heldOnly, warning, warning, warning, heldOnly, warning, heldOnly, heldOnly, heldOnly, {}}
		for i := range steps {
			steps[i].nodes = append(steps[i].nodes, peers[i])
		}
		steps[6].nodes[0], steps[6].nodes[1] = steps[6].nodes[1], steps[6].nodes[0]
	}
	return steps
}

func TestPMGGatewayAgePollRecovery(t *testing.T) {
	for _, tc := range []struct {
		name  string
		nodes []string
	}{
		{"one-node", []string{"relay"}},
		{"two-nodes", []string{"relay", "peer"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			manager, poll := newPMGQueuePoller(t, 15, tc.nodes...)
			var start time.Time
			var prior *alerts.Alert
			for _, step := range pmgGatewayAgeSteps(len(tc.nodes) > 1) {
				t.Run(step.name, func(t *testing.T) {
					nodes, reads := poll(step.nodes...)
					wantReads := int32(4 * len(tc.nodes))
					for i, node := range nodes {
						want, queue := step.nodes[i], node.QueueStatus
						if want.unread {
							wantReads-- // The hold read failed, so the client skipped incoming.
							if queue != nil {
								t.Errorf("%s: failed read produced a queue: %+v", node.Name, queue)
							}
						} else if queue == nil || queue.OldestAge != want.wantAgeSeconds || queue.Hold != want.hold {
							t.Errorf("%s: unexpected delivery-age/held projection: %+v", node.Name, queue)
						}
					}
					if reads != wantReads {
						t.Errorf("read %d qshape rows, want %d for this poll", reads, wantReads)
					}
					var found *alerts.Alert
					for _, a := range manager.GetActiveAlerts() {
						if a.ID == pmgGatewayAgeAlertID {
							if found != nil {
								t.Fatal("duplicate gateway age incident")
							}
							found = &a
						}
					}
					if step.level == "" {
						if found != nil {
							t.Errorf("unexpected gateway age incident: %+v", found)
						}
					} else if found == nil || found.Level != step.level || found.Value != step.minutes {
						t.Errorf("gateway age=%+v, want %s at %g minutes", found, step.level, step.minutes)
					}
					if found != nil {
						if start.IsZero() {
							start = found.StartTime
						}
						if !start.Equal(found.StartTime) {
							t.Error("gateway age incident was replaced instead of updated")
						}
					}
					if step.retain && (prior == nil || !reflect.DeepEqual(prior, found)) {
						t.Error("uncertain reading changed the last observed gateway incident")
					}
					prior = found

					resolved := 0
					for _, a := range manager.GetRecentlyResolved() {
						if a.ID == pmgGatewayAgeAlertID {
							resolved++
						}
					}
					if (!step.drained && resolved != 0) || (step.drained && resolved != 1) {
						t.Errorf("resolved %d times (verified delivery drain: %t)", resolved, step.drained)
					}
				})
			}
		})
	}
}

func TestPMGGatewayAgeProvenCriticalWhilePeerUnread(t *testing.T) {
	manager, poll := newPMGQueuePoller(t, 15, "relay", "peer")
	poll(pmgNodeQueuePhase{unread: true}, pmgNodeQueuePhase{deferred: 1, deferredBucket: "1280m+"})
	for _, a := range manager.GetActiveAlerts() {
		if a.ID == pmgGatewayAgeAlertID && a.Level == alerts.AlertLevelCritical && a.Value == 1280 {
			return
		}
	}
	t.Fatal("an unread peer or open-ended bucket hid a proven critical breach")
}
