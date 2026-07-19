package model

import (
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/operationaltrust"
)

// validProviderObservation builds a ProtectionProviderObservation that passes
// Validate(). Fields are concrete and deterministic so individual error
// branches can be exercised by mutating one field at a time.
func validProviderObservation(t *testing.T) ProtectionProviderObservation {
	t.Helper()
	observedAt := time.Date(2026, 7, 18, 20, 0, 0, 0, time.UTC)
	ingestedAt := observedAt.Add(2 * time.Second)
	source := operationaltrust.EvidenceSource{Provider: "proxmox", Collector: "pulse-core"}
	subject := operationaltrust.EvidenceSubject{ResourceID: "resource-123"}
	id, err := operationaltrust.NewEvidenceID(source, subject, observedAt, "provider-event-7")
	if err != nil {
		t.Fatalf("NewEvidenceID() error = %v", err)
	}
	envelope := operationaltrust.EvidenceEnvelope{
		ID:           id,
		Source:       source,
		Subject:      subject,
		ObservedAt:   observedAt,
		IngestedAt:   ingestedAt,
		Completeness: operationaltrust.EvidenceComplete,
		Confidence:   operationaltrust.EvidenceConfirmed,
		Permissions:  operationaltrust.EvidencePermissionsSufficient,
	}
	return ProtectionProviderObservation{
		ID:                  id,
		Provider:            ProviderProxmoxPVE,
		Source:              "pulse-core",
		Scope:               "pve-a",
		JobState:            OutcomeSuccess,
		HistoryCompleteness: ProtectionHistoryComplete,
		Permissions:         operationaltrust.EvidencePermissionsSufficient,
		ObservedAt:          observedAt,
		IngestedAt:          ingestedAt,
		Evidence:            envelope,
	}
}

func TestProtectionProviderObservationCloneIsDeep(t *testing.T) {
	base := validProviderObservation(t)
	// Equip the evidence envelope with mutable pointer/map state so we can
	// observe whether Clone() shares it with the original.
	validUntil := base.ObservedAt.Add(time.Hour)
	base.Evidence.ValidUntil = &validUntil
	base.Evidence.Reason = &operationaltrust.EvidenceReason{Code: "inferred"}
	base.Evidence.Correlation = &operationaltrust.IdentityCorrelation{
		Rule:           "normalized_hostname",
		MatchedFields:  map[string]string{"hostname": "db-01"},
		CandidateCount: 1,
	}

	clone := base.Clone()

	// Mutate every mutable subfield reachable through the clone's evidence.
	*clone.Evidence.ValidUntil = validUntil.Add(time.Hour)
	clone.Evidence.Reason.Code = "mutated"
	clone.Evidence.Correlation.MatchedFields["hostname"] = "db-99"

	if !base.Evidence.ValidUntil.Equal(validUntil) {
		t.Fatalf("Clone() shared ValidUntil pointer: base = %v, want %v",
			*base.Evidence.ValidUntil, validUntil)
	}
	if base.Evidence.Reason == nil || base.Evidence.Reason.Code != "inferred" {
		v := "<nil>"
		if base.Evidence.Reason != nil {
			v = base.Evidence.Reason.Code
		}
		t.Fatalf("Clone() shared Reason pointer: base.Code = %q, want %q", v, "inferred")
	}
	if base.Evidence.Correlation.MatchedFields["hostname"] != "db-01" {
		t.Fatalf("Clone() shared Correlation map: base = %q, want %q",
			base.Evidence.Correlation.MatchedFields["hostname"], "db-01")
	}

	// Sanity: the clone must reflect the mutations we applied to it.
	if !clone.Evidence.ValidUntil.Equal(validUntil.Add(time.Hour)) {
		t.Fatalf("clone did not record ValidUntil mutation: = %v", *clone.Evidence.ValidUntil)
	}
	if clone.Evidence.Reason.Code != "mutated" {
		t.Fatalf("clone did not record Reason mutation: = %q", clone.Evidence.Reason.Code)
	}
	if clone.Evidence.Correlation.MatchedFields["hostname"] != "db-99" {
		t.Fatalf("clone did not record Correlation mutation: = %q",
			clone.Evidence.Correlation.MatchedFields["hostname"])
	}

	// Scalar fields copied through.
	if clone.ID != base.ID || clone.Provider != base.Provider || clone.Scope != base.Scope {
		t.Fatalf("Clone() dropped scalar fields: %+v", clone)
	}
}

func TestProtectionProviderObservationValidate(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		obs := validProviderObservation(t)
		if err := obs.Validate(); err != nil {
			t.Fatalf("Validate() error = %v, want nil", err)
		}
	})

	cases := []struct {
		name      string
		mutate    func(*ProtectionProviderObservation)
		errSubstr string
	}{
		{
			name:      "empty id",
			mutate:    func(o *ProtectionProviderObservation) { o.ID = "  " },
			errSubstr: "observation id is required",
		},
		{
			name:      "empty provider",
			mutate:    func(o *ProtectionProviderObservation) { o.Provider = "  " },
			errSubstr: "observation provider is required",
		},
		{
			name:      "empty source",
			mutate:    func(o *ProtectionProviderObservation) { o.Source = "" },
			errSubstr: "observation source is required",
		},
		{
			name:      "empty scope",
			mutate:    func(o *ProtectionProviderObservation) { o.Scope = " " },
			errSubstr: "observation scope is required",
		},
		{
			name:      "invalid job state",
			mutate:    func(o *ProtectionProviderObservation) { o.JobState = Outcome("bogus") },
			errSubstr: "job state",
		},
		{
			name: "invalid history completeness",
			mutate: func(o *ProtectionProviderObservation) {
				o.HistoryCompleteness = ProtectionHistoryCompleteness("nope")
			},
			errSubstr: "history completeness",
		},
		{
			name: "invalid permissions",
			mutate: func(o *ProtectionProviderObservation) {
				o.Permissions = operationaltrust.EvidencePermissions("nope")
			},
			errSubstr: "permissions",
		},
		{
			name:      "zero observedAt",
			mutate:    func(o *ProtectionProviderObservation) { o.ObservedAt = time.Time{} },
			errSubstr: "times are required",
		},
		{
			name:      "zero ingestedAt",
			mutate:    func(o *ProtectionProviderObservation) { o.IngestedAt = time.Time{} },
			errSubstr: "times are required",
		},
		{
			name: "evidence observedAt mismatch",
			mutate: func(o *ProtectionProviderObservation) {
				o.ObservedAt = o.ObservedAt.Add(time.Minute)
			},
			errSubstr: "observation times must match",
		},
		{
			name: "evidence ingestedAt mismatch",
			mutate: func(o *ProtectionProviderObservation) {
				o.IngestedAt = o.IngestedAt.Add(time.Minute)
			},
			errSubstr: "ingestion times must match",
		},
		{
			name:      "evidence id mismatch",
			mutate:    func(o *ProtectionProviderObservation) { o.ID += "-suffix" },
			errSubstr: "id must match its evidence id",
		},
		{
			name: "evidence validate fails",
			mutate: func(o *ProtectionProviderObservation) {
				o.Evidence.Completeness = operationaltrust.EvidenceCompleteness("bogus")
			},
			errSubstr: "provider observation evidence:",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			obs := validProviderObservation(t)
			tc.mutate(&obs)
			err := obs.Validate()
			if err == nil {
				t.Fatalf("Validate() error = nil, want error containing %q", tc.errSubstr)
			}
			if !strings.Contains(err.Error(), tc.errSubstr) {
				t.Fatalf("Validate() error = %q, want substring %q", err.Error(), tc.errSubstr)
			}
		})
	}
}

func TestProtectionPosturePolicyValidate(t *testing.T) {
	cases := []struct {
		name    string
		policy  ProtectionPosturePolicy
		wantErr bool
		substr  string
	}{
		{
			name:    "valid with requireVerification",
			policy:  ProtectionPosturePolicy{FreshnessWindow: 5 * time.Minute, VerificationWindow: time.Hour, RequireVerification: true},
			wantErr: false,
		},
		{
			name:    "freshness zero",
			policy:  ProtectionPosturePolicy{FreshnessWindow: 0, VerificationWindow: time.Hour},
			wantErr: true,
			substr:  "freshness window must be positive",
		},
		{
			name:    "freshness negative",
			policy:  ProtectionPosturePolicy{FreshnessWindow: -time.Second, VerificationWindow: time.Hour},
			wantErr: true,
			substr:  "freshness window must be positive",
		},
		{
			name:    "verification zero",
			policy:  ProtectionPosturePolicy{FreshnessWindow: time.Minute, VerificationWindow: 0},
			wantErr: true,
			substr:  "verification window must be positive",
		},
		{
			name:    "verification negative",
			policy:  ProtectionPosturePolicy{FreshnessWindow: time.Minute, VerificationWindow: -time.Second},
			wantErr: true,
			substr:  "verification window must be positive",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.policy.Validate()
			if tc.wantErr {
				if err == nil {
					t.Fatalf("Validate() error = nil, want error containing %q", tc.substr)
				}
				if !strings.Contains(err.Error(), tc.substr) {
					t.Fatalf("Validate() error = %q, want substring %q", err.Error(), tc.substr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Validate() error = %v, want nil", err)
			}
		})
	}
}

func TestProtectionPosturePolicyPayload(t *testing.T) {
	t.Run("maps whole-second durations and flag", func(t *testing.T) {
		policy := ProtectionPosturePolicy{
			FreshnessWindow:     90 * time.Second,
			VerificationWindow:  3600 * time.Second,
			RequireVerification: true,
		}
		payload := policy.Payload()
		if payload.FreshnessWindowSeconds != 90 {
			t.Fatalf("FreshnessWindowSeconds = %d, want 90", payload.FreshnessWindowSeconds)
		}
		if payload.VerificationWindowSeconds != 3600 {
			t.Fatalf("VerificationWindowSeconds = %d, want 3600", payload.VerificationWindowSeconds)
		}
		if payload.RequireVerification != true {
			t.Fatalf("RequireVerification = %v, want true", payload.RequireVerification)
		}
	})

	t.Run("truncates sub-second durations toward zero", func(t *testing.T) {
		// Payload uses int64(d / time.Second), which truncates fractional
		// seconds toward zero for positive durations.
		policy := ProtectionPosturePolicy{
			FreshnessWindow:     1500 * time.Millisecond,
			VerificationWindow:  2*time.Second + 700*time.Millisecond,
			RequireVerification: false,
		}
		payload := policy.Payload()
		if payload.FreshnessWindowSeconds != 1 {
			t.Fatalf("FreshnessWindowSeconds = %d, want 1 (truncated)", payload.FreshnessWindowSeconds)
		}
		if payload.VerificationWindowSeconds != 2 {
			t.Fatalf("VerificationWindowSeconds = %d, want 2 (truncated)", payload.VerificationWindowSeconds)
		}
		if payload.RequireVerification != false {
			t.Fatalf("RequireVerification = %v, want false", payload.RequireVerification)
		}
	})
}

func TestProtectionProviderSummaryNormalize(t *testing.T) {
	t.Run("trims whitespace and preserves valid enums and pass-through fields", func(t *testing.T) {
		attemptAt := time.Date(2026, 7, 18, 20, 0, 0, 0, time.UTC)
		successAt := attemptAt.Add(time.Hour)
		summary := ProtectionProviderSummary{
			Provider:              "  proxmox-pve  ",
			Source:                "  pulse-core\n",
			Scope:                 "\tpve-a ",
			JobState:              OutcomeWarning,
			HistoryCompleteness:   ProtectionHistoryPartial,
			Permissions:           operationaltrust.EvidencePermissionsPartial,
			RepositoryResourceIDs: []string{"repo-b", "  ", "repo-a", "repo-a"},
			EvidenceIDs:           []string{"e2", "e1", "e1", ""},
			LastAttemptAt:         &attemptAt,
			LastSuccessAt:         &successAt,
			BackupPointCount:      7,
			SnapshotPointCount:    3,
			VerificationExpected:  true,
		}
		got := summary.normalize()
		if got.Provider != ProviderProxmoxPVE {
			t.Fatalf("Provider = %q, want %q", got.Provider, ProviderProxmoxPVE)
		}
		if got.Source != "pulse-core" {
			t.Fatalf("Source = %q, want %q", got.Source, "pulse-core")
		}
		if got.Scope != "pve-a" {
			t.Fatalf("Scope = %q, want %q", got.Scope, "pve-a")
		}
		if got.JobState != OutcomeWarning {
			t.Fatalf("JobState = %q, want %q (valid value must be preserved)",
				got.JobState, OutcomeWarning)
		}
		if got.HistoryCompleteness != ProtectionHistoryPartial {
			t.Fatalf("HistoryCompleteness = %q, want %q",
				got.HistoryCompleteness, ProtectionHistoryPartial)
		}
		if got.Permissions != operationaltrust.EvidencePermissionsPartial {
			t.Fatalf("Permissions = %q, want %q",
				got.Permissions, operationaltrust.EvidencePermissionsPartial)
		}
		wantRepos := []string{"repo-a", "repo-b"}
		if len(got.RepositoryResourceIDs) != len(wantRepos) {
			t.Fatalf("RepositoryResourceIDs = %v, want %v", got.RepositoryResourceIDs, wantRepos)
		}
		for i := range wantRepos {
			if got.RepositoryResourceIDs[i] != wantRepos[i] {
				t.Fatalf("RepositoryResourceIDs = %v, want %v", got.RepositoryResourceIDs, wantRepos)
			}
		}
		wantEvidence := []string{"e1", "e2"}
		if len(got.EvidenceIDs) != len(wantEvidence) {
			t.Fatalf("EvidenceIDs = %v, want %v", got.EvidenceIDs, wantEvidence)
		}
		for i := range wantEvidence {
			if got.EvidenceIDs[i] != wantEvidence[i] {
				t.Fatalf("EvidenceIDs = %v, want %v", got.EvidenceIDs, wantEvidence)
			}
		}
		if got.BackupPointCount != 7 || got.SnapshotPointCount != 3 || !got.VerificationExpected {
			t.Fatalf("normalize dropped pass-through fields: %+v", got)
		}
	})

	t.Run("replaces invalid enums with unknown sentinels", func(t *testing.T) {
		summary := ProtectionProviderSummary{
			Provider:            ProviderProxmoxPBS,
			Source:              "src",
			Scope:               "scope",
			JobState:            Outcome("bogus"),
			HistoryCompleteness: ProtectionHistoryCompleteness("nope"),
			Permissions:         operationaltrust.EvidencePermissions("nope"),
		}
		got := summary.normalize()
		if got.JobState != OutcomeUnknown {
			t.Fatalf("JobState = %q, want %q", got.JobState, OutcomeUnknown)
		}
		if got.HistoryCompleteness != ProtectionHistoryUnknown {
			t.Fatalf("HistoryCompleteness = %q, want %q",
				got.HistoryCompleteness, ProtectionHistoryUnknown)
		}
		if got.Permissions != operationaltrust.EvidencePermissionsUnknown {
			t.Fatalf("Permissions = %q, want %q",
				got.Permissions, operationaltrust.EvidencePermissionsUnknown)
		}
	})

	t.Run("nil slice fields become empty non-nil slices", func(t *testing.T) {
		summary := ProtectionProviderSummary{
			Provider: ProviderProxmoxPVE,
			Source:   "src",
			Scope:    "scope",
		}
		got := summary.normalize()
		if got.RepositoryResourceIDs == nil {
			t.Fatal("RepositoryResourceIDs = nil, want non-nil empty slice")
		}
		if len(got.RepositoryResourceIDs) != 0 {
			t.Fatalf("len(RepositoryResourceIDs) = %d, want 0", len(got.RepositoryResourceIDs))
		}
		if got.EvidenceIDs == nil {
			t.Fatal("EvidenceIDs = nil, want non-nil empty slice")
		}
		if len(got.EvidenceIDs) != 0 {
			t.Fatalf("len(EvidenceIDs) = %d, want 0", len(got.EvidenceIDs))
		}
	})
}

func TestCloneTime(t *testing.T) {
	t.Run("nil returns nil", func(t *testing.T) {
		if got := cloneTime(nil); got != nil {
			t.Fatalf("cloneTime(nil) = %v, want nil", *got)
		}
	})

	t.Run("non-nil returns distinct pointer with equal UTC value", func(t *testing.T) {
		// Construct a time in a non-UTC fixed zone so we can also confirm the
		// UTC normalisation performed by cloneTime (uses a fixed offset to
		// avoid depending on system tzdata availability).
		loc := time.FixedZone("EDT", -4*3600)
		original := time.Date(2026, 7, 18, 16, 0, 0, 0, loc) // 16:00 EDT == 20:00 UTC
		cloned := cloneTime(&original)

		if cloned == nil {
			t.Fatal("cloneTime(non-nil) = nil, want non-nil")
		}
		if cloned == &original {
			t.Fatal("cloneTime returned the same pointer as input, want a distinct pointer")
		}
		if !cloned.Equal(original) {
			t.Fatalf("cloned time = %v, want Equal to original %v", *cloned, original)
		}
		if cloned.Location() != time.UTC {
			t.Fatalf("cloned location = %v, want UTC", cloned.Location())
		}
		// Mutating the clone must not move the original instant.
		*cloned = cloned.Add(time.Hour)
		if !original.Equal(time.Date(2026, 7, 18, 16, 0, 0, 0, loc)) {
			t.Fatalf("original was mutated through clone: = %v", original)
		}
	})
}

func TestValidOutcome(t *testing.T) {
	for _, tc := range []Outcome{
		OutcomeSuccess,
		OutcomeWarning,
		OutcomeFailed,
		OutcomeRunning,
		OutcomeUnknown,
	} {
		if !validOutcome(tc) {
			t.Errorf("validOutcome(%q) = false, want true", string(tc))
		}
	}
	for _, tc := range []Outcome{"", "success ", "SUCCESS", "pending", "ok"} {
		if validOutcome(tc) {
			t.Errorf("validOutcome(%q) = true, want false", string(tc))
		}
	}
}

func TestValidEvidencePermissions(t *testing.T) {
	for _, tc := range []operationaltrust.EvidencePermissions{
		operationaltrust.EvidencePermissionsSufficient,
		operationaltrust.EvidencePermissionsPartial,
		operationaltrust.EvidencePermissionsDenied,
		operationaltrust.EvidencePermissionsUnknown,
	} {
		if !validEvidencePermissions(tc) {
			t.Errorf("validEvidencePermissions(%q) = false, want true", string(tc))
		}
	}
	for _, tc := range []operationaltrust.EvidencePermissions{"", "FULL", "sufficient "} {
		if validEvidencePermissions(tc) {
			t.Errorf("validEvidencePermissions(%q) = true, want false", string(tc))
		}
	}
}

func TestNormalizeSortedStrings(t *testing.T) {
	cases := []struct {
		name string
		in   []string
		want []string
	}{
		{name: "nil", in: nil, want: []string{}},
		{name: "empty", in: []string{}, want: []string{}},
		{name: "all blanks dropped", in: []string{" ", "", "\t"}, want: []string{}},
		{name: "trims surrounding whitespace", in: []string{" b ", "a"}, want: []string{"a", "b"}},
		{name: "sorts and dedups", in: []string{"c", "a", "b", "a", "c"}, want: []string{"a", "b", "c"}},
		{name: "already clean stays clean", in: []string{"a", "b", "c"}, want: []string{"a", "b", "c"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeSortedStrings(tc.in)
			if got == nil {
				t.Fatalf("normalizeSortedStrings(%v) returned nil, want non-nil slice", tc.in)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("normalizeSortedStrings(%v) = %v, want %v", tc.in, got, tc.want)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Fatalf("normalizeSortedStrings(%v) = %v, want %v", tc.in, got, tc.want)
				}
			}
		})
	}
}

func TestSortedUniqueStrings(t *testing.T) {
	cases := []struct {
		name string
		in   []string
		want bool
	}{
		{name: "nil is vacuously true", in: nil, want: true},
		{name: "empty is vacuously true", in: []string{}, want: true},
		{name: "single element", in: []string{"a"}, want: true},
		{name: "sorted unique", in: []string{"a", "b", "c"}, want: true},
		{name: "whitespace-only rejected", in: []string{"a", " "}, want: false},
		{name: "empty string rejected", in: []string{"a", ""}, want: false},
		{name: "duplicate rejected", in: []string{"a", "a"}, want: false},
		{name: "unsorted rejected", in: []string{"b", "a"}, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := sortedUniqueStrings(tc.in); got != tc.want {
				t.Fatalf("sortedUniqueStrings(%v) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestCompareProviderStates(t *testing.T) {
	mk := func(provider Provider, scope, source string) ProtectionProviderState {
		return ProtectionProviderState{Provider: provider, Scope: scope, Source: source}
	}
	providerPVE := mk(ProviderProxmoxPVE, "scope-1", "src-1")
	providerPBS := mk(ProviderProxmoxPBS, "scope-1", "src-1")
	sameAsPVE := mk(ProviderProxmoxPVE, "scope-1", "src-1")
	higherScope := mk(ProviderProxmoxPVE, "scope-2", "src-1")
	higherSource := mk(ProviderProxmoxPVE, "scope-1", "src-2")

	t.Run("equal returns zero", func(t *testing.T) {
		if got := compareProviderStates(providerPVE, sameAsPVE); got != 0 {
			t.Fatalf("compareProviderStates(equal) = %d, want 0", got)
		}
	})
	t.Run("negative when provider ranks lower", func(t *testing.T) {
		// "proxmox-pbs" < "proxmox-pve"
		if got := compareProviderStates(providerPBS, providerPVE); got >= 0 {
			t.Fatalf("compareProviderStates(pbs,pve) = %d, want negative", got)
		}
	})
	t.Run("positive when provider ranks higher", func(t *testing.T) {
		if got := compareProviderStates(providerPVE, providerPBS); got <= 0 {
			t.Fatalf("compareProviderStates(pve,pbs) = %d, want positive", got)
		}
	})
	t.Run("differentiates by scope when provider equal", func(t *testing.T) {
		if got := compareProviderStates(providerPVE, higherScope); got >= 0 {
			t.Fatalf("compareProviderStates(scope-1,scope-2) = %d, want negative", got)
		}
		if got := compareProviderStates(higherScope, providerPVE); got <= 0 {
			t.Fatalf("compareProviderStates(scope-2,scope-1) = %d, want positive", got)
		}
	})
	t.Run("differentiates by source when provider and scope equal", func(t *testing.T) {
		if got := compareProviderStates(providerPVE, higherSource); got >= 0 {
			t.Fatalf("compareProviderStates(src-1,src-2) = %d, want negative", got)
		}
		if got := compareProviderStates(higherSource, providerPVE); got <= 0 {
			t.Fatalf("compareProviderStates(src-2,src-1) = %d, want positive", got)
		}
	})
}
