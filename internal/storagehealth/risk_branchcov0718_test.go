package storagehealth

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// Pointer helpers keep malformed-input branches explicit at the call site.
func int64Ptr(v int64) *int64 { return &v }
func intPtr(v int) *int       { return &v }

// reasonSeverity returns the severity for the first reason matching code, or
// the zero RiskLevel when no such reason exists. Used to assert both the
// presence of a reason and its severity in a single check.
func reasonSeverity(assessment Assessment, code string) (RiskLevel, bool) {
	for _, r := range assessment.Reasons {
		if r.Code == code {
			return r.Severity, true
		}
	}
	return "", false
}

// --- AssessPhysicalDisk: branch coverage over risk.go:48-88 ---

func TestAssessPhysicalDisk_Branches(t *testing.T) {
	tests := []struct {
		name        string
		disk        models.PhysicalDisk
		wantLevel   RiskLevel
		wantReasons []string // codes that MUST be present (independent of order)
	}{
		{
			// nil SmartAttributes arm: every *int64 pointer is skipped; only
			// Model/Health/Temperature/Wearout are sourced from the disk.
			name:      "healthy disk with nil smart attributes stays healthy",
			disk:      models.PhysicalDisk{Model: "Crucial MX500", Health: "PASSED", Temperature: 35, Wearout: 80},
			wantLevel: RiskHealthy,
		},
		{
			// non-nil SmartAttributes arm with all pointers set but all benign:
			// proves every `attrs.X != nil` branch executes without escalating.
			name: "healthy disk with benign smart attributes stays healthy",
			disk: models.PhysicalDisk{
				Health: "PASSED", Temperature: 35, Wearout: 80,
				SmartAttributes: &models.SMARTAttributes{
					PowerOnHours:         int64Ptr(1000),
					PowerCycles:          int64Ptr(50),
					ReallocatedSectors:   int64Ptr(0),
					PendingSectors:       int64Ptr(0),
					OfflineUncorrectable: int64Ptr(0),
					UDMACRCErrors:        int64Ptr(0),
					PercentageUsed:       intPtr(10),
					AvailableSpare:       intPtr(90),
					MediaErrors:          int64Ptr(0),
					UnsafeShutdowns:      int64Ptr(0),
				},
			},
			wantLevel: RiskHealthy,
		},
		{
			// FAILED health (no firmware-bug model) -> critical health_status.
			name:        "failed health escalates to critical",
			disk:        models.PhysicalDisk{Model: "Crucial MX500", Health: "FAILED"},
			wantLevel:   RiskCritical,
			wantReasons: []string{"health_status"},
		},
		{
			// Pending sectors via SMART attributes -> critical.
			name: "pending sectors via smart attributes critical",
			disk: models.PhysicalDisk{
				Health: "PASSED",
				SmartAttributes: &models.SMARTAttributes{
					PendingSectors: int64Ptr(5),
				},
			},
			wantLevel:   RiskCritical,
			wantReasons: []string{"pending_sectors"},
		},
		{
			// Offline uncorrectable via SMART attributes -> critical.
			name: "offline uncorrectable via smart attributes critical",
			disk: models.PhysicalDisk{
				Health: "PASSED",
				SmartAttributes: &models.SMARTAttributes{
					OfflineUncorrectable: int64Ptr(2),
				},
			},
			wantLevel:   RiskCritical,
			wantReasons: []string{"offline_uncorrectable"},
		},
		{
			// Media errors via SMART attributes -> critical.
			name: "media errors via smart attributes critical",
			disk: models.PhysicalDisk{
				Health: "PASSED",
				SmartAttributes: &models.SMARTAttributes{
					MediaErrors: int64Ptr(1),
				},
			},
			wantLevel:   RiskCritical,
			wantReasons: []string{"media_errors"},
		},
		{
			// Wearout <= 5 (sourced from disk.Wearout directly) -> critical.
			name:        "wearout at critical threshold",
			disk:        models.PhysicalDisk{Health: "PASSED", Wearout: 3},
			wantLevel:   RiskCritical,
			wantReasons: []string{"wearout_low"},
		},
		{
			// Wearout 6..9 (warning band) -> warning.
			name:        "wearout at warning threshold",
			disk:        models.PhysicalDisk{Health: "PASSED", Wearout: 8},
			wantLevel:   RiskWarning,
			wantReasons: []string{"wearout_low"},
		},
		{
			// Available spare <= 10 via SMART -> critical.
			name: "nvme available spare critical",
			disk: models.PhysicalDisk{
				Health: "PASSED",
				SmartAttributes: &models.SMARTAttributes{
					AvailableSpare: intPtr(8),
				},
			},
			wantLevel:   RiskCritical,
			wantReasons: []string{"nvme_available_spare_low"},
		},
		{
			// Available spare 11..19 via SMART -> warning.
			name: "nvme available spare warning",
			disk: models.PhysicalDisk{
				Health: "PASSED",
				SmartAttributes: &models.SMARTAttributes{
					AvailableSpare: intPtr(15),
				},
			},
			wantLevel:   RiskWarning,
			wantReasons: []string{"nvme_available_spare_low"},
		},
		{
			// PercentageUsed >= 95 via SMART -> critical.
			name: "nvme percentage used critical",
			disk: models.PhysicalDisk{
				Health: "PASSED",
				SmartAttributes: &models.SMARTAttributes{
					PercentageUsed: intPtr(97),
				},
			},
			wantLevel:   RiskCritical,
			wantReasons: []string{"nvme_percentage_used_high"},
		},
		{
			// PercentageUsed 90..94 via SMART -> warning.
			name: "nvme percentage used warning",
			disk: models.PhysicalDisk{
				Health: "PASSED",
				SmartAttributes: &models.SMARTAttributes{
					PercentageUsed: intPtr(92),
				},
			},
			wantLevel:   RiskWarning,
			wantReasons: []string{"nvme_percentage_used_high"},
		},
		{
			// Temperature >= 70 -> critical.
			name:        "temperature critical",
			disk:        models.PhysicalDisk{Health: "PASSED", Temperature: 72},
			wantLevel:   RiskCritical,
			wantReasons: []string{"temperature_high"},
		},
		{
			// Temperature 60..69 -> warning.
			name:        "temperature warning",
			disk:        models.PhysicalDisk{Health: "PASSED", Temperature: 63},
			wantLevel:   RiskWarning,
			wantReasons: []string{"temperature_high"},
		},
		{
			// Reallocated sectors > 0 via SMART -> warning.
			name: "reallocated sectors warning",
			disk: models.PhysicalDisk{
				Health: "PASSED",
				SmartAttributes: &models.SMARTAttributes{
					ReallocatedSectors: int64Ptr(4),
				},
			},
			wantLevel:   RiskWarning,
			wantReasons: []string{"reallocated_sectors"},
		},
		{
			// UDMA CRC errors > 0 via SMART -> monitor.
			name: "udma crc errors monitor",
			disk: models.PhysicalDisk{
				Health: "PASSED",
				SmartAttributes: &models.SMARTAttributes{
					UDMACRCErrors: int64Ptr(10),
				},
			},
			wantLevel:   RiskMonitor,
			wantReasons: []string{"crc_errors"},
		},
		{
			// Samsung 980 firmware-bug model suppresses FAILED health_status.
			name:      "samsung 980 firmware bug suppresses failed health",
			disk:      models.PhysicalDisk{Model: "Samsung SSD 980 Pro", Health: "FAILED"},
			wantLevel: RiskHealthy,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AssessPhysicalDisk(tt.disk)
			if got.Level != tt.wantLevel {
				t.Fatalf("AssessPhysicalDisk level = %q, want %q; reasons=%+v", got.Level, tt.wantLevel, got.Reasons)
			}
			for _, code := range tt.wantReasons {
				if _, ok := reasonSeverity(got, code); !ok {
					t.Errorf("expected reason %q present, got reasons=%+v", code, got.Reasons)
				}
			}
		})
	}
}

// TestAssessPhysicalDisk_NilSmartAttributesDoesNotPanic pins the nil-SmartAttributes
// branch in isolation: a disk with zero-valued fields must not deref nil pointers.
func TestAssessPhysicalDisk_NilSmartAttributesDoesNotPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("AssessPhysicalDisk panicked on nil SmartAttributes: %v", r)
		}
	}()
	got := AssessPhysicalDisk(models.PhysicalDisk{})
	if got.Level != RiskHealthy {
		t.Fatalf("empty disk level = %q, want %q", got.Level, RiskHealthy)
	}
	if len(got.Reasons) != 0 {
		t.Fatalf("empty disk should produce no reasons, got %+v", got.Reasons)
	}
}

// TestAssessPhysicalDisk_MultipleReasonsTakeHighest drives several conditions
// at once and asserts both the merged level (critical) and that every reason
// is preserved through the SMART-attribute mapping.
func TestAssessPhysicalDisk_MultipleReasonsTakeHighest(t *testing.T) {
	got := AssessPhysicalDisk(models.PhysicalDisk{
		Health:      "PASSED",
		Temperature: 63, // warning
		SmartAttributes: &models.SMARTAttributes{
			PendingSectors: int64Ptr(1), // critical
			UDMACRCErrors:  int64Ptr(2), // monitor
		},
	})
	if got.Level != RiskCritical {
		t.Fatalf("expected critical (highest), got %q; reasons=%+v", got.Level, got.Reasons)
	}
	if len(got.Reasons) != 3 {
		t.Fatalf("expected 3 reasons (temperature_high, pending_sectors, crc_errors), got %d: %+v", len(got.Reasons), got.Reasons)
	}
	for _, code := range []string{"pending_sectors", "temperature_high", "crc_errors"} {
		if _, ok := reasonSeverity(got, code); !ok {
			t.Errorf("expected reason %q present, got %+v", code, got.Reasons)
		}
	}
}

// --- AssessHostSMARTDisk: branch coverage over risk.go:90-131 ---
//
// AssessHostSMARTDisk differs from AssessPhysicalDisk in two ways:
//   - Wearout is initialised to -1 (so the wearout branches only fire when
//     PercentageUsed is supplied and derives Wearout into a positive band).
//   - When PercentageUsed is set, Wearout is computed as 100 - PercentageUsed.

func TestAssessHostSMARTDisk_Branches(t *testing.T) {
	tests := []struct {
		name        string
		disk        models.HostDiskSMART
		wantLevel   RiskLevel
		wantReasons []string
	}{
		{
			// nil Attributes arm: only Model/Health/Temperature are sourced;
			// Wearout stays -1 so the wearout branch is skipped.
			name:      "nil attributes stays healthy",
			disk:      models.HostDiskSMART{Model: "WD Blue", Health: "PASSED", Temperature: 35},
			wantLevel: RiskHealthy,
		},
		{
			// non-nil Attributes arm with benign values: every pointer is read
			// but none escalate. Proves nil-deref safety for the SMART path.
			name: "benign attributes stays healthy",
			disk: models.HostDiskSMART{
				Health: "PASSED", Temperature: 35,
				Attributes: &models.SMARTAttributes{
					PowerOnHours:         int64Ptr(120),
					PowerCycles:          int64Ptr(8),
					ReallocatedSectors:   int64Ptr(0),
					PendingSectors:       int64Ptr(0),
					OfflineUncorrectable: int64Ptr(0),
					UDMACRCErrors:        int64Ptr(0),
					PercentageUsed:       intPtr(20), // Wearout = 80, healthy
					AvailableSpare:       intPtr(99),
					MediaErrors:          int64Ptr(0),
					UnsafeShutdowns:      int64Ptr(0),
				},
			},
			wantLevel: RiskHealthy,
		},
		{
			// FAILED health on a host SMART disk -> critical health_status.
			name:        "failed health critical",
			disk:        models.HostDiskSMART{Model: "WD Blue", Health: "FAILED"},
			wantLevel:   RiskCritical,
			wantReasons: []string{"health_status"},
		},
		{
			// Pending sectors via host SMART attributes -> critical.
			name: "pending sectors critical",
			disk: models.HostDiskSMART{
				Health: "PASSED",
				Attributes: &models.SMARTAttributes{
					PendingSectors: int64Ptr(3),
				},
			},
			wantLevel:   RiskCritical,
			wantReasons: []string{"pending_sectors"},
		},
		{
			// Offline uncorrectable via host SMART attributes -> critical.
			name: "offline uncorrectable critical",
			disk: models.HostDiskSMART{
				Health: "PASSED",
				Attributes: &models.SMARTAttributes{
					OfflineUncorrectable: int64Ptr(1),
				},
			},
			wantLevel:   RiskCritical,
			wantReasons: []string{"offline_uncorrectable"},
		},
		{
			// Media errors via host SMART attributes -> critical.
			name: "media errors critical",
			disk: models.HostDiskSMART{
				Health: "PASSED",
				Attributes: &models.SMARTAttributes{
					MediaErrors: int64Ptr(2),
				},
			},
			wantLevel:   RiskCritical,
			wantReasons: []string{"media_errors"},
		},
		{
			// PercentageUsed >= 95 derives Wearout <= 5: BOTH the percentage-used
			// critical reason AND the wearout_low critical reason should fire.
			name: "percentage used critical derives wearout critical",
			disk: models.HostDiskSMART{
				Health: "PASSED",
				Attributes: &models.SMARTAttributes{
					PercentageUsed: intPtr(97), // Wearout = 3 -> critical
				},
			},
			wantLevel:   RiskCritical,
			wantReasons: []string{"nvme_percentage_used_high", "wearout_low"},
		},
		{
			// PercentageUsed 90..94 derives Wearout 6..9: both reasons at warning.
			name: "percentage used warning derives wearout warning",
			disk: models.HostDiskSMART{
				Health: "PASSED",
				Attributes: &models.SMARTAttributes{
					PercentageUsed: intPtr(92), // Wearout = 8 -> warning
				},
			},
			wantLevel:   RiskWarning,
			wantReasons: []string{"nvme_percentage_used_high", "wearout_low"},
		},
		{
			// Available spare <= 10 via host SMART -> critical.
			name: "nvme available spare critical",
			disk: models.HostDiskSMART{
				Health: "PASSED",
				Attributes: &models.SMARTAttributes{
					AvailableSpare: intPtr(7),
				},
			},
			wantLevel:   RiskCritical,
			wantReasons: []string{"nvme_available_spare_low"},
		},
		{
			// Available spare 11..19 via host SMART -> warning.
			name: "nvme available spare warning",
			disk: models.HostDiskSMART{
				Health: "PASSED",
				Attributes: &models.SMARTAttributes{
					AvailableSpare: intPtr(15),
				},
			},
			wantLevel:   RiskWarning,
			wantReasons: []string{"nvme_available_spare_low"},
		},
		{
			// Temperature >= 70 -> critical.
			name:        "temperature critical",
			disk:        models.HostDiskSMART{Health: "PASSED", Temperature: 75},
			wantLevel:   RiskCritical,
			wantReasons: []string{"temperature_high"},
		},
		{
			// Temperature 60..69 -> warning.
			name:        "temperature warning",
			disk:        models.HostDiskSMART{Health: "PASSED", Temperature: 60},
			wantLevel:   RiskWarning,
			wantReasons: []string{"temperature_high"},
		},
		{
			// Reallocated sectors > 0 via host SMART -> warning.
			name: "reallocated sectors warning",
			disk: models.HostDiskSMART{
				Health: "PASSED",
				Attributes: &models.SMARTAttributes{
					ReallocatedSectors: int64Ptr(6),
				},
			},
			wantLevel:   RiskWarning,
			wantReasons: []string{"reallocated_sectors"},
		},
		{
			// UDMA CRC errors > 0 via host SMART -> monitor.
			name: "udma crc errors monitor",
			disk: models.HostDiskSMART{
				Health: "PASSED",
				Attributes: &models.SMARTAttributes{
					UDMACRCErrors: int64Ptr(11),
				},
			},
			wantLevel:   RiskMonitor,
			wantReasons: []string{"crc_errors"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AssessHostSMARTDisk(tt.disk)
			if got.Level != tt.wantLevel {
				t.Fatalf("AssessHostSMARTDisk level = %q, want %q; reasons=%+v", got.Level, tt.wantLevel, got.Reasons)
			}
			for _, code := range tt.wantReasons {
				if _, ok := reasonSeverity(got, code); !ok {
					t.Errorf("expected reason %q present, got %+v", code, got.Reasons)
				}
			}
		})
	}
}

// TestAssessHostSMARTDisk_NilAttributesDoesNotPanic pins the nil-Attributes
// branch in isolation; with default Wearout=-1, no wearout reason fires.
func TestAssessHostSMARTDisk_NilAttributesDoesNotPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("AssessHostSMARTDisk panicked on nil Attributes: %v", r)
		}
	}()
	got := AssessHostSMARTDisk(models.HostDiskSMART{})
	if got.Level != RiskHealthy {
		t.Fatalf("empty disk level = %q, want %q", got.Level, RiskHealthy)
	}
	if len(got.Reasons) != 0 {
		t.Fatalf("empty disk should produce no reasons, got %+v", got.Reasons)
	}
	// Sanity: when Wearout is computed from a default -1 (no PercentageUsed),
	// the wearout_low branch must not fire even with attributes present.
	got2 := AssessHostSMARTDisk(models.HostDiskSMART{
		Health:     "PASSED",
		Attributes: &models.SMARTAttributes{PowerOnHours: int64Ptr(100)},
	})
	if _, ok := reasonSeverity(got2, "wearout_low"); ok {
		t.Fatalf("wearout_low must not fire when Wearout is negative")
	}
}

// --- SummarizeAssessments: branch coverage over topology.go:335-345 ---

func TestSummarizeAssessments(t *testing.T) {
	t.Run("empty returns healthy with no reasons", func(t *testing.T) {
		got := SummarizeAssessments()
		if got.Level != RiskHealthy {
			t.Fatalf("level = %q, want %q", got.Level, RiskHealthy)
		}
		if len(got.Reasons) != 0 {
			t.Fatalf("expected no reasons, got %+v", got.Reasons)
		}
	})

	t.Run("single assessment passes through level", func(t *testing.T) {
		in := Assessment{
			Level: RiskWarning,
			Reasons: []Reason{
				{Code: "a", Severity: RiskWarning, Summary: "a summary"},
			},
		}
		got := SummarizeAssessments(in)
		if got.Level != RiskWarning {
			t.Fatalf("level = %q, want %q", got.Level, RiskWarning)
		}
		if len(got.Reasons) != 1 || got.Reasons[0].Code != "a" {
			t.Fatalf("reasons not preserved, got %+v", got.Reasons)
		}
	})

	t.Run("multiple differing severities picks highest", func(t *testing.T) {
		healthy := Assessment{Level: RiskHealthy, Reasons: []Reason{{Code: "h", Severity: RiskHealthy, Summary: "h"}}}
		monitor := Assessment{Level: RiskMonitor, Reasons: []Reason{{Code: "m", Severity: RiskMonitor, Summary: "m"}}}
		warning := Assessment{Level: RiskWarning, Reasons: []Reason{{Code: "w", Severity: RiskWarning, Summary: "w"}}}
		critical := Assessment{Level: RiskCritical, Reasons: []Reason{{Code: "c", Severity: RiskCritical, Summary: "c"}}}

		got := SummarizeAssessments(healthy, monitor, warning, critical)
		if got.Level != RiskCritical {
			t.Fatalf("level = %q, want %q (highest of inputs)", got.Level, RiskCritical)
		}
		// All reasons must be merged through.
		if len(got.Reasons) != 4 {
			t.Fatalf("expected 4 merged reasons, got %d: %+v", len(got.Reasons), got.Reasons)
		}
		gotCodes := map[string]bool{}
		for _, r := range got.Reasons {
			gotCodes[r.Code] = true
		}
		for _, want := range []string{"h", "m", "w", "c"} {
			if !gotCodes[want] {
				t.Errorf("expected merged reason %q present, got %+v", want, got.Reasons)
			}
		}
	})

	t.Run("reasons sorted by severity descending then code", func(t *testing.T) {
		// Two critical-severity reasons with codes "b" and "a" (so secondary
		// sort by code is exercised) plus a warning reason.
		in := Assessment{
			Level: RiskCritical,
			Reasons: []Reason{
				{Code: "b", Severity: RiskCritical, Summary: "b"},
				{Code: "a", Severity: RiskCritical, Summary: "a"},
				{Code: "z", Severity: RiskWarning, Summary: "z"},
			},
		}
		got := SummarizeAssessments(in)
		if len(got.Reasons) != 3 {
			t.Fatalf("expected 3 reasons, got %d: %+v", len(got.Reasons), got.Reasons)
		}
		// Criticals first (severity desc), and within equal severity codes ascending.
		wantOrder := []string{"a", "b", "z"}
		for i, want := range wantOrder {
			if got.Reasons[i].Code != want {
				t.Errorf("reasons[%d].Code = %q, want %q (full: %+v)", i, got.Reasons[i].Code, want, got.Reasons)
			}
		}
	})

	t.Run("healthy inputs dominate only when no higher severity present", func(t *testing.T) {
		// All inputs healthy -> summary healthy.
		got := SummarizeAssessments(Assessment{Level: RiskHealthy}, Assessment{Level: RiskHealthy})
		if got.Level != RiskHealthy {
			t.Fatalf("level = %q, want %q", got.Level, RiskHealthy)
		}
	})
}
