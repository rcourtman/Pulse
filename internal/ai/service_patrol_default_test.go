package ai

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// TestEffectivePatrolAutonomyDefaultsToAskFirstOnlyWhenLicensed pins the Pro
// default: an install that never chose a Patrol mode runs Ask first only on a
// positive investigation licence answer. A missing checker (licence not yet
// known) and an unlicensed install stay in Watch only, and an explicit choice
// always wins.
func TestEffectivePatrolAutonomyDefaultsToAskFirstOnlyWhenLicensed(t *testing.T) {
	cases := []struct {
		name    string
		checker LicenseChecker
		stored  string
		want    string
	}{
		{name: "licensed, never chose", checker: &mockLicenseChecker{hasFeature: true}, stored: "", want: config.PatrolAutonomyApproval},
		{name: "licensed, chose watch only", checker: &mockLicenseChecker{hasFeature: true}, stored: config.PatrolAutonomyMonitor, want: config.PatrolAutonomyMonitor},
		{name: "licensed, chose ask first", checker: &mockLicenseChecker{hasFeature: true}, stored: config.PatrolAutonomyApproval, want: config.PatrolAutonomyApproval},
		{name: "unlicensed, never chose", checker: &mockLicenseChecker{hasFeature: false}, stored: "", want: config.PatrolAutonomyMonitor},
		{name: "unlicensed, chose ask first", checker: &mockLicenseChecker{hasFeature: false}, stored: config.PatrolAutonomyApproval, want: config.PatrolAutonomyMonitor},
		{name: "licence not yet known, never chose", checker: nil, stored: "", want: config.PatrolAutonomyMonitor},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := NewService(nil, nil)
			svc.cfg = &config.AIConfig{PatrolAutonomyLevel: tc.stored}
			if tc.checker != nil {
				svc.SetLicenseChecker(tc.checker)
			}
			if got := svc.GetEffectivePatrolAutonomyLevel(); got != tc.want {
				t.Fatalf("GetEffectivePatrolAutonomyLevel() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestPatrolInvestigationLicensedTreatsMissingCheckerAsUnlicensed(t *testing.T) {
	svc := NewService(nil, nil)
	if svc.PatrolInvestigationLicensed() {
		t.Fatal("a missing licence checker must not count as a positive licence")
	}
	svc.SetLicenseChecker(&mockLicenseChecker{hasFeature: true})
	if !svc.PatrolInvestigationLicensed() {
		t.Fatal("a licensed checker must count as a positive licence")
	}
}
