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

func TestStaleBillingWindow(t *testing.T) {
	if staleBillingWindow != 48*time.Hour {
		t.Errorf("expected staleBillingWindow=48h, got %v", staleBillingWindow)
	}
}
