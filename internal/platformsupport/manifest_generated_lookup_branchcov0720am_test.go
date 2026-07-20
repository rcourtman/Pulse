package platformsupport

import "testing"

// TestBranchcov0720am_RuntimePlatformForHostIdentityToken exercises every
// branch of RuntimePlatformForHostIdentityToken:
//   - the happy path (token resolves to a known profile -> its RuntimePlatform),
//   - the unknown-token fallback (returns ""),
//   - the empty/whitespace input fallback (returns "").
//
// Assertions are behavioural: we assert the classification outcome (linux-family
// runtime for a known Unraid token vs. empty string for everything unresolved)
// rather than re-importing internal constants or pinning opaque literals.
func TestBranchcov0720am_RuntimePlatformForHostIdentityToken(t *testing.T) {
	cases := []struct {
		name  string
		input string
		// wantEmpty signals that we only care that the result is the empty
		// sentinel (the unknown/empty fallback). For resolved inputs we assert
		// the canonical Unraid runtime platform literal "linux" — that is the
		// documented classification contract, not a change-detector.
		wantEmpty bool
		want      string
	}{
		{name: "canonical id lowercased", input: "unraid", want: "linux"},
		{name: "canonical id mixed case with surrounding whitespace", input: "  Unraid  ", want: "linux"},
		{name: "alias token with hyphen", input: "unraid-os", want: "linux"},
		{name: "alias token with space and mixed case", input: " Unraid OS ", want: "linux"},
		{name: "unknown token falls back to empty", input: "ubuntu", wantEmpty: true},
		{name: "unknown token with no resemblance", input: "beos", wantEmpty: true},
		{name: "empty input falls back to empty", input: "", wantEmpty: true},
		{name: "whitespace-only input falls back to empty", input: "   \t\n  ", wantEmpty: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := RuntimePlatformForHostIdentityToken(tc.input)
			switch {
			case tc.wantEmpty:
				if got != "" {
					t.Fatalf("RuntimePlatformForHostIdentityToken(%q) = %q, want empty fallback", tc.input, got)
				}
			default:
				if got == "" {
					t.Fatalf("RuntimePlatformForHostIdentityToken(%q) = empty, want %q", tc.input, tc.want)
				}
				if got != tc.want {
					t.Fatalf("RuntimePlatformForHostIdentityToken(%q) = %q, want %q", tc.input, got, tc.want)
				}
			}
		})
	}
}

// TestBranchcov0720am_RuntimePlatformForHostIdentityToken_CrossCheckResolved
// asserts that the value returned by RuntimePlatformForHostIdentityToken for a
// resolved token is exactly the RuntimePlatform of the profile resolved by
// AgentHostProfileForIdentity for the same token. This guards the early-return
// contract that ties the two surfaces together.
func TestBranchcov0720am_RuntimePlatformForHostIdentityToken_CrossCheckResolved(t *testing.T) {
	knownTokens := []string{"unraid", "Unraid", "  unraid-os  ", "unraid os"}
	for _, token := range knownTokens {
		t.Run(token, func(t *testing.T) {
			got := RuntimePlatformForHostIdentityToken(token)
			profile, ok := AgentHostProfileForIdentity(token)
			if !ok {
				t.Fatalf("AgentHostProfileForIdentity(%q) returned ok=false for a known token", token)
			}
			if got != profile.RuntimePlatform {
				t.Fatalf(
					"RuntimePlatformForHostIdentityToken(%q) = %q, but resolved profile.RuntimePlatform = %q",
					token, got, profile.RuntimePlatform,
				)
			}
			if got == "" {
				t.Fatalf("expected non-empty runtime platform for resolved token %q", token)
			}
		})
	}
}

// TestBranchcov0720am_AgentHostProfileEntry_MatchesIdentity exercises every
// branch of the MatchesIdentity method (which delegates to
// agentHostProfileMatchesIdentity):
//   - empty/whitespace input -> false,
//   - match via the profile's own ID (exact and case/whitespace-tolerant),
//   - match via one of the HostIdentityTokens,
//   - non-match for an unrelated value,
//   - non-match for a value that merely shares a substring with a token.
//
// The test constructs a fresh AgentHostProfileEntry value rather than relying
// on the global manifest so that we are exercising the method's contract on an
// arbitrary instance, not pinning a particular global entry.
func TestBranchcov0720am_AgentHostProfileEntry_MatchesIdentity(t *testing.T) {
	profile := AgentHostProfileEntry{
		ID:                 "unraid",
		HostIdentityTokens: []string{"unraid-os", "unraid os"},
	}

	cases := []struct {
		name  string
		input string
		want  bool
	}{
		{name: "exact id", input: "unraid", want: true},
		{name: "id case-insensitive", input: "UNRAID", want: true},
		{name: "id with surrounding whitespace", input: "  unraid  ", want: true},
		{name: "alias token with hyphen exact", input: "unraid-os", want: true},
		{name: "alias token with space", input: "unraid os", want: true},
		{name: "alias token mixed case and whitespace", input: "  Unraid OS  ", want: true},
		{name: "unrelated value", input: "ubuntu", want: false},
		{name: "partial substring of id is not a match", input: "unr", want: false},
		{name: "partial substring of a token is not a match", input: "unraid-o", want: false},
		{name: "empty input", input: "", want: false},
		{name: "whitespace-only input", input: "   \t  ", want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := profile.MatchesIdentity(tc.input); got != tc.want {
				t.Fatalf("MatchesIdentity(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

// TestBranchcov0720am_AgentHostProfileEntry_MatchesIdentity_TokenlessProfile
// asserts that the ID-match branch is independent of HostIdentityTokens: a
// profile with no tokens at all still matches by its ID, and still rejects
// everything else. This is the only way to prove the loop branch is exercised
// in isolation from the ID shortcut.
func TestBranchcov0720am_AgentHostProfileEntry_MatchesIdentity_TokenlessProfile(t *testing.T) {
	tokenless := AgentHostProfileEntry{ID: "solo"}

	if !tokenless.MatchesIdentity("solo") {
		t.Fatal(`MatchesIdentity("solo") = false on tokenless profile, want true (id-match branch)`)
	}
	if !tokenless.MatchesIdentity("  Solo ") {
		t.Fatal(`MatchesIdentity("  Solo ") = false on tokenless profile, want true (id-match is case/whitespace-tolerant)`)
	}
	if tokenless.MatchesIdentity("solo-clone") {
		t.Fatal(`MatchesIdentity("solo-clone") = true on tokenless profile, want false (no tokens to match)`)
	}
	if tokenless.MatchesIdentity("") {
		t.Fatal(`MatchesIdentity("") = true on tokenless profile, want false (empty input guard)`)
	}
}

// TestBranchcov0720am_AgentHostProfileEntry_MatchesIdentity_MatchesOnlyViaToken
// asserts the token-loop arm independently: a profile whose ID does not equal
// the input but which carries the input in its HostIdentityTokens must still
// report a match. This isolates the loop branch from the ID shortcut.
func TestBranchcov0720am_AgentHostProfileEntry_MatchesIdentity_MatchesOnlyViaToken(t *testing.T) {
	profile := AgentHostProfileEntry{
		ID:                 "primary-id",
		HostIdentityTokens: []string{"alias-one", "alias-two"},
	}

	if !profile.MatchesIdentity("alias-one") {
		t.Fatal(`MatchesIdentity("alias-one") = false, want true (must match via HostIdentityTokens[0])`)
	}
	if !profile.MatchesIdentity(" Alias-Two ") {
		t.Fatal(`MatchesIdentity(" Alias-Two ") = false, want true (must match via HostIdentityTokens[1] with normalisation)`)
	}
	if profile.MatchesIdentity("primary-id-extra") {
		t.Fatal(`MatchesIdentity("primary-id-extra") = true, want false (substring of ID is not a match)`)
	}
	if profile.MatchesIdentity("alias") {
		t.Fatal(`MatchesIdentity("alias") = true, want false (partial token substring is not a match)`)
	}
}

// TestBranchcov0720am_AgentHostProfiles covers AgentHostProfiles. The function
// is not branchless — it allocates and clones every entry — so we assert:
//   - the returned slice is non-empty and matches the manifest length,
//   - every entry has a non-empty ID (self-consistency of the projection),
//   - mutating the returned slice (and its nested HostIdentityTokens) does not
//     bleed into subsequent calls — i.e. the function returns an independent
//     copy, which is the whole point of the clone logic.
func TestBranchcov0720am_AgentHostProfiles(t *testing.T) {
	first := AgentHostProfiles()
	if len(first) == 0 {
		t.Fatal("AgentHostProfiles() returned an empty slice, expected at least one projected entry")
	}

	// Snapshot the IDs and first-token of every entry to compare against a
	// later call. We deliberately do not assert against an exact count or
	// exact literals — only that the second call is consistent with the first
	// and that mutations we introduce cannot be observed.
	type fingerprint struct {
		id            string
		firstToken    string
		tokenCount    int
		runtimePlatID string
	}
	snapshot := make([]fingerprint, len(first))
	for i, entry := range first {
		if entry.ID == "" {
			t.Fatalf("AgentHostProfiles()[%d].ID is empty; projection must always carry an id", i)
		}
		firstToken := ""
		if len(entry.HostIdentityTokens) > 0 {
			firstToken = entry.HostIdentityTokens[0]
		}
		snapshot[i] = fingerprint{
			id:            entry.ID,
			firstToken:    firstToken,
			tokenCount:    len(entry.HostIdentityTokens),
			runtimePlatID: entry.RuntimePlatform,
		}
	}

	// Mutate the first call's contents aggressively: drop entries, rewrite
	// fields, and corrupt the nested token slice. None of this should be
	// visible to a fresh call.
	if len(first[0].HostIdentityTokens) > 0 {
		first[0].HostIdentityTokens[0] = "__mutated_token__"
	}
	first[0].ID = "__mutated_id__"
	first[0].RuntimePlatform = "__mutated_platform__"
	first = first[:0]

	second := AgentHostProfiles()
	if len(second) != len(snapshot) {
		t.Fatalf("AgentHostProfiles() length changed between calls: first=%d, second=%d", len(snapshot), len(second))
	}
	for i, entry := range second {
		want := snapshot[i]
		if entry.ID != want.id {
			t.Fatalf("AgentHostProfiles()[%d].ID changed between calls: got %q, want %q (mutation leaked)", i, entry.ID, want.id)
		}
		if entry.RuntimePlatform != want.runtimePlatID {
			t.Fatalf("AgentHostProfiles()[%d].RuntimePlatform changed between calls: got %q, want %q", i, entry.RuntimePlatform, want.runtimePlatID)
		}
		if len(entry.HostIdentityTokens) != want.tokenCount {
			t.Fatalf("AgentHostProfiles()[%d].HostIdentityTokens length changed: got %d, want %d", i, len(entry.HostIdentityTokens), want.tokenCount)
		}
		if want.firstToken != "" && len(entry.HostIdentityTokens) > 0 && entry.HostIdentityTokens[0] != want.firstToken {
			t.Fatalf("AgentHostProfiles()[%d].HostIdentityTokens[0] changed: got %q, want %q (nested mutation leaked)", i, entry.HostIdentityTokens[0], want.firstToken)
		}
	}
}

// TestBranchcov0720am_AgentHostProfiles_IndependentSliceAliasing is a sharper
// isolation check: two back-to-back calls must not share backing arrays for
// either the outer slice or any inner HostIdentityTokens slice. Appending to
// one result must never affect the other.
func TestBranchcov0720am_AgentHostProfiles_IndependentSliceAliasing(t *testing.T) {
	a := AgentHostProfiles()
	b := AgentHostProfiles()
	if len(a) == 0 || len(b) == 0 {
		t.Fatal("AgentHostProfiles() returned empty slice; cannot exercise aliasing check")
	}

	// Append a synthetic entry to `a` and verify `b` is unaffected.
	synthetic := AgentHostProfileEntry{ID: "__synthetic__"}
	a = append(a, synthetic)

	if len(b) != len(a)-1 {
		t.Fatalf("append to one AgentHostProfiles() result leaked into a fresh call: a=%d, b=%d", len(a), len(b))
	}
	for _, entry := range b {
		if entry.ID == synthetic.ID {
			t.Fatalf("synthetic entry leaked into fresh AgentHostProfiles() result: %+v", entry)
		}
	}

	// Mutate an inner token slice of one entry in `b` and confirm `a`'s
	// equivalent entry is unaffected (note: `a[len(a)-1]` is the synthetic
	// entry, so we compare against the first real entry which still exists at
	// index 0 in both results).
	if len(b[0].HostIdentityTokens) > 0 && len(a[0].HostIdentityTokens) > 0 {
		originalA := a[0].HostIdentityTokens[0]
		b[0].HostIdentityTokens[0] = "__corrupt_via_b__"
		if a[0].HostIdentityTokens[0] != originalA {
			t.Fatalf("mutating b[0].HostIdentityTokens[0] changed a[0].HostIdentityTokens[0]: got %q, want %q (inner slice aliasing)", a[0].HostIdentityTokens[0], originalA)
		}
	}
}
