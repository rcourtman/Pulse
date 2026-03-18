package monitoring

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
)

func TestBroadcastEscalatedAlert_NoHubIsNoOp(t *testing.T) {
	var monitor *Monitor

	assertNotPanics(t, func() {
		monitor.broadcastEscalatedAlert(nil, &alerts.Alert{ID: "alert-1"})
	})
}

func TestBroadcastEscalatedAlert_NilAlertIsNoOp(t *testing.T) {
	monitor := &Monitor{}

	assertNotPanics(t, func() {
		monitor.broadcastEscalatedAlert(nil, nil)
	})
}

func assertNotPanics(t *testing.T, fn func()) {
	t.Helper()

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("unexpected panic: %v", r)
		}
	}()

	fn()
}
