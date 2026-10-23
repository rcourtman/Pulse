package chat

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
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

func TestToolResultUsesSharedAgentCapabilitiesShape(t *testing.T) {
	var shared agentcapabilities.ProviderToolResult = ToolResult{
		ToolUseID: "call-1",
		Content:   "done",
		IsError:   true,
	}
	assert.Equal(t, "call-1", shared.ToolUseID)

	payload, err := json.Marshal(ToolResult{
		ToolUseID: "call-1",
		Content:   "done",
		IsError:   true,
	})
	require.NoError(t, err)
	assert.JSONEq(t, `{"tool_use_id":"call-1","content":"done","is_error":true}`, string(payload))
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

func TestSessionListMarksSystemSessions(t *testing.T) {
	store, err := NewSessionStore(t.TempDir())
	require.NoError(t, err)

	_, err = store.EnsureSession("patrol-main")
	require.NoError(t, err)
	userSession, err := store.Create()
	require.NoError(t, err)

	sessions, err := store.List()
	require.NoError(t, err)
	require.Len(t, sessions, 2)

	byID := make(map[string]Session, len(sessions))
	for _, s := range sessions {
		byID[s.ID] = s
	}
	assert.True(t, byID["patrol-main"].System, "patrol-main must be marked system so clients keep it out of resumable chat lists")
	assert.False(t, byID[userSession.ID].System, "user sessions must not be marked system")
}

func TestIsSystemSessionID(t *testing.T) {
	assert.True(t, IsSystemSessionID("patrol-main"))
	assert.True(t, IsSystemSessionID("patrol-eval"))
	assert.True(t, IsSystemSessionID("investigation-abc123"))
	assert.False(t, IsSystemSessionID("patrol-adjacent-user-session"))
	assert.False(t, IsSystemSessionID(""))
	assert.False(t, IsSystemSessionID("my-chat"))
}
