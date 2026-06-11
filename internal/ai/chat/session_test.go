package chat

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessionStore(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "session-store-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	store, err := NewSessionStore(tempDir)
	require.NoError(t, err)

	t.Run("Create and Get", func(t *testing.T) {
		session, err := store.Create()
		require.NoError(t, err)
		assert.NotEmpty(t, session.ID)
		assert.Empty(t, session.Title)

		retrieved, err := store.Get(session.ID)
		require.NoError(t, err)
		assert.Equal(t, session.ID, retrieved.ID)
	})

	t.Run("List", func(t *testing.T) {
		// New store for isolation
		d := filepath.Join(tempDir, "list-test")
		s, _ := NewSessionStore(d)

		s1, _ := s.Create()
		time.Sleep(10 * time.Millisecond) // Ensure time difference
		s2, _ := s.Create()

		sessions, err := s.List()
		require.NoError(t, err)
		require.Len(t, sessions, 2)
		// Should be newest first
		assert.Equal(t, s2.ID, sessions[0].ID)
		assert.Equal(t, s1.ID, sessions[1].ID)
	})

	t.Run("Delete", func(t *testing.T) {
		session, _ := store.Create()
		err := store.Delete(session.ID)
		assert.NoError(t, err)

		_, err = store.Get(session.ID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "session not found")
	})

	t.Run("Rename", func(t *testing.T) {
		session, _ := store.Create()
		renamed, err := store.Rename(session.ID, "  Renamed\nsession  ")
		require.NoError(t, err)
		assert.Equal(t, "Renamed session", renamed.Title)

		retrieved, err := store.Get(session.ID)
		require.NoError(t, err)
		assert.Equal(t, "Renamed session", retrieved.Title)
	})

	t.Run("Rename rejects empty title", func(t *testing.T) {
		session, _ := store.Create()
		_, err := store.Rename(session.ID, "   ")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "session title required")
	})

	t.Run("AddMessage and Title Generation", func(t *testing.T) {
		session, _ := store.Create()
		msg := Message{
			Role:    "user",
			Content: "What is the status of node-1?",
		}
		err := store.AddMessage(session.ID, msg)
		require.NoError(t, err)

		updated, _ := store.Get(session.ID)
		assert.Equal(t, "What is the status of node-1?", updated.Title)
		assert.Equal(t, 1, updated.MessageCount)

		messages, err := store.GetMessages(session.ID)
		require.NoError(t, err)
		require.Len(t, messages, 1)
		assert.Equal(t, "What is the status of node-1?", messages[0].Content)
		assert.NotNil(t, messages[0].ToolCalls)
	})

	t.Run("UpdateLastMessage", func(t *testing.T) {
		session, _ := store.Create()
		_ = store.AddMessage(session.ID, Message{Role: "assistant", Content: "Thinking..."})

		updatedMsg := Message{Role: "assistant", Content: "Resolved."}
		err := store.UpdateLastMessage(session.ID, updatedMsg)
		require.NoError(t, err)

		messages, _ := store.GetMessages(session.ID)
		assert.Equal(t, "Resolved.", messages[0].Content)
	})

	t.Run("EnsureSession", func(t *testing.T) {
		session, err := store.EnsureSession("")
		assert.NoError(t, err)
		assert.NotEmpty(t, session.ID)

		sessionFixed, err := store.EnsureSession("fixed-id")
		assert.NoError(t, err)
		assert.Equal(t, "fixed-id", sessionFixed.ID)

		retrieved, err := store.EnsureSession("fixed-id")
		assert.NoError(t, err)
		assert.Equal(t, sessionFixed.ID, retrieved.ID)
	})

	t.Run("TrimMessages keeps the most recent N", func(t *testing.T) {
		// Patrol-main session was reused across every scheduled run
		// and grew to 16 MB / 3,593 messages within a month. Without
		// a per-session bound, the file rewrites on every AddMessage
		// got linearly more expensive. TrimMessages caps the slice to
		// the most recent N messages so the on-disk footprint stays
		// bounded and the I/O cost per write stays flat.
		session, err := store.EnsureSession("trim-test")
		require.NoError(t, err)
		for i := 0; i < 50; i++ {
			require.NoError(t, store.AddMessage(session.ID, Message{
				Role:    "user",
				Content: fmt.Sprintf("message-%d", i),
			}))
		}

		// Trim to the last 10 — must drop messages 0..39 and keep 40..49.
		require.NoError(t, store.TrimMessages(session.ID, 10))
		got, err := store.GetMessages(session.ID)
		require.NoError(t, err)
		require.Len(t, got, 10)
		assert.Equal(t, "message-40", got[0].Content)
		assert.Equal(t, "message-49", got[9].Content)
	})

	t.Run("TrimMessages is a no-op below threshold", func(t *testing.T) {
		// When the session already has fewer messages than the cap,
		// no rewrite should happen and no messages should be lost.
		session, err := store.EnsureSession("trim-noop")
		require.NoError(t, err)
		for i := 0; i < 5; i++ {
			require.NoError(t, store.AddMessage(session.ID, Message{
				Role:    "user",
				Content: fmt.Sprintf("msg-%d", i),
			}))
		}
		require.NoError(t, store.TrimMessages(session.ID, 200))
		got, err := store.GetMessages(session.ID)
		require.NoError(t, err)
		require.Len(t, got, 5)
	})

	t.Run("TrimMessages with non-positive keep is a no-op", func(t *testing.T) {
		// keepMostRecent <= 0 must not silently truncate the entire
		// session — that would be a footgun. Callers can disable the
		// cap explicitly by passing 0 for user-driven chat sessions
		// where full history is the product.
		session, err := store.EnsureSession("trim-zero")
		require.NoError(t, err)
		for i := 0; i < 3; i++ {
			require.NoError(t, store.AddMessage(session.ID, Message{
				Role:    "user",
				Content: fmt.Sprintf("zero-%d", i),
			}))
		}
		require.NoError(t, store.TrimMessages(session.ID, 0))
		require.NoError(t, store.TrimMessages(session.ID, -5))
		got, err := store.GetMessages(session.ID)
		require.NoError(t, err)
		require.Len(t, got, 3)
	})
}

func TestSessionStoreForkClonesTranscriptAndHandoffSummary(t *testing.T) {
	store, err := NewSessionStore(t.TempDir())
	require.NoError(t, err)

	source, err := store.Create()
	require.NoError(t, err)

	require.NoError(t, store.AddMessage(source.ID, Message{
		ID:      "user-1",
		Role:    "user",
		Content: "Inspect the storage warning",
	}))
	require.NoError(t, store.AddMessage(source.ID, Message{
		ID:      "assistant-1",
		Role:    "assistant",
		Content: "I found the warning.",
		ToolCalls: []ToolCall{
			{
				ID:    "tool-1",
				Name:  "pulse_query",
				Input: map[string]interface{}{"action": "resource"},
			},
		},
	}))
	require.NoError(t, store.SetModelHandoffEnvelope(
		source.ID,
		"finding-storage-1",
		"private model handoff context must stay model-only",
		[]HandoffResource{{ID: "storage:pool-a", Name: "pool-a", Type: "storage", Node: "nas-a"}},
		[]HandoffAction{{
			FindingID:      "finding-storage-1",
			ApprovalID:     "approval-1",
			ApprovalStatus: "pending",
			Description:    "Review storage warning",
		}},
		HandoffMetadata{},
	))

	fork, err := store.Fork(source.ID)
	require.NoError(t, err)
	require.NotNil(t, fork)
	assert.NotEqual(t, source.ID, fork.ID)
	assert.Equal(t, "Fork of Inspect the storage warning", fork.Title)
	assert.Equal(t, 2, fork.MessageCount)
	require.NotNil(t, fork.HandoffSummary)
	assert.Equal(t, sessionHandoffKindPatrolFinding, fork.HandoffSummary.Kind)
	assert.Equal(t, "finding-storage-1", fork.HandoffSummary.FindingID)
	assert.Equal(t, 1, fork.HandoffSummary.ResourceCount)
	assert.Equal(t, 1, fork.HandoffSummary.ActionCount)
	assert.True(t, fork.HandoffSummary.RequiresApproval)
	require.NotNil(t, fork.HandoffSummary.PrimaryResource)
	assert.Equal(t, "pool-a", fork.HandoffSummary.PrimaryResource.Name)

	forkMessages, err := store.GetMessages(fork.ID)
	require.NoError(t, err)
	require.Len(t, forkMessages, 2)
	assert.Equal(t, "user-1", forkMessages[0].ID)
	assert.Equal(t, "Inspect the storage warning", forkMessages[0].Content)
	require.Len(t, forkMessages[1].ToolCalls, 1)
	assert.Equal(t, "tool-1", forkMessages[1].ToolCalls[0].ID)

	require.NoError(t, store.AddMessage(fork.ID, Message{Role: "user", Content: "Continue here"}))
	sourceMessages, err := store.GetMessages(source.ID)
	require.NoError(t, err)
	require.Len(t, sourceMessages, 2)
}

func TestSessionStoreUndoRedoLastTurn(t *testing.T) {
	store, err := NewSessionStore(t.TempDir())
	require.NoError(t, err)

	session, err := store.Create()
	require.NoError(t, err)

	require.NoError(t, store.AddMessage(session.ID, Message{ID: "u1", Role: "user", Content: "First prompt"}))
	require.NoError(t, store.AddMessage(session.ID, Message{ID: "a1", Role: "assistant", Content: "First answer"}))
	require.NoError(t, store.AddMessage(session.ID, Message{ID: "u2", Role: "user", Content: "Second prompt"}))
	require.NoError(t, store.AddMessage(session.ID, Message{ID: "a2", Role: "assistant", Content: "Second answer"}))

	undo, err := store.UndoLastTurn(session.ID)
	require.NoError(t, err)
	require.NotNil(t, undo)
	assert.True(t, undo.Success)
	assert.Equal(t, "Second prompt", undo.RestoredPrompt)
	assert.Equal(t, 2, undo.RemovedMessages)
	assert.True(t, undo.CanRedo)

	summary, err := store.Get(session.ID)
	require.NoError(t, err)
	assert.True(t, summary.CanRedo)

	messages, err := store.GetMessages(session.ID)
	require.NoError(t, err)
	require.Len(t, messages, 2)
	assert.Equal(t, "u1", messages[0].ID)
	assert.Equal(t, "a1", messages[1].ID)

	undo, err = store.UndoLastTurn(session.ID)
	require.NoError(t, err)
	assert.True(t, undo.Success)
	assert.Equal(t, "First prompt", undo.RestoredPrompt)

	messages, err = store.GetMessages(session.ID)
	require.NoError(t, err)
	assert.Empty(t, messages)

	redo, err := store.RedoLastTurn(session.ID)
	require.NoError(t, err)
	require.NotNil(t, redo)
	assert.True(t, redo.Success)
	assert.Equal(t, 2, redo.RestoredMessages)
	assert.True(t, redo.CanRedo)

	messages, err = store.GetMessages(session.ID)
	require.NoError(t, err)
	require.Len(t, messages, 2)
	assert.Equal(t, "First prompt", messages[0].Content)

	redo, err = store.RedoLastTurn(session.ID)
	require.NoError(t, err)
	assert.True(t, redo.Success)
	assert.False(t, redo.CanRedo)

	summary, err = store.Get(session.ID)
	require.NoError(t, err)
	assert.False(t, summary.CanRedo)

	messages, err = store.GetMessages(session.ID)
	require.NoError(t, err)
	require.Len(t, messages, 4)
	assert.Equal(t, "Second prompt", messages[2].Content)

	undo, err = store.UndoLastTurn(session.ID)
	require.NoError(t, err)
	assert.True(t, undo.CanRedo)
	summary, err = store.Get(session.ID)
	require.NoError(t, err)
	assert.True(t, summary.CanRedo)
	require.NoError(t, store.AddMessage(session.ID, Message{ID: "u3", Role: "user", Content: "Replacement"}))
	summary, err = store.Get(session.ID)
	require.NoError(t, err)
	assert.False(t, summary.CanRedo)

	redo, err = store.RedoLastTurn(session.ID)
	require.NoError(t, err)
	assert.False(t, redo.Success)
	assert.False(t, redo.CanRedo)
	assert.Equal(t, "No undone turn to redo.", redo.Message)
}

func TestSessionStoreForkRejectsMissingOrInvalidSession(t *testing.T) {
	store, err := NewSessionStore(t.TempDir())
	require.NoError(t, err)

	_, err = store.Fork("../bad")
	require.Error(t, err)

	_, err = store.Fork("missing-session")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "session not found")
}

func TestSessionStoreMigratesLegacySessionFileOnWrite(t *testing.T) {
	store, err := NewSessionStore(t.TempDir())
	require.NoError(t, err)

	now := time.Now().UTC().Round(time.Second)
	legacy := sessionData{
		ID:        "legacy-session",
		Title:     "",
		Messages:  []Message{},
		CreatedAt: now,
		UpdatedAt: now,
	}

	payload, err := json.MarshalIndent(legacy, "", "  ")
	require.NoError(t, err)

	legacyPath := filepath.Join(store.dataDir, legacy.ID+".json")
	require.NoError(t, os.WriteFile(legacyPath, payload, 0600))

	canonicalPath, err := store.sessionPath(legacy.ID)
	require.NoError(t, err)

	_, err = os.Stat(canonicalPath)
	require.ErrorIs(t, err, os.ErrNotExist)

	err = store.AddMessage(legacy.ID, Message{
		Role:    "user",
		Content: "hello from legacy storage",
	})
	require.NoError(t, err)

	session, err := store.Get(legacy.ID)
	require.NoError(t, err)
	assert.Equal(t, legacy.ID, session.ID)
	assert.Equal(t, "hello from legacy storage", session.Title)

	messages, err := store.GetMessages(legacy.ID)
	require.NoError(t, err)
	require.Len(t, messages, 1)
	assert.Equal(t, "hello from legacy storage", messages[0].Content)

	_, err = os.Stat(canonicalPath)
	require.NoError(t, err)

	_, err = os.Stat(legacyPath)
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestSessionStoreEnsureSessionUsesDirectLegacyFileWithoutFullScan(t *testing.T) {
	store, err := NewSessionStore(t.TempDir())
	require.NoError(t, err)

	now := time.Now().UTC().Round(time.Second)
	legacy := sessionData{
		ID:        "legacy-direct-session",
		Title:     "direct legacy title",
		Messages:  []Message{},
		CreatedAt: now,
		UpdatedAt: now,
	}
	payload, err := json.MarshalIndent(legacy, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(store.dataDir, legacy.ID+".json"), payload, 0600))

	session, err := store.EnsureSession(legacy.ID)
	require.NoError(t, err)
	assert.Equal(t, legacy.ID, session.ID)
	assert.Equal(t, legacy.Title, session.Title)
}

func TestSessionStoreListDoesNotHoldStoreMutex(t *testing.T) {
	store, err := NewSessionStore(t.TempDir())
	require.NoError(t, err)

	_, err = store.Create()
	require.NoError(t, err)

	store.mu.Lock()
	defer store.mu.Unlock()

	done := make(chan error, 1)
	go func() {
		_, err := store.List()
		done <- err
	}()

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(250 * time.Millisecond):
		t.Fatal("List blocked on the session store mutex")
	}
}

func TestSessionStoreListServesSummariesFromCacheUntilFileChanges(t *testing.T) {
	store, err := NewSessionStore(t.TempDir())
	require.NoError(t, err)

	session, err := store.Create()
	require.NoError(t, err)
	require.NoError(t, store.AddMessage(session.ID, Message{Role: "user", Content: "aaaa"}))
	require.NoError(t, store.AddMessage(session.ID, Message{Role: "assistant", Content: "reply"}))

	sessions, err := store.List()
	require.NoError(t, err)
	require.Len(t, sessions, 1)
	assert.Equal(t, "aaaa", sessions[0].Title)
	assert.Equal(t, 2, sessions[0].MessageCount)

	// Rewrite the file out of band with same-length content and restore the
	// original mtime: List must serve the cached summary without re-reading.
	path, err := store.sessionPath(session.ID)
	require.NoError(t, err)
	info, err := os.Stat(path)
	require.NoError(t, err)
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	patched := strings.Replace(string(raw), `"title": "aaaa"`, `"title": "bbbb"`, 1)
	require.NotEqual(t, string(raw), patched, "test fixture must contain the title to patch")
	require.NoError(t, os.WriteFile(path, []byte(patched), 0600))
	require.NoError(t, os.Chtimes(path, info.ModTime(), info.ModTime()))

	sessions, err = store.List()
	require.NoError(t, err)
	require.Len(t, sessions, 1)
	assert.Equal(t, "aaaa", sessions[0].Title, "unchanged (mtime, size) must serve the cached summary")

	// Bump the mtime: List must notice the change and re-parse the file.
	require.NoError(t, os.Chtimes(path, info.ModTime().Add(2*time.Second), info.ModTime().Add(2*time.Second)))
	sessions, err = store.List()
	require.NoError(t, err)
	require.Len(t, sessions, 1)
	assert.Equal(t, "bbbb", sessions[0].Title, "changed mtime must invalidate the cached summary")
	assert.Equal(t, 2, sessions[0].MessageCount)
}

func TestSessionStoreSummaryIndexSurvivesRestart(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSessionStore(dir)
	require.NoError(t, err)

	session, err := store.Create()
	require.NoError(t, err)
	require.NoError(t, store.AddMessage(session.ID, Message{Role: "user", Content: "indexed title"}))

	// A fresh store (simulated restart) must serve the summary from the
	// persisted index without re-reading the transcript: corrupt the message
	// payload on disk at unchanged (mtime, size) and expect the indexed
	// summary, not a parse of the corrupted file.
	path, err := store.sessionPath(session.ID)
	require.NoError(t, err)
	info, err := os.Stat(path)
	require.NoError(t, err)
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	patched := strings.Replace(string(raw), `"content": "indexed title"`, `"content": "altered titlexx"`, 1)
	require.NotEqual(t, string(raw), patched)
	require.NoError(t, os.WriteFile(path, []byte(patched), 0600))
	require.NoError(t, os.Chtimes(path, info.ModTime(), info.ModTime()))

	reopened, err := NewSessionStore(dir)
	require.NoError(t, err)
	sessions, err := reopened.List()
	require.NoError(t, err)
	require.Len(t, sessions, 1)
	assert.Equal(t, "indexed title", sessions[0].Title)
	assert.Equal(t, 1, sessions[0].MessageCount)

	// The index file itself must never show up as a session.
	for _, s := range sessions {
		assert.NotEmpty(t, s.ID)
	}
}

func TestSessionStoreListDropsExternallyDeletedSessions(t *testing.T) {
	store, err := NewSessionStore(t.TempDir())
	require.NoError(t, err)

	keep, err := store.Create()
	require.NoError(t, err)
	drop, err := store.Create()
	require.NoError(t, err)

	sessions, err := store.List()
	require.NoError(t, err)
	require.Len(t, sessions, 2)

	path, err := store.sessionPath(drop.ID)
	require.NoError(t, err)
	require.NoError(t, os.Remove(path))

	sessions, err = store.List()
	require.NoError(t, err)
	require.Len(t, sessions, 1)
	assert.Equal(t, keep.ID, sessions[0].ID)
}

func TestSessionStoreEnsureSessionDoesNotScanIndirectLegacyFilesForNewSession(t *testing.T) {
	store, err := NewSessionStore(t.TempDir())
	require.NoError(t, err)

	targetID := "new-chat-session"
	now := time.Now().UTC().Round(time.Second)
	indirectLegacy := sessionData{
		ID:        targetID,
		Title:     "should not be loaded during create",
		Messages:  []Message{},
		CreatedAt: now.Add(-time.Hour),
		UpdatedAt: now.Add(-time.Hour),
	}
	payload, err := json.MarshalIndent(indirectLegacy, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(store.dataDir, "unrelated-legacy-name.json"), payload, 0600))

	session, err := store.EnsureSession(targetID)
	require.NoError(t, err)
	assert.Equal(t, targetID, session.ID)
	assert.Empty(t, session.Title)
	assert.True(t, session.CreatedAt.After(indirectLegacy.CreatedAt))

	canonicalPath, err := store.sessionPath(targetID)
	require.NoError(t, err)
	_, err = os.Stat(canonicalPath)
	require.NoError(t, err)
}

func TestSessionStoreGetStillFindsIndirectLegacySession(t *testing.T) {
	store, err := NewSessionStore(t.TempDir())
	require.NoError(t, err)

	now := time.Now().UTC().Round(time.Second)
	legacy := sessionData{
		ID:        "indirect-legacy-session",
		Title:     "indirect legacy title",
		Messages:  []Message{},
		CreatedAt: now,
		UpdatedAt: now,
	}
	payload, err := json.MarshalIndent(legacy, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(store.dataDir, "legacy-file-with-old-name.json"), payload, 0600))

	session, err := store.Get(legacy.ID)
	require.NoError(t, err)
	assert.Equal(t, legacy.ID, session.ID)
	assert.Equal(t, legacy.Title, session.Title)
}

func TestSessionStoreSessionPathUsesOpaqueHashedLeaf(t *testing.T) {
	store, err := NewSessionStore(t.TempDir())
	require.NoError(t, err)

	sessionID := "guest-alpha_123"
	path, err := store.sessionPath(sessionID)
	require.NoError(t, err)

	base := filepath.Base(path)
	assert.True(t, strings.HasSuffix(base, ".json"))
	assert.NotContains(t, base, "guest")
	assert.NotContains(t, base, "alpha")
}

func TestMessage_UsesCanonicalEmptyCollections(t *testing.T) {
	payload, err := json.Marshal(EmptyMessage())
	require.NoError(t, err)
	assert.Contains(t, string(payload), `"tool_calls":[]`)

	payload, err = json.Marshal(EmptyToolCall())
	require.NoError(t, err)
	assert.Contains(t, string(payload), `"input":{}`)

	payload, err = json.Marshal(Message{
		ToolCalls: []ToolCall{{
			ID:   "call-1",
			Name: "diagnose",
		}},
	}.NormalizeCollections())
	require.NoError(t, err)
	assert.Contains(t, string(payload), `"input":{}`)
}

func TestGenerateTitle(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Short message", "Short message"},
		{"This is a very long message that should definitely be truncated because it exceeds the fifty character limit", "This is a very long message that should..."},
		{"Multiple    spaces    cleaned", "Multiple spaces cleaned"},
		{"New\nLines\nRemoved", "New Lines Removed"},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, generateTitle(tt.input))
	}
}
