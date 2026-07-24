package config

import (
	"encoding/json"
	"errors"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Tests in this file use the TestBranchcov0724pm prefix so the scoped run
//
//	go test ./internal/config/ -run '^TestBranchcov0724pm' -count=1
//
// selects only them. They raise coverage for previously-uncovered (0.0%)
// AI chat session persistence helpers in persistence.go:
//
//   - ConfigPersistence.SaveAIChatSessions       (persistence.go:3758)
//   - ConfigPersistence.SaveAIChatSession        (persistence.go:3853)
//   - ConfigPersistence.DeleteAIChatSession      (persistence.go:3866)
//   - ConfigPersistence.GetAIChatSessionsForUser (persistence.go:3877)
//   - ConfigPersistence.CleanupOldAIChatSessions (persistence.go:3900)
//
// Every target is file-backed and exercised under t.TempDir (no network,
// daemon, SSH, or live database), so none is skipped on purity grounds.
// Each ConfigPersistence is constructed exactly the way the existing tests in
// this package do it: NewConfigPersistence(tempDir) + EnsureConfigDir, which
// also wires up encryption, so the encrypted save/load paths are exercised.
// The no-encryption path is covered by explicitly setting crypto = nil, and
// filesystem error arms are covered via the package's existing mockFSError.

// newAIChatTestCP builds a ConfigPersistence rooted at a fresh temp dir, the
// same way persistence_ai_test.go / persistence_profiles_branchcov0724pm_test.go do.
func newAIChatTestCP(t *testing.T) *ConfigPersistence {
	t.Helper()
	cp := NewConfigPersistence(t.TempDir())
	require.NoError(t, cp.EnsureConfigDir())
	return cp
}

// aiChatSessionAt builds a fully-populated session for deterministic round
// trips. updatedAt is caller-controlled so cleanup tests never depend on
// time.Now-relative fuzz that could flake.
func aiChatSessionAt(id, username string, updatedAt time.Time) *AIChatSession {
	return &AIChatSession{
		ID:        id,
		Username:  username,
		Title:     "session " + id,
		CreatedAt: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
		UpdatedAt: updatedAt,
		Messages: []AIChatMessage{
			{
				ID:        id + "-msg-1",
				Role:      "user",
				Content:   "hello from " + username,
				Timestamp: updatedAt,
				Model:     "test-model",
				Tokens:    &AIChatMessageTokens{Input: 5, Output: 7},
				ToolCalls: []AIChatToolCall{{Name: "ping", Input: "in", Output: "out", Success: true}},
			},
		},
	}
}

// aiChatSessionIDs returns a sorted, dedup-independent list of session IDs from
// a slice, so survivor/membership assertions are order-independent.
func aiChatSessionIDs(sessions []*AIChatSession) []string {
	ids := make([]string, len(sessions))
	for i, s := range sessions {
		ids[i] = s.ID
	}
	sort.Strings(ids)
	return ids
}

// ---------------------------------------------------------------------------
// SaveAIChatSessions  (persistence.go:3758)
// ---------------------------------------------------------------------------

func TestBranchcov0724pmSaveAIChatSessions(t *testing.T) {
	t.Run("encrypted save of empty map round trips as empty", func(t *testing.T) {
		cp := newAIChatTestCP(t)

		require.NoError(t, cp.SaveAIChatSessions(map[string]*AIChatSession{}))

		loaded, err := cp.LoadAIChatSessions()
		require.NoError(t, err)
		require.NotNil(t, loaded.Sessions)
		assert.Empty(t, loaded.Sessions, "an explicitly empty map must round trip as empty")
		assert.Equal(t, 1, loaded.Version)
	})

	t.Run("nil map round trips as a non-nil empty map", func(t *testing.T) {
		cp := newAIChatTestCP(t)

		require.NoError(t, cp.SaveAIChatSessions(nil))

		loaded, err := cp.LoadAIChatSessions()
		require.NoError(t, err)
		require.NotNil(t, loaded.Sessions, "LoadAIChatSessions must materialize nil sessions as a non-nil map")
		assert.Empty(t, loaded.Sessions)
	})

	t.Run("encrypted save of populated map round trips verbatim", func(t *testing.T) {
		cp := newAIChatTestCP(t)

		updated := time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)
		want := map[string]*AIChatSession{
			"s1": aiChatSessionAt("s1", "alice", updated),
			"s2": aiChatSessionAt("s2", "bob", updated.Add(time.Hour)),
		}
		require.NoError(t, cp.SaveAIChatSessions(want))

		loaded, err := cp.LoadAIChatSessions()
		require.NoError(t, err)
		require.Len(t, loaded.Sessions, 2)
		assert.Equal(t, want["s1"].Title, loaded.Sessions["s1"].Title)
		assert.Equal(t, want["s2"].Messages, loaded.Sessions["s2"].Messages)
		assert.Equal(t, want["s1"].Messages[0].Tokens, loaded.Sessions["s1"].Messages[0].Tokens,
			"nested pointer tokens must survive the encrypted round trip")

		// The file on disk must NOT be plaintext JSON when encryption is active.
		raw, err := os.ReadFile(cp.aiChatSessionsFile)
		require.NoError(t, err)
		assert.False(t, json.Valid(raw), "with crypto enabled the persisted bytes must be encrypted, not raw JSON")
	})

	t.Run("nil-crypto plaintext save round trips and is stored as plaintext", func(t *testing.T) {
		cp := newAIChatTestCP(t)
		cp.crypto = nil // exercise the c.crypto == nil branch

		updated := time.Date(2025, 3, 1, 8, 0, 0, 0, time.UTC)
		want := map[string]*AIChatSession{
			"plain-1": aiChatSessionAt("plain-1", "alice", updated),
		}
		require.NoError(t, cp.SaveAIChatSessions(want))

		loaded, err := cp.LoadAIChatSessions()
		require.NoError(t, err)
		require.Len(t, loaded.Sessions, 1)
		assert.Equal(t, want["plain-1"].Content0(), loaded.Sessions["plain-1"].Content0())

		// With crypto disabled the bytes on disk must be valid plaintext JSON.
		raw, err := os.ReadFile(cp.aiChatSessionsFile)
		require.NoError(t, err)
		var direct AIChatSessionsData
		assert.NoError(t, json.Unmarshal(raw, &direct), "nil-crypto file must be plaintext JSON")
		assert.Equal(t, 1, direct.Version)
	})

	t.Run("wraps EnsureConfigDir (mkdir) error", func(t *testing.T) {
		cp := newAIChatTestCP(t)
		mfs := &mockFSError{FileSystem: defaultFileSystem{}, mkdirError: errors.New("boom-mkdir")}
		cp.SetFileSystem(mfs)

		err := cp.SaveAIChatSessions(map[string]*AIChatSession{"s1": aiChatSessionAt("s1", "u", time.Now())})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "prepare config directory for ai chat sessions")
		assert.Contains(t, err.Error(), "boom-mkdir")
	})

	t.Run("wraps write error after encryption", func(t *testing.T) {
		cp := newAIChatTestCP(t)
		mfs := &mockFSError{FileSystem: defaultFileSystem{}, writeError: errors.New("boom-write")}
		cp.SetFileSystem(mfs)

		err := cp.SaveAIChatSessions(map[string]*AIChatSession{"s1": aiChatSessionAt("s1", "u", time.Now())})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "persist ai chat sessions")
		assert.Contains(t, err.Error(), "boom-write")
	})
}

// Content0 returns the first message's content, a tiny convenience so test
// assertions stay readable.
func (s *AIChatSession) Content0() string {
	if len(s.Messages) == 0 {
		return ""
	}
	return s.Messages[0].Content
}

// ---------------------------------------------------------------------------
// SaveAIChatSession  (persistence.go:3853)
// ---------------------------------------------------------------------------

func TestBranchcov0724pmSaveAIChatSession(t *testing.T) {
	t.Run("saves a new session into an empty store", func(t *testing.T) {
		cp := newAIChatTestCP(t)

		sess := aiChatSessionAt("sess-1", "alice", time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC))
		require.NoError(t, cp.SaveAIChatSession(sess))

		loaded, err := cp.LoadAIChatSessions()
		require.NoError(t, err)
		require.Len(t, loaded.Sessions, 1)
		assert.Equal(t, "alice", loaded.Sessions["sess-1"].Username)
		// SaveAIChatSession stamps UpdatedAt to ~now; assert it is recent and
		// non-zero rather than an exact value to avoid time.Now fuzz.
		assert.False(t, loaded.Sessions["sess-1"].UpdatedAt.IsZero())
		assert.True(t, time.Since(loaded.Sessions["sess-1"].UpdatedAt) < 5*time.Second,
			"UpdatedAt must be set to the current time by SaveAIChatSession")
	})

	t.Run("updates an existing session in place", func(t *testing.T) {
		cp := newAIChatTestCP(t)

		require.NoError(t, cp.SaveAIChatSession(aiChatSessionAt("sess-1", "alice", time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))))

		// Save the same ID again with different content/title.
		updated := aiChatSessionAt("sess-1", "alice", time.Date(2025, 1, 3, 0, 0, 0, 0, time.UTC))
		updated.Title = "renamed title"
		require.NoError(t, cp.SaveAIChatSession(updated))

		loaded, err := cp.LoadAIChatSessions()
		require.NoError(t, err)
		require.Len(t, loaded.Sessions, 1, "updating must not duplicate the session")
		assert.Equal(t, "renamed title", loaded.Sessions["sess-1"].Title)
	})

	t.Run("wraps load error", func(t *testing.T) {
		cp := newAIChatTestCP(t)
		mfs := &mockFSError{FileSystem: defaultFileSystem{}, readError: errors.New("boom-read")}
		cp.SetFileSystem(mfs)

		err := cp.SaveAIChatSession(aiChatSessionAt("x", "u", time.Now()))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "load ai chat sessions")
	})

	t.Run("wraps save error from the underlying bulk save", func(t *testing.T) {
		cp := newAIChatTestCP(t)
		mfs := &mockFSError{FileSystem: defaultFileSystem{}, writeError: errors.New("boom-write")}
		cp.SetFileSystem(mfs)

		err := cp.SaveAIChatSession(aiChatSessionAt("x", "u", time.Now()))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "persist ai chat sessions")
	})
}

// ---------------------------------------------------------------------------
// DeleteAIChatSession  (persistence.go:3866)
// ---------------------------------------------------------------------------

func TestBranchcov0724pmDeleteAIChatSession(t *testing.T) {
	t.Run("deletes an existing session while preserving the others", func(t *testing.T) {
		cp := newAIChatTestCP(t)
		ts := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
		require.NoError(t, cp.SaveAIChatSessions(map[string]*AIChatSession{
			"keep": aiChatSessionAt("keep", "alice", ts),
			"gone": aiChatSessionAt("gone", "alice", ts.Add(time.Hour)),
		}))

		require.NoError(t, cp.DeleteAIChatSession("gone"))

		loaded, err := cp.LoadAIChatSessions()
		require.NoError(t, err)
		require.Len(t, loaded.Sessions, 1)
		assert.Contains(t, loaded.Sessions, "keep")
		assert.NotContains(t, loaded.Sessions, "gone")
	})

	t.Run("deleting a non-existent id is a no-op (no error, store unchanged)", func(t *testing.T) {
		cp := newAIChatTestCP(t)
		ts := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
		require.NoError(t, cp.SaveAIChatSessions(map[string]*AIChatSession{
			"a": aiChatSessionAt("a", "alice", ts),
			"b": aiChatSessionAt("b", "alice", ts.Add(time.Hour)),
		}))

		require.NoError(t, cp.DeleteAIChatSession("does-not-exist"))

		loaded, err := cp.LoadAIChatSessions()
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{"a", "b"}, aiChatSessionIDs(mapValues(loaded.Sessions)))
	})

	t.Run("deleting from an empty store succeeds and leaves it empty", func(t *testing.T) {
		cp := newAIChatTestCP(t)

		require.NoError(t, cp.DeleteAIChatSession("nothing-here"))

		loaded, err := cp.LoadAIChatSessions()
		require.NoError(t, err)
		assert.Empty(t, loaded.Sessions)
	})

	t.Run("wraps load error", func(t *testing.T) {
		cp := newAIChatTestCP(t)
		mfs := &mockFSError{FileSystem: defaultFileSystem{}, readError: errors.New("boom-read")}
		cp.SetFileSystem(mfs)

		err := cp.DeleteAIChatSession("x")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "load ai chat sessions")
	})

	t.Run("wraps save error when re-persisting after deletion", func(t *testing.T) {
		cp := newAIChatTestCP(t)
		ts := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
		require.NoError(t, cp.SaveAIChatSessions(map[string]*AIChatSession{
			"gone": aiChatSessionAt("gone", "alice", ts),
		}))
		mfs := &mockFSError{FileSystem: defaultFileSystem{}, writeError: errors.New("boom-write")}
		cp.SetFileSystem(mfs)

		err := cp.DeleteAIChatSession("gone")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "persist ai chat sessions")
	})
}

// ---------------------------------------------------------------------------
// GetAIChatSessionsForUser  (persistence.go:3877)
// ---------------------------------------------------------------------------

func TestBranchcov0724pmGetAIChatSessionsForUser(t *testing.T) {
	t.Run("returns only the owning user's sessions, excluding every other user", func(t *testing.T) {
		cp := newAIChatTestCP(t)
		ts := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
		require.NoError(t, cp.SaveAIChatSessions(map[string]*AIChatSession{
			"a1": aiChatSessionAt("a1", "alice", ts),
			"a2": aiChatSessionAt("a2", "alice", ts.Add(2*time.Hour)),
			"b1": aiChatSessionAt("b1", "bob", ts.Add(time.Hour)),
			"b2": aiChatSessionAt("b2", "bob", ts.Add(3*time.Hour)),
		}))

		got, err := cp.GetAIChatSessionsForUser("alice")
		require.NoError(t, err)
		require.Len(t, got, 2, "only alice's two sessions must be returned")
		assert.ElementsMatch(t, []string{"a1", "a2"}, aiChatSessionIDs(got))
		// The other user's sessions must NOT appear.
		for _, s := range got {
			assert.Equal(t, "alice", s.Username, "no session belonging to another user may leak through")
		}
	})

	t.Run("empty username returns all sessions across every user", func(t *testing.T) {
		cp := newAIChatTestCP(t)
		ts := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
		require.NoError(t, cp.SaveAIChatSessions(map[string]*AIChatSession{
			"a1": aiChatSessionAt("a1", "alice", ts),
			"b1": aiChatSessionAt("b1", "bob", ts.Add(time.Hour)),
		}))

		got, err := cp.GetAIChatSessionsForUser("")
		require.NoError(t, err)
		require.Len(t, got, 2)
		assert.ElementsMatch(t, []string{"a1", "b1"}, aiChatSessionIDs(got))
	})

	t.Run("result is sorted by UpdatedAt descending", func(t *testing.T) {
		cp := newAIChatTestCP(t)
		base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
		require.NoError(t, cp.SaveAIChatSessions(map[string]*AIChatSession{
			"old":   aiChatSessionAt("old", "alice", base),
			"newer": aiChatSessionAt("newer", "alice", base.Add(48*time.Hour)),
			"mid":   aiChatSessionAt("mid", "alice", base.Add(24*time.Hour)),
		}))

		got, err := cp.GetAIChatSessionsForUser("")
		require.NoError(t, err)
		require.Len(t, got, 3)
		assert.Equal(t, []string{"newer", "mid", "old"}, []string{got[0].ID, got[1].ID, got[2].ID},
			"sessions must be ordered most-recently-updated first")
		for i := 1; i < len(got); i++ {
			assert.False(t, got[i].UpdatedAt.After(got[i-1].UpdatedAt),
				"slice must be descending by UpdatedAt")
		}
	})

	t.Run("empty store returns an empty slice and nil error", func(t *testing.T) {
		cp := newAIChatTestCP(t)

		got, err := cp.GetAIChatSessionsForUser("nobody")
		require.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("non-matching username returns an empty slice", func(t *testing.T) {
		cp := newAIChatTestCP(t)
		require.NoError(t, cp.SaveAIChatSessions(map[string]*AIChatSession{
			"a1": aiChatSessionAt("a1", "alice", time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)),
		}))

		got, err := cp.GetAIChatSessionsForUser("carol")
		require.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("consecutive calls return equal but independent results", func(t *testing.T) {
		cp := newAIChatTestCP(t)
		ts := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
		require.NoError(t, cp.SaveAIChatSessions(map[string]*AIChatSession{
			"a1": aiChatSessionAt("a1", "alice", ts),
		}))

		first, err := cp.GetAIChatSessionsForUser("alice")
		require.NoError(t, err)
		second, err := cp.GetAIChatSessionsForUser("alice")
		require.NoError(t, err)

		require.Len(t, first, 1)
		require.Len(t, second, 1)
		assert.Equal(t, first[0].ID, second[0].ID)
		// Each call re-reads from disk, so the returned pointers are distinct
		// objects; mutating one result must not affect the other.
		assert.NotSame(t, first[0], second[0], "separate calls must not alias the same struct")
		first[0].Title = "mutated"
		assert.NotEqual(t, "mutated", second[0].Title, "mutating one result must not leak into another call's result")
	})

	t.Run("wraps load error", func(t *testing.T) {
		cp := newAIChatTestCP(t)
		mfs := &mockFSError{FileSystem: defaultFileSystem{}, readError: errors.New("boom-read")}
		cp.SetFileSystem(mfs)

		got, err := cp.GetAIChatSessionsForUser("alice")
		require.Error(t, err)
		assert.Nil(t, got)
	})
}

// ---------------------------------------------------------------------------
// CleanupOldAIChatSessions  (persistence.go:3900)
// ---------------------------------------------------------------------------

func TestBranchcov0724pmCleanupOldAIChatSessions(t *testing.T) {
	// Deterministic timestamps chosen to be unambiguously ancient / future
	// relative to any realistic "now" so the Before(cutoff) comparison never
	// flakes. With maxAge = 24h, cutoff ~= today; year 2000 is always before
	// it and year 2099 is always after it.
	const ancientYear = 2000
	const futureYear = 2099
	ancient := time.Date(ancientYear, 1, 1, 0, 0, 0, 0, time.UTC)
	future := time.Date(futureYear, 1, 1, 0, 0, 0, 0, time.UTC)
	maxAge := 24 * time.Hour

	t.Run("removes only stale sessions, returns count, preserves survivors", func(t *testing.T) {
		cp := newAIChatTestCP(t)
		require.NoError(t, cp.SaveAIChatSessions(map[string]*AIChatSession{
			"old-1":   aiChatSessionAt("old-1", "alice", ancient),
			"old-2":   aiChatSessionAt("old-2", "bob", ancient.Add(time.Second)),
			"fresh-1": aiChatSessionAt("fresh-1", "alice", future),
			"fresh-2": aiChatSessionAt("fresh-2", "bob", future.Add(-time.Second)),
		}))

		removed, err := cp.CleanupOldAIChatSessions(maxAge)
		require.NoError(t, err)
		assert.Equal(t, 2, removed, "exactly the two ancient sessions must be removed")

		loaded, err := cp.LoadAIChatSessions()
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{"fresh-1", "fresh-2"}, aiChatSessionIDs(mapValues(loaded.Sessions)),
			"only the future-dated sessions may survive")
	})

	t.Run("all fresh sessions yield zero removals and leave the store untouched", func(t *testing.T) {
		cp := newAIChatTestCP(t)
		require.NoError(t, cp.SaveAIChatSessions(map[string]*AIChatSession{
			"f1": aiChatSessionAt("f1", "alice", future),
			"f2": aiChatSessionAt("f2", "bob", future.Add(-time.Hour)),
		}))

		removed, err := cp.CleanupOldAIChatSessions(maxAge)
		require.NoError(t, err)
		assert.Equal(t, 0, removed, "no fresh session may be removed")

		loaded, err := cp.LoadAIChatSessions()
		require.NoError(t, err)
		assert.Len(t, loaded.Sessions, 2, "store must be unchanged when nothing was removed")
	})

	t.Run("all stale sessions are removed", func(t *testing.T) {
		cp := newAIChatTestCP(t)
		require.NoError(t, cp.SaveAIChatSessions(map[string]*AIChatSession{
			"o1": aiChatSessionAt("o1", "alice", ancient),
			"o2": aiChatSessionAt("o2", "bob", ancient.Add(time.Minute)),
		}))

		removed, err := cp.CleanupOldAIChatSessions(maxAge)
		require.NoError(t, err)
		assert.Equal(t, 2, removed)

		loaded, err := cp.LoadAIChatSessions()
		require.NoError(t, err)
		assert.Empty(t, loaded.Sessions, "every stale session must be purged")
	})

	t.Run("cleanup over an empty store removes nothing and does not error", func(t *testing.T) {
		cp := newAIChatTestCP(t)

		removed, err := cp.CleanupOldAIChatSessions(maxAge)
		require.NoError(t, err)
		assert.Equal(t, 0, removed)
	})

	t.Run("wraps load error", func(t *testing.T) {
		cp := newAIChatTestCP(t)
		mfs := &mockFSError{FileSystem: defaultFileSystem{}, readError: errors.New("boom-read")}
		cp.SetFileSystem(mfs)

		removed, err := cp.CleanupOldAIChatSessions(maxAge)
		require.Error(t, err)
		assert.Equal(t, 0, removed)
	})

	t.Run("wraps save error when removals occur", func(t *testing.T) {
		cp := newAIChatTestCP(t)
		require.NoError(t, cp.SaveAIChatSessions(map[string]*AIChatSession{
			"old-1":   aiChatSessionAt("old-1", "alice", ancient),
			"fresh-1": aiChatSessionAt("fresh-1", "alice", future),
		}))
		mfs := &mockFSError{FileSystem: defaultFileSystem{}, writeError: errors.New("boom-write")}
		cp.SetFileSystem(mfs)

		removed, err := cp.CleanupOldAIChatSessions(maxAge)
		require.Error(t, err)
		assert.Equal(t, 0, removed, "a save error must surface as zero confirmed removals")
		assert.Contains(t, err.Error(), "persist ai chat sessions")
	})
}

// mapValues returns the values of a string-keyed session map as a slice, for
// the survivor assertions that consume []*AIChatSession.
func mapValues(m map[string]*AIChatSession) []*AIChatSession {
	out := make([]*AIChatSession, 0, len(m))
	for _, v := range m {
		out = append(out, v)
	}
	return out
}
