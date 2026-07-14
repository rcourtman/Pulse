package qualification

import (
	"strings"
	"testing"
)

// This file is a white-box table-test suite for the pure validation and
// formatting helpers in contribution.go. Every helper and shared var
// introduced here is prefixed with qualcontrib to avoid collisions with
// sibling test files in package qualification.

// qualcontribDigest is a valid lowercase 64-char hex SHA-256 digest used to
// populate provenance fields in synthetic ContributionRun values.
var qualcontribDigest = strings.Repeat("a", 64)

// qualcontribBadDigest is a non-empty string that does not match the SHA-256
// hex pattern, exercising the digest-format validation arms of Validate().
var qualcontribBadDigest = "not-a-valid-sha256-digest"

// qualcontribValidRun builds a ContributionRun whose fields satisfy every
// Validate() provenance and digest check. The bound parameter controls the
// ChallengeBound flag so callers can build both bound and unbound bundles.
func qualcontribValidRun(bound bool) ContributionRun {
	return ContributionRun{
		ScenarioID:     "watch.scenario-fixture",
		Model:          "provider:model-a",
		ManifestDigest: qualcontribDigest,
		HarnessGitSHA:  "abc1234",
		EvidenceDigest: qualcontribDigest,
		ReportDigest:   qualcontribDigest,
		ChallengeBound: bound,
	}
}

// qualcontribValidBundle returns a ContributionBundle that passes Validate()
// with an unbound community challenge.
func qualcontribValidBundle() ContributionBundle {
	return ContributionBundle{
		SchemaVersion: ContributionSchemaVersion,
		EvidenceClass: "community-candidate",
		NetworkUpload: false,
		Privacy: ContributionPrivacy{
			AllowlistOnly:      true,
			ManualReviewNeeded: true,
			Excluded:           []string{"secrets and credentials", "raw model output"},
		},
		Runs: []ContributionRun{qualcontribValidRun(false)},
		Challenge: ContributionChallenge{
			BoundAtRun:  false,
			Consistent:  true,
			Explanation: "test explanation",
		},
	}
}

func TestValidateContributionChallenge(t *testing.T) {
	cases := []struct {
		name    string
		value   string
		wantErr bool
		errSub  string
	}{
		{"empty string is allowed", "", false, ""},
		{"whitespace only trimmed to empty", "   \t\n", false, ""},
		{"minimum length sixteen chars passes", "0123456789abcdef", false, ""},
		{"maximum length one hundred twenty eight chars passes", strings.Repeat("x", 128), false, ""},
		{"valid with dots underscores hyphens passes", "abc_def-123.4567", false, ""},
		{"too short at fifteen chars fails", "0123456789abcde", true, "community challenge must be 16-128"},
		{"too long at one hundred twenty nine chars fails", strings.Repeat("x", 129), true, "community challenge must be 16-128"},
		{"contains space fails", "challenge contains", true, "community challenge must be 16-128"},
		{"contains invalid exclamation fails", "challenge_value_123!", true, "community challenge must be 16-128"},
		{"contains tab character fails", "challenge\tvalue_12345", true, "community challenge must be 16-128"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateContributionChallenge(tc.value)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("ValidateContributionChallenge(%q) returned nil, want error", tc.value)
				}
				if tc.errSub != "" && !strings.Contains(err.Error(), tc.errSub) {
					t.Fatalf("ValidateContributionChallenge(%q) error = %q, want substring %q", tc.value, err.Error(), tc.errSub)
				}
			} else if err != nil {
				t.Fatalf("ValidateContributionChallenge(%q) returned %v, want nil", tc.value, err)
			}
		})
	}
}

func TestValidateContributionIdentity(t *testing.T) {
	cases := []struct {
		name    string
		label   string
		value   string
		wantErr bool
		errSub  string
	}{
		{"empty value passes", "model", "", false, ""},
		{"plain alphanumeric passes", "provider", "provider-a", false, ""},
		{"value with dots and hyphens passes", "harness revision", "abc1234.def-5", false, ""},
		{"value at exactly five hundred twelve chars passes", "model", strings.Repeat("x", 512), false, ""},
		{"value over five hundred twelve chars fails", "model", strings.Repeat("x", 513), true, "exceeds the community export length limit"},
		{"newline in value fails", "model", "model\nv2", true, "contains a line break"},
		{"carriage return in value fails", "model", "model\rv2", true, "contains a line break"},
		{"redacted marker fails", "provider", "[REDACTED]", true, "was redacted"},
		{"secret shaped password fails", "model", "password=s3cr3t", true, "resembles secret-bearing content"},
		{"secret shaped api key fails", "provider", "api_key=abc123", true, "resembles secret-bearing content"},
		{"bearer token fails", "Pulse version", "Authorization: Bearer xyz123", true, "resembles secret-bearing content"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateContributionIdentity(tc.label, tc.value)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("validateContributionIdentity(%q, %q) returned nil, want error", tc.label, tc.value)
				}
				if tc.errSub != "" && !strings.Contains(err.Error(), tc.errSub) {
					t.Fatalf("validateContributionIdentity(%q, %q) error = %q, want substring %q", tc.label, tc.value, err.Error(), tc.errSub)
				}
			} else if err != nil {
				t.Fatalf("validateContributionIdentity(%q, %q) returned %v, want nil", tc.label, tc.value, err)
			}
		})
	}
}

func TestEveryRunChallengeBound(t *testing.T) {
	cases := []struct {
		name string
		runs []ContributionRun
		want bool
	}{
		{"nil slice returns false", nil, false},
		{"empty slice returns false", []ContributionRun{}, false},
		{"single bound run returns true", []ContributionRun{{ChallengeBound: true}}, true},
		{"multiple all bound returns true", []ContributionRun{{ChallengeBound: true}, {ChallengeBound: true}, {ChallengeBound: true}}, true},
		{"single unbound run returns false", []ContributionRun{{ChallengeBound: false}}, false},
		{"bound then unbound returns false", []ContributionRun{{ChallengeBound: true}, {ChallengeBound: false}}, false},
		{"unbound then bound returns false", []ContributionRun{{ChallengeBound: false}, {ChallengeBound: true}}, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := everyRunChallengeBound(tc.runs); got != tc.want {
				t.Fatalf("everyRunChallengeBound() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestObservationsPassedOrEmpty(t *testing.T) {
	cases := []struct {
		name         string
		observations []PredicateObservation
		want         bool
	}{
		{"nil returns true", nil, true},
		{"empty returns true", []PredicateObservation{}, true},
		{"single passed returns true", []PredicateObservation{{Passed: true}}, true},
		{"multiple all passed returns true", []PredicateObservation{{Passed: true}, {Passed: true}}, true},
		{"single failed returns false", []PredicateObservation{{Passed: false}}, false},
		{"passed then failed returns false", []PredicateObservation{{Passed: true}, {Passed: false}}, false},
		{"failed then passed returns false", []PredicateObservation{{Passed: false}, {Passed: true}}, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := observationsPassedOrEmpty(tc.observations); got != tc.want {
				t.Fatalf("observationsPassedOrEmpty() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestReportPhasePassed(t *testing.T) {
	representative := []PhaseTiming{
		{Name: "preflight", Passed: true},
		{Name: "normal_collection_convergence", Passed: false},
		{Name: "real_model_patrol", Passed: true},
	}

	cases := []struct {
		name   string
		phases []PhaseTiming
		lookup string
		want   bool
	}{
		{"nil phases returns false", nil, "preflight", false},
		{"empty phases returns false", []PhaseTiming{}, "preflight", false},
		{"present and passed returns true", representative, "preflight", true},
		{"present and failed returns false", representative, "normal_collection_convergence", false},
		{"absent name returns false", representative, "does_not_exist", false},
		{"empty lookup on populated phases returns false", representative, "", false},
		{"first occurrence wins when duplicated first passed", []PhaseTiming{{Name: "preflight", Passed: true}, {Name: "preflight", Passed: false}}, "preflight", true},
		{"first occurrence wins when duplicated first failed", []PhaseTiming{{Name: "preflight", Passed: false}, {Name: "preflight", Passed: true}}, "preflight", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := reportPhasePassed(tc.phases, tc.lookup); got != tc.want {
				t.Fatalf("reportPhasePassed(_, %q) = %v, want %v", tc.lookup, got, tc.want)
			}
		})
	}
}

func TestContributionBundleValidate(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(*ContributionBundle)
		wantErr bool
		errSub  string
	}{
		{"valid unbound bundle passes", nil, false, ""},
		{"valid bundle with replay digest passes", func(b *ContributionBundle) {
			b.Runs[0].ReplayDigest = qualcontribDigest
		}, false, ""},
		{"valid bound bundle with consistent nonce passes", func(b *ContributionBundle) {
			b.Challenge.BoundAtRun = true
			b.Challenge.Consistent = true
			b.Challenge.Nonce = "challenge-0123456789ab"
			b.Runs[0].ChallengeBound = true
		}, false, ""},
		{"wrong schema version fails", func(b *ContributionBundle) { b.SchemaVersion = "wrong" }, true, "unsupported contribution schema"},
		{"wrong evidence class fails", func(b *ContributionBundle) { b.EvidenceClass = "wrong" }, true, "unsupported contribution evidence class"},
		{"network upload true fails", func(b *ContributionBundle) { b.NetworkUpload = true }, true, "cannot claim a network upload"},
		{"privacy allowlist false fails", func(b *ContributionBundle) { b.Privacy.AllowlistOnly = false }, true, "privacy boundary is incomplete"},
		{"privacy manual review false fails", func(b *ContributionBundle) { b.Privacy.ManualReviewNeeded = false }, true, "privacy boundary is incomplete"},
		{"privacy no excluded entries fails", func(b *ContributionBundle) { b.Privacy.Excluded = nil }, true, "privacy boundary is incomplete"},
		{"no runs fails", func(b *ContributionBundle) { b.Runs = nil }, true, "contains no runs"},
		{"invalid challenge nonce fails", func(b *ContributionBundle) { b.Challenge.Nonce = "short" }, true, "community challenge must be 16-128"},
		{"bound challenge with empty nonce fails", func(b *ContributionBundle) {
			b.Challenge.BoundAtRun = true
			b.Challenge.Nonce = ""
			b.Runs[0].ChallengeBound = true
		}, true, "bound community challenge must be present"},
		{"bound challenge inconsistent fails", func(b *ContributionBundle) {
			b.Challenge.BoundAtRun = true
			b.Challenge.Consistent = false
			b.Challenge.Nonce = "challenge-0123456789ab"
			b.Runs[0].ChallengeBound = true
		}, true, "bound community challenge must be present"},
		{"run empty scenario id fails", func(b *ContributionBundle) { b.Runs[0].ScenarioID = "" }, true, "incomplete model, scenario, manifest, or harness provenance"},
		{"run empty model fails", func(b *ContributionBundle) { b.Runs[0].Model = "" }, true, "incomplete model, scenario, manifest, or harness provenance"},
		{"run empty manifest digest fails", func(b *ContributionBundle) { b.Runs[0].ManifestDigest = "" }, true, "incomplete model, scenario, manifest, or harness provenance"},
		{"run empty harness git sha fails", func(b *ContributionBundle) { b.Runs[0].HarnessGitSHA = "" }, true, "incomplete model, scenario, manifest, or harness provenance"},
		{"run invalid evidence digest fails", func(b *ContributionBundle) { b.Runs[0].EvidenceDigest = qualcontribBadDigest }, true, "invalid evidence digest"},
		{"run invalid manifest digest fails", func(b *ContributionBundle) { b.Runs[0].ManifestDigest = qualcontribBadDigest }, true, "invalid manifest digest"},
		{"run invalid report digest fails", func(b *ContributionBundle) { b.Runs[0].ReportDigest = qualcontribBadDigest }, true, "invalid report digest"},
		{"run invalid replay digest fails", func(b *ContributionBundle) { b.Runs[0].ReplayDigest = qualcontribBadDigest }, true, "invalid replay digest"},
		{"binding mismatch bundle unbound run bound fails", func(b *ContributionBundle) { b.Runs[0].ChallengeBound = true }, true, "challenge binding disagrees with bundle"},
		{"binding mismatch bundle bound run unbound fails", func(b *ContributionBundle) {
			b.Challenge.BoundAtRun = true
			b.Challenge.Consistent = true
			b.Challenge.Nonce = "challenge-0123456789ab"
		}, true, "challenge binding disagrees with bundle"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bundle := qualcontribValidBundle()
			if tc.mutate != nil {
				tc.mutate(&bundle)
			}
			err := bundle.Validate()
			if tc.wantErr {
				if err == nil {
					t.Fatalf("Validate() returned nil, want error")
				}
				if tc.errSub != "" && !strings.Contains(err.Error(), tc.errSub) {
					t.Fatalf("Validate() error = %q, want substring %q", err.Error(), tc.errSub)
				}
			} else if err != nil {
				t.Fatalf("Validate() returned %v, want nil", err)
			}
		})
	}
}

func TestRenderContributionReadme(t *testing.T) {
	t.Run("representative bundle renders all required sections", func(t *testing.T) {
		bundle := ContributionBundle{
			Runs: []ContributionRun{{}, {}, {}},
			Challenge: ContributionChallenge{
				BoundAtRun: true,
				Consistent: true,
			},
			Privacy: ContributionPrivacy{
				Excluded: []string{"secrets and credentials", "raw model output", "infrastructure endpoints"},
			},
		}
		got := renderContributionReadme(bundle)

		required := []string{
			"# Pulse Patrol community evidence candidate",
			"No network upload was performed",
			"not Pulse certification",
			"Runs: 3",
			"Challenge bound at run time: true",
			"Challenge consistent across runs: true",
			"Export policy: allowlisted aggregate and provenance fields only",
			"Excluded by construction:",
			"- secrets and credentials",
			"- raw model output",
			"- infrastructure endpoints",
		}
		for _, sub := range required {
			if !strings.Contains(got, sub) {
				t.Fatalf("renderContributionReadme missing %q\nGot:\n%s", sub, got)
			}
		}
	})

	t.Run("unbound challenge renders false flags", func(t *testing.T) {
		bundle := ContributionBundle{
			Runs: []ContributionRun{{}},
			Challenge: ContributionChallenge{
				BoundAtRun: false,
				Consistent: false,
			},
			Privacy: ContributionPrivacy{
				Excluded: []string{"nothing-sensitive"},
			},
		}
		got := renderContributionReadme(bundle)
		for _, sub := range []string{
			"Runs: 1",
			"Challenge bound at run time: false",
			"Challenge consistent across runs: false",
			"- nothing-sensitive",
		} {
			if !strings.Contains(got, sub) {
				t.Fatalf("renderContributionReadme missing %q\nGot:\n%s", sub, got)
			}
		}
	})

	t.Run("empty excluded list renders header without item entries", func(t *testing.T) {
		bundle := ContributionBundle{
			Runs:    []ContributionRun{{}},
			Privacy: ContributionPrivacy{},
		}
		got := renderContributionReadme(bundle)
		idx := strings.Index(got, "Excluded by construction:")
		if idx < 0 {
			t.Fatalf("renderContributionReadme missing excluded header\nGot:\n%s", got)
		}
		tail := got[idx:]
		if strings.Contains(tail, "- ") {
			t.Fatalf("expected no excluded item lines after header, got tail:\n%s", tail)
		}
	})
}
