package api

import (
	"testing"
	"time"
)

func TestReconcileInterval(t *testing.T) {
	if reconcileInterval != 6*time.Hour {
		t.Errorf("expected reconcileInterval=6h, got %v", reconcileInterval)
	}
}

func TestStaleSubscriptionStateWindow(t *testing.T) {
	if staleSubscriptionState != 48*time.Hour {
		t.Errorf("expected staleSubscriptionState=48h, got %v", staleSubscriptionState)
	}
}
