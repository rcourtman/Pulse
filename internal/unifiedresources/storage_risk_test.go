package unifiedresources

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/storagehealth"
)

// --- StorageRiskFromAssessment ---

func TestStorageRiskFromAssessment_HealthyNoReasons(t *testing.T) {
	got := StorageRiskFromAssessment(storagehealth.Assessment{Level: storagehealth.RiskHealthy})
	if got != nil {
		t.Error("healthy with no reasons should return nil")
	}
}

func TestStorageRiskFromAssessment_HealthyWithReasons(t *testing.T) {
	got := StorageRiskFromAssessment(storagehealth.Assessment{
		Level: storagehealth.RiskHealthy,
		Reasons: []storagehealth.Reason{
			{Code: "info", Severity: storagehealth.RiskHealthy, Summary: "info"},
		},
	})
	if got == nil {
		t.Fatal("healthy with reasons should return non-nil")
	}
	if len(got.Reasons) != 1 {
		t.Errorf("expected 1 reason, got %d", len(got.Reasons))
	}
}

func TestStorageRiskFromAssessment_CriticalConvertsReasons(t *testing.T) {
	got := StorageRiskFromAssessment(storagehealth.Assessment{
		Level: storagehealth.RiskCritical,
		Reasons: []storagehealth.Reason{
			{Code: "pending_sectors", Severity: storagehealth.RiskCritical, Summary: "Bad sectors"},
			{Code: "temperature_high", Severity: storagehealth.RiskWarning, Summary: "Hot disk"},
		},
	})
	if got == nil {
		t.Fatal("expected non-nil")
	}
	if got.Level != storagehealth.RiskCritical {
		t.Errorf("expected critical level, got %s", got.Level)
	}
	if len(got.Reasons) != 2 {
		t.Fatalf("expected 2 reasons, got %d", len(got.Reasons))
	}
	if got.Reasons[0].Code != "pending_sectors" || got.Reasons[1].Code != "temperature_high" {
		t.Error("reasons should preserve order and codes")
	}
}

// --- StorageStatus ---

func TestStorageStatus_NilRisk(t *testing.T) {
	got := StorageStatus(StatusOnline, nil)
	if got != StatusOnline {
		t.Errorf("nil risk should return base status, got %q", got)
	}
}

func TestStorageStatus_CriticalRisk(t *testing.T) {
	risk := &StorageRisk{Level: storagehealth.RiskCritical}
	got := StorageStatus(StatusOnline, risk)
	if got != StatusWarning {
		t.Errorf("critical risk should promote to warning, got %q", got)
	}
}

func TestStorageStatus_WarningRisk(t *testing.T) {
	risk := &StorageRisk{Level: storagehealth.RiskWarning}
	got := StorageStatus(StatusOnline, risk)
	if got != StatusWarning {
		t.Errorf("warning risk should promote to warning, got %q", got)
	}
}

func TestStorageStatus_CriticalRiskButAlreadyOffline(t *testing.T) {
	risk := &StorageRisk{Level: storagehealth.RiskCritical}
	got := StorageStatus(StatusOffline, risk)
	if got != StatusOffline {
		t.Errorf("offline should not be overridden by risk, got %q", got)
	}
}

func TestStorageStatus_HealthyRisk(t *testing.T) {
	risk := &StorageRisk{Level: storagehealth.RiskHealthy}
	got := StorageStatus(StatusOnline, risk)
	if got != StatusOnline {
		t.Errorf("healthy risk should preserve base, got %q", got)
	}
}

// --- isInternalHostRAIDDevice ---

func TestIsInternalHostRAIDDevice(t *testing.T) {
	tests := []struct {
		device   string
		expected bool
	}{
		{"/dev/md0", true},
		{"/dev/md1", true},
		{"md0", true},
		{"MD0", true},
		{"/dev/md2", false},
		{"/dev/sda", false},
		{"", false},
	}
	for _, tt := range tests {
		got := isInternalHostRAIDDevice(tt.device)
		if got != tt.expected {
			t.Errorf("isInternalHostRAIDDevice(%q) = %v, want %v", tt.device, got, tt.expected)
		}
	}
}
