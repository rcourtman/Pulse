package hostagent

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// These tests raise branch/function coverage for previously-uncovered
// functions in the hostagent package without modifying any source file.
//
// Targets:
//   - tokenAlreadyExists (proxmox_setup.go:213) — pure two-channel classifier.
//   - observerBufferFileName (agent.go:879) — pure name builder.
//   - isAlreadyRegistered (proxmox_setup.go:1494) — filesystem predicate.
//
// Every subtest is independent (own t.TempDir() where the filesystem is
// touched) and asserts concrete return values, not "did not panic".

// TestBranchcov0723AmTokenAlreadyExists covers tokenAlreadyExists
// (proxmox_setup.go:213). The function lowercases — but does NOT trim — both
// fmt.Sprintf("%v", err) and output, then returns true if EITHER contains the
// single phrase "already exists". Each row drives a distinct arm/normalisation
// rule: nil err (which stringifies to "<nil>"), the err channel only, the
// output channel only, both, case folding on each channel, a wrapped error
// whose match is only reachable via fmt's flattening of %w, whitespace-padded
// output (not trimmed, but the substring search still hits), and several
// near-miss inputs that must return false.
func TestBranchcov0723AmTokenAlreadyExists(t *testing.T) {
	wrappedInner := errors.New("parameter 'token' already exists for this user")
	wrappedErr := fmt.Errorf("token rotation step failed: %w", wrappedInner)

	tests := []struct {
		name   string
		err    error
		output string
		want   bool
	}{
		{"NilErrAndEmptyOutputIsFalse", nil, "", false},
		{"PhraseInErrorOnlyMatches", errors.New("400 Bad Request: already exists"), "", true},
		{"PhraseInOutputOnlyMatches", nil, "pveum stderr: token already exists", true},
		{"PhraseInBothChannelsMatches", errors.New("already exists"), "already exists", true},
		{"UpperCaseErrorFoldedByToLower", errors.New("ALREADY EXISTS"), "", true},
		{"MixedCaseOutputFoldedByToLower", nil, "Token ALREADY Exists", true},
		{
			// The outer wrapper message contains no "already exists"; the match
			// is only possible because fmt.Sprintf("%v", err) flattens %w.
			name:   "WrappedErrorCarryingPhraseMatchesViaFlattening",
			err:    wrappedErr,
			output: "",
			want:   true,
		},
		{"PaddedOutputNotTrimmedButStillContainsPhrase", nil, "   \t already exists \n  ", true},
		{"NearMatchMissingTrailingSIsFalse", errors.New("already exist"), "", false},
		{"NearMissMisspelledIsFalse", nil, "alredy exists", false},
		{"WordsPresentButPhraseAbsentIsFalse", nil, "the token exists, registered already", false},
		{"SuperficiallySimilarUnrelatedMessageIsFalse", errors.New("permission denied"), "no changes made", false},
		{"NilErrStringifiesToNilAndMisses", nil, "created token abc123", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tokenAlreadyExists(tt.err, tt.output)
			if got != tt.want {
				t.Errorf("tokenAlreadyExists(err=%v, output=%q) = %v, want %v", tt.err, tt.output, got, tt.want)
			}
		})
	}
}

// TestBranchcov0723AmObserverBufferFileName covers observerBufferFileName
// (agent.go:879), a pure name builder. It asserts the exact formatted string
// and that the supplied id is interpolated verbatim (no sanitisation), which is
// the real behavioural contract callers rely on.
func TestBranchcov0723AmObserverBufferFileName(t *testing.T) {
	tests := []struct {
		name string
		id   string
		want string
	}{
		{"TypicalID", "obs-1", "report-buffer-observer-obs-1.json"},
		{"EmptyIDProducesEmptySegment", "", "report-buffer-observer-.json"},
		{"IDSuffixedWithUUID", "9cf3a1b2-uuid", "report-buffer-observer-9cf3a1b2-uuid.json"},
		{"IDInsertedVerbatimNoSanitisation", "a/b", "report-buffer-observer-a/b.json"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := observerBufferFileName(tt.id); got != tt.want {
				t.Errorf("observerBufferFileName(%q) = %q, want %q", tt.id, got, tt.want)
			}
		})
	}
}

// TestBranchcov0723AmIsAlreadyRegistered covers isAlreadyRegistered
// (proxmox_setup.go:1494), a predicate that reports whether the legacy state
// marker file exists. Each subtest uses its own t.TempDir() and a collector
// backed by os.Stat so the filesystem state is real but isolated. The false
// arm (no marker), the true arm (legacy marker present), and a guard proving a
// *different* state filename does not satisfy the predicate are all asserted.
func TestBranchcov0723AmIsAlreadyRegistered(t *testing.T) {
	newSetup := func(t *testing.T, stateDir string) *ProxmoxSetup {
		t.Helper()
		return &ProxmoxSetup{
			stateDir:  stateDir,
			collector: &mockCollector{statFn: os.Stat},
		}
	}

	t.Run("NoMarkerFileIsFalse", func(t *testing.T) {
		dir := t.TempDir()
		p := newSetup(t, dir)
		if p.isAlreadyRegistered() {
			t.Errorf("isAlreadyRegistered() = true in empty dir %q, want false", dir)
		}
	})

	t.Run("LegacyMarkerPresentIsTrue", func(t *testing.T) {
		dir := t.TempDir()
		legacyPath := filepath.Join(dir, "proxmox-registered")
		if err := os.WriteFile(legacyPath, []byte("2026-01-01T00:00:00Z"), 0600); err != nil {
			t.Fatalf("write legacy marker: %v", err)
		}
		p := newSetup(t, dir)
		if !p.isAlreadyRegistered() {
			t.Errorf("isAlreadyRegistered() = false with marker at %q, want true", legacyPath)
		}
	})

	t.Run("OnlyPerTypeMarkerDoesNotSatisfyLegacyPredicate", func(t *testing.T) {
		// isAlreadyRegistered looks exclusively at the legacy
		// "proxmox-registered" path; a per-type marker (e.g. the PVE one)
		// must not cause it to report true.
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "proxmox-pve-registered"), []byte("x"), 0600); err != nil {
			t.Fatalf("write pve marker: %v", err)
		}
		p := newSetup(t, dir)
		if p.isAlreadyRegistered() {
			t.Errorf("isAlreadyRegistered() = true with only a per-type marker present, want false (legacy path only)")
		}
	})
}
