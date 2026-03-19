package ai

import (
	"testing"
	"time"

	unifiedresources "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestRecordUnifiedResourceExport_UsesCanonicalPrivacyHelpers(t *testing.T) {
	cases := []struct {
		name           string
		counts         map[unifiedresources.ResourceSensitivity]int
		localOnlyCount int
		redactionHints []unifiedresources.ResourceRedactionHint
		wantDecision   unifiedresources.ExportDecision
		wantReason     string
		wantRedactions []string
	}{
		{
			name:           "public",
			counts:         map[unifiedresources.ResourceSensitivity]int{unifiedresources.ResourceSensitivityPublic: 1},
			wantDecision:   unifiedresources.ExportAllowed,
			wantReason:     "public unified resource context",
			wantRedactions: nil,
		},
		{
			name:           "internal",
			counts:         map[unifiedresources.ResourceSensitivity]int{unifiedresources.ResourceSensitivityInternal: 1},
			wantDecision:   unifiedresources.ExportRequiresConsent,
			wantReason:     "internal unified resource context requires export consent",
			wantRedactions: nil,
		},
		{
			name:           "redacted",
			counts:         map[unifiedresources.ResourceSensitivity]int{unifiedresources.ResourceSensitivitySensitive: 1},
			localOnlyCount: 1,
			redactionHints: []unifiedresources.ResourceRedactionHint{
				unifiedresources.ResourceRedactionHostname,
				unifiedresources.ResourceRedactionHostname,
				unifiedresources.ResourceRedactionPath,
			},
			wantDecision:   unifiedresources.ExportRedacted,
			wantReason:     "governed unified resource context exported in redacted form",
			wantRedactions: []string{"hostname", "path"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := unifiedresources.NewMemoryStore()
			svc := &Service{
				orgID:                    "org-1",
				resourceExportStore:      store,
				resourceExportStoreOrgID: "org-1",
			}

			svc.recordUnifiedResourceExport(
				"anthropic:test-model",
				"canonical summary",
				unifiedresources.ResourceStats{
					Total: 1,
					ByType: map[unifiedresources.ResourceType]int{
						unifiedresources.ResourceTypeAgent: 1,
					},
				},
				tc.counts,
				tc.localOnlyCount,
				tc.redactionHints,
			)

			audits, err := store.GetExportAudits(time.Time{}, 10)
			if err != nil {
				t.Fatalf("GetExportAudits: %v", err)
			}
			if len(audits) != 1 {
				t.Fatalf("expected 1 export audit, got %d", len(audits))
			}
			if audits[0].Decision != tc.wantDecision {
				t.Fatalf("decision = %q, want %q", audits[0].Decision, tc.wantDecision)
			}
			if audits[0].Destination != "anthropic:test-model" {
				t.Fatalf("destination = %q, want anthropic:test-model", audits[0].Destination)
			}
			if audits[0].EnvelopeHash == "" {
				t.Fatal("expected envelope hash to be recorded")
			}
			if len(audits[0].Redactions) != len(tc.wantRedactions) {
				t.Fatalf("redactions = %#v, want %#v", audits[0].Redactions, tc.wantRedactions)
			}
			for i := range tc.wantRedactions {
				if audits[0].Redactions[i] != tc.wantRedactions[i] {
					t.Fatalf("redactions = %#v, want %#v", audits[0].Redactions, tc.wantRedactions)
				}
			}

			decision, reason := unifiedresources.ExportDecisionForContext(
				unifiedresources.ExportSensitivityFloor(tc.counts),
				tc.localOnlyCount,
				len(tc.wantRedactions),
			)
			if decision != tc.wantDecision {
				t.Fatalf("shared decision = %q, want %q", decision, tc.wantDecision)
			}
			if reason != tc.wantReason {
				t.Fatalf("shared reason = %q, want %q", reason, tc.wantReason)
			}
		})
	}
}
